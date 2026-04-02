# =============================================
# 阶段1: 构建前端
# =============================================
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# =============================================
# 阶段2: 构建后端
# =============================================
FROM golang:1.22-alpine AS backend
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=1 go build -o nowen-video ./cmd/server

# =============================================
# 阶段3: 运行镜像
# =============================================
FROM alpine:3.19

# 安装FFmpeg（含硬件加速支持）和必要运行时
RUN apk add --no-cache \
    ffmpeg \
    tzdata \
    ca-certificates \
    # Intel VAAPI/QSV 支持
    intel-media-driver \
    libva-intel-driver \
    mesa-va-gallium \
    && rm -rf /var/cache/apk/*

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

# 创建 entrypoint 脚本：以 root 启动修复权限，然后切换到 nowen 用户运行
RUN printf '#!/bin/sh\nchown -R nowen:nowen /data /cache 2>/dev/null || true\nexec su-exec nowen nowen-video\n' > /entrypoint.sh \
    && chmod +x /entrypoint.sh

# 安装 su-exec（轻量级的用户切换工具）
RUN apk add --no-cache su-exec

CMD ["/entrypoint.sh"]
