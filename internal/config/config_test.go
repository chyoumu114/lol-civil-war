package config

import (
	"strings"
	"testing"
)

// TestLoadRejectsUnsafeDefaults 覆盖 L-08/M-03/M-05 的启动自检。
func TestLoadRejectsUnsafeDefaults(t *testing.T) {
	base := map[string]string{
		"LCW_DB_DSN":        "",
		"LCW_BASE_URL":      "http://localhost:8080",
		"LCW_DB_USER":       "root",
		"LCW_DB_PASSWORD":   "",
		"LCW_DEV_MODE":      "false",
		"LCW_DEV_LOGIN":     "false",
		"LCW_COOKIE_SECURE": "auto",
	}
	cases := []struct {
		name    string
		env     map[string]string
		wantSub string
	}{
		{name: "空数据库口令", env: map[string]string{}, wantSub: "LCW_DB_PASSWORD 为空"},
		{name: "生产形态开发登录", env: map[string]string{
			"LCW_DB_PASSWORD": "secret",
			"LCW_BASE_URL":    "https://lol.example.com",
			"LCW_DEV_LOGIN":   "true",
			"LCW_DEV_MODE":    "false",
		}, wantSub: "禁止开启开发登录"},
		{name: "http 强制 Secure", env: map[string]string{
			"LCW_DB_PASSWORD":   "secret",
			"LCW_DEV_MODE":      "true",
			"LCW_COOKIE_SECURE": "true",
		}, wantSub: "要求 LCW_BASE_URL 使用 https"},
		{name: "非法 BaseURL", env: map[string]string{
			"LCW_DB_PASSWORD": "secret",
			"LCW_DEV_MODE":    "true",
			"LCW_BASE_URL":    "lol.example.com",
		}, wantSub: "LCW_BASE_URL 不合法"},
		{name: "非法可信代理网段", env: map[string]string{
			"LCW_DB_PASSWORD":     "secret",
			"LCW_DEV_MODE":        "true",
			"LCW_TRUSTED_PROXIES": "10.0.0.0/99",
		}, wantSub: "LCW_TRUSTED_PROXIES"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for key, value := range base {
				t.Setenv(key, value)
			}
			for key, value := range tc.env {
				t.Setenv(key, value)
			}
			// 显式置空可覆盖 base 中的值（t.Setenv 支持空串）。
			if tc.name == "空数据库口令" {
				t.Setenv("LCW_DB_PASSWORD", "")
			}
			_, err := Load()
			if err == nil {
				t.Fatalf("期望启动失败，实际成功")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("错误信息 = %q，期望包含 %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestLoadDevModeAllowsEmptyPassword 确认显式开发模式可以无口令启动。
func TestLoadDevModeAllowsEmptyPassword(t *testing.T) {
	t.Setenv("LCW_DB_DSN", "")
	t.Setenv("LCW_BASE_URL", "http://localhost:8080")
	t.Setenv("LCW_DB_PASSWORD", "")
	t.Setenv("LCW_DEV_MODE", "true")
	t.Setenv("LCW_DEV_LOGIN", "true")
	t.Setenv("LCW_COOKIE_SECURE", "auto")
	t.Setenv("LCW_TRUSTED_PROXIES", "")
	t.Setenv("LCW_TRUST_PROXY", "false")
	if _, err := Load(); err != nil {
		t.Fatalf("LCW_DEV_MODE=true 时应允许空口径启动: %v", err)
	}
}

// TestParseTrustedProxies 检查 CIDR 与单 IP 的归一化。
func TestParseTrustedProxies(t *testing.T) {
	prefixes, err := parseTrustedProxies("10.0.0.0/8, 127.0.0.1, ::1")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(prefixes) != 3 {
		t.Fatalf("前缀数量 = %d", len(prefixes))
	}
	if prefixes[0].String() != "10.0.0.0/8" || prefixes[1].String() != "127.0.0.1/32" || prefixes[2].String() != "::1/128" {
		t.Fatalf("解析结果异常: %v", prefixes)
	}
	if _, err := parseTrustedProxies("nope"); err == nil {
		t.Fatal("非法地址应报错")
	}
}
