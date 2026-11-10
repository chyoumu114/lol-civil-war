package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/chyoumu114/lol-civil-war/internal/store"
)

const (
	sessionCookieName       = "lcw_session"
	secureSessionCookieName = "__Host-lcw_session"
	csrfCookieName          = "XSRF-TOKEN"
	csrfHeaderName          = "X-XSRF-TOKEN"
	csrfAltHeaderName       = "X-CSRF-TOKEN"

	// 匿名会话只用于存放 QQ OAuth state 等临时状态，生命周期必须很短（H-02）。
	anonSessionIdleTTL = 2 * time.Hour
	anonSessionMaxTTL  = 4 * time.Hour
)

type ctxKeySession struct{}

type sessionState struct {
	session store.Session
	isNew   bool
}

type sessionManager struct {
	st      *store.Store
	idleTTL time.Duration
	maxTTL  time.Duration
	secure  bool
}

func newSessionManager(st *store.Store, idleTTL, maxTTL time.Duration, secure bool) *sessionManager {
	// 测试或旧配置可能直接构造 Config（不走 Load），这里做一次兜底，避免 0 值让会话立即过期。
	if idleTTL <= 0 {
		idleTTL = 168 * time.Hour
	}
	if maxTTL < idleTTL {
		maxTTL = idleTTL
	}
	return &sessionManager{st: st, idleTTL: idleTTL, maxTTL: maxTTL, secure: secure}
}

// sessionPolicy 描述某条路径对会话的需求（H-02）。
//
//   - load：有 Cookie 时读取会话（只读请求也能识别已登录用户），但绝不新建；
//   - create：没有会话时允许显式创建（仅登录/OAuth 引导路径需要）。
//
// 业务 API 一律 load-only：匿名请求即便没有 Cookie 也不会在数据库里留下会话行。
type sessionPolicy struct {
	load   bool
	create bool
}

func policyForPath(path string) sessionPolicy {
	switch {
	case path == "/api/meta" || path == "/api/me":
		return sessionPolicy{load: true}
	case path == "/auth/qq" || path == "/auth/dev":
		// 发起 QQ 登录/开发登录需要服务端保存 OAuth state。
		return sessionPolicy{load: true, create: true}
	case strings.HasPrefix(path, "/auth/"), strings.HasPrefix(path, "/api/auth/"):
		return sessionPolicy{load: true}
	case strings.HasPrefix(path, "/api/groups/"):
		return sessionPolicy{load: true}
	}
	return sessionPolicy{}
}

// sessionFloodError 表示匿名会话创建触发了 IP 限流。
type sessionFloodError struct{ retry int }

func (e *sessionFloodError) Error() string { return "session creation rate limited" }

// load 读取会话；create 为 true 且无可复用会话时才新建。
// createGuard 在真正建库前调用（用于 IP 限流），返回 false 时返回 *sessionFloodError。
// 未创建且无会话时返回 (nil, nil)，由调用方按匿名请求继续处理。
func (m *sessionManager) load(ctx context.Context, w http.ResponseWriter, r *http.Request, create bool, createGuard func() (bool, int)) (*sessionState, error) {
	now := time.Now().UTC()
	for _, name := range m.readCookieNames() {
		cookie, err := r.Cookie(name)
		if err != nil || cookie.Value == "" {
			continue
		}
		session, err := m.st.GetSession(ctx, cookie.Value)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			return nil, err
		}
		if sessionExpired(session, now) {
			_ = m.st.DeleteSession(ctx, session.ID)
			continue
		}
		// 只续期空闲时间；绝对寿命（AbsoluteExpiresAt）不随活动延长。
		if session.ExpiresAt.Sub(now) < m.idleTTL/2 {
			session.ExpiresAt = now.Add(m.idleTTL)
			if err := m.st.PutSession(ctx, session); err != nil {
				return nil, err
			}
		}
		m.setCookies(w, session)
		return &sessionState{session: session}, nil
	}
	if !create {
		return nil, nil
	}
	if createGuard != nil {
		if allowed, retry := createGuard(); !allowed {
			return nil, &sessionFloodError{retry: retry}
		}
	}

	session := m.newSession(now, nil)
	if err := m.st.PutSession(ctx, session); err != nil {
		return nil, err
	}
	m.setCookies(w, session)
	return &sessionState{session: session, isNew: true}, nil
}

// newSession 生成会话；未登录（匿名）时使用更短的空闲/绝对寿命。
func (m *sessionManager) newSession(now time.Time, userID *int64) store.Session {
	idle, maximal := m.idleTTL, m.maxTTL
	if userID == nil {
		idle, maximal = anonSessionIdleTTL, anonSessionMaxTTL
	}
	return store.Session{
		ID:                randomToken(40),
		UserID:            userID,
		CSRF:              randomToken(40),
		ExpiresAt:         now.Add(idle),
		AbsoluteExpiresAt: now.Add(maximal),
		CreatedAt:         now,
	}
}

// sessionExpired 同时检查空闲超时与绝对寿命。
func sessionExpired(session store.Session, now time.Time) bool {
	if !session.ExpiresAt.IsZero() && !now.Before(session.ExpiresAt) {
		return true
	}
	if !session.AbsoluteExpiresAt.IsZero() && !now.Before(session.AbsoluteExpiresAt) {
		return true
	}
	return false
}

// sessionCookieName 在 HTTPS 场景使用 __Host- 前缀（CWE-614/1004 加固）。
func (m *sessionManager) sessionCookieName() string {
	if m.secure {
		return secureSessionCookieName
	}
	return sessionCookieName
}

// readCookieNames 读取顺序：优先 __Host-，再兼容旧 Cookie 名。
func (m *sessionManager) readCookieNames() []string {
	if m.secure {
		return []string{secureSessionCookieName, sessionCookieName}
	}
	return []string{sessionCookieName}
}

// setCookies 写入会话与 CSRF Cookie；同名 Cookie 以最后一次为准。
func (m *sessionManager) setCookies(w http.ResponseWriter, session store.Session) {
	now := time.Now()
	expires := session.ExpiresAt
	if expires.IsZero() || !expires.After(now) {
		expires = now.Add(m.idleTTL)
	}
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	m.replaceCookie(w, &http.Cookie{ // #nosec G124 -- Secure 按部署配置、HttpOnly 已开启、SameSite=Lax
		Name:     m.sessionCookieName(),
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Expires:  expires,
	})
	// XSRF-TOKEN 必须允许 JS 读取（axios 依赖它自动加请求头）。
	m.replaceCookie(w, &http.Cookie{ // #nosec G124 -- XSRF-TOKEN 必须可被前端 JS 读取（双提交 Cookie 设计）
		Name:     csrfCookieName,
		Value:    session.CSRF,
		Path:     "/",
		HttpOnly: false,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Expires:  expires,
	})
}

// ensureCSRFCookie 为“还没有会话”的入口请求下发无状态的 CSRF Token（双提交 Cookie）。
//
// 这样 /api/meta 与 SPA 页面不再需要通过创建数据库会话来发放 Token（H-02），
// 同时保留 axios 直接读取 Cookie 的既有约定。
func (m *sessionManager) ensureCSRFCookie(w http.ResponseWriter, r *http.Request) {
	if sessionFrom(r.Context()) != nil {
		// 已有会话：withSession 已经下发与会话绑定的 Token。
		return
	}
	for _, header := range w.Header().Values("Set-Cookie") {
		if strings.HasPrefix(header, csrfCookieName+"=") {
			return
		}
	}
	if _, err := r.Cookie(csrfCookieName); err == nil {
		return
	}
	expires := time.Now().Add(anonSessionIdleTTL)
	m.replaceCookie(w, &http.Cookie{ // #nosec G124 -- 无状态 XSRF-TOKEN 必须可被前端 JS 读取
		Name:     csrfCookieName,
		Value:    randomToken(40),
		Path:     "/",
		HttpOnly: false,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(time.Until(expires).Seconds()),
		Expires:  expires,
	})
}

func (m *sessionManager) replaceCookie(w http.ResponseWriter, cookie *http.Cookie) {
	headers := w.Header().Values("Set-Cookie")
	kept := make([]string, 0, len(headers))
	prefix := cookie.Name + "="
	for _, header := range headers {
		if strings.HasPrefix(header, prefix) {
			continue
		}
		kept = append(kept, header)
	}
	w.Header()["Set-Cookie"] = kept
	http.SetCookie(w, cookie)
}

// clearCookies 让浏览器立即删除会话 Cookie（同时清理新旧两种名字）。
func (m *sessionManager) clearCookies(w http.ResponseWriter) {
	names := []string{sessionCookieName, csrfCookieName}
	if m.secure {
		names = append(names, secureSessionCookieName)
	}
	for _, name := range names {
		m.replaceCookie(w, &http.Cookie{ // #nosec G124 -- 删除用 Cookie（MaxAge<0），属性与下发时一致
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: name != csrfCookieName,
			Secure:   m.secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
		})
	}
}

// rotate 登录后重建会话（防止会话固定攻击），并重置绝对寿命。
func (m *sessionManager) rotate(ctx context.Context, w http.ResponseWriter, r *http.Request, userID int64) error {
	now := time.Now().UTC()
	session := m.newSession(now, &userID)
	if err := m.st.PutSession(ctx, session); err != nil {
		return err
	}
	if old := sessionFrom(r.Context()); old != nil && old.session.ID != session.ID {
		_ = m.st.DeleteSession(ctx, old.session.ID)
	}
	m.setCookies(w, session)
	if old := sessionFrom(r.Context()); old != nil {
		old.session = session
	}
	return nil
}

// logout 删除会话并清 Cookie。
func (m *sessionManager) logout(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if old := sessionFrom(r.Context()); old != nil && old.session.ID != "" {
		if err := m.st.DeleteSession(ctx, old.session.ID); err != nil {
			return err
		}
	}
	m.clearCookies(w)
	return nil
}

// validateCSRF 校验写请求。
//
//   - 只接受请求头（删除 query `_token` 回退，L-04）；
//   - 有会话时与会话内 Token 比对；
//   - 无会话时使用双提交 Cookie（Cookie 与会话无关，但由入口页下发）。
func (m *sessionManager) validateCSRF(r *http.Request) bool {
	// 现代浏览器会带 Sec-Fetch-Site；跨站写请求直接拒绝。
	if site := strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")); site != "" && site != "same-origin" && site != "none" {
		return false
	}
	token := r.Header.Get(csrfHeaderName)
	if token == "" {
		token = r.Header.Get(csrfAltHeaderName)
	}
	if token == "" {
		return false
	}
	if state := sessionFrom(r.Context()); state != nil && state.session.CSRF != "" {
		if subtle.ConstantTimeCompare([]byte(token), []byte(state.session.CSRF)) == 1 {
			return true
		}
	}
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(cookie.Value)) == 1
}

// withSession 为需要会话的路径建立会话上下文。
func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		policy := policyForPath(r.URL.Path)
		if !policy.load {
			next.ServeHTTP(w, r)
			return
		}
		ip := clientIP(r, s.Cfg.TrustedProxies)
		// 匿名会话创建做独立的 IP 限流，堵住“换 Cookie 就换会话”的无界创建（H-02）。
		state, err := s.sessions.load(r.Context(), w, r, policy.create, func() (bool, int) {
			return s.limiter.allow("session:"+ip, 120, time.Minute)
		})
		if err != nil {
			var flood *sessionFloodError
			if errors.As(err, &flood) {
				w.Header().Set("Retry-After", itoa(flood.retry))
				auditLog("session_rate_limited", "ip", ip, "path", r.URL.Path)
				writeMessage(w, http.StatusTooManyRequests, "请求过于频繁，请稍后重试")
				return
			}
			writeError(w, err)
			return
		}
		if state == nil {
			next.ServeHTTP(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeySession{}, state)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withCSRF 校验写请求的 CSRF Token，并为入口请求下发无状态 Token。
func (s *Server) withCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldIssueCSRFCookie(r) {
			s.sessions.ensureCSRFCookie(w, r)
		}
		if isUnsafeMethod(r.Method) && !s.sessions.validateCSRF(r) {
			auditLog("csrf_reject", "ip", clientIP(r, s.Cfg.TrustedProxies),
				"method", r.Method, "path", r.URL.Path)
			writeMessage(w, 419, "CSRF token mismatch.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// shouldIssueCSRFCookie 只给“入口请求”下发无状态 Token：
// /api/meta、/api/me 以及 SPA/HTML 页面。
func shouldIssueCSRFCookie(r *http.Request) bool {
	if isUnsafeMethod(r.Method) {
		return false
	}
	switch {
	case r.URL.Path == "/api/meta" || r.URL.Path == "/api/me":
		return true
	case strings.HasPrefix(r.URL.Path, "/api/"), strings.HasPrefix(r.URL.Path, "/auth/"):
		return false
	case r.URL.Path == "/up":
		return false
	default:
		return true
	}
}

func sessionFrom(ctx context.Context) *sessionState {
	state, _ := ctx.Value(ctxKeySession{}).(*sessionState)
	return state
}

func userIDFrom(r *http.Request) (int64, bool) {
	state := sessionFrom(r.Context())
	if state == nil || state.session.UserID == nil {
		return 0, false
	}
	return *state.session.UserID, true
}

// randomToken 生成 URL 安全的随机串；crypto/rand 不可用时直接 panic（fail-closed）。
func randomToken(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const maxAcceptable = 256 - (256 % len(alphabet))
	buf := make([]byte, n)
	var b [1]byte
	for i := 0; i < n; {
		if _, err := rand.Read(b[:]); err != nil {
			panic("crypto/rand 不可用: " + err.Error())
		}
		if int(b[0]) >= maxAcceptable {
			continue // 拒绝采样，消除取模偏差
		}
		buf[i] = alphabet[int(b[0])%len(alphabet)]
		i++
	}
	return string(buf)
}
