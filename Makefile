.PHONY: all build run dev clean docker

# 默认目标
all: build

# 构建后端
build:
	cd web && npm run build
	CGO_ENABLED=1 go build -o bin/nowen-video ./cmd/server

# 仅构建后端
build-server:
	CGO_ENABLED=1 go build -o bin/nowen-video ./cmd/server

# 仅构建前端
build-web:
	cd web && npm run build

# 开发模式运行后端
dev:
	NOWEN_DEBUG=true go run ./cmd/server

# 开发模式运行前端
dev-web:
	cd web && npm run dev

# 运行（生产模式）
run: build
	./bin/nowen-video

# Docker构建
docker:
	docker-compose up --build -d

# Docker停止
docker-stop:
	docker-compose down

# 清理
clean:
	rm -rf bin/
	rm -rf cache/transcode/
	cd web && rm -rf dist/ node_modules/

# 安装前端依赖
install-web:
	cd web && npm install

# Go依赖整理
tidy:
	go mod tidy
