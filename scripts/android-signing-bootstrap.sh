#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPOSITORY="cropflre/nowen-video"
KEYSTORE_PATH=""
SECRETS_ENV=""
KEY_ALIAS="nowen-video"
SET_GITHUB_SECRETS=false
EXPECTED_FINGERPRINT="07ac3f214fbb8ac44e85fa1f65610dcbcff8fe04876c417364c3905dfb8b6bcd"

usage() {
  cat <<'USAGE'
Usage:
  scripts/android-signing-bootstrap.sh --keystore PATH [options]

Configure or recover the official Nowen Video Android production signing setup.
This script NEVER generates or rotates the production key. The production
certificate is pinned and any different keystore is rejected.

Options:
  --keystore PATH                 Existing production keystore
  --secrets-env PATH              Optional env backup containing signing passwords
  --alias ALIAS                   Key alias (default: nowen-video)
  --repository OWNER/REPO         GitHub repository
  --set-github-secrets            Write the four Android signing Actions Secrets
  -h, --help                      Show this help

Environment:
  ANDROID_KEYSTORE_PASSWORD
  ANDROID_KEY_PASSWORD

If --secrets-env is omitted, a sibling file named
<keystore-without-extension>.secrets.env is loaded automatically when present.
USAGE
}

fail() { printf 'error: %s\n' "$*" >&2; exit 1; }
require_command() { command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"; }

while (($# > 0)); do
  case "$1" in
    --keystore) KEYSTORE_PATH="${2:-}"; shift 2 ;;
    --secrets-env) SECRETS_ENV="${2:-}"; shift 2 ;;
    --alias) KEY_ALIAS="${2:-}"; shift 2 ;;
    --repository) REPOSITORY="${2:-}"; shift 2 ;;
    --set-github-secrets) SET_GITHUB_SECRETS=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown argument: $1" ;;
  esac
done

require_command keytool
require_command openssl
require_command base64
require_command python3

[[ -n "$KEYSTORE_PATH" ]] || fail "--keystore is required"
[[ -n "$KEY_ALIAS" ]] || fail "--alias must not be empty"

KEYSTORE_PATH="$(python3 - "$KEYSTORE_PATH" <<'PY'
import os, sys
print(os.path.abspath(os.path.expanduser(sys.argv[1])))
PY
)"
[[ -f "$KEYSTORE_PATH" ]] || fail "production keystore not found: $KEYSTORE_PATH"

if [[ -z "$SECRETS_ENV" ]]; then
  candidate="${KEYSTORE_PATH%.*}.secrets.env"
  [[ -f "$candidate" ]] && SECRETS_ENV="$candidate"
fi

if [[ -n "$SECRETS_ENV" ]]; then
  SECRETS_ENV="$(python3 - "$SECRETS_ENV" <<'PY'
import os, sys
print(os.path.abspath(os.path.expanduser(sys.argv[1])))
PY
)"
  [[ -f "$SECRETS_ENV" ]] || fail "secret backup not found: $SECRETS_ENV"
  set -a
  # shellcheck disable=SC1090
  source "$SECRETS_ENV"
  set +a
fi

[[ -n "${ANDROID_KEYSTORE_PASSWORD:-}" ]] || fail "ANDROID_KEYSTORE_PASSWORD is required"
[[ -n "${ANDROID_KEY_PASSWORD:-}" ]] || fail "ANDROID_KEY_PASSWORD is required"

ANDROID_KEY_ALIAS="$KEY_ALIAS" \
  bash "$ROOT_DIR/scripts/android-signing-preflight.sh" \
    --version 1.2.9 \
    --keystore "$KEYSTORE_PATH" \
    --alias "$KEY_ALIAS" \
    --repository "$REPOSITORY" \
    --expected-fingerprint "$EXPECTED_FINGERPRINT" \
    --skip-git-checks

if [[ "$SET_GITHUB_SECRETS" == true ]]; then
  require_command gh
  gh auth status >/dev/null
  gh repo view "$REPOSITORY" >/dev/null
  base64 < "$KEYSTORE_PATH" | tr -d '\r\n' | gh secret set ANDROID_KEYSTORE_BASE64 --repo "$REPOSITORY"
  printf '%s' "$ANDROID_KEYSTORE_PASSWORD" | gh secret set ANDROID_KEYSTORE_PASSWORD --repo "$REPOSITORY"
  printf '%s' "$KEY_ALIAS" | gh secret set ANDROID_KEY_ALIAS --repo "$REPOSITORY"
  printf '%s' "$ANDROID_KEY_PASSWORD" | gh secret set ANDROID_KEY_PASSWORD --repo "$REPOSITORY"
fi

printf '\nAndroid production signing bootstrap passed.\n'
printf 'Keystore: %s\n' "$KEYSTORE_PATH"
printf 'Certificate SHA-256: %s\n' "$EXPECTED_FINGERPRINT"
if [[ "$SET_GITHUB_SECRETS" == true ]]; then
  printf 'GitHub Actions secrets: configured for %s\n' "$REPOSITORY"
else
  printf 'GitHub Actions secrets: not changed (pass --set-github-secrets to configure them).\n'
fi
printf '\nThis repository will reject any different production signing certificate.\n'
