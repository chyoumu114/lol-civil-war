package service

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/chyoumu114/lol-civil-war/internal/store"
)

var qqNumberRe = regexp.MustCompile(`^\d{5,12}$`)

// LoginWithOpenID 按 OpenID 登录/注册，并同步 QQ 昵称与头像。
func (a *App) LoginWithOpenID(ctx context.Context, openid, nickname string, avatar *string) (store.User, error) {
	openid = strings.TrimSpace(openid)
	if openid == "" {
		return store.User{}, NewError(502, "QQ 登录失败：无法获取 OpenID")
	}
	nickname = truncateRunes(strings.TrimSpace(nickname), 64)
	if nickname == "" {
		nickname = "QQ 用户"
	}
	var avatarPtr *string
	if avatar != nil {
		value := strings.TrimSpace(*avatar)
		// QQ 返回的头像历史上是 http，HTTPS 站点会被浏览器拦截。
		if strings.HasPrefix(value, "http://") {
			value = "https://" + strings.TrimPrefix(value, "http://")
		}
		value = truncateRunes(value, 255)
		if value != "" {
			avatarPtr = &value
		}
	}
	return a.St.UpsertUser(ctx, openid, nickname, avatarPtr, false)
}

// DevLogin 开发登录：openid 前缀 dev:。
func (a *App) DevLogin(ctx context.Context, qq, nickname string) (store.User, error) {
	qq = strings.TrimSpace(qq)
	if !qqNumberRe.MatchString(qq) {
		return store.User{}, ErrInvalid("开发登录的 QQ 号必须是 5-12 位数字")
	}
	nickname = truncateRunes(strings.TrimSpace(nickname), 64)
	if nickname == "" {
		nickname = "测试玩家" + qq
	}
	return a.St.UpsertUser(ctx, "dev:"+qq, nickname, nil, true)
}

// SafeRedirect 只允许站内相对路径，避免开放重定向。
func SafeRedirect(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "/"
	}
	return path
}

// ErrInvalidLoginState 是 QQ 回调 state 校验失败。
func ErrInvalidLoginState() *Error {
	return NewError(400, "登录状态已过期，请重新发起 QQ 登录")
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

// IsNotFound 判断是否为记录不存在。
func IsNotFound(err error) bool { return errors.Is(err, store.ErrNotFound) }
