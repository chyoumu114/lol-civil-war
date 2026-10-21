#!/usr/bin/env bash
# 本地开发启动：
#   1. 没有显式配置外部数据库时，自动启动一个 MySQL 开发容器（默认 127.0.0.1:3307）
#   2. 开启开发登录，后端监听 127.0.0.1:8080
#
# 用法：
#   make dev
#   LCW_DB_HOST=127.0.0.1 LCW_DB_PORT=3306 LCW_DB_USER=lcw LCW_DB_PASSWORD=*** make dev
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

# WSL 的非交互 shell 往往没有把 Go 加进 PATH
if ! command -v go >/dev/null 2>&1 && [ -x /usr/local/go/bin/go ]; then
  export PATH=/usr/local/go/bin:$PATH
fi

if [ -z "${LCW_DB_DSN:-}" ] && [ -z "${LCW_DB_HOST:-}" ]; then
  DB_CONTAINER=${LCW_DEV_DB_CONTAINER:-lol-civil-war-mysql-dev}
  DB_PORT=${LCW_DEV_DB_PORT:-3307}
  DB_PASSWORD=${LCW_DEV_DB_PASSWORD:-lcw-dev}
  DB_NAME=${LCW_DEV_DB_NAME:-lol_civil_war_dev}
  MYSQL_IMAGE=${MYSQL_IMAGE:-mysql:8.4}

  if ! command -v docker >/dev/null 2>&1; then
    echo "未找到 docker。也可以指定外部 MySQL：LCW_DB_HOST=... make dev" >&2
    exit 1
  fi

  if ! docker inspect "$DB_CONTAINER" >/dev/null 2>&1; then
    echo "创建开发 MySQL 容器 $DB_CONTAINER（端口 $DB_PORT）..."
    docker run -d --name "$DB_CONTAINER" \
      -e "MYSQL_ROOT_PASSWORD=$DB_PASSWORD" \
      -e "MYSQL_DATABASE=$DB_NAME" \
      -p "127.0.0.1:$DB_PORT:3306" \
      "$MYSQL_IMAGE" >/dev/null
  else
    docker start "$DB_CONTAINER" >/dev/null 2>&1 || true
  fi

  echo -n "等待 MySQL 就绪"
  ready=0
  for _ in $(seq 1 90); do
    if docker exec "$DB_CONTAINER" mysqladmin ping -uroot -p"$DB_PASSWORD" --silent >/dev/null 2>&1; then
      ready=1
      break
    fi
    echo -n "."
    sleep 1
  done
  echo
  if [ "$ready" -ne 1 ]; then
    echo "MySQL 启动失败，容器日志：" >&2
    docker logs --tail=50 "$DB_CONTAINER" >&2
    exit 1
  fi

  # 容器已存在但数据库被删时，补建一次
  docker exec "$DB_CONTAINER" mysql -uroot -p"$DB_PASSWORD" \
    -e "CREATE DATABASE IF NOT EXISTS \`$DB_NAME\` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;" \
    >/dev/null 2>&1 || true

  export LCW_DB_HOST=127.0.0.1
  export LCW_DB_PORT="$DB_PORT"
  export LCW_DB_USER=root
  export LCW_DB_PASSWORD="$DB_PASSWORD"
  export LCW_DB_NAME="$DB_NAME"
fi

export LCW_ADDR=${LCW_ADDR:-127.0.0.1:8080}
export LCW_DATA_DIR=${LCW_DATA_DIR:-./data}
export LCW_ALLOWED_GROUPS=${LCW_ALLOWED_GROUPS:-123456789}
export LCW_DEV_LOGIN=${LCW_DEV_LOGIN:-true}
export LCW_BASE_URL=${LCW_BASE_URL:-http://127.0.0.1:8080}

cat <<EOF
后端:   http://127.0.0.1:8080
数据库: $LCW_DB_USER@$LCW_DB_HOST:$LCW_DB_PORT/$LCW_DB_NAME
登录:   http://127.0.0.1:8080/auth/dev?qq=10001&nickname=测试玩家&redirect=/
前端:   cd web && npm run dev   （Vite 会把 /api、/auth 代理到 8080）
EOF

exec go run ./cmd/lol-civil-war
