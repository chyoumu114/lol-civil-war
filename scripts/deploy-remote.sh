#!/usr/bin/env bash
# 部署到远端服务器：上传 compose 与配置、拉取镜像、重启、健康检查。
#
# 需要两个本地文件（均已 gitignore）：
#   deploy/.env.deploy  服务器与 ACR 信息
#   deploy/.env.prod    远端运行配置（LCW_*）
#
# deploy/.env.deploy 字段：
#   DEPLOY_HOST=203.0.113.10
#   DEPLOY_USER=root
#   DEPLOY_PORT=22
#   DEPLOY_SSH_KEY=~/.ssh/deploy_key
#   DEPLOY_DIR=/opt/lol-civil-war
#   ACR_REGISTRY=...  ACR_USERNAME=...  ACR_PASSWORD=...
#   LCW_IMAGE=<registry>/<namespace>/lol-civil-war:<tag>
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DEPLOY_ENV=${DEPLOY_ENV_FILE:-$ROOT/deploy/.env.deploy}
PROD_ENV=${PROD_ENV_FILE:-$ROOT/deploy/.env.prod}
[ -f "$DEPLOY_ENV" ] || { echo "缺少 $DEPLOY_ENV" >&2; exit 1; }
[ -f "$PROD_ENV" ] || { echo "缺少 $PROD_ENV" >&2; exit 1; }

set -a; . "$DEPLOY_ENV"; set +a
: "${DEPLOY_HOST:?请设置 DEPLOY_HOST}"
DEPLOY_USER=${DEPLOY_USER:-root}
DEPLOY_PORT=${DEPLOY_PORT:-22}
DEPLOY_DIR=${DEPLOY_DIR:-/opt/lol-civil-war}
DEPLOY_SSH_KEY=${DEPLOY_SSH_KEY:-$HOME/.ssh/id_ed25519}
LCW_IMAGE=${LCW_IMAGE:?请设置 LCW_IMAGE（本次要部署的完整镜像地址）}

# 从远端配置里取健康检查端口（LCW_BIND=127.0.0.1:18020）
BIND=$(grep -E '^LCW_BIND=' "$PROD_ENV" | tail -1 | cut -d= -f2- || true)
HEALTH_PORT=${BIND##*:}
HEALTH_PORT=${HEALTH_PORT:-18020}

SSH=(ssh -i "$DEPLOY_SSH_KEY" -p "$DEPLOY_PORT" -o StrictHostKeyChecking=accept-new "$DEPLOY_USER@$DEPLOY_HOST")
SCP=(scp -i "$DEPLOY_SSH_KEY" -P "$DEPLOY_PORT" -o StrictHostKeyChecking=accept-new)

echo "== 准备远端目录 $DEPLOY_HOST:$DEPLOY_DIR =="
"${SSH[@]}" "mkdir -p '$DEPLOY_DIR'"

echo "== 上传 compose 与运行配置 =="
"${SCP[@]}" "$ROOT/deploy/compose.yml" "$DEPLOY_USER@$DEPLOY_HOST:$DEPLOY_DIR/compose.yml"
"${SCP[@]}" "$PROD_ENV" "$DEPLOY_USER@$DEPLOY_HOST:$DEPLOY_DIR/.env"
"${SSH[@]}" "chmod 600 '$DEPLOY_DIR/.env'"

echo "== 远端登录 ACR 并拉取镜像 =="
if [ -n "${ACR_REGISTRY:-}" ] && [ -n "${ACR_USERNAME:-}" ] && [ -n "${ACR_PASSWORD:-}" ]; then
  printf '%s' "$ACR_PASSWORD" | "${SSH[@]}" "docker login '$ACR_REGISTRY' -u '$ACR_USERNAME' --password-stdin"
fi
"${SSH[@]}" "cd '$DEPLOY_DIR' && LCW_IMAGE='$LCW_IMAGE' docker compose -f compose.yml pull && LCW_IMAGE='$LCW_IMAGE' docker compose -f compose.yml up -d"

echo "== 健康检查 =="
"${SSH[@]}" "for i in \$(seq 1 60); do curl -fsS http://127.0.0.1:$HEALTH_PORT/api/health && exit 0; sleep 2; done; echo '健康检查失败'; cd '$DEPLOY_DIR' && docker compose -f compose.yml logs --tail=80 app; exit 1"

echo "完成：$LCW_IMAGE"
