# LOL 内战工具（lol-civil-war）

面向 QQ 群的《英雄联盟》5v5 自定义内战组织工具：报名、自动战力平衡、对局记录、赛后评分与排行榜。

后端由 Go 1.26 重写，单二进制 + MySQL 8，内置前端静态资源；按顺序执行几条命令即可完成部署。

## 功能

- **QQ 登录**：QQ 互联 OAuth 2.0；本地开发可用开发登录。
- **按群隔离**：所有数据按 QQ 群隔离，URL 使用随机 key，不暴露群号。
- **群内资料**：昵称、QQ 号（首次填写后锁定）、最高段位、擅长位置。
- **内战组织**：自定义标题、大区、BO1/BO3/BO5、是否 BP、报名截止/开赛时间、备注。
- **报名与替补**：主选/补位位置 + 两套段位；截止前可自行修改，截止后由组织者代改。
- **自动战力平衡**：按段位、大区系数、位置影响力计算战力，回溯搜索输出 5 套最平衡阵容。
- **对局记录**：红蓝双方阵容、KDA/补刀/经济/伤害、BP 禁用；上传 `.rofl` 自动填充战绩。
- **结束比赛**：组织者统一锁定记录；之后报名者/观战者可对每局出场玩家打 1–5 星。
- **排行榜与个人分析**：评分、KDA、胜率、场均数据、最佳/最差队友、最近对局。
- **PC / 手机自适应**：Vue 3 + Arco Design Vue。

## 技术栈

| 组件 | 选型 |
| --- | --- |
| 后端 | Go 1.26.x（标准库 `net/http`，无 Web 框架） |
| 数据库 | MySQL 8.0+ / 8.4 LTS（驱动 `github.com/go-sql-driver/mysql`） |
| 前端 | Vue 3 + Arco Design Vue + Vite，构建产物嵌入二进制 |
| 部署 | Docker Compose：应用容器 + MySQL 容器 |

## 部署（从 GitHub 拉取，按顺序执行）

```bash
# 1. 拉取代码
git clone https://github.com/chyoumu114/lol-civil-war.git
cd lol-civil-war/deploy

# 2. 准备配置
cp .env.example .env

# 3. 编辑 .env：至少填写
#    LCW_ALLOWED_GROUPS=你的QQ群号
#    LCW_BASE_URL=https://你的域名
#    MYSQL_PASSWORD=随机强口令
#    MYSQL_ROOT_PASSWORD=随机强口令
#    （QQ 登录还需 LCW_QQ_APP_ID / LCW_QQ_APP_KEY）

# 4. 启动（首次会自动拉取镜像并初始化数据库）
docker compose up -d

# 5. 检查
docker compose ps
curl -fsS http://127.0.0.1:18020/api/health
```

浏览器访问 `http://localhost:18020/`。数据在 `lol-civil-war-mysql-data` 卷里。

> 应用镜像默认使用 `lol-civil-war:latest`。生产建议改为你发布到镜像仓库的具体 tag：
> `LCW_IMAGE=<registry>/<namespace>/lol-civil-war:v0.2.0`。

### 从源码运行

需要 Go 1.26+、Node.js 22+、一个可用的 MySQL 8 实例与数据库：

```bash
make build

export LCW_DB_HOST=127.0.0.1 LCW_DB_PORT=3306 \
       LCW_DB_USER=lcw LCW_DB_PASSWORD=你的密码 LCW_DB_NAME=lol_civil_war

LCW_ALLOWED_GROUPS=123456789 LCW_DEV_LOGIN=true ./bin/lol-civil-war
```

首次启动会自动建表；数据库需要事先创建（字符集 `utf8mb4`）。

### 本地开发

```bash
# 终端 1：后端（监听 8080，开启开发登录）
LCW_ALLOWED_GROUPS=123456789 LCW_DEV_LOGIN=true \
LCW_DB_HOST=127.0.0.1 LCW_DB_USER=lcw LCW_DB_PASSWORD=你的密码 LCW_DB_NAME=lol_civil_war \
make run

# 终端 2：前端热更新（5173，代理 /api 与 /auth 到 8080）
cd web && npm install && npm run dev
```

开发登录：

```
http://localhost:8080/auth/dev?qq=10001&nickname=测试玩家&redirect=/
```

> 生产环境必须保持 `LCW_DEV_LOGIN=false`，并使用 HTTPS。

## 配置

应用通过环境变量（或 `.env`）配置；Compose 会读取 `deploy/.env` 用于插值和容器环境。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LCW_ADDR` | `:8080` | 监听地址 |
| `LCW_DATA_DIR` | `./data` | 数据目录（上传临时文件、可选图标） |
| `LCW_BASE_URL` | `http://localhost:8080` | 对外访问地址，影响 QQ 回调与 Cookie Secure |
| `LCW_TIMEZONE` | `Asia/Shanghai` | 展示时区 |
| `LCW_ALLOWED_GROUPS` | 空 | 群号白名单，逗号分隔；**为空时拒绝所有群** |
| `LCW_DEV_LOGIN` | `false` | 是否开放 `/auth/dev` |
| `LCW_QQ_APP_ID` | 空 | QQ 互联 AppID |
| `LCW_QQ_APP_KEY` | 空 | QQ 互联 AppKey |
| `LCW_QQ_REDIRECT_URI` | `${BASE_URL}/auth/qq/callback` | QQ 回调地址 |
| `LCW_SESSION_TTL_HOURS` | `720` | 登录会话有效期 |
| `LCW_ROFL_MAX_KB` | `131072` | 录像上传上限（128MB） |
| `LCW_COOKIE_SECURE` | `auto` | `auto` 时按 `LCW_BASE_URL` 是否为 https 决定 |
| `LCW_TRUST_PROXY` | `true` | 是否信任 `X-Forwarded-*` |
| `LCW_CHAMPION_ICON_DIR` | `${DATA_DIR}/champions` | 可选本地英雄图标目录 |
| `LCW_DB_DSN` | 空 | 完整 MySQL DSN；设置后忽略下面的分项 |
| `LCW_DB_HOST` / `LCW_DB_PORT` | `127.0.0.1` / `3306` | MySQL 地址 |
| `LCW_DB_USER` / `LCW_DB_PASSWORD` | `root` / 空 | MySQL 账号 |
| `LCW_DB_NAME` | `lol_civil_war` | 数据库名 |

Compose 额外使用：`MYSQL_DATABASE`、`MYSQL_USER`、`MYSQL_PASSWORD`、`MYSQL_ROOT_PASSWORD`、`MYSQL_IMAGE`、`LCW_IMAGE`、`LCW_BIND`。

### QQ 互联

1. 在 [QQ 互联开放平台](https://connect.qq.com/) 创建网站应用；
2. 回调地址填 `https://你的域名/auth/qq/callback`；
3. 把 AppID / AppKey 写入 `LCW_QQ_APP_ID` / `LCW_QQ_APP_KEY`；
4. 设置 `LCW_BASE_URL=https://你的域名` 并重启容器。

QQ 互联只返回 OpenID，不返回 QQ 号。本项目用 OpenID 识别账号，用户首次进入某个群时补全 5–12 位 QQ 号（群内展示用，首次填写后不可修改）。

## 部署拓扑

```
浏览器 → Nginx/Caddy (443) → 127.0.0.1:18020 → app 容器
                                                   └── mysql:3306（内部网络）
                                                         └── 卷 lol-civil-war-mysql-data
```

HTTPS、反向代理、备份、升级与故障排查见 [docs/deploy.md](docs/deploy.md)。

## 测试

```bash
# 启动一个测试用 MySQL 8
docker run -d --name lcw-mysql-test -e MYSQL_ROOT_PASSWORD=root \
  -p 127.0.0.1:3307:3306 mysql:8.4

# 单元 + HTTP 集成测试（每个测试自动建独立库并清理）
LCW_TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3307)/' go test ./...

# 端到端（需要服务开启开发登录）
python3 scripts/e2e.py --base http://127.0.0.1:8080 --group 123456789
```

## 目录结构

```
cmd/lol-civil-war/       程序入口
internal/config/         环境变量与 .env 配置
internal/domain/         纯业务规则：段位/战力、赛制、自动匹配、ROFL 解析、英雄库
internal/store/          MySQL 持久化（schema.sql + 各聚合的查询）
internal/service/        应用服务：权限、事务、DTO 组装
internal/web/            HTTP 路由、会话、CSRF、限流、静态资源
internal/webui/          内嵌前端产物（dist 由构建生成）
web/                     Vue 3 前端源码
deploy/                  Dockerfile、compose、配置模板
docs/                    架构、接口、部署文档
scripts/                 发布 / 远程部署 / 端到端验收 / 图标下载
```

## 数据与隐私

- 业务数据在 MySQL；备份与恢复见部署文档。
- 上传的 `.rofl` **不落盘**，只保存 sha256 与解析结果。
- 接口对 QQ 号做脱敏（保留前段，隐藏最后三位）；群号不出现在 URL 中。
- 仓库不包含任何真实群号、服务器、域名或凭据；部署时通过环境变量注入。

## 已知限制

- 默认单实例部署；会话存 MySQL，但限流在进程内，多副本需要额外的共享限流。
- ROFL 录像从游戏加载开始记录，**不包含 Ban/Pick**；BP 需手动记录。
- 仅支持 14.11+ 的 ROFL2 录像；14.9–14.10 录像无元数据。
- 报名规模很大时自动匹配可能触及搜索预算（默认 300 万节点），结果不保证全局最优。
- 仓库不含英雄头像（Riot 版权素材）；默认使用 DataDragon CDN，离线环境可用 `scripts/fetch-champion-icons.py` 下载到本地目录。

## 许可与声明

本项目使用 [GPL-3.0](LICENSE) 许可。

本工具与 Riot Games 无关联；League of Legends、英雄名称与相关素材是 Riot Games, Inc. 的商标/财产，不属于 GPL 授权范围。详见 [NOTICE](NOTICE)。
