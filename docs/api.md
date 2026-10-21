# HTTP 接口

- 基础路径：`/api`
- 认证：同域 Cookie 会话（`lcw_session`）；未登录返回 401。
- CSRF：写请求必须带 `X-XSRF-TOKEN` 请求头（值来自 `XSRF-TOKEN` Cookie），失败返回 419。
- 请求/响应均为 JSON；上传录像为 `multipart/form-data`，字段名 `file`。
- 错误：`{"message": "..."}`；表单校验额外带 `errors`。
- 时间：ISO8601，例 `2026-10-21T18:00:00+08:00`。

## 公共接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/health` | 健康检查；数据库异常返回 503 |
| GET | `/api/meta` | 大区、段位、位置、赛制、开发登录开关 |
| GET | `/api/me` | 当前登录用户；未登录 `authenticated=false` |
| GET | `/api/power/calculate` | 战力计算器：`position`、`region`、`tier`、`division`、`lp` |
| GET | `/api/champions/library` | 英雄资料库搜索：`q`、`position` |
| GET | `/api/champions` | DataDragon 英雄名与最新版本：`version` |

`/api/health` 响应：

```json
{"ok": true, "database": "ok", "redis": "disabled", "time": "2026-10-21T18:00:00+08:00", "version": "v1.0.0"}
```

## 登录

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/auth/register` | 邮箱注册：`{"email","password"}`，成功即登录 |
| POST | `/api/auth/login` | 邮箱登录：`{"email","password"}` |
| GET | `/auth/qq?redirect=/` | 302 到 QQ 授权页；未配置凭据返回 503 |
| GET | `/auth/qq/callback` | QQ 回调；直接登录或 302 到 `/qq-complete` |
| GET | `/api/auth/callback` | 同上（QQ 后台兼容地址） |
| GET | `/auth/qq/bind?redirect=/` | 已登录用户发起绑定 QQ；未登录 401 |
| GET | `/api/auth/pending` | 查询当前会话是否有待补全的 QQ 身份 |
| POST | `/api/auth/qq/complete` | 首次 QQ 登录补全：`{"email","password"}` |
| GET | `/auth/dev?qq=10001&nickname=测试&redirect=/` | 开发登录；`LCW_DEV_LOGIN=false` 时 404 |
| POST | `/auth/logout` | 退出登录 |

`/api/me` 的 `user` 字段：

```json
{
  "id": 1,
  "userNo": "U12345678",
  "name": "玩家",
  "avatar": null,
  "email": "player@example.com",
  "isDev": false,
  "qqBound": false,
  "hasPassword": true
}
```

QQ 快捷登录只有配置 `LCW_QQ_APP_ID/KEY` 时可用；未配置时 `/auth/qq` 返回 503，前端不显示入口。

首次 QQ 快捷登录的流程：

1. 回调发现 OpenID 未绑定（或旧账号没有邮箱密码）；
2. 302 到前端 `/qq-complete?redirect=...`；
3. 页面调用 `/api/auth/pending` 展示 QQ 昵称；
4. 提交 `/api/auth/qq/complete`：
   - 邮箱不存在：创建账号（生成 `user_no`）并绑定；
   - 邮箱已存在：校验密码；未绑定 QQ 则绑定，已绑定其他 QQ 返回 409。

已登录邮箱用户可访问 `/auth/qq/bind` 绑定 QQ，之后才能直接用快捷登录。

## 群

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/groups/resolve` | 用群号换公开 key：`{"groupNumber":"123456789"}` → `{"key":"...","name":null}` |
| GET | `/api/groups/{group}` | 群信息 + 我的资料 + meta |
| PUT | `/api/groups/{group}/profile` | 保存群内资料 |

资料请求：

```json
{
  "nickname": "玩家昵称",
  "qqNumber": "123456789",
  "rank": {"region": "艾欧尼亚", "tier": "DIAMOND", "division": 3, "lp": 0},
  "positions": ["TOP", "JUNGLE"]
}
```

QQ 号可修改，但同一群内不能重复（422）。接口返回的 QQ 号是脱敏值（如 `123***`）；保存时原样回传脱敏值或留空表示不修改，填新的完整号码才会更新。

## 组织

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/groups/{group}/events` | 列表：`status=open|closed|finished|all`、`page`、`page_size` |
| POST | `/api/groups/{group}/events` | 发起组织（201） |
| GET | `/api/groups/{group}/events/{event}` | 组织详情 |
| PUT | `/api/groups/{group}/events/{event}` | 修改组织信息（全量） |
| POST | `/api/groups/{group}/events/{event}/end` | 结束比赛并锁定全部对局 |

创建/修改请求：

```json
{
  "title": "周五夜战",
  "region": "艾欧尼亚",
  "bestOf": 3,
  "hasBp": true,
  "signupDeadline": "2026-10-25T20:00:00+08:00",
  "startsAt": "2026-10-25T21:00:00+08:00",
  "note": "请提前上线"
}
```

组织详情包含：

- `event`、`permissions`（10 个权限位）；
- `registrations`（报名与替补）、`participantCount`、`substituteCount`、`spectatorCount`；
- `myRegistration`、`mySpectator`；
- `series`：蓝/红胜场、1队/2队胜场、系列赛胜方、每局摘要；
- `roster`：已套用阵容（null 表示未编排）；
- `draft`：当前草稿局（null 表示没有）。

## 报名 / 替补 / 观战

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/groups/{group}/events/{event}/registrations` | 本人报名/改报名 |
| PUT | `/api/groups/{group}/events/{event}/registrations/{member}` | 组织者代改（本人也可） |
| POST | `/api/groups/{group}/events/{event}/registrations/{member}/spectator` | 组织者标记/取消替补：`{"spectator":true}` |
| DELETE | `/api/groups/{group}/events/{event}/registrations/{member}` | 取消报名 |
| POST | `/api/groups/{group}/events/{event}/spectators` | 观战开关：`{"watching":true}` |

以上接口成功后统一返回组织详情。

报名请求：

```json
{
  "isSpectator": false,
  "mainPosition": "TOP",
  "subPositions": ["JUNGLE", "MIDDLE"],
  "rank": {"region": "艾欧尼亚", "tier": "DIAMOND", "division": 3, "lp": 0},
  "subRank": {"region": "艾欧尼亚", "tier": "EMERALD", "division": 2, "lp": 0},
  "note": ""
}
```

## 编排

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/groups/{group}/events/{event}/matches/auto` | 自动匹配，返回最多 5 套方案 |
| POST | `/api/groups/{group}/events/{event}/roster` | 套用方案（服务端重算战力） |

自动匹配响应：

```json
{
  "plans": [{"team1": [/* 5 个 Slot */], "team2": [], "team1Power": 20000, "team2Power": 19980, "diff": 20, "mainCount": 10}],
  "exhausted": false,
  "nodes": 1054,
  "positionCounts": {"TOP": 3, "JUNGLE": 2, "MIDDLE": 2, "BOTTOM": 2, "UTILITY": 2},
  "candidateCount": 11,
  "eventId": 1,
  "bestOf": 3
}
```

套用请求：

```json
{"team1": [{"registrationId": 1, "position": "TOP"}], "team2": []}
```

## 对局

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/groups/{group}/events/{event}/matches` | 新增一局（201） |
| GET | `/api/groups/{group}/events/{event}/matches/{gameNo}` | 一局详情 |
| PUT | `/api/groups/{group}/events/{event}/matches/{gameNo}` | 保存草稿（全量覆盖 players/bans） |
| DELETE | `/api/groups/{group}/events/{event}/matches/{gameNo}` | 删除一局并重排局号 |
| POST | `/api/groups/{group}/events/{event}/matches/{gameNo}/ratings` | 赛后评分：`{"targetMemberId":2,"score":5}` |
| POST | `/api/groups/{group}/events/{event}/matches/{gameNo}/rofl` | 上传录像（multipart，字段 `file`） |

保存请求：

```json
{
  "team1Side": "blue",
  "blueWin": true,
  "durationMs": 1800000,
  "players": [{
    "side": "blue", "position": "TOP", "slot": 1, "memberId": 2,
    "playerName": "玩家", "champion": "Garen",
    "kills": 5, "deaths": 2, "assists": 8, "cs": 180, "gold": 12000,
    "damageToChampions": 20000, "damageTaken": 15000, "visionScore": 20
  }],
  "bans": [{"side": "blue", "slot": 1, "champion": "Darius"}]
}
```

一局详情包含 `match`、`players`、`bans`、`rofl`、`event`、`registrations`、`canRate`、`ratings`、`ratingLabels`。

## 排行与个人分析

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/groups/{group}/ranking?metric=all` | 排行榜 |
| GET | `/api/groups/{group}/players/{member}` | 个人分析 |

`metric`：`all`、`rating`、`kills`、`deaths`、`assists`、`cs`、`gold`、`damage`、`taken`、`vision`；未知值回退 `kills`。

## 常见错误

| 状态 | 场景 |
| --- | --- |
| 401 | 未登录 / 会话过期 |
| 403 | 非组织者操作、群号不在白名单 |
| 404 | 组织、成员或对局不存在 |
| 409 | 报名已截止、比赛已结束、系列赛已分出胜负 |
| 419 | CSRF Token 缺失或不匹配（刷新页面重试） |
| 422 | 参数校验失败、录像无法解析 |
| 429 | 请求过于频繁 |
