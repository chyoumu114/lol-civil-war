package roster

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chyoumu114/lol-civil-war/internal/domain/rank"
)

// Matcher 通过回溯搜索生成最平衡的阵容方案。
type Matcher struct {
	calc     *rank.PowerCalculator
	maxPlans int
	budget   int

	best       []Plan
	used       map[int64]bool
	seen       map[string]bool
	picked     [5][2]pick
	nodes      int
	exhausted  bool
	byPosition map[rank.Position][]pick
}

type pick struct {
	cand  Candidate
	power int
}

// NewMatcher 构造匹配器；maxPlans 为返回方案数，budget 为搜索节点预算。
func NewMatcher(calc *rank.PowerCalculator, maxPlans, budget int) *Matcher {
	if maxPlans < 1 {
		maxPlans = 5
	}
	if budget < 1000 {
		budget = 3_000_000
	}
	return &Matcher{calc: calc, maxPlans: maxPlans, budget: budget}
}

// Match 执行匹配。候选不足或某位置不足 2 人时返回 *Error。
func (m *Matcher) Match(candidates []Candidate) (Result, error) {
	m.byPosition = m.buildCandidatesByPosition(candidates)

	positionCounts := make(map[string]int, len(rank.Positions))
	missing := make([]string, 0, len(rank.Positions))
	for _, pos := range rank.Positions {
		count := len(m.byPosition[pos])
		positionCounts[string(pos)] = count
		if count < 2 {
			missing = append(missing, pos.Label())
		}
	}

	candidateCount := 0
	seenReg := make(map[int64]struct{}, len(candidates))
	for _, c := range candidates {
		if _, ok := seenReg[c.RegistrationID]; !ok {
			seenReg[c.RegistrationID] = struct{}{}
			candidateCount++
		}
	}

	if candidateCount < 10 {
		return Result{}, &Error{Message: fmt.Sprintf("报名人数不足：需要至少 10 人，当前 %d 人", candidateCount)}
	}
	if len(missing) > 0 {
		return Result{}, &Error{Message: "以下位置报名不足 2 人，无法匹配：" + strings.Join(missing, "、")}
	}

	m.best = nil
	m.used = make(map[int64]bool, 12)
	m.seen = make(map[string]bool)
	m.picked = [5][2]pick{}
	m.nodes = 0
	m.exhausted = false

	m.dfs(0, 0, 0)

	sort.SliceStable(m.best, func(i, j int) bool { return planLess(m.best[i], m.best[j]) })

	plans := m.best
	if len(plans) > m.maxPlans {
		plans = plans[:m.maxPlans]
	}
	if plans == nil {
		plans = make([]Plan, 0)
	}

	return Result{
		Plans:          plans,
		Exhausted:      m.exhausted,
		Nodes:          m.nodes,
		PositionCounts: positionCounts,
		CandidateCount: candidateCount,
	}, nil
}

// buildCandidatesByPosition 构建每个位置的候选列表。
// 主选优先：该位置主选人数够 2 人时不再引入补位候选。
func (m *Matcher) buildCandidatesByPosition(candidates []Candidate) map[rank.Position][]pick {
	out := make(map[rank.Position][]pick, len(rank.Positions))
	for _, pos := range rank.Positions {
		main := make([]pick, 0, 4)
		sub := make([]pick, 0, 4)
		for _, c := range candidates {
			switch {
			case c.MainPosition != nil && *c.MainPosition == pos:
				main = append(main, pick{cand: c, power: m.calc.Power(c.RankFor(pos), pos)})
			case c.CanPlay(pos):
				sub = append(sub, pick{cand: c, power: m.calc.Power(c.RankFor(pos), pos)})
			}
		}
		if len(main) >= 2 {
			out[pos] = main
		} else {
			out[pos] = append(main, sub...)
		}
	}
	return out
}

func (m *Matcher) dfs(index, team1Power, team2Power int) {
	if m.exhausted {
		return
	}
	m.nodes++
	if m.nodes > m.budget {
		m.exhausted = true
		return
	}
	if index == len(rank.Positions) {
		m.record(team1Power, team2Power)
		return
	}

	pos := rank.Positions[index]
	list := m.byPosition[pos]
	count := len(list)

	for i := 0; i < count-1; i++ {
		team1 := list[i]
		if m.used[team1.cand.RegistrationID] {
			continue
		}
		m.used[team1.cand.RegistrationID] = true

		for j := i + 1; j < count; j++ {
			team2 := list[j]
			if m.used[team2.cand.RegistrationID] {
				continue
			}
			m.used[team2.cand.RegistrationID] = true

			// 同一对候选人尝试两种 1队/2队 归属，避免“列表靠前者总去 1 队”。
			m.picked[index] = [2]pick{team1, team2}
			m.dfs(index+1, team1Power+team1.power, team2Power+team2.power)
			if !m.exhausted {
				m.picked[index] = [2]pick{team2, team1}
				m.dfs(index+1, team1Power+team2.power, team2Power+team1.power)
			}

			delete(m.used, team2.cand.RegistrationID)

			if m.exhausted {
				delete(m.used, team1.cand.RegistrationID)
				return
			}
		}
		delete(m.used, team1.cand.RegistrationID)
	}
}

func (m *Matcher) record(team1Power, team2Power int) {
	team1 := make([]Slot, 0, len(rank.Positions))
	team2 := make([]Slot, 0, len(rank.Positions))
	mainCount := 0

	for index, pos := range rank.Positions {
		a, b := m.picked[index][0], m.picked[index][1]
		team1 = append(team1, m.slot(a, pos))
		team2 = append(team2, m.slot(b, pos))
		if a.cand.MainPosition != nil && *a.cand.MainPosition == pos {
			mainCount++
		}
		if b.cand.MainPosition != nil && *b.cand.MainPosition == pos {
			mainCount++
		}
	}

	// 规范化 1队/2队，避免“整体交换队伍”的重复方案。
	team1IDs := slotRegistrationIDs(team1)
	team2IDs := slotRegistrationIDs(team2)
	swap := team1Power > team2Power ||
		(team1Power == team2Power && lessInt64Slice(team2IDs, team1IDs))
	if swap {
		team1, team2 = team2, team1
		team1Power, team2Power = team2Power, team1Power
	}

	key := planKey(team1, team2, team1Power, team2Power, mainCount)
	if m.seen[key] {
		return
	}
	m.seen[key] = true

	m.best = append(m.best, Plan{
		Team1:      team1,
		Team2:      team2,
		Team1Power: team1Power,
		Team2Power: team2Power,
		MainCount:  mainCount,
	})
	sort.SliceStable(m.best, func(i, j int) bool { return planLess(m.best[i], m.best[j]) })
	if len(m.best) > m.maxPlans {
		m.best = m.best[:m.maxPlans]
	}
}

func (m *Matcher) slot(p pick, pos rank.Position) Slot {
	c := p.cand
	profile := c.RankFor(pos)
	kind := "sub"
	if c.MainPosition != nil && *c.MainPosition == pos {
		kind = "main"
	}
	var subLabel *string
	if c.SubRank != nil {
		label := c.SubRank.Label()
		subLabel = &label
	}
	return Slot{
		RegistrationID:   c.RegistrationID,
		MemberID:         c.MemberID,
		Name:             c.Name,
		Position:         string(pos),
		PositionLabel:    pos.Label(),
		Power:            p.power,
		RankLabel:        profile.Label(),
		Avatar:           c.Avatar,
		MainRankLabel:    c.MainRank.Label(),
		SubRankLabel:     subLabel,
		PositionRankKind: kind,
		Rank:             RankPayloadOf(profile),
	}
}

// planLess 对应旧实现：主选人数降序 → 战力差升序 → 1队战力升序。
func planLess(a, b Plan) bool {
	if a.MainCount != b.MainCount {
		return a.MainCount > b.MainCount
	}
	if a.Diff() != b.Diff() {
		return a.Diff() < b.Diff()
	}
	return a.Team1Power < b.Team1Power
}

func slotRegistrationIDs(slots []Slot) []int64 {
	ids := make([]int64, 0, len(slots))
	for _, s := range slots {
		ids = append(ids, s.RegistrationID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func lessInt64Slice(a, b []int64) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

func planKey(team1, team2 []Slot, p1, p2, mainCount int) string {
	ids := make([]int64, 0, len(team1)+len(team2))
	for _, s := range team1 {
		ids = append(ids, s.RegistrationID)
	}
	ids = append(ids, -1)
	for _, s := range team2 {
		ids = append(ids, s.RegistrationID)
	}
	return fmt.Sprintf("%v|%d|%d|%d", ids, p1, p2, mainCount)
}
