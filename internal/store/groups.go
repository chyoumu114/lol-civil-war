package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const memberColumns = `
	gm.id, gm.group_id, gm.user_id, gm.qq_number, gm.nickname,
	gm.rank_region, gm.rank_tier, gm.rank_division, gm.rank_lp, gm.positions,
	gm.created_at, gm.updated_at, u.name, u.avatar
`

// GroupByPublicKey 按公开 key 查询群。
func (s *Store) GroupByPublicKey(ctx context.Context, key string) (Group, error) {
	row := s.conn().QueryRowContext(ctx,
		`SELECT id, qq_group_number, public_key, name, created_at, updated_at FROM groups WHERE public_key = ?`, key)
	return scanGroup(row)
}

// GroupByNumber 按 QQ 群号查询群。
func (s *Store) GroupByNumber(ctx context.Context, number string) (Group, error) {
	row := s.conn().QueryRowContext(ctx,
		`SELECT id, qq_group_number, public_key, name, created_at, updated_at FROM groups WHERE qq_group_number = ?`, number)
	return scanGroup(row)
}

// GroupByID 按主键查询群。
func (s *Store) GroupByID(ctx context.Context, id int64) (Group, error) {
	row := s.conn().QueryRowContext(ctx,
		`SELECT id, qq_group_number, public_key, name, created_at, updated_at FROM groups WHERE id = ?`, id)
	return scanGroup(row)
}

// EnsureGroup 按群号取群，不存在则创建（白名单校验在 service 层）。
func (s *Store) EnsureGroup(ctx context.Context, number, publicKey string) (Group, error) {
	group, err := s.GroupByNumber(ctx, number)
	if err == nil {
		return group, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Group{}, err
	}

	now := time.Now().UTC()
	_, err = s.conn().ExecContext(ctx,
		`INSERT INTO groups (qq_group_number, public_key, name, created_at, updated_at)
		 VALUES (?, ?, NULL, ?, ?)
		 ON CONFLICT(qq_group_number) DO NOTHING`,
		number, publicKey, millis(now), millis(now))
	if err != nil {
		return Group{}, fmt.Errorf("创建群失败: %w", err)
	}
	return s.GroupByNumber(ctx, number)
}

func scanGroup(row interface{ Scan(...any) error }) (Group, error) {
	var (
		g    Group
		name sql.NullString
		c, u int64
	)
	err := row.Scan(&g.ID, &g.QQGroupNumber, &g.PublicKey, &name, &c, &u)
	if errors.Is(err, sql.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	if err != nil {
		return Group{}, fmt.Errorf("读取群失败: %w", err)
	}
	g.Name = stringPtr(name)
	g.CreatedAt = fromMillis(c)
	g.UpdatedAt = fromMillis(u)
	return g, nil
}

// EnsureMember 取群内成员资料，不存在则创建占位资料。
func (s *Store) EnsureMember(ctx context.Context, groupID, userID int64, fallbackNickname string) (Member, error) {
	member, err := s.MemberByGroupAndUser(ctx, groupID, userID)
	if err == nil {
		if member.Nickname == "" {
			member.Nickname = fallbackNickname
			if _, err := s.conn().ExecContext(ctx,
				`UPDATE group_members SET nickname = ?, updated_at = ? WHERE id = ?`,
				member.Nickname, millis(time.Now().UTC()), member.ID); err != nil {
				return Member{}, fmt.Errorf("更新成员昵称失败: %w", err)
			}
		}
		return member, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Member{}, err
	}

	if fallbackNickname == "" {
		fallbackNickname = "QQ 用户"
	}
	now := time.Now().UTC()
	_, err = s.conn().ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, nickname, positions, created_at, updated_at)
		 VALUES (?, ?, ?, '[]', ?, ?)
		 ON CONFLICT(group_id, user_id) DO NOTHING`,
		groupID, userID, fallbackNickname, millis(now), millis(now))
	if err != nil {
		return Member{}, fmt.Errorf("创建成员资料失败: %w", err)
	}
	return s.MemberByGroupAndUser(ctx, groupID, userID)
}

// MemberByGroupAndUser 按群 + 用户查询成员资料。
func (s *Store) MemberByGroupAndUser(ctx context.Context, groupID, userID int64) (Member, error) {
	row := s.conn().QueryRowContext(ctx, `
		SELECT `+memberColumns+`
		FROM group_members gm LEFT JOIN users u ON u.id = gm.user_id
		WHERE gm.group_id = ? AND gm.user_id = ?`, groupID, userID)
	return scanMember(row)
}

// MemberByID 按群 + 成员主键查询成员资料。
func (s *Store) MemberByID(ctx context.Context, groupID, memberID int64) (Member, error) {
	row := s.conn().QueryRowContext(ctx, `
		SELECT `+memberColumns+`
		FROM group_members gm LEFT JOIN users u ON u.id = gm.user_id
		WHERE gm.group_id = ? AND gm.id = ?`, groupID, memberID)
	return scanMember(row)
}

// MemberByIDAny 按成员主键查询（不再限定群，仅用于内部关联）。
func (s *Store) MemberByIDAny(ctx context.Context, memberID int64) (Member, error) {
	row := s.conn().QueryRowContext(ctx, `
		SELECT `+memberColumns+`
		FROM group_members gm LEFT JOIN users u ON u.id = gm.user_id
		WHERE gm.id = ?`, memberID)
	return scanMember(row)
}

// UpdateMemberProfile 全量保存群内资料。
func (s *Store) UpdateMemberProfile(ctx context.Context, m Member) error {
	_, err := s.conn().ExecContext(ctx, `
		UPDATE group_members SET
			nickname = ?, qq_number = ?, rank_region = ?, rank_tier = ?,
			rank_division = ?, rank_lp = ?, positions = ?, updated_at = ?
		WHERE id = ?`,
		m.Nickname, nullString(m.QQNumber), nullString(m.RankRegion), nullString(m.RankTier),
		nullIntPtr(m.RankDivision), m.RankLP, jsonList(m.Positions), millis(time.Now().UTC()), m.ID)
	if err != nil {
		return fmt.Errorf("保存成员资料失败: %w", err)
	}
	return nil
}

// SetMemberQQNumber 首次设置 QQ 号。
func (s *Store) SetMemberQQNumber(ctx context.Context, memberID int64, qqNumber string) error {
	_, err := s.conn().ExecContext(ctx,
		`UPDATE group_members SET qq_number = ?, updated_at = ? WHERE id = ?`,
		qqNumber, millis(time.Now().UTC()), memberID)
	return err
}

func scanMember(row interface{ Scan(...any) error }) (Member, error) {
	var (
		m         Member
		qqNumber  sql.NullString
		region    sql.NullString
		tier      sql.NullString
		division  sql.NullInt64
		positions sql.NullString
		created   int64
		updated   int64
		userName  sql.NullString
		avatar    sql.NullString
	)
	err := row.Scan(&m.ID, &m.GroupID, &m.UserID, &qqNumber, &m.Nickname,
		&region, &tier, &division, &m.RankLP, &positions,
		&created, &updated, &userName, &avatar)
	if errors.Is(err, sql.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	if err != nil {
		return Member{}, fmt.Errorf("读取成员资料失败: %w", err)
	}
	m.QQNumber = stringPtr(qqNumber)
	m.RankRegion = stringPtr(region)
	m.RankTier = stringPtr(tier)
	m.RankDivision = intPtr(division)
	m.Positions = parseJSONList(positions)
	m.CreatedAt = fromMillis(created)
	m.UpdatedAt = fromMillis(updated)
	m.UserName = userName.String
	m.Avatar = stringPtr(avatar)
	return m, nil
}

func nullIntPtr(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}
