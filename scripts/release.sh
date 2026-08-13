#!/usr/bin/env bash
# =============================================================================
# Nowen Video unified product release orchestrator
#
# Safe order:
#   1. Require clean/synced main and a new semantic version
#   2. Gate the exact commit on Server CI
#   3. Build signed Android + Desktop release candidates and wait
#   4. Build/push the official Docker image and verify its manifest
#   5. Push the product git tag
#   6. Wait for tag-triggered Android/Desktop release workflows
#
# Docker is intentionally published before the git tag: if the server image
# fails, no product tag is created and client release workflows never start.
# =============================================================================
set -euo pipefail

IMAGE_NAME="cropflre/nowen-video"
GITHUB_REPO="cropflre/nowen-video"
DEFAULT_BRANCH="main"
DEFAULT_PLATFORMS="linux/amd64,linux/arm64"
BUILDX_BUILDER="nowen-video-builder"

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
step() { echo; echo "${C_BOLD}${C_CYAN}==== $* ====${C_RESET}"; }
trap 'echo; die "已被用户中断（SIGINT）"' INT

VERSION=""
ASSUME_YES=0
DO_PULL=1
DO_LATEST=1
LATEST_EXPLICIT=0
DO_GIT_TAG=1
RUN_CI_GATE=1
RUN_ACTIONS_PREFLIGHT=1
WAIT_RELEASE_ACTIONS=1
ALLOW_NON_MAIN=0
DRY_RUN=0
PLATFORMS="$DEFAULT_PLATFORMS"
MULTIARCH=1
EXPLICIT_PLATFORM=0

usage() {
    cat <<USAGE
用法: $0 [选项]

默认流程：main/CI 门禁 → Android + Desktop 候选构建 → Docker 发布 →
产品 git tag → 等待 Android/Desktop 正式工作流完成。

  -v, --version VERSION       版本，例如 1.2.9 / v1.2.9 / 1.2.9-rc.1
  -y, --yes                   跳过交互确认
      --no-pull               不 pull；本地 HEAD 仍必须等于 origin/main
      --latest                强制更新 Docker :latest
      --no-latest             不更新 Docker :latest
                              默认稳定版更新 latest，预发布版不更新
      --no-git-tag            只发 Docker，不创建产品 tag
      --no-ci-gate            跳过 GitHub main CI 门禁（不推荐）
      --no-actions-preflight  跳过 Android/Desktop 候选构建（不推荐）
      --no-wait-actions       tag 推送后不等待正式客户端工作流
      --allow-non-main        允许非 main（仅恢复/特殊场景）
      --no-multiarch          只构建本机架构
      --amd64-only            只发布 linux/amd64
      --arm64-only            只发布 linux/arm64
      --platform LIST         自定义 buildx 平台列表
      --dry-run               只展示计划，不执行发布写操作
  -h, --help                  显示帮助

示例:
  $0
  $0 -v 1.2.9 -y
  $0 -v 1.2.9-rc.1 -y
  $0 -v 1.2.9 --dry-run
  $0 -v 1.2.9 --no-git-tag --no-actions-preflight
USAGE
    exit 0
}

while [ $# -gt 0 ]; do
    case "$1" in
        -v|--version) VERSION="${2:-}"; shift 2 ;;
        -y|--yes) ASSUME_YES=1; shift ;;
        --no-pull) DO_PULL=0; shift ;;
        --latest) DO_LATEST=1; LATEST_EXPLICIT=1; shift ;;
        --no-latest) DO_LATEST=0; LATEST_EXPLICIT=1; shift ;;
        --no-git-tag) DO_GIT_TAG=0; shift ;;
        --no-ci-gate) RUN_CI_GATE=0; shift ;;
        --no-actions-preflight) RUN_ACTIONS_PREFLIGHT=0; shift ;;
        --no-wait-actions) WAIT_RELEASE_ACTIONS=0; shift ;;
        --allow-non-main) ALLOW_NON_MAIN=1; shift ;;
        --no-multiarch) MULTIARCH=0; shift ;;
        --amd64-only) MULTIARCH=1; PLATFORMS="linux/amd64"; EXPLICIT_PLATFORM=1; shift ;;
        --arm64-only) MULTIARCH=1; PLATFORMS="linux/arm64"; EXPLICIT_PLATFORM=1; shift ;;
        --platform) MULTIARCH=1; PLATFORMS="${2:-}"; EXPLICIT_PLATFORM=1; shift 2 ;;
        --dry-run) DRY_RUN=1; shift ;;
        -h|--help) usage ;;
        *) die "未知参数: $1（使用 -h 查看帮助）" ;;
    esac
done

[ "$MULTIARCH" = "0" ] && [ "$EXPLICIT_PLATFORM" = "1" ] && die "--no-multiarch 与平台参数互斥"
[ "$MULTIARCH" = "1" ] && [ -z "${PLATFORMS// }" ] && die "--platform 不能为空"

run_argv() {
    if [ "$DRY_RUN" = "1" ]; then
        printf '  %sDRY-RUN%s' "$C_YELLOW" "$C_RESET"
        printf ' %q' "$@"
        printf '\n'
    else
        "$@"
    fi
}
require_cmd() { command -v "$1" >/dev/null 2>&1 || die "缺少命令: $1"; }

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

step "本地与仓库预检"
require_cmd git
require_cmd docker
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "当前目录不是 git 仓库"
[ -f Dockerfile ] || die "仓库根目录未找到 Dockerfile"

if [ -n "$(git status --porcelain)" ]; then
    git status --short | head -30
    die "工作区必须完全干净后才能发布"
fi

CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$CURRENT_BRANCH" != "$DEFAULT_BRANCH" ] && [ "$ALLOW_NON_MAIN" != "1" ]; then
    die "正式发布必须从 ${DEFAULT_BRANCH} 执行；当前分支: ${CURRENT_BRANCH}"
fi

info "当前分支: $CURRENT_BRANCH"
run_argv git fetch origin "$DEFAULT_BRANCH" --tags --prune
if [ "$DO_PULL" = "1" ]; then
    run_argv git pull --ff-only origin "$CURRENT_BRANCH"
fi

LOCAL_SHA="$(git rev-parse HEAD)"
if [ "$CURRENT_BRANCH" = "$DEFAULT_BRANCH" ]; then
    REMOTE_SHA="$(git rev-parse "origin/${DEFAULT_BRANCH}")"
    [ "$LOCAL_SHA" = "$REMOTE_SHA" ] || die "本地 HEAD 不等于 origin/${DEFAULT_BRANCH}；拒绝发布过期代码"
fi
ok "源码已锁定: $(git log -1 --pretty=format:'%h  %s')"

docker info >/dev/null 2>&1 || die "Docker daemon 不可用"
if [ "$MULTIARCH" = "1" ]; then
    docker buildx version >/dev/null 2>&1 || die "docker buildx 不可用"
fi

suggest_next_version() {
    local latest base major rest minor patch
    latest="$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -1 | sed 's/^v//')" || latest=""
    if [ -z "$latest" ]; then echo "0.1.0"; return; fi
    base="${latest%%-*}"; major="${base%%.*}"; rest="${base#*.}"; minor="${rest%%.*}"; patch="${rest#*.}"
    [[ "$major" =~ ^[0-9]+$ ]] || major=0
    [[ "$minor" =~ ^[0-9]+$ ]] || minor=0
    [[ "$patch" =~ ^[0-9]+$ ]] || patch=0
    echo "${major}.${minor}.$((patch + 1))"
}
validate_version() { printf '%s' "$1" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'; }

if [ -z "$VERSION" ]; then
    SUGGEST="$(suggest_next_version)"
    echo
    echo "${C_BOLD}请输入本次发布版本号${C_RESET}"
    echo "建议: ${C_GREEN}${SUGGEST}${C_RESET}（回车采用建议值）"
    read -r -p "> " VERSION
    VERSION="${VERSION:-$SUGGEST}"
fi
VERSION="${VERSION#v}"
validate_version "$VERSION" || die "版本号格式非法: $VERSION"
VERSION_TAG="v${VERSION}"
IS_PRERELEASE=0
[[ "$VERSION" == *-* ]] && IS_PRERELEASE=1

if [ "$LATEST_EXPLICIT" = "0" ]; then
    [ "$IS_PRERELEASE" = "1" ] && DO_LATEST=0 || DO_LATEST=1
fi
[ "$IS_PRERELEASE" = "1" ] && [ "$DO_LATEST" = "1" ] && warn "预发布 ${VERSION_TAG} 将覆盖 Docker :latest（显式选择）"

git rev-parse -q --verify "refs/tags/${VERSION_TAG}" >/dev/null 2>&1 && die "本地 tag ${VERSION_TAG} 已存在"
git ls-remote --exit-code --tags origin "refs/tags/${VERSION_TAG}" >/dev/null 2>&1 && die "远端 tag ${VERSION_TAG} 已存在"

GIT_SHA="$(git rev-parse HEAD)"
GIT_SHORT_SHA="$(git rev-parse --short=12 HEAD)"
BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"

NEED_GH=0
[ "$RUN_CI_GATE" = "1" ] && NEED_GH=1
[ "$RUN_ACTIONS_PREFLIGHT" = "1" ] && NEED_GH=1
[ "$DO_GIT_TAG" = "1" ] && [ "$WAIT_RELEASE_ACTIONS" = "1" ] && NEED_GH=1
if [ "$NEED_GH" = "1" ]; then
    require_cmd gh
    gh auth status >/dev/null 2>&1 || die "GitHub CLI 未登录，请执行 gh auth login"
    gh repo view "$GITHUB_REPO" >/dev/null 2>&1 || die "当前 gh 身份无法访问 ${GITHUB_REPO}"
fi

check_workflow_for_commit() {
    local workflow="$1" label="$2" required="$3" line run_id status conclusion url
    line="$(gh run list --repo "$GITHUB_REPO" --workflow "$workflow" --commit "$GIT_SHA" --event push --limit 1 \
        --json databaseId,status,conclusion,url --jq '.[0] | [.databaseId, .status, (.conclusion // ""), .url] | @tsv' 2>/dev/null || true)"
    if [ -z "$line" ]; then
        [ "$required" = "1" ] && die "未找到 ${label} 对 ${GIT_SHORT_SHA} 的 CI 记录"
        info "${label}: 当前提交未触发，跳过已有运行检查"
        return
    fi
    IFS=$'\t' read -r run_id status conclusion url <<EOF2
$line
EOF2
    if [ "$status" != "completed" ]; then
        info "${label}: 等待 run #${run_id}"
        gh run watch "$run_id" --repo "$GITHUB_REPO" --exit-status
    else
        [ "$conclusion" = "success" ] || die "${label} 未通过: ${conclusion} (${url})"
    fi
    ok "${label}: PASS"
}

dispatch_and_wait() {
    local workflow="$1" label="$2" previous_id new_id attempt
    shift 2
    previous_id="$(gh run list --repo "$GITHUB_REPO" --workflow "$workflow" --event workflow_dispatch --branch "$DEFAULT_BRANCH" --limit 1 \
        --json databaseId --jq '.[0].databaseId // 0' 2>/dev/null || echo 0)"
    info "触发 ${label} 候选构建 ..."
    gh workflow run "$workflow" --repo "$GITHUB_REPO" --ref "$DEFAULT_BRANCH" "$@"
    new_id=""; attempt=0
    while [ "$attempt" -lt 45 ]; do
        new_id="$(gh run list --repo "$GITHUB_REPO" --workflow "$workflow" --event workflow_dispatch --branch "$DEFAULT_BRANCH" --commit "$GIT_SHA" --limit 1 \
            --json databaseId --jq '.[0].databaseId // empty' 2>/dev/null || true)"
        [ -n "$new_id" ] && [ "$new_id" != "$previous_id" ] && break
        sleep 2; attempt=$((attempt + 1))
    done
    [ -n "$new_id" ] || die "无法定位刚触发的 ${label} workflow run"
    info "等待 ${label} run #${new_id} ..."
    gh run watch "$new_id" --repo "$GITHUB_REPO" --exit-status
    ok "${label} 候选构建: PASS"
}

wait_tag_workflow() {
    local workflow="$1" label="$2" run_id attempt
    run_id=""; attempt=0
    while [ "$attempt" -lt 60 ]; do
        run_id="$(gh run list --repo "$GITHUB_REPO" --workflow "$workflow" --event push --branch "$VERSION_TAG" --commit "$GIT_SHA" --limit 1 \
            --json databaseId --jq '.[0].databaseId // empty' 2>/dev/null || true)"
        [ -n "$run_id" ] && break
        sleep 2; attempt=$((attempt + 1))
    done
    [ -n "$run_id" ] || die "未找到 ${label} 的 tag workflow (${VERSION_TAG})"
    info "等待 ${label} 正式工作流 run #${run_id} ..."
    gh run watch "$run_id" --repo "$GITHUB_REPO" --exit-status
    ok "${label} 正式工作流: PASS"
}

step "发布计划"
echo "  repo             : ${GITHUB_REPO}"
echo "  commit           : ${GIT_SHA}"
echo "  version          : ${VERSION_TAG}"
echo "  channel          : $([ "$IS_PRERELEASE" = "1" ] && echo prerelease || echo stable)"
echo "  Docker           : ${IMAGE_NAME}:${VERSION_TAG}"
echo "  Docker latest    : $([ "$DO_LATEST" = "1" ] && echo yes || echo no)"
echo "  Git tag          : $([ "$DO_GIT_TAG" = "1" ] && echo yes || echo no)"
echo "  CI gate          : $([ "$RUN_CI_GATE" = "1" ] && echo yes || echo no)"
echo "  client preflight : $([ "$RUN_ACTIONS_PREFLIGHT" = "1" ] && echo yes || echo no)"
echo "  wait tag Actions : $([ "$WAIT_RELEASE_ACTIONS" = "1" ] && echo yes || echo no)"
echo "  platforms        : $([ "$MULTIARCH" = "1" ] && echo "$PLATFORMS" || echo local)"
[ "$DRY_RUN" = "1" ] && echo "  mode             : DRY-RUN"

if [ "$ASSUME_YES" != "1" ]; then
    echo
    read -r -p "确认按以上计划发布？[y/N] " ans
    case "$ans" in [yY]|[yY][eE][sS]) ;; *) die "已取消" ;; esac
fi

if [ "$DRY_RUN" = "1" ]; then
    step "DRY-RUN 发布顺序"
    [ "$RUN_CI_GATE" = "1" ] && echo "  1. 检查 Server CI（以及存在时的 Android CI）"
    [ "$RUN_ACTIONS_PREFLIGHT" = "1" ] && echo "  2. 触发并等待 Android + Desktop 候选构建"
    echo "  3. 构建并推送 Docker ${IMAGE_NAME}:${VERSION_TAG}"
    [ "$DO_LATEST" = "1" ] && echo "     同时更新 ${IMAGE_NAME}:latest"
    [ "$DO_GIT_TAG" = "1" ] && echo "  4. 创建并推送 git tag ${VERSION_TAG}"
    [ "$DO_GIT_TAG" = "1" ] && [ "$WAIT_RELEASE_ACTIONS" = "1" ] && echo "  5. 等待 Android + Desktop 正式工作流"
    ok "DRY-RUN 完成"
    exit 0
fi

if [ "$RUN_CI_GATE" = "1" ]; then
    step "GitHub CI 门禁"
    check_workflow_for_commit "server-ci.yml" "Server CI" 1
    check_workflow_for_commit "android.yml" "Android CI" 0
fi

if [ "$RUN_ACTIONS_PREFLIGHT" = "1" ]; then
    step "客户端正式候选门禁"
    dispatch_and_wait "release-android.yml" "Android signed release" -f "version_name=${VERSION}"
    dispatch_and_wait "release-desktop.yml" "Desktop release" -f "version_name=${VERSION}" -f "target=windows"
fi

START_TS="$(date +%s)"
BUILD_TAGS=(-t "${IMAGE_NAME}:${VERSION_TAG}")
[ "$DO_LATEST" = "1" ] && BUILD_TAGS+=(-t "${IMAGE_NAME}:latest")
OCI_LABELS=(
    --label "org.opencontainers.image.version=${VERSION}"
    --label "org.opencontainers.image.revision=${GIT_SHA}"
    --label "org.opencontainers.image.created=${BUILD_DATE}"
    --label "org.opencontainers.image.source=https://github.com/${GITHUB_REPO}"
    --label "org.opencontainers.image.title=nowen-video"
    --label "org.opencontainers.image.description=Nowen Video official release image"
)

if [ "$MULTIARCH" = "1" ]; then
    step "准备 Docker buildx"
    if docker buildx inspect "$BUILDX_BUILDER" >/dev/null 2>&1; then
        run_argv docker buildx use "$BUILDX_BUILDER"
    else
        run_argv docker buildx create --name "$BUILDX_BUILDER" --driver docker-container --use
    fi
    run_argv docker buildx inspect --bootstrap

    step "构建并推送 Docker 多架构镜像"
    BUILD_START="$(date +%s)"
    run_argv docker buildx build --platform "$PLATFORMS" -f "$REPO_ROOT/Dockerfile" \
        --build-arg "NOWEN_VERSION=$VERSION" "${BUILD_TAGS[@]}" "${OCI_LABELS[@]}" --push "$REPO_ROOT"
    BUILD_END="$(date +%s)"; BUILD_DURATION=$((BUILD_END - BUILD_START))

    step "验证 Docker manifest"
    MANIFEST_TEXT="$(docker buildx imagetools inspect "${IMAGE_NAME}:${VERSION_TAG}")"
    echo "$MANIFEST_TEXT" | grep -E 'Name:|MediaType:|Digest:|Platform:' | head -30 || true
    old_ifs="$IFS"; IFS=','
    for platform in $PLATFORMS; do
        platform="${platform// /}"
        [ -z "$platform" ] && continue
        printf '%s\n' "$MANIFEST_TEXT" | grep -Eq "Platform:[[:space:]]+${platform//\//\\/}([[:space:]]|$)" || die "远端 manifest 缺少平台: ${platform}"
    done
    IFS="$old_ifs"
else
    step "构建并推送 Docker 单架构镜像"
    BUILD_START="$(date +%s)"
    run_argv docker build -f "$REPO_ROOT/Dockerfile" --build-arg "NOWEN_VERSION=$VERSION" \
        "${BUILD_TAGS[@]}" "${OCI_LABELS[@]}" "$REPO_ROOT"
    run_argv docker push "${IMAGE_NAME}:${VERSION_TAG}"
    [ "$DO_LATEST" = "1" ] && run_argv docker push "${IMAGE_NAME}:latest"
    BUILD_END="$(date +%s)"; BUILD_DURATION=$((BUILD_END - BUILD_START))
fi
ok "Docker 发布完成"

DIGEST="$(docker buildx imagetools inspect "${IMAGE_NAME}:${VERSION_TAG}" 2>/dev/null | sed -n 's/^Digest:[[:space:]]*//p' | head -1 || true)"

if [ "$DO_GIT_TAG" = "1" ]; then
    step "创建产品 git tag"
    git tag -a "$VERSION_TAG" -m "Nowen Video ${VERSION_TAG}"
    if git push origin "$VERSION_TAG"; then
        ok "git tag ${VERSION_TAG} 已推送"
    else
        warn "Docker 已成功发布，但 git tag 推送失败；本地 tag 已保留。"
        warn "修复 GitHub 认证后执行: git push origin ${VERSION_TAG}"
        die "产品 tag 推送失败"
    fi

    if [ "$WAIT_RELEASE_ACTIONS" = "1" ]; then
        step "等待 tag 正式发布工作流"
        wait_tag_workflow "release-android.yml" "Android"
        wait_tag_workflow "release-desktop.yml" "Desktop"
        if gh release view "$VERSION_TAG" --repo "$GITHUB_REPO" >/dev/null 2>&1; then
            RELEASE_LINE="$(gh release view "$VERSION_TAG" --repo "$GITHUB_REPO" --json url,isDraft,isPrerelease --jq '[.url, .isDraft, .isPrerelease] | @tsv')"
            IFS=$'\t' read -r RELEASE_URL RELEASE_DRAFT RELEASE_PRERELEASE <<EOF3
$RELEASE_LINE
EOF3
            ok "GitHub Release 已生成: ${RELEASE_URL}"
            [ "$RELEASE_DRAFT" = "true" ] && info "Release 仍为 Draft，检查产物后再公开"
        else
            die "客户端工作流已结束，但没有找到 ${VERSION_TAG} 的 GitHub Release"
        fi
    fi
fi

END_TS="$(date +%s)"; TOTAL=$((END_TS - START_TS))
step "发布完成"
echo "  Docker          : ${IMAGE_NAME}:${VERSION_TAG}"
[ "$DO_LATEST" = "1" ] && echo "  Docker latest   : ${IMAGE_NAME}:latest"
[ -n "$DIGEST" ] && echo "  Docker digest   : ${DIGEST}"
[ "$DO_GIT_TAG" = "1" ] && echo "  Git tag         : ${VERSION_TAG}"
echo "  commit          : ${GIT_SHA}"
echo "  total time      : ${TOTAL}s (Docker ${BUILD_DURATION}s)"
ok "Nowen Video ${VERSION_TAG} 发版流程完成 🎉"
