package service

import (
	"strings"
	"testing"
)

// TestSafeRedirect 覆盖 L-03 的表驱动用例。
func TestSafeRedirect(t *testing.T) {
	cases := map[string]string{
		"":                    "/",
		"/":                   "/",
		"//evil.com":          "/",
		`/\evil.com`:          "/",
		"/a/../b":             "/",
		"../a":                "/",
		"https://evil.com":    "/",
		"http://evil.com/x":   "/",
		"/ok?x=1":             "/ok?x=1",
		"/group/abc":          "/group/abc",
		"/a\r\nSet-Cookie: x": "/",
	}
	for input, want := range cases {
		if got := SafeRedirect(input); got != want {
			t.Errorf("SafeRedirect(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestValidatePassword 检查 M-02 的新口令下限。
func TestValidatePassword(t *testing.T) {
	if err := validatePassword("123456789"); err == nil {
		t.Fatal("9 位密码应被拒绝")
	}
	if err := validatePassword("1234567890"); err != nil {
		t.Fatalf("10 位密码应通过: %v", err)
	}
	if err := validatePassword(strings.Repeat("a", 73)); err == nil {
		t.Fatal("超过 72 字节的密码应被拒绝")
	}
}

// TestMaskQQ 确认 QQ 脱敏遮盖中段（I-03/N-7.4）。
func TestMaskQQ(t *testing.T) {
	value := "123456789"
	masked := maskQQ(&value)
	if masked == nil || *masked != "12****89" {
		t.Fatalf("maskQQ = %v, want 12****89", masked)
	}
	short := "12345"
	if got := maskQQ(&short); got == nil || *got != "***" {
		t.Fatalf("短 QQ 脱敏 = %v", got)
	}
	if maskQQ(nil) != nil {
		t.Fatal("nil 应返回 nil")
	}
}

// TestRandomDigits 确认随机数字生成失败可见、结果长度固定。
func TestRandomDigits(t *testing.T) {
	value, err := randomDigits(8)
	if err != nil {
		t.Fatalf("randomDigits 失败: %v", err)
	}
	if len(value) != 8 {
		t.Fatalf("randomDigits 长度 = %d", len(value))
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			t.Fatalf("randomDigits 返回了非数字字符: %q", value)
		}
	}
}
