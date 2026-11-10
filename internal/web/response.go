package web

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/chyoumu114/lol-civil-war/internal/service"
)

// writeJSON 输出 JSON。
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("写入响应失败", "error", err)
	}
}

// writeMessage 输出 {"message": "..."}。
func writeMessage(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"message": message})
}

// writeValidation 输出带 errors 字段的校验错误。
func writeValidation(w http.ResponseWriter, field, message string) {
	writeJSON(w, 422, map[string]any{
		"message": message,
		"errors":  map[string][]string{field: {message}},
	})
}

// writeError 把业务异常或内部错误转成 JSON 响应。
func writeError(w http.ResponseWriter, err error) {
	var bizErr *service.Error
	if errors.As(err, &bizErr) {
		writeMessage(w, bizErr.Status, bizErr.Message)
		return
	}
	slog.Error("请求处理失败", "error", err)
	writeMessage(w, 500, "服务器开小差了，请稍后重试")
}

// notFoundJSON 返回统一的 JSON 404。
func notFoundJSON(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Not Found"
	}
	writeMessage(w, http.StatusNotFound, message)
}

// decodeJSON 读取请求体并解析；请求体过大返回 413。
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	return dec.Decode(dst)
}

// isUnsafeMethod 判断是否需要 CSRF 校验。
func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

// clientIP 取真实客户端 IP。
//
// 安全约定（对应 H-01）：
//   - 默认只相信 TCP 对端地址；
//   - 仅当对端命中可信代理网段时，才解析 X-Forwarded-For；
//   - X-Forwarded-For 必须从右向左取第一个“不可信”地址，
//     左侧可以任意伪造，不能直接取最左值。
func clientIP(r *http.Request, trusted []netip.Prefix) string {
	remote := hostFromAddrPort(r.RemoteAddr)
	remoteAddr, ok := parseIP(remote)
	if !ok || !isTrustedProxy(remoteAddr, trusted) {
		return remote
	}

	if ip := forwardedClientIP(trusted, r.Header.Values("X-Forwarded-For")); ip != "" {
		return ip
	}
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		if addr, ok := parseIP(strings.Trim(ip, "[]")); ok {
			return addr.String()
		}
	}
	return remote
}

// hostFromAddrPort 从 "ip:port" 中取出 IP；格式异常时原样返回并去掉方括号。
func hostFromAddrPort(addrPort string) string {
	if host, _, err := net.SplitHostPort(strings.TrimSpace(addrPort)); err == nil {
		return host
	}
	return strings.Trim(strings.TrimSpace(addrPort), "[]")
}

// forwardedClientIP 从右向左遍历 X-Forwarded-For，返回第一个不可信地址。
// 整条链只要出现无法解析的地址就直接放弃（返回空），避免把可疑值当作客户端地址。
func forwardedClientIP(trusted []netip.Prefix, values []string) string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if item := strings.TrimSpace(part); item != "" {
				items = append(items, item)
			}
		}
	}
	for i := len(items) - 1; i >= 0; i-- {
		addr, ok := parseIP(strings.Trim(items[i], "[]"))
		if !ok {
			return ""
		}
		if isTrustedProxy(addr, trusted) {
			continue
		}
		return addr.String()
	}
	return ""
}

func parseIP(raw string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func isTrustedProxy(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
