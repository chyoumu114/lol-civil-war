// Package config 负责从环境变量与 .env 文件加载运行配置。
//
// 设计原则：
//  1. 所有可变配置都通过环境变量注入，容器里用 env_file 提供；
//  2. 敏感信息（数据库口令、QQ AppSecret、群白名单）绝不写进代码或仓库；
//  3. 所有配置都有安全默认值，未配置时服务仍可启动（群白名单为空则拒绝所有群）。
package config

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Config 是应用的全部运行配置。
type Config struct {
	// 监听地址，例如 :8080。
	Addr string
	// 数据目录：上传临时文件、可选英雄图标。
	DataDir string
	// 对外访问地址，用于推导 QQ 回调地址与 Cookie Secure 判定。
	BaseURL string
	// 时区名称，默认 Asia/Shanghai。
	TimeZone string

	// MySQL 连接：可直接给 DSN，也可用分项配置拼装。
	DBDSN  string
	DBHost string
	DBUser string
	DBName string

	// 允许开通的 QQ 群号（白名单）。为空时所有群都会被拒绝。
	AllowedGroups []string
	// 是否开放开发登录（仅本地调试使用，生产必须关闭）。
	DevLogin bool

	// QQ 互联（网站应用）凭据。
	QQAppID       string
	QQAppKey      string
	QQRedirectURI string

	// ROFL 录像上传上限（KB）。
	RoflMaxKB int64
	// 会话有效期。
	SessionTTL time.Duration
	// 是否强制 Secure Cookie；auto 时按 BaseURL 的 scheme 判断。
	CookieSecure bool
	// 是否信任反向代理传来的 X-Forwarded-*。
	TrustProxy bool
	// 本地英雄图标目录（可选）。目录内按 {DataDragon key}.png 命名。
	ChampionIconDir string
	// DataDragon 版本，空表示自动探测。
	DDragonVersion string
	// 自动匹配返回方案数与搜索节点预算。
	RosterMaxPlans int
	RosterBudget   int
	// 段位小段步长。
	DivisionStep int
}

// Load 读取配置：先加载 .env（不覆盖已存在的环境变量），再逐项解析。
func Load() (*Config, error) {
	loadDotEnv(".env")
	loadDotEnv(filepath.Join(env("LCW_DATA_DIR", "./data"), ".env"))

	cfg := &Config{
		Addr:            env("LCW_ADDR", ":8080"),
		DataDir:         env("LCW_DATA_DIR", "./data"),
		BaseURL:         strings.TrimRight(env("LCW_BASE_URL", "http://localhost:8080"), "/"),
		TimeZone:        env("LCW_TIMEZONE", "Asia/Shanghai"),
		DBHost:          env("LCW_DB_HOST", "127.0.0.1"),
		DBUser:          env("LCW_DB_USER", "root"),
		DBName:          env("LCW_DB_NAME", "lol_civil_war"),
		AllowedGroups:   splitList(env("LCW_ALLOWED_GROUPS", "")),
		DevLogin:        envBool("LCW_DEV_LOGIN", false),
		QQAppID:         env("LCW_QQ_APP_ID", ""),
		QQAppKey:        env("LCW_QQ_APP_KEY", ""),
		QQRedirectURI:   env("LCW_QQ_REDIRECT_URI", ""),
		RoflMaxKB:       envInt64("LCW_ROFL_MAX_KB", 131072),
		SessionTTL:      time.Duration(envInt64("LCW_SESSION_TTL_HOURS", 720)) * time.Hour,
		TrustProxy:      envBool("LCW_TRUST_PROXY", true),
		ChampionIconDir: env("LCW_CHAMPION_ICON_DIR", ""),
		DDragonVersion:  env("LCW_DDRAGON_VERSION", ""),
		RosterMaxPlans:  int(envInt64("LCW_ROSTER_MAX_PLANS", 5)),
		RosterBudget:    int(envInt64("LCW_ROSTER_BUDGET", 3000000)),
		DivisionStep:    int(envInt64("LCW_DIVISION_STEP", 100)),
	}

	dsn := strings.TrimSpace(env("LCW_DB_DSN", ""))
	if dsn == "" {
		var err error
		dsn, err = buildMySQLDSN(
			env("LCW_DB_HOST", "127.0.0.1"),
			env("LCW_DB_PORT", "3306"),
			env("LCW_DB_USER", "root"),
			env("LCW_DB_PASSWORD", ""),
			env("LCW_DB_NAME", "lol_civil_war"),
		)
		if err != nil {
			return nil, err
		}
	}
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("LCW_DB_DSN 不合法: %w", err)
	}
	// 时间统一用整数毫秒存储，不需要驱动做时间解析。
	parsed.ParseTime = false
	parsed.Loc = time.UTC
	if parsed.Timeout == 0 {
		parsed.Timeout = 5 * time.Second
	}
	if parsed.ReadTimeout == 0 {
		parsed.ReadTimeout = 30 * time.Second
	}
	if parsed.WriteTimeout == 0 {
		parsed.WriteTimeout = 30 * time.Second
	}
	cfg.DBDSN = parsed.FormatDSN()
	cfg.DBHost = parsed.Addr
	cfg.DBUser = parsed.User
	cfg.DBName = parsed.DBName

	if cfg.QQRedirectURI == "" {
		cfg.QQRedirectURI = cfg.BaseURL + "/auth/qq/callback"
	}
	if cfg.ChampionIconDir == "" {
		cfg.ChampionIconDir = filepath.Join(cfg.DataDir, "champions")
	}

	switch strings.ToLower(env("LCW_COOKIE_SECURE", "auto")) {
	case "true", "1", "yes":
		cfg.CookieSecure = true
	case "false", "0", "no":
		cfg.CookieSecure = false
	default:
		cfg.CookieSecure = strings.HasPrefix(cfg.BaseURL, "https://")
	}

	if cfg.RosterMaxPlans < 1 {
		cfg.RosterMaxPlans = 1
	}
	if cfg.RosterBudget < 1000 {
		cfg.RosterBudget = 1000
	}
	if cfg.DivisionStep < 1 {
		cfg.DivisionStep = 100
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 720 * time.Hour
	}

	if _, err := time.LoadLocation(cfg.TimeZone); err != nil {
		return nil, fmt.Errorf("LCW_TIMEZONE 不合法: %w", err)
	}
	return cfg, nil
}

// buildMySQLDSN 用分项配置拼装 DSN，避免口令中的特殊字符破坏格式。
func buildMySQLDSN(host, port, user, password, name string) (string, error) {
	if host == "" {
		return "", errors.New("LCW_DB_HOST 不能为空")
	}
	if port == "" {
		port = "3306"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("LCW_DB_PORT 不合法: %w", err)
	}
	mysqlCfg := mysql.NewConfig()
	mysqlCfg.User = user
	mysqlCfg.Passwd = password
	mysqlCfg.Net = "tcp"
	mysqlCfg.Addr = net.JoinHostPort(host, port)
	mysqlCfg.DBName = name
	mysqlCfg.ParseTime = false
	mysqlCfg.Loc = time.UTC
	mysqlCfg.Timeout = 5 * time.Second
	mysqlCfg.ReadTimeout = 30 * time.Second
	mysqlCfg.WriteTimeout = 30 * time.Second
	mysqlCfg.Params = map[string]string{
		"charset":   "utf8mb4",
		"collation": "utf8mb4_0900_ai_ci",
	}
	return mysqlCfg.FormatDSN(), nil
}

// QQConfigured 报告 QQ 互联凭据是否完整。
func (c *Config) QQConfigured() bool {
	return c.QQAppID != "" && c.QQAppKey != ""
}

// Location 返回配置的时区。
func (c *Config) Location() *time.Location {
	loc, err := time.LoadLocation(c.TimeZone)
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

// EnsureDataDir 创建数据目录并返回绝对路径。
func (c *Config) EnsureDataDir() (string, error) {
	abs, err := filepath.Abs(c.DataDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return "", err
	}
	return abs, nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func envInt64(key string, def int64) int64 {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return def
	}
	return n
}

func splitList(v string) []string {
	out := make([]string, 0, 4)
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv 读取 KEY=VALUE 形式的 .env 文件；已存在的环境变量优先。
// 文件不存在不是错误，方便容器里只用环境变量。
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"' || val[0] == '\'' && val[len(val)-1] == '\'') {
			val = val[1 : len(val)-1]
		}
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, val)
	}
}

// ErrNoGroups 表示未配置群白名单时的明确错误。
var ErrNoGroups = errors.New("未配置群白名单：请设置 LCW_ALLOWED_GROUPS")
