#!/usr/bin/env bash
# =============================================================================
# nowen-video 发布脚本
#
# 功能：
#   1. 交互式输入版本号（带校验 + 自动建议下一版本）
#   2. git pull 前检查工作区干净度
#   3. 一次 docker build 同时打 :vX.Y.Z + :latest 两个 tag
#   4. 分别 push 到 Docker Hub
#   5. 同步打 git tag 并推送到 GitHub
#
# 使用：
#   ./scripts/release.sh                     # 全交互
#   ./scripts/release.sh -v 1.3.0 -y         # 指定版本 + 跳过确认
#   ./scripts/release.sh -v 1.3.0-rc.1 --no-latest   # 预发布，不动 latest
#   ./scripts/release.sh -v 1.3.0 --no-pull          # 不 git pull
#   ./scripts/release.sh -v 1.3.0 --no-git-tag       # 不打 git tag
#   ./scripts/release.sh -v 1.3.0 --dry-run          # 只打印命令不执行
# =============================================================================

set -euo pipefail

# -------------------- 配置 --------------------
IMAGE_NAME="cropflre/nowen-video"
DEFAULT_BRANCH="main"

# -------------------- 彩色输出 --------------------
if [ -t 1 ] && command -v tput >/dev/null 2>&1 && [ "$(tput colors 2>/dev/null || echo 0)" -ge 8 ]; then
    C_RED="$(tput setaf 1)"
    C_GREEN="$(tput setaf 2)"
    C_YELLOW="$(tput setaf 3)"
    C_BLUE="$(tput setaf 4)"
    C_CYAN="$(tput setaf 6)"
    C_BOLD="$(tput bold)"
    C_RESET="$(tput sgr0)"
else
    C_RED=""; C_GREEN=""; C_YELLOW=""; C_BLUE=""; C_CYAN=""; C_BOLD=""; C_RESET=""
fi

info()  { echo "${C_BLUE}[*]${C_RESET} $*"; }
ok()    { echo "${C_GREEN}[✓]${C_RESET} $*"; }
warn()  { echo "${C_YELLOW}[!]${C_RESET} $*" >&2; }
die()   { echo "${C_RED}[✗]${C_RESET} $*" >&2; exit 1; }
step()  { echo; echo "${C_BOLD}${C_CYAN}==== $* ====${C_RESET}"; }

# -------------------- 参数解析 --------------------
VERSION=""
ASSUME_YES=0
DO_PULL=1
DO_LATEST=1
DO_GIT_TAG=1
DRY_RUN=0

usage() {
    cat <<EOF
用法: $0 [选项]

选项:
  -v, --version VERSION    指定版本号（例: 1.3.0 或 v1.3.0）
  -y, --yes                跳过所有确认
      --no-pull            不执行 git pull
      --no-latest          不打 :latest tag
      --no-git-tag         不打 git tag / 不推送到 GitHub
      --dry-run            仅打印命令，不真实执行
  -h, --help               显示帮助
EOF
    exit 0
}

while [ $# -gt 0 ]; do
    case "$1" in
        -v|--version)   VERSION="${2:-}"; shift 2 ;;
        -y|--yes)       ASSUME_YES=1; shift ;;
        --no-pull)      DO_PULL=0; shift ;;
        --no-latest)    DO_LATEST=0; shift ;;
        --no-git-tag)   DO_GIT_TAG=0; shift ;;
        --dry-run)      DRY_RUN=1; shift ;;
        -h|--help)      usage ;;
        *)              die "未知参数: $1（使用 -h 查看帮助）" ;;
    esac
done

run() {
    if [ "$DRY_RUN" = "1" ]; then
        echo "  ${C_YELLOW}DRY-RUN${C_RESET} $*"
    else
        eval "$@"
    fi
}

# -------------------- 前置检查 --------------------
# 定位到仓库根目录（脚本可能被从任意目录调用）
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

info "工作目录：$REPO_ROOT"

# 必须在 git 仓库里
git rev-parse --is-inside-work-tree >/dev/null 2>&1 \
    || die "当前目录不是 git 仓库"

# docker 可用
command -v docker >/dev/null 2>&1 || die "未安装 docker"
docker info >/dev/null 2>&1 || die "docker daemon 不可用（请启动 docker）"

# Dockerfile 存在
[ -f Dockerfile ] || die "仓库根目录未找到 Dockerfile"

CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
info "当前分支：$CURRENT_BRANCH"
if [ "$CURRENT_BRANCH" != "$DEFAULT_BRANCH" ]; then
    warn "当前不在 $DEFAULT_BRANCH 分支，继续？"
    if [ "$ASSUME_YES" != "1" ]; then
        read -r -p "[y/N] " ans
        case "$ans" in [yY]|[yY][eE][sS]) ;; *) die "已取消" ;; esac
    fi
fi

# 工作区脏检查
if ! git diff-index --quiet HEAD --; then
    warn "工作区有未提交的改动："
    git status --short | head -20
    die "请先提交或 stash 再发布"
fi

# 暂存区检查
if ! git diff --cached --quiet; then
    die "暂存区有未提交的改动，请先 commit"
fi

# -------------------- git pull --------------------
if [ "$DO_PULL" = "1" ]; then
    info "git pull --ff-only origin $CURRENT_BRANCH ..."
    run "git pull --ff-only origin \"$CURRENT_BRANCH\""
    ok "代码已是最新：$(git log -1 --pretty=format:'%h  %s')"
else
    info "跳过 git pull（--no-pull）"
fi

# -------------------- 版本号确定 --------------------
# 找最新的 v*.*.* tag，算下一版本建议值
suggest_next_version() {
    local latest
    latest="$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -1 | sed 's/^v//')" || latest=""
    if [ -z "$latest" ]; then
        echo "0.1.0"
        return
    fi
    # 只取基础 MAJOR.MINOR.PATCH，忽略预发布后缀
    local base="${latest%%-*}"
    local major minor patch
    IFS='.' read -r major minor patch <<EOF
$base
EOF
    patch=$((patch + 1))
    echo "${major}.${minor}.${patch}"
}

validate_version() {
    # 支持 1.2.3 / 1.2.3-rc.1 / 1.2.3-beta.2 等
    echo "$1" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'
}

if [ -z "$VERSION" ]; then
    SUGGEST="$(suggest_next_version)"
    echo
    echo "${C_BOLD}请输入本次发布版本号${C_RESET}（格式：1.2.3 或 v1.2.3，可带 -rc.1 等后缀）"
    echo "   建议：${C_GREEN}${SUGGEST}${C_RESET}（回车使用建议值）"
    read -r -p "> " VERSION
    VERSION="${VERSION:-$SUGGEST}"
fi

# 去除前缀 v
VERSION="${VERSION#v}"
validate_version "$VERSION" || die "版本号格式非法：$VERSION（期望 X.Y.Z 或 X.Y.Z-rc.N）"
VERSION_TAG="v${VERSION}"

# 检查 git tag 是否已存在
if [ "$DO_GIT_TAG" = "1" ] && git rev-parse "refs/tags/${VERSION_TAG}" >/dev/null 2>&1; then
    die "git tag ${VERSION_TAG} 已存在"
fi

# -------------------- 发布摘要 --------------------
GIT_COMMIT="$(git log -1 --pretty=format:'%h  %s')"
GIT_SHA="$(git rev-parse HEAD)"
BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"

step "发布摘要"
echo "  镜像仓库      : ${IMAGE_NAME}"
echo "  版本 tag      : ${VERSION_TAG}"
echo "  同步 latest   : $([ "$DO_LATEST" = "1" ] && echo yes || echo no)"
echo "  同步 git tag  : $([ "$DO_GIT_TAG" = "1" ] && echo yes || echo no)"
echo "  git commit    : ${GIT_COMMIT}"
echo "  构建时间      : ${BUILD_DATE}"
[ "$DRY_RUN" = "1" ] && echo "  ${C_YELLOW}模式          : DRY-RUN（不真实执行）${C_RESET}"

if [ "$ASSUME_YES" != "1" ]; then
    echo
    read -r -p "确认发布？[y/N] " ans
    case "$ans" in [yY]|[yY][eE][sS]) ;; *) die "已取消" ;; esac
fi

# -------------------- build --------------------
START_TS=$(date +%s)

BUILD_TAGS=( -t "${IMAGE_NAME}:${VERSION_TAG}" )
[ "$DO_LATEST" = "1" ] && BUILD_TAGS+=( -t "${IMAGE_NAME}:latest" )

# OCI 标签：便于 docker inspect 时追溯
OCI_LABELS=(
    --label "org.opencontainers.image.version=${VERSION_TAG}"
    --label "org.opencontainers.image.revision=${GIT_SHA}"
    --label "org.opencontainers.image.created=${BUILD_DATE}"
    --label "org.opencontainers.image.source=https://github.com/cropflre/nowen-video"
    --label "org.opencontainers.image.title=nowen-video"
)

step "开始构建"
BUILD_CMD=( docker build "${BUILD_TAGS[@]}" "${OCI_LABELS[@]}" . )
echo "  ${BUILD_CMD[*]}"

BUILD_START=$(date +%s)
run "${BUILD_CMD[@]}"
BUILD_END=$(date +%s)
BUILD_DURATION=$((BUILD_END - BUILD_START))
ok "构建完成，用时 ${BUILD_DURATION}s"

# -------------------- push --------------------
step "推送镜像"

PUSH_START=$(date +%s)
info "推送：${IMAGE_NAME}:${VERSION_TAG}"
run "docker push \"${IMAGE_NAME}:${VERSION_TAG}\""

if [ "$DO_LATEST" = "1" ]; then
    info "推送：${IMAGE_NAME}:latest"
    run "docker push \"${IMAGE_NAME}:latest\""
fi
PUSH_END=$(date +%s)
PUSH_DURATION=$((PUSH_END - PUSH_START))

# 尝试获取 digest
DIGEST=""
if [ "$DRY_RUN" != "1" ]; then
    DIGEST="$(docker inspect --format='{{index .RepoDigests 0}}' "${IMAGE_NAME}:${VERSION_TAG}" 2>/dev/null || echo "")"
fi

# -------------------- git tag --------------------
if [ "$DO_GIT_TAG" = "1" ]; then
    step "打 git tag 并推送到 GitHub"
    # 本地 tag：已存在就跳过创建（可能上次 push 失败后重试）
    if git rev-parse -q --verify "refs/tags/${VERSION_TAG}" >/dev/null 2>&1; then
        info "本地 tag ${VERSION_TAG} 已存在，跳过创建"
    else
        info "git tag -a ${VERSION_TAG} -m 'Release ${VERSION_TAG}'"
        run "git tag -a \"${VERSION_TAG}\" -m \"Release ${VERSION_TAG}\""
    fi
    info "git push origin ${VERSION_TAG}"
    if [ "$DRY_RUN" = "1" ]; then
        echo "  (dry-run) git push origin \"${VERSION_TAG}\""
    elif git push origin "${VERSION_TAG}"; then
        ok "git tag ${VERSION_TAG} 已推送"
    else
        echo
        echo "${C_YELLOW}[!] git push tag 失败（镜像已成功推送至 Docker Hub，本地 tag 已保留）${C_RESET}"
        echo "    常见原因：GitHub 已禁用密码认证，需使用 PAT 或 SSH key"
        echo "    修复方式任选一种，然后补推："
        echo "      git push origin ${VERSION_TAG}"
        echo
        echo "    方案 A（PAT，推荐）："
        echo "      1. https://github.com/settings/tokens 生成 fine-grained token（Contents: RW）"
        echo "      2. git config --global credential.helper store"
        echo "      3. git push origin ${VERSION_TAG}   # 用户名: GitHub 用户名；密码: 粘贴 PAT"
        echo
        echo "    方案 B（SSH key）："
        echo "      1. ssh-keygen -t ed25519 -C \"\$(hostname)\""
        echo "      2. cat ~/.ssh/id_ed25519.pub  → 添加到 https://github.com/settings/keys"
        echo "      3. git remote set-url origin git@github.com:<user>/<repo>.git"
        echo "      4. git push origin ${VERSION_TAG}"
        die "git tag 推送失败"
    fi
else
    info "跳过 git tag（--no-git-tag）"
fi

# -------------------- 完成 --------------------
END_TS=$(date +%s)
TOTAL=$((END_TS - START_TS))

step "发布完成"
echo "  ${C_GREEN}${IMAGE_NAME}:${VERSION_TAG}${C_RESET}  ←  已推送"
[ "$DO_LATEST" = "1" ] && echo "  ${C_GREEN}${IMAGE_NAME}:latest${C_RESET}  ←  已推送"
[ "$DO_GIT_TAG" = "1" ] && echo "  ${C_GREEN}git tag ${VERSION_TAG}${C_RESET}  ←  已推送到 GitHub"
echo "  总耗时        : ${TOTAL}s （build ${BUILD_DURATION}s + push ${PUSH_DURATION}s）"
[ -n "$DIGEST" ] && echo "  digest        : ${DIGEST}"

echo
ok "发布成功 🎉"
