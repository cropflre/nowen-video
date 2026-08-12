#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION_NAME="0.1.0-rc.1"
KEYSTORE_PATH=""
KEY_ALIAS="${ANDROID_V2_KEY_ALIAS:-}"
REPOSITORY="cropflre/nowen-video"
REPORT_PATH=""
EXPECTED_FINGERPRINT="${ANDROID_V2_CERTIFICATE_SHA256:-}"
SET_GITHUB_SECRETS=false
SKIP_GIT_CHECKS=false
SELF_TEST=false
LEGACY_RELEASE_TAG="v1.2.5"
LEGACY_APK_NAME="nowen-video-android-1.2.5.apk"

usage() {
  cat <<'USAGE'
Usage:
  scripts/android-v2-signing-preflight.sh --keystore PATH --alias ALIAS [options]
  scripts/android-v2-signing-preflight.sh --self-test

Required environment variables for a real keystore:
  ANDROID_V2_KEYSTORE_PASSWORD
  ANDROID_V2_KEY_PASSWORD

Options:
  --version VERSION                 Android versionName
  --keystore PATH                   Historical Android release keystore path
  --alias ALIAS                     Private-key alias (or ANDROID_V2_KEY_ALIAS)
  --expected-fingerprint SHA256     Expected signing certificate SHA-256
  --repository OWNER/REPO           GitHub repository
  --report PATH                     Write a non-sensitive JSON preflight report
  --set-github-secrets              Configure the existing five Android Actions secrets
  --skip-git-checks                 Skip main/clean/remote/tag checks (for CI)
  --self-test                       Generate a temporary key and test validator mechanics
  -h, --help                        Show this help

Production rule:
  com.nowen.video now replaces Android V1. A production key must therefore be
  the historical V1 release signer. When --set-github-secrets is used this
  script verifies the key against the published v1.2.5 APK before writing any
  repository secret.
USAGE
}

fail() { printf 'error: %s\n' "$*" >&2; exit 1; }
require_command() { command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"; }

normalize_fingerprint() {
  local value
  value="$(printf '%s' "$1" | tr -d '[:space:]:' | tr '[:upper:]' '[:lower:]')"
  [[ "$value" =~ ^[0-9a-f]{64}$ ]] || { printf 'error: certificate SHA-256 must contain exactly 64 hexadecimal characters\n' >&2; return 1; }
  printf '%s\n' "$value"
}

resolve_certificate_fingerprint() {
  local keystore="$1" alias="$2" output
  output="$(keytool -exportcert -rfc -keystore "$keystore" -alias "$alias" -storepass:env ANDROID_V2_KEYSTORE_PASSWORD 2>/dev/null | openssl x509 -noout -fingerprint -sha256)"
  normalize_fingerprint "${output#*=}"
}

validate_private_key_password() {
  local keystore="$1" alias="$2" temp_dir status=0
  temp_dir="$(mktemp -d)"
  printf 'android-signing-preflight\n' > "$temp_dir/payload.txt"
  jar --create --file "$temp_dir/preflight.jar" -C "$temp_dir" payload.txt >/dev/null || status=$?
  if (( status == 0 )); then
    jarsigner -keystore "$keystore" -storepass:env ANDROID_V2_KEYSTORE_PASSWORD -keypass:env ANDROID_V2_KEY_PASSWORD "$temp_dir/preflight.jar" "$alias" >/dev/null || status=$?
  fi
  (( status != 0 )) || jarsigner -verify "$temp_dir/preflight.jar" >/dev/null || status=$?
  rm -rf "$temp_dir"
  return "$status"
}

validate_keystore() {
  local keystore="$1" alias="$2"
  [[ -f "$keystore" && -r "$keystore" ]] || fail "keystore not found or unreadable: $keystore"
  [[ -n "${ANDROID_V2_KEYSTORE_PASSWORD:-}" ]] || fail "ANDROID_V2_KEYSTORE_PASSWORD is required"
  [[ -n "${ANDROID_V2_KEY_PASSWORD:-}" ]] || fail "ANDROID_V2_KEY_PASSWORD is required"
  [[ -n "$alias" ]] || fail "key alias is required"
  keytool -list -keystore "$keystore" -alias "$alias" -storepass:env ANDROID_V2_KEYSTORE_PASSWORD >/dev/null
  validate_private_key_password "$keystore" "$alias" || fail "private-key password or alias is invalid"
  resolve_certificate_fingerprint "$keystore" "$alias"
}

find_apksigner() {
  local root candidate
  command -v apksigner >/dev/null 2>&1 && { command -v apksigner; return; }
  for root in "${ANDROID_HOME:-}" "${ANDROID_SDK_ROOT:-}"; do
    [[ -n "$root" && -d "$root/build-tools" ]] || continue
    candidate="$(find "$root/build-tools" -type f -name apksigner | sort -V | tail -n 1)"
    [[ -x "$candidate" ]] && { printf '%s\n' "$candidate"; return; }
  done
  return 1
}

verify_historical_v1_signer() {
  local candidate_fingerprint="$1" temp_dir apksigner legacy_output legacy_fingerprint
  require_command gh
  apksigner="$(find_apksigner)" || fail "apksigner is required to validate the historical V1 signer"
  temp_dir="$(mktemp -d)"
  GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}" gh release download "$LEGACY_RELEASE_TAG" \
    --repo "$REPOSITORY" --pattern "$LEGACY_APK_NAME" --dir "$temp_dir" >/dev/null
  legacy_output="$("$apksigner" verify --verbose --print-certs "$temp_dir/$LEGACY_APK_NAME")"
  rm -rf "$temp_dir"
  legacy_fingerprint="$(printf '%s\n' "$legacy_output" | sed -n 's/.*certificate SHA-256 digest: //Ip' | head -1)"
  legacy_fingerprint="$(normalize_fingerprint "$legacy_fingerprint")" || fail "unable to resolve historical V1 certificate"
  [[ "$candidate_fingerprint" == "$legacy_fingerprint" ]] || fail "keystore is NOT the Android V1 production signer; refusing production takeover"
}

run_self_test() {
  require_command keytool; require_command jarsigner; require_command jar; require_command openssl
  local temp_dir keystore fingerprint
  temp_dir="$(mktemp -d)"; keystore="$temp_dir/android-test.jks"
  export ANDROID_V2_KEYSTORE_PASSWORD='android-ci-password'
  export ANDROID_V2_KEY_PASSWORD='android-ci-password'
  keytool -genkeypair -noprompt -keystore "$keystore" -storepass "$ANDROID_V2_KEYSTORE_PASSWORD" -keypass "$ANDROID_V2_KEY_PASSWORD" -alias android-test -keyalg RSA -keysize 2048 -validity 2 -dname 'CN=Android Signing Preflight, OU=CI, O=Nowen, L=CI, S=CI, C=US' >/dev/null 2>&1
  fingerprint="$(validate_keystore "$keystore" android-test)"
  [[ "$fingerprint" =~ ^[0-9a-f]{64}$ ]] || fail "self-test produced invalid fingerprint"
  export ANDROID_V2_KEY_PASSWORD='wrong'
  validate_private_key_password "$keystore" android-test >/dev/null 2>&1 && fail "wrong key password must be rejected"
  export ANDROID_V2_KEY_PASSWORD='android-ci-password'
  normalize_fingerprint 'not-a-fingerprint' >/dev/null 2>&1 && fail "invalid fingerprint must be rejected"
  rm -rf "$temp_dir"
  printf 'Android signing preflight self-test passed\n'
}

check_git_state() {
  require_command git
  git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail "not inside a Git worktree"
  local branch source_commit remote_commit tag
  branch="$(git -C "$ROOT_DIR" branch --show-current)"
  [[ "$branch" == main ]] || fail "release preflight must run on main; current branch: ${branch:-detached}"
  [[ -z "$(git -C "$ROOT_DIR" status --porcelain)" ]] || fail "working tree is not clean"
  git -C "$ROOT_DIR" fetch --quiet origin main
  source_commit="$(git -C "$ROOT_DIR" rev-parse HEAD)"; remote_commit="$(git -C "$ROOT_DIR" rev-parse origin/main)"
  [[ "$source_commit" == "$remote_commit" ]] || fail "HEAD does not match origin/main"
  tag="v${VERSION_NAME}"
  git -C "$ROOT_DIR" show-ref --verify --quiet "refs/tags/$tag" && fail "local tag already exists: $tag"
  git -C "$ROOT_DIR" ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null 2>&1 && fail "remote tag already exists: $tag"
}

configure_github_secrets() {
  local keystore="$1" alias="$2" fingerprint="$3"
  require_command gh; require_command base64
  verify_historical_v1_signer "$fingerprint"
  gh auth status >/dev/null; gh repo view "$REPOSITORY" >/dev/null
  base64 < "$keystore" | tr -d '\r\n' | gh secret set ANDROID_V2_KEYSTORE_BASE64 --repo "$REPOSITORY"
  printf '%s' "$ANDROID_V2_KEYSTORE_PASSWORD" | gh secret set ANDROID_V2_KEYSTORE_PASSWORD --repo "$REPOSITORY"
  printf '%s' "$alias" | gh secret set ANDROID_V2_KEY_ALIAS --repo "$REPOSITORY"
  printf '%s' "$ANDROID_V2_KEY_PASSWORD" | gh secret set ANDROID_V2_KEY_PASSWORD --repo "$REPOSITORY"
  printf '%s' "$fingerprint" | gh secret set ANDROID_V2_CERTIFICATE_SHA256 --repo "$REPOSITORY"
  printf 'Configured Android production signing secrets using the verified V1 signer.\n'
}

write_report() {
  local output="$1" fingerprint="$2" version_code="$3" source_commit="$4" branch="$5"
  mkdir -p "$(dirname "$output")"
  python3 - "$output" "$VERSION_NAME" "$version_code" "$REPOSITORY" "$source_commit" "$branch" "$fingerprint" "$KEY_ALIAS" <<'PY'
import datetime, json, pathlib, sys
output, version_name, version_code, repository, commit, branch, fingerprint, alias = sys.argv[1:]
payload = {
    "schema_version": 1, "product": "Nowen Video Android",
    "checked_at_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "repository": repository, "source": {"commit": commit, "branch": branch},
    "version": {"name": version_name, "code": int(version_code)},
    "signing": {"key_alias": alias, "certificate_sha256": fingerprint},
    "sensitive_values_included": False,
}
pathlib.Path(output).write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
PY
}

while (($# > 0)); do
  case "$1" in
    --version) VERSION_NAME="${2:-}"; shift 2 ;;
    --keystore) KEYSTORE_PATH="${2:-}"; shift 2 ;;
    --alias) KEY_ALIAS="${2:-}"; shift 2 ;;
    --expected-fingerprint) EXPECTED_FINGERPRINT="${2:-}"; shift 2 ;;
    --repository) REPOSITORY="${2:-}"; shift 2 ;;
    --report) REPORT_PATH="${2:-}"; shift 2 ;;
    --set-github-secrets) SET_GITHUB_SECRETS=true; shift ;;
    --skip-git-checks) SKIP_GIT_CHECKS=true; shift ;;
    --self-test) SELF_TEST=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown argument: $1" ;;
  esac
done

if [[ "$SELF_TEST" == true ]]; then run_self_test; exit 0; fi
require_command keytool; require_command jarsigner; require_command jar; require_command openssl; require_command python3
[[ -n "$KEYSTORE_PATH" ]] || fail "--keystore is required"
[[ -n "$KEY_ALIAS" ]] || fail "--alias or ANDROID_V2_KEY_ALIAS is required"
VERSION_CODE="$(bash "$ROOT_DIR/scripts/android-v2-version.sh" "$VERSION_NAME")"
[[ "$SKIP_GIT_CHECKS" == true ]] || check_git_state
FINGERPRINT="$(validate_keystore "$KEYSTORE_PATH" "$KEY_ALIAS")"
if [[ -n "$EXPECTED_FINGERPRINT" ]]; then
  EXPECTED_FINGERPRINT="$(normalize_fingerprint "$EXPECTED_FINGERPRINT")"
  [[ "$FINGERPRINT" == "$EXPECTED_FINGERPRINT" ]] || fail "keystore certificate SHA-256 does not match expected fingerprint"
fi
SOURCE_COMMIT="${GITHUB_SHA:-}"; SOURCE_BRANCH="${GITHUB_REF_NAME:-}"
if [[ -z "$SOURCE_COMMIT" ]] && command -v git >/dev/null 2>&1 && git -C "$ROOT_DIR" rev-parse HEAD >/dev/null 2>&1; then
  SOURCE_COMMIT="$(git -C "$ROOT_DIR" rev-parse HEAD)"; SOURCE_BRANCH="$(git -C "$ROOT_DIR" branch --show-current)"
fi
SOURCE_COMMIT="${SOURCE_COMMIT:-unknown}"; SOURCE_BRANCH="${SOURCE_BRANCH:-unknown}"
printf 'Android signing preflight passed\nversionName=%s\nversionCode=%s\ncertificateSha256=%s\nsourceCommit=%s\n' "$VERSION_NAME" "$VERSION_CODE" "$FINGERPRINT" "$SOURCE_COMMIT"
[[ -z "$REPORT_PATH" ]] || write_report "$REPORT_PATH" "$FINGERPRINT" "$VERSION_CODE" "$SOURCE_COMMIT" "$SOURCE_BRANCH"
[[ "$SET_GITHUB_SECRETS" != true ]] || configure_github_secrets "$KEYSTORE_PATH" "$KEY_ALIAS" "$FINGERPRINT"
