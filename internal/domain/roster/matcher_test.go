package roster

import (
	"strings"
	"testing"

	"github.com/chyoumu114/lol-civil-war/internal/domain/rank"
)

func mustProfile(t *testing.T, tier rank.Tier, division *int) rank.Profile {
	t.Helper()
	profile, err := rank.NewProfile("艾欧尼亚", tier, division, 0, 100)
	if err != nil {
		t.Fatalf("构造段位失败: %v", err)
	}
	return profile
}

// buildCandidates 每个位置各 2 名主选玩家，补位为下一个位置。
func buildCandidates(t *testing.T, perPosition int) []Candidate {
	t.Helper()
	division := 3
	mainRank := mustProfile(t, rank.TierDiamond, &division)
	subDivision := 2
	subRank := mustProfile(t, rank.TierEmerald, &subDivision)

	candidates := make([]Candidate, 0, len(rank.Positions)*perPosition)
	id := int64(1)
	for _, position := range rank.Positions {
		for i := 0; i < perPosition; i++ {
			pos := position
			next := rank.Positions[(len(candidates)/perPosition+1)%len(rank.Positions)]
			candidates = append(candidates, Candidate{
				RegistrationID: id,
				MemberID:       id,
				Name:           "玩家" + itoa(id),
				MainPosition:   &pos,
				SubPositions:   []rank.Position{next},
				MainRank:       mainRank,
				SubRank:        &subRank,
			})
			id++
		}
	}
	return candidates
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func TestMatcherBuildsBalancedPlans(t *testing.T) {
	candidates := buildCandidates(t, 2)
	matcher := NewMatcher(rank.NewPowerCalculator(100), 5, 1_000_000)

	result, err := matcher.Match(candidates)
	if err != nil {
		t.Fatalf("匹配失败: %v", err)
	}
	if len(result.Plans) == 0 {
		t.Fatal("没有返回方案")
	}
	if result.CandidateCount != 10 {
		t.Fatalf("候选人数 = %d, want 10", result.CandidateCount)
	}
	if len(result.PositionCounts) != 5 {
		t.Fatalf("位置统计 = %v", result.PositionCounts)
	}

	seen := map[int64]bool{}
	for _, plan := range result.Plans {
		if len(plan.Team1) != 5 || len(plan.Team2) != 5 {
			t.Fatalf("队伍人数异常: %d/%d", len(plan.Team1), len(plan.Team2))
		}
		if plan.MainCount != 10 {
			t.Fatalf("主选人数 = %d, want 10", plan.MainCount)
		}
		positions := map[string]bool{}
		for _, slot := range append(append([]Slot{}, plan.Team1...), plan.Team2...) {
			if seen[slot.RegistrationID] {
				t.Fatalf("玩家 %d 在方案内重复", slot.RegistrationID)
			}
			seen[slot.RegistrationID] = true
			positions[slot.Position] = true
			if slot.Power <= 0 {
				t.Fatalf("战力异常: %+v", slot)
			}
		}
		if len(positions) != 5 {
			t.Fatalf("方案没有覆盖 5 个位置: %v", positions)
		}
		seen = map[int64]bool{}
		if plan.Diff() != 0 {
			t.Fatalf("同段位玩家应能配出 0 差值，实际 %d", plan.Diff())
		}
	}

	// 方案应按主选人数降序、差值升序排列
	for i := 1; i < len(result.Plans); i++ {
		prev, cur := result.Plans[i-1], result.Plans[i]
		if prev.MainCount < cur.MainCount {
			t.Fatalf("方案未按主选人数降序: %d < %d", prev.MainCount, cur.MainCount)
		}
		if prev.MainCount == cur.MainCount && prev.Diff() > cur.Diff() {
			t.Fatalf("方案未按差值升序: %d > %d", prev.Diff(), cur.Diff())
		}
	}
}

func TestMatcherRejectsInsufficientCandidates(t *testing.T) {
	candidates := buildCandidates(t, 2)
	matcher := NewMatcher(rank.NewPowerCalculator(100), 5, 100_000)
	_, err := matcher.Match(candidates[:9])
	if err == nil {
		t.Fatal("9 人不应匹配成功")
	}
	if !strings.Contains(err.Error(), "报名人数不足") {
		t.Fatalf("错误信息不符: %v", err)
	}
}

func TestMatcherRejectsMissingPosition(t *testing.T) {
	candidates := buildCandidates(t, 2)
	// 去掉所有能打辅助的玩家，再用额外的主选玩家补足 10 人
	filtered := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.CanPlay(rank.PositionUtility) {
			continue
		}
		filtered = append(filtered, candidate)
	}
	top, jungle := rank.PositionTop, rank.PositionJungle
	for i := 0; i < 4; i++ {
		extra := filtered[0]
		extra.RegistrationID = int64(100 + i)
		extra.MemberID = int64(100 + i)
		extra.MainPosition = &top
		extra.SubPositions = []rank.Position{jungle}
		filtered = append(filtered, extra)
	}
	matcher := NewMatcher(rank.NewPowerCalculator(100), 5, 100_000)
	_, err := matcher.Match(filtered)
	if err == nil {
		t.Fatal("缺辅助时不应匹配成功")
	}
	if !strings.Contains(err.Error(), "辅助") {
		t.Fatalf("错误信息应指出缺少辅助: %v", err)
	}
}

func TestCandidateRankFor(t *testing.T) {
	top, mid := rank.PositionTop, rank.PositionMiddle
	division := 3
	main := mustProfile(t, rank.TierDiamond, &division)
	subDivision := 1
	sub := mustProfile(t, rank.TierGold, &subDivision)
	candidate := Candidate{
		MainPosition: &top,
		SubPositions: []rank.Position{mid},
		MainRank:     main,
		SubRank:      &sub,
	}
	if candidate.RankFor(top).Tier != rank.TierDiamond {
		t.Fatal("主选位置应使用主段位")
	}
	if candidate.RankFor(mid).Tier != rank.TierGold {
		t.Fatal("补位位置应使用补位段位")
	}
	if !candidate.CanPlay(mid) || !candidate.CanPlay(top) {
		t.Fatal("主选与补位都应可打")
	}
	if candidate.CanPlay(rank.PositionUtility) {
		t.Fatal("未报名位置不应可打")
	}
}
