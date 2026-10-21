# 架构设计

本文是项目的权威设计说明，实现以本文为准。

## 1. 目标与边界

为 QQ 群组织《英雄联盟》5v5 内战提供闭环：

1. 群成员用 QQ 登录，维护群内资料；
2. 组织者发起内战（标题、大区、赛制、BP、报名截止、开赛时间）；
3. 群成员报名或替补，截止前可自行修改；
4. 组织者手动编排，或用自动战力平衡从报名者中选出 10 人，生成多套 5v5 阵容；
5. 记录每局红蓝阵容、BP 与战绩，可上传 `.rofl` 自动填充；
6. 组织者「结束比赛」后锁定全部记录，报名者/观战者评分，按群统计排行榜与个人分析。

约束：

- 所有业务数据按 QQ 群隔离，同一 QQ 号在不同群是两套资料；
- 页面适配 PC 与手机；
- 群号必须在白名单中；URL 使用随机公开 key，不暴露群号。

## 2. 技术栈与部署形态

| 组件 | 选型 |
| --- | --- |
| 语言 | Go 1.26.x（标准库 `net/http`） |
| 数据库 | MySQL 8.0+ / 8.4 LTS（`github.com/go-sql-driver/mysql`） |
| 前端 | Vue 3 + Arco Design Vue + Vite，构建后 `go:embed` 进二进制 |
| 部署 | 单容器，数据卷挂载 `/data` |

生产拓扑：

```
浏览器 ──HTTPS──> Nginx/Caddy ──> 127.0.0.1:18020 ──> app 容器
                                                       ├─ /data（上传临时文件、可选图标）
                                                       └─ mysql:3306（内部网络）
                                                             └─ 卷 lol-civil-war-mysql-data
```

## 3. 代码分层

```
cmd/lol-civil-war/       main：加载配置、启动 HTTP、优雅关闭
internal/config/         环境变量 / .env
internal/domain/         纯计算，不依赖 HTTP 与数据库
  champion/              英雄资料库（内嵌 JSON）与搜索
  match/                 BO1/BO3/BO5 判定
  rank/                  段位、大区、位置、战力
  rofl/                  ROFL/ROFL2 元数据解析
  roster/                自动匹配（回溯 + Top N）
internal/store/          MySQL 持久化：schema.sql、SQL 查询、事务
internal/service/        应用服务：权限、事务编排、DTO
internal/web/            路由、会话/CSRF、限流、静态资源
internal/webui/          内嵌前端产物
web/                     Vue 3 源码
```

依赖方向：`web → service → store/domain`，domain 不反向依赖任何层。

## 4. 数据模型

| 表 | 说明 |
| --- | --- |
| `users` | 全局账号：`qq_openid` 唯一，`name`、`avatar`、`is_dev` |
| `qq_groups` | QQ 群：`qq_group_number` 唯一，`public_key` 唯一（表名避开 MySQL 保留字 `groups`） |
| `group_members` | 群内资料：`(group_id,user_id)` 唯一；QQ 号、昵称、段位、位置 |
| `events` | 内战组织：大区、赛制、BP、报名截止、开赛时间、`ended_at` |
| `registrations` | 报名/替补：`(event_id,member_id)` 唯一；主选/补位位置与两套段位 |
| `event_rosters` | 套用阵容：`event_id` 唯一，`payload` JSON（1队/2队各 5 人） |
| `event_spectators` | 独立观战：`(event_id,member_id)` 唯一 |
| `matches` | 一局：`(event_id,game_no)` 唯一；`team1_side`、`blue_win`、`status` |
| `match_players` | 一局 10 人：蓝/红、位置、槽位、英雄、KDA/补刀/经济/伤害 |
| `match_bans` | BP 禁用：`(match_id,side,slot)` 唯一 |
| `match_player_ratings` | 赛后评分：`(match_id,rater,target)` 唯一，1–5 星 |
| `rofl_uploads` | 录像解析结果：`sha256` 唯一，`parsed` JSON |
| `sessions` | 服务端会话：会话 id、用户、CSRF、OAuth 临时状态 |

所有表使用 InnoDB + `utf8mb4`；时间统一以 UTC 毫秒（BIGINT）存储，对外输出按 `Asia/Shanghai` 的 ISO8601（带偏移）。
列表/对象字段使用 LONGTEXT 存 JSON；应用启动时执行幂等建表语句。

## 5. 登录与会话

- QQ 互联 OAuth 2.0：`/auth/qq` → 授权页 → `/auth/qq/callback` → 换 access_token → 换 OpenID → 拉用户信息。
- 会话：随机 40 位 id 存在 `sessions` 表，Cookie `lcw_session`（HttpOnly、SameSite=Lax）。
- CSRF：每个会话一个 token，写入非 HttpOnly 的 `XSRF-TOKEN` Cookie；前端 axios 自动放进 `X-XSRF-TOKEN` 头；写请求校验，失败返回 419。
- 登录成功重建会话（防会话固定）。
- QQ 互联不返回 QQ 号，因此 OpenID 是身份，群内首次补全 QQ 号用于展示。

## 6. 战力与自动匹配

### 6.1 段位量化

- 黑铁四 = 1000，每升一小段 +100；钻石一 = 3700；
- 大师/宗师/王者没有小段，`基础分 = 4000 + LP`；
- 大区系数：艾欧尼亚 1.1、峡谷之巅 1.3、韩服 1.5，其余 1.0；
- 位置影响力：上单 0.17、打野 0.23、中单 0.22、下路 0.17、辅助 0.21。

```
战力 = round(基础分 × 大区系数 × (1 + 位置影响力))
```

主选位置用「最高段位」，补位位置用「补位段位」（未填回退最高段位）。

### 6.2 自动匹配

输入为非替补报名者；每名玩家只能出现一次，且只能打主选或补位位置。

- 按 上→野→中→下→辅 顺序回溯，每个位置选 2 人，并尝试两种 1队/2队 归属；
- 若某位置主选人数 ≥ 2，则不再从补位里抢人；
- 叶子节点规范化 1/2 队（避免整体交换产生重复方案），按
  `主选人数降序 → 战力差升序 → 1队战力升序` 维护 Top 5；
- 节点预算默认 300 万，超预算时返回当前最优并标记 `exhausted`；
- 某位置候选不足 2 人、或总人数不足 10 人 → 409 并给出具体原因。

算法在 `internal/domain/roster`，输入纯结构体，可独立测试。

## 7. 对局与赛制

- BO1/BO3/BO5 按 **1队/2队** 胜场判定，先赢 `bestOf/2+1` 局结束；
- 每局 `team1_side` 决定 1队是蓝方还是红方；BP 始终按蓝/红展示；
- 新局从已套用阵容生成 10 名玩家；出场玩家可来自群成员，也允许临时玩家（`member_id` 为空）；
- 对局保存为草稿，可反复修改；组织者「结束比赛」时统一置为 `submitted`；
- 取消某个成员的参赛资格（标记替补）会把他从已套用阵容移除，但保留其位置字段。

## 8. ROFL 解析

- 只读取文件末尾的明文 JSON 元数据（ROFL2，14.11+），不解析游戏数据包；
- 支持 ROFL1（旧格式，尽力兼容）；14.9–14.10 无元数据，返回明确错误；
- 上传按 `sha256` 幂等：同一文件重复上传直接复用解析结果；
- 解析结果按 `side + position` 自动写回对局阵容，同时返回映射建议；
- 录像原文件不落盘，只保存 sha256 与解析结果。

## 9. 排行榜与个人分析

只统计组织者已结束（`events.ended_at` 非空）且 `status=submitted` 的对局：

- 排行指标：评分、击杀、死亡、助攻、补刀、经济、总伤害、承伤、视野；
- 合并排行（`metric=all`）按评分降序、KDA 降序；
- 个人分析：场次/胜率/合计/场均/单场最高、同队胜率（同队 ≥ 2 场）的最佳/最差队友、最近 10 局。

## 10. API 约定

- 基础路径 `/api`，JSON；字段 camelCase（分页 meta 为 snake_case，兼容旧前端）。
- 状态码：401 未登录、403 无权限/白名单、404 不存在、409 状态冲突、419 CSRF、422 校验失败、429 限流。
- 错误结构：`{"message": "..."}`；少数表单校验额外带 `errors: {field: [...]}`。
- 时间：ISO8601，`Asia/Shanghai`。

完整端点见 [api.md](api.md)。

## 11. 关键决策

1. **MySQL 8**：与应用容器一起由 Compose 编排，数据卷持久化；比单文件嵌入式数据库更适合并发写入与后续扩展，代价是多一个数据库容器。
2. **单二进制 + 内嵌前端**：不需要 Nginx 托管静态文件，不存在跨域与路径前缀问题。
3. **保留 Cookie Session + CSRF**：与前端 axios 默认行为一致，避免引入 Token 存储与过期处理。
4. **ROFL 不落盘**：只存 sha256 与解析结果，节省磁盘并天然去重。
5. **自动匹配精确搜索**：报名规模通常 ≤ 30，回溯 + 预算足够；规模变大再考虑 beam search。
6. **英雄资料库内嵌**：离线可用；头像作为版权素材不打包，默认走 DataDragon CDN。
