前端构建产物目录

仓库不提交 dist 内容（体积大且每次构建都变），只保留本占位文件，
让 `go:embed all:dist` 在没有先构建前端时也能编译通过。

构建方式：
  make web          # 或 cd web && npm ci && npm run build && cp -r dist/* ../internal/webui/dist/

Docker 镜像会在构建阶段自动完成前端构建，无需手工操作。
