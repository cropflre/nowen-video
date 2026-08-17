#!/usr/bin/env bash
# =============================================================================
# Nowen Video product release guard
#
# Default stable release from main:
#   Docker amd64/arm64 + Android APK/AAB + fnOS FPK + Git tag + GitHub Release.
#
# Contract (aligned with nowen-note):
#   - preflight every account/tool/permission before expensive build/push work;
#   - build/verify candidates before public release completion;
#   - GitHub Release remains Draft until all requested channels pass;
#   - only this outer guard is allowed to print the final "release succeeded";
#   - any failed command/check makes the whole release command fail.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ADVANCED_SCRIPT="$SCRIPT_DIR/release-advanced.sh"
FPK_SCRIPT="$SCRIPT_DIR/fpk/build-fpk.mjs"
DOCKER_AUTH_SCRIPT="$SCRIPT_DIR/dockerhub-auth-preflight.mjs"
NOTES_SCRIPT="$SCRIPT_DIR/generate-release-notes.mjs"
DEFAULT_BRANCH="main"
GITHUB_REPO="cropflre/nowen-video"
IMAGE_NAME="cropflre/nowen-video"

for required in "$ADVANCED_SCRIPT" "$FPK_SCRIPT" "$DOCKER_AUTH_SCRIPT" "$NOTES_SCRIPT"; do
  [ -f "$required" ] || { echo "[release] 缺少 $required" >&2; exit 1; }
done

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

RELEASE_STARTED=0
RELEASE_SUCCESS=0
CURRENT_PHASE="初始化"
on_exit() {
  local rc=$?
  if [ "$RELEASE_STARTED" = "1" ] && [ "$RELEASE_SUCCESS" != "1" ]; then
    echo >&2
    echo "${C_RED}[✗] Nowen Video 发版失败${C_RESET}" >&2
    echo "    阶段: ${CURRENT_PHASE}" >&2
    echo "    退出码: ${rc}" >&2
    echo "    规则: 任一渠道/门禁失败，整次发版均判定失败，不输出成功状态。" >&2
  fi
}
trap on_exit EXIT
trap 'CURRENT_PHASE="用户中断"; exit 130' INT

usage() {
  cat <<'EOF'
用法:
  ./scripts/release.sh                         # 交互式四渠道正式发版
  ./scripts/release.sh -v 1.2.6 -y --no-desktop
  ./scripts/release.sh -v 1.2.6 -y --no-desktop --dry-run

Wrapper 选项:
  --no-fpk       不构建/上传飞牛 fnOS .fpk
  --draft        全资产核验后仍保留 GitHub Draft Release

其它参数原样透传给 scripts/release-advanced.sh。
默认正式版：Docker + Android APK/AAB + fnOS FPK + GitHub Release。
EOF
}

validate_version() { printf '%s' "$1" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'; }
latest_stable_version() {
  git tag --list 'v*' --sort=-v:refname 2>/dev/null | sed -n -E 's/^v([0-9]+\.[0-9]+\.[0-9]+)$/\1/p' | head -1
}
suggest_next_patch() {
  local latest="$1" major minor patch
  [ -n "$latest" ] || { echo "0.1.0"; return; }
  IFS=. read -r major minor patch <<<"$latest"
  echo "${major}.${minor}.$((10#$patch + 1))"
}
require_cmd() { command -v "$1" >/dev/null 2>&1 || die "缺少命令: $1"; }

PUBLISH_FPK=1
KEEP_DRAFT=0
DRY_RUN=0
DO_PULL=1
DO_LATEST=1
LATEST_EXPLICIT=0
VERSION=""
HAS_ANDROID=1
HAS_DESKTOP=1
HAS_GIT_TAG=1
WAIT_ACTIONS=1
VERIFY_PLATFORMS=1
EXPECTED_PLATFORMS="linux/amd64,linux/arm64"
RELEASE_SOURCE_SHA=""
FPK_FILE=""
FPK_SUM=""
RELEASE_NOTES_FILE=""
PASSTHROUGH=()

parse_args() {
  local args=("$@") i=0 arg
  while [ "$i" -lt "${#args[@]}" ]; do
    arg="${args[$i]}"
    case "$arg" in
      -h|--help) usage; exit 0 ;;
      --no-fpk) PUBLISH_FPK=0 ;;
      --draft) KEEP_DRAFT=1 ;;
      --dry-run) DRY_RUN=1; PASSTHROUGH+=("$arg") ;;
      --no-pull) DO_PULL=0; PASSTHROUGH+=("$arg") ;;
      --latest) DO_LATEST=1; LATEST_EXPLICIT=1; PASSTHROUGH+=("$arg") ;;
      --no-latest) DO_LATEST=0; LATEST_EXPLICIT=1; PASSTHROUGH+=("$arg") ;;
      --server-only)
        HAS_ANDROID=0; HAS_DESKTOP=0; HAS_GIT_TAG=0; PUBLISH_FPK=0; WAIT_ACTIONS=0
        PASSTHROUGH+=("$arg")
        ;;
      --no-android) HAS_ANDROID=0; PASSTHROUGH+=("$arg") ;;
      --no-desktop) HAS_DESKTOP=0; PASSTHROUGH+=("$arg") ;;
      --no-git-tag) HAS_GIT_TAG=0; PASSTHROUGH+=("$arg") ;;
      --no-wait-actions) WAIT_ACTIONS=0; PASSTHROUGH+=("$arg") ;;
      --amd64-only) EXPECTED_PLATFORMS="linux/amd64"; PASSTHROUGH+=("$arg") ;;
      --arm64-only) EXPECTED_PLATFORMS="linux/arm64"; PASSTHROUGH+=("$arg") ;;
      --no-multiarch) VERIFY_PLATFORMS=0; PASSTHROUGH+=("$arg") ;;
      --platform)
        [ $((i + 1)) -lt "${#args[@]}" ] || die "$arg 缺少平台列表"
        EXPECTED_PLATFORMS="${args[$((i + 1))]}"
        PASSTHROUGH+=("$arg" "${args[$((i + 1))]}")
        i=$((i + 1))
        ;;
      -v|--version)
        [ $((i + 1)) -lt "${#args[@]}" ] || die "$arg 缺少版本号"
        VERSION="${args[$((i + 1))]#v}"
        PASSTHROUGH+=("$arg" "${args[$((i + 1))]}")
        i=$((i + 1))
        ;;
      *) PASSTHROUGH+=("$arg") ;;
    esac
    i=$((i + 1))
  done
}

lock_release_source() {
  CURRENT_PHASE="锁定 main 源码"
  require_cmd git
  git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "当前目录不是 Git 仓库"
  [ -z "$(git status --porcelain)" ] || { git status --short; die "正式发版前工作区必须完全干净"; }
  [ "$(git rev-parse --abbrev-ref HEAD)" = "$DEFAULT_BRANCH" ] || die "正式发版必须从 main 执行"

  git fetch origin "$DEFAULT_BRANCH" --tags --prune || die "无法获取 origin/main/tags"
  if [ "$DO_PULL" = "1" ]; then git pull --ff-only origin "$DEFAULT_BRANCH"; fi
  [ -z "$(git status --porcelain)" ] || die "同步 main 后工作区不干净"

  local local_sha remote_sha
  local_sha="$(git rev-parse HEAD)"
  remote_sha="$(git rev-parse "origin/${DEFAULT_BRANCH}")"
  [ "$local_sha" = "$remote_sha" ] || die "本地 main 不等于 origin/main；拒绝发布不同 commit"
  RELEASE_SOURCE_SHA="$local_sha"

  git rev-parse -q --verify "refs/tags/v${VERSION}" >/dev/null 2>&1 && die "本地 tag v${VERSION} 已存在"
  if git ls-remote --exit-code --tags origin "refs/tags/v${VERSION}" >/dev/null 2>&1; then
    die "远端 tag v${VERSION} 已存在"
  fi
  ok "发布源码锁定: ${RELEASE_SOURCE_SHA}"
}

check_required_workflow() {
  local workflow="$1" label="$2" line run_id status conclusion url
  line="$(gh run list --repo "$GITHUB_REPO" --workflow "$workflow" --commit "$RELEASE_SOURCE_SHA" --event push --limit 1 \
    --json databaseId,status,conclusion,url --jq '.[0] | [.databaseId, .status, (.conclusion // ""), .url] | @tsv' 2>/dev/null || true)"
  [ -n "$line" ] || die "未找到 ${label} 对当前 main commit 的运行记录"
  IFS=$'\t' read -r run_id status conclusion url <<EOF_RUN
$line
EOF_RUN
  if [ "$status" != "completed" ]; then
    info "${label}: 等待 run #${run_id}"
    gh run watch "$run_id" --repo "$GITHUB_REPO" --exit-status
  else
    [ "$conclusion" = "success" ] || die "${label} 未通过: ${conclusion} (${url})"
  fi
  ok "${label}: PASS"
}

check_android_secrets() {
  [ "$HAS_ANDROID" = "1" ] || return 0
  local secrets name missing=0
  secrets="$(gh secret list --repo "$GITHUB_REPO" --json name --jq '.[].name')" \
    || die "无法读取 Android Actions Secrets；拒绝继续"
  for name in ANDROID_KEYSTORE_BASE64 ANDROID_KEYSTORE_PASSWORD ANDROID_KEY_ALIAS ANDROID_KEY_PASSWORD; do
    if ! printf '%s\n' "$secrets" | grep -Fxq "$name"; then
      warn "缺少 Android production secret: $name"
      missing=1
    fi
  done
  [ "$missing" = "0" ] || die "Android 正式签名 Secrets 未配置完整"
  gh workflow view release-android.yml --repo "$GITHUB_REPO" >/dev/null 2>&1 \
    || die "release-android.yml 不可用或无权限触发"
  ok "Android production signing Secrets: 4/4"
}

preflight_release_environment() {
  CURRENT_PHASE="账号、工具与权限预检"
  step "发布账号 / 工具 / 权限预检"

  require_cmd node
  local node_major
  node_major="$(node -p 'Number(process.versions.node.split(".")[0])')"
  [ "${node_major:-0}" -ge 20 ] || die "Node.js 需要 >=20，当前 $(node -v)"
  ok "Node.js: $(node -v)"

  require_cmd docker
  docker info >/dev/null 2>&1 || die "Docker daemon 不可用"
  docker buildx version >/dev/null 2>&1 || die "docker buildx 不可用"
  node "$DOCKER_AUTH_SCRIPT" "$IMAGE_NAME"
  ok "Docker Hub 登录与 ${IMAGE_NAME} push 权限: PASS"

  if [ "$HAS_GIT_TAG" = "1" ] || [ "$HAS_ANDROID" = "1" ] || [ "$PUBLISH_FPK" = "1" ] || [ "$HAS_DESKTOP" = "1" ]; then
    require_cmd gh
    gh auth status >/dev/null 2>&1 || die "GitHub CLI 未登录，请先执行 gh auth login"
    local permission
    permission="$(gh repo view "$GITHUB_REPO" --json viewerPermission --jq '.viewerPermission' 2>/dev/null || true)"
    case "$permission" in ADMIN|MAINTAIN|WRITE) ;; *) die "当前 GitHub 身份没有 ${GITHUB_REPO} 写权限（viewerPermission=${permission:-unknown}）" ;; esac
    ok "GitHub CLI 登录与仓库写权限: ${permission}"
  fi

  if [ "$HAS_GIT_TAG" = "1" ]; then
    local probe_ref
    probe_ref="refs/tags/__nowen_release_preflight_$(git rev-parse --short=8 HEAD)_$(date +%s)"
    git push --dry-run origin "HEAD:${probe_ref}" >/dev/null 2>&1 \
      || die "Git remote 写权限预检失败；无法保证后续推送产品 tag"
    ok "Git remote tag push 权限: PASS (dry-run)"
  fi

  check_android_secrets

  if [ "$HAS_GIT_TAG" = "1" ] || [ "$HAS_ANDROID" = "1" ] || [ "$PUBLISH_FPK" = "1" ]; then
    check_required_workflow "server-ci.yml" "Server CI"
    check_required_workflow "release-contract.yml" "Release Contract"
  fi
}

generate_release_announcement() {
  CURRENT_PHASE="生成发版公告"
  step "生成统一发版公告"
  rm -rf "$REPO_ROOT/dist-release"
  mkdir -p "$REPO_ROOT/dist-release"
  RELEASE_NOTES_FILE="$REPO_ROOT/dist-release/RELEASE_NOTES-v${VERSION}.md"
  local args=(--version "$VERSION" --output "$RELEASE_NOTES_FILE")
  [ "$HAS_ANDROID" = "1" ] || args+=(--no-android)
  [ "$PUBLISH_FPK" = "1" ] || args+=(--no-fpk)
  [ "$HAS_DESKTOP" = "1" ] && args+=(--desktop)
  node "$NOTES_SCRIPT" "${args[@]}"
  [ -s "$RELEASE_NOTES_FILE" ] || die "发版公告生成失败"
  ok "发版公告: $RELEASE_NOTES_FILE"
}

prepare_fpk() {
  [ "$PUBLISH_FPK" = "1" ] || return 0
  [ "$DRY_RUN" = "0" ] || { info "DRY-RUN：跳过 fnOS 实际打包"; return 0; }
  CURRENT_PHASE="构建飞牛 fnOS FPK"
  [[ "$VERSION" != *-* ]] || die "fnOS manifest 要求纯 X.Y.Z；预发布 ${VERSION} 请加 --no-fpk"
  step "飞牛 fnOS FPK 发布前打包"
  rm -rf "$REPO_ROOT/dist-fpk"
  FPK_VERSION="$VERSION" FPK_IMAGE_TAG="v${VERSION}" DOCKERHUB_REPO="$IMAGE_NAME" node "$FPK_SCRIPT"
  FPK_FILE="$REPO_ROOT/dist-fpk/nowen-video-${VERSION}.fpk"
  FPK_SUM="$REPO_ROOT/dist-fpk/SHA256SUMS-fpk.txt"
  [ -s "$FPK_FILE" ] || die "FPK 产物不存在: $FPK_FILE"
  [ -s "$FPK_SUM" ] || die "FPK checksum 不存在: $FPK_SUM"
  [ "$(git rev-parse HEAD)" = "$RELEASE_SOURCE_SHA" ] || die "FPK 打包期间源码 commit 已变化"
  [ -z "$(git status --porcelain)" ] || die "FPK 构建污染了 Git 工作区"
  ok "fnOS 候选产物: $(basename "$FPK_FILE")"
}

run_advanced_stage() {
  CURRENT_PHASE="Docker / Android / GitHub 核心发布"
  step "Docker / Android / GitHub 核心发布"
  local log_file status
  log_file="$(mktemp "${TMPDIR:-/tmp}/nowen-video-release-core.XXXXXX.log")"
  set +e
  NOWEN_RELEASE_ORCHESTRATED=1 bash "$ADVANCED_SCRIPT" "${PASSTHROUGH[@]}" 2>&1 \
    | sed -E 's/Nowen Video (v[^ ]+) 同步发版流程完成 🎉/Nowen Video \1 核心发布阶段完成，等待统一发布守卫最终核验/' \
    | tee "$log_file"
  status=${PIPESTATUS[0]}
  set -e
  if [ "$status" -ne 0 ]; then
    rm -f "$log_file"
    die "核心发布脚本失败"
  fi
  rm -f "$log_file"
  [ "$(git rev-parse HEAD)" = "$RELEASE_SOURCE_SHA" ] || die "核心发布阶段源码 commit 发生变化"
}

publish_and_verify_fpk() {
  [ "$PUBLISH_FPK" = "1" ] || return 0
  [ "$DRY_RUN" = "0" ] || return 0
  CURRENT_PHASE="上传并远端校验飞牛 FPK"
  [ "$(git rev-parse HEAD)" = "$RELEASE_SOURCE_SHA" ] || die "产品发布 commit 与 FPK commit 不一致"
  local tag="v${VERSION}" fpk="nowen-video-${VERSION}.fpk" checksum="SHA256SUMS-fpk.txt" tmp names
  step "上传并核验飞牛 fnOS Release 资产"
  gh release view "$tag" --repo "$GITHUB_REPO" >/dev/null 2>&1 || die "未找到 ${tag} GitHub Release"
  gh release upload "$tag" "$FPK_FILE" "$FPK_SUM" --repo "$GITHUB_REPO" --clobber
  names="$(gh release view "$tag" --repo "$GITHUB_REPO" --json assets --jq '.assets[].name')"
  printf '%s\n' "$names" | grep -Fxq "$fpk" || die "GitHub Release 缺少 $fpk"
  printf '%s\n' "$names" | grep -Fxq "$checksum" || die "GitHub Release 缺少 $checksum"

  tmp="$(mktemp -d "${TMPDIR:-/tmp}/nowen-video-fpk.XXXXXX")"
  gh release download "$tag" --repo "$GITHUB_REPO" --pattern "$fpk" --pattern "$checksum" --dir "$tmp"
  node - "$tmp/$fpk" "$tmp/$checksum" <<'NODE'
const fs = require('fs'); const crypto = require('crypto');
const [file, sums] = process.argv.slice(2);
const expected = fs.readFileSync(sums, 'utf8').trim().split(/\s+/)[0];
const actual = crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
if (!expected || expected !== actual) { console.error(`FPK checksum mismatch: ${actual} != ${expected}`); process.exit(1); }
console.log(`FPK remote checksum OK: ${actual}`);
NODE
  rm -rf -- "$tmp"
  ok "飞牛 FPK 远端资产与 SHA256: PASS"
}

verify_docker_release() {
  CURRENT_PHASE="最终校验 Docker Hub"
  local version_ref="${IMAGE_NAME}:v${VERSION}" manifest version_digest latest_digest old_ifs platform
  manifest="$(docker buildx imagetools inspect "$version_ref")" || die "无法读取远端 Docker manifest: $version_ref"
  version_digest="$(printf '%s\n' "$manifest" | sed -n 's/^Digest:[[:space:]]*//p' | head -1)"
  [ -n "$version_digest" ] || die "Docker v${VERSION} 缺少远端 digest"

  if [ "$VERIFY_PLATFORMS" = "1" ]; then
    old_ifs="$IFS"; IFS=','
    for platform in $EXPECTED_PLATFORMS; do
      platform="${platform// /}"
      [ -z "$platform" ] && continue
      printf '%s\n' "$manifest" | grep -Eq "Platform:[[:space:]]+${platform//\//\\/}([[:space:]]|$)" \
        || { IFS="$old_ifs"; die "Docker manifest 缺少平台: $platform"; }
    done
    IFS="$old_ifs"
  fi

  if [ "$DO_LATEST" = "1" ]; then
    latest_digest="$(docker buildx imagetools inspect "${IMAGE_NAME}:latest" | sed -n 's/^Digest:[[:space:]]*//p' | head -1)"
    [ "$latest_digest" = "$version_digest" ] || die "Docker latest digest 与 v${VERSION} 不一致"
  fi
  ok "Docker Hub manifest/digest: PASS (${version_digest})"
}

verify_remote_tag() {
  [ "$HAS_GIT_TAG" = "1" ] || return 0
  CURRENT_PHASE="最终校验 Git tag"
  local tag="v${VERSION}" remote_commit
  remote_commit="$(git ls-remote origin "refs/tags/${tag}^{}" | awk 'NR==1{print $1}')"
  if [ -z "$remote_commit" ]; then
    remote_commit="$(git ls-remote origin "refs/tags/${tag}" | awk 'NR==1{print $1}')"
  fi
  [ "$remote_commit" = "$RELEASE_SOURCE_SHA" ] || die "远端 tag ${tag} 未指向本次 main commit"
  ok "Git tag ${tag} -> ${RELEASE_SOURCE_SHA}: PASS"
}

verify_and_publish_release() {
  [ "$HAS_GIT_TAG" = "1" ] || return 0
  CURRENT_PHASE="最终校验 GitHub Release"
  local tag="v${VERSION}" names state is_draft is_prerelease body
  names="$(gh release view "$tag" --repo "$GITHUB_REPO" --json assets --jq '.assets[].name' 2>/dev/null || true)"

  if [ "$HAS_ANDROID" = "1" ]; then
    for asset in "nowen-video-android-${VERSION}.apk" "nowen-video-android-${VERSION}.aab" "release-manifest.json" "SHA256SUMS.txt"; do
      printf '%s\n' "$names" | grep -Fxq "$asset" || die "Release 缺少 Android 资产: $asset"
    done
  fi
  if [ "$PUBLISH_FPK" = "1" ]; then
    printf '%s\n' "$names" | grep -Fxq "nowen-video-${VERSION}.fpk" || die "Release 缺少 fnOS FPK"
    printf '%s\n' "$names" | grep -Fxq "SHA256SUMS-fpk.txt" || die "Release 缺少 fnOS checksum"
  fi

  [ -s "$RELEASE_NOTES_FILE" ] || die "统一发版公告不存在"
  gh release edit "$tag" --repo "$GITHUB_REPO" --notes-file "$RELEASE_NOTES_FILE" >/dev/null
  body="$(gh release view "$tag" --repo "$GITHUB_REPO" --json body --jq '.body')"
  printf '%s' "$body" | grep -Fq "Nowen Video v${VERSION} 发布公告" || die "GitHub Release 正文未更新为统一发版公告"

  if [ "$KEEP_DRAFT" = "1" ]; then
    gh release edit "$tag" --repo "$GITHUB_REPO" --draft=true >/dev/null
  elif [[ "$VERSION" == *-* ]]; then
    gh release edit "$tag" --repo "$GITHUB_REPO" --draft=false --prerelease=true >/dev/null
  else
    gh release edit "$tag" --repo "$GITHUB_REPO" --draft=false --prerelease=false >/dev/null
  fi

  state="$(gh release view "$tag" --repo "$GITHUB_REPO" --json isDraft,isPrerelease --jq '[.isDraft,.isPrerelease] | @tsv')"
  IFS=$'\t' read -r is_draft is_prerelease <<EOF_STATE
$state
EOF_STATE
  if [ "$KEEP_DRAFT" = "1" ]; then
    [ "$is_draft" = "true" ] || die "要求保留 Draft，但 Release 已公开"
    ok "GitHub Release 全资产/公告: PASS（按要求保持 Draft）"
  else
    [ "$is_draft" = "false" ] || die "GitHub Release 仍是 Draft"
    if [[ "$VERSION" == *-* ]]; then
      [ "$is_prerelease" = "true" ] || die "预发布版本未标记 prerelease"
    else
      [ "$is_prerelease" = "false" ] || die "稳定版被错误标记为 prerelease"
    fi
    ok "GitHub Release 全资产/公告/公开状态: PASS"
  fi
}

run_product_release() {
  [ -n "$VERSION" ] || die "无法确定版本号"
  validate_version "$VERSION" || die "版本格式不正确: $VERSION"
  RELEASE_STARTED=1

  if [ "$LATEST_EXPLICIT" = "0" ] && [[ "$VERSION" == *-* ]]; then DO_LATEST=0; fi
  if [ "$PUBLISH_FPK" = "1" ]; then
    [ "$HAS_ANDROID" = "1" ] || die "FPK 默认依赖 Android workflow 创建产品 Release；--no-android 时请同时加 --no-fpk"
    [ "$HAS_GIT_TAG" = "1" ] || die "FPK 发布需要产品 git tag"
    [ "$WAIT_ACTIONS" = "1" ] || die "FPK 发布需要等待 GitHub Release；请移除 --no-wait-actions 或加 --no-fpk"
  fi

  lock_release_source
  preflight_release_environment
  generate_release_announcement
  prepare_fpk

  if [ "$DRY_RUN" = "1" ]; then
    CURRENT_PHASE="DRY-RUN"
    run_advanced_stage
    RELEASE_SUCCESS=1
    step "DRY-RUN 完成"
    ok "登录、权限、当前 main CI/Release Contract 与发布计划均已通过预检；未执行发布写操作"
    return 0
  fi

  run_advanced_stage
  publish_and_verify_fpk

  step "最终全渠道验收"
  verify_docker_release
  verify_remote_tag
  verify_and_publish_release

  [ "$(git rev-parse HEAD)" = "$RELEASE_SOURCE_SHA" ] || die "发版结束时源码 commit 已变化"
  CURRENT_PHASE="完成"
  RELEASE_SUCCESS=1
  step "统一发版成功"
  echo "  commit        : ${RELEASE_SOURCE_SHA}"
  echo "  Docker        : ${IMAGE_NAME}:v${VERSION}"
  [ "$DO_LATEST" = "1" ] && echo "  Docker latest : ${IMAGE_NAME}:latest"
  [ "$HAS_ANDROID" = "1" ] && echo "  Android       : APK + AAB + release manifest/checksums"
  [ "$PUBLISH_FPK" = "1" ] && echo "  飞牛 fnOS     : nowen-video-${VERSION}.fpk + SHA256"
  [ "$HAS_GIT_TAG" = "1" ] && echo "  GitHub        : v${VERSION} Release"
  echo "  发版公告      : ${RELEASE_NOTES_FILE}"
  if [ "$KEEP_DRAFT" = "1" ]; then
    ok "Nowen Video v${VERSION} 全渠道构建与核验成功；GitHub Release 按要求保持 Draft"
  else
    ok "Nowen Video v${VERSION} 全渠道发版成功 🎉"
  fi
}

cd "$REPO_ROOT"
if [ "$#" -gt 0 ]; then
  parse_args "$@"
  [ "$PUBLISH_FPK" = "0" ] || [ -n "$VERSION" ] || die "带参数发布 FPK 时请显式提供 -v X.Y.Z；或使用 --no-fpk"
  run_product_release
  exit 0
fi

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "当前目录不是 Git 仓库"
CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$CURRENT_BRANCH" != "$DEFAULT_BRANCH" ]; then
  warn "当前分支是 ${CURRENT_BRANCH}，正式发版需要 main。"
  read -r -p "是否自动切换到 main？[Y/n] " answer
  case "${answer:-y}" in
    y|Y|yes|YES|Yes) [ -z "$(git status --porcelain)" ] || die "工作区不干净"; git checkout main ;;
    *) die "请切换到 main 后再发版" ;;
  esac
fi

git fetch origin main --tags --prune || die "获取 origin/main 失败"
LATEST="$(latest_stable_version)"; SUGGEST="$(suggest_next_patch "$LATEST")"
echo; printf '%s\n' "${C_BOLD}Nowen Video 一键产品发版${C_RESET}"
echo "默认目标：Docker + Android + 飞牛 fnOS + GitHub Release"
echo "发布规则：先检查登录/权限/CI；任一渠道失败，整次发版失败。"
read -r -p "版本号 [${SUGGEST}]: " VERSION
VERSION="${VERSION:-$SUGGEST}"; VERSION="${VERSION#v}"
validate_version "$VERSION" || die "版本格式不正确: $VERSION"

echo; echo "1) Docker + Android + 飞牛 fnOS + GitHub Release  [默认]"
echo "2) 上述全部 + Windows Desktop"
echo "3) 仅 Docker Server"
read -r -p "请选择 [1/2/3，默认 1]: " choice; choice="${choice:-1}"
PASSTHROUGH=(-v "$VERSION" -y)
case "$choice" in
  1) HAS_DESKTOP=0; PASSTHROUGH+=(--no-desktop) ;;
  2) HAS_DESKTOP=1 ;;
  3) PUBLISH_FPK=0; HAS_ANDROID=0; HAS_DESKTOP=0; HAS_GIT_TAG=0; WAIT_ACTIONS=0; PASSTHROUGH+=(--server-only) ;;
  *) die "无效选择: $choice" ;;
esac

echo; echo "版本      : v${VERSION}"
echo "源码      : main @ $(git rev-parse --short=12 HEAD)"
echo "预检      : GitHub 登录/写权限 + Docker Hub 登录/push 权限 + CI + Release Contract"
echo "Docker    : linux/amd64 + linux/arm64"
[ "$HAS_ANDROID" = "1" ] && echo "Android   : APK + AAB（production signing）"
[ "$PUBLISH_FPK" = "1" ] && echo "飞牛      : nowen-video-${VERSION}.fpk"
[ "$HAS_GIT_TAG" = "1" ] && echo "GitHub    : v${VERSION}（全资产验证后自动公开）"
echo "公告      : 自动参考 nowen-note 风格按提交分类生成"
read -r -p "确认开始？[Y/n] " answer
case "${answer:-y}" in y|Y|yes|YES|Yes) ;; *) die "已取消" ;; esac
run_product_release
