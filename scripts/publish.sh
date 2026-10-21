#!/usr/bin/env bash
# 构建并推送镜像到阿里云 ACR（本地发布用）。
#
# 用法：
#   scripts/publish.sh            # 用时间戳作为 tag
#   scripts/publish.sh v1.0.0     # 指定 tag
#
# 需要 deploy/.env（已 gitignore），字段：
#   ACR_REGISTRY=registry.cn-hangzhou.aliyuncs.com
#   ACR_NAMESPACE=your-namespace
#   ACR_REPO=lol-civil-war
#   ACR_USERNAME=...
#   ACR_PASSWORD=...
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
ENV_FILE=${PUBLISH_ENV_FILE:-$ROOT/deploy/.env}
[ -f "$ENV_FILE" ] || { echo "缺少 $ENV_FILE（可从 deploy/.env.example 复制）" >&2; exit 1; }
# shellcheck disable=SC1090
set -a; . "$ENV_FILE"; set +a

: "${ACR_REGISTRY:?请设置 ACR_REGISTRY}"
: "${ACR_NAMESPACE:?请设置 ACR_NAMESPACE}"
: "${ACR_USERNAME:?请设置 ACR_USERNAME}"
: "${ACR_PASSWORD:?请设置 ACR_PASSWORD}"
ACR_REPO=${ACR_REPO:-lol-civil-war}
TAG=${1:-$(date +%Y%m%d-%H%M%S)}
IMAGE="$ACR_REGISTRY/$ACR_NAMESPACE/$ACR_REPO:$TAG"

echo "== 构建 $IMAGE =="
docker build \
  --provenance=false \
  --sbom=false \
  --build-arg "VERSION=$TAG" \
  --build-arg "GOPROXY=${DOCKER_GOPROXY:-https://proxy.golang.org,direct}" \
  --build-arg "NPM_REGISTRY=${DOCKER_NPM_REGISTRY:-https://registry.npmjs.org}" \
  -f "$ROOT/deploy/Dockerfile" \
  -t "$IMAGE" \
  "$ROOT"

echo "== 登录 ACR =="
printf '%s' "$ACR_PASSWORD" | docker login "$ACR_REGISTRY" --username "$ACR_USERNAME" --password-stdin

echo "== 推送 =="
docker push "$IMAGE"
docker tag "$IMAGE" "$ACR_REGISTRY/$ACR_NAMESPACE/$ACR_REPO:latest" || true
docker push "$ACR_REGISTRY/$ACR_NAMESPACE/$ACR_REPO:latest" || true

echo "完成：$IMAGE"
