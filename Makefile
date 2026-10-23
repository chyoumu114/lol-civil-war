# 常用开发命令。Windows 下可用 Git Bash / WSL 执行。
SHELL := /bin/sh
VERSION ?= dev
CHAMPION_VERSION ?= 16.20.1
BINARY := lol-civil-war
GO ?= $(shell command -v go 2>/dev/null || echo /usr/local/go/bin/go)
NPM ?= npm

.PHONY: help web build dev run test vet fmt clean champions

help:
	@echo "make web     构建前端并同步到 internal/webui/dist"
	@echo "make build   构建单二进制（含内嵌前端）"
	@echo "make dev     启动开发 MySQL + 后端（8080，开发登录）"
	@echo "make run     本地运行（需自行提供 LCW_DB_* 环境变量）"
	@echo "make test    运行全部 Go 测试"
	@echo "make vet     静态检查"
	@echo "make fmt     格式化 Go 代码"
	@echo "make champions 更新英雄库+头像（CHAMPION_VERSION=$(CHAMPION_VERSION)）"

web:
	cd web && $(NPM) ci --no-audit --no-fund && $(NPM) run build
	rm -rf internal/webui/dist/assets internal/webui/dist/index.html internal/webui/dist/favicon.svg
	cp -R web/dist/. internal/webui/dist/

build: web
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/$(BINARY) ./cmd/lol-civil-war

run:
	$(GO) run ./cmd/lol-civil-war

dev:
	./scripts/dev.sh

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './web/*')

champions:
	python3 scripts/update-champions.py --version $(CHAMPION_VERSION)

clean:
	rm -rf bin internal/webui/dist/assets internal/webui/dist/index.html
