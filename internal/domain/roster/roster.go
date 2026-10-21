// Package roster 实现自动战力平衡匹配。
//
// 规则：从报名者中选出 10 人，两队各 5 人，每队 上/野/中/下/辅 各一名；
// 每名玩家只能出现一次，且只能打自己主选或补位的位置。优先让玩家打主选位置，
// 主选人数相同时再最小化两队总战力差，输出前 N 个方案。
package roster

import (
	"github.com/chyoumu114/lol-civil-war/internal/domain/rank"
)

// Candidate 是一名可参与匹配的报名者。
type Candidate struct {
	RegistrationID int64
	MemberID       int64
	Name           string
	MainPosition   *rank.Position
	SubPositions   []rank.Position
	MainRank       rank.Profile
	SubRank        *rank.Profile
	Note           *string
	Avatar         *string
}

// CanPlay 报告该玩家能否打某个位置（主选 + 补位）。
func (c Candidate) CanPlay(position rank.Position) bool {
	if c.MainPosition != nil && *c.MainPosition == position {
		return true
	}
	for _, sub := range c.SubPositions {
		if sub == position {
			return true
		}
	}
	return false
}

// RankFor 返回打该位置时使用的段位：主选优先，否则用补位段位（未填回退主段位）。
func (c Candidate) RankFor(position rank.Position) rank.Profile {
	if c.MainPosition != nil && *c.MainPosition == position {
		return c.MainRank
	}
	if c.SubRank != nil {
		return *c.SubRank
	}
	return c.MainRank
}

// RankPayload 是段位的 JSON 结构（不含 baseScore）。
type RankPayload struct {
	Region    string `json:"region"`
	Tier      string `json:"tier"`
	TierLabel string `json:"tierLabel"`
	Division  *int   `json:"division"`
	LP        int    `json:"lp"`
	Label     string `json:"label"`
}

// RankPayloadOf 把段位值对象转成 JSON 结构。
func RankPayloadOf(profile rank.Profile) RankPayload {
	return RankPayload{
		Region:    profile.Region,
		Tier:      string(profile.Tier),
		TierLabel: profile.Tier.Label(),
		Division:  profile.Division,
		LP:        profile.LP,
		Label:     profile.Label(),
	}
}

// Slot 是阵容里的一个位置槽。
type Slot struct {
	RegistrationID   int64       `json:"registrationId"`
	MemberID         int64       `json:"memberId"`
	Name             string      `json:"name"`
	Position         string      `json:"position"`
	PositionLabel    string      `json:"positionLabel"`
	Power            int         `json:"power"`
	RankLabel        string      `json:"rankLabel"`
	Avatar           *string     `json:"avatar"`
	MainRankLabel    string      `json:"mainRankLabel"`
	SubRankLabel     *string     `json:"subRankLabel"`
	PositionRankKind string      `json:"positionRankKind"`
	Rank             RankPayload `json:"rank"`
}

// Plan 是一套 5v5 阵容方案。Team1/Team2 表示组织页的 1队/2队，
// 不是比赛里的蓝/红方（蓝红由每局的 team1Side 决定）。
type Plan struct {
	Team1      []Slot `json:"team1"`
	Team2      []Slot `json:"team2"`
	Team1Power int    `json:"team1Power"`
	Team2Power int    `json:"team2Power"`
	MainCount  int    `json:"mainCount"`
}

// Diff 返回两队战力差的绝对值。
func (p Plan) Diff() int {
	d := p.Team1Power - p.Team2Power
	if d < 0 {
		return -d
	}
	return d
}

// Result 是自动匹配结果。
type Result struct {
	Plans          []Plan         `json:"plans"`
	Exhausted      bool           `json:"exhausted"`
	Nodes          int            `json:"nodes"`
	PositionCounts map[string]int `json:"positionCounts"`
	CandidateCount int            `json:"candidateCount"`
}

// Error 表示无法完成匹配的业务错误。
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }
