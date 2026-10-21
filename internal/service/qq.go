package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// QQProfile 是 QQ 互联返回的登录资料。
type QQProfile struct {
	OpenID   string
	Nickname string
	Avatar   *string
}

// QQClient 封装 QQ 互联 OAuth 2.0。
type QQClient struct {
	appID      string
	appKey     string
	redirect   string
	httpClient *http.Client
}

// NewQQClient 构造 QQ 互联客户端。
func NewQQClient(appID, appKey, redirect string) *QQClient {
	return &QQClient{
		appID:    appID,
		appKey:   appKey,
		redirect: redirect,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Configured 报告 QQ 互联凭据是否完整。
func (c *QQClient) Configured() bool { return c.appID != "" && c.appKey != "" }

// AuthorizeURL 生成授权页地址。
func (c *QQClient) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", c.appID)
	q.Set("redirect_uri", c.redirect)
	q.Set("state", state)
	return "https://graph.qq.com/oauth2.0/authorize?" + q.Encode()
}

// Exchange 用回调 code 换取用户信息。
func (c *QQClient) Exchange(ctx context.Context, code string) (QQProfile, error) {
	token, err := c.fetchAccessToken(ctx, code)
	if err != nil {
		return QQProfile{}, err
	}
	openid, err := c.fetchOpenID(ctx, token)
	if err != nil {
		return QQProfile{}, err
	}
	info, err := c.fetchUserInfo(ctx, token, openid)
	if err != nil {
		return QQProfile{}, err
	}

	avatar := firstString(info, "figureurl_qq_2", "figureurl_qq_1")
	if strings.HasPrefix(avatar, "http://") {
		avatar = "https://" + strings.TrimPrefix(avatar, "http://")
	}
	var avatarPtr *string
	if avatar != "" {
		avatarPtr = &avatar
	}
	nickname, _ := info["nickname"].(string)

	return QQProfile{OpenID: openid, Nickname: nickname, Avatar: avatarPtr}, nil
}

func (c *QQClient) fetchAccessToken(ctx context.Context, code string) (string, error) {
	q := url.Values{}
	q.Set("grant_type", "authorization_code")
	q.Set("client_id", c.appID)
	q.Set("client_secret", c.appKey)
	q.Set("code", code)
	q.Set("redirect_uri", c.redirect)
	q.Set("fmt", "json")

	body, status, err := c.get(ctx, "https://graph.qq.com/oauth2.0/token?"+q.Encode())
	if err != nil || status/100 != 2 {
		return "", NewError(502, "QQ 登录失败：无法获取 access_token")
	}
	data := decodeMaybeJSONP(body)
	if token := firstString(data, "access_token"); token != "" {
		return token, nil
	}
	message := firstString(data, "error_description", "error")
	if message == "" {
		message = "授权码无效或已过期"
	}
	return "", NewError(502, "QQ 登录失败："+message)
}

func (c *QQClient) fetchOpenID(ctx context.Context, token string) (string, error) {
	q := url.Values{}
	q.Set("access_token", token)
	q.Set("fmt", "json")

	body, status, err := c.get(ctx, "https://graph.qq.com/oauth2.0/me?"+q.Encode())
	if err != nil || status/100 != 2 {
		return "", NewError(502, "QQ 登录失败：无法获取 OpenID")
	}
	data := decodeMaybeJSONP(body)
	openid := firstString(data, "openid")
	if openid == "" {
		return "", NewError(502, "QQ 登录失败：无法获取 OpenID")
	}
	return openid, nil
}

func (c *QQClient) fetchUserInfo(ctx context.Context, token, openid string) (map[string]any, error) {
	q := url.Values{}
	q.Set("access_token", token)
	q.Set("oauth_consumer_key", c.appID)
	q.Set("openid", openid)
	q.Set("fmt", "json")

	body, status, err := c.get(ctx, "https://graph.qq.com/user/get_user_info?"+q.Encode())
	if err != nil || status/100 != 2 {
		return nil, NewError(502, "QQ 登录失败：无法获取用户信息")
	}
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, NewError(502, "QQ 登录失败：无法获取用户信息")
	}
	ret, ok := data["ret"]
	if !ok || fmt.Sprintf("%.0f", toFloat(ret)) != "0" {
		return nil, NewError(502, "QQ 登录失败：无法获取用户信息")
	}
	return data, nil
}

func (c *QQClient) get(ctx context.Context, rawURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return body, resp.StatusCode, err
}

// decodeMaybeJSONP 兼容 QQ 接口返回的 JSONP 与 a=b&c=d 形式。
func decodeMaybeJSONP(body []byte) map[string]any {
	text := strings.TrimSpace(string(body))
	if strings.HasPrefix(text, "callback") {
		start := strings.Index(text, "(")
		end := strings.LastIndex(text, ")")
		if start >= 0 && end > start {
			text = text[start+1 : end]
		}
	}
	if !strings.HasPrefix(strings.TrimSpace(text), "{") {
		values, err := url.ParseQuery(text)
		if err == nil && len(values) > 0 {
			out := make(map[string]any, len(values))
			for key, list := range values {
				if len(list) > 0 {
					out[key] = list[0]
				}
			}
			return out
		}
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return nil
	}
	return data
}

func firstString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key]; ok {
			if str, ok := value.(string); ok && str != "" {
				return str
			}
		}
	}
	return ""
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		var f float64
		_, _ = fmt.Sscanf(v, "%f", &f)
		return f
	default:
		return -1
	}
}
