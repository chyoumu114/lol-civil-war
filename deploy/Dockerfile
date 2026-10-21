# syntax=docker/dockerfile:1

# ---------- 前端构建 ----------
FROM node:22.23.4-alpine AS web
# 固定构建版本，避免基础镜像漂移导致线上产物不一致
RUN test "$(node -v)" = "v22.23.4"
WORKDIR /src/web
ARG NPM_REGISTRY=https://registry.npmjs.org
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund --registry=${NPM_REGISTRY}
COPY web/ ./
RUN npm run build

# ---------- Go 构建 ----------
FROM golang:1.27.3-alpine AS build
RUN GOTOOLCHAIN=local go version | grep -q 'go1.27.3'
WORKDIR /src
ARG VERSION=dev
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=${GOPROXY}
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 把前端产物放进 embed 目录（覆盖仓库里的占位文件）
COPY --from=web /src/web/dist ./internal/webui/dist
RUN go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/lol-civil-war ./cmd/lol-civil-war

# ---------- 运行镜像 ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && \
    addgroup -g 10001 -S app && \
    adduser -u 10001 -S app -G app && \
    mkdir -p /data && chown -R app:app /data
WORKDIR /app
COPY --from=build /out/lol-civil-war /app/lol-civil-war
USER 10001:10001
ENV LCW_ADDR=:8080 \
    LCW_DATA_DIR=/data \
    LCW_TIMEZONE=Asia/Shanghai
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O- http://127.0.0.1:8080/api/health >/dev/null || exit 1
ENTRYPOINT ["/app/lol-civil-war"]
