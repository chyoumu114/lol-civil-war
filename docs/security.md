# 安全加固记录

本文记录 2026-11 依据安全审计（`shenji.md`）完成的修复。审计报告原文只保存在本机，未随仓库提交。

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
| L-06 | 群白名单存在性 oracle / 解析路径建群 | 未开通统一返回 404 中性文案；解析路径不再建群；DB 白名单由 `group add` 建档，仅静态环境变量白名单允许首访建档 | `internal/service/group.go`、`store/admin.go` |
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

## 3. 残留风险（未在本轮代码中完全消除）

- **注册接口仍会提示邮箱已注册**：为了可用性保留 409 文案，账号枚举仍可通过注册路径探测。彻底消除需要邮箱验证码流程。
- **ROFL 解析器仍一次性读入最多 32MB 元数据**：已通过上传上限与并发配额限制影响面，未改为流式解析。
- **限流是单实例进程内状态**：重启清零、多副本不共享；多副本部署需换 Redis 等共享存储。
- **CSRF 采用“会话 Token + 双提交 Cookie”双保险**：入口页无会话时使用双提交；已拒绝跨站 `Sec-Fetch-Site`，但依赖浏览器 Cookie 隔离。
- **`LCW_MIGRATE_ON_START` 默认仍为 true**：兼容旧部署；按上文关闭后才能真正做到运行时账号无 DDL 权限。
- **审计日志覆盖关键事件但未做告警**：需要接入日志平台后配置增长率/失败率告警。

## 4. 本地验证

```bash
# 单元 + MySQL 集成测试
LCW_TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3307)/' go test ./...

# 静态与漏洞扫描
go vet ./...
gosec -severity medium -confidence medium ./...
govulncheck ./...

# 从旧库结构升级
lol-civil-war migrate
```

本轮已验证：全量测试通过；旧库（缺 `group_id`、`absolute_expires_at`、旧 `uk_rofl_sha256`）迁移成功；同一录像可在第二个群独立去重；HTTP/HTTPS 两种 Cookie 形态、CSP/HSTS、三类配置自检按预期生效。
