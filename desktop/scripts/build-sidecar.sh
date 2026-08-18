#!/usr/bin/env bash
# 构建 Desktop 2.0 Go Media Core sidecar。
# Tauri externalBin 产物：src-tauri/bin/nowen-video-server-<target-triple>。

set -euo pipefail

PRODUCTION=false
for arg in "$@"; do
    case "$arg" in
        --production|-p) PRODUCTION=true ;;
    esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DESKTOP_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PROJECT_ROOT="$(cd "$DESKTOP_ROOT/.." && pwd)"
TAURI_ROOT="$DESKTOP_ROOT/src-tauri"
BIN_DIR="$TAURI_ROOT/bin"
mkdir -p "$BIN_DIR"

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

GO_OS="$(go env GOOS)"
GO_ARCH="$(go env GOARCH)"
case "${GO_OS}/${GO_ARCH}" in
    linux/amd64)   TRIPLE="x86_64-unknown-linux-gnu" ;;
    linux/arm64)   TRIPLE="aarch64-unknown-linux-gnu" ;;
    darwin/amd64)  TRIPLE="x86_64-apple-darwin" ;;
    darwin/arm64)  TRIPLE="aarch64-apple-darwin" ;;
    windows/amd64) TRIPLE="x86_64-pc-windows-msvc" ;;
    windows/arm64) TRIPLE="aarch64-pc-windows-msvc" ;;
    *) echo "不支持的平台: ${GO_OS}/${GO_ARCH}" >&2; exit 1 ;;
esac

EXT=""
[[ "$GO_OS" == "windows" ]] && EXT=".exe"
OUT_PATH="$BIN_DIR/nowen-video-server-${TRIPLE}${EXT}"
DEV_COPY="$BIN_DIR/nowen-video-server${EXT}"
VERSION_PACKAGE="github.com/nowen-video/nowen-video/internal/version.Version"
BUILD_ARGS=("-ldflags" "-s -w -X ${VERSION_PACKAGE}=${APP_VERSION_RESOLVED}" "-o" "$OUT_PATH")
[[ "$PRODUCTION" == "true" ]] && BUILD_ARGS+=("-trimpath")
BUILD_ARGS+=("./cmd/server-lite")

echo "====================================="
echo " Nowen Video Desktop 2.0 Media Core"
echo "====================================="
echo "版本: $APP_VERSION_RESOLVED"
echo "目标: $TRIPLE"
echo "产物: $OUT_PATH"

cd "$PROJECT_ROOT"
go build "${BUILD_ARGS[@]}"
cp "$OUT_PATH" "$DEV_COPY"

CONFIG_EXAMPLE="$PROJECT_ROOT/config.example.yaml"
CONFIG_TARGET="$BIN_DIR/config.yaml"
if [[ -f "$CONFIG_EXAMPLE" && ! -f "$CONFIG_TARGET" ]]; then
    cp "$CONFIG_EXAMPLE" "$CONFIG_TARGET"
fi

echo "✅ Desktop Media Core 构建完成: $(du -h "$OUT_PATH" | cut -f1)"
