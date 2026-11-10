# 安全加固记录

本文记录 2026-11 依据三轮安全审计完成的修复与上线记录。审计原文（`shenji.md`、`lol-civil-war-security-audit-round3.md`）只保存在本机，未随仓库提交。

## 1. 修复清单

| 编号 | 问题 | 处理方式 | 主要位置 |
| --- | --- | --- | --- |
| H-01 | 伪造 `X-Forwarded-For` 绕过 IP 限流 | 默认不信任代理；`LCW_TRUSTED_PROXIES` 支持 CIDR；只对可信对端解析 XFF，且从右向左取第一个不可信地址；登录按邮箱做失败计数退避 | `internal/config/config.go`、`internal/web/response.go`、`internal/web/handlers_auth.go` |
| H-02 | 匿名请求无界创建会话 | `/api/meta`、`/api/me` 只读会话不建会话；会话创建只发生在 `/auth/qq`、`/auth/dev`，并按 IP 限 120 次/分；匿名会话空闲 2h、绝对 4h；入口页改用无状态双提交 CSRF Cookie | `internal/web/session.go`、`internal/web/server.go` |
| M-01 | 账号枚举（文案/时序） | 登录失败统一为“邮箱或密码不正确”；账号不存在/无密码时也执行一次 bcrypt 哑哈希比较 | `internal/service/auth.go` |
| M-02 | 弱口令、无账号级限制 | 密码下限提升到 10 位；同一邮箱 15 分钟内失败 10 次进入退避，成功登录清零（对不存在的邮箱同样计数） | `internal/service/auth.go`、`internal/web/handlers_auth.go` |
| M-03 | 开发登录误配 | `https` 或非回环地址开启 `LCW_DEV_LOGIN` 且未显式 `LCW_DEV_MODE=true` 时拒绝启动 | `internal/config/config.go` |
| M-04 | 会话无绝对寿命 | 会话增加 `absolute_expires_at`；空闲超时与绝对寿命分离 | `store/schema.sql`、`store/sessions.go`、`web/session.go` |
| M-05 | Cookie/安全头加固不足 | HTTPS 使用 `__Host-lcw_session`；补齐 CSP、Permissions-Policy、HSTS（仅 HTTPS）；HTML 使用 `no-store`；校验 `BaseURL`/`CookieSecure` 组合 | `internal/web/session.go`、`internal/web/server.go`、`internal/config/config.go` |
| M-06 | DataDragon 版本放大 | 只允许“配置版本 / latest / 已缓存版本”；缓存上限 32 条；负缓存 5 分钟；同键请求合并 | `internal/service/ddragon.go`、`web/handlers_public.go` |
| M-07 | 录像上传资源放大 | 默认上限降至 64MB；上传 5 次/分/用户 + 全局并发 2；multipart 内存阈值 8MB；文件名只取 basename | `internal/config/config.go`、`web/handlers_match.go` |
| L-01 | 录像跨群去重覆盖 | `rofl_uploads` 增加 `group_id`，唯一键改为 `(group_id, sha256)`；归属更新带群条件；迁移自动回填 | `store/schema.sql`、`store/upgrade.go`、`store/matches.go`、`service/match.go` |
| L-02 | health 泄露内部错误 | 匿名响应只返回 `database: "error"`，细节写日志 | `internal/web/server.go` |
| L-03 | 反斜杠开放重定向 | `SafeRedirect` 用 `url.Parse` 校验，并额外拒绝反斜杠、控制字符与 `.`/`..` 段 | `internal/service/auth.go` |
| L-04 | CSRF Token 走 query | 删除 `_token` 查询参数回退，只接受请求头 | `internal/web/session.go` |
| L-05 | `match_players.raw` 无约束 | 只接受 JSON 对象且 ≤8KB；单人 raw 结构与体积都受限 | `internal/service/match.go` |
| L-06 | 群白名单存在性 oracle / 解析路径建群 | 未开通统一返回 404 中性文案；解析路径完全只读；DB 白名单由 `group add` 建档，环境变量白名单在启动期由 `provisionEnvGroups` 建档（N-4） | `internal/service/group.go`、`store/admin.go`、`cmd/lol-civil-war/main.go` |
| L-07 | 随机源静默降级 | 删除固定 key 降级；`randomDigits`/`newUserNo` 返回错误；`crypto/rand.Text` 失败即 panic；随机串用拒绝采样 | `service/auth.go`、`service/group.go`、`store/upgrade.go`、`web/session.go` |
| L-08 | 配置默认值不安全 | 数据库口令为空且非 `LCW_DEV_MODE` 时拒绝启动；`BaseURL` 格式校验；`TrustProxy` 默认 false | `internal/config/config.go` |
| L-09 | 应用账号需要 DDL 权限 | 新增 `migrate` 子命令与 `LCW_MIGRATE_ON_START=false`，运行时可只给 DML 权限 | `cmd/lol-civil-war`、`internal/store/store.go` |
| L-10 | 安全审计日志缺失 | 登录成功/失败/退避、CSRF 拒绝、限流、开发登录、白名单增删、群解析失败写入结构化 `audit` 事件 | `internal/web/audit.go` 及各 handler |
| L-11 | 自动匹配 CPU 放大 | 全局并发 2 + 15s 超时；匹配器每 256 节点检查 `context`，超时返回当前最优 | `web/handlers_registration.go`、`domain/roster/matcher.go` |
| I-01 | OAuth state 非常量时间比较 | 改用 `subtle.ConstantTimeCompare` | `web/handlers_auth.go` |
| I-02 | 输入反射进 message | 未知位置、群不可用等改为固定文案 | `web/handlers_public.go`、`service/group.go` |
| I-03 | QQ 脱敏弱 / 数字群号直达 | QQ 隐藏中段只留前后 2 位；`/api/groups/{group}` 不再接受纯数字群号 | `service/mapper.go`、`service/group.go` |
| I-04 | 供应链门禁缺失 | CI 增加 `gosec -severity medium`、`govulncheck` 与 `npm audit --omit=dev` | `.github/workflows/ci.yml` |

## 2. 行为变化与运维动作

1. **Cookie 名称**：HTTPS 下会话 Cookie 变为 `__Host-lcw_session`，HTTP 调试仍为 `lcw_session`。升级后旧会话会失效一次。
2. **群解析**：未开通/不在白名单/尚未建档统一返回 404；URL 只能使用 `public_key`，不能再用数字群号。
3. **白名单建档**：`group add <群号>` 现在会同时创建群记录并打印 `key`；`group list` 可查看。
4. **密码策略**：注册/补全密码至少 10 位（登录不校验旧密码长度）。
5. **迁移**：默认启动时自动迁移；生产建议预执行 `lol-civil-war migrate`，并设置 `LCW_MIGRATE_ON_START=false`，运行时账号只保留 `SELECT/INSERT/UPDATE/DELETE`。
6. **反向代理**：使用 Nginx/Caddy 时配置 `LCW_TRUSTED_PROXIES`，否则所有请求的限流键都会退化为代理地址（可用，但粒度变粗）。示例见 `deploy/.env.example`。

## 3. 第三轮回归修复（2026-11-10）

在 v0.5.0 上线后，依据第三轮审计报告又做了一轮加固。

> 说明：第三轮报告对 M-02（账号级限速）与 L-10（登录审计）标注为“未修复/部分修复”，但复核 v0.5.0 HEAD 确认两项当时已经实现；本轮保留实现并补充测试与注册审计事件。以下修复已随 **v0.5.1** 上线。

| 编号 | 处理方式 | 主要位置 |
| --- | --- | --- |
| I-05 | `go.mod` 最低版本从 `1.27.3` 降到 `1.26.0`（`golang.org/x/crypto v0.58` 的最低要求）；CI 的 `setup-go` 按 go.mod 安装 1.26.x，安全门禁可真正执行；发布镜像仍用 Go 1.27.3 构建并验证 | `go.mod`、`.github/workflows/ci.yml`、`deploy/Dockerfile` |
| N-6 | 有会话时 **只接受会话绑定的 CSRF Token**，删除双提交回退；SPA/HTML 页面也加载会话并刷新 XSRF Cookie，避免误删 Cookie 后无法恢复 | `internal/web/session.go` |
| N-1 | `maxMetadataBytes` 32MB → 8MB；`statsJson` 改流式解码，玩家条目上限 64；解析记录 `metaBytes`/耗时 | `internal/domain/rofl/parser.go`、`internal/service/match.go` |
| N-2 | 注册增加 IP（10 次/10 分钟）与同邮箱（5 次/小时）计数；409 冲突写审计 | `internal/web/handlers_auth.go` |
| N-3 | 登录账号级失败退避在 v0.5.0 已实现（`loginBlocked`），本轮补测试确认三条失败路径状态码/响应体一致 | `internal/web/handlers_auth.go`、`flow_test.go` |
| N-4 | 解析路径彻底只读；环境变量白名单群改为 **启动期一次性建档**（`provisionEnvGroups`） | `internal/service/group.go`、`cmd/lol-civil-war/main.go` |
| N-5 | 新增 `register_conflict` / `register_rate_limited` / `register_blocked` 审计；登录、DevLogin、QQ、白名单审计在 v0.5.0 已覆盖 | `internal/web/handlers_auth.go`、`audit.go` |
| N-7.1 | Cookie `MaxAge` 取 `min(空闲到期, 绝对到期)` | `internal/web/session.go` |
| N-7.2 | DDragon `calls` 随版本缓存一起清理 | `internal/service/ddragon.go` |
| N-7.3 | 删除 `X-Real-IP` 回退，只信 `X-Forwarded-For` | `internal/web/response.go` |
| N-7.4 | `maskQQ` 增加单测，确认只保留前后 2 位、隐藏中段 | `internal/service/auth_test.go` |

新增测试：CSRF 登录态拒绝双提交、注册 IP 限流、登录失败三路径一致性、玩家条目上限、QQ 脱敏。

## 4. 残留风险（未在本轮代码中完全消除）

- **注册接口仍会提示邮箱已注册**：已加 IP/邮箱计数与审计，但换邮箱批量探测仍可行。彻底消除需要邮箱验证码流程。
- **ROFL 外层元数据仍整体 `json.Unmarshal`**：已把上限降到 8MB、`statsJson` 改流式解码并限制玩家条目；极端文件仍可能有数十 MB 峰值内存，但已受上传上限与全局并发约束。
- **限流是单实例进程内状态**：重启清零、多副本不共享；多副本部署需换 Redis 等共享存储。
- **CSRF**：有会话时严格绑定会话 Token；无会话（登录/注册/入口页）才使用双提交。`XSRF-TOKEN` 尚未改 `__Host-` 前缀（需前端 axios 同步改名），单域名部署下风险有限。
- **`LCW_MIGRATE_ON_START` 默认仍为 true**：兼容旧部署；按上文关闭后才能真正做到运行时账号无 DDL 权限。
- **审计日志覆盖关键事件但未做告警**：需要接入日志平台后配置增长率/失败率告警。

## 5. 验证

```bash
# 单元 + MySQL 集成测试（Go 1.26+；发布镜像用 Go 1.27.3）
LCW_TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3307)/' go test ./...

# 静态与漏洞扫描
go vet ./...
gosec -severity medium -confidence medium ./...
govulncheck ./...

# 从旧库结构升级（v0.5.1 无结构变更，可跳过）
lol-civil-war migrate
```

已完成验证：

- Go **1.26.0** 与 **1.27.3** 双工具链下 `go build` / `go vet` / `go test ./...`（含 MySQL 集成）全部通过；
- `gosec` **0 issues**，`govulncheck` **0 个可达漏洞**；
- 旧库（缺 `group_id`、`absolute_expires_at`、旧 `uk_rofl_sha256`）迁移成功且幂等，同一录像可在第二个群独立去重；
- 真实起服冒烟：环境变量白名单启动期建档、HTML 页面刷新会话 CSRF、登录态 `Cookie==Header` 返回 419、静态资源不下发 Cookie；
- 线上 v0.5.1 验证：健康检查、CSP/HSTS、`__Host-lcw_session`、群解析 200/404、可信代理真实 IP、数据量不变。

> 注意：本机若设置 `GOSUMDB=off`，`GOTOOLCHAIN=go1.26.0` 下载工具链会失败，需临时 `GOSUMDB=sum.golang.org`。

## 6. 发布记录

| 版本 | 代码 | 镜像 | 上线时间 | 部署前备份 |
| --- | --- | --- | --- | --- |
| v0.5.0 | `27b5183` | `.../lol-civil-war:v0.5.0` | 2026-11-10 14:22 | `backup-20261110-140024.sql.gz` |
| v0.5.1 | `e3d47d8` | `.../lol-civil-war:v0.5.1` | 2026-11-10 18:48 | `backup-20261110-184819.sql.gz` |

- v0.5.0：安全加固第一批（含数据库迁移：会话绝对寿命、录像按群去重）。
- v0.5.1：第三轮审计修复；**无数据库结构变更**。
- 回滚：把远端 `/opt/lol-civil-war/.env` 的 `LCW_IMAGE` 改回上一版本，执行 `docker compose up -d`。
