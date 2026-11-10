package web

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

// TestClientIP 覆盖 H-01：只有可信代理的 X-Forwarded-For 才被采纳，且从右向左解析。
func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	cases := []struct {
		name   string
		remote string
		xff    string
		realIP string
		want   string
	}{
		{name: "无代理直连", remote: "8.8.8.8:1234", want: "8.8.8.8"},
		{name: "可信代理取最右不可信地址", remote: "10.0.0.5:1234", xff: "1.1.1.1, 10.0.0.5", want: "1.1.1.1"},
		{name: "伪造左端无效", remote: "10.0.0.5:1234", xff: "6.6.6.6, 5.5.5.5, 10.0.0.7", want: "5.5.5.5"},
		{name: "全部可信回退对端", remote: "10.0.0.5:1234", xff: "10.0.0.6, 10.0.0.5", want: "10.0.0.5"},
		{name: "非法链整条忽略", remote: "10.0.0.5:1234", xff: "not-an-ip, 10.0.0.5", want: "10.0.0.5"},
		{name: "不可信对端忽略 XFF", remote: "8.8.8.8:1234", xff: "1.1.1.1", realIP: "2.2.2.2", want: "8.8.8.8"},
		// N-7.3：不再回退 X-Real-IP，避免上游透传客户端自带值造成伪造。
		{name: "可信代理且无 XFF 忽略 X-Real-IP", remote: "10.0.0.5:1234", realIP: "9.9.9.9", want: "10.0.0.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tc.remote
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.realIP != "" {
				r.Header.Set("X-Real-IP", tc.realIP)
			}
			if got := clientIP(r, trusted); got != tc.want {
				t.Fatalf("clientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSessionPolicy 固定 H-02 的会话创建边界：只有登录/OAuth 引导路径允许新建会话。
func TestSessionPolicy(t *testing.T) {
	cases := []struct {
		path   string
		load   bool
		create bool
	}{
		{path: "/api/meta", load: true, create: false},
		{path: "/api/me", load: true, create: false},
		{path: "/api/groups/abc", load: true, create: false},
		{path: "/auth/qq", load: true, create: true},
		{path: "/auth/dev", load: true, create: true},
		{path: "/auth/qq/callback", load: true, create: false},
		{path: "/api/auth/login", load: true, create: false},
		{path: "/api/health", load: false, create: false},
		{path: "/api/champions", load: false, create: false},
		{path: "/civil-war", load: true, create: false},
		{path: "/civil-war/KEY/events/1", load: true, create: false},
		{path: "/assets/index-abc.js", load: false, create: false},
		{path: "/favicon.svg", load: false, create: false},
	}
	for _, tc := range cases {
		got := policyForPath(tc.path)
		if got.load != tc.load || got.create != tc.create {
			t.Errorf("policyForPath(%q) = %+v, want load=%v create=%v", tc.path, got, tc.load, tc.create)
		}
	}
}
