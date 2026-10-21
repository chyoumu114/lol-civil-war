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
		sess        Session
		userID      sql.NullInt64
		pendingUser sql.NullInt64
		bindUser    sql.NullInt64
		expires     int64
		created     int64
	)
	err := s.conn().QueryRowContext(ctx, `
		SELECT id, user_id, csrf, oauth_state, oauth_redirect,
		       qq_pending_openid, qq_pending_user_id, qq_pending_name, qq_pending_avatar,
		       qq_bind_user_id, expires_at, created_at
		FROM sessions WHERE id = ?`, id).
		Scan(&sess.ID, &userID, &sess.CSRF, &sess.OAuthState, &sess.OAuthRedirect,
			&sess.QQPendingOpenID, &pendingUser, &sess.QQPendingName, &sess.QQPendingAvatar,
			&bindUser, &expires, &created)
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
	if pendingUser.Valid {
		v := pendingUser.Int64
		sess.QQPendingUserID = &v
	}
	if bindUser.Valid {
		v := bindUser.Int64
		sess.QQBindUserID = &v
	}
	sess.ExpiresAt = fromMillis(expires)
	sess.CreatedAt = fromMillis(created)
	return sess, nil
}

// PutSession 新建或更新会话。
func (s *Store) PutSession(ctx context.Context, sess Session) error {
	var userID, pendingUser, bindUser any
	if sess.UserID != nil {
		userID = *sess.UserID
	}
	if sess.QQPendingUserID != nil {
		pendingUser = *sess.QQPendingUserID
	}
	if sess.QQBindUserID != nil {
		bindUser = *sess.QQBindUserID
	}
	created := millis(sess.CreatedAt)
	if sess.CreatedAt.IsZero() {
		created = millis(time.Now().UTC())
	}
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, csrf, oauth_state, oauth_redirect,
			qq_pending_openid, qq_pending_user_id, qq_pending_name, qq_pending_avatar,
			qq_bind_user_id, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) AS new
		ON DUPLICATE KEY UPDATE
			user_id = new.user_id,
			csrf = new.csrf,
			oauth_state = new.oauth_state,
			oauth_redirect = new.oauth_redirect,
			qq_pending_openid = new.qq_pending_openid,
			qq_pending_user_id = new.qq_pending_user_id,
			qq_pending_name = new.qq_pending_name,
			qq_pending_avatar = new.qq_pending_avatar,
			qq_bind_user_id = new.qq_bind_user_id,
			expires_at = new.expires_at`,
		sess.ID, userID, sess.CSRF, sess.OAuthState, sess.OAuthRedirect,
		sess.QQPendingOpenID, pendingUser, sess.QQPendingName, sess.QQPendingAvatar,
		bindUser, millis(sess.ExpiresAt), created)
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
