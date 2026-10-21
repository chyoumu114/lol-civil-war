// Package webui 内嵌构建后的前端静态资源。
//
// 仓库只提交 dist/README.txt 占位文件，前端产物由 `make web` 或 Docker 构建生成；
// 未构建时服务端会返回一段提示页，而不是启动失败。
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// Dist 返回前端产物目录。
func Dist() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return embedded
	}
	return sub
}
