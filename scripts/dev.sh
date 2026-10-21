#!/usr/bin/env bash
# 本地开发启动：开启开发登录，默认监听 127.0.0.1:8080。
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

export LCW_ADDR=${LCW_ADDR:-127.0.0.1:8080}
export LCW_DATA_DIR=${LCW_DATA_DIR:-./data}
export LCW_ALLOWED_GROUPS=${LCW_ALLOWED_GROUPS:-123456789}
export LCW_DEV_LOGIN=${LCW_DEV_LOGIN:-true}
export LCW_BASE_URL=${LCW_BASE_URL:-http://127.0.0.1:8080}

echo "开发登录：http://127.0.0.1:8080/auth/dev?qq=10001&nickname=测试玩家&redirect=/"
exec go run ./cmd/lol-civil-war
