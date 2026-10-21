package web

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
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

// clientIP 取真实客户端 IP（信任反向代理时使用 X-Forwarded-For）。
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				return ip
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}
	host := r.RemoteAddr
	if index := strings.LastIndex(host, ":"); index > 0 {
		host = host[:index]
	}
	return host
}
