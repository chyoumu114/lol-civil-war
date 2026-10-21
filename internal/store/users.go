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
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO users (qq_openid, name, avatar, is_dev, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?) AS new
		ON DUPLICATE KEY UPDATE
			name = IF(new.name <> '', new.name, users.name),
			avatar = IF(new.avatar IS NOT NULL AND new.avatar <> '', new.avatar, users.avatar),
			is_dev = new.is_dev,
			updated_at = new.updated_at`,
		openid, name, nullString(avatar), boolInt(isDev), millis(now), millis(now))
	if err != nil {
		return User{}, fmt.Errorf("保存账号失败: %w", err)
	}
	return s.UserByOpenID(ctx, openid)
}

// UserByOpenID 按 QQ OpenID 查询账号。
func (s *Store) UserByOpenID(ctx context.Context, openid string) (User, error) {
	row := s.conn().QueryRowContext(ctx,
		`SELECT id, qq_openid, name, avatar, is_dev, created_at, updated_at FROM users WHERE qq_openid = ?`, openid)
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
