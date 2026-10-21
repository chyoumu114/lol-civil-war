package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/chyoumu114/lol-civil-war/internal/domain/rank"
	"github.com/chyoumu114/lol-civil-war/internal/domain/roster"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

// AutoMatchResult 是自动匹配响应。
type AutoMatchResult struct {
	roster.Result
	EventID int64 `json:"eventId"`
	BestOf  int   `json:"bestOf"`
}

// RosterSlotInput 是套用方案时提交的一个槽位。
type RosterSlotInput struct {
	RegistrationID int64  `json:"registrationId"`
	Position       string `json:"position"`
}

// RosterApplyPayload 是套用方案的请求体。
type RosterApplyPayload struct {
	Team1 *[]RosterSlotInput `json:"team1"`
	Team2 *[]RosterSlotInput `json:"team2"`
}

// AutoMatch 自动匹配，返回前 N 个最平衡方案。
func (a *App) AutoMatch(ctx context.Context, event store.Event, actor store.Member) (AutoMatchResult, error) {
	if event.CreatorMemberID != actor.ID {
		return AutoMatchResult{}, ErrForbidden("只有组织者可以执行该操作")
	}
	if event.EndedAt != nil {
		return AutoMatchResult{}, ErrConflict("比赛已结束，不能再调整阵容")
	}

	registrations, err := a.St.RegistrationsByEvent(ctx, event.ID)
	if err != nil {
		return AutoMatchResult{}, err
	}
	candidates := make([]roster.Candidate, 0, len(registrations))
	for _, registration := range registrations {
		candidate := a.toCandidate(registration)
		if candidate != nil {
			candidates = append(candidates, *candidate)
		}
	}

	matcher := roster.NewMatcher(a.Calc, a.Cfg.RosterMaxPlans, a.Cfg.RosterBudget)
	result, err := matcher.Match(candidates)
	if err != nil {
		var rosterErr *roster.Error
		if ok := asRosterError(err, &rosterErr); ok {
			return AutoMatchResult{}, ErrConflict(rosterErr.Message)
		}
		return AutoMatchResult{}, err
	}
	return AutoMatchResult{Result: result, EventID: event.ID, BestOf: event.BestOf}, nil
}

// ApplyRoster 套用阵容：服务端重新计算战力，不信任前端数值。
func (a *App) ApplyRoster(ctx context.Context, event store.Event, actor store.Member, payload RosterApplyPayload) (json.RawMessage, error) {
	if event.CreatorMemberID != actor.ID {
		return nil, ErrForbidden("只有组织者可以执行该操作")
	}
	if event.EndedAt != nil {
		return nil, ErrConflict("比赛已结束，不能再调整阵容")
	}
	if payload.Team1 == nil || payload.Team2 == nil {
		return nil, ErrInvalid("阵容方案必须包含1队和2队")
	}

	registrations, err := a.St.RegistrationsByEvent(ctx, event.ID)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]store.Registration, len(registrations))
	for _, registration := range registrations {
		byID[registration.ID] = registration
	}

	seenPlayers := map[int64]bool{}
	result := map[string][]roster.Slot{"team1": {}, "team2": {}}
	totals := map[string]int{"team1": 0, "team2": 0}

	for _, side := range []string{"team1", "team2"} {
		slots := payload.Team1
		if side == "team2" {
			slots = payload.Team2
		}
		seenPositions := map[string]bool{}
		for _, slot := range *slots {
			if slot.RegistrationID <= 0 {
				continue // 允许空位，队伍信息可未满人保存
			}
			position, ok := rank.ParsePosition(slot.Position)
			if !ok {
				return nil, ErrInvalid("阵容中的位置不合法")
			}
			if seenPlayers[slot.RegistrationID] {
				return nil, ErrInvalid("同一名玩家不能出现在阵容中两次")
			}
			if seenPositions[string(position)] {
				return nil, ErrInvalid("同一队不能有两个" + position.Label())
			}

			registration, ok := byID[slot.RegistrationID]
			if !ok || registration.IsSpectator {
				return nil, ErrInvalid("阵容中包含未报名本次内战的玩家")
			}
			candidate := a.toCandidate(registration)
			if candidate == nil || !candidate.CanPlay(position) {
				return nil, ErrInvalid(registration.MemberNickname + " 不能打" + position.Label())
			}

			seenPlayers[slot.RegistrationID] = true
			seenPositions[string(position)] = true

			power := a.Calc.Power(candidate.RankFor(position), position)
			totals[side] += power
			result[side] = append(result[side], rosterSlot(candidate, position, power))
		}
	}

	out := RosterPayload{
		Team1:      result["team1"],
		Team2:      result["team2"],
		Team1Power: totals["team1"],
		Team2Power: totals["team2"],
		Diff:       absInt(totals["team1"] - totals["team2"]),
		AppliedAt:  a.iso(a.now()),
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	if err := a.St.UpsertRoster(ctx, event.ID, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// RosterPayload 是阵容方案（1队/2队，不是蓝/红方）。
type RosterPayload struct {
	Team1      []roster.Slot `json:"team1"`
	Team2      []roster.Slot `json:"team2"`
	Team1Power int           `json:"team1Power"`
	Team2Power int           `json:"team2Power"`
	Diff       int           `json:"diff"`
	AppliedAt  string        `json:"appliedAt"`
}

// RosterPayloadFromStore 读取并解析已保存的阵容。
func (a *App) RosterPayloadFromStore(ctx context.Context, eventID int64) (*RosterPayload, error) {
	stored, err := a.St.RosterByEvent(ctx, eventID)
	if err != nil || stored == nil || len(stored.Payload) == 0 {
		return nil, err
	}
	var payload RosterPayload
	if err := json.Unmarshal(stored.Payload, &payload); err != nil {
		return nil, fmt.Errorf("阵容数据损坏: %w", err)
	}
	return &payload, nil
}

// RemoveMemberFromRoster 把某成员从已套用阵容中移除（标记替补时使用）。
func (a *App) RemoveMemberFromRoster(ctx context.Context, eventID, memberID int64) error {
	payload, err := a.RosterPayloadFromStore(ctx, eventID)
	if err != nil || payload == nil {
		return err
	}
	payload.Team1 = filterSlots(payload.Team1, memberID)
	payload.Team2 = filterSlots(payload.Team2, memberID)
	payload.Team1Power = sumPower(payload.Team1)
	payload.Team2Power = sumPower(payload.Team2)
	payload.Diff = absInt(payload.Team1Power - payload.Team2Power)
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return a.St.UpsertRoster(ctx, eventID, raw)
}

// toCandidate 把报名转成匹配候选；资料不足以匹配时返回 nil。
func (a *App) toCandidate(registration store.Registration) *roster.Candidate {
	if registration.IsSpectator || registration.RankTier == "" {
		return nil
	}
	var main *rank.Position
	if registration.MainPosition != nil {
		if position, ok := rank.ParsePosition(*registration.MainPosition); ok {
			main = &position
		}
	}
	subs := make([]rank.Position, 0, len(registration.SubPositions))
	for _, raw := range registration.SubPositions {
		position, ok := rank.ParsePosition(raw)
		if ok {
			subs = append(subs, position)
		}
	}
	if main == nil && len(subs) == 0 {
		return nil
	}

	mainRank := a.registrationRank(registration, false)
	var subRank *rank.Profile
	if registration.SubRankTier != nil && *registration.SubRankTier != "" {
		profile := a.registrationRank(registration, true)
		subRank = &profile
	}

	var note *string
	if registration.Note != nil {
		note = registration.Note
	}
	avatar := registration.MemberAvatar
	return &roster.Candidate{
		RegistrationID: registration.ID,
		MemberID:       registration.MemberID,
		Name:           registration.MemberNickname,
		MainPosition:   main,
		SubPositions:   subs,
		MainRank:       mainRank,
		SubRank:        subRank,
		Note:           note,
		Avatar:         avatar,
	}
}

func rosterSlot(candidate *roster.Candidate, position rank.Position, power int) roster.Slot {
	profile := candidate.RankFor(position)
	kind := "sub"
	if candidate.MainPosition != nil && *candidate.MainPosition == position {
		kind = "main"
	}
	var subLabel *string
	if candidate.SubRank != nil {
		label := candidate.SubRank.Label()
		subLabel = &label
	}
	return roster.Slot{
		RegistrationID:   candidate.RegistrationID,
		MemberID:         candidate.MemberID,
		Name:             candidate.Name,
		Position:         string(position),
		PositionLabel:    position.Label(),
		Power:            power,
		RankLabel:        profile.Label(),
		Avatar:           candidate.Avatar,
		MainRankLabel:    candidate.MainRank.Label(),
		SubRankLabel:     subLabel,
		PositionRankKind: kind,
		Rank:             roster.RankPayloadOf(profile),
	}
}

func rosterPayload(r *store.EventRoster) json.RawMessage {
	if r == nil {
		return nil
	}
	return r.Payload
}

// rosterPayloadIsComplete 判断两队是否各 5 人且位置恰好覆盖五个分路。
func rosterPayloadIsComplete(payload json.RawMessage) bool {
	if len(payload) == 0 {
		return false
	}
	var data struct {
		Team1 []struct {
			Position string `json:"position"`
		} `json:"team1"`
		Team2 []struct {
			Position string `json:"position"`
		} `json:"team2"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return false
	}
	expected := rank.PositionKeys()
	sort.Strings(expected)
	for _, team := range [][]struct {
		Position string `json:"position"`
	}{data.Team1, data.Team2} {
		if len(team) != 5 {
			return false
		}
		positions := make([]string, 0, 5)
		for _, item := range team {
			positions = append(positions, item.Position)
		}
		sort.Strings(positions)
		if strings.Join(positions, ",") != strings.Join(expected, ",") {
			return false
		}
	}
	return true
}

func filterSlots(slots []roster.Slot, memberID int64) []roster.Slot {
	out := make([]roster.Slot, 0, len(slots))
	for _, slot := range slots {
		if slot.MemberID != memberID {
			out = append(out, slot)
		}
	}
	return out
}

func sumPower(slots []roster.Slot) int {
	total := 0
	for _, slot := range slots {
		total += slot.Power
	}
	return total
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func asRosterError(err error, target **roster.Error) bool {
	return errors.As(err, target)
}
