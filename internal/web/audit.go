package web

import (
	"log/slog"
	"strings"
)

// auditLog 输出结构化安全审计事件（L-10）。
//
// 约定：事件名使用 snake_case，字段只放排查所需的最小信息；
// 邮箱等个人信息先脱敏，避免日志本身成为泄露面。
func auditLog(event string, attrs ...any) {
	args := make([]any, 0, len(attrs)+2)
	args = append(args, "event", event)
	args = append(args, attrs...)
	slog.Info("audit", args...)
}

// maskEmail 只保留首字符与域名，例如 a***@example.com。
func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return "***"
	}
	local, domain := email[:at], email[at:]
	if len(local) <= 1 {
		return "*" + domain
	}
	return local[:1] + "***" + domain
}
