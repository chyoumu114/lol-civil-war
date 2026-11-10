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

// 统一的“群不可用”文案：不区分“未配置白名单”与“白名单但尚未建档”，
// 避免通过 403/404 差异枚举白名单（L-06）。
const groupUnavailableMessage = "该群暂未开通内战工具，请联系管理员"

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

// ResolveGroup 解析 URL 中的群标识：只接受随机公开 key（I-03 关闭数字群号直达）。
func (a *App) ResolveGroup(ctx context.Context, identifier string) (store.Group, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || groupNumberRe.MatchString(identifier) {
		return store.Group{}, a.groupNotAllowed()
	}
	group, err := a.St.GroupByPublicKey(ctx, identifier)
	if err != nil {
		if IsNotFound(err) {
			return store.Group{}, a.groupNotAllowed()
		}
		return store.Group{}, err
	}
	allowed, allowErr := a.groupAllowed(ctx, group.QQGroupNumber)
	if allowErr != nil {
		return store.Group{}, allowErr
	}
	if !allowed {
		return store.Group{}, a.groupNotAllowed()
	}
	return group, nil
}

// ResolveGroupByNumber 用群号换取群。
//
// 只读取已存在的群记录：数据库白名单的建档由 `group add` 完成，
// 解析路径不再插入新行（L-06）。仅环境变量白名单（静态、数量有限）允许首访自动建档，
// 保证“只改 .env 就能用”的部署方式仍然成立。
func (a *App) ResolveGroupByNumber(ctx context.Context, number string) (store.Group, error) {
	allowed, err := a.groupAllowed(ctx, number)
	if err != nil {
		return store.Group{}, err
	}
	if !allowed {
		return store.Group{}, a.groupNotAllowed()
	}
	group, err := a.St.GroupByNumber(ctx, number)
	if err == nil {
		return group, nil
	}
	if !IsNotFound(err) {
		return store.Group{}, err
	}
	if !a.envGroupAllowed(number) {
		return store.Group{}, a.groupNotAllowed()
	}
	// crypto/rand.Text 在随机源不可用时 panic：fail-closed，绝不退化到固定 key（L-07）。
	return a.St.EnsureGroup(ctx, number, rand.Text())
}

// groupAllowed 判断群号是否被允许：环境变量白名单（引导/兜底）或数据库白名单。
func (a *App) groupAllowed(ctx context.Context, number string) (bool, error) {
	if a.envGroupAllowed(number) {
		return true, nil
	}
	return a.St.IsGroupAllowed(ctx, number)
}

func (a *App) envGroupAllowed(number string) bool {
	for _, allowed := range a.Cfg.AllowedGroups {
		if allowed == number {
			return true
		}
	}
	return false
}

func (a *App) groupNotAllowed() *Error {
	return NewError(404, groupUnavailableMessage)
}

// EnsureMember 取（或创建）当前登录用户在群内的资料。
func (a *App) EnsureMember(ctx context.Context, group store.Group, user store.User) (store.Member, error) {
	// 昵称允许暂为空；资料完整性会阻止报名/发起组织，用户在个人资料里补填。
	return a.St.EnsureMember(ctx, group.ID, user.ID, user.Name)
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
