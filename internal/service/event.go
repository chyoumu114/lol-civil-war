package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/chyoumu114/lol-civil-war/internal/domain/match"
	"github.com/chyoumu114/lol-civil-war/internal/domain/rank"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

// EventPayload 是创建/修改组织的请求体。
type EventPayload struct {
	Title          *string `json:"title"`
	Region         string  `json:"region"`
	BestOf         int     `json:"bestOf"`
	HasBP          bool    `json:"hasBp"`
	SignupDeadline string  `json:"signupDeadline"`
	StartsAt       string  `json:"startsAt"`
	Note           *string `json:"note"`
}

// EventCounts 是列表页各状态数量。
type EventCounts struct {
	Open     int `json:"open"`
	Closed   int `json:"closed"`
	Finished int `json:"finished"`
	All      int `json:"all"`
}

// EventListMeta 是分页信息（与旧接口一致使用 snake_case）。
type EventListMeta struct {
	CurrentPage int         `json:"current_page"`
	PageSize    int         `json:"page_size"`
	LastPage    int         `json:"last_page"`
	Total       int         `json:"total"`
	Counts      EventCounts `json:"counts"`
}

// EventListResult 是组织列表响应。
type EventListResult struct {
	Data []EventListItem `json:"data"`
	Meta EventListMeta   `json:"meta"`
}

// ListEvents 返回组织列表（状态在内存中过滤后分页）。
func (a *App) ListEvents(ctx context.Context, group store.Group, status string, page, pageSize int) (EventListResult, error) {
	events, regCounts, stats, err := a.St.EventListData(ctx, group.ID, 500)
	if err != nil {
		return EventListResult{}, err
	}

	switch status {
	case "open", "closed", "finished":
	default:
		status = "all"
	}

	items := make([]EventListItem, 0, len(events))
	counts := EventCounts{All: len(events)}
	for _, event := range events {
		// 列表页的状态口径与详情一致：已结束优先。
		state := a.eventStatus(event)
		switch state {
		case "open":
			counts.Open++
		case "closed":
			counts.Closed++
		case "finished":
			counts.Finished++
		}
		if status != "all" && state != status {
			continue
		}
		stat := stats[event.ID]
		items = append(items, EventListItem{
			EventView:         a.eventView(event),
			RegistrationCount: regCounts[event.ID],
			MatchCount:        stat.Count,
			Team1Wins:         stat.Team1Wins,
			Team2Wins:         stat.Team2Wins,
		})
	}

	total := len(items)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 1
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	paged := items[start:end]
	if paged == nil {
		paged = []EventListItem{}
	}
	lastPage := (total + pageSize - 1) / pageSize
	if lastPage < 1 {
		lastPage = 1
	}

	return EventListResult{
		Data: paged,
		Meta: EventListMeta{
			CurrentPage: page,
			PageSize:    pageSize,
			LastPage:    lastPage,
			Total:       total,
			Counts:      counts,
		},
	}, nil
}

// CreateEvent 发起组织。
func (a *App) CreateEvent(ctx context.Context, group store.Group, creator store.Member, payload EventPayload) (EventView, error) {
	if !isProfileComplete(creator) {
		return EventView{}, ErrInvalid("请先在个人资料中补全 QQ 号、主选段位和擅长位置")
	}
	normalized, err := a.normalizeEventPayload(payload, true)
	if err != nil {
		return EventView{}, err
	}
	normalized.GroupID = group.ID
	normalized.CreatorMemberID = creator.ID
	event, err := a.St.CreateEvent(ctx, normalized)
	if err != nil {
		return EventView{}, err
	}
	return a.eventView(event), nil
}

// UpdateEvent 修改组织基本信息（仅组织者、结束前）。
func (a *App) UpdateEvent(ctx context.Context, event store.Event, actor store.Member, payload EventPayload) (EventView, error) {
	if event.CreatorMemberID != actor.ID {
		return EventView{}, ErrForbidden("只有组织者可以修改组织信息")
	}
	if event.EndedAt != nil {
		return EventView{}, ErrConflict("比赛已结束，组织信息不可修改")
	}
	normalized, err := a.normalizeEventPayload(payload, false)
	if err != nil {
		return EventView{}, err
	}
	event.Title = normalized.Title
	event.Region = normalized.Region
	event.BestOf = normalized.BestOf
	event.HasBP = normalized.HasBP
	event.SignupDeadline = normalized.SignupDeadline
	event.StartsAt = normalized.StartsAt
	event.Note = normalized.Note
	if err := a.St.UpdateEvent(ctx, event); err != nil {
		return EventView{}, err
	}
	updated, err := a.St.EventByGroupAndID(ctx, event.GroupID, event.ID)
	if err != nil {
		return EventView{}, err
	}
	return a.eventView(updated), nil
}

// EndEvent 结束比赛：锁定全部对局并纳入排行。
func (a *App) EndEvent(ctx context.Context, event store.Event, actor store.Member) (EventDetailView, error) {
	if event.CreatorMemberID != actor.ID {
		return EventDetailView{}, ErrForbidden("只有组织者可以结束比赛")
	}
	if event.EndedAt != nil {
		return EventDetailView{}, ErrConflict("比赛已经结束过了")
	}
	if err := a.St.EndEvent(ctx, event.ID, a.now()); err != nil {
		return EventDetailView{}, err
	}
	updated, err := a.St.EventByGroupAndID(ctx, event.GroupID, event.ID)
	if err != nil {
		return EventDetailView{}, err
	}
	return a.EventDetail(ctx, updated, actor)
}

// EventDetail 返回组织详情（报名、比分、阵容、权限）。
func (a *App) EventDetail(ctx context.Context, event store.Event, viewer store.Member) (EventDetailView, error) {
	registrations, err := a.St.RegistrationsByEvent(ctx, event.ID)
	if err != nil {
		return EventDetailView{}, err
	}
	matches, err := a.St.MatchesByEvent(ctx, event.ID)
	if err != nil {
		return EventDetailView{}, err
	}
	// 比赛记录卡片需要每局双方的英雄头像；BO5 最多 5 局，逐局查询足够轻量。
	playersByMatch := make(map[int64][]store.MatchPlayer, len(matches))
	for _, match := range matches {
		players, playerErr := a.St.PlayersByMatch(ctx, match.ID)
		if playerErr != nil {
			return EventDetailView{}, playerErr
		}
		playersByMatch[match.ID] = players
	}
	roster, err := a.St.RosterByEvent(ctx, event.ID)
	if err != nil {
		return EventDetailView{}, err
	}
	spectatorIDs, err := a.St.SpectatorMemberIDs(ctx, event.ID)
	if err != nil {
		return EventDetailView{}, err
	}

	isOrganizer := event.CreatorMemberID == viewer.ID
	signupOpen := a.now().Before(event.SignupDeadline)
	ended := event.EndedAt != nil

	sorted := make([]store.Registration, 0, len(registrations))
	sorted = append(sorted, registrations...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.IsSpectator != b.IsSpectator {
			return !a.IsSpectator
		}
		aOrg := a.MemberID == event.CreatorMemberID
		bOrg := b.MemberID == event.CreatorMemberID
		if aOrg != bOrg {
			return aOrg
		}
		return a.ID < b.ID
	})

	views := make([]RegistrationView, 0, len(sorted))
	var myRegistration *RegistrationView
	participantCount, substituteCount := 0, 0
	for _, registration := range sorted {
		editable := isOrganizer || (signupOpen && registration.MemberID == viewer.ID)
		view := a.registrationView(registration, event, editable)
		views = append(views, view)
		if registration.IsSpectator {
			substituteCount++
		} else {
			participantCount++
		}
		if registration.MemberID == viewer.ID {
			copyView := view
			copyView.Editable = signupOpen && !ended
			myRegistration = &copyView
		}
	}

	series := a.seriesOf(event, matches, playersByMatch)
	rosterComplete := rosterPayloadIsComplete(rosterPayload(roster))

	var draft *DraftView
	if !ended {
		for _, m := range matches {
			if m.Status == "draft" {
				draft = &DraftView{GameNo: m.GameNo}
				break
			}
		}
	}

	permissions := PermissionsView{
		IsOrganizer:            isOrganizer,
		SignupOpen:             signupOpen,
		IsEnded:                ended,
		CanRegister:            signupOpen && !ended && isProfileComplete(viewer),
		CanManageRegistrations: isOrganizer && !ended,
		CanArrange:             isOrganizer && !ended,
		CanAutoMatch:           isOrganizer && !ended,
		CanSpectate:            !ended,
		CanRecordMatch: isOrganizer && !ended && rosterComplete &&
			series.TeamWinner == nil && len(matches) < event.BestOf,
		CanEnd: isOrganizer && !ended,
	}

	rosterJSON := json.RawMessage(nil)
	if roster != nil && len(roster.Payload) > 0 {
		rosterJSON = roster.Payload
	}

	return EventDetailView{
		Event:            a.eventView(event),
		Permissions:      permissions,
		Registrations:    views,
		ParticipantCount: participantCount,
		SubstituteCount:  substituteCount,
		SpectatorCount:   len(spectatorIDs),
		MySpectator:      containsInt64(spectatorIDs, viewer.ID),
		MyRegistration:   myRegistration,
		Series:           series,
		Roster:           rosterJSON,
		Draft:            draft,
	}, nil
}

// normalizeEventPayload 校验并归一化组织表单。
func (a *App) normalizeEventPayload(payload EventPayload, requireFutureDeadline bool) (store.Event, error) {
	region := strings.TrimSpace(payload.Region)
	if !rank.IsRegion(region) {
		return store.Event{}, ErrInvalid("请选择有效的比赛大区")
	}
	if !match.IsValid(payload.BestOf) {
		return store.Event{}, ErrInvalid("赛制只能是 BO1 / BO3 / BO5")
	}

	deadline, err := a.parseEventTime(payload.SignupDeadline)
	if err != nil {
		return store.Event{}, ErrInvalid("报名截止时间或比赛开始时间格式不正确")
	}
	startsAt, err := a.parseEventTime(payload.StartsAt)
	if err != nil {
		return store.Event{}, ErrInvalid("报名截止时间或比赛开始时间格式不正确")
	}
	if requireFutureDeadline && !deadline.After(a.now()) {
		return store.Event{}, ErrInvalid("报名截止时间必须晚于当前时间")
	}
	if !deadline.Before(startsAt) {
		return store.Event{}, ErrInvalid("比赛开始时间必须晚于报名截止时间")
	}

	title := ""
	if payload.Title != nil {
		title = strings.TrimSpace(*payload.Title)
	}
	if len([]rune(title)) > 60 {
		return store.Event{}, ErrInvalid("标题最多 60 个字符")
	}
	note := ""
	if payload.Note != nil {
		note = strings.TrimSpace(*payload.Note)
	}
	if len([]rune(note)) > 500 {
		return store.Event{}, ErrInvalid("备注最多 500 个字符")
	}

	event := store.Event{
		Region:    region,
		BestOf:    payload.BestOf,
		HasBP:     payload.HasBP,
		StartsAt:  startsAt,
		CreatedAt: a.now(),
		UpdatedAt: a.now(),
	}
	event.SignupDeadline = deadline
	if title != "" {
		event.Title = &title
	}
	if note != "" {
		event.Note = &note
	}
	return event, nil
}

// parseEventTime 兼容前端 ISO8601 与常见手工格式；空串按“当前时间”处理。
func (a *App) parseEventTime(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return a.now(), nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, a.Loc); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, errInvalidTime
}

var errInvalidTime = &timeParseError{}

type timeParseError struct{}

func (*timeParseError) Error() string { return "invalid time" }

func containsInt64(list []int64, target int64) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}
