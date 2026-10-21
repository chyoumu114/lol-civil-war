package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const registrationSelect = `
	SELECT r.id, r.event_id, r.member_id, r.rank_region, r.rank_tier, r.rank_division, r.rank_lp,
	       r.main_position, r.sub_positions, r.sub_rank_region, r.sub_rank_tier,
	       r.sub_rank_division, r.sub_rank_lp, r.note, r.is_spectator, r.created_at, r.updated_at,
	       gm.nickname, u.avatar, gm.qq_number
	FROM registrations r
	JOIN group_members gm ON gm.id = r.member_id
	LEFT JOIN users u ON u.id = gm.user_id
`

// UpsertRegistration 新增或更新报名，返回带成员信息的最新记录。
func (s *Store) UpsertRegistration(ctx context.Context, r Registration) (Registration, error) {
	now := time.Now().UTC()
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO registrations (event_id, member_id, rank_region, rank_tier, rank_division, rank_lp,
			main_position, sub_positions, sub_rank_region, sub_rank_tier, sub_rank_division, sub_rank_lp,
			note, is_spectator, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(event_id, member_id) DO UPDATE SET
			rank_region = excluded.rank_region,
			rank_tier = excluded.rank_tier,
			rank_division = excluded.rank_division,
			rank_lp = excluded.rank_lp,
			main_position = excluded.main_position,
			sub_positions = excluded.sub_positions,
			sub_rank_region = excluded.sub_rank_region,
			sub_rank_tier = excluded.sub_rank_tier,
			sub_rank_division = excluded.sub_rank_division,
			sub_rank_lp = excluded.sub_rank_lp,
			note = excluded.note,
			is_spectator = excluded.is_spectator,
			updated_at = excluded.updated_at`,
		r.EventID, r.MemberID, r.RankRegion, r.RankTier, nullIntPtr(r.RankDivision), r.RankLP,
		nullString(r.MainPosition), jsonList(r.SubPositions), nullString(r.SubRankRegion),
		nullString(r.SubRankTier), nullIntPtr(r.SubRankDivision), r.SubRankLP,
		nullString(r.Note), boolInt(r.IsSpectator), millis(now), millis(now))
	if err != nil {
		return Registration{}, fmt.Errorf("保存报名失败: %w", err)
	}
	return s.RegistrationByEventMember(ctx, r.EventID, r.MemberID)
}

// RegistrationByEventMember 查询某人在某次组织的报名。
func (s *Store) RegistrationByEventMember(ctx context.Context, eventID, memberID int64) (Registration, error) {
	row := s.conn().QueryRowContext(ctx, registrationSelect+` WHERE r.event_id = ? AND r.member_id = ?`, eventID, memberID)
	return scanRegistration(row)
}

// RegistrationsByEvent 查询某次组织的全部报名（按 id 升序）。
func (s *Store) RegistrationsByEvent(ctx context.Context, eventID int64) ([]Registration, error) {
	rows, err := s.conn().QueryContext(ctx, registrationSelect+` WHERE r.event_id = ? ORDER BY r.id ASC`, eventID)
	if err != nil {
		return nil, fmt.Errorf("查询报名失败: %w", err)
	}
	defer rows.Close()

	out := make([]Registration, 0, 16)
	for rows.Next() {
		r, err := scanRegistration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateRegistrationSpectator 只更新替补标记。
func (s *Store) UpdateRegistrationSpectator(ctx context.Context, eventID, memberID int64, spectator bool) error {
	res, err := s.conn().ExecContext(ctx,
		`UPDATE registrations SET is_spectator = ?, updated_at = ? WHERE event_id = ? AND member_id = ?`,
		boolInt(spectator), millis(time.Now().UTC()), eventID, memberID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteRegistration 取消报名（不存在不报错）。
func (s *Store) DeleteRegistration(ctx context.Context, eventID, memberID int64) error {
	_, err := s.conn().ExecContext(ctx,
		`DELETE FROM registrations WHERE event_id = ? AND member_id = ?`, eventID, memberID)
	return err
}

// RosterByEvent 查询组织套用的阵容；不存在返回 nil。
func (s *Store) RosterByEvent(ctx context.Context, eventID int64) (*EventRoster, error) {
	row := s.conn().QueryRowContext(ctx,
		`SELECT id, event_id, payload, created_at, updated_at FROM event_rosters WHERE event_id = ?`, eventID)
	var (
		r       EventRoster
		payload string
		created int64
		updated int64
	)
	err := row.Scan(&r.ID, &r.EventID, &payload, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取阵容失败: %w", err)
	}
	r.Payload = json.RawMessage(payload)
	r.CreatedAt = fromMillis(created)
	r.UpdatedAt = fromMillis(updated)
	return &r, nil
}

// UpsertRoster 保存组织阵容（每个组织只保留一份）。
func (s *Store) UpsertRoster(ctx context.Context, eventID int64, payload json.RawMessage) error {
	now := time.Now().UTC()
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO event_rosters (event_id, payload, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(event_id) DO UPDATE SET payload = excluded.payload, updated_at = excluded.updated_at`,
		eventID, string(payload), millis(now), millis(now))
	if err != nil {
		return fmt.Errorf("保存阵容失败: %w", err)
	}
	return nil
}

// SpectatorMemberIDs 返回某次组织的观战成员 id 列表。
func (s *Store) SpectatorMemberIDs(ctx context.Context, eventID int64) ([]int64, error) {
	rows, err := s.conn().QueryContext(ctx,
		`SELECT member_id FROM event_spectators WHERE event_id = ? ORDER BY id`, eventID)
	if err != nil {
		return nil, fmt.Errorf("查询观战失败: %w", err)
	}
	defer rows.Close()
	ids := make([]int64, 0, 8)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SetEventSpectator 添加/取消观战（幂等）。
func (s *Store) SetEventSpectator(ctx context.Context, eventID, memberID int64, watching bool) error {
	if watching {
		now := time.Now().UTC()
		_, err := s.conn().ExecContext(ctx, `
			INSERT INTO event_spectators (event_id, member_id, created_at, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(event_id, member_id) DO NOTHING`,
			eventID, memberID, millis(now), millis(now))
		return err
	}
	_, err := s.conn().ExecContext(ctx,
		`DELETE FROM event_spectators WHERE event_id = ? AND member_id = ?`, eventID, memberID)
	return err
}

func scanRegistration(row interface{ Scan(...any) error }) (Registration, error) {
	var (
		r            Registration
		division     sql.NullInt64
		mainPos      sql.NullString
		subPositions sql.NullString
		subRegion    sql.NullString
		subTier      sql.NullString
		subDivision  sql.NullInt64
		note         sql.NullString
		spectator    int
		created      int64
		updated      int64
		nickname     sql.NullString
		avatar       sql.NullString
		qqNumber     sql.NullString
	)
	err := row.Scan(&r.ID, &r.EventID, &r.MemberID, &r.RankRegion, &r.RankTier, &division, &r.RankLP,
		&mainPos, &subPositions, &subRegion, &subTier, &subDivision, &r.SubRankLP, &note, &spectator,
		&created, &updated, &nickname, &avatar, &qqNumber)
	if errors.Is(err, sql.ErrNoRows) {
		return Registration{}, ErrNotFound
	}
	if err != nil {
		return Registration{}, fmt.Errorf("读取报名失败: %w", err)
	}
	r.RankDivision = intPtr(division)
	r.MainPosition = stringPtr(mainPos)
	r.SubPositions = parseJSONList(subPositions)
	r.SubRankRegion = stringPtr(subRegion)
	r.SubRankTier = stringPtr(subTier)
	r.SubRankDivision = intPtr(subDivision)
	r.Note = stringPtr(note)
	r.IsSpectator = spectator == 1
	r.CreatedAt = fromMillis(created)
	r.UpdatedAt = fromMillis(updated)
	r.MemberNickname = nickname.String
	r.MemberAvatar = stringPtr(avatar)
	r.MemberQQNumber = stringPtr(qqNumber)
	return r, nil
}
