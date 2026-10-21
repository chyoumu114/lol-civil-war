// Package store 是 SQLite 持久化层：只做数据读写，不写业务规则。
//
// 表结构与 Laravel 版本一一对应，便于对照迁移；时间统一存 UTC 毫秒，
// 对外由 service 层转换为业务时区。
package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

// ErrNotFound 表示记录不存在。
var ErrNotFound = errors.New("record not found")

// Store 封装数据库连接；事务中的 Store 共享同一个 *sql.Tx。
type Store struct {
	db *sql.DB
	tx *sql.Tx
}

// conn 返回当前应使用的连接（事务优先）。
func (s *Store) conn() execer {
	if s.tx != nil {
		return s.tx
	}
	return s.db
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Open 打开（必要时创建）数据库并执行建表语句。
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
	}

	dsn := path
	if !strings.HasPrefix(dsn, "file:") {
		dsn = "file:" + filepath.ToSlash(dsn)
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	dsn += sep + "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	if _, err := db.Exec(string(mustSchema())); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}
	return &Store{db: db}, nil
}

// Close 关闭数据库。
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Ping 检查数据库可用性。
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Tx 在事务中执行 fn；fn 返回错误则回滚。
func (s *Store) Tx(ctx context.Context, fn func(tx *Store) error) error {
	if s.tx != nil {
		return fn(s)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	txStore := &Store{db: s.db, tx: tx}
	if err := fn(txStore); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

func mustSchema() []byte {
	data, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		panic(err)
	}
	return data
}

// ---- 通用转换工具 ----

func millis(t time.Time) int64 { return t.UTC().UnixMilli() }

func fromMillis(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

func nullTimePtr(n sql.NullInt64) *time.Time {
	if !n.Valid {
		return nil
	}
	t := fromMillis(n.Int64)
	return &t
}

func intPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func stringPtr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	v := n.String
	return &v
}

func jsonList(values []string) string {
	if values == nil {
		values = []string{}
	}
	data, _ := json.Marshal(values)
	return string(data)
}

func parseJSONList(raw sql.NullString) []string {
	if !raw.Valid || raw.String == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(raw.String), &out); err != nil || out == nil {
		return []string{}
	}
	return out
}
