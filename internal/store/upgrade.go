package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"
)

// upgradeSchema 对已有数据库做幂等升级（新库执行时基本都是 no-op）。
// 当前负责：
//   - users 增加 user_no / email / password_hash，QQ OpenID 改为可空；
//   - 为存量用户回填 user_no 并建立唯一索引；
//   - sessions 增加 QQ 绑定/待注册状态字段；
//   - group_members 增加群内 QQ 号唯一索引（无重复时）。
func upgradeSchema(ctx context.Context, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// users 新列（先允许 NULL，回填后再收紧 user_no）
	columns := []struct{ name, ddl string }{
		{"user_no", "user_no VARCHAR(16) NULL"},
		{"email", "email VARCHAR(255) NULL"},
		{"password_hash", "password_hash VARCHAR(255) NULL"},
	}
	for _, col := range columns {
		if err := ensureColumn(ctx, db, "users", col.name, col.ddl); err != nil {
			return err
		}
	}

	// qq_openid 现在允许邮箱用户为空
	nullable, err := columnNullable(ctx, db, "users", "qq_openid")
	if err != nil {
		return err
	}
	if !nullable {
		if _, err := db.ExecContext(ctx,
			"ALTER TABLE users MODIFY qq_openid VARCHAR(128) NULL"); err != nil {
			return fmt.Errorf("修改 qq_openid 可空失败: %w", err)
		}
	}

	if err := backfillUserNos(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx,
		"ALTER TABLE users MODIFY user_no VARCHAR(16) NOT NULL"); err != nil {
		return fmt.Errorf("收紧 user_no 非空失败: %w", err)
	}
	if err := ensureUniqueIndex(ctx, db, "users", "uk_users_user_no", "user_no"); err != nil {
		return err
	}
	if err := ensureUniqueIndex(ctx, db, "users", "uk_users_email", "email"); err != nil {
		return err
	}

	// sessions 的 QQ 待注册 / 绑定状态
	sessionColumns := []struct{ name, ddl string }{
		{"qq_pending_openid", "qq_pending_openid VARCHAR(128) NOT NULL DEFAULT ''"},
		{"qq_pending_user_id", "qq_pending_user_id BIGINT NULL"},
		{"qq_pending_name", "qq_pending_name VARCHAR(64) NOT NULL DEFAULT ''"},
		{"qq_pending_avatar", "qq_pending_avatar VARCHAR(255) NOT NULL DEFAULT ''"},
		{"qq_bind_user_id", "qq_bind_user_id BIGINT NULL"},
	}
	for _, col := range sessionColumns {
		if err := ensureColumn(ctx, db, "sessions", col.name, col.ddl); err != nil {
			return err
		}
	}

	// QQ 号在群内唯一（NULL 不参与唯一约束）
	duplicates, err := duplicateQQNumbers(ctx, db)
	if err != nil {
		return err
	}
	if duplicates == 0 {
		if err := ensureUniqueIndex(ctx, db, "group_members", "uk_group_members_qq", "group_id, qq_number"); err != nil {
			return err
		}
	}

	// sessions：空闲超时与绝对寿命分离（M-04）；存量会话按 30 天绝对寿命回填。
	if err := ensureColumn(ctx, db, "sessions", "absolute_expires_at", "absolute_expires_at BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE sessions SET absolute_expires_at = created_at + ? WHERE absolute_expires_at = 0`,
		int64((30*24*time.Hour)/time.Millisecond)); err != nil {
		return fmt.Errorf("回填会话绝对寿命失败: %w", err)
	}

	// sessions：令牌改为 SHA-256 摘要（V-04），同时把绝对寿命收敛到 14 天。
	// 旧行是 40 字符明文令牌，迁移后旧 Cookie 仍可用（查询时同样做摘要）。
	if _, err := db.ExecContext(ctx,
		`UPDATE sessions SET id = SHA2(id, 256) WHERE id NOT REGEXP '^[0-9a-f]{64}$'`); err != nil {
		return fmt.Errorf("会话令牌摘要迁移失败: %w", err)
	}
	const sessionMaxTTLMS = int64((14 * 24 * time.Hour) / time.Millisecond)
	if _, err := db.ExecContext(ctx,
		`UPDATE sessions SET absolute_expires_at = created_at + ?
		 WHERE absolute_expires_at = 0 OR absolute_expires_at > created_at + ?`,
		sessionMaxTTLMS, sessionMaxTTLMS); err != nil {
		return fmt.Errorf("收敛会话绝对寿命失败: %w", err)
	}
	if err := ensureIndex(ctx, db, "sessions", "idx_sessions_user", "user_id"); err != nil {
		return err
	}

	// rofl_uploads：录像去重改为按群隔离（L-01）。
	if err := ensureColumn(ctx, db, "rofl_uploads", "group_id", "group_id BIGINT NULL"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE rofl_uploads r
		JOIN matches m ON m.id = r.match_id
		JOIN events e ON e.id = m.event_id
		SET r.group_id = e.group_id
		WHERE r.group_id IS NULL`); err != nil {
		return fmt.Errorf("回填录像群归属失败: %w", err)
	}
	if err := dropIndexIfExists(ctx, db, "rofl_uploads", "uk_rofl_sha256"); err != nil {
		return err
	}
	if err := ensureUniqueIndex(ctx, db, "rofl_uploads", "uk_rofl_group_sha256", "group_id, sha256"); err != nil {
		return err
	}
	return nil
}

func ensureColumn(ctx context.Context, db *sql.DB, table, column, ddl string) error {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
		table, column).Scan(&count)
	if err != nil {
		return fmt.Errorf("检查列 %s.%s 失败: %w", table, column, err)
	}
	if count > 0 {
		return nil
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE `"+table+"` ADD COLUMN "+ddl); err != nil {
		return fmt.Errorf("添加列 %s.%s 失败: %w", table, column, err)
	}
	return nil
}

func columnNullable(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	var nullable string
	err := db.QueryRowContext(ctx, `
		SELECT is_nullable FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
		table, column).Scan(&nullable)
	if err != nil {
		return false, fmt.Errorf("检查列 %s.%s 失败: %w", table, column, err)
	}
	return nullable == "YES", nil
}

func dropIndexIfExists(ctx context.Context, db *sql.DB, table, index string) error {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`,
		table, index).Scan(&count)
	if err != nil {
		return fmt.Errorf("检查索引 %s.%s 失败: %w", table, index, err)
	}
	if count == 0 {
		return nil
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE `"+table+"` DROP INDEX `"+index+"`"); err != nil {
		return fmt.Errorf("删除索引 %s.%s 失败: %w", table, index, err)
	}
	return nil
}

func ensureIndex(ctx context.Context, db *sql.DB, table, index, columns string) error {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`,
		table, index).Scan(&count)
	if err != nil {
		return fmt.Errorf("检查索引 %s.%s 失败: %w", table, index, err)
	}
	if count > 0 {
		return nil
	}
	if _, err := db.ExecContext(ctx,
		"ALTER TABLE `"+table+"` ADD INDEX `"+index+"` ("+columns+")"); err != nil {
		return fmt.Errorf("创建索引 %s.%s 失败: %w", table, index, err)
	}
	return nil
}

func ensureUniqueIndex(ctx context.Context, db *sql.DB, table, index, columns string) error {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`,
		table, index).Scan(&count)
	if err != nil {
		return fmt.Errorf("检查索引 %s.%s 失败: %w", table, index, err)
	}
	if count > 0 {
		return nil
	}
	if _, err := db.ExecContext(ctx,
		"ALTER TABLE `"+table+"` ADD UNIQUE KEY `"+index+"` ("+columns+")"); err != nil {
		return fmt.Errorf("创建索引 %s.%s 失败: %w", table, index, err)
	}
	return nil
}

func backfillUserNos(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT id FROM users WHERE user_no IS NULL OR user_no = ''`)
	if err != nil {
		return fmt.Errorf("查询待回填用户失败: %w", err)
	}
	ids := make([]int64, 0, 64)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, id := range ids {
		var lastErr error
		for attempt := 0; attempt < 20; attempt++ {
			userNo, genErr := newUserNo()
			if genErr != nil {
				return genErr
			}
			var exists int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE user_no = ?`, userNo).Scan(&exists); err != nil {
				return fmt.Errorf("检查 user_no 冲突失败: %w", err)
			}
			if exists > 0 {
				continue
			}
			_, lastErr = db.ExecContext(ctx, `UPDATE users SET user_no = ? WHERE id = ?`, userNo, id)
			if lastErr == nil {
				break
			}
			if !isDuplicateKey(lastErr) {
				break
			}
		}
		if lastErr != nil {
			return fmt.Errorf("回填用户 %d 的 user_no 失败: %w", id, lastErr)
		}
	}
	return nil
}

func duplicateQQNumbers(ctx context.Context, db *sql.DB) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT group_id, qq_number FROM group_members
			WHERE qq_number IS NOT NULL AND qq_number <> ''
			GROUP BY group_id, qq_number HAVING COUNT(*) > 1
		) t`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("检查重复 QQ 号失败: %w", err)
	}
	return count, nil
}

// newUserNo 生成 U + 8 位随机数字；随机源失败时返回错误，绝不退化为时间戳（L-07）。
func newUserNo() (string, error) {
	const digits = "0123456789"
	const maxAcceptable = 250 // 250 = 25 * 10，拒绝采样消除取模偏差
	buf := make([]byte, 8)
	var b [1]byte
	for i := 0; i < len(buf); {
		if _, err := rand.Read(b[:]); err != nil {
			return "", fmt.Errorf("crypto/rand 不可用: %w", err)
		}
		if int(b[0]) >= maxAcceptable {
			continue
		}
		buf[i] = digits[int(b[0])%len(digits)]
		i++
	}
	return "U" + string(buf), nil
}
