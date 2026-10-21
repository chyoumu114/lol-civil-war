# 部署指南

目标：把服务部署到一台 Linux 服务器，用 HTTPS 域名对外提供访问。

## 1. 环境要求

- Linux + Docker 24+（可选 Docker Compose v2）
- 1 核 1G 起即可（SQLite 与 Go 常驻内存很小）
- 一个可用的域名；QQ 登录要求回调地址可访问（HTTPS）
- 宿主 Nginx 或 Caddy 负责 80/443 与证书

## 2. 准备配置

```bash
git clone https://github.com/chyoumu114/lol-civil-war.git
cd lol-civil-war/deploy
cp .env.example .env
```

编辑 `.env`，至少填写：

| 变量 | 示例 | 说明 |
| --- | --- | --- |
| `LCW_ALLOWED_GROUPS` | `123456789,987654321` | 群白名单，逗号分隔 |
| `LCW_BASE_URL` | `https://lol.example.com` | 对外地址 |
| `LCW_QQ_APP_ID` / `LCW_QQ_APP_KEY` | — | QQ 互联凭据 |
| `LCW_DEV_LOGIN` | `false` | 生产必须关闭 |
| `LCW_BIND` | `127.0.0.1:18020` | 只监听本机，交给反向代理 |

> `.env` 含凭据，已在 `.gitignore` 中；不要提交。

## 3. 启动

```bash
docker compose up -d
docker compose ps
curl -fsS http://127.0.0.1:18020/api/health
```

看到 `{"ok":true,...}` 即启动成功。数据在 `lol-civil-war-data` 卷里。

如果没有仓库、只想部署现成镜像：

```bash
docker run -d --name lol-civil-war \
  -p 127.0.0.1:18020:8080 \
  -v lol-civil-war-data:/data \
  -e LCW_ALLOWED_GROUPS=123456789 \
  -e LCW_BASE_URL=https://lol.example.com \
  -e LCW_QQ_APP_ID=你的AppID \
  -e LCW_QQ_APP_KEY=你的AppKey \
  <镜像地址>:<tag>
```

## 4. 反向代理与 HTTPS

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

## 5. QQ 互联配置

1. 在 <https://connect.qq.com/> 创建「网站应用」；
2. 回调地址填 `https://lol.example.com/auth/qq/callback`；
3. 把 AppID / AppKey 写入 `.env` 的 `LCW_QQ_APP_ID` / `LCW_QQ_APP_KEY`；
4. `docker compose up -d` 重建容器生效。

QQ 互联只返回 OpenID，不返回 QQ 号；用户首次进群时补全 QQ 号（群内展示用）。

## 6. 构建与发布镜像（ACR）

本地发布脚本会构建镜像并推送到阿里云 ACR：

```bash
# deploy/.env 中填写（不要提交）：
#   ACR_REGISTRY=registry.cn-shenzhen.aliyuncs.com
#   ACR_NAMESPACE=your-namespace
#   ACR_REPO=lol-civil-war
#   ACR_USERNAME=...
#   ACR_PASSWORD=...
scripts/publish.sh v1.0.0
```

国内构建加速（可选）：

```bash
DOCKER_GOPROXY=https://goproxy.cn,direct \
DOCKER_NPM_REGISTRY=https://registry.npmmirror.com \
scripts/publish.sh v1.0.0
```

远程部署（可选，脚本会登录 ACR、拉镜像、重启并健康检查）：

```bash
# deploy/.env.deploy：服务器与 ACR 信息
# deploy/.env.prod：远端运行配置（LCW_*）
scripts/deploy-remote.sh
```

## 7. 备份与恢复

数据只有 SQLite 一个文件。最稳妥的方式是停容器后复制：

```bash
docker compose stop app
docker run --rm -v lol-civil-war-data:/data -v "$PWD":/backup alpine \
  sh -c 'cp /data/lol-civil-war.db /backup/lol-civil-war-$(date +%F).db'
docker compose start app
```

也可以在线备份（需要 sqlite3）：

```bash
docker run --rm -v lol-civil-war-data:/data -v "$PWD":/backup alpine \
  sh -c 'apk add --no-cache sqlite >/dev/null && sqlite3 /data/lol-civil-war.db ".backup /backup/lol-civil-war-$(date +%F).db"'
```

恢复：停容器 → 把备份复制回卷内的 `/data/lol-civil-war.db`（同时删除 `-wal`/`-shm`）→ 启动。

## 8. 升级

```bash
git pull
cd deploy
docker compose pull     # 如果用远程镜像
docker compose up -d
```

数据库结构使用 `CREATE TABLE IF NOT EXISTS`，小版本升级无需手工迁移；涉及表结构变更时会在发布说明中注明。

## 9. 故障排查

| 现象 | 排查 |
| --- | --- |
| 访问 502 | `docker compose ps` / `docker compose logs --tail=100 app`；确认 `LCW_BIND` 与 proxy_pass 一致 |
| 登录后立刻掉线 | `LCW_BASE_URL` 的协议与访问协议是否一致；反向代理是否传 `X-Forwarded-Proto` |
| QQ 登录 403 / redirect_uri 不合法 | QQ 后台回调地址与 `LCW_QQ_REDIRECT_URI` 必须完全一致 |
| 写请求 419 | 浏览器没有带上 `XSRF-TOKEN`；确认同域访问、没有禁用 Cookie |
| 上传录像 413 | 反向代理 `client_max_body_size` 至少 128m |
| 头像不显示 | DataDragon CDN 不可达；可本地下载图标后挂载到 `/data/champions` |
| 群号被拒绝 | 把群号加入 `LCW_ALLOWED_GROUPS` 并重启 |

## 10. 安全清单

- [ ] `LCW_DEV_LOGIN=false`
- [ ] 使用 HTTPS，`LCW_COOKIE_SECURE=auto` 或 `true`
- [ ] `LCW_BIND=127.0.0.1:18020`，不要直接暴露到公网
- [ ] 群白名单只保留实际使用的群
- [ ] `.env` 不提交、不共享；QQ AppKey 与数据库备份妥善保管
- [ ] 定期备份数据卷
- [ ] 限制服务器 SSH 来源与权限
