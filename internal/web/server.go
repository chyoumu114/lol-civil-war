package web

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chyoumu114/lol-civil-war/internal/config"
	"github.com/chyoumu114/lol-civil-war/internal/service"
	"github.com/chyoumu114/lol-civil-war/internal/store"
	"github.com/chyoumu114/lol-civil-war/internal/webui"
)

// Server 承载 HTTP 路由、会话与静态资源。
type Server struct {
	App     *service.App
	Cfg     *config.Config
	Version string

	sessions *sessionManager
	limiter  *limiter
	ui       fs.FS
}

// New 构造 HTTP 服务。
func New(app *service.App, cfg *config.Config, version string) *Server {
	return &Server{
		App:      app,
		Cfg:      cfg,
		Version:  version,
		sessions: newSessionManager(app.St, cfg.SessionTTL, cfg.CookieSecure),
		limiter:  newLimiter(),
		ui:       webui.Dist(),
	}
}

type authedHandler func(w http.ResponseWriter, r *http.Request, user store.User)
type groupHandler func(w http.ResponseWriter, r *http.Request, group store.Group, member store.Member)
type eventHandler func(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member)

// Handler 组装全部中间件与路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.routes(mux)

	var handler http.Handler = mux
	handler = s.withCSRF(handler)
	handler = s.withSession(handler)
	handler = s.withSecurityHeaders(handler)
	handler = s.withLogging(handler)
	handler = s.withRecover(handler)
	return handler
}

func (s *Server) routes(mux *http.ServeMux) {
	authLimit := s.limit(120, time.Minute, func(r *http.Request) string {
		return "auth:" + clientIP(r, s.Cfg.TrustProxy)
	})
	resolveLimit := s.limit(60, time.Minute, func(r *http.Request) string {
		return "resolve:" + clientIP(r, s.Cfg.TrustProxy)
	})

	// 公共接口
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/meta", s.handleMeta)
	mux.HandleFunc("GET /api/power/calculate", s.handlePower)
	mux.HandleFunc("GET /api/me", s.handleMe)
	mux.HandleFunc("GET /api/champions/library", s.handleChampionLibrary)
	mux.HandleFunc("GET /api/champions", s.handleChampionNames)
	mux.HandleFunc("POST /api/groups/resolve", resolveLimit(s.handleGroupResolve))

	// 登录
	mux.HandleFunc("GET /auth/qq", authLimit(s.handleQQRedirect))
	mux.HandleFunc("GET /auth/qq/callback", authLimit(s.handleQQCallback))
	mux.HandleFunc("GET /api/auth/callback", authLimit(s.handleQQCallback))
	mux.HandleFunc("POST /api/auth/register", authLimit(s.handleEmailRegister))
	mux.HandleFunc("POST /api/auth/login", authLimit(s.handleEmailLogin))
	mux.HandleFunc("GET /api/auth/pending", authLimit(s.handleQQPending))
	mux.HandleFunc("POST /api/auth/qq/complete", authLimit(s.handleQQComplete))
	mux.HandleFunc("GET /auth/qq/bind", authLimit(s.handleQQBindRedirect))
	mux.HandleFunc("GET /auth/dev", authLimit(s.handleDevLogin))
	mux.HandleFunc("POST /auth/logout", s.handleLogout)

	// 群内业务
	mux.HandleFunc("GET /api/groups/{group}", s.groupScoped(s.handleGroupShow))
	mux.HandleFunc("PUT /api/groups/{group}/profile", s.groupScoped(s.handleProfileUpdate))
	mux.HandleFunc("GET /api/groups/{group}/events", s.groupScoped(s.handleEventList))
	mux.HandleFunc("POST /api/groups/{group}/events", s.groupScoped(s.handleEventCreate))
	mux.HandleFunc("GET /api/groups/{group}/ranking", s.groupScoped(s.handleRanking))
	mux.HandleFunc("GET /api/groups/{group}/players/{member}", s.groupScoped(s.handlePlayerAnalysis))

	// 组织内业务
	mux.HandleFunc("GET /api/groups/{group}/events/{event}", s.eventScoped(s.handleEventShow))
	mux.HandleFunc("PUT /api/groups/{group}/events/{event}", s.eventScoped(s.handleEventUpdate))
	mux.HandleFunc("POST /api/groups/{group}/events/{event}/end", s.eventScoped(s.handleEventEnd))
	mux.HandleFunc("POST /api/groups/{group}/events/{event}/registrations", s.eventScoped(s.handleRegistrationSave))
	mux.HandleFunc("PUT /api/groups/{group}/events/{event}/registrations/{member}", s.eventScoped(s.handleRegistrationUpdate))
	mux.HandleFunc("DELETE /api/groups/{group}/events/{event}/registrations/{member}", s.eventScoped(s.handleRegistrationDelete))
	mux.HandleFunc("POST /api/groups/{group}/events/{event}/registrations/{member}/spectator", s.eventScoped(s.handleRegistrationSpectator))
	mux.HandleFunc("POST /api/groups/{group}/events/{event}/spectators", s.eventScoped(s.handleEventSpectator))
	mux.HandleFunc("POST /api/groups/{group}/events/{event}/matches/auto", s.eventScoped(s.handleAutoMatch))
	mux.HandleFunc("POST /api/groups/{group}/events/{event}/roster", s.eventScoped(s.handleRosterApply))
	mux.HandleFunc("POST /api/groups/{group}/events/{event}/matches", s.eventScoped(s.handleMatchCreate))
	mux.HandleFunc("GET /api/groups/{group}/events/{event}/matches/{gameNo}", s.eventScoped(s.handleMatchShow))
	mux.HandleFunc("PUT /api/groups/{group}/events/{event}/matches/{gameNo}", s.eventScoped(s.handleMatchSave))
	mux.HandleFunc("DELETE /api/groups/{group}/events/{event}/matches/{gameNo}", s.eventScoped(s.handleMatchDelete))
	mux.HandleFunc("POST /api/groups/{group}/events/{event}/matches/{gameNo}/ratings", s.eventScoped(s.handleMatchRate))
	mux.HandleFunc("POST /api/groups/{group}/events/{event}/matches/{gameNo}/rofl", s.eventScoped(s.handleRoflUpload))

	// 未匹配的 API / 认证路径返回 JSON 404
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { notFoundJSON(w, "Not Found") })
	mux.HandleFunc("/auth/", func(w http.ResponseWriter, r *http.Request) { notFoundJSON(w, "Not Found") })

	// 静态资源与 SPA
	mux.HandleFunc("GET /up", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", s.handleStatic)
}

// authed 要求登录，注入当前账号。
func (s *Server) authed(fn authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFrom(r)
		if !ok {
			writeError(w, service.ErrUnauthorized())
			return
		}
		user, err := s.App.St.UserByID(r.Context(), userID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, service.ErrUnauthorized())
				return
			}
			writeError(w, err)
			return
		}
		fn(w, r, user)
	}
}

// groupScoped 解析群、创建成员资料并限流。
func (s *Server) groupScoped(fn groupHandler) http.HandlerFunc {
	return s.authed(func(w http.ResponseWriter, r *http.Request, user store.User) {
		allowed, retry := s.limiter.allow("user:"+itoa64(user.ID), 120, time.Minute)
		if !allowed {
			w.Header().Set("Retry-After", itoa(retry))
			writeMessage(w, http.StatusTooManyRequests, "Too Many Attempts.")
			return
		}
		group, err := s.App.ResolveGroup(r.Context(), r.PathValue("group"))
		if err != nil {
			writeError(w, err)
			return
		}
		member, err := s.App.EnsureMember(r.Context(), group, user)
		if err != nil {
			writeError(w, err)
			return
		}
		fn(w, r, group, member)
	})
}

// eventScoped 在 groupScoped 基础上解析组织。
func (s *Server) eventScoped(fn eventHandler) http.HandlerFunc {
	return s.groupScoped(func(w http.ResponseWriter, r *http.Request, group store.Group, member store.Member) {
		eventID, _ := strconv.ParseInt(r.PathValue("event"), 10, 64)
		event, err := s.App.St.EventByGroupAndID(r.Context(), group.ID, eventID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, service.ErrNotFound("组织不存在或不属于当前群"))
				return
			}
			writeError(w, err)
			return
		}
		fn(w, r, group, event, member)
	})
}

// memberFromPath 解析路径里的成员 id。
func (s *Server) memberFromPath(r *http.Request, group store.Group) (store.Member, error) {
	memberID, _ := strconv.ParseInt(r.PathValue("member"), 10, 64)
	member, err := s.App.St.MemberByID(r.Context(), group.ID, memberID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Member{}, service.ErrNotFound("找不到该群成员")
		}
		return store.Member{}, err
	}
	return member, nil
}

func (s *Server) gameNoFromPath(r *http.Request) int {
	value, _ := strconv.Atoi(r.PathValue("gameNo"))
	return value
}

// ---- 中间件 ----

func (s *Server) withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic", "path", r.URL.Path, "value", rec)
				writeMessage(w, http.StatusInternalServerError, "服务器开小差了，请稍后重试")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		if !strings.HasPrefix(r.URL.Path, "/assets/") && !strings.HasPrefix(r.URL.Path, "/champions/") {
			slog.Info("http", "method", r.Method, "path", r.URL.Path,
				"status", recorder.status, "duration", time.Since(start).Round(time.Millisecond).String())
		}
	})
}

func (s *Server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// ---- 静态资源 ----

// handleStatic 优先返回前端产物，未命中时回退 index.html（SPA 路由）。
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		notFoundJSON(w, "Not Found")
		return
	}

	if strings.HasPrefix(r.URL.Path, "/champions/") {
		s.serveChampionIcon(w, r)
		return
	}

	clean := path.Clean("/" + r.URL.Path)
	relative := strings.TrimPrefix(clean, "/")
	if relative != "" && s.fileExists(relative) {
		s.serveEmbeddedFile(w, r, relative)
		return
	}
	// 带扩展名却没命中，视为静态资源 404，避免把 404 变成 index.html。
	if path.Ext(relative) != "" {
		http.NotFound(w, r)
		return
	}
	s.serveIndex(w, r)
}

func (s *Server) fileExists(name string) bool {
	info, err := fs.Stat(s.ui, name)
	return err == nil && !info.IsDir()
}

func (s *Server) serveEmbeddedFile(w http.ResponseWriter, r *http.Request, name string) {
	data, err := fs.ReadFile(s.ui, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	info, err := fs.Stat(s.ui, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	http.ServeContent(w, r, name, info.ModTime(), bytes.NewReader(data))
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(s.ui, "index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(placeholderHTML))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

// serveChampionIcon 从可选目录读取英雄图标；缺失时返回 404，前端会回落 CDN。
func (s *Server) serveChampionIcon(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.URL.Path)
	if name == "." || name == "/" || name == "" || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(s.Cfg.ChampionIconDir, name)
	file, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=2592000")
	http.ServeContent(w, r, name, info.ModTime(), file)
}

const placeholderHTML = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>LOL 内战工具</title>
<style>body{font-family:system-ui,sans-serif;background:#0b1020;color:#e5e7eb;display:grid;place-items:center;height:100vh;margin:0}
main{max-width:560px;padding:32px;border:1px solid #26304a;border-radius:16px;background:#111830}
code{background:#0b1020;padding:2px 6px;border-radius:6px;color:#93c5fd}h1{font-size:20px}</style></head>
<body><main><h1>前端尚未构建</h1>
<p>后端已经启动，但没有找到前端产物。</p>
<p>请执行 <code>make web</code>（或 <code>cd web &amp;&amp; npm ci &amp;&amp; npm run build</code>）后重新启动服务。</p>
</main></body></html>`

func (s *Server) healthPayload(ctx context.Context) (map[string]any, bool) {
	database := "ok"
	if err := s.App.St.Ping(ctx); err != nil {
		database = "error: " + err.Error()
	}
	ok := database == "ok"
	return map[string]any{
		"ok":       ok,
		"database": database,
		"redis":    "disabled",
		"time":     s.App.Now().In(s.App.Loc).Format("2006-01-02T15:04:05-07:00"),
		"version":  s.Version,
	}, ok
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	payload, ok := s.healthPayload(ctx)
	status := http.StatusOK
	if !ok {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, payload)
}

// itoa 小工具（避免每个文件都导入 strconv）。
func itoa(v int) string     { return strconv.Itoa(v) }
func itoa64(v int64) string { return strconv.FormatInt(v, 10) }
