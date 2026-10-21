package service

import (
	"context"
	"sort"

	"github.com/chyoumu114/lol-civil-war/internal/store"
)

// rankingMetrics 是排行榜支持的指标（rating 单独处理）。
var rankingMetrics = []string{"rating", "kills", "deaths", "assists", "cs", "gold", "damage", "taken", "vision"}

// Ranking 返回排行榜；未知指标回退到击杀。
func (a *App) Ranking(ctx context.Context, group store.Group, metric string) (RankingView, error) {
	if metric == "" {
		metric = "kills"
	}
	if metric == "all" {
		return a.allRanking(ctx, group)
	}
	if !containsString(rankingMetrics, metric) {
		metric = "kills"
	}
	if metric == "rating" {
		return a.ratingRanking(ctx, group)
	}

	rows, err := a.St.StatsRows(ctx, group.ID)
	if err != nil {
		return RankingView{}, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return metricValue(rows[i], metric) > metricValue(rows[j], metric)
	})

	items := make([]RankingStatItem, 0, len(rows))
	for index, row := range rows {
		item := statItem(row)
		item.Rank = index + 1
		items = append(items, item)
	}
	return RankingView{Metric: metric, Metrics: rankingMetrics, Items: items}, nil
}

// allRanking 合并统计与评分，供前端单表排序。
func (a *App) allRanking(ctx context.Context, group store.Group) (RankingView, error) {
	rows, err := a.St.StatsRows(ctx, group.ID)
	if err != nil {
		return RankingView{}, err
	}
	ratingRows, err := a.St.RatingRows(ctx, group.ID)
	if err != nil {
		return RankingView{}, err
	}

	statsByMember := make(map[int64]store.PlayerStat, len(rows))
	order := make([]int64, 0, len(rows)+len(ratingRows))
	for _, row := range rows {
		statsByMember[row.MemberID] = row
		order = append(order, row.MemberID)
	}
	ratingsByMember := make(map[int64]store.MemberRating, len(ratingRows))
	for _, row := range ratingRows {
		ratingsByMember[row.MemberID] = row
		if _, exists := statsByMember[row.MemberID]; !exists {
			order = append(order, row.MemberID)
		}
	}

	items := make([]RankingAllItem, 0, len(order))
	seen := map[int64]bool{}
	for _, memberID := range order {
		if seen[memberID] {
			continue
		}
		seen[memberID] = true

		item := RankingAllItem{}
		if stat, ok := statsByMember[memberID]; ok {
			item.RankingStatItem = statItem(stat)
		} else if rating, ok := ratingsByMember[memberID]; ok {
			item.MemberID = memberID
			item.Nickname = rating.Nickname
			item.Avatar = rating.Avatar
			item.QQNumber = maskQQ(rating.QQNumber)
		}
		if rating, ok := ratingsByMember[memberID]; ok {
			item.Rating = round2(rating.Rating)
			item.RatingCount = rating.RatingCount
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Rating != items[j].Rating {
			return items[i].Rating > items[j].Rating
		}
		return items[i].KDA > items[j].KDA
	})
	for index := range items {
		items[index].Rank = index + 1
	}
	return RankingView{Metric: "all", Metrics: rankingMetrics, Items: items}, nil
}

// ratingRanking 返回评分榜（字段集与统计榜不同）。
func (a *App) ratingRanking(ctx context.Context, group store.Group) (RankingView, error) {
	rows, err := a.St.RatingRows(ctx, group.ID)
	if err != nil {
		return RankingView{}, err
	}
	items := make([]RankingRatingItem, 0, len(rows))
	for index, row := range rows {
		items = append(items, RankingRatingItem{
			Rank:        index + 1,
			MemberID:    row.MemberID,
			Nickname:    row.Nickname,
			Avatar:      row.Avatar,
			Rating:      round2(row.Rating),
			RatingCount: row.RatingCount,
		})
	}
	return RankingView{Metric: "rating", Metrics: rankingMetrics, Items: items}, nil
}

// PlayerAnalysis 返回个人分析。
func (a *App) PlayerAnalysis(ctx context.Context, group store.Group, memberID int64) (PlayerAnalysisView, error) {
	member, err := a.St.MemberByID(ctx, group.ID, memberID)
	if err != nil {
		if IsNotFound(err) {
			return PlayerAnalysisView{}, ErrNotFound("找不到该成员")
		}
		return PlayerAnalysisView{}, err
	}

	games, err := a.St.PlayerGames(ctx, group.ID, memberID)
	if err != nil {
		return PlayerAnalysisView{}, err
	}

	summary := PlayerSummaryView{}
	for _, game := range games {
		summary.Games++
		if game.Win {
			summary.Wins++
		}
		summary.Kills += game.Kills
		summary.Deaths += game.Deaths
		summary.Assists += game.Assists
		summary.CS += game.CS
		summary.Gold += game.Gold
		summary.Damage += game.Damage
		if game.Kills > summary.BestKills {
			summary.BestKills = game.Kills
		}
		if game.Damage > summary.BestDamage {
			summary.BestDamage = game.Damage
		}
	}
	summary.Losses = summary.Games - summary.Wins
	if summary.Games > 0 {
		gamesCount := float64(summary.Games)
		summary.WinRate = round1(float64(summary.Wins) / gamesCount * 100)
		summary.AvgKills = round1(float64(summary.Kills) / gamesCount)
		summary.AvgDeaths = round1(float64(summary.Deaths) / gamesCount)
		summary.AvgAssists = round1(float64(summary.Assists) / gamesCount)
		summary.AvgCS = round1(float64(summary.CS) / gamesCount)
		summary.AvgGold = round1(float64(summary.Gold) / gamesCount)
		summary.AvgDamage = round1(float64(summary.Damage) / gamesCount)
	}

	teammates, err := a.teammates(ctx, games, memberID)
	if err != nil {
		return PlayerAnalysisView{}, err
	}

	recent := make([]RecentGameView, 0, 10)
	for index, game := range games {
		if index >= 10 {
			break
		}
		recent = append(recent, RecentGameView{
			MatchID:  game.MatchID,
			EventID:  game.EventID,
			GameNo:   game.GameNo,
			Region:   game.Region,
			BestOf:   game.BestOf,
			Champion: game.Champion,
			Position: game.Position,
			Win:      game.Win,
			Kills:    game.Kills,
			Deaths:   game.Deaths,
			Assists:  game.Assists,
			CS:       game.CS,
			Gold:     game.Gold,
			Damage:   game.Damage,
		})
	}

	view := PlayerAnalysisView{
		Summary:   summary,
		Teammates: teammates,
		Recent:    recent,
	}
	view.Member.ID = member.ID
	view.Member.Nickname = member.Nickname
	view.Member.QQNumber = maskQQ(member.QQNumber)
	view.Member.Positions = nonNil(member.Positions)
	return view, nil
}

// teammates 统计同队胜率（同队场次 >= 2），返回最佳/最差各 5 人。
func (a *App) teammates(ctx context.Context, games []store.PlayerGame, memberID int64) (TeammatesView, error) {
	view := TeammatesView{Best: []TeammateView{}, Worst: []TeammateView{}}
	if len(games) == 0 {
		return view, nil
	}

	matchIDs := make([]int64, 0, len(games))
	mySide := make(map[int64]string, len(games))
	for _, game := range games {
		matchIDs = append(matchIDs, game.MatchID)
		mySide[game.MatchID] = game.Side
	}

	rows, err := a.St.TeammateRows(ctx, matchIDs, memberID)
	if err != nil {
		return view, err
	}

	type stat struct {
		nickname string
		qqNumber *string
		games    int
		wins     int
	}
	stats := map[int64]*stat{}
	for _, row := range rows {
		if mySide[row.MatchID] != row.Side {
			continue
		}
		entry, ok := stats[row.MemberID]
		if !ok {
			entry = &stat{nickname: row.Nickname, qqNumber: row.QQNumber}
			stats[row.MemberID] = entry
		}
		entry.games++
		if row.Win {
			entry.wins++
		}
	}

	items := make([]TeammateView, 0, len(stats))
	for id, entry := range stats {
		if entry.games < 2 {
			continue
		}
		items = append(items, TeammateView{
			MemberID: id,
			Nickname: entry.nickname,
			QQNumber: maskQQ(entry.qqNumber),
			Games:    entry.games,
			Wins:     entry.wins,
			WinRate:  round1(float64(entry.wins) / float64(entry.games) * 100),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].WinRate != items[j].WinRate {
			return items[i].WinRate > items[j].WinRate
		}
		return items[i].Games > items[j].Games
	})

	view.Best = take(items, 5)
	reversed := make([]TeammateView, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		reversed = append(reversed, items[i])
	}
	view.Worst = take(reversed, 5)
	return view, nil
}

func statItem(row store.PlayerStat) RankingStatItem {
	games := row.Games
	kda := 0.0
	if row.Deaths > 0 {
		kda = round1(float64(row.Kills+row.Assists) / float64(row.Deaths))
	} else {
		kda = float64(row.Kills + row.Assists)
	}
	item := RankingStatItem{
		MemberID:    row.MemberID,
		Nickname:    row.Nickname,
		Avatar:      row.Avatar,
		QQNumber:    maskQQ(row.QQNumber),
		Games:       games,
		Wins:        row.Wins,
		Losses:      games - row.Wins,
		Kills:       row.Kills,
		Deaths:      row.Deaths,
		Assists:     row.Assists,
		CS:          row.CS,
		Gold:        row.Gold,
		Damage:      row.Damage,
		DamageTaken: row.DamageTaken,
		VisionScore: row.VisionScore,
		KDA:         kda,
	}
	if games > 0 {
		count := float64(games)
		item.WinRate = round1(float64(row.Wins) / count * 100)
		item.AvgKills = round1(float64(row.Kills) / count)
		item.AvgDeaths = round1(float64(row.Deaths) / count)
		item.AvgAssists = round1(float64(row.Assists) / count)
		item.AvgCS = round1(float64(row.CS) / count)
		item.AvgGold = round1(float64(row.Gold) / count)
		item.AvgDamage = round1(float64(row.Damage) / count)
		item.AvgDamageTaken = round1(float64(row.DamageTaken) / count)
		item.AvgVisionScore = round1(float64(row.VisionScore) / count)
	}
	return item
}

func metricValue(row store.PlayerStat, metric string) float64 {
	switch metric {
	case "kills":
		return float64(row.Kills)
	case "deaths":
		return float64(row.Deaths)
	case "assists":
		return float64(row.Assists)
	case "cs":
		return float64(row.CS)
	case "gold":
		return float64(row.Gold)
	case "damage":
		return float64(row.Damage)
	case "taken":
		return float64(row.DamageTaken)
	case "vision":
		return float64(row.VisionScore)
	default:
		return float64(row.Kills)
	}
}

func take[T any](items []T, n int) []T {
	if len(items) <= n {
		out := make([]T, len(items))
		copy(out, items)
		return out
	}
	out := make([]T, n)
	copy(out, items[:n])
	return out
}

func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
