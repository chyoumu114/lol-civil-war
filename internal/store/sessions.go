package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// GetSession 读取会话；过期或不存在返回 ErrNotFound。
func (s *Store) GetSession(ctx context.Context, id string) (Session, error) {
	var (
		sess    Session
		userID  sql.NullInt64
		expires int64
		created int64
	)
	err := s.conn().QueryRowContext(ctx,
		`SELECT id, user_id, csrf, oauth_state, oauth_redirect, expires_at, created_at FROM sessions WHERE id = ?`, id).
		Scan(&sess.ID, &userID, &sess.CSRF, &sess.OAuthState, &sess.OAuthRedirect, &expires, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("读取会话失败: %w", err)
	}
	if userID.Valid {
		v := userID.Int64
		sess.UserID = &v
	}
	sess.ExpiresAt = fromMillis(expires)
	sess.CreatedAt = fromMillis(created)
	return sess, nil
}

// PutSession 新建或更新会话。
func (s *Store) PutSession(ctx context.Context, sess Session) error {
	var userID any
	if sess.UserID != nil {
		userID = *sess.UserID
	}
	created := millis(sess.CreatedAt)
	if sess.CreatedAt.IsZero() {
		created = millis(time.Now().UTC())
	}
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, csrf, oauth_state, oauth_redirect, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			user_id = excluded.user_id,
			csrf = excluded.csrf,
			oauth_state = excluded.oauth_state,
			oauth_redirect = excluded.oauth_redirect,
			expires_at = excluded.expires_at`,
		sess.ID, userID, sess.CSRF, sess.OAuthState, sess.OAuthRedirect, millis(sess.ExpiresAt), created)
	if err != nil {
		return fmt.Errorf("保存会话失败: %w", err)
	}
	return nil
}

// DeleteSession 删除会话。
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.conn().ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DeleteExpiredSessions 清理过期会话（顺带避免表无限增长）。
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := s.conn().ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, millis(now))
	return err
}
