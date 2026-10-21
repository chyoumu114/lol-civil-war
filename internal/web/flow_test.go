package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/chyoumu114/lol-civil-war/internal/config"
	"github.com/chyoumu114/lol-civil-war/internal/service"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

const testGroup = "123456789"

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		Addr:            "127.0.0.1:0",
		DataDir:         dir,
		DBPath:          filepath.Join(dir, "test.db"),
		BaseURL:         "http://example.test",
		TimeZone:        "Asia/Shanghai",
		AllowedGroups:   []string{testGroup},
		DevLogin:        true,
		SessionTTL:      time.Hour,
		TrustProxy:      false,
		ChampionIconDir: filepath.Join(dir, "champions"),
		RosterMaxPlans:  5,
		RosterBudget:    200000,
		DivisionStep:    100,
		RoflMaxKB:       1024,
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	app := service.New(cfg, st)
	server := New(app, cfg, "test")
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(func() {
		ts.Close()
		_ = st.Close()
	})
	return ts, st
}

type apiClient struct {
	t    *testing.T
	base string
	http *http.Client
}

func newAPIClient(t *testing.T, base string) *apiClient {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &apiClient{t: t, base: base, http: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
}

func (c *apiClient) csrf() string {
	parsed, _ := url.Parse(c.base)
	for _, cookie := range c.http.Jar.Cookies(parsed) {
		if cookie.Name == csrfCookieName {
			return cookie.Value
		}
	}
	return ""
}

func (c *apiClient) raw(method, path string, body any, withCSRF bool) (int, []byte) {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if withCSRF {
		req.Header.Set(csrfHeaderName, c.csrf())
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func (c *apiClient) do(method, path string, body any, wantStatus int) map[string]any {
	c.t.Helper()
	status, data := c.raw(method, path, body, true)
	if status != wantStatus {
		c.t.Fatalf("%s %s -> %d, want %d: %s", method, path, status, wantStatus, data)
	}
	if len(data) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		c.t.Fatalf("%s %s 响应不是 JSON 对象: %s", method, path, data)
	}
	return out
}

func (c *apiClient) devLogin(qq, nickname string) {
	c.t.Helper()
	// 先取一次 meta 拿到 XSRF Cookie
	c.do("GET", "/api/meta", nil, http.StatusOK)
	status, _ := c.raw("GET", fmt.Sprintf("/auth/dev?qq=%s&nickname=%s&redirect=/", qq, url.QueryEscape(nickname)), nil, false)
	if status != http.StatusOK {
		c.t.Fatalf("开发登录失败: %d", status)
	}
	me := c.do("GET", "/api/me", nil, http.StatusOK)
	if me["authenticated"] != true {
		c.t.Fatalf("开发登录后未登录: %v", me)
	}
}

func (c *apiClient) resolveGroup() string {
	c.t.Helper()
	data := c.do("POST", "/api/groups/resolve", map[string]any{"groupNumber": testGroup}, http.StatusOK)
	key, _ := data["key"].(string)
	if key == "" {
		c.t.Fatalf("群 key 为空: %v", data)
	}
	return key
}

func (c *apiClient) saveProfile(key, nickname, qq, position string) map[string]any {
	c.t.Helper()
	payload := map[string]any{
		"nickname":  nickname,
		"qqNumber":  qq,
		"rank":      map[string]any{"region": "艾欧尼亚", "tier": "DIAMOND", "division": 3, "lp": 0},
		"positions": []string{position},
	}
	data := c.do("PUT", "/api/groups/"+key+"/profile", payload, http.StatusOK)
	if data["profileComplete"] != true {
		c.t.Fatalf("资料未完整: %v", data)
	}
	return data
}

func TestCivilWarFlow(t *testing.T) {
	ts, _ := newTestServer(t)

	// 健康检查不应创建会话（容器每 30 秒探活一次）
	healthResp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	healthResp.Body.Close()
	if cookies := healthResp.Header.Values("Set-Cookie"); len(cookies) != 0 {
		t.Fatalf("健康检查不应下发 Cookie: %v", cookies)
	}

	// CSRF：缺少 Token 的写请求应被拒绝
	anon := newAPIClient(t, ts.URL)
	anon.do("GET", "/api/meta", nil, http.StatusOK)
	if status, body := anon.raw("POST", "/api/groups/resolve", map[string]any{"groupNumber": testGroup}, false); status != 419 {
		t.Fatalf("缺少 CSRF 应返回 419，实际 %d: %s", status, body)
	}

	// 入口页引导流程：GET /api/meta 拿到 XSRF Cookie 后，POST resolve 必须成功
	bootstrap := newAPIClient(t, ts.URL)
	bootstrap.do("GET", "/api/meta", nil, http.StatusOK)
	if bootstrap.csrf() == "" {
		t.Fatal("/api/meta 未下发 XSRF-TOKEN Cookie")
	}
	resolved := bootstrap.do("POST", "/api/groups/resolve", map[string]any{"groupNumber": testGroup}, http.StatusOK)
	if resolved["key"] == nil || resolved["key"] == "" {
		t.Fatalf("入口引导 resolve 失败: %v", resolved)
	}

	// 白名单外拒绝
	whitelist := anon.do("POST", "/api/groups/resolve", map[string]any{"groupNumber": "999999999"}, http.StatusForbidden)
	if whitelist["message"] == nil {
		t.Fatalf("白名单外响应异常: %v", whitelist)
	}

	positions := []string{"TOP", "TOP", "JUNGLE", "JUNGLE", "MIDDLE", "MIDDLE", "BOTTOM", "BOTTOM", "UTILITY", "UTILITY"}
	clients := make([]*apiClient, 0, 10)
	for index := 0; index < 10; index++ {
		nickname := fmt.Sprintf("测试玩家%d", index+1)
		client := newAPIClient(t, ts.URL)
		client.devLogin(fmt.Sprintf("%d", 20001+index), nickname)
		key := client.resolveGroup()
		client.saveProfile(key, nickname, fmt.Sprintf("%d", 20001+index), positions[index])
		clients = append(clients, client)
	}

	organizer := clients[0]
	key := organizer.resolveGroup()
	groupData := organizer.do("GET", "/api/groups/"+key, nil, http.StatusOK)
	member := groupData["member"].(map[string]any)
	memberID := int64(member["id"].(float64))

	// 发起组织
	deadline := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	starts := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	created := organizer.do("POST", "/api/groups/"+key+"/events", map[string]any{
		"title": "自动化测试赛", "region": "艾欧尼亚", "bestOf": 1, "hasBp": true,
		"signupDeadline": deadline, "startsAt": starts, "note": "test",
	}, http.StatusCreated)
	event := created["event"].(map[string]any)
	eventID := int64(event["id"].(float64))
	if event["status"] != "open" {
		t.Fatalf("组织状态 = %v", event["status"])
	}

	// 全部报名
	for index, client := range clients {
		payload := map[string]any{
			"isSpectator":  false,
			"mainPosition": positions[index],
			"subPositions": []string{positions[(index+1)%len(positions)]},
			"rank":         map[string]any{"region": "艾欧尼亚", "tier": "DIAMOND", "division": 3, "lp": 0},
			"subRank":      map[string]any{"region": "艾欧尼亚", "tier": "EMERALD", "division": 2, "lp": 0},
			"note":         "",
		}
		client.do("POST", fmt.Sprintf("/api/groups/%s/events/%d/registrations", key, eventID), payload, http.StatusOK)
	}

	detail := organizer.do("GET", fmt.Sprintf("/api/groups/%s/events/%d", key, eventID), nil, http.StatusOK)
	if int(detail["participantCount"].(float64)) != 10 {
		t.Fatalf("报名人数 = %v", detail["participantCount"])
	}

	// 自动匹配 + 套用
	result := organizer.do("POST", fmt.Sprintf("/api/groups/%s/events/%d/matches/auto", key, eventID), map[string]any{}, http.StatusOK)
	plans := result["plans"].([]any)
	if len(plans) == 0 {
		t.Fatal("自动匹配没有返回方案")
	}
	plan := plans[0].(map[string]any)
	buildTeam := func(raw any) []map[string]any {
		slots := raw.([]any)
		out := make([]map[string]any, 0, len(slots))
		for _, item := range slots {
			slot := item.(map[string]any)
			out = append(out, map[string]any{
				"registrationId": slot["registrationId"],
				"position":       slot["position"],
			})
		}
		return out
	}
	roster := organizer.do("POST", fmt.Sprintf("/api/groups/%s/events/%d/roster", key, eventID), map[string]any{
		"team1": buildTeam(plan["team1"]),
		"team2": buildTeam(plan["team2"]),
	}, http.StatusOK)["roster"].(map[string]any)
	if roster["team1Power"].(float64) <= 0 {
		t.Fatalf("阵容战力异常: %v", roster)
	}

	// 记录对局
	matchCreated := organizer.do("POST", fmt.Sprintf("/api/groups/%s/events/%d/matches", key, eventID), nil, http.StatusCreated)
	if int(matchCreated["match"].(map[string]any)["gameNo"].(float64)) != 1 {
		t.Fatalf("局号异常: %v", matchCreated)
	}
	match := organizer.do("GET", fmt.Sprintf("/api/groups/%s/events/%d/matches/1", key, eventID), nil, http.StatusOK)
	players := match["players"].([]any)
	if len(players) != 10 {
		t.Fatalf("对局玩家数 = %d", len(players))
	}

	payloadPlayers := make([]map[string]any, 0, len(players))
	for _, item := range players {
		player := item.(map[string]any)
		payloadPlayers = append(payloadPlayers, map[string]any{
			"side": player["side"], "position": player["position"], "slot": player["slot"],
			"memberId": player["memberId"], "playerName": player["playerName"], "champion": "Garen",
			"kills": 5, "deaths": 2, "assists": 8, "cs": 180, "gold": 12000,
			"damageToChampions": 20000, "damageTaken": 15000, "visionScore": 20,
		})
	}
	saved := organizer.do("PUT", fmt.Sprintf("/api/groups/%s/events/%d/matches/1", key, eventID), map[string]any{
		"team1Side": "red", "blueWin": true, "durationMs": 1800000,
		"players": payloadPlayers,
		"bans": []map[string]any{
			{"side": "blue", "slot": 1, "champion": "Darius"},
			{"side": "red", "slot": 1, "champion": "Ahri"},
		},
	}, http.StatusOK)
	if len(saved["bans"].([]any)) != 2 {
		t.Fatalf("BP 数量 = %v", saved["bans"])
	}

	// 结束比赛
	ended := organizer.do("POST", fmt.Sprintf("/api/groups/%s/events/%d/end", key, eventID), nil, http.StatusOK)
	if ended["event"].(map[string]any)["status"] != "finished" {
		t.Fatalf("结束后状态 = %v", ended["event"])
	}

	// 锁定校验
	organizer.do("PUT", fmt.Sprintf("/api/groups/%s/events/%d/matches/1", key, eventID),
		map[string]any{"team1Side": "blue"}, http.StatusConflict)

	// 评分
	target := int64(0)
	for _, item := range players {
		player := item.(map[string]any)
		if player["side"] == "blue" && player["memberId"] != nil {
			target = int64(player["memberId"].(float64))
			break
		}
	}
	if target == 0 {
		t.Fatal("没有找到可评分玩家")
	}
	summary := clients[1].do("POST", fmt.Sprintf("/api/groups/%s/events/%d/matches/1/ratings", key, eventID), map[string]any{
		"targetMemberId": target, "score": 5,
	}, http.StatusOK)
	if summary["count"].(float64) != 1 || summary["myScore"].(float64) != 5 {
		t.Fatalf("评分汇总异常: %v", summary)
	}

	// 排行榜 / 个人分析
	ranking := organizer.do("GET", fmt.Sprintf("/api/groups/%s/ranking?metric=all", key), nil, http.StatusOK)
	if len(ranking["items"].([]any)) != 10 {
		t.Fatalf("排行榜人数 = %d", len(ranking["items"].([]any)))
	}
	analysis := organizer.do("GET", fmt.Sprintf("/api/groups/%s/players/%d", key, memberID), nil, http.StatusOK)
	if analysis["summary"].(map[string]any)["games"].(float64) != 1 {
		t.Fatalf("个人分析场次 = %v", analysis["summary"])
	}
	if len(analysis["recent"].([]any)) != 1 {
		t.Fatalf("最近对局 = %v", analysis["recent"])
	}

	// 静态 SPA 回落
	resp, err := http.Get(ts.URL + "/civil-war")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("id=\"app\"")) {
		t.Fatalf("SPA 回落异常: %d", resp.StatusCode)
	}
}
