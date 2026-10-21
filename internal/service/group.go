package service

import (
	"context"
	"crypto/rand"
	"regexp"
	"strings"

	"github.com/chyoumu114/lol-civil-war/internal/domain/rank"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

var groupNumberRe = regexp.MustCompile(`^\d{5,12}$`)

// ProfilePayload 是资料保存请求。
type ProfilePayload struct {
	Nickname  string    `json:"nickname"`
	QQNumber  string    `json:"qqNumber"`
	Rank      RankInput `json:"rank"`
	Positions []string  `json:"positions"`
}

// RankInput 是段位输入。
type RankInput struct {
	Region   string `json:"region"`
	Tier     string `json:"tier"`
	Division *int   `json:"division"`
	LP       int    `json:"lp"`
}

// ResolveGroup 解析 URL 中的群标识：优先公开 key，兼容旧数字群号。
func (a *App) ResolveGroup(ctx context.Context, identifier string) (store.Group, error) {
	group, err := a.St.GroupByPublicKey(ctx, identifier)
	if err == nil {
		allowed, allowErr := a.groupAllowed(ctx, group.QQGroupNumber)
		if allowErr != nil {
			return store.Group{}, allowErr
		}
		if !allowed {
			return store.Group{}, a.groupNotAllowed(group.QQGroupNumber)
		}
		return group, nil
	}
	if !IsNotFound(err) {
		return store.Group{}, err
	}
	if !groupNumberRe.MatchString(identifier) {
		return store.Group{}, a.groupNotAllowed(identifier)
	}
	return a.ResolveGroupByNumber(ctx, identifier)
}

// ResolveGroupByNumber 用群号换取群（白名单校验，不存在则创建）。
func (a *App) ResolveGroupByNumber(ctx context.Context, number string) (store.Group, error) {
	allowed, err := a.groupAllowed(ctx, number)
	if err != nil {
		return store.Group{}, err
	}
	if !allowed {
		return store.Group{}, a.groupNotAllowed(number)
	}
	group, err := a.St.EnsureGroup(ctx, number, randomKey(32))
	if err != nil {
		return store.Group{}, err
	}
	return group, nil
}

// groupAllowed 判断群号是否被允许：环境变量白名单（引导/兜底）或数据库白名单。
func (a *App) groupAllowed(ctx context.Context, number string) (bool, error) {
	for _, allowed := range a.Cfg.AllowedGroups {
		if allowed == number {
			return true, nil
		}
	}
	return a.St.IsGroupAllowed(ctx, number)
}

func (a *App) groupNotAllowed(number string) *Error {
	return NewError(403, "群号 "+number+" 未开通内战工具，请联系管理员添加到白名单")
}

// EnsureMember 取（或创建）当前登录用户在群内的资料。
func (a *App) EnsureMember(ctx context.Context, group store.Group, user store.User) (store.Member, error) {
	nickname := user.Name
	if nickname == "" {
		nickname = "QQ 用户"
	}
	return a.St.EnsureMember(ctx, group.ID, user.ID, nickname)
}

// UpdateProfile 保存群内资料（QQ 号首次填写后锁定）。
func (a *App) UpdateProfile(ctx context.Context, member store.Member, payload ProfilePayload) (store.Member, MemberView, error) {
	nickname := strings.TrimSpace(payload.Nickname)
	if nickname == "" || len([]rune(nickname)) > 16 {
		return store.Member{}, MemberView{}, ErrInvalid("昵称长度需在 1-16 个字符之间")
	}

	// 页面展示的是脱敏 QQ（含 *）；原样提交/留空表示不修改。
	qqRaw := strings.TrimSpace(payload.QQNumber)
	qqNumber := qqRaw
	if qqRaw == "" || strings.Contains(qqRaw, "*") {
		if member.QQNumber == nil || *member.QQNumber == "" {
			return store.Member{}, MemberView{}, ErrInvalid("请填写 5-12 位数字的 QQ 号")
		}
		qqNumber = *member.QQNumber
	} else if !qqNumberRe.MatchString(qqRaw) {
		return store.Member{}, MemberView{}, ErrInvalid("请填写 5-12 位数字的 QQ 号")
	}
	taken, err := a.St.MemberQQNumberTaken(ctx, member.GroupID, qqNumber, member.ID)
	if err != nil {
		return store.Member{}, MemberView{}, err
	}
	if taken {
		return store.Member{}, MemberView{}, ErrInvalid("该 QQ 号已被群内其他成员使用")
	}

	region := strings.TrimSpace(payload.Rank.Region)
	if !rank.IsRegion(region) {
		return store.Member{}, MemberView{}, ErrInvalid("请选择有效的比赛大区")
	}
	tier, ok := rank.ParseTier(payload.Rank.Tier)
	if !ok {
		return store.Member{}, MemberView{}, ErrInvalid("请选择有效的最高段位")
	}
	var division *int
	lp := 0
	if tier.HasDivisions() {
		if payload.Rank.Division == nil || *payload.Rank.Division < 1 || *payload.Rank.Division > 4 {
			return store.Member{}, MemberView{}, ErrInvalid(tier.Label() + "必须选择 1-4 的小段")
		}
		d := *payload.Rank.Division
		division = &d
	} else {
		lp = payload.Rank.LP
		if lp < 0 || lp > 100000 {
			return store.Member{}, MemberView{}, ErrInvalid("分数需在 0-100000 之间")
		}
	}

	positions := make([]string, 0, len(payload.Positions))
	seen := map[string]struct{}{}
	for _, raw := range payload.Positions {
		position, ok := rank.ParsePosition(raw)
		if !ok {
			continue
		}
		key := string(position)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		positions = append(positions, key)
	}
	if len(positions) == 0 {
		return store.Member{}, MemberView{}, ErrInvalid("请至少选择一个擅长位置")
	}

	member.Nickname = nickname
	member.QQNumber = &qqNumber
	member.RankRegion = &region
	tierCode := string(tier)
	member.RankTier = &tierCode
	member.RankDivision = division
	member.RankLP = lp
	member.Positions = positions

	if err := a.St.UpdateMemberProfile(ctx, member); err != nil {
		return store.Member{}, MemberView{}, err
	}
	updated, err := a.St.MemberByID(ctx, member.GroupID, member.ID)
	if err != nil {
		return store.Member{}, MemberView{}, err
	}
	return updated, a.memberView(updated), nil
}

func randomKey(n int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand 不可用时退化为固定前缀，保证服务仍能启动。
		return strings.Repeat("0", n)
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf)
}
