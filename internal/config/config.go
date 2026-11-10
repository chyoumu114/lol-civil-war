// Package config 负责从环境变量与 .env 文件加载运行配置。
//
// 设计原则：
//  1. 所有可变配置都通过环境变量注入，容器里用 env_file 提供；
//  2. 敏感信息（数据库口令、QQ AppSecret、群白名单）绝不写进代码或仓库；
//  3. 安全相关默认值一律收紧：默认不信任代理、默认不允许空口令、生产形态禁止开发登录。
package config

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
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
	DBDSN      string
	DBHost     string
	DBUser     string
	DBPassword string
	DBName     string

	// 允许开通的 QQ 群号（白名单）。为空时所有群都会被拒绝。
	AllowedGroups []string
	// 是否开放开发登录（仅本地调试使用，生产必须关闭）。
	DevLogin bool
	// DevMode 显式声明“本地/内网调试环境”，用于放宽仅生产需要收紧的校验。
	DevMode bool

	// QQ 互联（网站应用）凭据。
	QQAppID       string
	QQAppKey      string
	QQRedirectURI string

	// ROFL 录像上传上限（KB）。
	RoflMaxKB int64
	// 录像上传的全局并发配额。
	UploadConcurrency int
	// 会话空闲有效期。
	SessionTTL time.Duration
	// 会话绝对寿命（登录后无论如何最多存活这么久）。
	SessionMaxTTL time.Duration
	// 是否强制 Secure Cookie；auto 时按 BaseURL 的 scheme 判断。
	CookieSecure bool
	// 是否信任反向代理传来的 X-Forwarded-*（deprecated，等价于配置回环地址为可信代理）。
	TrustProxy bool
	// 可信反向代理网段；只有来源命中这些网段的请求才会解析 X-Forwarded-For。
	TrustedProxies []netip.Prefix
	// 本地英雄图标目录（可选）。目录内按 {DataDragon key}.png 命名。
	ChampionIconDir string
	// DataDragon 版本，空表示自动探测。
	DDragonVersion string
	// 自动匹配返回方案数与搜索节点预算。
	RosterMaxPlans int
	RosterBudget   int
	// 段位小段步长。
	DivisionStep int
	// 启动时是否自动建表/升级表结构（生产建议关闭，改用 migrate 子命令）。
	MigrateOnStart bool
}

// Load 读取配置：先加载 .env（不覆盖已存在的环境变量），再逐项解析。
func Load() (*Config, error) {
	loadDotEnv(".env")
	loadDotEnv(filepath.Join(env("LCW_DATA_DIR", "./data"), ".env"))

	trustedProxies, err := parseTrustedProxies(env("LCW_TRUSTED_PROXIES", ""))
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		Addr:              env("LCW_ADDR", ":8080"),
		DataDir:           env("LCW_DATA_DIR", "./data"),
		BaseURL:           strings.TrimRight(env("LCW_BASE_URL", "http://localhost:8080"), "/"),
		TimeZone:          env("LCW_TIMEZONE", "Asia/Shanghai"),
		DBHost:            env("LCW_DB_HOST", "127.0.0.1"),
		DBUser:            env("LCW_DB_USER", "root"),
		DBName:            env("LCW_DB_NAME", "lol_civil_war"),
		AllowedGroups:     splitList(env("LCW_ALLOWED_GROUPS", "")),
		DevLogin:          envBool("LCW_DEV_LOGIN", false),
		DevMode:           envBool("LCW_DEV_MODE", false),
		QQAppID:           env("LCW_QQ_APP_ID", ""),
		QQAppKey:          env("LCW_QQ_APP_KEY", ""),
		QQRedirectURI:     env("LCW_QQ_REDIRECT_URI", ""),
		RoflMaxKB:         envInt64("LCW_ROFL_MAX_KB", 64*1024),
		UploadConcurrency: int(envInt64("LCW_UPLOAD_CONCURRENCY", 2)),
		SessionTTL:        time.Duration(envInt64("LCW_SESSION_TTL_HOURS", 168)) * time.Hour,
		SessionMaxTTL:     time.Duration(envInt64("LCW_SESSION_MAX_HOURS", 720)) * time.Hour,
		TrustProxy:        envBool("LCW_TRUST_PROXY", false),
		TrustedProxies:    trustedProxies,
		ChampionIconDir:   env("LCW_CHAMPION_ICON_DIR", ""),
		DDragonVersion:    env("LCW_DDRAGON_VERSION", ""),
		RosterMaxPlans:    int(envInt64("LCW_ROSTER_MAX_PLANS", 5)),
		RosterBudget:      int(envInt64("LCW_ROSTER_BUDGET", 3000000)),
		DivisionStep:      int(envInt64("LCW_DIVISION_STEP", 100)),
		MigrateOnStart:    envBool("LCW_MIGRATE_ON_START", true),
	}

	dsn := strings.TrimSpace(env("LCW_DB_DSN", ""))
	if dsn == "" {
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
	cfg.DBPassword = parsed.Passwd
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
		cfg.SessionTTL = 168 * time.Hour
	}
	if cfg.SessionMaxTTL <= 0 {
		cfg.SessionMaxTTL = 720 * time.Hour
	}
	if cfg.SessionMaxTTL < cfg.SessionTTL {
		cfg.SessionMaxTTL = cfg.SessionTTL
	}
	if cfg.RoflMaxKB < 1 {
		cfg.RoflMaxKB = 64 * 1024
	}
	if cfg.UploadConcurrency < 1 {
		cfg.UploadConcurrency = 1
	}
	if cfg.UploadConcurrency > 8 {
		cfg.UploadConcurrency = 8
	}
	// LCW_TRUST_PROXY=true 且未显式配置网段时，只信任本机回环地址，
	// 避免“打开开关就信任任意 X-Forwarded-For”的老问题。
	if cfg.TrustProxy && len(cfg.TrustedProxies) == 0 {
		cfg.TrustedProxies = loopbackPrefixes()
	}

	if _, err := time.LoadLocation(cfg.TimeZone); err != nil {
		return nil, fmt.Errorf("LCW_TIMEZONE 不合法: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validate 做启动前的安全自检；不满足时直接拒绝启动。
func (c *Config) validate() error {
	base, err := url.Parse(c.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" ||
		(base.Scheme != "http" && base.Scheme != "https") {
		return fmt.Errorf("LCW_BASE_URL 不合法（应形如 https://lol.example.com）: %q", c.BaseURL)
	}
	// 数据库口令为空意味着几乎肯定在用高权限空口令账号，生产必须拒绝。
	if c.DBPassword == "" && !c.DevMode {
		return errors.New("LCW_DB_PASSWORD 为空：请配置数据库口令；本地无口令开发请显式设置 LCW_DEV_MODE=true")
	}
	// 开发登录等价于“任何知道 QQ 号的人都能登录任意账号”，生产形态必须拒绝。
	if c.DevLogin && !isLoopbackHost(base.Hostname()) && (base.Scheme == "https" || !c.DevMode) {
		return errors.New("当前配置禁止开启开发登录（LCW_DEV_LOGIN=true）：仅限回环地址，或显式设置 LCW_DEV_MODE=true 的内网调试")
	}
	if c.CookieSecure && base.Scheme != "https" {
		return errors.New("LCW_COOKIE_SECURE=true 要求 LCW_BASE_URL 使用 https，否则浏览器不会回传会话 Cookie")
	}
	return nil
}

// Warnings 返回不致命但需要运维关注的安全配置提示。
func (c *Config) Warnings() []string {
	out := make([]string, 0, 4)
	if !c.CookieSecure {
		out = append(out, "会话 Cookie 未启用 Secure（LCW_BASE_URL 不是 https，或 LCW_COOKIE_SECURE=false）")
	}
	if len(c.TrustedProxies) == 0 {
		out = append(out, "未配置 LCW_TRUSTED_PROXIES：X-Forwarded-For 会被忽略，限流与审计按直连对端地址计")
	}
	if c.DevLogin {
		out = append(out, "开发登录已开启（/auth/dev），生产环境必须设置 LCW_DEV_LOGIN=false")
	}
	if c.MigrateOnStart {
		out = append(out, "启动时自动执行建表/升级（需要 DDL 权限）；生产可用 migrate 子命令并设置 LCW_MIGRATE_ON_START=false")
	}
	return out
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

// parseTrustedProxies 解析逗号分隔的 CIDR/单 IP 列表。
func parseTrustedProxies(raw string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			prefix, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, fmt.Errorf("LCW_TRUSTED_PROXIES 含非法网段 %q: %w", part, err)
			}
			out = append(out, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("LCW_TRUSTED_PROXIES 含非法地址 %q: %w", part, err)
		}
		addr = addr.Unmap()
		out = append(out, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return out, nil
}

// loopbackPrefixes 返回本机回环网段（127.0.0.0/8 与 ::1/128）。
func loopbackPrefixes() []netip.Prefix {
	return []netip.Prefix{
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
	}
}

// isLoopbackHost 判断 URL 主机名是否为回环地址。
func isLoopbackHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}
	addr, err := netip.ParseAddr(h)
	return err == nil && addr.Unmap().IsLoopback()
}

// loadDotEnv 读取 KEY=VALUE 形式的 .env 文件；已存在的环境变量优先。
// 文件不存在不是错误，方便容器里只用环境变量。
func loadDotEnv(path string) {
	// #nosec G304 -- path 是固定的 .env 路径，不来自请求参数
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
