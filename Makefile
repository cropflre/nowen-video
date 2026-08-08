.PHONY: all build build-lite build-full build-server build-server-full build-web run run-full dev dev-full dev-server dev-server-full dev-web clean docker docker-full docker-stop tidy

VERSION ?= $(shell git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null | sed 's/^v//' || echo 0.1.0)
GO_VERSION_PKG := github.com/nowen-video/nowen-video/internal/version.Version
GO_LDFLAGS := -s -w -X $(GO_VERSION_PKG)=$(VERSION)
DEV_SERVER_PORT ?= 28888
DEV_WEB_PORT ?= 28889
DEV_API_PROXY ?= http://localhost:$(DEV_SERVER_PORT)

# 默认构建 NAS 轻量版
all: build

build: build-lite

build-lite: build-web build-server

# 保留完整兼容服务，供 Emby、预处理、番号、音乐/图片等高级能力使用
build-full: build-web build-server-full

build-server:
	CGO_ENABLED=1 NOWEN_VERSION=$(VERSION) go build -ldflags "$(GO_LDFLAGS)" -o bin/nowen-video ./cmd/server-lite

build-server-full:
	CGO_ENABLED=1 NOWEN_VERSION=$(VERSION) go build -ldflags "$(GO_LDFLAGS)" -o bin/nowen-video-full ./cmd/server

build-web:
	cd web && VITE_APP_VERSION=$(VERSION) npm run build

# 默认开发模式运行轻量服务。
# Go 服务直接读取 web/dist，因此每次启动前必须重建当前分支前端，
# 避免继续提供其他分支或旧版本遗留的首页、菜单和页面。
dev: build-web
	NOWEN_APP_PORT=$(DEV_SERVER_PORT) NOWEN_DEBUG=true NOWEN_VERSION=$(VERSION) go run -ldflags "$(GO_LDFLAGS)" ./cmd/server-lite

dev-full: build-web
	NOWEN_APP_PORT=$(DEV_SERVER_PORT) NOWEN_DEBUG=true NOWEN_VERSION=$(VERSION) go run -ldflags "$(GO_LDFLAGS)" ./cmd/server

# 仅供明确需要复用现有 dist 的后端调试场景使用。
# 常规开发请使用 make dev / make dev-full。
dev-server:
	NOWEN_APP_PORT=$(DEV_SERVER_PORT) NOWEN_DEBUG=true NOWEN_VERSION=$(VERSION) go run -ldflags "$(GO_LDFLAGS)" ./cmd/server-lite

dev-server-full:
	NOWEN_APP_PORT=$(DEV_SERVER_PORT) NOWEN_DEBUG=true NOWEN_VERSION=$(VERSION) go run -ldflags "$(GO_LDFLAGS)" ./cmd/server

dev-web:
	cd web && WEB_PORT=$(DEV_WEB_PORT) VITE_API_PROXY_TARGET=$(DEV_API_PROXY) VITE_APP_VERSION=$(VERSION) npm run dev

run: build-lite
	./bin/nowen-video

run-full: build-full
	./bin/nowen-video-full

docker:
	docker-compose up --build -d

docker-full:
	docker build -f Dockerfile.full -t nowen-video:full .

docker-stop:
	docker-compose down

clean:
	rm -rf bin/
	rm -rf cache/transcode/
	cd web && rm -rf dist/ node_modules/

install-web:
	cd web && npm install

tidy:
	go mod tidy
