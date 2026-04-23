# syntax=docker/dockerfile:1.6
# =============================================
# 多架构构建：支持 linux/amd64 与 linux/arm64
# 使用方式：
#   docker buildx build --platform linux/amd64,linux/arm64 -t nowen-video:latest .
# 说明：
#   - 前端阶段固定在构建机架构，产物是纯静态文件，与运行架构无关
#   - 后端走 Go 原生交叉编译（纯 Go SQLite，CGO=0）
#   - 运行阶段按架构条件安装硬件加速驱动（Intel 驱动仅 amd64）
# =============================================

# =============================================
# 阶段1: 构建前端（锁定在构建机本地架构，避免 QEMU 跑 npm 极慢）
# =============================================
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# =============================================
# 阶段2: 构建后端（在构建机本地架构运行 Go 工具链，交叉编译到目标架构）
# =============================================
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS backend
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
# 使用纯 Go SQLite (glebarez/sqlite)，可以 CGO_ENABLED=0 直接交叉编译
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o nowen-video ./cmd/server

# =============================================
# 阶段3: 运行镜像（目标架构的 alpine）
# =============================================
FROM alpine:3.19
ARG TARGETARCH

# 基础依赖：所有架构都需要
RUN apk add --no-cache \
    ffmpeg \
    tzdata \
    ca-certificates \
    su-exec \
    coreutils \
    && rm -rf /var/cache/apk/* \
    && ln -sf /bin/nice /usr/bin/nice

# 架构相关的硬件加速驱动
# - amd64: Intel QSV/VAAPI (intel-media-driver + libva-intel-driver)
# - arm64: 通用 mesa VAAPI 驱动（Mali/Panfrost/RPi 等），Intel 专有驱动不可用
RUN set -eux; \
    if [ "${TARGETARCH}" = "amd64" ]; then \
        apk add --no-cache \
            intel-media-driver \
            libva-intel-driver \
            mesa-va-gallium \
            libva-utils; \
    else \
        apk add --no-cache \
            mesa-va-gallium \
            libva-utils; \
    fi; \
    rm -rf /var/cache/apk/*

# GPU 检测脚本
RUN printf '#!/bin/sh\n\
if [ -c /dev/dri/renderD128 ]; then\n\
  echo "GPU device available: $(vainfo 2>/dev/null | grep -o "driver.*" | head -1)"\n\
  exit 0\n\
else\n\
  echo "No GPU device found, falling back to software transcoding"\n\
  exit 1\n\
fi\n' > /usr/local/bin/check-gpu \
    && chmod +x /usr/local/bin/check-gpu

# 创建非root用户
RUN addgroup -S nowen && adduser -S nowen -G nowen

WORKDIR /app

COPY --from=backend /app/nowen-video /usr/local/bin/nowen-video
# 复制前端构建产物
COPY --from=frontend /app/web/dist /app/web/dist

# 创建数据目录并设置权限（确保挂载卷时nowen用户也能写入）
RUN mkdir -p /data /cache /media && chown -R nowen:nowen /data /cache /media

# 默认环境变量
ENV NOWEN_APP_PORT=8080
ENV NOWEN_APP_DATA_DIR=/data
ENV NOWEN_APP_WEB_DIR=/app/web/dist
ENV NOWEN_APP_HW_ACCEL=auto
ENV NOWEN_APP_TRANSCODE_PRESET=veryfast
ENV NOWEN_APP_MAX_TRANSCODE_JOBS=2
ENV NOWEN_DATABASE_DB_PATH=/data/nowen.db
ENV NOWEN_CACHE_CACHE_DIR=/cache
ENV NOWEN_LOGGING_LEVEL=info
ENV TZ=Asia/Shanghai

EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/api/auth/login || exit 1

# 创建 entrypoint 脚本：支持 PUID/PGID 自定义用户，修复权限后切换用户运行
RUN printf '#!/bin/sh\n\
# 支持通过 PUID/PGID 环境变量自定义运行用户（兼容NAS场景）\n\
PUID=${PUID:-$(id -u nowen)}\n\
PGID=${PGID:-$(id -g nowen)}\n\
\n\
# 如果指定了自定义 UID/GID，则修改 nowen 用户\n\
if [ "$PUID" != "$(id -u nowen)" ] || [ "$PGID" != "$(id -g nowen)" ]; then\n\
  deluser nowen 2>/dev/null || true\n\
  delgroup nowen 2>/dev/null || true\n\
  addgroup -g "$PGID" -S nowen\n\
  adduser -u "$PUID" -G nowen -S nowen\n\
fi\n\
\n\
# 修复数据目录权限\n\
chown -R nowen:nowen /data /cache 2>/dev/null || true\n\
# 确保 /media 目录可读（不递归 chown，避免大量文件耗时）\n\
chown nowen:nowen /media 2>/dev/null || true\n\
\n\
exec su-exec nowen nowen-video\n' > /entrypoint.sh \
    && chmod +x /entrypoint.sh

CMD ["/entrypoint.sh"]
