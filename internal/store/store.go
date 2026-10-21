// Package store 是 MySQL 持久化层：只做数据读写，不写业务规则。
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
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
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

// Open 连接 MySQL 并初始化表结构。启动阶段会重试，等待 MySQL 就绪。
func Open(dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("数据库 DSN 不能为空")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 MySQL 失败: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(3 * time.Minute)

	var pingErr error
	for attempt := 0; attempt < 15; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pingErr = db.PingContext(ctx)
		cancel()
		if pingErr == nil {
			break
		}
		if attempt < 14 {
			time.Sleep(2 * time.Second)
		}
	}
	if pingErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("连接 MySQL 失败: %w", pingErr)
	}

	if err := initSchema(db); err != nil {
		_ = db.Close()
		return nil, err
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

// initSchema 按分号拆分 schema.sql 并逐条执行（不开启 multiStatements）。
func initSchema(db *sql.DB) error {
	for _, statement := range schemaStatements() {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("初始化数据库失败: %w", err)
		}
	}
	return nil
}

func schemaStatements() []string {
	raw := string(mustSchema())
	var cleaned strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		cleaned.WriteString(line)
		cleaned.WriteString("\n")
	}

	parts := strings.Split(cleaned.String(), ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if statement := strings.TrimSpace(part); statement != "" {
			out = append(out, statement)
		}
	}
	return out
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
