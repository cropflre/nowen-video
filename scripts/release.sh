#!/usr/bin/env bash
# =============================================================================
# Nowen Video 傻瓜式发版入口
#
# 零参数运行：进入交互向导，默认一路回车即可完成 Server + Android 同步发版。
# 带参数运行：直接透传给 release-advanced.sh，保留完整高级能力。
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ADVANCED_SCRIPT="$SCRIPT_DIR/release-advanced.sh"
DEFAULT_BRANCH="main"

[ -f "$ADVANCED_SCRIPT" ] || { echo "[release] 缺少 $ADVANCED_SCRIPT" >&2; exit 1; }

# 兼容原来的高级命令。只要传了参数，就不进入向导。
if [ "$#" -gt 0 ]; then
    exec bash "$ADVANCED_SCRIPT" "$@"
fi

if [ -t 1 ] && command -v tput >/dev/null 2>&1 && [ "$(tput colors 2>/dev/null || echo 0)" -ge 8 ]; then
    C_RED="$(tput setaf 1)"; C_GREEN="$(tput setaf 2)"; C_YELLOW="$(tput setaf 3)"
    C_BLUE="$(tput setaf 4)"; C_CYAN="$(tput setaf 6)"; C_BOLD="$(tput bold)"; C_RESET="$(tput sgr0)"
else
    C_RED=""; C_GREEN=""; C_YELLOW=""; C_BLUE=""; C_CYAN=""; C_BOLD=""; C_RESET=""
fi

info() { echo "${C_BLUE}[*]${C_RESET} $*"; }
ok()   { echo "${C_GREEN}[✓]${C_RESET} $*"; }
warn() { echo "${C_YELLOW}[!]${C_RESET} $*" >&2; }
die()  { echo "${C_RED}[✗]${C_RESET} $*" >&2; exit 1; }

trap 'echo; die "已取消发版"' INT

clear_if_terminal() {
    [ -t 1 ] && command -v clear >/dev/null 2>&1 && clear || true
}

validate_version() {
    printf '%s' "$1" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'
}

latest_stable_version() {
    git tag --list 'v*' --sort=-v:refname 2>/dev/null \
        | sed -n -E 's/^v([0-9]+\.[0-9]+\.[0-9]+)$/\1/p' \
        | head -1
}

suggest_next_patch() {
    local latest="$1" major minor patch
    if [ -z "$latest" ]; then
        echo "0.1.0"
        return
    fi
    IFS=. read -r major minor patch <<<"$latest"
    echo "${major}.${minor}.$((10#$patch + 1))"
}

cd "$REPO_ROOT"
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "当前目录不是 Git 仓库"

clear_if_terminal
cat <<'BANNER'
╔══════════════════════════════════════════════════════╗
║              Nowen Video 一键发版向导              ║
╠══════════════════════════════════════════════════════╣
║  默认一路回车：Server + Android 同版本正式发布     ║
║  自动完成：CI → Docker → APK/AAB → Tag → Release   ║
╚══════════════════════════════════════════════════════╝
BANNER

echo
info "正在获取 main 和最新版本信息..."
CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$CURRENT_BRANCH" != "$DEFAULT_BRANCH" ]; then
    warn "当前分支是 ${CURRENT_BRANCH}，正式发版需要 main。"
    read -r -p "是否自动切换到 main？[Y/n] " switch_answer
    case "${switch_answer:-y}" in
        [yY]|[yY][eE][sS])
            [ -z "$(git status --porcelain)" ] || die "工作区有未提交改动，无法自动切换 main"
            git checkout "$DEFAULT_BRANCH"
            ;;
        *) die "已取消；请切到 main 后再运行" ;;
    esac
fi

if ! git fetch origin "$DEFAULT_BRANCH" --tags --prune; then
    die "获取 GitHub main/tags 失败，请检查网络或 GitHub 登录"
fi

LATEST="$(latest_stable_version)"
SUGGEST="$(suggest_next_patch "$LATEST")"

echo
printf '%s\n' "${C_BOLD}① 版本号${C_RESET}"
echo "   当前最新稳定版：${C_GREEN}${LATEST:-暂无}${C_RESET}"
read -r -p "   本次版本 [${SUGGEST}]: " VERSION
VERSION="${VERSION:-$SUGGEST}"
VERSION="${VERSION#v}"
validate_version "$VERSION" || die "版本格式不正确：$VERSION，例如 1.2.9 或 1.3.0-rc.1"

IS_PRERELEASE=0
[[ "$VERSION" == *-* ]] && IS_PRERELEASE=1

echo
printf '%s\n' "${C_BOLD}② 发布内容${C_RESET}"
echo "   ${C_GREEN}1) Server + Android${C_RESET}   [默认，推荐]"
echo "      Docker amd64/arm64 + Android APK/AAB + GitHub Release"
echo "   2) Server + Android + Desktop"
echo "      在默认方案基础上再发布 Windows 桌面端"
echo "   3) 仅 Server"
echo "      只发布 Docker，不发 Android/Desktop，也不创建产品 Release"
read -r -p "   请选择 [1/2/3，默认 1]: " TARGET_CHOICE
TARGET_CHOICE="${TARGET_CHOICE:-1}"

ADVANCED_ARGS=(-v "$VERSION" -y)
TARGET_LABEL=""
case "$TARGET_CHOICE" in
    1)
        TARGET_LABEL="Server + Android"
        ADVANCED_ARGS+=(--no-desktop)
        ;;
    2)
        TARGET_LABEL="Server + Android + Desktop"
        ;;
    3)
        TARGET_LABEL="仅 Server"
        ADVANCED_ARGS+=(--server-only)
        ;;
    *) die "无效选择：$TARGET_CHOICE" ;;
esac

CHANNEL_LABEL="正式版"
LATEST_LABEL="更新"
if [ "$IS_PRERELEASE" = "1" ]; then
    CHANNEL_LABEL="预发布版"
    LATEST_LABEL="不更新"
fi

echo
printf '%s\n' "${C_BOLD}③ 最终确认${C_RESET}"
echo "   版本       : ${C_GREEN}v${VERSION}${C_RESET} (${CHANNEL_LABEL})"
echo "   发布内容   : ${TARGET_LABEL}"
echo "   Docker     : linux/amd64 + linux/arm64"
echo "   Docker tag : cropflre/nowen-video:v${VERSION}"
echo "   latest     : ${LATEST_LABEL}"
if [ "$TARGET_CHOICE" != "3" ]; then
    echo "   Git tag    : v${VERSION}"
    echo "   Release    : GitHub Draft Release"
fi
if [ "$TARGET_CHOICE" = "1" ] || [ "$TARGET_CHOICE" = "2" ]; then
    echo "   Android    : nowen-video-android-${VERSION}.apk"
    echo "                nowen-video-android-${VERSION}.aab"
fi
if [ "$TARGET_CHOICE" = "2" ]; then
    echo "   Desktop    : Windows EXE/MSI"
fi

echo
read -r -p "确认开始发版？[Y/n] " CONFIRM
case "${CONFIRM:-y}" in
    [yY]|[yY][eE][sS]) ;;
    *) die "已取消发版" ;;
esac

echo
echo "${C_CYAN}======================================================${C_RESET}"
echo "${C_BOLD}开始发布 Nowen Video v${VERSION}${C_RESET}"
echo "${C_CYAN}======================================================${C_RESET}"
echo

# 高级脚本会重新执行所有安全校验：main 同步、版本防倒退、CI、Android
# production signing、Docker manifest、Git tag、Release 资产核验等。
exec bash "$ADVANCED_SCRIPT" "${ADVANCED_ARGS[@]}"
