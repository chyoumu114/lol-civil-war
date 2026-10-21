package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

// ErrDuplicate 表示唯一键冲突（邮箱、QQ 绑定、user_no 等）。
var ErrDuplicate = errors.New("duplicate key")

const userColumns = `id, user_no, email, password_hash, qq_openid, name, avatar, is_dev, created_at, updated_at`

// CreateUser 创建账号。user_no 必须由调用方生成，email/password_hash/qq_openid 可为空。
func (s *Store) CreateUser(ctx context.Context, userNo string, email, passwordHash, qqOpenID *string, name string, avatar *string, isDev bool) (User, error) {
	now := time.Now().UTC()
	res, err := s.conn().ExecContext(ctx, `
		INSERT INTO users (user_no, email, password_hash, qq_openid, name, avatar, is_dev, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userNo, nullString(email), nullString(passwordHash), nullString(qqOpenID), name, nullString(avatar),
		boolInt(isDev), millis(now), millis(now))
	if err != nil {
		if isDuplicateKey(err) {
			return User{}, fmt.Errorf("%w: %v", ErrDuplicate, err)
		}
		return User{}, fmt.Errorf("创建账号失败: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}
	return s.UserByID(ctx, id)
}

// UserByID 按主键查询账号。
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	row := s.conn().QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id)
	return scanUser(row)
}

// UserByNo 按系统生成的 user_no 查询账号。
func (s *Store) UserByNo(ctx context.Context, userNo string) (User, error) {
	row := s.conn().QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE user_no = ?`, userNo)
	return scanUser(row)
}

// UserByEmail 按邮箱查询账号（邮箱统一小写）。
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	row := s.conn().QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE email = ?`, email)
	return scanUser(row)
}

// UserByQQOpenID 按 QQ OpenID 查询已绑定账号。
func (s *Store) UserByQQOpenID(ctx context.Context, openid string) (User, error) {
	row := s.conn().QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE qq_openid = ?`, openid)
	return scanUser(row)
}

// SetUserEmailPassword 为账号设置/更新邮箱与密码。
func (s *Store) SetUserEmailPassword(ctx context.Context, userID int64, email, passwordHash string) error {
	_, err := s.conn().ExecContext(ctx,
		`UPDATE users SET email = ?, password_hash = ?, updated_at = ? WHERE id = ?`,
		email, passwordHash, millis(time.Now().UTC()), userID)
	if err != nil {
		if isDuplicateKey(err) {
			return fmt.Errorf("%w: %v", ErrDuplicate, err)
		}
		return fmt.Errorf("保存邮箱密码失败: %w", err)
	}
	return nil
}

// BindQQOpenID 把 QQ OpenID 绑定到账号，并同步 QQ 昵称/头像。
func (s *Store) BindQQOpenID(ctx context.Context, userID int64, openid, name string, avatar *string) error {
	_, err := s.conn().ExecContext(ctx, `
		UPDATE users SET
			qq_openid = ?,
			name = IF(? <> '', ?, name),
			avatar = IF(? IS NOT NULL AND ? <> '', ?, avatar),
			updated_at = ?
		WHERE id = ?`,
		openid, name, name, nullString(avatar), nullString(avatar), nullString(avatar),
		millis(time.Now().UTC()), userID)
	if err != nil {
		if isDuplicateKey(err) {
			return fmt.Errorf("%w: %v", ErrDuplicate, err)
		}
		return fmt.Errorf("绑定 QQ 失败: %w", err)
	}
	return nil
}

// UpdateUserNameAvatar 同步 QQ 昵称/头像（用于已绑定账号每次登录）。
func (s *Store) UpdateUserNameAvatar(ctx context.Context, userID int64, name string, avatar *string) error {
	_, err := s.conn().ExecContext(ctx, `
		UPDATE users SET
			name = IF(? <> '', ?, name),
			avatar = IF(? IS NOT NULL AND ? <> '', ?, avatar),
			updated_at = ?
		WHERE id = ?`,
		name, name, nullString(avatar), nullString(avatar), nullString(avatar),
		millis(time.Now().UTC()), userID)
	return err
}

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var (
		u            User
		email        sql.NullString
		passwordHash sql.NullString
		openid       sql.NullString
		avatar       sql.NullString
		isDev        int
		created      int64
		updated      int64
	)
	err := row.Scan(&u.ID, &u.UserNo, &email, &passwordHash, &openid, &u.Name, &avatar,
		&isDev, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("读取账号失败: %w", err)
	}
	u.Email = stringPtr(email)
	u.PasswordHash = stringPtr(passwordHash)
	u.QQOpenID = stringPtr(openid)
	u.Avatar = stringPtr(avatar)
	u.IsDev = isDev == 1
	u.CreatedAt = fromMillis(created)
	u.UpdatedAt = fromMillis(updated)
	return u, nil
}

func isDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
