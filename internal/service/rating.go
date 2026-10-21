package service

import (
	"context"

	"github.com/chyoumu114/lol-civil-war/internal/store"
)

// ratingLabels 是 1-5 星对应的标签。
func ratingLabels() map[string]string {
	return map[string]string{
		"1": "拉了",
		"2": "混子",
		"3": "稳定",
		"4": "优秀",
		"5": "夯",
	}
}

// canRate 判断当前用户能否评分：组织者、报名者或观战者，且比赛已结束。
func (a *App) canRate(ctx context.Context, event store.Event, viewer store.Member) (bool, error) {
	if event.EndedAt == nil {
		return false, nil
	}
	if event.CreatorMemberID == viewer.ID {
		return true, nil
	}
	registrations, err := a.St.RegistrationsByEvent(ctx, event.ID)
	if err != nil {
		return false, err
	}
	for _, registration := range registrations {
		if registration.MemberID == viewer.ID {
			return true, nil
		}
	}
	spectators, err := a.St.SpectatorMemberIDs(ctx, event.ID)
	if err != nil {
		return false, err
	}
	return containsInt64(spectators, viewer.ID), nil
}

// ratingSummary 返回一局内每名出场玩家的评分汇总；无人可评时返回 []。
func (a *App) ratingSummary(ctx context.Context, matchID, viewerID int64) (any, error) {
	players, err := a.St.PlayersByMatch(ctx, matchID)
	if err != nil {
		return nil, err
	}
	ratings, err := a.St.RatingsByMatch(ctx, matchID)
	if err != nil {
		return nil, err
	}

	memberIDs := make([]int64, 0, len(players))
	seen := map[int64]bool{}
	for _, player := range players {
		if player.MemberID == nil || seen[*player.MemberID] {
			continue
		}
		seen[*player.MemberID] = true
		memberIDs = append(memberIDs, *player.MemberID)
	}
	if len(memberIDs) == 0 {
		return []any{}, nil
	}

	out := make(map[string]RatingSummaryView, len(memberIDs))
	for _, memberID := range memberIDs {
		var (
			sum       int
			count     int
			myScore   *int
			raterSeen = map[int64]bool{}
		)
		for _, rating := range ratings {
			if rating.TargetMemberID != memberID {
				continue
			}
			sum += rating.Score
			if !raterSeen[rating.RaterMemberID] {
				raterSeen[rating.RaterMemberID] = true
				count++
			}
			if rating.RaterMemberID == viewerID {
				score := rating.Score
				myScore = &score
			}
		}
		average := 0.0
		if count > 0 {
			average = round2(float64(sum) / float64(count))
		}
		out[itoa64(memberID)] = RatingSummaryView{
			MemberID: memberID,
			Average:  average,
			Count:    count,
			MyScore:  myScore,
		}
	}
	return out, nil
}
