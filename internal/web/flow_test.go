package web

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/chyoumu114/lol-civil-war/internal/config"
	"github.com/chyoumu114/lol-civil-war/internal/service"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

const testGroup = "123456789"

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	baseDSN := strings.TrimSpace(os.Getenv("LCW_TEST_MYSQL_DSN"))
	if baseDSN == "" {
		t.Skip("未设置 LCW_TEST_MYSQL_DSN，跳过 MySQL 集成测试")
	}
	baseCfg, err := mysql.ParseDSN(baseDSN)
	if err != nil {
		t.Fatalf("LCW_TEST_MYSQL_DSN 不合法: %v", err)
	}

	// 用管理连接为本次测试创建独立数据库，测试结束删除，互不影响。
	adminCfg := *baseCfg
	adminCfg.DBName = ""
	admin, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatalf("连接 MySQL 失败: %v", err)
	}
	admin.SetMaxOpenConns(1)

	dbName := fmt.Sprintf("lcw_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE `" + dbName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		_ = admin.Close()
		t.Fatalf("创建测试数据库失败（需要 CREATE 权限）: %v", err)
	}

	testCfg := *baseCfg
	testCfg.DBName = dbName
	testCfg.ParseTime = false
	testCfg.Loc = time.UTC
	dsn := testCfg.FormatDSN()

	dir := t.TempDir()
	cfg := &config.Config{
		Addr:            "127.0.0.1:0",
		DataDir:         dir,
		DBDSN:           dsn,
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
	st, err := store.Open(dsn)
	if err != nil {
		_, _ = admin.Exec("DROP DATABASE `" + dbName + "`")
		_ = admin.Close()
		t.Fatalf("打开数据库失败: %v", err)
	}
	app := service.New(cfg, st)
	server := New(app, cfg, "test")
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(func() {
		ts.Close()
		_ = st.Close()
		_, _ = admin.Exec("DROP DATABASE `" + dbName + "`")
		_ = admin.Close()
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

	// 蓝/红方只在单局内有意义：1队为红方、蓝方获胜时，系列赛胜方必须是 2队。
	detailAfterEnd := organizer.do("GET", fmt.Sprintf("/api/groups/%s/events/%d", key, eventID), nil, http.StatusOK)
	series := detailAfterEnd["series"].(map[string]any)
	if series["team1Wins"].(float64) != 0 || series["team2Wins"].(float64) != 1 || series["teamWinner"].(float64) != 2 {
		t.Fatalf("系列赛应按 1队/2队 计分，实际: %v", series)
	}
	if _, ok := series["winnerSide"]; ok {
		t.Fatalf("系列赛不应再返回胜方颜色: %v", series)
	}
	matchSummary := series["matches"].([]any)[0].(map[string]any)
	if _, ok := matchSummary["winnerSide"]; ok || matchSummary["winnerTeam"].(float64) != 2 {
		t.Fatalf("单局胜方队伍异常: %v", matchSummary)
	}
	list := organizer.do("GET", fmt.Sprintf("/api/groups/%s/events", key), nil, http.StatusOK)
	var listed map[string]any
	for _, raw := range list["data"].([]any) {
		item := raw.(map[string]any)
		if int64(item["id"].(float64)) == eventID {
			listed = item
			break
		}
	}
	if listed == nil || listed["team1Wins"].(float64) != 0 || listed["team2Wins"].(float64) != 1 {
		t.Fatalf("组织列表比分应按 1队/2队 计分，实际: %v", listed)
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
	// 未执行前端构建时返回“前端尚未构建”提示页，也算 SPA 回落正常。
	built := bytes.Contains(body, []byte("id=\"app\""))
	placeholder := bytes.Contains(body, []byte("前端尚未构建"))
	if resp.StatusCode != http.StatusOK || (!built && !placeholder) {
		t.Fatalf("SPA 回落异常: %d", resp.StatusCode)
	}
}

// TestDatabaseWhitelist 验证通过数据库添加的白名单群无需重启即可访问。
func TestDatabaseWhitelist(t *testing.T) {
	ts, st := newTestServer(t)
	ctx := context.Background()

	const number = "987654321"
	if err := st.AddAllowedGroup(ctx, number, "集成测试"); err != nil {
		t.Fatalf("添加白名单失败: %v", err)
	}

	client := newAPIClient(t, ts.URL)
	client.do("GET", "/api/meta", nil, http.StatusOK)
	resolved := client.do("POST", "/api/groups/resolve", map[string]any{"groupNumber": number}, http.StatusOK)
	if key, _ := resolved["key"].(string); key == "" {
		t.Fatalf("数据库白名单群未返回 key: %v", resolved)
	}

	removed, err := st.RemoveAllowedGroup(ctx, number)
	if err != nil || !removed {
		t.Fatalf("移除白名单失败: removed=%v err=%v", removed, err)
	}
	status, body := client.raw("POST", "/api/groups/resolve", map[string]any{"groupNumber": number}, true)
	if status != http.StatusForbidden {
		t.Fatalf("移除白名单后应返回 403，实际 %d: %s", status, body)
	}
}

// sessionCookie 取出测试客户端当前会话 id。
func sessionCookie(t *testing.T, client *apiClient) string {
	t.Helper()
	parsed, err := url.Parse(client.base)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range client.http.Jar.Cookies(parsed) {
		if cookie.Name == sessionCookieName {
			return cookie.Value
		}
	}
	t.Fatal("找不到会话 Cookie")
	return ""
}

// setPendingQQ 直接给测试会话写入待补全的 QQ 身份（模拟 QQ 回调后的状态）。
func setPendingQQ(t *testing.T, st *store.Store, client *apiClient, openid, name string, userID *int64) {
	t.Helper()
	client.do("GET", "/api/meta", nil, http.StatusOK)
	id := sessionCookie(t, client)
	sess, err := st.GetSession(context.Background(), id)
	if err != nil {
		t.Fatalf("读取会话失败: %v", err)
	}
	sess.QQPendingOpenID = openid
	sess.QQPendingName = name
	sess.QQPendingAvatar = ""
	sess.QQPendingUserID = userID
	if err := st.PutSession(context.Background(), sess); err != nil {
		t.Fatalf("写入会话失败: %v", err)
	}
}

func TestEmailAuth(t *testing.T) {
	ts, st := newTestServer(t)
	ctx := context.Background()

	client := newAPIClient(t, ts.URL)
	client.do("GET", "/api/meta", nil, http.StatusOK)

	registered := client.do("POST", "/api/auth/register", map[string]any{
		"email": "player@example.com", "password": "password123",
	}, http.StatusOK)
	user := registered["user"].(map[string]any)
	if user["userNo"] == nil || user["userNo"] == "" {
		t.Fatalf("注册后没有 userNo: %v", user)
	}
	if user["email"] != "player@example.com" || user["hasPassword"] != true || user["qqBound"] != false {
		t.Fatalf("注册返回字段异常: %v", user)
	}
	if user["name"] != "" {
		t.Fatalf("邮箱注册昵称应为空，实际: %v", user["name"])
	}
	userNo := user["userNo"].(string)

	// 邮箱注册后首次进群：昵称保持为空，资料不完整
	key := client.resolveGroup()
	groupData := client.do("GET", "/api/groups/"+key, nil, http.StatusOK)
	member := groupData["member"].(map[string]any)
	if member["nickname"] != "" {
		t.Fatalf("邮箱注册用户进群后昵称应为空，实际: %v", member["nickname"])
	}
	if groupData["profileComplete"] != false {
		t.Fatalf("空昵称不应算资料完整: %v", groupData)
	}

	// 邮箱重复
	status, _ := client.raw("POST", "/api/auth/register", map[string]any{
		"email": "PLAYER@example.com", "password": "password123",
	}, true)
	if status != http.StatusConflict {
		t.Fatalf("重复邮箱应 409，实际 %d", status)
	}

	// 密码错误
	anon := newAPIClient(t, ts.URL)
	anon.do("GET", "/api/meta", nil, http.StatusOK)
	status, _ = anon.raw("POST", "/api/auth/login", map[string]any{
		"email": "player@example.com", "password": "wrong-password",
	}, true)
	if status != http.StatusUnauthorized {
		t.Fatalf("错误密码应 401，实际 %d", status)
	}

	// 正常登录
	loggedIn := anon.do("POST", "/api/auth/login", map[string]any{
		"email": "player@example.com", "password": "password123",
	}, http.StatusOK)
	if loggedIn["user"].(map[string]any)["userNo"] != userNo {
		t.Fatalf("登录后 userNo 不一致: %v", loggedIn)
	}

	// 数据库里的身份仍是同一个 user_no
	dbUser, err := st.UserByEmail(ctx, "player@example.com")
	if err != nil || dbUser.UserNo != userNo {
		t.Fatalf("数据库 user_no 异常: %+v err=%v", dbUser, err)
	}
	if dbUser.Name != "" {
		t.Fatalf("数据库邮箱账号昵称应为空，实际: %q", dbUser.Name)
	}
}

func TestQQCompleteFlow(t *testing.T) {
	ts, st := newTestServer(t)
	ctx := context.Background()

	// 先注册一个邮箱账号（不绑定 QQ）
	emailClient := newAPIClient(t, ts.URL)
	emailClient.do("GET", "/api/meta", nil, http.StatusOK)
	registered := emailClient.do("POST", "/api/auth/register", map[string]any{
		"email": "bind@example.com", "password": "password123",
	}, http.StatusOK)
	emailUserID := int64(registered["user"].(map[string]any)["id"].(float64))

	// 1) 新 QQ + 已存在邮箱 + 正确密码 → 绑定到已有账号
	qqClient := newAPIClient(t, ts.URL)
	setPendingQQ(t, st, qqClient, "qq-openid-1", "QQ玩家", nil)
	completed := qqClient.do("POST", "/api/auth/qq/complete", map[string]any{
		"email": "bind@example.com", "password": "password123",
	}, http.StatusOK)
	if int64(completed["user"].(map[string]any)["id"].(float64)) != emailUserID {
		t.Fatalf("QQ 补全没有绑定到已有邮箱账号: %v", completed)
	}
	if completed["user"].(map[string]any)["qqBound"] != true {
		t.Fatalf("QQ 绑定状态异常: %v", completed)
	}
	boundUser, err := st.UserByQQOpenID(ctx, "qq-openid-1")
	if err != nil || boundUser.ID != emailUserID {
		t.Fatalf("按 QQ OpenID 查询失败: %+v err=%v", boundUser, err)
	}

	// 2) 同一邮箱 + 另一个 QQ → 409
	second := newAPIClient(t, ts.URL)
	setPendingQQ(t, st, second, "qq-openid-2", "另一个QQ", nil)
	status, _ := second.raw("POST", "/api/auth/qq/complete", map[string]any{
		"email": "bind@example.com", "password": "password123",
	}, true)
	if status != http.StatusConflict {
		t.Fatalf("邮箱已绑定其他 QQ 应 409，实际 %d", status)
	}

	// 3) 新 QQ + 新邮箱 → 创建账号
	third := newAPIClient(t, ts.URL)
	setPendingQQ(t, st, third, "qq-openid-3", "新QQ玩家", nil)
	created := third.do("POST", "/api/auth/qq/complete", map[string]any{
		"email": "newqq@example.com", "password": "password123",
	}, http.StatusOK)
	if created["user"].(map[string]any)["qqBound"] != true {
		t.Fatalf("新 QQ 注册异常: %v", created)
	}
	newUser, err := st.UserByQQOpenID(ctx, "qq-openid-3")
	if err != nil || newUser.Email == nil || *newUser.Email != "newqq@example.com" {
		t.Fatalf("新 QQ 账号数据异常: %+v err=%v", newUser, err)
	}
	if newUser.UserNo == "" {
		t.Fatal("新 QQ 账号缺少 user_no")
	}
	if newUser.Name != "新QQ玩家" {
		t.Fatalf("首次 QQ 注册应使用 QQ 昵称，实际: %q", newUser.Name)
	}

	// 首次 QQ 注册后进群，群昵称也应默认使用 QQ 昵称
	qqKey := third.resolveGroup()
	qqGroupData := third.do("GET", "/api/groups/"+qqKey, nil, http.StatusOK)
	qqMember := qqGroupData["member"].(map[string]any)
	if qqMember["nickname"] != "新QQ玩家" {
		t.Fatalf("QQ 首次注册的群昵称异常: %v", qqMember["nickname"])
	}

	// 4) 旧数据账号（有 openid、无邮箱密码）→ 邮箱密码补到同一个账号
	legacyOpenID := "legacy-openid"
	legacy, err := st.CreateUser(ctx, "U00000001", nil, nil, &legacyOpenID, "旧账号", nil, false)
	if err != nil {
		t.Fatalf("创建旧账号失败: %v", err)
	}
	legacyID := legacy.ID
	fourth := newAPIClient(t, ts.URL)
	setPendingQQ(t, st, fourth, legacyOpenID, "旧账号", &legacyID)
	fourth.do("POST", "/api/auth/qq/complete", map[string]any{
		"email": "legacy@example.com", "password": "password123",
	}, http.StatusOK)
	legacyAfter, err := st.UserByID(ctx, legacyID)
	if err != nil || legacyAfter.Email == nil || *legacyAfter.Email != "legacy@example.com" {
		t.Fatalf("旧账号补全邮箱失败: %+v err=%v", legacyAfter, err)
	}
	if legacyAfter.UserNo != "U00000001" {
		t.Fatalf("旧账号 user_no 被改变: %s", legacyAfter.UserNo)
	}
}

func TestQQNumberEditableUnique(t *testing.T) {
	ts, st := newTestServer(t)
	ctx := context.Background()

	first := newAPIClient(t, ts.URL)
	first.devLogin("40001", "甲")
	key := first.resolveGroup()
	first.saveProfile(key, "甲", "40001", "TOP")

	// 页面回传的是脱敏 QQ（40***），原样保存应保留原值
	groupData := first.do("GET", "/api/groups/"+key, nil, http.StatusOK)
	groupID := int64(groupData["group"].(map[string]any)["id"].(float64))
	memberID := int64(groupData["member"].(map[string]any)["id"].(float64))
	first.do("PUT", "/api/groups/"+key+"/profile", map[string]any{
		"nickname":  "甲",
		"qqNumber":  "40***",
		"rank":      map[string]any{"region": "艾欧尼亚", "tier": "GOLD", "division": 4, "lp": 0},
		"positions": []string{"TOP"},
	}, http.StatusOK)
	member, err := st.MemberByID(ctx, groupID, memberID)
	if err != nil || member.QQNumber == nil || *member.QQNumber != "40001" {
		t.Fatalf("脱敏 QQ 保存后原值被改动: %+v err=%v", member, err)
	}

	second := newAPIClient(t, ts.URL)
	second.devLogin("40002", "乙")
	second.resolveGroup()
	second.saveProfile(key, "乙", "40002", "JUNGLE")

	profilePayload := func(qq string) map[string]any {
		return map[string]any{
			"nickname":  "乙",
			"qqNumber":  qq,
			"rank":      map[string]any{"region": "艾欧尼亚", "tier": "GOLD", "division": 4, "lp": 0},
			"positions": []string{"JUNGLE"},
		}
	}

	// 改成别人的 QQ 号 → 422
	status, _ := second.raw("PUT", "/api/groups/"+key+"/profile", profilePayload("40001"), true)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("重复 QQ 应 422，实际 %d", status)
	}
	// 改成自己的新 QQ → 允许
	second.do("PUT", "/api/groups/"+key+"/profile", profilePayload("40009"), http.StatusOK)
	// 第一个用户再改成 40009 → 422
	status, _ = first.raw("PUT", "/api/groups/"+key+"/profile", map[string]any{
		"nickname":  "甲",
		"qqNumber":  "40009",
		"rank":      map[string]any{"region": "艾欧尼亚", "tier": "GOLD", "division": 4, "lp": 0},
		"positions": []string{"TOP"},
	}, true)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("跨成员重复 QQ 应 422，实际 %d", status)
	}
}
