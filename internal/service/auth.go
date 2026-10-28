package service

import (
	"context"
	"crypto/rand"
	"errors"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/chyoumu114/lol-civil-war/internal/store"
)

var (
	qqNumberRe = regexp.MustCompile(`^\d{5,12}$`)
	emailRe    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	bcryptCost = 12
)

// PendingQQ 是 QQ 快捷登录通过后、尚未完成邮箱注册的临时身份。
type PendingQQ struct {
	OpenID string
	UserID int64 // 0 表示全新 QQ 账号；>0 表示旧数据里已存在但缺邮箱密码的账号
	Name   string
	Avatar string
}

// RegisterEmail 邮箱注册并创建账号（不自动绑定 QQ）。
func (a *App) RegisterEmail(ctx context.Context, email, password string) (store.User, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return store.User{}, err
	}
	if err := validatePassword(password); err != nil {
		return store.User{}, err
	}

	if _, err := a.St.UserByEmail(ctx, email); err == nil {
		return store.User{}, ErrConflict("该邮箱已注册，请直接登录")
	} else if !errors.Is(err, store.ErrNotFound) {
		return store.User{}, err
	}

	userNo, err := a.newUserNo(ctx)
	if err != nil {
		return store.User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return store.User{}, err
	}
	hashValue := string(hash)
	// 邮箱注册不自动生成昵称：昵称与 QQ 号一样，由用户首次进群时必填。
	name := ""

	user, err := a.St.CreateUser(ctx, userNo, &email, &hashValue, nil, name, nil, false)
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return store.User{}, ErrConflict("该邮箱已注册，请直接登录")
		}
		return store.User{}, err
	}
	return user, nil
}

// LoginEmail 邮箱 + 密码登录。
func (a *App) LoginEmail(ctx context.Context, email, password string) (store.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return store.User{}, ErrInvalid("请输入邮箱和密码")
	}
	user, err := a.St.UserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.User{}, NewError(401, "邮箱或密码不正确")
		}
		return store.User{}, err
	}
	if user.PasswordHash == nil || *user.PasswordHash == "" {
		return store.User{}, NewError(401, "该邮箱尚未设置密码")
	}
	if bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)) != nil {
		return store.User{}, NewError(401, "邮箱或密码不正确")
	}
	return user, nil
}

// CompleteQQ 首次 QQ 快捷登录时补全邮箱密码：
//   - 新 QQ：邮箱不存在则创建账号并绑定；邮箱已存在则校验密码后绑定；
//   - 旧数据用户（已有 openid、无邮箱密码）：把邮箱密码补到同一个账号上。
func (a *App) CompleteQQ(ctx context.Context, pending PendingQQ, email, password string) (store.User, error) {
	if strings.TrimSpace(pending.OpenID) == "" {
		return store.User{}, NewError(400, "QQ 登录状态已失效，请重新发起快捷登录")
	}
	email, err := normalizeEmail(email)
	if err != nil {
		return store.User{}, err
	}
	if err := validatePassword(password); err != nil {
		return store.User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return store.User{}, err
	}
	hashValue := string(hash)
	avatarPtr := optionalString(pending.Avatar)
	name := truncateRunes(strings.TrimSpace(pending.Name), 64)
	if name == "" {
		name = "QQ 用户"
	}

	if pending.UserID > 0 {
		user, err := a.St.UserByID(ctx, pending.UserID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return store.User{}, NewError(400, "账号不存在，请重新登录")
			}
			return store.User{}, err
		}
		if user.QQOpenID == nil || *user.QQOpenID != pending.OpenID {
			return store.User{}, NewError(409, "QQ 登录状态已变化，请重新发起快捷登录")
		}
		if user.Email != nil && *user.Email != "" && *user.Email != email {
			return store.User{}, NewError(409, "该账号已绑定其他邮箱")
		}
		if other, err := a.St.UserByEmail(ctx, email); err == nil && other.ID != user.ID {
			return store.User{}, NewError(409, "该邮箱已被其他账号使用")
		} else if err != nil && !errors.Is(err, store.ErrNotFound) {
			return store.User{}, err
		}
		if err := a.St.SetUserEmailPassword(ctx, user.ID, email, hashValue); err != nil {
			if errors.Is(err, store.ErrDuplicate) {
				return store.User{}, NewError(409, "该邮箱已被其他账号使用")
			}
			return store.User{}, err
		}
		return a.St.UserByID(ctx, user.ID)
	}

	existing, err := a.St.UserByEmail(ctx, email)
	if err == nil {
		if existing.PasswordHash == nil || bcrypt.CompareHashAndPassword([]byte(*existing.PasswordHash), []byte(password)) != nil {
			return store.User{}, ErrInvalid("邮箱或密码不正确")
		}
		if existing.QQOpenID != nil {
			if *existing.QQOpenID == pending.OpenID {
				return existing, nil
			}
			return store.User{}, NewError(409, "该邮箱已绑定其他 QQ")
		}
		if err := a.St.BindQQOpenID(ctx, existing.ID, pending.OpenID, name, avatarPtr); err != nil {
			if errors.Is(err, store.ErrDuplicate) {
				return store.User{}, NewError(409, "该 QQ 已绑定其他账号")
			}
			return store.User{}, err
		}
		return a.St.UserByID(ctx, existing.ID)
	}
	if !errors.Is(err, store.ErrNotFound) {
		return store.User{}, err
	}

	userNo, err := a.newUserNo(ctx)
	if err != nil {
		return store.User{}, err
	}
	user, err := a.St.CreateUser(ctx, userNo, &email, &hashValue, &pending.OpenID, name, avatarPtr, false)
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return store.User{}, NewError(409, "邮箱或 QQ 已被其他账号使用")
		}
		return store.User{}, err
	}
	return user, nil
}

// LoginQQ 使用 QQ 登录，不存在则直接创建只有 QQ 的账号。
// QQ 登录不要求邮箱和密码，账号仍有系统生成的 user_no。
func (a *App) LoginQQ(ctx context.Context, openid, name, avatar string) (store.User, error) {
	openid = strings.TrimSpace(openid)
	if openid == "" {
		return store.User{}, NewError(502, "QQ 登录失败：无法获取 OpenID")
	}
	name = truncateRunes(strings.TrimSpace(name), 64)
	if name == "" {
		name = "QQ 用户"
	}
	avatarPtr := optionalString(avatar)

	if user, err := a.St.UserByQQOpenID(ctx, openid); err == nil {
		if err := a.St.UpdateUserNameAvatar(ctx, user.ID, name, avatarPtr); err != nil {
			return store.User{}, err
		}
		return a.St.UserByID(ctx, user.ID)
	} else if !errors.Is(err, store.ErrNotFound) {
		return store.User{}, err
	}

	userNo, err := a.newUserNo(ctx)
	if err != nil {
		return store.User{}, err
	}
	user, err := a.St.CreateUser(ctx, userNo, nil, nil, &openid, name, avatarPtr, false)
	if err != nil {
		// 并发回调可能已经创建了同一 QQ，重新按 OpenID 取一次。
		if errors.Is(err, store.ErrDuplicate) {
			if existing, findErr := a.St.UserByQQOpenID(ctx, openid); findErr == nil {
				return existing, nil
			}
		}
		return store.User{}, err
	}
	return user, nil
}

// BindQQ 已登录用户绑定 QQ 快捷登录。
func (a *App) BindQQ(ctx context.Context, userID int64, openid, name string, avatar *string) (store.User, error) {
	openid = strings.TrimSpace(openid)
	if openid == "" {
		return store.User{}, NewError(502, "QQ 登录失败：无法获取 OpenID")
	}
	user, err := a.St.UserByID(ctx, userID)
	if err != nil {
		return store.User{}, err
	}
	if user.QQOpenID != nil && *user.QQOpenID != "" && !strings.HasPrefix(*user.QQOpenID, "dev:") {
		if *user.QQOpenID == openid {
			_ = a.St.UpdateUserNameAvatar(ctx, user.ID, name, avatar)
			return a.St.UserByID(ctx, user.ID)
		}
		return store.User{}, NewError(409, "当前账号已绑定其他 QQ")
	}
	if other, err := a.St.UserByQQOpenID(ctx, openid); err == nil && other.ID != user.ID {
		return store.User{}, NewError(409, "该 QQ 已绑定其他账号")
	} else if err != nil && !errors.Is(err, store.ErrNotFound) {
		return store.User{}, err
	}
	if err := a.St.BindQQOpenID(ctx, user.ID, openid, name, avatar); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return store.User{}, NewError(409, "该 QQ 已绑定其他账号")
		}
		return store.User{}, err
	}
	return a.St.UserByID(ctx, user.ID)
}

// DevLogin 开发登录：openid 前缀 dev:，仅本地调试使用。
func (a *App) DevLogin(ctx context.Context, qq, nickname string) (store.User, error) {
	qq = strings.TrimSpace(qq)
	if !qqNumberRe.MatchString(qq) {
		return store.User{}, ErrInvalid("开发登录的 QQ 号必须是 5-12 位数字")
	}
	nickname = truncateRunes(strings.TrimSpace(nickname), 64)
	if nickname == "" {
		nickname = "测试玩家" + qq
	}
	openid := "dev:" + qq

	if user, err := a.St.UserByQQOpenID(ctx, openid); err == nil {
		_ = a.St.UpdateUserNameAvatar(ctx, user.ID, nickname, nil)
		return a.St.UserByID(ctx, user.ID)
	} else if !errors.Is(err, store.ErrNotFound) {
		return store.User{}, err
	}

	userNo, err := a.newUserNo(ctx)
	if err != nil {
		return store.User{}, err
	}
	return a.St.CreateUser(ctx, userNo, nil, nil, &openid, nickname, nil, true)
}

// newUserNo 生成系统唯一用户编号：U + 8 位数字，冲突时重试。
func (a *App) newUserNo(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		userNo := "U" + randomDigits(8)
		if _, err := a.St.UserByNo(ctx, userNo); err == nil {
			continue
		} else if !errors.Is(err, store.ErrNotFound) {
			return "", err
		}
		return userNo, nil
	}
	return "", errors.New("生成 user_no 失败，请重试")
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", ErrInvalid("请输入邮箱")
	}
	if len(email) > 255 || !emailRe.MatchString(email) {
		return "", ErrInvalid("邮箱格式不正确")
	}
	return email, nil
}

func validatePassword(password string) error {
	if len([]rune(password)) < 8 {
		return ErrInvalid("密码至少 8 位")
	}
	if len([]byte(password)) > 72 {
		return ErrInvalid("密码最多 72 字节")
	}
	return nil
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func randomDigits(n int) string {
	const digits = "0123456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return strings.Repeat("0", n)
	}
	for i := range buf {
		buf[i] = digits[int(buf[i])%len(digits)]
	}
	return string(buf)
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
