package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// PlayerStat 是排行榜的一行统计（按成员聚合）。
type PlayerStat struct {
	MemberID    int64
	Nickname    string
	Avatar      *string
	QQNumber    *string
	Games       int
	Wins        int
	Kills       int
	Deaths      int
	Assists     int
	CS          int
	Gold        int
	Damage      int
	DamageTaken int
	VisionScore int
}

// MemberRating 是评分汇总。
type MemberRating struct {
	MemberID    int64
	Nickname    string
	Avatar      *string
	QQNumber    *string
	Rating      float64
	RatingCount int
}

// PlayerGame 是个人分析中的一场对局。
type PlayerGame struct {
	MatchID  int64
	EventID  int64
	GameNo   int
	Region   string
	BestOf   int
	Side     string
	Position string
	Champion string
	Win      bool
	Kills    int
	Deaths   int
	Assists  int
	CS       int
	Gold     int
	Damage   int
}

// TeammateRow 是同队队友的一条出场记录。
type TeammateRow struct {
	MatchID  int64
	Side     string
	MemberID int64
	Win      bool
	Nickname string
	QQNumber *string
}

// StatsRows 返回群内已结束组织、已提交对局的成员统计（上限 100）。
func (s *Store) StatsRows(ctx context.Context, groupID int64) ([]PlayerStat, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT p.member_id, gm.nickname, u.avatar, gm.qq_number,
		       count(*),
		       CAST(coalesce(sum(CASE WHEN p.win = 1 THEN 1 ELSE 0 END), 0) AS SIGNED),
		       CAST(coalesce(sum(p.kills), 0) AS SIGNED), CAST(coalesce(sum(p.deaths), 0) AS SIGNED),
		       CAST(coalesce(sum(p.assists), 0) AS SIGNED),
		       CAST(coalesce(sum(p.cs), 0) AS SIGNED), CAST(coalesce(sum(p.gold), 0) AS SIGNED),
		       CAST(coalesce(sum(p.damage_to_champions), 0) AS SIGNED),
		       CAST(coalesce(sum(p.damage_taken), 0) AS SIGNED),
		       CAST(coalesce(sum(p.vision_score), 0) AS SIGNED)
		FROM match_players p
		JOIN matches m ON m.id = p.match_id
		JOIN events e ON e.id = m.event_id
		JOIN group_members gm ON gm.id = p.member_id
		LEFT JOIN users u ON u.id = gm.user_id
		WHERE e.group_id = ? AND e.ended_at IS NOT NULL AND m.status = 'submitted'
		  AND p.member_id IS NOT NULL
		GROUP BY p.member_id, gm.nickname, u.avatar, gm.qq_number
		LIMIT 100`, groupID)
	if err != nil {
		return nil, fmt.Errorf("统计排行榜失败: %w", err)
	}
	defer rows.Close()

	out := make([]PlayerStat, 0, 32)
	for rows.Next() {
		var st PlayerStat
		var avatar, qqNumber sql.NullString
		if err := rows.Scan(&st.MemberID, &st.Nickname, &avatar, &qqNumber, &st.Games, &st.Wins,
			&st.Kills, &st.Deaths, &st.Assists, &st.CS, &st.Gold, &st.Damage, &st.DamageTaken,
			&st.VisionScore); err != nil {
			return nil, fmt.Errorf("读取排行榜失败: %w", err)
		}
		st.Avatar = stringPtr(avatar)
		st.QQNumber = stringPtr(qqNumber)
		out = append(out, st)
	}
	return out, rows.Err()
}

// RatingRows 返回群内成员的平均评分（上限 100）。
func (s *Store) RatingRows(ctx context.Context, groupID int64) ([]MemberRating, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT r.target_member_id, gm.nickname, u.avatar, gm.qq_number,
		       CAST(avg(r.score) AS DOUBLE),
		       count(DISTINCT r.rater_member_id)
		FROM match_player_ratings r
		JOIN matches m ON m.id = r.match_id
		JOIN events e ON e.id = m.event_id
		JOIN group_members gm ON gm.id = r.target_member_id
		LEFT JOIN users u ON u.id = gm.user_id
		WHERE e.group_id = ? AND e.ended_at IS NOT NULL AND m.status = 'submitted'
		GROUP BY r.target_member_id, gm.nickname, u.avatar, gm.qq_number
		ORDER BY avg(r.score) DESC, count(DISTINCT r.rater_member_id) DESC
		LIMIT 100`, groupID)
	if err != nil {
		return nil, fmt.Errorf("统计评分失败: %w", err)
	}
	defer rows.Close()

	out := make([]MemberRating, 0, 32)
	for rows.Next() {
		var mr MemberRating
		var avatar, qqNumber sql.NullString
		if err := rows.Scan(&mr.MemberID, &mr.Nickname, &avatar, &qqNumber, &mr.Rating, &mr.RatingCount); err != nil {
			return nil, fmt.Errorf("读取评分失败: %w", err)
		}
		mr.Avatar = stringPtr(avatar)
		mr.QQNumber = stringPtr(qqNumber)
		out = append(out, mr)
	}
	return out, rows.Err()
}

// PlayerGames 返回某成员的全部已提交对局（match_id 降序）。
func (s *Store) PlayerGames(ctx context.Context, groupID, memberID int64) ([]PlayerGame, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT p.match_id, e.id, m.game_no, e.region, e.best_of, p.side, p.position, p.champion,
		       p.win, p.kills, p.deaths, p.assists, p.cs, p.gold, p.damage_to_champions
		FROM match_players p
		JOIN matches m ON m.id = p.match_id
		JOIN events e ON e.id = m.event_id
		WHERE e.group_id = ? AND e.ended_at IS NOT NULL AND m.status = 'submitted'
		  AND p.member_id = ?
		ORDER BY p.match_id DESC`, groupID, memberID)
	if err != nil {
		return nil, fmt.Errorf("查询个人对局失败: %w", err)
	}
	defer rows.Close()

	out := make([]PlayerGame, 0, 16)
	for rows.Next() {
		var (
			g   PlayerGame
			win int
		)
		if err := rows.Scan(&g.MatchID, &g.EventID, &g.GameNo, &g.Region, &g.BestOf, &g.Side,
			&g.Position, &g.Champion, &win, &g.Kills, &g.Deaths, &g.Assists, &g.CS, &g.Gold,
			&g.Damage); err != nil {
			return nil, fmt.Errorf("读取个人对局失败: %w", err)
		}
		g.Win = win == 1
		out = append(out, g)
	}
	return out, rows.Err()
}

// TeammateRows 返回这些对局里除本人外的所有出场玩家（用于队友分析）。
func (s *Store) TeammateRows(ctx context.Context, matchIDs []int64, memberID int64) ([]TeammateRow, error) {
	if len(matchIDs) == 0 {
		return []TeammateRow{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(matchIDs)), ",")
	args := make([]any, 0, len(matchIDs)+1)
	for _, id := range matchIDs {
		args = append(args, id)
	}
	args = append(args, memberID)

	query := fmt.Sprintf(`
		SELECT p.match_id, p.side, p.member_id, p.win, gm.nickname, gm.qq_number
		FROM match_players p
		JOIN group_members gm ON gm.id = p.member_id
		WHERE p.match_id IN (%s) AND p.member_id IS NOT NULL AND p.member_id <> ?`, placeholders)
	rows, err := s.conn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询队友失败: %w", err)
	}
	defer rows.Close()

	out := make([]TeammateRow, 0, 64)
	for rows.Next() {
		var (
			row      TeammateRow
			win      int
			qqNumber sql.NullString
		)
		if err := rows.Scan(&row.MatchID, &row.Side, &row.MemberID, &win, &row.Nickname, &qqNumber); err != nil {
			return nil, fmt.Errorf("读取队友失败: %w", err)
		}
		row.Win = win == 1
		row.QQNumber = stringPtr(qqNumber)
		out = append(out, row)
	}
	return out, rows.Err()
}
