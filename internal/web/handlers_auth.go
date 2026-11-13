package web

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/chyoumu114/lol-civil-war/internal/service"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

const (
	// 账号级登录退避（M-01/M-02）：同一邮箱连续失败 10 次后锁 15 分钟。
	// 计数只发生在失败时，成功登录会清零，避免正常用户被误伤。
	loginFailLimit  = 10
	loginFailWindow = 15 * time.Minute

	// 注册侧限制（N-2）：单 IP 10 次/10 分钟；同一邮箱 5 次/小时。
	// 同一邮箱的计数只用于给探测者增加摩擦，无法根治“换邮箱批量探测”，
	// 彻底方案需要邮箱验证码流程。
	registerIPLimit     = 10
	registerIPWindow    = 10 * time.Minute
	registerEmailLimit  = 5
	registerEmailWindow = time.Hour

	// 单 IP 登录失败总量（跨账号），配合邮箱级退避抵御撞库（V-06）。
	loginIPFailLimit  = 50
	loginIPFailWindow = 10 * time.Minute
)

type emailAuthPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginKey 以邮箱为键，不区分大小写；不存在的邮箱也会计数，
// 因此 429 本身不会成为账号存在性信号。
func loginKey(email string) string {
	return "login:" + strings.ToLower(strings.TrimSpace(email))
}

// loginBlocked 判断当前邮箱是否处于退避期。
func (s *Server) loginBlocked(w http.ResponseWriter, r *http.Request, email string) bool {
	if strings.TrimSpace(email) == "" {
		return false
	}
	blocked, retry := s.limiter.blocked(loginKey(email), loginFailLimit, loginFailWindow)
	if !blocked {
		return false
	}
	w.Header().Set("Retry-After", itoa(retry))
	auditLog("login_blocked", "email", maskEmail(email), "ip", clientIP(r, s.Cfg.TrustedProxies))
	writeMessage(w, http.StatusTooManyRequests, "尝试次数过多，请稍后再试")
	return true
}

// handleEmailRegister 邮箱注册并直接登录。
func (s *Server) handleEmailRegister(w http.ResponseWriter, r *http.Request) {
	var payload emailAuthPayload
	if err := decodeJSON(w, r, &payload, 64<<10); err != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "请求格式不正确")
		return
	}
	ip := clientIP(r, s.Cfg.TrustedProxies)
	if allowed, retry := s.limiter.allow("register-ip:"+ip, registerIPLimit, registerIPWindow); !allowed {
		w.Header().Set("Retry-After", itoa(retry))
		auditLog("register_rate_limited", "ip", ip)
		writeMessage(w, http.StatusTooManyRequests, "操作过于频繁，请稍后再试")
		return
	}
	email := strings.TrimSpace(payload.Email)
	if email != "" {
		if blocked, retry := s.limiter.blocked(registerKey(email), registerEmailLimit, registerEmailWindow); blocked {
			w.Header().Set("Retry-After", itoa(retry))
			auditLog("register_blocked", "email", maskEmail(email), "ip", ip)
			writeMessage(w, http.StatusTooManyRequests, "操作过于频繁，请稍后再试")
			return
		}
	}
	user, err := s.App.RegisterEmail(r.Context(), payload.Email, payload.Password)
	if err != nil {
		// 409 说明邮箱已存在；计入该邮箱的探测次数并审计（N-2/N-5）。
		var bizErr *service.Error
		if email != "" && errors.As(err, &bizErr) && bizErr.Status == http.StatusConflict {
			s.limiter.hit(registerKey(email), registerEmailWindow)
			auditLog("register_conflict", "email", maskEmail(email), "ip", ip)
		}
		writeError(w, err)
		return
	}
	auditLog("register_success", "userNo", user.UserNo, "ip", ip)
	s.loginJSON(w, r, user)
}

// registerKey 以邮箱为键，给同一邮箱的重复探测增加摩擦。
func registerKey(email string) string {
	return "register:" + strings.ToLower(strings.TrimSpace(email))
}

// handleEmailLogin 邮箱密码登录。
func (s *Server) handleEmailLogin(w http.ResponseWriter, r *http.Request) {
	var payload emailAuthPayload
	if err := decodeJSON(w, r, &payload, 64<<10); err != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "请求格式不正确")
		return
	}
	if s.loginBlocked(w, r, payload.Email) {
		return
	}
	ip := clientIP(r, s.Cfg.TrustedProxies)
	if blocked, retry := s.limiter.blocked("login-ip:"+ip, loginIPFailLimit, loginIPFailWindow); blocked {
		w.Header().Set("Retry-After", itoa(retry))
		auditLog("login_ip_blocked", "ip", ip)
		writeMessage(w, http.StatusTooManyRequests, "尝试次数过多，请稍后再试")
		return
	}
	user, err := s.App.LoginEmail(r.Context(), payload.Email, payload.Password)
	if err != nil {
		s.limiter.hit(loginKey(payload.Email), loginFailWindow)
		s.limiter.hit("login-ip:"+ip, loginIPFailWindow)
		auditLog("login_failed", "email", maskEmail(payload.Email), "ip", ip)
		writeError(w, err)
		return
	}
	s.limiter.clear(loginKey(payload.Email))
	auditLog("login_success", "userNo", user.UserNo, "ip", clientIP(r, s.Cfg.TrustedProxies))
	s.loginJSON(w, r, user)
}

// handleQQRedirect 发起 QQ 快捷登录（仅配置了 QQ 互联时可用）。
func (s *Server) handleQQRedirect(w http.ResponseWriter, r *http.Request) {
	if !s.App.QQ.Configured() {
		writeMessage(w, http.StatusServiceUnavailable,
			"QQ 互联未配置：请设置环境变量 LCW_QQ_APP_ID / LCW_QQ_APP_KEY，或使用邮箱登录")
		return
	}
	state := sessionFrom(r.Context())
	if state == nil {
		writeMessage(w, http.StatusInternalServerError, "会话不可用，请刷新后重试")
		return
	}
	state.session.OAuthState = randomToken(40)
	state.session.OAuthRedirect = service.SafeRedirect(r.URL.Query().Get("redirect"))
	state.session.QQBindUserID = nil
	if err := s.App.St.PutSession(r.Context(), state.session); err != nil {
		writeError(w, err)
		return
	}
	http.Redirect(w, r, s.App.QQ.AuthorizeURL(state.session.OAuthState), http.StatusFound)
}

// handleQQBindRedirect 已登录用户发起“绑定 QQ 快捷登录”。
func (s *Server) handleQQBindRedirect(w http.ResponseWriter, r *http.Request) {
	if !s.App.QQ.Configured() {
		writeMessage(w, http.StatusServiceUnavailable,
			"QQ 互联未配置：请设置环境变量 LCW_QQ_APP_ID / LCW_QQ_APP_KEY")
		return
	}
	userID, ok := userIDFrom(r)
	if !ok {
		writeError(w, service.ErrUnauthorized())
		return
	}
	state := sessionFrom(r.Context())
	if state == nil {
		writeMessage(w, http.StatusInternalServerError, "会话不可用，请刷新后重试")
		return
	}
	state.session.OAuthState = randomToken(40)
	state.session.OAuthRedirect = service.SafeRedirect(r.URL.Query().Get("redirect"))
	bindID := userID
	state.session.QQBindUserID = &bindID
	if err := s.App.St.PutSession(r.Context(), state.session); err != nil {
		writeError(w, err)
		return
	}
	http.Redirect(w, r, s.App.QQ.AuthorizeURL(state.session.OAuthState), http.StatusFound)
}

// handleQQCallback 处理 QQ 授权回调：普通登录、绑定、或引导补全邮箱。
func (s *Server) handleQQCallback(w http.ResponseWriter, r *http.Request) {
	state := sessionFrom(r.Context())
	if state == nil {
		writeMessage(w, http.StatusBadRequest, "登录状态已过期，请重新发起 QQ 登录")
		return
	}
	gotState := r.URL.Query().Get("state")
	if gotState == "" || state.session.OAuthState == "" ||
		subtle.ConstantTimeCompare([]byte(gotState), []byte(state.session.OAuthState)) != 1 {
		auditLog("oauth_state_mismatch", "ip", clientIP(r, s.Cfg.TrustedProxies))
		writeMessage(w, http.StatusBadRequest, "登录状态已过期，请重新发起 QQ 登录")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeMessage(w, http.StatusBadRequest, "QQ 授权失败：缺少 code")
		return
	}

	redirectTarget := service.SafeRedirect(state.session.OAuthRedirect)
	profile, err := s.App.QQ.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, err)
		return
	}

	// 绑定流程：只允许当前登录账号给自己绑定。
	if state.session.QQBindUserID != nil {
		bindID := *state.session.QQBindUserID
		currentID, ok := userIDFrom(r)
		if !ok || currentID != bindID {
			writeError(w, service.NewError(400, "登录状态已变化，请重新发起绑定"))
			return
		}
		if _, err := s.App.BindQQ(r.Context(), bindID, profile.OpenID, profile.Nickname, profile.Avatar); err != nil {
			writeError(w, err)
			return
		}
		state.session.QQBindUserID = nil
		state.session.OAuthState = ""
		state.session.OAuthRedirect = ""
		if err := s.App.St.PutSession(r.Context(), state.session); err != nil {
			writeError(w, err)
			return
		}
		auditLog("qq_bind_success", "userId", bindID, "ip", clientIP(r, s.Cfg.TrustedProxies))
		http.Redirect(w, r, redirectTarget, http.StatusFound)
		return
	}

	// 普通快捷登录：已有 QQ 账号直接登录，没有则创建 QQ-only 账号。
	// 不再要求绑定邮箱和密码。
	user, err := s.App.LoginQQ(r.Context(), profile.OpenID, profile.Nickname, avatarValue(profile.Avatar))
	if err != nil {
		writeError(w, err)
		return
	}
	state.session.OAuthState = ""
	state.session.OAuthRedirect = ""
	if err := s.sessions.rotate(r.Context(), w, r, user.ID); err != nil {
		writeError(w, err)
		return
	}
	auditLog("qq_login_success", "userNo", user.UserNo, "ip", clientIP(r, s.Cfg.TrustedProxies))
	http.Redirect(w, r, redirectTarget, http.StatusFound)
}

// handleQQPending 返回当前会话中待补全的 QQ 身份（给完成注册页展示）。
func (s *Server) handleQQPending(w http.ResponseWriter, r *http.Request) {
	state := sessionFrom(r.Context())
	if state == nil || state.session.QQPendingOpenID == "" {
		writeJSON(w, http.StatusOK, map[string]any{"pending": false})
		return
	}
	var avatar any
	if state.session.QQPendingAvatar != "" {
		avatar = state.session.QQPendingAvatar
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pending":  true,
		"nickname": state.session.QQPendingName,
		"avatar":   avatar,
	})
}

// handleQQComplete 首次 QQ 登录时提交邮箱密码，完成注册/绑定。
func (s *Server) handleQQComplete(w http.ResponseWriter, r *http.Request) {
	state := sessionFrom(r.Context())
	if state == nil || state.session.QQPendingOpenID == "" {
		writeMessage(w, http.StatusBadRequest, "没有待完成的 QQ 登录，请重新发起")
		return
	}
	var payload emailAuthPayload
	if err := decodeJSON(w, r, &payload, 64<<10); err != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "请求格式不正确")
		return
	}
	pending := service.PendingQQ{
		OpenID: state.session.QQPendingOpenID,
		Name:   state.session.QQPendingName,
		Avatar: state.session.QQPendingAvatar,
	}
	if state.session.QQPendingUserID != nil {
		pending.UserID = *state.session.QQPendingUserID
	}
	user, err := s.App.CompleteQQ(r.Context(), pending, payload.Email, payload.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	// 登录会重建会话，旧的 pending 字段随之清除。
	s.loginJSON(w, r, user)
}

func (s *Server) handleDevLogin(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.DevLogin {
		notFoundJSON(w, "Not Found")
		return
	}
	qq := r.URL.Query().Get("qq")
	if qq == "" {
		qq = "10001"
	}
	user, err := s.App.DevLogin(r.Context(), qq, r.URL.Query().Get("nickname"))
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.sessions.rotate(r.Context(), w, r, user.ID); err != nil {
		writeError(w, err)
		return
	}
	auditLog("dev_login", "qq", qq, "userNo", user.UserNo, "ip", clientIP(r, s.Cfg.TrustedProxies))
	// #nosec G710 -- 目标已过 SafeRedirect（仅站内相对路径）
	http.Redirect(w, r, service.SafeRedirect(r.URL.Query().Get("redirect")), http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.sessions.logout(r.Context(), w, r); err != nil {
		writeError(w, err)
		return
	}
	if state := sessionFrom(r.Context()); state != nil && state.session.UserID != nil {
		auditLog("logout", "userId", *state.session.UserID, "ip", clientIP(r, s.Cfg.TrustedProxies))
	}
	writeMessage(w, http.StatusOK, "已退出登录")
}

// handleLogoutAll 撤销当前账号的全部会话（V-04）。
func (s *Server) handleLogoutAll(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFrom(r)
	if !ok {
		writeError(w, service.ErrUnauthorized())
		return
	}
	if err := s.App.St.RevokeUserSessions(r.Context(), userID); err != nil {
		writeError(w, err)
		return
	}
	s.sessions.clearCookies(w)
	auditLog("logout_all", "userId", userID, "ip", clientIP(r, s.Cfg.TrustedProxies))
	writeMessage(w, http.StatusOK, "已退出全部设备")
}

// loginJSON 登录成功后重建会话并返回当前账号。
func (s *Server) loginJSON(w http.ResponseWriter, r *http.Request, user store.User) {
	if err := s.sessions.rotate(r.Context(), w, r, user.ID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.meView(user))
}

func (s *Server) meView(user store.User) service.MeView {
	return service.MeView{
		Authenticated: true,
		User:          s.App.UserView(user),
		DevLogin:      s.Cfg.DevLogin,
	}
}

func avatarValue(avatar *string) string {
	if avatar == nil {
		return ""
	}
	return *avatar
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}
