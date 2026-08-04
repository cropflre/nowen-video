#!/usr/bin/env bash
# 一键启动 nowen-video 桌面端开发环境
#
# 用法:
#   bash desktop/scripts/dev.sh [--rebuild-sidecar]

set -euo pipefail

REBUILD_SIDECAR=false
for arg in "$@"; do
    case $arg in
        --rebuild-sidecar|-r) REBUILD_SIDECAR=true ;;
    esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DESKTOP_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PROJECT_ROOT="$(cd "$DESKTOP_ROOT/.." && pwd)"
DEV_WEB_PORT=28889

normalize_version() {
    local raw="${1:-}"
    raw="${raw#refs/tags/}"
    raw="${raw#v}"
    if [[ "$raw" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
        echo "$raw"
    fi
}

resolve_app_version() {
    local candidate normalized tag
    for candidate in "${NOWEN_VERSION:-}" "${APP_VERSION:-}" "${GITHUB_REF_NAME:-}"; do
        normalized="$(normalize_version "$candidate")"
        if [[ -n "$normalized" ]]; then echo "$normalized"; return; fi
    done
    if tag="$(git -C "$PROJECT_ROOT" describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null)"; then
        normalized="$(normalize_version "$tag")"
        if [[ -n "$normalized" ]]; then echo "$normalized"; return; fi
    fi
    echo "0.1.0"
}

APP_VERSION_RESOLVED="$(resolve_app_version)"
export NOWEN_VERSION="$APP_VERSION_RESOLVED"
export APP_VERSION="$APP_VERSION_RESOLVED"
export VITE_APP_VERSION="$APP_VERSION_RESOLVED"
export WEB_PORT="$DEV_WEB_PORT"

echo "============================================"
echo " nowen-video Desktop 开发环境启动"
echo " Version: $APP_VERSION_RESOLVED"
echo " Vite port: $DEV_WEB_PORT"
echo "============================================"

BIN_DIR="$DESKTOP_ROOT/bin"
EXT=""
if [[ "$(go env GOOS)" == "windows" ]]; then EXT=".exe"; fi
SIDECAR="$BIN_DIR/nowen-video$EXT"

# Step 1: 构建 sidecar
if [[ "$REBUILD_SIDECAR" == "true" || ! -f "$SIDECAR" ]]; then
    echo ""
    echo "[1/3] 构建 Go sidecar..."
    bash "$SCRIPT_DIR/build-sidecar.sh"
else
    echo ""
    echo "[1/3] ✅ sidecar 已存在，跳过构建"
fi

# Step 2: 启动前端
echo ""
echo "[2/3] 启动前端 Vite dev server..."
WEB_ROOT="$PROJECT_ROOT/web"
if [[ ! -d "$WEB_ROOT/node_modules" ]]; then
    echo "  首次运行，安装依赖..."
    (cd "$WEB_ROOT" && npm install)
fi

cleanup() {
    echo ""
    echo "清理 Vite 后台进程..."
    if [[ -n "${VITE_PID:-}" ]]; then
        kill "$VITE_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

(cd "$WEB_ROOT" && npm run dev -- --port "$DEV_WEB_PORT" --strictPort) &
VITE_PID=$!
echo "  Vite 已启动 (PID: $VITE_PID)"

# 等待 vite 就绪
for i in $(seq 1 30); do
    if curl -sf "http://localhost:$DEV_WEB_PORT" >/dev/null 2>&1; then
        echo "  ✅ Vite ready"
        break
    fi
    sleep 1
done

# Step 3: Tauri dev
echo ""
echo "[3/3] 启动 Tauri 桌面壳..."
cd "$DESKTOP_ROOT/src-tauri"
cargo tauri dev
