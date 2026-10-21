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
	sessionCookieName = "lcw_session"
	csrfCookieName    = "XSRF-TOKEN"
	csrfHeaderName    = "X-XSRF-TOKEN"
)

type ctxKeySession struct{}

type sessionState struct {
	session store.Session
	isNew   bool
}

type sessionManager struct {
	st     *store.Store
	ttl    time.Duration
	secure bool
}

func newSessionManager(st *store.Store, ttl time.Duration, secure bool) *sessionManager {
	return &sessionManager{st: st, ttl: ttl, secure: secure}
}

// load 读取或创建会话，并写入响应 Cookie。
func (m *sessionManager) load(ctx context.Context, w http.ResponseWriter, r *http.Request) (*sessionState, error) {
	now := time.Now().UTC()
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		session, err := m.st.GetSession(ctx, cookie.Value)
		if err == nil {
			if now.Before(session.ExpiresAt) {
				// 只在剩余时间不足一半时续期，减少写库次数。
				if session.ExpiresAt.Sub(now) < m.ttl/2 {
					session.ExpiresAt = now.Add(m.ttl)
					if err := m.st.PutSession(ctx, session); err != nil {
						return nil, err
					}
				}
				state := &sessionState{session: session}
				m.setCookies(w, session)
				return state, nil
			}
			_ = m.st.DeleteSession(ctx, session.ID)
		} else if !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
	}

	session := store.Session{
		ID:        randomToken(40),
		CSRF:      randomToken(40),
		ExpiresAt: now.Add(m.ttl),
		CreatedAt: now,
	}
	if err := m.st.PutSession(ctx, session); err != nil {
		return nil, err
	}
	m.setCookies(w, session)
	return &sessionState{session: session, isNew: true}, nil
}

// setCookies 写入会话与 CSRF Cookie；同名 Cookie 以最后一次为准。
func (m *sessionManager) setCookies(w http.ResponseWriter, session store.Session) {
	maxAge := int(m.ttl.Seconds())
	expires := time.Now().Add(m.ttl)
	m.replaceCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Expires:  expires,
	})
	// XSRF-TOKEN 必须允许 JS 读取（axios 依赖它自动加请求头）。
	m.replaceCookie(w, &http.Cookie{
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

// clearCookies 让浏览器立即删除会话 Cookie。
func (m *sessionManager) clearCookies(w http.ResponseWriter) {
	for _, name := range []string{sessionCookieName, csrfCookieName} {
		m.replaceCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: name == sessionCookieName,
			Secure:   m.secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
		})
	}
}

// rotate 登录后重建会话（防止会话固定攻击）。
func (m *sessionManager) rotate(ctx context.Context, w http.ResponseWriter, r *http.Request, userID int64) error {
	now := time.Now().UTC()
	session := store.Session{
		ID:        randomToken(40),
		UserID:    &userID,
		CSRF:      randomToken(40),
		ExpiresAt: now.Add(m.ttl),
		CreatedAt: now,
	}
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

func (m *sessionManager) validateCSRF(r *http.Request) bool {
	state := sessionFrom(r.Context())
	if state == nil || state.session.ID == "" {
		return false
	}
	token := r.Header.Get(csrfHeaderName)
	if token == "" {
		token = r.URL.Query().Get("_token")
	}
	if token == "" {
		token = r.Header.Get("X-CSRF-TOKEN")
	}
	return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(state.session.CSRF)) == 1
}

// withSession 为 /api、/auth 请求建立会话上下文。
func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !needsSession(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		state, err := s.sessions.load(r.Context(), w, r)
		if err != nil {
			writeError(w, err)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeySession{}, state)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withCSRF 校验写请求的 CSRF Token。
func (s *Server) withCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !needsSession(r.URL.Path) || !isUnsafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		if !s.sessions.validateCSRF(r) {
			writeMessage(w, 419, "CSRF token mismatch.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// needsSession 只为真正需要登录态/CSRF 的路径创建会话，
// 避免健康检查、英雄库等匿名请求在数据库里留下大量无用会话。
func needsSession(path string) bool {
	if strings.HasPrefix(path, "/auth/") || strings.HasPrefix(path, "/api/auth/") {
		return true
	}
	// /api/meta 是前端入口页用来“先取 XSRF Cookie 再发 resolve”的引导请求，
	// 必须建立会话，否则第一次 POST /api/groups/resolve 会 419。
	if path == "/api/me" || path == "/api/meta" || path == "/api/groups/resolve" {
		return true
	}
	return strings.HasPrefix(path, "/api/groups/")
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

func randomToken(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand 不可用: " + err.Error())
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf)
}
