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
# 多架构平台：覆盖 x86_64 服务器 + ARM64 设备（OES / A311D / OECT / RK3566 等）
DEFAULT_PLATFORMS="linux/amd64,linux/arm64"
# buildx builder 名称（自动创建）
BUILDX_BUILDER="nowen-video-builder"

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

# Ctrl-C 友好退出
trap 'echo; die "已被用户中断（SIGINT）"' INT

# -------------------- 参数解析 --------------------
VERSION=""
ASSUME_YES=0
DO_PULL=1
DO_LATEST=1
DO_GIT_TAG=1
DRY_RUN=0
# 多架构相关
PLATFORMS="$DEFAULT_PLATFORMS"
MULTIARCH=1

# 跟踪用户是否通过命令行显式指定了某些选项
# （显式指定过，则交互式菜单不再询问，直接用用户给的值）
EXPLICIT_LATEST=0
EXPLICIT_GIT_TAG=0
EXPLICIT_MULTIARCH=0
EXPLICIT_PLATFORM=0

usage() {
    cat <<EOF
用法: $0 [选项]

不带任何参数运行时，进入${C_BOLD}懒人交互模式${C_RESET}（一路回车即用默认值）。

选项:
  -v, --version VERSION    指定版本号（例: 1.3.0 或 v1.3.0）
  -y, --yes                跳过所有确认（全默认，适合 CI）
      --no-pull            不执行 git pull
      --no-latest          不打 :latest tag
      --no-git-tag         不打 git tag / 不推送到 GitHub
      --no-multiarch       只构建本机架构（单架构 + 本地 load，不走 buildx push）
      --platform LIST      指定构建平台（默认: $DEFAULT_PLATFORMS）
                           示例: linux/amd64,linux/arm64,linux/arm/v7
      --dry-run            仅打印命令，不真实执行
  -h, --help               显示帮助

示例:
  $0                              # 全交互（懒人模式）
  $0 -y                           # 全默认，适合 CI / 重复发布
  $0 -v 1.3.0                     # 指定版本 + 其余交互
  $0 -v 1.3.0-rc.1 --no-latest    # 预发布，不动 latest
EOF
    exit 0
}

while [ $# -gt 0 ]; do
    case "$1" in
        -v|--version)   VERSION="${2:-}"; shift 2 ;;
        -y|--yes)       ASSUME_YES=1; shift ;;
        --no-pull)      DO_PULL=0; shift ;;
        --no-latest)    DO_LATEST=0; EXPLICIT_LATEST=1; shift ;;
        --no-git-tag)   DO_GIT_TAG=0; EXPLICIT_GIT_TAG=1; shift ;;
        --no-multiarch) MULTIARCH=0; EXPLICIT_MULTIARCH=1; shift ;;
        --platform)     PLATFORMS="${2:-}"; EXPLICIT_PLATFORM=1; shift 2 ;;
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

# run_argv：按参数数组原样执行（不经 eval 二次解析），用于参数含空格/等号等
# 特殊字符的场景（例如 docker build 的 --label k=v 参数）。
run_argv() {
    if [ "$DRY_RUN" = "1" ]; then
        echo "  ${C_YELLOW}DRY-RUN${C_RESET} $*"
    else
        "$@"
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

# buildx 可用（多架构构建必需）
if [ "$MULTIARCH" = "1" ]; then
    docker buildx version >/dev/null 2>&1 \
        || die "docker buildx 不可用。Debian 上安装: apt install docker-buildx-plugin，或参考 https://docs.docker.com/build/install-buildx/"
fi

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

# 工作区脏检查（包含未跟踪文件）
if [ -n "$(git status --porcelain)" ]; then
    warn "工作区有未提交的改动或未跟踪文件："
    git status --short | head -20
    die "请先提交/stash/忽略后再发布"
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
    local major="${base%%.*}"
    local rest="${base#*.}"
    local minor="${rest%%.*}"
    local patch="${rest#*.}"
    # 防御：非数字时退化为 0
    [[ "$major" =~ ^[0-9]+$ ]] || major=0
    [[ "$minor" =~ ^[0-9]+$ ]] || minor=0
    [[ "$patch" =~ ^[0-9]+$ ]] || patch=0
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

# -------------------- 懒人交互菜单 --------------------
# 非 -y 模式下，针对用户未显式指定的选项逐项询问，全部带默认值，回车即选默认。
# 已通过命令行显式指定过的选项会被跳过，尊重用户的输入。
if [ "$ASSUME_YES" != "1" ]; then
    step "发布选项（回车使用默认值）"

    # 1) 构建模式
    if [ "$EXPLICIT_MULTIARCH" = "0" ] && [ "$EXPLICIT_PLATFORM" = "0" ]; then
        echo "  构建模式："
        echo "    ${C_GREEN}1)${C_RESET} 多架构（${DEFAULT_PLATFORMS}） ${C_BOLD}[默认，推荐]${C_RESET}"
        echo "    2) 仅本机架构（最快，仅本地自用）"
        echo "    3) 自定义平台列表"
        read -r -p "  选择 [1/2/3]（默认 1）: " mode_ans
        case "${mode_ans:-1}" in
            1|"")
                MULTIARCH=1
                PLATFORMS="$DEFAULT_PLATFORMS"
                ;;
            2)
                MULTIARCH=0
                ;;
            3)
                MULTIARCH=1
                read -r -p "  输入平台列表（逗号分隔，如 linux/amd64,linux/arm64,linux/arm/v7）: " custom_platforms
                if [ -z "$custom_platforms" ]; then
                    warn "未输入，回退到默认 $DEFAULT_PLATFORMS"
                    PLATFORMS="$DEFAULT_PLATFORMS"
                else
                    PLATFORMS="$custom_platforms"
                fi
                ;;
            *)
                die "无效选择：$mode_ans"
                ;;
        esac
        echo
    fi

    # 2) 是否同步打 :latest
    if [ "$EXPLICIT_LATEST" = "0" ]; then
        default_hint="Y/n"
        read -r -p "  同时打 :latest tag？[${default_hint}]（默认 Y）: " latest_ans
        case "${latest_ans:-y}" in
            [yY]|[yY][eE][sS]) DO_LATEST=1 ;;
            [nN]|[nN][oO])     DO_LATEST=0 ;;
            *)                 DO_LATEST=1 ;;
        esac
        echo
    fi

    # 3) 是否打 git tag 并推送
    if [ "$EXPLICIT_GIT_TAG" = "0" ]; then
        read -r -p "  同时打 git tag 并推送到 GitHub？[Y/n]（默认 Y）: " tag_ans
        case "${tag_ans:-y}" in
            [yY]|[yY][eE][sS]) DO_GIT_TAG=1 ;;
            [nN]|[nN][oO])     DO_GIT_TAG=0 ;;
            *)                 DO_GIT_TAG=1 ;;
        esac
        echo
    fi

    # 用户选择打 git tag 时，再次校验 tag 冲突（避免用户在菜单里改主意后漏检）
    if [ "$DO_GIT_TAG" = "1" ] && git rev-parse "refs/tags/${VERSION_TAG}" >/dev/null 2>&1; then
        die "git tag ${VERSION_TAG} 已存在"
    fi
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
if [ "$MULTIARCH" = "1" ]; then
    echo "  构建架构      : ${PLATFORMS}"
    echo "  构建模式      : buildx 多架构（build + push 合并）"
else
    echo "  构建架构      : 本机单架构（--no-multiarch）"
    echo "  构建模式      : 经典 docker build + docker push"
fi
echo "  git commit    : ${GIT_COMMIT}"
echo "  构建时间      : ${BUILD_DATE}"
[ "$DRY_RUN" = "1" ] && echo "  ${C_YELLOW}模式          : DRY-RUN（不真实执行）${C_RESET}"

if [ "$ASSUME_YES" != "1" ]; then
    echo
    read -r -p "确认发布？[y/N] " ans
    case "$ans" in [yY]|[yY][eE][sS]) ;; *) die "已取消" ;; esac
fi

# -------------------- build & push --------------------
# 多架构模式：使用 buildx，一次性 build + push（多架构 image 无法 load 到
# 本地 daemon，必须直接推远端）
# 单架构模式：沿用传统 docker build + docker push 两步
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
    --label "org.opencontainers.image.description=nowen-video release image (multi-arch: ${PLATFORMS})"
)

# Docker Hub 登录预检：避免最后 push 阶段才失败
if [ "$DRY_RUN" != "1" ]; then
    if ! docker system info 2>/dev/null | grep -qE '^\s*Username:'; then
        warn "未检测到 Docker Hub 登录态（docker info 未发现 Username）"
        warn "若推送到 ${IMAGE_NAME} 需要认证，请先执行：docker login"
        if [ "$ASSUME_YES" != "1" ]; then
            read -r -p "仍然继续？[y/N] " ans
            case "$ans" in [yY]|[yY][eE][sS]) ;; *) die "已取消" ;; esac
        fi
    fi
fi

if [ "$MULTIARCH" = "1" ]; then
    # ========== 多架构路径 ==========
    step "准备 buildx builder"
    # 如果当前 current builder 不是 docker-container 驱动，就创建/使用专用 builder
    NEED_BUILDER=1
    if docker buildx inspect "$BUILDX_BUILDER" >/dev/null 2>&1; then
        info "复用已存在的 builder: $BUILDX_BUILDER"
        run_argv docker buildx use "$BUILDX_BUILDER"
        NEED_BUILDER=0
    fi
    if [ "$NEED_BUILDER" = "1" ]; then
        info "创建 buildx builder: $BUILDX_BUILDER（docker-container 驱动）"
        run_argv docker buildx create --name "$BUILDX_BUILDER" --driver docker-container --use
    fi
    # 启动并拉取 QEMU 模拟器（跨架构构建在 x86 主机上需要）
    info "初始化 builder（bootstrap QEMU 多架构支持）"
    run_argv docker buildx inspect --bootstrap

    step "开始构建并推送（多架构：${PLATFORMS}）"
    BUILD_CMD=(
        docker buildx build
        --platform "$PLATFORMS"
        -f "$REPO_ROOT/Dockerfile"
        "${BUILD_TAGS[@]}"
        "${OCI_LABELS[@]}"
        --push
        "$REPO_ROOT"
    )
    echo "  ${BUILD_CMD[*]}"

    BUILD_START=$(date +%s)
    run_argv "${BUILD_CMD[@]}"
    BUILD_END=$(date +%s)
    BUILD_DURATION=$((BUILD_END - BUILD_START))
    PUSH_DURATION=0   # buildx --push 已经推送完成，push 阶段耗时合并进 build
    ok "多架构构建+推送完成，用时 ${BUILD_DURATION}s"

    # 拉取 manifest 确认多架构
    if [ "$DRY_RUN" != "1" ]; then
        info "远端 manifest 摘要（${VERSION_TAG}）："
        docker buildx imagetools inspect "${IMAGE_NAME}:${VERSION_TAG}" 2>/dev/null \
            | grep -E 'Name:|MediaType:|Platform:' | head -20 || true
        if [ "$DO_LATEST" = "1" ]; then
            info "远端 manifest 摘要（latest）："
            docker buildx imagetools inspect "${IMAGE_NAME}:latest" 2>/dev/null \
                | grep -E 'Name:|MediaType:|Platform:' | head -20 || true
        fi
    fi
else
    # ========== 单架构路径（沿用经典 build + push） ==========
    step "开始构建（单架构）"
    # 明确 -f Dockerfile 与上下文路径 "$REPO_ROOT"，避免个别环境下 docker build 被
    # 劫持为 buildx bake 模式时无法正确定位 Dockerfile
    BUILD_CMD=( docker build -f "$REPO_ROOT/Dockerfile" "${BUILD_TAGS[@]}" "${OCI_LABELS[@]}" "$REPO_ROOT" )
    echo "  ${BUILD_CMD[*]}"

    BUILD_START=$(date +%s)
    run_argv "${BUILD_CMD[@]}"
    BUILD_END=$(date +%s)
    BUILD_DURATION=$((BUILD_END - BUILD_START))
    ok "构建完成，用时 ${BUILD_DURATION}s"

    step "推送镜像"
    PUSH_START=$(date +%s)
    info "推送：${IMAGE_NAME}:${VERSION_TAG}"
    run_argv docker push "${IMAGE_NAME}:${VERSION_TAG}"
    if [ "$DO_LATEST" = "1" ]; then
        info "推送：${IMAGE_NAME}:latest"
        run_argv docker push "${IMAGE_NAME}:latest"
    fi
    PUSH_END=$(date +%s)
    PUSH_DURATION=$((PUSH_END - PUSH_START))
fi

# 尝试获取 digest
DIGEST=""
if [ "$DRY_RUN" != "1" ]; then
    if [ "$MULTIARCH" = "1" ]; then
        # 多架构：从 imagetools 读取 manifest list digest
        DIGEST="$(docker buildx imagetools inspect "${IMAGE_NAME}:${VERSION_TAG}" --format '{{.Manifest.Digest}}' 2>/dev/null || echo "")"
        [ -n "$DIGEST" ] && DIGEST="${IMAGE_NAME}@${DIGEST}"
    else
        DIGEST="$(docker inspect --format='{{index .RepoDigests 0}}' "${IMAGE_NAME}:${VERSION_TAG}" 2>/dev/null || echo "")"
    fi
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
if [ "$MULTIARCH" = "1" ]; then
    echo "  总耗时        : ${TOTAL}s （buildx build+push ${BUILD_DURATION}s）"
else
    echo "  总耗时        : ${TOTAL}s （build ${BUILD_DURATION}s + push ${PUSH_DURATION}s）"
fi
[ -n "$DIGEST" ] && echo "  digest        : ${DIGEST}"

echo
ok "发布成功 🎉"
