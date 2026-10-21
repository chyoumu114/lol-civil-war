package service

import (
	"context"
	"strings"

	"github.com/chyoumu114/lol-civil-war/internal/domain/rank"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

// RegistrationPayload 是报名/修改报名的请求体。
type RegistrationPayload struct {
	IsSpectator  bool       `json:"isSpectator"`
	MainPosition string     `json:"mainPosition"`
	SubPositions []string   `json:"subPositions"`
	Rank         RankInput  `json:"rank"`
	SubRank      *RankInput `json:"subRank"`
	Note         string     `json:"note"`
}

// registrationRankInput 解析并校验一段段位。
// 返回 region/tier/division/lp；defaults 用于补位段位留空时继承主段位。
func (a *App) registrationRankInput(input RankInput, label string, defaults *RankInput) (string, string, *int, int, error) {
	region := strings.TrimSpace(input.Region)
	tierRaw := strings.TrimSpace(input.Tier)

	if region == "" && tierRaw == "" && defaults != nil {
		return defaults.Region, defaults.Tier, defaults.Division, defaults.LP, nil
	}
	if !rank.IsRegion(region) {
		return "", "", nil, 0, ErrInvalid(label + "：请选择有效的大区")
	}
	tier, ok := rank.ParseTier(tierRaw)
	if !ok {
		return "", "", nil, 0, ErrInvalid(label + "：请选择有效段位")
	}
	if tier.HasDivisions() {
		if input.Division == nil || *input.Division < 1 || *input.Division > 4 {
			return "", "", nil, 0, ErrInvalid(label + "：" + tier.Label() + "必须选择 1-4 的小段")
		}
		d := *input.Division
		return region, string(tier), &d, 0, nil
	}
	if input.LP < 0 || input.LP > 100000 {
		return "", "", nil, 0, ErrInvalid(label + "：分数需在 0-100000 之间")
	}
	return region, string(tier), nil, input.LP, nil
}

// SaveRegistration 报名或修改报名。
func (a *App) SaveRegistration(ctx context.Context, event store.Event, target store.Member, actorIsOrganizer bool, payload RegistrationPayload) (store.Registration, error) {
	if event.EndedAt != nil {
		return store.Registration{}, ErrConflict("比赛已结束，报名不可修改")
	}
	if !isProfileComplete(target) {
		return store.Registration{}, ErrInvalid("该成员尚未补全个人资料（QQ 号、段位、擅长位置）")
	}
	signupOpen := a.now().Before(event.SignupDeadline)
	if !actorIsOrganizer && !signupOpen {
		return store.Registration{}, ErrConflict("报名已截止，修改报名信息请联系组织者")
	}

	var mainPosition *rank.Position
	rawMain := strings.TrimSpace(payload.MainPosition)
	if rawMain != "" {
		position, ok := rank.ParsePosition(rawMain)
		if !ok {
			return store.Registration{}, ErrInvalid("主选位置不合法")
		}
		mainPosition = &position
	}

	subs := make([]string, 0, len(payload.SubPositions))
	seen := map[string]struct{}{}
	for _, raw := range payload.SubPositions {
		position, ok := rank.ParsePosition(raw)
		if !ok {
			continue
		}
		key := string(position)
		if mainPosition != nil && *mainPosition == position {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		subs = append(subs, key)
	}
	if !payload.IsSpectator && mainPosition == nil && len(subs) == 0 {
		return store.Registration{}, ErrInvalid("主选位置和补位位置至少填写一个")
	}

	region, tier, division, lp, err := a.registrationRankInput(payload.Rank, "主选段位", nil)
	if err != nil {
		return store.Registration{}, err
	}
	mainRank := &RankInput{Region: region, Tier: tier, Division: division, LP: lp}

	var subRank *RankInput
	if payload.SubRank != nil {
		subRegion, subTier, subDivision, subLP, err := a.registrationRankInput(*payload.SubRank, "补位段位", mainRank)
		if err != nil {
			return store.Registration{}, err
		}
		subRank = &RankInput{Region: subRegion, Tier: subTier, Division: subDivision, LP: subLP}
	}

	note := strings.TrimSpace(payload.Note)
	if len([]rune(note)) > 200 {
		return store.Registration{}, ErrInvalid("备注最多 200 个字符")
	}

	var mainPositionValue *string
	if mainPosition != nil {
		v := string(*mainPosition)
		mainPositionValue = &v
	}

	registration := store.Registration{
		EventID:      event.ID,
		MemberID:     target.ID,
		RankRegion:   region,
		RankTier:     tier,
		RankDivision: division,
		RankLP:       lp,
		MainPosition: mainPositionValue,
		SubPositions: subs,
		IsSpectator:  payload.IsSpectator,
	}
	if note != "" {
		registration.Note = &note
	}
	if subRank != nil {
		registration.SubRankRegion = &subRank.Region
		registration.SubRankTier = &subRank.Tier
		registration.SubRankDivision = subRank.Division
		registration.SubRankLP = subRank.LP
	}
	return a.St.UpsertRegistration(ctx, registration)
}

// ToggleSpectator 组织者标记/取消替补。
func (a *App) ToggleSpectator(ctx context.Context, event store.Event, actor store.Member, target store.Member, spectator bool) error {
	if event.CreatorMemberID != actor.ID {
		return ErrForbidden("只有组织者可以标记替补")
	}
	if event.EndedAt != nil {
		return ErrConflict("比赛已结束，报名不可修改")
	}
	if err := a.St.UpdateRegistrationSpectator(ctx, event.ID, target.ID, spectator); err != nil {
		if IsNotFound(err) {
			return ErrNotFound("该成员还没有报名")
		}
		return err
	}
	// 从已套用阵容移除该成员，但不修改其位置字段（取消替补后可直接恢复）。
	return a.RemoveMemberFromRoster(ctx, event.ID, target.ID)
}

// DeleteRegistration 取消报名（幂等）。
func (a *App) DeleteRegistration(ctx context.Context, event store.Event, actor, target store.Member, actorIsOrganizer bool) error {
	if !actorIsOrganizer && target.ID != actor.ID {
		return ErrForbidden("只能取消自己的报名")
	}
	if event.EndedAt != nil {
		return ErrConflict("比赛已结束，报名不可修改")
	}
	if !actorIsOrganizer && !a.now().Before(event.SignupDeadline) {
		return ErrConflict("报名已截止，取消报名请联系组织者")
	}
	return a.St.DeleteRegistration(ctx, event.ID, target.ID)
}
