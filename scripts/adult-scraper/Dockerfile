# syntax=docker/dockerfile:1.6
# =============================================
# 番号刮削 Python 微服务 · 独立部署版
# =============================================
# 默认已集成进主镜像 cropflre/nowen-video（Go 后端会自动拉起子进程），
# 只有当你希望把刮削服务独立部署到不同主机 / 不同容器时，才需要单独构建此镜像。
#
# 多架构构建：
#   docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
#       -t yourname/nowen-adult-scraper:latest --push .
#
# 采用 Alpine 预编译包（py3-lxml/py3-flask 等），避免 ARM 下 pip 编译 lxml 耗时数十分钟。
# =============================================

FROM python:3.12-alpine

WORKDIR /app

# 安装 Alpine 社区源提供的预编译 Python 包（amd64/arm64/armv7 通用）
# 这样完全跳过 pip 编译，镜像体积小且构建飞快
RUN apk add --no-cache \
        py3-flask \
        py3-requests \
        py3-beautifulsoup4 \
        py3-lxml \
        ca-certificates \
        tzdata \
        wget \
    && rm -rf /var/cache/apk/*

# 保留 requirements.txt 用于版本追踪（实际不执行 pip install，避免二次编译）
COPY requirements.txt .
COPY app.py .

# 环境变量（可在 docker-compose 中覆盖）
ENV SCRAPER_HOST=0.0.0.0
ENV SCRAPER_PORT=5000
ENV DEBUG=false
ENV SCRAPER_API_KEY=""
ENV JAVBUS_URL="https://www.javbus.com"
ENV JAVDB_URL="https://javdb.com"
ENV PYTHONUNBUFFERED=1
ENV PYTHONIOENCODING=UTF-8
ENV TZ=Asia/Shanghai

EXPOSE 5000

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:5000/health || exit 1

CMD ["python3", "app.py"]
