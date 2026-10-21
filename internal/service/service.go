// Package service 是应用层：组织业务规则、事务边界与领域调用。
//
// 分层约定：domain 只做纯计算；store 只做持久化；service 负责“取数据 → 算 → 落库”；
// web 层只做参数校验与序列化。
package service

import (
	"time"

	"github.com/chyoumu114/lol-civil-war/internal/config"
	"github.com/chyoumu114/lol-civil-war/internal/domain/rank"
	"github.com/chyoumu114/lol-civil-war/internal/domain/rofl"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

// App 聚合配置、存储与领域服务。
type App struct {
	Cfg     *config.Config
	St      *store.Store
	Loc     *time.Location
	Calc    *rank.PowerCalculator
	Parser  rofl.Parser
	DDragon *DDragon
	QQ      *QQClient

	// Now 便于测试注入时间。
	Now func() time.Time
}

// New 构造应用服务。
func New(cfg *config.Config, st *store.Store) *App {
	return &App{
		Cfg:     cfg,
		St:      st,
		Loc:     cfg.Location(),
		Calc:    rank.NewPowerCalculator(cfg.DivisionStep),
		DDragon: NewDDragon(cfg),
		QQ:      NewQQClient(cfg.QQAppID, cfg.QQAppKey, cfg.QQRedirectURI),
		Now:     func() time.Time { return time.Now() },
	}
}

// now 返回当前时间（UTC）。
func (a *App) now() time.Time { return a.Now().UTC() }

// iso 按业务时区输出 ISO8601（与旧接口一致，带 +08:00 偏移、无毫秒）。
func (a *App) iso(t time.Time) string {
	return t.In(a.Loc).Format("2006-01-02T15:04:05-07:00")
}

// isoPtr 输出可空时间。
func (a *App) isoPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := a.iso(*t)
	return &v
}

// Error 是业务异常，由 web 层转换为对应状态码的 JSON。
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

// NewError 构造业务异常。
func NewError(status int, message string) *Error {
	return &Error{Status: status, Message: message}
}

// ErrUnauthorized 未登录。
func ErrUnauthorized() *Error { return NewError(401, "请先登录 QQ") }

// ErrForbidden 无权限。
func ErrForbidden(message string) *Error { return NewError(403, message) }

// ErrNotFound 资源不存在。
func ErrNotFound(message string) *Error { return NewError(404, message) }

// ErrConflict 状态冲突。
func ErrConflict(message string) *Error { return NewError(409, message) }

// ErrInvalid 参数不合法（默认 422）。
func ErrInvalid(message string) *Error { return NewError(422, message) }
