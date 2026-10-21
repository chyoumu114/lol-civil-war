package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MatchStats 是组织维度的对局统计。
type MatchStats struct {
	Count    int
	BlueWins int
	RedWins  int
}

const eventSelect = `
	SELECT e.id, e.group_id, e.creator_member_id, e.title, e.region, e.best_of, e.has_bp,
	       e.signup_deadline, e.starts_at, e.note, e.ended_at, e.created_at, e.updated_at,
	       gm.nickname, u.avatar, gm.qq_number
	FROM events e
	JOIN group_members gm ON gm.id = e.creator_member_id
	LEFT JOIN users u ON u.id = gm.user_id
`

// CreateEvent 新建组织并返回带 creator 信息的记录。
func (s *Store) CreateEvent(ctx context.Context, e Event) (Event, error) {
	now := time.Now().UTC()
	res, err := s.conn().ExecContext(ctx, `
		INSERT INTO events (group_id, creator_member_id, title, region, best_of, has_bp,
			signup_deadline, starts_at, note, ended_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.GroupID, e.CreatorMemberID, nullString(e.Title), e.Region, e.BestOf, boolInt(e.HasBP),
		millis(e.SignupDeadline), millis(e.StartsAt), nullString(e.Note), nullTime(e.EndedAt),
		millis(now), millis(now))
	if err != nil {
		return Event{}, fmt.Errorf("创建组织失败: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Event{}, err
	}
	return s.EventByGroupAndID(ctx, e.GroupID, id)
}

// UpdateEvent 保存组织基本信息（不允许改创建者/群）。
func (s *Store) UpdateEvent(ctx context.Context, e Event) error {
	res, err := s.conn().ExecContext(ctx, `
		UPDATE events SET title = ?, region = ?, best_of = ?, has_bp = ?,
			signup_deadline = ?, starts_at = ?, note = ?, ended_at = ?, updated_at = ?
		WHERE id = ?`,
		nullString(e.Title), e.Region, e.BestOf, boolInt(e.HasBP),
		millis(e.SignupDeadline), millis(e.StartsAt), nullString(e.Note), nullTime(e.EndedAt),
		millis(time.Now().UTC()), e.ID)
	if err != nil {
		return fmt.Errorf("保存组织失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// EventByGroupAndID 按群 + 组织主键查询。
func (s *Store) EventByGroupAndID(ctx context.Context, groupID, eventID int64) (Event, error) {
	row := s.conn().QueryRowContext(ctx, eventSelect+` WHERE e.group_id = ? AND e.id = ?`, groupID, eventID)
	return scanEvent(row)
}

// EventByID 按主键查询组织（内部使用）。
func (s *Store) EventByID(ctx context.Context, eventID int64) (Event, error) {
	row := s.conn().QueryRowContext(ctx, eventSelect+` WHERE e.id = ?`, eventID)
	return scanEvent(row)
}

// EventListData 一次性取出组织列表、报名数与对局统计，避免 N+1 查询。
func (s *Store) EventListData(ctx context.Context, groupID int64, limit int) ([]Event, map[int64]int, map[int64]MatchStats, error) {
	rows, err := s.conn().QueryContext(ctx,
		eventSelect+` WHERE e.group_id = ? ORDER BY e.created_at DESC, e.id DESC LIMIT ?`, groupID, limit)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("查询组织列表失败: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0, 32)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, nil, nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}

	regCounts := map[int64]int{}
	regRows, err := s.conn().QueryContext(ctx, `
		SELECT r.event_id, count(*) FROM registrations r
		JOIN events e ON e.id = r.event_id
		WHERE e.group_id = ? GROUP BY r.event_id`, groupID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("统计报名人数失败: %w", err)
	}
	defer regRows.Close()
	for regRows.Next() {
		var (
			eventID int64
			count   int
		)
		if err := regRows.Scan(&eventID, &count); err != nil {
			return nil, nil, nil, err
		}
		regCounts[eventID] = count
	}

	stats := map[int64]MatchStats{}
	matchRows, err := s.conn().QueryContext(ctx, `
		SELECT m.event_id, count(*),
		       CAST(coalesce(sum(CASE WHEN m.blue_win = 1 THEN 1 ELSE 0 END), 0) AS SIGNED),
		       CAST(coalesce(sum(CASE WHEN m.blue_win = 0 THEN 1 ELSE 0 END), 0) AS SIGNED)
		FROM matches m JOIN events e ON e.id = m.event_id
		WHERE e.group_id = ? GROUP BY m.event_id`, groupID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("统计对局失败: %w", err)
	}
	defer matchRows.Close()
	for matchRows.Next() {
		var (
			eventID int64
			stat    MatchStats
		)
		if err := matchRows.Scan(&eventID, &stat.Count, &stat.BlueWins, &stat.RedWins); err != nil {
			return nil, nil, nil, err
		}
		stats[eventID] = stat
	}

	return events, regCounts, stats, nil
}

// EndEvent 标记组织已结束，并把其下所有对局置为 submitted。
func (s *Store) EndEvent(ctx context.Context, eventID int64, endedAt time.Time) error {
	return s.Tx(ctx, func(tx *Store) error {
		if _, err := tx.conn().ExecContext(ctx,
			`UPDATE events SET ended_at = ?, updated_at = ? WHERE id = ?`,
			millis(endedAt), millis(endedAt), eventID); err != nil {
			return fmt.Errorf("结束组织失败: %w", err)
		}
		if _, err := tx.conn().ExecContext(ctx,
			`UPDATE matches SET status = 'submitted', submitted_at = ?, updated_at = ? WHERE event_id = ?`,
			millis(endedAt), millis(endedAt), eventID); err != nil {
			return fmt.Errorf("锁定对局失败: %w", err)
		}
		return nil
	})
}

func scanEvent(row interface{ Scan(...any) error }) (Event, error) {
	var (
		e        Event
		title    sql.NullString
		note     sql.NullString
		ended    sql.NullInt64
		created  int64
		updated  int64
		deadline int64
		starts   int64
		nickname sql.NullString
		avatar   sql.NullString
		qqNumber sql.NullString
	)
	err := row.Scan(&e.ID, &e.GroupID, &e.CreatorMemberID, &title, &e.Region, &e.BestOf, &e.HasBP,
		&deadline, &starts, &note, &ended, &created, &updated,
		&nickname, &avatar, &qqNumber)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("读取组织失败: %w", err)
	}
	e.Title = stringPtr(title)
	e.Note = stringPtr(note)
	e.EndedAt = nullTimePtr(ended)
	e.CreatedAt = fromMillis(created)
	e.UpdatedAt = fromMillis(updated)
	e.SignupDeadline = fromMillis(deadline)
	e.StartsAt = fromMillis(starts)
	e.CreatorNickname = nickname.String
	e.CreatorAvatar = stringPtr(avatar)
	e.CreatorQQNumber = stringPtr(qqNumber)
	return e, nil
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return millis(*t)
}
