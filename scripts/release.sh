#!/usr/bin/env bash
# =============================================================================
# Nowen Video product release guard
#
# Default stable release from main:
#   Docker amd64/arm64 + Android APK/AAB + fnOS FPK + Git tag + GitHub Release.
# The underlying Docker/Android/Desktop contract remains in release-advanced.sh.
# This wrapper adds fnOS preflight packaging and only publishes the draft GitHub
# Release after all requested product assets are verified remotely.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ADVANCED_SCRIPT="$SCRIPT_DIR/release-advanced.sh"
FPK_SCRIPT="$SCRIPT_DIR/fpk/build-fpk.mjs"
DEFAULT_BRANCH="main"
GITHUB_REPO="cropflre/nowen-video"
IMAGE_NAME="cropflre/nowen-video"

[ -f "$ADVANCED_SCRIPT" ] || { echo "[release] 缺少 $ADVANCED_SCRIPT" >&2; exit 1; }
[ -f "$FPK_SCRIPT" ] || { echo "[release] 缺少 $FPK_SCRIPT" >&2; exit 1; }

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
trap 'echo; die "已取消发版"' INT

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

PUBLISH_FPK=1
KEEP_DRAFT=0
DRY_RUN=0
VERSION=""
HAS_ANDROID=1
HAS_GIT_TAG=1
WAIT_ACTIONS=1
PASSTHROUGH=()

parse_args() {
  local args=("$@") i=0 arg
  while [ "$i" -lt "${#args[@]}" ]; do
    arg="${args[$i]}"
    case "$arg" in
      --no-fpk) PUBLISH_FPK=0 ;;
      --draft) KEEP_DRAFT=1 ;;
      --dry-run) DRY_RUN=1; PASSTHROUGH+=("$arg") ;;
      --server-only) HAS_ANDROID=0; HAS_GIT_TAG=0; PUBLISH_FPK=0; PASSTHROUGH+=("$arg") ;;
      --no-android) HAS_ANDROID=0; PASSTHROUGH+=("$arg") ;;
      --no-git-tag) HAS_GIT_TAG=0; PASSTHROUGH+=("$arg") ;;
      --no-wait-actions) WAIT_ACTIONS=0; PASSTHROUGH+=("$arg") ;;
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

prepare_fpk() {
  [ "$PUBLISH_FPK" = "1" ] || return 0
  [ "$DRY_RUN" = "0" ] || { info "DRY-RUN：跳过 fnOS 实际打包"; return 0; }
  [[ "$VERSION" != *-* ]] || die "fnOS manifest 要求纯 X.Y.Z；预发布 ${VERSION} 请加 --no-fpk"
  command -v node >/dev/null 2>&1 || die "飞牛打包需要 Node.js 20+"

  step "飞牛 fnOS FPK 发布前打包"
  rm -rf "$REPO_ROOT/dist-fpk"
  FPK_VERSION="$VERSION" \
  FPK_IMAGE_TAG="v${VERSION}" \
  DOCKERHUB_REPO="$IMAGE_NAME" \
    node "$FPK_SCRIPT"

  FPK_FILE="$REPO_ROOT/dist-fpk/nowen-video-${VERSION}.fpk"
  FPK_SUM="$REPO_ROOT/dist-fpk/SHA256SUMS-fpk.txt"
  [ -s "$FPK_FILE" ] || die "FPK 产物不存在: $FPK_FILE"
  [ -s "$FPK_SUM" ] || die "FPK checksum 不存在: $FPK_SUM"
  ok "fnOS 候选产物已生成: $(basename "$FPK_FILE")"
}

publish_and_verify_fpk() {
  [ "$PUBLISH_FPK" = "1" ] || return 0
  [ "$DRY_RUN" = "0" ] || return 0
  command -v gh >/dev/null 2>&1 || die "上传 FPK 需要 gh CLI"
  local tag="v${VERSION}" fpk="nowen-video-${VERSION}.fpk" checksum="SHA256SUMS-fpk.txt"
  local tmp names

  step "上传并核验飞牛 fnOS Release 资产"
  gh release view "$tag" --repo "$GITHUB_REPO" >/dev/null 2>&1 || die "未找到 ${tag} GitHub Release"
  gh release upload "$tag" "$FPK_FILE" "$FPK_SUM" --repo "$GITHUB_REPO" --clobber
  names="$(gh release view "$tag" --repo "$GITHUB_REPO" --json assets --jq '.assets[].name')"
  printf '%s\n' "$names" | grep -Fxq "$fpk" || die "GitHub Release 缺少 $fpk"
  printf '%s\n' "$names" | grep -Fxq "$checksum" || die "GitHub Release 缺少 $checksum"

  tmp="$(mktemp -d "${TMPDIR:-/tmp}/nowen-video-fpk.XXXXXX")"
  trap 'rm -rf -- "${tmp:-}"' RETURN
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
  trap - RETURN
  ok "飞牛 FPK 远端资产与 SHA256 均已核验"
}

verify_product_release() {
  [ "$DRY_RUN" = "0" ] || return 0
  [ "$HAS_GIT_TAG" = "1" ] || return 0
  local tag="v${VERSION}" names
  command -v gh >/dev/null 2>&1 || die "最终 Release 核验需要 gh CLI"
  names="$(gh release view "$tag" --repo "$GITHUB_REPO" --json assets --jq '.assets[].name' 2>/dev/null || true)"
  if [ "$HAS_ANDROID" = "1" ]; then
    printf '%s\n' "$names" | grep -Fxq "nowen-video-android-${VERSION}.apk" || die "Release 缺少 Android APK"
    printf '%s\n' "$names" | grep -Fxq "nowen-video-android-${VERSION}.aab" || die "Release 缺少 Android AAB"
  fi
  if [ "$PUBLISH_FPK" = "1" ]; then
    printf '%s\n' "$names" | grep -Fxq "nowen-video-${VERSION}.fpk" || die "Release 缺少 fnOS FPK"
  fi

  if [ "$KEEP_DRAFT" = "1" ]; then
    ok "所有请求渠道已核验；按 --draft 要求保留 Draft"
    return 0
  fi
  if [[ "$VERSION" == *-* ]]; then
    gh release edit "$tag" --repo "$GITHUB_REPO" --draft=false --prerelease=true >/dev/null
  else
    gh release edit "$tag" --repo "$GITHUB_REPO" --draft=false --prerelease=false >/dev/null
  fi
  ok "GitHub Release ${tag} 已通过全渠道核验并正式发布"
}

run_product_release() {
  [ -n "$VERSION" ] || die "无法确定版本号"
  validate_version "$VERSION" || die "版本格式不正确: $VERSION"
  if [ "$PUBLISH_FPK" = "1" ]; then
    [ "$HAS_ANDROID" = "1" ] || die "默认 FPK 发布依赖 Android workflow 创建产品 Release；使用 --no-android 时请同时加 --no-fpk"
    [ "$HAS_GIT_TAG" = "1" ] || die "FPK 发布需要产品 git tag"
    [ "$WAIT_ACTIONS" = "1" ] || die "FPK 发布需要等待 GitHub Release；请移除 --no-wait-actions 或加 --no-fpk"
  fi
  prepare_fpk
  step "Docker / Android / GitHub 产品发布"
  bash "$ADVANCED_SCRIPT" "${PASSTHROUGH[@]}"
  publish_and_verify_fpk
  verify_product_release

  step "统一发版完成"
  echo "  commit        : $(git rev-parse HEAD)"
  echo "  Docker        : ${IMAGE_NAME}:v${VERSION}"
  [ "$HAS_ANDROID" = "1" ] && echo "  Android       : APK + AAB"
  [ "$PUBLISH_FPK" = "1" ] && echo "  飞牛 fnOS     : nowen-video-${VERSION}.fpk"
  [ "$HAS_GIT_TAG" = "1" ] && echo "  GitHub        : v${VERSION} Release"
  ok "Nowen Video v${VERSION} 四渠道发布完成"
}

cd "$REPO_ROOT"

if [ "$#" -gt 0 ]; then
  parse_args "$@"
  if [ "$PUBLISH_FPK" = "1" ] && [ -z "$VERSION" ]; then
    die "带参数并发布 FPK 时请显式提供 -v X.Y.Z；或使用 --no-fpk"
  fi
  run_product_release
  exit 0
fi

# -------------------- zero-argument interactive wizard --------------------
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
LATEST="$(latest_stable_version)"
SUGGEST="$(suggest_next_patch "$LATEST")"

echo
printf '%s\n' "${C_BOLD}Nowen Video 一键产品发版${C_RESET}"
echo "默认目标：Docker + Android + 飞牛 fnOS + GitHub Release"
echo
read -r -p "版本号 [${SUGGEST}]: " VERSION
VERSION="${VERSION:-$SUGGEST}"; VERSION="${VERSION#v}"
validate_version "$VERSION" || die "版本格式不正确: $VERSION"

echo
echo "1) Docker + Android + 飞牛 fnOS + GitHub Release  [默认]"
echo "2) 上述全部 + Windows Desktop"
echo "3) 仅 Docker Server"
read -r -p "请选择 [1/2/3，默认 1]: " choice
choice="${choice:-1}"
PASSTHROUGH=(-v "$VERSION" -y)
case "$choice" in
  1) PASSTHROUGH+=(--no-desktop) ;;
  2) ;;
  3) PUBLISH_FPK=0; HAS_ANDROID=0; HAS_GIT_TAG=0; PASSTHROUGH+=(--server-only) ;;
  *) die "无效选择: $choice" ;;
esac

echo
printf '%s\n' "版本      : v${VERSION}"
printf '%s\n' "源码      : main @ $(git rev-parse --short=12 HEAD)"
printf '%s\n' "Docker    : linux/amd64 + linux/arm64"
[ "$HAS_ANDROID" = "1" ] && printf '%s\n' "Android   : APK + AAB（production signing）"
[ "$PUBLISH_FPK" = "1" ] && printf '%s\n' "飞牛      : nowen-video-${VERSION}.fpk"
[ "$HAS_GIT_TAG" = "1" ] && printf '%s\n' "GitHub    : v${VERSION}（全资产验证后自动公开）"
echo
read -r -p "确认开始？[Y/n] " answer
case "${answer:-y}" in y|Y|yes|YES|Yes) ;; *) die "已取消" ;; esac
run_product_release
