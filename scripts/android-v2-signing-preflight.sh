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

usage() {
  cat <<'USAGE'
Usage:
  scripts/android-v2-signing-preflight.sh --keystore PATH --alias ALIAS [options]
  scripts/android-v2-signing-preflight.sh --self-test

Required environment variables for a real keystore:
  ANDROID_V2_KEYSTORE_PASSWORD
  ANDROID_V2_KEY_PASSWORD

Options:
  --version VERSION                 Android versionName (default: 0.1.0-rc.1)
  --keystore PATH                   V2 release keystore path
  --alias ALIAS                     Private-key alias (or ANDROID_V2_KEY_ALIAS)
  --expected-fingerprint SHA256     Expected signing certificate SHA-256
  --repository OWNER/REPO           GitHub repository (default: cropflre/nowen-video)
  --report PATH                     Write a non-sensitive JSON preflight report
  --set-github-secrets              Configure all five Android V2 Actions secrets with gh
  --skip-git-checks                 Skip main/clean/remote/tag checks (for CI)
  --self-test                       Generate a temporary keystore and test the validator
  -h, --help                        Show this help

The script never prints or writes keystore passwords, key passwords, or keystore Base64.
USAGE
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

normalize_fingerprint() {
  local value
  value="$(printf '%s' "$1" | tr -d '[:space:]:' | tr '[:upper:]' '[:lower:]')"
  if [[ ! "$value" =~ ^[0-9a-f]{64}$ ]]; then
    printf 'error: certificate SHA-256 must contain exactly 64 hexadecimal characters\n' >&2
    return 1
  fi
  printf '%s\n' "$value"
}

resolve_certificate_fingerprint() {
  local keystore="$1"
  local alias="$2"
  local output
  output="$(
    keytool -exportcert -rfc \
      -keystore "$keystore" \
      -alias "$alias" \
      -storepass:env ANDROID_V2_KEYSTORE_PASSWORD 2>/dev/null \
      | openssl x509 -noout -fingerprint -sha256
  )"
  normalize_fingerprint "${output#*=}"
}

validate_private_key_password() {
  local keystore="$1"
  local alias="$2"
  local temp_dir
  temp_dir="$(mktemp -d)"

  printf 'android-v2-signing-preflight\n' > "$temp_dir/payload.txt"
  jar --create --file "$temp_dir/preflight.jar" -C "$temp_dir" payload.txt >/dev/null
  jarsigner \
    -keystore "$keystore" \
    -storepass:env ANDROID_V2_KEYSTORE_PASSWORD \
    -keypass:env ANDROID_V2_KEY_PASSWORD \
    "$temp_dir/preflight.jar" "$alias" >/dev/null
  jarsigner -verify "$temp_dir/preflight.jar" >/dev/null
  rm -rf "$temp_dir"
}

validate_keystore() {
  local keystore="$1"
  local alias="$2"
  [[ -f "$keystore" ]] || fail "keystore not found: $keystore"
  [[ -r "$keystore" ]] || fail "keystore is not readable: $keystore"
  [[ -n "${ANDROID_V2_KEYSTORE_PASSWORD:-}" ]] || fail "ANDROID_V2_KEYSTORE_PASSWORD is required"
  [[ -n "${ANDROID_V2_KEY_PASSWORD:-}" ]] || fail "ANDROID_V2_KEY_PASSWORD is required"
  [[ -n "$alias" ]] || fail "key alias is required"

  keytool -list \
    -keystore "$keystore" \
    -alias "$alias" \
    -storepass:env ANDROID_V2_KEYSTORE_PASSWORD >/dev/null
  validate_private_key_password "$keystore" "$alias"
  resolve_certificate_fingerprint "$keystore" "$alias"
}

run_self_test() {
  require_command keytool
  require_command jarsigner
  require_command jar
  require_command openssl

  local temp_dir keystore fingerprint
  temp_dir="$(mktemp -d)"
  keystore="$temp_dir/android-v2-test.jks"
  export ANDROID_V2_KEYSTORE_PASSWORD='android-ci-store'
  export ANDROID_V2_KEY_PASSWORD='android-ci-key'

  keytool -genkeypair -noprompt \
    -keystore "$keystore" \
    -storepass "$ANDROID_V2_KEYSTORE_PASSWORD" \
    -keypass "$ANDROID_V2_KEY_PASSWORD" \
    -alias android-v2-test \
    -keyalg RSA \
    -keysize 2048 \
    -validity 2 \
    -dname 'CN=Android V2 Signing Preflight, OU=CI, O=Nowen, L=CI, S=CI, C=US' >/dev/null 2>&1

  fingerprint="$(validate_keystore "$keystore" android-v2-test)"
  [[ "$fingerprint" =~ ^[0-9a-f]{64}$ ]] || fail "self-test produced an invalid fingerprint"

  if normalize_fingerprint 'not-a-fingerprint' >/dev/null 2>&1; then
    fail "invalid fingerprint must be rejected"
  fi

  rm -rf "$temp_dir"
  printf 'Android V2 signing preflight self-test passed\n'
}

check_git_state() {
  require_command git
  git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail "not inside a Git worktree"

  local branch source_commit remote_commit tag
  branch="$(git -C "$ROOT_DIR" branch --show-current)"
  [[ "$branch" == "main" ]] || fail "release preflight must run on main; current branch: ${branch:-detached}"
  [[ -z "$(git -C "$ROOT_DIR" status --porcelain)" ]] || fail "working tree is not clean"

  git -C "$ROOT_DIR" fetch --quiet origin main
  source_commit="$(git -C "$ROOT_DIR" rev-parse HEAD)"
  remote_commit="$(git -C "$ROOT_DIR" rev-parse origin/main)"
  [[ "$source_commit" == "$remote_commit" ]] || fail "HEAD does not match origin/main"

  tag="android-v2-v${VERSION_NAME}"
  if git -C "$ROOT_DIR" show-ref --verify --quiet "refs/tags/$tag"; then
    fail "local tag already exists: $tag"
  fi
  if git -C "$ROOT_DIR" ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null 2>&1; then
    fail "remote tag already exists: $tag"
  fi
}

configure_github_secrets() {
  local keystore="$1"
  local alias="$2"
  local fingerprint="$3"
  require_command gh
  require_command base64

  gh auth status >/dev/null
  gh repo view "$REPOSITORY" >/dev/null

  base64 < "$keystore" | tr -d '\r\n' | gh secret set ANDROID_V2_KEYSTORE_BASE64 --repo "$REPOSITORY"
  printf '%s' "$ANDROID_V2_KEYSTORE_PASSWORD" | gh secret set ANDROID_V2_KEYSTORE_PASSWORD --repo "$REPOSITORY"
  printf '%s' "$alias" | gh secret set ANDROID_V2_KEY_ALIAS --repo "$REPOSITORY"
  printf '%s' "$ANDROID_V2_KEY_PASSWORD" | gh secret set ANDROID_V2_KEY_PASSWORD --repo "$REPOSITORY"
  printf '%s' "$fingerprint" | gh secret set ANDROID_V2_CERTIFICATE_SHA256 --repo "$REPOSITORY"

  local names required
  names="$(gh secret list --repo "$REPOSITORY" --json name --jq '.[].name')"
  for required in \
    ANDROID_V2_KEYSTORE_BASE64 \
    ANDROID_V2_KEYSTORE_PASSWORD \
    ANDROID_V2_KEY_ALIAS \
    ANDROID_V2_KEY_PASSWORD \
    ANDROID_V2_CERTIFICATE_SHA256; do
    grep -Fxq "$required" <<< "$names" || fail "GitHub secret was not found after configuration: $required"
  done
  printf 'Configured and verified five Android V2 GitHub Actions secrets for %s\n' "$REPOSITORY"
}

write_report() {
  local output="$1"
  local fingerprint="$2"
  local version_code="$3"
  local source_commit="$4"
  local branch="$5"

  mkdir -p "$(dirname "$output")"
  python3 - "$output" "$VERSION_NAME" "$version_code" "$REPOSITORY" "$source_commit" "$branch" "$fingerprint" "$KEY_ALIAS" <<'PY'
import datetime
import json
import pathlib
import sys

output, version_name, version_code, repository, commit, branch, fingerprint, alias = sys.argv[1:]
payload = {
    "schema_version": 1,
    "product": "Nowen Video Android V2",
    "checked_at_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "repository": repository,
    "source": {"commit": commit, "branch": branch},
    "version": {"name": version_name, "code": int(version_code)},
    "signing": {"key_alias": alias, "certificate_sha256": fingerprint},
    "sensitive_values_included": False,
}
path = pathlib.Path(output)
path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
print(f"Wrote non-sensitive preflight report: {path}")
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
[[ -n "$KEY_ALIAS" ]] || fail "--alias or ANDROID_V2_KEY_ALIAS is required"

VERSION_CODE="$(bash "$ROOT_DIR/scripts/android-v2-version.sh" "$VERSION_NAME")"
if [[ "$SKIP_GIT_CHECKS" != true ]]; then
  check_git_state
fi

FINGERPRINT="$(validate_keystore "$KEYSTORE_PATH" "$KEY_ALIAS")"
if [[ -n "$EXPECTED_FINGERPRINT" ]]; then
  EXPECTED_FINGERPRINT="$(normalize_fingerprint "$EXPECTED_FINGERPRINT")"
  [[ "$FINGERPRINT" == "$EXPECTED_FINGERPRINT" ]] || fail "keystore certificate SHA-256 does not match the expected fingerprint"
fi

SOURCE_COMMIT="${GITHUB_SHA:-}"
SOURCE_BRANCH="${GITHUB_REF_NAME:-}"
if [[ -z "$SOURCE_COMMIT" ]] && command -v git >/dev/null 2>&1 && git -C "$ROOT_DIR" rev-parse HEAD >/dev/null 2>&1; then
  SOURCE_COMMIT="$(git -C "$ROOT_DIR" rev-parse HEAD)"
  SOURCE_BRANCH="$(git -C "$ROOT_DIR" branch --show-current)"
fi
SOURCE_COMMIT="${SOURCE_COMMIT:-unknown}"
SOURCE_BRANCH="${SOURCE_BRANCH:-unknown}"

printf 'Android V2 signing preflight passed\n'
printf 'versionName=%s\n' "$VERSION_NAME"
printf 'versionCode=%s\n' "$VERSION_CODE"
printf 'certificateSha256=%s\n' "$FINGERPRINT"
printf 'sourceCommit=%s\n' "$SOURCE_COMMIT"

if [[ -n "$REPORT_PATH" ]]; then
  write_report "$REPORT_PATH" "$FINGERPRINT" "$VERSION_CODE" "$SOURCE_COMMIT" "$SOURCE_BRANCH"
fi

if [[ "$SET_GITHUB_SECRETS" == true ]]; then
  configure_github_secrets "$KEYSTORE_PATH" "$KEY_ALIAS" "$FINGERPRINT"
fi
