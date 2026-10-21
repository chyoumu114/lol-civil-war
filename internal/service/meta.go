package service

import (
	"context"

	"github.com/chyoumu114/lol-civil-war/internal/domain/match"
	"github.com/chyoumu114/lol-civil-war/internal/domain/rank"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

// Meta 返回前端静态配置。
func (a *App) Meta() MetaView {
	tiers := make([]TierView, 0, len(rank.Tiers))
	for _, tier := range rank.Tiers {
		tiers = append(tiers, TierView{
			Code:         string(tier),
			Label:        tier.Label(),
			HasDivisions: tier.HasDivisions(),
			StartScore:   tier.StartScore(),
		})
	}

	positions := make([]PositionView, 0, len(rank.Positions))
	for _, position := range rank.Positions {
		positions = append(positions, PositionView{
			Code:      string(position),
			Label:     position.Label(),
			Short:     position.Short(),
			Influence: rank.Influence(position),
		})
	}

	return MetaView{
		Regions:      rank.RegionNames(),
		RankTiers:    tiers,
		Positions:    positions,
		BestOf:       match.Allowed,
		DevLogin:     a.Cfg.DevLogin,
		QQConfigured: a.Cfg.QQConfigured(),
		MaxRoflKB:    a.Cfg.RoflMaxKB,
	}
}

// PowerInput 是战力计算器参数。
type PowerInput struct {
	Position string
	Region   string
	Tier     string
	Division *int
	LP       int
}

// CalculatePower 计算战力并返回拆解。
func (a *App) CalculatePower(input PowerInput) (PowerView, error) {
	position, ok := rank.ParsePosition(input.Position)
	if !ok {
		return PowerView{}, ErrInvalid("分路不合法")
	}
	if !rank.IsRegion(input.Region) {
		return PowerView{}, ErrInvalid("大区不合法")
	}
	tier, ok := rank.ParseTier(input.Tier)
	if !ok {
		return PowerView{}, ErrInvalid("段位不合法")
	}

	var division *int
	lp := 0
	if tier.HasDivisions() {
		if input.Division == nil || *input.Division < 1 || *input.Division > 4 {
			return PowerView{}, ErrInvalid("小段必须是 1-4")
		}
		d := *input.Division
		division = &d
	} else {
		lp = input.LP
	}

	profile, err := rank.NewProfile(input.Region, tier, division, lp, a.Cfg.DivisionStep)
	if err != nil {
		return PowerView{}, ErrInvalid(err.Error())
	}
	power := a.Calc.Power(profile, position)
	return PowerView{
		Position:          string(position),
		PositionLabel:     position.Label(),
		PositionShort:     position.Short(),
		Region:            profile.Region,
		Tier:              string(tier),
		TierLabel:         tier.Label(),
		Division:          division,
		LP:                lp,
		Rank:              rankViewOf(profile),
		BaseScore:         profile.BaseScore(),
		RegionCoefficient: rank.RegionCoefficient(profile.Region),
		PositionInfluence: rank.Influence(position),
		Power:             power,
	}, nil
}

// SetSpectator 添加/取消观战。
func (a *App) SetSpectator(ctx context.Context, event store.Event, member store.Member, watching bool) error {
	if event.EndedAt != nil {
		return ErrConflict("比赛已结束，不能修改观战状态")
	}
	return a.St.SetEventSpectator(ctx, event.ID, member.ID, watching)
}
