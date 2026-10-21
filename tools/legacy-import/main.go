// Command legacy-import 把旧版 lol-youmu-tools（Laravel）的 MySQL 数据导入新项目。
//
// 只做数据搬运：保留主键、按外键顺序写入、把 DATETIME 转成 UTC 毫秒，
// 并把旧群号加入新库的数据库白名单。
//
// 用法：
//
//	go run ./tools/legacy-import \
//	  -source 'root:pass@tcp(127.0.0.1:33062)/lolyoumu?charset=utf8mb4' \
//	  -target 'root:pass@tcp(127.0.0.1:3307)/lol_civil_war_dev?charset=utf8mb4' \
//	  -truncate
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/chyoumu114/lol-civil-war/internal/store"
)

type tableSpec struct {
	source string
	target string
}

// 按外键依赖顺序排列。
var tableSpecs = []tableSpec{
	{"users", "users"},
	{"groups", "qq_groups"},
	{"group_members", "group_members"},
	{"events", "events"},
	{"registrations", "registrations"},
	{"event_rosters", "event_rosters"},
	{"event_spectators", "event_spectators"},
	{"matches", "matches"},
	{"match_players", "match_players"},
	{"match_bans", "match_bans"},
	{"match_player_ratings", "match_player_ratings"},
	{"rofl_uploads", "rofl_uploads"},
}

var datetimeColumns = map[string]bool{
	"created_at":      true,
	"updated_at":      true,
	"ended_at":        true,
	"signup_deadline": true,
	"starts_at":       true,
	"submitted_at":    true,
}

var errTableMissing = errors.New("table missing")

func main() {
	sourceDSN := flag.String("source", "", "旧库 DSN（lolyoumu）")
	targetDSN := flag.String("target", "", "新库 DSN（lol_civil_war）")
	truncate := flag.Bool("truncate", false, "导入前清空目标业务表")
	whitelist := flag.Bool("whitelist", true, "把旧群号加入目标数据库白名单")
	sourceTZ := flag.String("source-tz", "Asia/Shanghai", "旧库 DATETIME 的时区")
	flag.Parse()

	if *sourceDSN == "" || *targetDSN == "" {
		fmt.Fprintln(os.Stderr, "必须提供 -source 与 -target")
		flag.Usage()
		os.Exit(2)
	}
	loc, err := time.LoadLocation(*sourceTZ)
	if err != nil {
		log.Fatalf("时区不合法: %v", err)
	}

	srcCfg, err := mysql.ParseDSN(*sourceDSN)
	if err != nil {
		log.Fatalf("source DSN 不合法: %v", err)
	}
	srcCfg.ParseTime = false // DATETIME 以字符串读，自行按 source-tz 解析
	srcCfg.Loc = loc
	ensureCharset(srcCfg)

	dstCfg, err := mysql.ParseDSN(*targetDSN)
	if err != nil {
		log.Fatalf("target DSN 不合法: %v", err)
	}
	dstCfg.ParseTime = false
	dstCfg.Loc = time.UTC
	ensureCharset(dstCfg)

	if err := store.EnsureSchema(dstCfg.FormatDSN()); err != nil {
		log.Fatalf("初始化目标表结构失败: %v", err)
	}

	src, err := sql.Open("mysql", srcCfg.FormatDSN())
	if err != nil {
		log.Fatalf("连接旧库失败: %v", err)
	}
	defer src.Close()
	dst, err := sql.Open("mysql", dstCfg.FormatDSN())
	if err != nil {
		log.Fatalf("连接新库失败: %v", err)
	}
	defer dst.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := src.PingContext(ctx); err != nil {
		log.Fatalf("旧库不可用: %v", err)
	}
	if err := dst.PingContext(ctx); err != nil {
		log.Fatalf("新库不可用: %v", err)
	}

	if *truncate {
		if err := truncateTarget(ctx, dst); err != nil {
			log.Fatalf("清空目标表失败: %v", err)
		}
		fmt.Println("已清空目标业务表")
	}

	tx, err := dst.BeginTx(ctx, nil)
	if err != nil {
		log.Fatalf("开启事务失败: %v", err)
	}
	total := 0
	for _, spec := range tableSpecs {
		n, err := copyTable(ctx, tx, src, spec, loc)
		if errors.Is(err, errTableMissing) {
			fmt.Printf("%-24s 跳过（旧库没有该表）\n", spec.target)
			continue
		}
		if err != nil {
			_ = tx.Rollback()
			log.Fatalf("导入 %s 失败: %v", spec.target, err)
		}
		total += n
		fmt.Printf("%-24s %6d 行\n", spec.target, n)
	}

	if *whitelist {
		added, err := importAllowedGroups(ctx, tx, src)
		if err != nil {
			_ = tx.Rollback()
			log.Fatalf("导入群白名单失败: %v", err)
		}
		fmt.Printf("%-24s %6d 个\n", "allowed_groups", added)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("提交事务失败: %v", err)
	}
	if err := resetAutoIncrement(ctx, dst); err != nil {
		log.Fatalf("重置自增失败: %v", err)
	}
	fmt.Printf("导入完成，共 %d 行\n", total)
}

func ensureCharset(cfg *mysql.Config) {
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	if _, ok := cfg.Params["charset"]; !ok {
		cfg.Params["charset"] = "utf8mb4"
	}
}

func truncateTarget(ctx context.Context, db *sql.DB) error {
	order := []string{
		"match_player_ratings", "match_bans", "match_players", "rofl_uploads",
		"matches", "event_spectators", "event_rosters", "registrations",
		"events", "group_members", "qq_groups", "users", "allowed_groups",
	}
	if _, err := db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return err
	}
	for _, table := range order {
		if _, err := db.ExecContext(ctx, "TRUNCATE TABLE `"+table+"`"); err != nil {
			return fmt.Errorf("%s: %w", table, err)
		}
	}
	_, err := db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1")
	return err
}

func copyTable(ctx context.Context, tx *sql.Tx, src *sql.DB, spec tableSpec, loc *time.Location) (int, error) {
	sourceCols, err := tableColumns(ctx, src, spec.source)
	if err != nil {
		return 0, err
	}
	targetCols, err := tableColumns(ctx, tx, spec.target)
	if err != nil {
		return 0, err
	}

	sourceSet := make(map[string]bool, len(sourceCols))
	for _, col := range sourceCols {
		sourceSet[col] = true
	}
	cols := make([]string, 0, len(targetCols))
	for _, col := range targetCols {
		if sourceSet[col] {
			cols = append(cols, col)
		}
	}
	if len(cols) == 0 {
		return 0, fmt.Errorf("没有可导入的公共列")
	}

	quoted := make([]string, len(cols))
	placeholders := make([]string, len(cols))
	for i, col := range cols {
		quoted[i] = "`" + col + "`"
		placeholders[i] = "?"
	}
	selectSQL := fmt.Sprintf("SELECT %s FROM `%s`", strings.Join(quoted, ", "), spec.source)
	if contains(cols, "id") {
		selectSQL += " ORDER BY `id`"
	}
	insertSQL := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)",
		spec.target, strings.Join(quoted, ", "), strings.Join(placeholders, ", "))

	rows, err := src.QueryContext(ctx, selectSQL)
	if err != nil {
		if isMissingTable(err) {
			return 0, errTableMissing
		}
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		raw := make([]sql.NullString, len(cols))
		scanArgs := make([]any, len(cols))
		for i := range raw {
			scanArgs[i] = &raw[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return count, err
		}

		values := make([]any, len(cols))
		for i, col := range cols {
			if !raw[i].Valid {
				values[i] = nil
				continue
			}
			value := raw[i].String
			switch {
			case datetimeColumns[col]:
				if ms, ok := parseLegacyTime(value, loc); ok {
					values[i] = ms
				} else {
					values[i] = nil
				}
			case col == "public_key" && strings.TrimSpace(value) == "":
				values[i] = randomKey(32)
			default:
				values[i] = value
			}
		}
		if _, err := tx.ExecContext(ctx, insertSQL, values...); err != nil {
			return count, fmt.Errorf("第 %d 行: %w", count+1, err)
		}
		count++
	}
	return count, rows.Err()
}

func importAllowedGroups(ctx context.Context, tx *sql.Tx, src *sql.DB) (int, error) {
	rows, err := src.QueryContext(ctx, "SELECT `qq_group_number` FROM `groups`")
	if err != nil {
		if isMissingTable(err) {
			return 0, nil
		}
		return 0, err
	}
	defer rows.Close()

	added := 0
	now := time.Now().UTC().UnixMilli()
	for rows.Next() {
		var number string
		if err := rows.Scan(&number); err != nil {
			return added, err
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT IGNORE INTO `allowed_groups` (`qq_group_number`, `note`, `created_at`) VALUES (?, ?, ?)",
			number, "从旧版导入", now); err != nil {
			return added, err
		}
		added++
	}
	return added, rows.Err()
}

func resetAutoIncrement(ctx context.Context, db *sql.DB) error {
	for _, spec := range tableSpecs {
		var maxID sql.NullInt64
		if err := db.QueryRowContext(ctx,
			"SELECT COALESCE(MAX(`id`), 0) FROM `"+spec.target+"`").Scan(&maxID); err != nil {
			return fmt.Errorf("%s: %w", spec.target, err)
		}
		next := maxID.Int64 + 1
		if next < 1 {
			next = 1
		}
		if _, err := db.ExecContext(ctx,
			fmt.Sprintf("ALTER TABLE `%s` AUTO_INCREMENT = %d", spec.target, next)); err != nil {
			return fmt.Errorf("%s: %w", spec.target, err)
		}
	}
	return nil
}

func tableColumns(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, table string) ([]string, error) {
	rows, err := q.QueryContext(ctx, "SELECT * FROM `"+table+"` LIMIT 0")
	if err != nil {
		if isMissingTable(err) {
			return nil, errTableMissing
		}
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	return cols, nil
}

func parseLegacyTime(value string, loc *time.Location) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "0000-00-00") {
		return 0, false
	}
	layouts := []string{
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t.UTC().UnixMilli(), true
		}
	}
	return 0, false
}

func isMissingTable(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && (mysqlErr.Number == 1146 || mysqlErr.Number == 1051)
}

func contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

func randomKey(n int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return strings.Repeat("0", n)
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf)
}
