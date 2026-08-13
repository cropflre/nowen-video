#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION_NAME="1.2.9"
KEYSTORE_PATH=""
KEY_ALIAS="${ANDROID_KEY_ALIAS:-nowen-video}"
REPOSITORY="cropflre/nowen-video"
REPORT_PATH=""
EXPECTED_FINGERPRINT=""
SET_GITHUB_SECRETS=false
SKIP_GIT_CHECKS=false
SELF_TEST=false

usage() {
  cat <<'USAGE'
Usage:
  scripts/android-signing-preflight.sh --keystore PATH --alias ALIAS [options]
  scripts/android-signing-preflight.sh --self-test

Environment:
  ANDROID_KEYSTORE_PASSWORD
  ANDROID_KEY_PASSWORD

Options:
  --version VERSION
  --keystore PATH
  --alias ALIAS
  --expected-fingerprint SHA256
  --repository OWNER/REPO
  --report PATH
  --set-github-secrets
  --skip-git-checks
  --self-test
USAGE
}

fail() { printf 'error: %s\n' "$*" >&2; exit 1; }
require_command() { command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"; }

normalize_fingerprint() {
  local value
  value="$(printf '%s' "$1" | tr -d '[:space:]:' | tr '[:upper:]' '[:lower:]')"
  [[ "$value" =~ ^[0-9a-f]{64}$ ]] || return 1
  printf '%s\n' "$value"
}

resolve_certificate_fingerprint() {
  local output
  output="$(keytool -exportcert -rfc \
    -keystore "$KEYSTORE_PATH" \
    -alias "$KEY_ALIAS" \
    -storepass:env ANDROID_KEYSTORE_PASSWORD 2>/dev/null | \
    openssl x509 -noout -fingerprint -sha256)"
  normalize_fingerprint "${output#*=}" || fail "unable to resolve certificate SHA-256"
}

validate_private_key_password() {
  local temp_dir status=0
  temp_dir="$(mktemp -d)"
  printf 'nowen-video-android-signing-preflight\n' > "$temp_dir/payload.txt"
  jar --create --file "$temp_dir/preflight.jar" -C "$temp_dir" payload.txt >/dev/null || status=$?
  if (( status == 0 )); then
    jarsigner \
      -keystore "$KEYSTORE_PATH" \
      -storepass:env ANDROID_KEYSTORE_PASSWORD \
      -keypass:env ANDROID_KEY_PASSWORD \
      "$temp_dir/preflight.jar" "$KEY_ALIAS" >/dev/null || status=$?
  fi
  if (( status == 0 )); then
    jarsigner -verify "$temp_dir/preflight.jar" >/dev/null || status=$?
  fi
  rm -rf "$temp_dir"
  return "$status"
}

validate_keystore() {
  [[ -f "$KEYSTORE_PATH" && -r "$KEYSTORE_PATH" ]] || fail "keystore not found or unreadable: $KEYSTORE_PATH"
  [[ -n "${ANDROID_KEYSTORE_PASSWORD:-}" ]] || fail "ANDROID_KEYSTORE_PASSWORD is required"
  [[ -n "${ANDROID_KEY_PASSWORD:-}" ]] || fail "ANDROID_KEY_PASSWORD is required"
  [[ -n "$KEY_ALIAS" ]] || fail "key alias is required"
  keytool -list \
    -keystore "$KEYSTORE_PATH" \
    -alias "$KEY_ALIAS" \
    -storepass:env ANDROID_KEYSTORE_PASSWORD >/dev/null
  validate_private_key_password || fail "private-key password or alias is invalid"
  resolve_certificate_fingerprint
}

check_git_state() {
  require_command git
  local branch head remote tag
  branch="$(git -C "$ROOT_DIR" branch --show-current)"
  [[ "$branch" == "main" ]] || fail "release preflight must run on main"
  [[ -z "$(git -C "$ROOT_DIR" status --porcelain)" ]] || fail "working tree is not clean"
  git -C "$ROOT_DIR" fetch --quiet origin main
  head="$(git -C "$ROOT_DIR" rev-parse HEAD)"
  remote="$(git -C "$ROOT_DIR" rev-parse origin/main)"
  [[ "$head" == "$remote" ]] || fail "HEAD does not match origin/main"
  tag="v${VERSION_NAME}"
  git -C "$ROOT_DIR" ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null 2>&1 && \
    fail "remote tag already exists: $tag" || true
}

write_report() {
  local fingerprint="$1" version_code="$2" source_commit="$3" source_branch="$4"
  [[ -n "$REPORT_PATH" ]] || return 0
  mkdir -p "$(dirname "$REPORT_PATH")"
  python3 - "$REPORT_PATH" "$VERSION_NAME" "$version_code" "$REPOSITORY" "$source_commit" "$source_branch" "$fingerprint" "$KEY_ALIAS" <<'PY'
import datetime, json, pathlib, sys
output, version_name, version_code, repository, commit, branch, fingerprint, alias = sys.argv[1:]
payload = {
    "schema_version": 1,
    "product": "Nowen Video Android",
    "checked_at_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "repository": repository,
    "source": {"commit": commit, "branch": branch},
    "version": {"name": version_name, "code": int(version_code)},
    "signing": {"key_alias": alias, "certificate_sha256": fingerprint},
    "sensitive_values_included": False,
}
pathlib.Path(output).write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
PY
}

configure_github_secrets() {
  require_command gh
  require_command base64
  gh auth status >/dev/null
  gh repo view "$REPOSITORY" >/dev/null
  base64 < "$KEYSTORE_PATH" | tr -d '\r\n' | gh secret set ANDROID_KEYSTORE_BASE64 --repo "$REPOSITORY"
  printf '%s' "$ANDROID_KEYSTORE_PASSWORD" | gh secret set ANDROID_KEYSTORE_PASSWORD --repo "$REPOSITORY"
  printf '%s' "$KEY_ALIAS" | gh secret set ANDROID_KEY_ALIAS --repo "$REPOSITORY"
  printf '%s' "$ANDROID_KEY_PASSWORD" | gh secret set ANDROID_KEY_PASSWORD --repo "$REPOSITORY"
  echo "Configured Android production signing secrets."
}

run_self_test() {
  require_command keytool
  require_command jarsigner
  require_command jar
  require_command openssl
  local temp_dir fingerprint
  temp_dir="$(mktemp -d)"
  KEYSTORE_PATH="$temp_dir/android-test.jks"
  KEY_ALIAS="android-test"
  export ANDROID_KEYSTORE_PASSWORD='android-ci-password'
  export ANDROID_KEY_PASSWORD='android-ci-password'
  keytool -genkeypair -noprompt \
    -keystore "$KEYSTORE_PATH" \
    -storepass "$ANDROID_KEYSTORE_PASSWORD" \
    -keypass "$ANDROID_KEY_PASSWORD" \
    -alias "$KEY_ALIAS" \
    -keyalg RSA -keysize 2048 -validity 2 \
    -dname 'CN=Android Signing Preflight, OU=CI, O=Nowen, L=CI, S=CI, C=US' >/dev/null 2>&1
  fingerprint="$(validate_keystore)"
  [[ "$fingerprint" =~ ^[0-9a-f]{64}$ ]] || fail "self-test produced invalid fingerprint"
  export ANDROID_KEY_PASSWORD='wrong'
  validate_private_key_password >/dev/null 2>&1 && fail "wrong key password must be rejected"
  rm -rf "$temp_dir"
  echo "Android signing preflight self-test passed"
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

if [[ "$SELF_TEST" == true ]]; then
  run_self_test
  exit 0
fi

require_command keytool
require_command jarsigner
require_command jar
require_command openssl
require_command python3
[[ -n "$KEYSTORE_PATH" ]] || fail "--keystore is required"
[[ -n "$KEY_ALIAS" ]] || fail "--alias is required"
VERSION_CODE="$(bash "$ROOT_DIR/scripts/android-version.sh" "$VERSION_NAME")"
[[ "$SKIP_GIT_CHECKS" == true ]] || check_git_state
FINGERPRINT="$(validate_keystore)"
if [[ -n "$EXPECTED_FINGERPRINT" ]]; then
  EXPECTED_FINGERPRINT="$(normalize_fingerprint "$EXPECTED_FINGERPRINT")" || fail "invalid expected fingerprint"
  [[ "$FINGERPRINT" == "$EXPECTED_FINGERPRINT" ]] || fail "keystore certificate SHA-256 does not match expected fingerprint"
fi
SOURCE_COMMIT="${GITHUB_SHA:-unknown}"
SOURCE_BRANCH="${GITHUB_REF_NAME:-unknown}"
if [[ "$SOURCE_COMMIT" == unknown ]] && command -v git >/dev/null 2>&1; then
  SOURCE_COMMIT="$(git -C "$ROOT_DIR" rev-parse HEAD 2>/dev/null || echo unknown)"
  SOURCE_BRANCH="$(git -C "$ROOT_DIR" branch --show-current 2>/dev/null || echo unknown)"
fi
printf 'Android signing preflight passed\nversionName=%s\nversionCode=%s\ncertificateSha256=%s\n' \
  "$VERSION_NAME" "$VERSION_CODE" "$FINGERPRINT"
write_report "$FINGERPRINT" "$VERSION_CODE" "$SOURCE_COMMIT" "$SOURCE_BRANCH"
[[ "$SET_GITHUB_SECRETS" == true ]] && configure_github_secrets || true
