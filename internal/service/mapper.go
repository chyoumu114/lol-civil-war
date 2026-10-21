package service

import (
	"math"
	"strings"

	"github.com/chyoumu114/lol-civil-war/internal/domain/match"
	"github.com/chyoumu114/lol-civil-war/internal/domain/rank"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

// maskQQ 页面展示脱敏：保留前段，隐藏最后三位。
func maskQQ(qq *string) *string {
	if qq == nil || *qq == "" {
		return nil
	}
	runes := []rune(*qq)
	if len(runes) <= 3 {
		v := "***"
		return &v
	}
	v := string(runes[:len(runes)-3]) + "***"
	return &v
}

func (a *App) userView(u *store.User) *UserView {
	if u == nil {
		return nil
	}
	return &UserView{ID: u.ID, Name: u.Name, Avatar: u.Avatar, IsDev: u.IsDev}
}

// memberRank 从成员资料构造段位（资料不完整返回 nil）。
func (a *App) memberRank(m store.Member) *rank.Profile {
	if m.RankRegion == nil || m.RankTier == nil {
		return nil
	}
	tier, ok := rank.ParseTier(*m.RankTier)
	if !ok {
		return nil
	}
	profile, err := rank.NewProfile(*m.RankRegion, tier, m.RankDivision, m.RankLP, a.Cfg.DivisionStep)
	if err != nil {
		return nil
	}
	return &profile
}

func isProfileComplete(m store.Member) bool {
	return m.QQNumber != nil && *m.QQNumber != "" && m.RankTier != nil && *m.RankTier != "" && len(m.Positions) > 0
}

func rankViewOf(profile rank.Profile) RankView {
	return RankView{
		Region:    profile.Region,
		Tier:      string(profile.Tier),
		TierLabel: profile.Tier.Label(),
		Division:  profile.Division,
		LP:        profile.LP,
		Label:     profile.Label(),
	}
}

func (a *App) memberView(m store.Member) MemberView {
	positions := m.Positions
	if positions == nil {
		positions = []string{}
	}
	view := MemberView{
		ID:              m.ID,
		QQNumber:        maskQQ(m.QQNumber),
		Nickname:        m.Nickname,
		Avatar:          m.Avatar,
		Positions:       positions,
		ProfileComplete: isProfileComplete(m),
		JoinedAt:        a.isoPtr(&m.CreatedAt),
	}
	if profile := a.memberRank(m); profile != nil {
		view.Rank = &MemberRankView{RankView: rankViewOf(*profile), BaseScore: profile.BaseScore()}
	}
	return view
}

// registrationRank 返回报名的主/补位段位（补位逐字段回退主段位）。
func (a *App) registrationRank(r store.Registration, sub bool) rank.Profile {
	region := r.RankRegion
	tierRaw := r.RankTier
	division := r.RankDivision
	lp := r.RankLP

	if sub {
		if r.SubRankRegion != nil && *r.SubRankRegion != "" {
			region = *r.SubRankRegion
		}
		if r.SubRankTier != nil && *r.SubRankTier != "" {
			tierRaw = *r.SubRankTier
		}
		if r.SubRankDivision != nil {
			division = r.SubRankDivision
		}
		if r.SubRankLP > 0 {
			lp = r.SubRankLP
		}
	}

	tier, ok := rank.ParseTier(tierRaw)
	if !ok {
		tier = rank.TierIron
	}
	profile, err := rank.NewProfile(region, tier, division, lp, a.Cfg.DivisionStep)
	if err != nil {
		profile = rank.Profile{Region: region, Tier: tier, Division: division, LP: lp, DivisionStep: a.Cfg.DivisionStep}
	}
	return profile
}

// registrationRankView 构造报名段位 JSON（label 与旧接口一致）。
func (a *App) registrationRankView(r store.Registration, sub bool) RankView {
	region := r.RankRegion
	tierRaw := r.RankTier
	division := r.RankDivision
	lp := r.RankLP

	if sub {
		if r.SubRankRegion != nil && *r.SubRankRegion != "" {
			region = *r.SubRankRegion
		}
		if r.SubRankTier != nil && *r.SubRankTier != "" {
			tierRaw = *r.SubRankTier
		}
		if r.SubRankDivision != nil {
			division = r.SubRankDivision
		}
		if r.SubRankLP > 0 {
			lp = r.SubRankLP
		}
	}

	tierLabel := tierRaw
	if tier, ok := rank.ParseTier(tierRaw); ok {
		tierLabel = tier.Label()
	}
	tail := ""
	if division != nil {
		tail = itoa(*division)
	} else if lp > 0 {
		tail = itoa(lp) + "分"
	}
	return RankView{
		Region:    region,
		Tier:      tierRaw,
		TierLabel: tierLabel,
		Division:  division,
		LP:        lp,
		Label:     strings.TrimSpace(region + " " + tierLabel + tail),
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// eventStatus 返回 open / closed / finished。
func (a *App) eventStatus(e store.Event) string {
	if e.EndedAt != nil {
		return "finished"
	}
	if a.now().Before(e.SignupDeadline) {
		return "open"
	}
	return "closed"
}

func (a *App) eventView(e store.Event) EventView {
	view := EventView{
		ID:             e.ID,
		Title:          e.Title,
		Region:         e.Region,
		BestOf:         e.BestOf,
		HasBP:          e.HasBP,
		SignupDeadline: a.isoPtr(&e.SignupDeadline),
		StartsAt:       a.isoPtr(&e.StartsAt),
		Note:           e.Note,
		Status:         a.eventStatus(e),
		EndedAt:        a.isoPtr(e.EndedAt),
		CreatedAt:      a.isoPtr(&e.CreatedAt),
	}
	view.Creator = &CreatorView{
		MemberID: e.CreatorMemberID,
		Nickname: e.CreatorNickname,
		Avatar:   e.CreatorAvatar,
		QQNumber: maskQQ(e.CreatorQQNumber),
	}
	return view
}

// registrationView 构造报名 JSON；editable 由调用方按场景决定。
func (a *App) registrationView(r store.Registration, e store.Event, editable bool) RegistrationView {
	subPositions := r.SubPositions
	if subPositions == nil {
		subPositions = []string{}
	}
	powers := a.positionPowers(r)
	return RegistrationView{
		ID:             r.ID,
		MemberID:       r.MemberID,
		Nickname:       r.MemberNickname,
		Avatar:         r.MemberAvatar,
		QQNumber:       maskQQ(r.MemberQQNumber),
		Rank:           a.registrationRankView(r, false),
		SubRank:        a.registrationRankView(r, true),
		MainPosition:   r.MainPosition,
		SubPositions:   subPositions,
		Note:           r.Note,
		IsSpectator:    r.IsSpectator,
		PositionPowers: powers,
		Editable:       editable,
		IsOrganizer:    r.MemberID == e.CreatorMemberID,
		CreatedAt:      a.isoPtr(&r.CreatedAt),
	}
}

// positionPowers 返回该报名者能打的每个位置的战力；空时返回 []（而不是 {}）。
func (a *App) positionPowers(r store.Registration) any {
	if r.IsSpectator || r.RankTier == "" {
		return []any{}
	}
	mainRank := a.registrationRank(r, false)
	subRank := a.registrationRank(r, true)

	subs := make(map[string]bool, len(r.SubPositions))
	for _, p := range r.SubPositions {
		subs[p] = true
	}

	out := make(map[string]int, len(rank.Positions))
	for _, pos := range rank.Positions {
		switch {
		case r.MainPosition != nil && *r.MainPosition == string(pos):
			out[string(pos)] = a.Calc.Power(mainRank, pos)
		case subs[string(pos)]:
			out[string(pos)] = a.Calc.Power(subRank, pos)
		}
	}
	if len(out) == 0 {
		return []any{}
	}
	return out
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }

// seriesOf 计算系列赛比分（按蓝/红与 1队/2队两套口径）。
func (a *App) seriesOf(e store.Event, matches []store.Match) SeriesView {
	view := SeriesView{
		BestOf:       e.BestOf,
		WinsRequired: match.WinsRequired(e.BestOf),
		Matches:      make([]SeriesMatchView, 0, len(matches)),
	}

	lastTeam1Side := ""
	for _, m := range matches {
		if m.BlueWin != nil {
			if *m.BlueWin {
				view.BlueWins++
			} else {
				view.RedWins++
			}
			side := m.Team1Side
			if side == "" {
				side = "blue"
			}
			lastTeam1Side = side
			if (side == "blue" && *m.BlueWin) || (side == "red" && !*m.BlueWin) {
				view.Team1Wins++
			} else {
				view.Team2Wins++
			}
		}
	}

	if side, ok := match.Winner(e.BestOf, view.BlueWins, view.RedWins); ok {
		s := side
		view.Winner = &s
	}

	need := view.WinsRequired
	if view.Team1Wins >= need || view.Team2Wins >= need {
		winner := 1
		if view.Team1Wins <= view.Team2Wins {
			winner = 2
		}
		view.TeamWinner = &winner
		if lastTeam1Side == "" {
			lastTeam1Side = "blue"
		}
		side := lastTeam1Side
		if winner == 2 {
			if lastTeam1Side == "blue" {
				side = "red"
			} else {
				side = "blue"
			}
		}
		view.WinnerSide = &side
	}

	for _, m := range matches {
		side := m.Team1Side
		if side == "" {
			side = "blue"
		}
		sm := SeriesMatchView{
			ID:          m.ID,
			GameNo:      m.GameNo,
			Status:      m.Status,
			BlueWin:     m.BlueWin,
			Team1Side:   side,
			DurationMS:  m.DurationMS,
			SubmittedAt: a.isoPtr(m.SubmittedAt),
		}
		if m.BlueWin != nil {
			winnerSide := "blue"
			if !*m.BlueWin {
				winnerSide = "red"
			}
			sm.WinnerSide = &winnerSide
			winnerTeam := 1
			if side != winnerSide {
				winnerTeam = 2
			}
			sm.WinnerTeam = &winnerTeam
		}
		view.Matches = append(view.Matches, sm)
	}
	return view
}

// MemberView 导出版本，供 web 层序列化。
func (a *App) MemberView(m store.Member) MemberView { return a.memberView(m) }
