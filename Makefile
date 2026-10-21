# 常用开发命令。Windows 下可用 Git Bash / WSL 执行。
SHELL := /bin/sh
VERSION ?= dev
BINARY := lol-civil-war
GO ?= go
NPM ?= npm

.PHONY: help web build run test vet fmt clean

help:
	@echo "make web     构建前端并同步到 internal/webui/dist"
	@echo "make build   构建单二进制（含内嵌前端）"
	@echo "make run     本地运行（需要 ./data 目录）"
	@echo "make test    运行全部 Go 测试"
	@echo "make vet     静态检查"
	@echo "make fmt     格式化 Go 代码"

web:
	cd web && $(NPM) ci --no-audit --no-fund && $(NPM) run build
	rm -rf internal/webui/dist/assets internal/webui/dist/index.html internal/webui/dist/favicon.svg
	cp -R web/dist/. internal/webui/dist/

build: web
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/$(BINARY) ./cmd/lol-civil-war

run:
	$(GO) run ./cmd/lol-civil-war

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './web/*')

clean:
	rm -rf bin internal/webui/dist/assets internal/webui/dist/index.html
