# 小白版部署教程

这份教程面向**第一次在 Linux 服务器上部署项目**的人。尽量按顺序执行，不要跳步。

如果你已经熟悉 Docker、反向代理和 HTTPS，可以直接看更短的 [部署指南](deploy.md)。

## 0. 先确认你手里有什么

建议准备：

- 一台 Linux 服务器，Ubuntu 22.04/24.04 或 Debian 12 都可以；
- 一个能 `sudo` 的账号；
- 源码构建建议 **2 核 CPU、2GB 内存、10GB 磁盘**；1GB 内存可能在前端构建时失败；
- 正式上线需要一个域名；
- 云服务器安全组放行 **22、80、443**，不要放行 3306 和 18020。

没有域名也可以先部署，通过 SSH 隧道在本机浏览器验证；正式上线再配域名和 HTTPS。

先检查资源：

```bash
free -h
df -h
```

如果是 1GB 内存，建议先加 2GB swap，再构建：

```bash
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
free -h
```

## 1. 安装基础工具和 Docker

先安装 Git、curl、证书和 openssl：

```bash
sudo apt update
sudo apt install -y git curl ca-certificates openssl
```

安装 Docker Engine 和 Compose 插件。可以使用 Docker 官方安装脚本：

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker "$USER"
newgrp docker
```

`newgrp` 会进入一个新的 shell；如果提示找不到命令，先退出 SSH 再重新登录，然后检查：

```bash
docker version
docker compose version
```

`docker compose version` 必须能输出版本号。只有 `docker` 没有 `compose` 不行。

> 国内网络如果下载 Docker 很慢，优先使用云厂商的 Docker 镜像或预装 Docker 的系统镜像；不要随便执行来源不明的安装脚本。

## 2. 拉取代码

```bash
sudo mkdir -p /opt
sudo git clone https://github.com/chyoumu114/lol-civil-war.git /opt/lol-civil-war
sudo chown -R "$USER":"$(id -gn)" /opt/lol-civil-war
cd /opt/lol-civil-war/deploy
```

复制配置：

```bash
cp .env.example .env
chmod 600 .env
```

`.env` 里有数据库密码，不要提交到 Git，也不要把内容发到公开聊天。

## 3. 填写 `.env`

用编辑器打开：

```bash
vi .env
```

至少修改下面几项：

```env
# 群白名单：QQ 群号，或任何稳定的数字群 ID；留空会拒绝所有群
LCW_ALLOWED_GROUPS=123456789

# 第一阶段先用本机地址，后面配好 HTTPS 再改成域名
LCW_BASE_URL=http://127.0.0.1:18020

# 生产必须关闭
LCW_DEV_LOGIN=false

# 数据库密码，两个都要换成随机强口令
MYSQL_PASSWORD=请替换成随机口令
MYSQL_ROOT_PASSWORD=请替换成另一个随机口令
```

生成随机口令：

```bash
openssl rand -base64 24
```

执行两次，分别填到 `MYSQL_PASSWORD` 和 `MYSQL_ROOT_PASSWORD`。

说明：

- `LCW_ALLOWED_GROUPS` 是启动白名单，不是“所有群”。可以填多个，用逗号分隔；具体数字 ID 可以先用一个测试值，之后用 `group add` 添加正式群。
- `LCW_BASE_URL` 必须和浏览器实际访问地址一致。先用隧道就用 `http://127.0.0.1:18020`，上线后改成 `https://你的域名`。
- QQ 快捷登录是可选的，不配置也能用邮箱注册；不需要就不要填 `LCW_QQ_*`。
- `LCW_IMAGE` 保持默认 `lol-civil-war:latest` 表示从源码构建；如果你拿到发布方提供的预构建镜像，再改成对应地址。

国内网络建议在 `.env` 打开：

```env
DOCKER_GOPROXY=https://goproxy.cn,direct
DOCKER_NPM_REGISTRY=https://registry.npmmirror.com
```

这只加速 Go/npm 依赖；如果 Docker Hub 基础镜像拉不下来，需要配置 Docker 镜像加速，或者改用预构建镜像。

## 4. 启动

### 路线 A：从源码构建（默认）

```bash
docker compose up -d --build
```

第一次会拉基础镜像、安装依赖、构建前端和 Go 二进制，通常需要几分钟。不要看到暂时没输出就按 Ctrl-C。

查看状态：

```bash
docker compose ps
```

两个服务都应该是 `running`，MySQL 最终应为 `healthy`。如果 app 一直重启，先看日志：

```bash
docker compose logs -f --tail=100 app
docker compose logs -f --tail=100 mysql
```

### 路线 B：使用预构建镜像（服务器配置低/网络差）

如果你拿到了发布方的镜像地址，在 `.env` 里设置：

```env
LCW_IMAGE=<registry>/<namespace>/lol-civil-war:<tag>
```

然后：

```bash
docker compose pull
docker compose up -d --no-build
```

不要再用 `--build`，否则又会走源码构建。

如果镜像仓库是私有的，先执行 `docker login <registry>`。

## 5. 第一次验证

先确认服务本身正常：

```bash
curl -fsS http://127.0.0.1:18020/api/health
```

应该返回类似：

```json
{"ok":true,"database":"ok","version":"..."}
```

注意：**应用的 18020 默认只监听服务器本机**，所以：

- 在服务器上 `curl 127.0.0.1:18020` 能通；
- 在你自己的电脑浏览器打开 `http://服务器IP:18020` **打不开**，这是正常的；
- 不要为了图省事把 18020 直接暴露到公网。

### 用 SSH 隧道在浏览器验证

在你的电脑上执行（不是服务器上）：

```bash
ssh -L 18020:127.0.0.1:18020 用户名@服务器IP
```

保持这个 SSH 窗口不要关，然后在本机浏览器打开：

```text
http://localhost:18020/
```

注册一个邮箱账号，按页面提示创建/进入群。如果提示群不允许，检查 `.env` 的 `LCW_ALLOWED_GROUPS` 是否包含你使用的群 ID。

## 6. 配置域名和 HTTPS

正式对外使用时，建议用 Caddy，配置比 Nginx 简单，证书自动申请。

### 6.1 域名解析

在域名服务商处添加 A 记录：

```text
lol.example.com  →  你的服务器公网 IP
```

等待解析生效，并在云服务器安全组放行 80、443。

### 6.2 安装 Caddy

Ubuntu/Debian 可用官方仓库：

```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install -y caddy
```

如果官方源不可达，按 [Caddy 官方安装文档](https://caddyserver.com/docs/install) 选择适合你系统的方式。

编辑配置：

```bash
sudo vi /etc/caddy/Caddyfile
```

写入（把域名换成你的）：

```caddyfile
lol.example.com {
    reverse_proxy 127.0.0.1:18020
}
```

检查并重载：

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
sudo systemctl status caddy --no-pager
```

### 6.3 修改应用地址

把 `.env` 改成：

```env
LCW_BASE_URL=https://lol.example.com
```

重新创建应用容器（不需要重新构建）：

```bash
docker compose up -d
```

然后访问：

```text
https://lol.example.com/
```

如果证书申请失败，优先检查：

- DNS 是否已经指向这台服务器；
- 云安全组和系统防火墙是否放行 80/443；
- `LCW_BASE_URL` 是否和实际域名完全一致。

Nginx 的配置方式见 [部署指南](deploy.md#3-反向代理与-https)。

### 6.4 系统防火墙

如果使用 ufw，至少放行 SSH、80、443：

```bash
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
sudo ufw status
```

不要放行 3306（MySQL）和 18020（应用回环端口）。

## 7. QQ 快捷登录（可选）

不配置 QQ 也能用邮箱完整注册和登录。需要 QQ 快捷登录时：

1. 在 [QQ 互联](https://connect.qq.com/) 创建网站应用；
2. 回调地址填 `https://你的域名/auth/qq/callback`；
3. 把 AppID/AppKey 填到 `.env` 的 `LCW_QQ_APP_ID` / `LCW_QQ_APP_KEY`；
4. `docker compose up -d` 重建容器。

配置成功后登录页默认显示 QQ 快捷登录，邮箱/密码入口折叠。首次 QQ 登录会直接创建账号并进入，不需要额外设置邮箱和密码；已有邮箱账号仍可展开邮箱入口登录。

## 8. 常见报错

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| `docker: command not found` | 没装 Docker | 回到第 1 步安装 |
| `permission denied ... docker.sock` | 当前用户不在 docker 组 | 执行 `sudo usermod -aG docker "$USER"` 后重新登录 |
| `docker: 'compose' is not a docker command` | 只有 Docker 没有 Compose 插件 | 安装 Docker Compose v2 |
| 拉取 `node/golang/mysql` 超时 | Docker Hub 网络问题 | 配镜像加速，或改用预构建镜像 |
| `port is already allocated` | 18020 被占用 | `ss -ltnp \| grep 18020`，停掉冲突服务或改 `LCW_BIND` |
| `curl 127.0.0.1:18020` 拒绝连接 | app 没启动/未 healthy | `docker compose ps`、`docker compose logs app` |
| 服务器上 curl 正常，浏览器打不开 | 只监听回环地址 | 用 SSH 隧道，或配 Caddy 反向代理 |
| HTTPS 502 | Caddy/Nginx 连不到 app | 确认 `curl 127.0.0.1:18020/api/health` 正常 |
| 提示群不允许 | 白名单没包含群 ID | 修改 `LCW_ALLOWED_GROUPS`，或执行 `group add` |
| MySQL unhealthy | 密码/卷/内存问题 | `docker compose logs mysql`；不要随意删卷 |

## 9. 更新与备份

更新代码前先备份 `.env` 和数据库。数据库备份、升级和恢复的完整步骤见 [部署指南](deploy.md)。

普通更新：

```bash
cd /opt/lol-civil-war
git pull
cd deploy
docker compose up -d --build
```

> `docker compose down` 不会删数据卷；`docker compose down -v` 会删数据库卷，除非你明确要清空数据，否则不要加 `-v`。

## 10. 上线前检查清单

- [ ] `docker compose ps` 中 app 正常、mysql healthy
- [ ] `curl http://127.0.0.1:18020/api/health` 返回 `ok: true`
- [ ] `.env` 里的 MySQL 密码已改成随机强口令
- [ ] `LCW_DEV_LOGIN=false`
- [ ] `LCW_ALLOWED_GROUPS` 包含正确的群 ID
- [ ] `LCW_BASE_URL` 和浏览器地址一致
- [ ] 域名解析、80/443、HTTPS 证书正常
- [ ] 18020 和 3306 没有对公网开放
- [ ] 已备份 `.env`，并知道数据库卷的位置
