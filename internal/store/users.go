package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// UpsertUser 按 OpenID 创建或更新账号；昵称/头像每次登录同步。
func (s *Store) UpsertUser(ctx context.Context, openid, name string, avatar *string, isDev bool) (User, error) {
	now := time.Now().UTC()
	row := s.conn().QueryRowContext(ctx, `
		INSERT INTO users (qq_openid, name, avatar, is_dev, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(qq_openid) DO UPDATE SET
			name = CASE WHEN excluded.name <> '' THEN excluded.name ELSE users.name END,
			avatar = CASE WHEN excluded.avatar IS NOT NULL AND excluded.avatar <> '' THEN excluded.avatar ELSE users.avatar END,
			is_dev = excluded.is_dev,
			updated_at = excluded.updated_at
		RETURNING id, qq_openid, name, avatar, is_dev, created_at, updated_at`,
		openid, name, nullString(avatar), boolInt(isDev), millis(now), millis(now))
	return scanUser(row)
}

// UserByID 按主键查询账号。
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	row := s.conn().QueryRowContext(ctx,
		`SELECT id, qq_openid, name, avatar, is_dev, created_at, updated_at FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var (
		u       User
		avatar  sql.NullString
		isDev   int
		created int64
		updated int64
	)
	err := row.Scan(&u.ID, &u.QQOpenID, &u.Name, &avatar, &isDev, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("读取账号失败: %w", err)
	}
	u.Avatar = stringPtr(avatar)
	u.IsDev = isDev == 1
	u.CreatedAt = fromMillis(created)
	u.UpdatedAt = fromMillis(updated)
	return u, nil
}
