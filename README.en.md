# LOL Group In-House Tournament Registration Tool

[简体中文](README.md) | [English](README.en.md)

<p align="center">
  <img src="docs/images/events.png" width="32%" alt="Event registration and roster" />
  <img src="docs/images/rank.png" width="32%" alt="Leaderboard" />
  <img src="docs/images/info.png" width="32%" alt="Match details" />
</p>

A 5v5 custom-game organizer for League of Legends communities — QQ groups, WeChat groups, Discord servers, and any other group. It covers registration, automatic team balancing, match records, post-game ratings, and leaderboards.

The backend was rewritten in Go 1.27.3 as a single binary with MySQL 8 and an embedded frontend. A few commands are enough for a complete deployment.

## Features

- **Email accounts**: register and sign in with an email address and password; QQ is not required. Every account gets a system-generated unique `user_no`.
- **Optional OAuth quick login**: QQ Connect is supported as an optional provider for mainland-China deployments; other communities can use email or replace it with their own OAuth provider. On first use, the user completes an email and password; an existing email is verified and linked, while an email account can link QQ later from the profile page.
- **Per-group isolation**: all data is isolated by group, and URLs use a random key instead of exposing the group ID.
- **Whitelist management**: bootstrap through an environment variable and manage groups in the database; groups can be added or removed online without a restart.
- **Member profiles**: nickname, member ID (labeled as QQ number in the current Chinese UI; editable and unique within the group), peak rank, and preferred positions.
- **Event organization**: custom title, region, BO1/BO3/BO5, optional draft (BP), registration deadline, start time, and notes.
- **Registration and substitutes**: primary/secondary positions and two rank values; players can edit before the deadline, and organizers can edit afterward.
- **Automatic balancing**: computes strength from rank, region coefficient, and position impact, then runs a backtracking search for the five most balanced lineups.
- **Match records**: blue/red lineups, KDA/CS/gold/damage, bans, and automatic stat filling from uploaded `.rofl` files.
- **Match finalization**: organizers lock the record; players and spectators can then rate each participant from 1 to 5 stars per game.
- **Leaderboard and player analysis**: ratings, KDA, win rate, per-game averages, best/worst teammates, and recent matches.
- **Responsive PC/mobile UI**: Vue 3 + Arco Design Vue.

## Tech stack

| Component | Choice |
| --- | --- |
| Backend | Go 1.27.3 (standard library `net/http`, no web framework) |
| Database | MySQL 8.0+ / 8.4 LTS (driver `github.com/go-sql-driver/mysql`) |
| Frontend | Vue 3 + Arco Design Vue + Vite; build output is embedded in the binary |
| Deployment | Docker Compose: application container + MySQL container |

## Deployment (from GitHub, run in order)

> **Localization note**
>
> To keep the codebase simple, the UI and backend messages are currently Chinese only. If you deploy for an English-speaking or other non-Chinese community, use an AI to translate the project's Chinese text into your target language before building and deploying. This English README is a documentation translation, not a runtime language pack.

```bash
# 1. Clone the repository
git clone https://github.com/chyoumu114/lol-civil-war.git
cd lol-civil-war/deploy

# 2. Prepare configuration
cp .env.example .env

# 3. Edit .env; at minimum set:
#    LCW_ALLOWED_GROUPS=YOUR_GROUP_ID
#    LCW_BASE_URL=https://your-domain.example
#    MYSQL_PASSWORD=a-strong-random-password
#    MYSQL_ROOT_PASSWORD=a-strong-random-password
#    (optional OAuth quick login; QQ Connect in mainland China: LCW_QQ_APP_ID / LCW_QQ_APP_KEY)

# 4. Start (builds the image, starts MySQL, and creates tables)
docker compose up -d --build

# 5. Check
docker compose ps
curl -fsS http://127.0.0.1:18020/api/health
```

Open `http://localhost:18020/` and click **Register** to create an email account. Data is stored in the `lol-civil-war-mysql-data` volume.

`YOUR_GROUP_ID` is any stable numeric ID used to isolate a community. For a QQ group, use the QQ group number; for Discord, WeChat, or another platform, use an equivalent numeric group ID and translate the QQ-specific labels in the UI as needed.

> Compose includes a `build` configuration. When deploying from source, use `docker compose up -d --build`.
> To deploy a prebuilt image instead, use `docker compose up -d` with:
> `LCW_IMAGE=<registry>/<namespace>/lol-civil-war:v0.2.0`.

### Run from source

Requirements: Go 1.27.3+, Node.js 22.23.4+, and a reachable MySQL 8 instance with a database:

```bash
make build

export LCW_DB_HOST=127.0.0.1 LCW_DB_PORT=3306 \
       LCW_DB_USER=lcw LCW_DB_PASSWORD=your-password LCW_DB_NAME=lol_civil_war

LCW_ALLOWED_GROUPS=123456789 LCW_DEV_LOGIN=true ./bin/lol-civil-war
```

Tables are created automatically on first start. Create the database beforehand with the `utf8mb4` character set.

### Local development

```bash
# Terminal 1: development MySQL (127.0.0.1:3307) + backend (8080)
make dev

# Terminal 2: frontend hot reload (5173; proxies /api and /auth to 8080)
cd web && npm install && npm run dev
```

`make dev` starts `lol-civil-war-mysql-dev` through Docker (port 3307, database `lol_civil_war_dev`).
If you already have MySQL, point the backend at it and skip Docker:

```bash
LCW_DB_HOST=127.0.0.1 LCW_DB_PORT=3306 \
LCW_DB_USER=lcw LCW_DB_PASSWORD=your-password LCW_DB_NAME=lol_civil_war \
make dev
```

If the backend is not on port 8080, override the frontend proxy target:

```bash
VITE_API_TARGET=http://127.0.0.1:18020 npm run dev
```

Development login:

```text
http://localhost:8080/auth/dev?qq=10001&nickname=TestPlayer&redirect=/
```

> Production must keep `LCW_DEV_LOGIN=false` and use HTTPS.

## Configuration

The application is configured through environment variables (or `.env`). Compose reads `deploy/.env` for interpolation and container environment variables.

| Variable | Default | Description |
| --- | --- | --- |
| `LCW_ADDR` | `:8080` | Listen address |
| `LCW_DATA_DIR` | `./data` | Data directory (temporary uploads and optional icons) |
| `LCW_BASE_URL` | `http://localhost:8080` | Public base URL; affects QQ callback and Secure cookies |
| `LCW_TIMEZONE` | `Asia/Shanghai` | Display time zone |
| `LCW_ALLOWED_GROUPS` | empty | Bootstrap group whitelist, comma-separated; **all groups are rejected when empty** |
| `LCW_DEV_LOGIN` | `false` | Expose `/auth/dev` |
| `LCW_QQ_APP_ID` | empty | Optional QQ Connect AppID (mainland China); enables quick login when set |
| `LCW_QQ_APP_KEY` | empty | Optional QQ Connect AppKey (mainland China) |
| `LCW_QQ_REDIRECT_URI` | `${BASE_URL}/auth/qq/callback` | QQ callback URL |
| `LCW_SESSION_TTL_HOURS` | `720` | Session lifetime |
| `LCW_ROFL_MAX_KB` | `131072` | Replay upload limit (128 MB) |
| `LCW_COOKIE_SECURE` | `auto` | With `auto`, derived from the `LCW_BASE_URL` scheme |
| `LCW_TRUST_PROXY` | `true` | Trust `X-Forwarded-*` headers |
| `LCW_CHAMPION_ICON_DIR` | `${DATA_DIR}/champions` | Local champion avatar directory |
| `LCW_DDRAGON_VERSION` | empty (auto-detect) | DataDragon version; the deployment template pins `16.20.1` |
| `LCW_DB_DSN` | empty | Full MySQL DSN; when set, the fields below are ignored |
| `LCW_DB_HOST` / `LCW_DB_PORT` | `127.0.0.1` / `3306` | MySQL address |
| `LCW_DB_USER` / `LCW_DB_PASSWORD` | `root` / empty | MySQL credentials |
| `LCW_DB_NAME` | `lol_civil_war` | Database name |

Compose also uses: `MYSQL_DATABASE`, `MYSQL_USER`, `MYSQL_PASSWORD`, `MYSQL_ROOT_PASSWORD`, `MYSQL_IMAGE`, `LCW_IMAGE`, and `LCW_BIND`.

### Accounts and optional OAuth login

Email registration is the portable path and works for any community. When QQ Connect is not configured, the login page only offers email registration/sign-in; all features remain available.

QQ Connect is one optional OAuth provider, mainly intended for deployments in mainland China. For Discord, WeChat, or other communities, use email accounts or replace QQ Connect with your own OAuth provider. The current Chinese UI labels the member identifier as “QQ number”; translate or adapt that label if your community does not use QQ.

#### QQ Connect (optional, mainland China)

1. Create a website application on [QQ Connect](https://connect.qq.com/).
2. Set the callback URL to `https://your-domain.example/auth/qq/callback`.
3. Put the AppID/AppKey into `LCW_QQ_APP_ID` / `LCW_QQ_APP_KEY`.
4. Set `LCW_BASE_URL=https://your-domain.example` and restart the container.

Rules:

- Every account has a generated unique `user_no`; a QQ OpenID is only an optional binding.
- Email registration does not invent a nickname. Nickname and the member ID (QQ number in the current UI) are required and completed on first joining a group.
- On first QQ quick registration, the group nickname defaults to the nickname returned by QQ.
- **First** QQ quick login redirects to a “complete registration” page and requires email and password:
  - Email does not exist → create a new account and bind QQ;
  - Email exists → verify the password, bind QQ if unbound, and reject an account already bound to another QQ.
- Email users can use quick login after choosing **Profile → Account → Bind QQ**.
- Legacy QQ accounts are asked to complete an email and password on first login, then keep the original account and group data.

QQ Connect returns only an OpenID, not a QQ number. The 5–12 digit member number is entered by the user; it can be changed but must be unique within the group. For non-QQ deployments, treat this field as a member ID.

### Group whitelist management

`LCW_ALLOWED_GROUPS` is the bootstrap whitelist (always allowed; useful when maintaining a single group). For day-to-day additions, use the database whitelist; changes take effect immediately without a restart:

```bash
# Docker deployment (run in the deploy directory)
docker compose exec app /app/lol-civil-war group add 123456789
docker compose exec app /app/lol-civil-war group list
docker compose exec app /app/lol-civil-war group remove 123456789

# Running from source (with LCW_DB_* environment variables)
go run ./cmd/lol-civil-war group add 123456789
```

`group add` accepts an optional note: `group add 123456789 "Friday in-house"`.

## Champion avatars and champion library updates

Champion avatars and data come from the **DataDragon CDN**:

```text
https://ddragon.leagueoflegends.com/cdn/
```

Avatar files are not committed to the repository. The loading order is:

```text
DataDragon CDN → server-local /champions/{key}.png → first character of the champion name
```

The server-local directory defaults to `/data/champions` and is configured by `LCW_CHAMPION_ICON_DIR`.

Update the champion library and avatars:

```bash
# Update to the default 16.20.1
make champions

# Pin a version
make champions CHAMPION_VERSION=16.20.1

# Or call the script directly
python3 scripts/update-champions.py --version 16.20.1
```

The command:

1. Fetches DataDragon `champion.json`;
2. Updates the version, champion names, and titles in `internal/domain/champion/library.json`;
3. Appends newly released champions (`positions`/`aliases` are empty and can be filled manually);
4. Downloads avatars to `data/champions/`.

Rebuild after updating the library, and set the deployment env `LCW_DDRAGON_VERSION` to the same version (or leave it empty for auto-detection):

```bash
make build
```

Copy avatars into the container volume in production:

```bash
docker cp data/champions/. lol-civil-war:/data/champions/
```

## Importing legacy data

The repository includes `tools/legacy-import`, which imports data from the older `lol-youmu-tools` (Laravel) MySQL database, preserves primary keys, converts timestamps, and adds the old groups to the whitelist:

```bash
# Back up the target database first; -truncate clears the target business tables
go run ./tools/legacy-import \
  -source 'root:old-password@tcp(127.0.0.1:33062)/lolyoumu?charset=utf8mb4' \
  -target 'root:new-password@tcp(127.0.0.1:3307)/lol_civil_war?charset=utf8mb4' \
  -truncate
```

## Deployment topology

```text
Browser → Nginx/Caddy (443) → 127.0.0.1:18020 → app container
                                                   └── mysql:3306 (internal network)
                                                         └── volume lol-civil-war-mysql-data
```

HTTPS, reverse proxy, backup, upgrade, and troubleshooting details are in [docs/deploy.md](docs/deploy.md) (Chinese).

## Testing

```bash
# Start a test MySQL 8
docker run -d --name lcw-mysql-test -e MYSQL_ROOT_PASSWORD=root \
  -p 127.0.0.1:3307:3306 mysql:8.4

# Unit + HTTP integration tests (each test creates and cleans its own database)
LCW_TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3307)/' go test ./...

# End-to-end (the service must have development login enabled)
python3 scripts/e2e.py --base http://127.0.0.1:8080 --group 123456789
```

## Project structure

```text
cmd/lol-civil-war/       entry point
internal/config/         environment variables and .env configuration
internal/domain/         business rules: rank/strength, formats, auto-balancing, ROFL parsing, champion library
internal/store/          MySQL persistence (schema.sql and aggregate queries)
internal/service/        application services: permissions, transactions, DTO assembly
internal/web/            HTTP routes, sessions, CSRF, rate limiting, static assets
internal/webui/          embedded frontend output (dist is generated by the build)
web/                     Vue 3 frontend source
deploy/                  Dockerfile, Compose, configuration templates
docs/                    architecture, API, and deployment documentation
scripts/                 release / remote deploy / end-to-end checks / icon downloads
```

## Data and privacy

- Business data lives in MySQL; see the deployment docs for backup and restore.
- Uploaded `.rofl` files are **not persisted**; only the SHA-256 hash and parsed results are stored.
- Member IDs (QQ numbers in mainland deployments) are masked by the API when present (the last three digits are hidden); group IDs do not appear in URLs.
- Passwords are stored as bcrypt hashes. The account identifier is the generated `user_no`, not a QQ OpenID.
- The repository contains no real group IDs, servers, domains, or credentials; provide them through environment variables at deployment time.

## Known limitations

- Single-instance deployment by default; sessions are stored in MySQL, but rate limiting is in-process and multiple replicas require a shared limiter.
- `.rofl` recordings start at game load and **do not include ban/pick**; record BP manually.
- Only ROFL2 replays from 14.11+ are supported; 14.9–14.10 replays have no metadata.
- Very large registration pools may hit the auto-balancing search budget (3 million nodes by default); the result is not guaranteed to be globally optimal.
- The repository does not bundle champion avatars (Riot copyright material); DataDragon CDN is used first, then the local `/data/champions` cache, then the first character of the champion name. Download avatars with `make champions`.

## License and notices

This project is licensed under [GPL-3.0](LICENSE).

This tool is not affiliated with Riot Games. League of Legends, champion names, and related assets are trademarks/property of Riot Games, Inc. and are not covered by the GPL. See [NOTICE](NOTICE).

Champion avatar data source: [DataDragon CDN](https://ddragon.leagueoflegends.com/cdn/). Avatar files are not committed to the repository; deployers may cache them locally as needed.
