package web

import (
	"net/http"

	"github.com/chyoumu114/lol-civil-war/internal/service"
)

func (s *Server) handleQQRedirect(w http.ResponseWriter, r *http.Request) {
	if !s.App.QQ.Configured() {
		writeMessage(w, http.StatusServiceUnavailable,
			"QQ 互联未配置：请设置环境变量 LCW_QQ_APP_ID / LCW_QQ_APP_KEY，或使用开发登录")
		return
	}
	state := sessionFrom(r.Context())
	if state == nil {
		writeMessage(w, http.StatusInternalServerError, "会话不可用，请刷新后重试")
		return
	}
	oauthState := randomToken(40)
	redirect := service.SafeRedirect(r.URL.Query().Get("redirect"))
	state.session.OAuthState = oauthState
	state.session.OAuthRedirect = redirect
	if err := s.App.St.PutSession(r.Context(), state.session); err != nil {
		writeError(w, err)
		return
	}
	http.Redirect(w, r, s.App.QQ.AuthorizeURL(oauthState), http.StatusFound)
}

func (s *Server) handleQQCallback(w http.ResponseWriter, r *http.Request) {
	state := sessionFrom(r.Context())
	if state == nil {
		writeMessage(w, http.StatusBadRequest, "登录状态已过期，请重新发起 QQ 登录")
		return
	}
	gotState := r.URL.Query().Get("state")
	if gotState == "" || state.session.OAuthState == "" || gotState != state.session.OAuthState {
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
	user, err := s.App.LoginWithOpenID(r.Context(), profile.OpenID, profile.Nickname, profile.Avatar)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.sessions.rotate(r.Context(), w, r, user.ID); err != nil {
		writeError(w, err)
		return
	}
	http.Redirect(w, r, redirectTarget, http.StatusFound)
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
	http.Redirect(w, r, service.SafeRedirect(r.URL.Query().Get("redirect")), http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.sessions.logout(r.Context(), w, r); err != nil {
		writeError(w, err)
		return
	}
	writeMessage(w, http.StatusOK, "已退出登录")
}
