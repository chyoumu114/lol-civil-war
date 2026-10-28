# 部署指南

> 第一次在 Linux 上部署、或者不熟悉 Docker/反向代理：先看 [小白版部署教程](deploy-beginner.md)。

目标：把服务部署到一台 Linux 服务器，用 HTTPS 域名对外提供访问。
应用与 MySQL 都由 Docker Compose 管理，数据库不对外暴露端口。

## 1. 环境要求

- Linux + Docker 24+、Docker Compose v2
- 1 核 1G 起（应用常驻内存很小；MySQL 建议至少 512MB）
- 一个可用的域名；使用 QQ 快捷登录时要求回调地址可访问（HTTPS）
- 宿主 Nginx 或 Caddy 负责 80/443 与证书

## 2. 按顺序执行

```bash
# 1. 拉取代码
git clone https://github.com/chyoumu114/lol-civil-war.git
cd lol-civil-war/deploy

# 2. 复制配置
cp .env.example .env

# 3. 编辑 .env（必填项见下表）
${EDITOR:-vi} .env

# 4. 启动：构建应用镜像 + 启动 MySQL
docker compose up -d --build

# 5. 查看状态与健康检查
docker compose ps
curl -fsS http://127.0.0.1:18020/api/health
```

看到 `{"ok":true,...}` 即启动成功。

`.env` 必填项：

| 变量 | 示例 | 说明 |
| --- | --- | --- |
| `LCW_ALLOWED_GROUPS` | `123456789,987654321` | 群白名单，逗号分隔 |
| `LCW_BASE_URL` | `https://lol.example.com` | 对外地址 |
| `LCW_QQ_APP_ID` / `LCW_QQ_APP_KEY` | — | QQ 互联凭据 |
| `LCW_DEV_LOGIN` | `false` | 生产必须关闭 |
| `LCW_BIND` | `127.0.0.1:18020` | 只监听本机，交给反向代理 |
| `MYSQL_DATABASE` | `lol_civil_war` | 数据库名 |
| `MYSQL_USER` / `MYSQL_PASSWORD` | `lcw` / 随机强口令 | 应用账号 |
| `MYSQL_ROOT_PASSWORD` | 随机强口令 | root 口令，仅备份/维护用 |
| `LCW_IMAGE` | `<registry>/<namespace>/lol-civil-war:v0.2.0` | 应用镜像 |

> `.env` 含凭据，已在 `.gitignore` 中；不要提交。
> 从源码构建时，如果访问 Go/npm 官方源很慢，可在 `.env` 打开 `DOCKER_GOPROXY` / `DOCKER_NPM_REGISTRY` 注释并使用国内镜像。
> 如果服务器拉取 Docker Hub 很慢，可把 `MYSQL_IMAGE` 换成私有仓库中的 mysql 8.4 镜像。

## 3. 反向代理与 HTTPS

### Nginx

```nginx
server {
    listen 443 ssl http2;
    server_name lol.example.com;

    ssl_certificate     /etc/letsencrypt/live/lol.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/lol.example.com/privkey.pem;

    client_max_body_size 128m;   # 录像上传

    location / {
        proxy_pass http://127.0.0.1:18020;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }
}
```

### Caddy

```caddyfile
lol.example.com {
    reverse_proxy 127.0.0.1:18020
    request_body {
        max_size 128MB
    }
}
```

Caddy 会自动申请证书。Nginx 可用 certbot：

```bash
certbot --nginx -d lol.example.com
```

> `LCW_BASE_URL` 必须与浏览器地址一致（协议 + 域名），否则 QQ 回调和 Secure Cookie 会异常。

## 4. QQ 互联配置（可选）

不配置 QQ 互联时，登录页只提供邮箱注册/登录，功能完整。需要 QQ 快捷登录再执行下面的步骤。

1. 在 <https://connect.qq.com/> 创建「网站应用」；
2. 回调地址填 `https://lol.example.com/auth/qq/callback`；
3. 把 AppID / AppKey 写入 `.env` 的 `LCW_QQ_APP_ID` / `LCW_QQ_APP_KEY`；
4. `docker compose up -d` 重建容器生效。

QQ 互联只返回 OpenID，不返回 QQ 号；用户首次进群时补全 QQ 号（群内展示用，可修改但群内唯一）。

配置 QQ 互联后，登录页默认突出 QQ 快捷登录，邮箱/密码入口折叠在下方。QQ 首次登录会直接创建只有 QQ 的账号并进入网站，不再要求设置邮箱和密码；已有邮箱账号仍可展开邮箱入口登录，之后可在「个人资料 → 账号」里绑定 QQ。

## 5. 白名单群管理

群白名单有两层：

- `LCW_ALLOWED_GROUPS`：启动引导白名单，始终允许（适合只维护固定几个群）。
- 数据库 `allowed_groups`：可在线增删，**立即生效、无需重启**。

```bash
cd /opt/lol-civil-war

# 添加（可带备注）
docker compose exec app /app/lol-civil-war group add 123456789 "周五内战群"

# 查看数据库白名单与环境变量白名单
docker compose exec app /app/lol-civil-war group list

# 移除
docker compose exec app /app/lol-civil-war group remove 123456789
```

> 与旧版不同：新群不需要改 `.env`、不需要重启。若某群还在 `LCW_ALLOWED_GROUPS` 中，`group remove` 后它仍会被环境变量放行。

## 6. 从旧版导入数据

`tools/legacy-import` 用于把 `lol-youmu-tools` 的 MySQL 数据迁移到新库（保留主键、DATETIME 转 UTC 毫秒、旧群自动加入白名单）。

```bash
go run ./tools/legacy-import \
  -source 'root:旧库密码@tcp(127.0.0.1:33062)/lolyoumu?charset=utf8mb4' \
  -target 'root:新库密码@tcp(127.0.0.1:3307)/lol_civil_war?charset=utf8mb4' \
  -truncate
```

- `-truncate` 会清空目标库业务表，执行前务必确认/备份；
- 旧库的 `sessions`、`cache`、`jobs` 不会导入；
- 导入完成后启动应用即可看到旧的组织、报名、对局、录像与评分。

## 7. 构建与发布镜像（ACR）

本地发布脚本会构建应用镜像并推送到阿里云 ACR：

```bash
# deploy/.env 中填写（不要提交）：
#   ACR_REGISTRY=registry.cn-shenzhen.aliyuncs.com
#   ACR_NAMESPACE=your-namespace
#   ACR_REPO=lol-civil-war
#   ACR_USERNAME=...
#   ACR_PASSWORD=...
scripts/publish.sh v0.2.0
```

国内构建加速（可选）：

```bash
DOCKER_GOPROXY=https://goproxy.cn,direct \
DOCKER_NPM_REGISTRY=https://registry.npmmirror.com \
scripts/publish.sh v0.2.0
```

远程部署（可选，脚本会登录 ACR、拉镜像、重启并健康检查）：

```bash
# deploy/.env.deploy：服务器与 ACR 信息
# deploy/.env.prod：远端运行配置（LCW_* / MYSQL_*）
scripts/deploy-remote.sh
```

## 8. 备份与恢复

MySQL 数据在命名卷 `lol-civil-war-mysql-data`。

备份（`--single-transaction` 不需要停应用）：

```bash
cd /opt/lol-civil-war
docker compose exec -T mysql sh -c \
  'exec mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" --single-transaction --routines --triggers "$MYSQL_DATABASE"' \
  > backup-$(date +%F).sql
```

恢复（建议先停应用，避免写入）：

```bash
docker compose stop app
docker compose exec -T mysql sh -c \
  'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' < backup-2026-10-21.sql
docker compose start app
```

上传的 `.rofl` 不落盘，无需单独备份。

## 9. 升级

```bash
cd /opt/lol-civil-war
docker compose pull app
docker compose up -d
```

数据库结构使用幂等的 `CREATE TABLE IF NOT EXISTS`，小版本升级无需手工迁移；涉及表结构变更时会在发布说明中注明。
MySQL 版本固定在 `.env` 的 `MYSQL_IMAGE`，升级数据库大版本前先备份。

## 10. 故障排查

| 现象 | 排查 |
| --- | --- |
| 应用反复重启 | `docker compose logs --tail=100 app`；通常是数据库连不上或口令不对 |
| MySQL 不健康 | `docker compose logs mysql`；确认 `.env` 的口令没有特殊字符未转义 |
| 访问 502 | `docker compose ps`；确认 `LCW_BIND` 与 proxy_pass 一致 |
| 登录后立刻掉线 | `LCW_BASE_URL` 的协议与访问协议是否一致；反向代理是否传 `X-Forwarded-Proto` |
| QQ 登录 403 / redirect_uri 不合法 | QQ 后台回调地址与 `LCW_QQ_REDIRECT_URI` 必须完全一致 |
| 写请求 419 | 浏览器没有带上 `XSRF-TOKEN`；确认同域访问、没有禁用 Cookie |
| 上传录像 413 | 反向代理 `client_max_body_size` 至少 128m |
| 头像不显示 | DataDragon CDN 不可达；可本地下载图标后挂载到 `/data/champions` |
| 群号被拒绝 | 把群号加入 `LCW_ALLOWED_GROUPS` 并重启 |

## 11. 安全清单

- [ ] `LCW_DEV_LOGIN=false`
- [ ] 使用 HTTPS，`LCW_COOKIE_SECURE=auto` 或 `true`
- [ ] `LCW_BIND=127.0.0.1:18020`，不要直接暴露到公网
- [ ] MySQL 不发布宿主端口（Compose 默认如此）
- [ ] `MYSQL_PASSWORD` / `MYSQL_ROOT_PASSWORD` 使用随机强口令
- [ ] 群白名单只保留实际使用的群
- [ ] `.env` 不提交、不共享；QQ AppKey 与数据库备份妥善保管
- [ ] 定期备份数据库卷
- [ ] 限制服务器 SSH 来源与权限
