#!/usr/bin/env python3
"""端到端验收脚本：用 HTTP API 跑完一局内战的完整流程。

用法（服务已启动）：
    python3 scripts/e2e.py --base http://127.0.0.1:8080 --group 123456789

脚本会创建 10 个开发登录账号（需要 LCW_DEV_LOGIN=true）并完成：
登录 → 群入口 → 资料 → 发起组织 → 报名 → 自动匹配 → 套用阵容 →
记录对局 → 结束比赛 → 赛后评分 → 排行榜 / 个人分析。
"""

from __future__ import annotations

import argparse
import json
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timedelta, timezone
from http.cookiejar import CookieJar

REGIONS = "艾欧尼亚"
TIER = "DIAMOND"
DIVISION = 3

POSITIONS = ["TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"]


class Client:
    def __init__(self, base: str, name: str):
        self.base = base.rstrip("/")
        self.name = name
        self.jar = CookieJar()
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.jar))

    # ---- 基础请求 ----
    def request(self, method: str, path: str, body: dict | None = None) -> tuple[int, dict | str]:
        url = self.base + path
        data = None
        headers = {"Accept": "application/json"}
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            headers["Content-Type"] = "application/json"
        token = self.csrf_token()
        if token:
            headers["X-XSRF-TOKEN"] = token
        req = urllib.request.Request(url, data=data, headers=headers, method=method)
        try:
            with self.opener.open(req, timeout=60) as resp:
                raw = resp.read()
                status = resp.status
        except urllib.error.HTTPError as exc:
            raw = exc.read()
            status = exc.code
            text = raw.decode("utf-8", "replace")
            raise AssertionError(f"[{self.name}] {method} {path} -> {status}: {text}") from None
        if not raw:
            return status, {}
        text = raw.decode("utf-8")
        try:
            return status, json.loads(text)
        except json.JSONDecodeError:
            return status, text

    def get(self, path: str) -> dict:
        return self.request("GET", path)[1]

    def post(self, path: str, body: dict | None = None) -> dict:
        return self.request("POST", path, body or {})[1]

    def put(self, path: str, body: dict) -> dict:
        return self.request("PUT", path, body)[1]

    def delete(self, path: str) -> dict:
        return self.request("DELETE", path, {})[1]

    def csrf_token(self) -> str | None:
        for cookie in self.jar:
            if cookie.name == "XSRF-TOKEN":
                return cookie.value
        return None

    def get_raw(self, path: str) -> tuple[int, bytes]:
        req = urllib.request.Request(self.base + path, headers={"Accept": "text/html"})
        try:
            with self.opener.open(req, timeout=30) as resp:
                return resp.status, resp.read()
        except urllib.error.HTTPError as exc:
            return exc.code, exc.read()

    # ---- 业务步骤 ----
    def dev_login(self, qq: str, nickname: str) -> None:
        query = urllib.parse.urlencode({"qq": qq, "nickname": nickname, "redirect": "/"})
        status, body = self.get_raw(f"/auth/dev?{query}")
        assert status == 200, f"dev login 失败: {status} {body!r}"
        me = self.get("/api/me")
        assert me["authenticated"], "dev login 后仍未登录"
        self.meta = self.get("/api/meta")

    def resolve_group(self, group_number: str) -> str:
        data = self.post("/api/groups/resolve", {"groupNumber": group_number})
        assert data.get("key"), f"resolve 失败: {data}"
        return data["key"]

    def group(self, key: str) -> dict:
        return self.get(f"/api/groups/{key}")

    def save_profile(self, key: str, nickname: str, qq: str, positions: list[str]) -> None:
        payload = {
            "nickname": nickname,
            "qqNumber": qq,
            "rank": {"region": REGIONS, "tier": TIER, "division": DIVISION, "lp": 0},
            "positions": positions,
        }
        data = self.put(f"/api/groups/{key}/profile", payload)
        assert data["profileComplete"], f"资料未完整: {data}"


def iso(delta_days: int) -> str:
    return (datetime.now(timezone.utc) + timedelta(days=delta_days)).isoformat()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", default="http://127.0.0.1:8080")
    parser.add_argument("--group", default="123456789")
    args = parser.parse_args()

    started = time.time()
    print(f"== 目标 {args.base} 群 {args.group} ==")

    # 0. 白名单外应被拒绝
    anon = Client(args.base, "anon")
    anon.get("/api/meta")
    try:
        anon.post("/api/groups/resolve", {"groupNumber": "999999999"})
        raise AssertionError("白名单外的群号没有被拒绝")
    except AssertionError as exc:
        if "未开通" not in str(exc):
            raise
        print("✓ 白名单外群号被拒绝")

    # 1. 10 名玩家登录 + 资料
    players: list[Client] = []
    profiles: list[dict] = []
    for index in range(10):
        qq = str(10001 + index)
        nickname = f"测试玩家{index + 1}"
        position = POSITIONS[index // 2]
        client = Client(args.base, nickname)
        client.dev_login(qq, nickname)
        key = client.resolve_group(args.group)
        client.save_profile(key, nickname, qq, [position, POSITIONS[(index // 2 + 1) % 5]])
        players.append(client)
        profiles.append({
            "client": client,
            "key": key,
            "member": client.group(key)["member"],
            "position": position,
            "qq": qq,
            "nickname": nickname,
        })
        print(f"✓ 玩家 {nickname} 资料就绪（{position}）")

    organizer = profiles[0]
    key = organizer["key"]
    org = organizer["client"]

    # 2. 发起组织
    event = org.post(f"/api/groups/{key}/events", {
        "title": "端到端测试赛",
        "region": REGIONS,
        "bestOf": 1,
        "hasBp": True,
        "signupDeadline": iso(1),
        "startsAt": iso(2),
        "note": "自动化验收",
    })["event"]
    event_id = event["id"]
    assert event["status"] == "open", event
    print(f"✓ 创建组织 #{event_id}")

    # 3. 报名
    for profile in profiles:
        client = profile["client"]
        client.post(f"/api/groups/{key}/events/{event_id}/registrations", {
            "isSpectator": False,
            "mainPosition": profile["position"],
            "subPositions": [POSITIONS[(POSITIONS.index(profile["position"]) + 1) % 5]],
            "rank": {"region": REGIONS, "tier": TIER, "division": DIVISION, "lp": 0},
            "subRank": {"region": REGIONS, "tier": "EMERALD", "division": 2, "lp": 0},
            "note": "",
        })
    detail = org.get(f"/api/groups/{key}/events/{event_id}")
    assert detail["participantCount"] == 10, detail["participantCount"]
    assert detail["permissions"]["canAutoMatch"], detail["permissions"]
    print("✓ 10 人报名完成")

    # 4. 自动匹配 + 套用阵容
    result = org.post(f"/api/groups/{key}/events/{event_id}/matches/auto")
    assert result["plans"], result
    plan = result["plans"][0]
    assert len(plan["team1"]) == 5 and len(plan["team2"]) == 5
    roster = org.post(f"/api/groups/{key}/events/{event_id}/roster", {
        "team1": [{"registrationId": s["registrationId"], "position": s["position"]} for s in plan["team1"]],
        "team2": [{"registrationId": s["registrationId"], "position": s["position"]} for s in plan["team2"]],
    })["roster"]
    assert roster["team1Power"] > 0
    print(f"✓ 自动匹配 {len(result['plans'])} 套方案，已套用（差值 {roster['diff']}）")

    # 5. 记录对局
    created = org.post(f"/api/groups/{key}/events/{event_id}/matches")["match"]
    assert created["gameNo"] == 1, created
    match = org.get(f"/api/groups/{key}/events/{event_id}/matches/1")
    assert len(match["players"]) == 10, len(match["players"])
    payload_players = []
    for player in match["players"]:
        payload_players.append({
            "side": player["side"],
            "position": player["position"],
            "slot": player["slot"],
            "memberId": player["memberId"],
            "playerName": player["playerName"],
            "champion": "Garen",
            "kills": 5,
            "deaths": 2,
            "assists": 8,
            "cs": 180,
            "gold": 12000,
            "damageToChampions": 20000,
            "damageTaken": 15000,
            "visionScore": 20,
        })
    saved = org.put(f"/api/groups/{key}/events/{event_id}/matches/1", {
        "team1Side": "red",
        "blueWin": True,
        "durationMs": 1800000,
        "players": payload_players,
        "bans": [
            {"side": "blue", "slot": 1, "champion": "Darius"},
            {"side": "red", "slot": 1, "champion": "Ahri"},
        ],
    })
    assert saved["match"]["team1Side"] == "red"
    assert len(saved["bans"]) == 2
    print("✓ 对局保存成功（含 BP 与战绩）")

    # 6. 结束比赛
    ended = org.post(f"/api/groups/{key}/events/{event_id}/end")
    assert ended["event"]["status"] == "finished"
    assert ended["event"]["endedAt"]
    print("✓ 组织已结束并锁定")

    # 7. 结束后不可再改
    try:
        org.put(f"/api/groups/{key}/events/{event_id}/matches/1", {"team1Side": "blue"})
        raise AssertionError("结束后仍然可以修改对局")
    except AssertionError as exc:
        if "比赛已结束" not in str(exc):
            raise
    print("✓ 结束后对局锁定")

    # 8. 赛后评分
    target_member = None
    for player in match["players"]:
        if player["side"] == "blue" and player["memberId"]:
            target_member = player["memberId"]
            break
    rater = profiles[1]["client"]
    summary = rater.post(f"/api/groups/{key}/events/{event_id}/matches/1/ratings", {
        "targetMemberId": target_member,
        "score": 5,
    })
    assert summary["count"] == 1 and summary["myScore"] == 5, summary
    print("✓ 评分成功")

    # 9. 排行榜与个人分析
    ranking = org.get(f"/api/groups/{key}/ranking?metric=all")
    assert ranking["items"], ranking
    assert any(item["memberId"] == target_member for item in ranking["items"]), ranking["items"][:2]
    analysis = org.get(f"/api/groups/{key}/players/{target_member}")
    assert analysis["summary"]["games"] == 1, analysis["summary"]
    print(f"✓ 排行榜 {len(ranking['items'])} 人，个人分析 {analysis['summary']['games']} 场")

    # 10. 静态资源与健康检查
    status, html = org.get_raw("/civil-war")
    assert status == 200 and b"<div id=\"app\">" in html, "SPA fallback 异常"
    health = org.get("/api/health")
    assert health["ok"], health
    print("✓ SPA 与健康检查正常")

    print(f"== 全部通过，用时 {time.time() - started:.1f}s ==")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except AssertionError as exc:
        print(f"\n✗ 验收失败: {exc}", file=sys.stderr)
        sys.exit(1)
