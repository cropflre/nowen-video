#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPOSITORY="cropflre/nowen-video"
OUTPUT_PATH="${HOME}/keys/nowen-video-android-production.jks"
KEY_ALIAS="nowen-video"
VALIDITY_DAYS=10000
SET_GITHUB_SECRETS=false
FORCE=false

usage() {
  cat <<'USAGE'
Usage:
  scripts/android-signing-bootstrap.sh [options]

Generate the one long-lived production signing key for the official Nowen Video
Android app. This key is independent from the retired Android V1 key.

Options:
  --output PATH                  Production keystore path
  --alias ALIAS                  Key alias (default: nowen-video)
  --repository OWNER/REPO        GitHub repository
  --set-github-secrets           Write the four Android signing Actions Secrets
  --force                        Replace an existing output keystore and backup env
  -h, --help                     Show this help

The script writes a sibling .env backup containing the generated passwords.
Neither the keystore nor the .env file may be committed to Git.
USAGE
}

fail() { printf 'error: %s\n' "$*" >&2; exit 1; }
require_command() { command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"; }

while (($# > 0)); do
  case "$1" in
    --output) OUTPUT_PATH="${2:-}"; shift 2 ;;
    --alias) KEY_ALIAS="${2:-}"; shift 2 ;;
    --repository) REPOSITORY="${2:-}"; shift 2 ;;
    --set-github-secrets) SET_GITHUB_SECRETS=true; shift ;;
    --force) FORCE=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown argument: $1" ;;
  esac
done

require_command keytool
require_command openssl
require_command base64
require_command python3

[[ -n "$OUTPUT_PATH" ]] || fail "--output must not be empty"
[[ -n "$KEY_ALIAS" ]] || fail "--alias must not be empty"

OUTPUT_PATH="$(python3 - "$OUTPUT_PATH" <<'PY'
import os, sys
print(os.path.abspath(os.path.expanduser(sys.argv[1])))
PY
)"
BACKUP_ENV="${OUTPUT_PATH%.*}.secrets.env"
FINGERPRINT_FILE="${OUTPUT_PATH%.*}.certificate-sha256.txt"

if [[ "$FORCE" != true ]]; then
  [[ ! -e "$OUTPUT_PATH" ]] || fail "keystore already exists: $OUTPUT_PATH"
  [[ ! -e "$BACKUP_ENV" ]] || fail "secret backup already exists: $BACKUP_ENV"
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"
umask 077
[[ "$FORCE" == true ]] && rm -f "$OUTPUT_PATH" "$BACKUP_ENV" "$FINGERPRINT_FILE"

ANDROID_KEYSTORE_PASSWORD="$(openssl rand -base64 36 | tr -d '\r\n')"
ANDROID_KEY_PASSWORD="$(openssl rand -base64 36 | tr -d '\r\n')"
export ANDROID_KEYSTORE_PASSWORD ANDROID_KEY_PASSWORD

keytool -genkeypair -noprompt \
  -storetype JKS \
  -keystore "$OUTPUT_PATH" \
  -storepass "$ANDROID_KEYSTORE_PASSWORD" \
  -keypass "$ANDROID_KEY_PASSWORD" \
  -alias "$KEY_ALIAS" \
  -keyalg RSA \
  -keysize 4096 \
  -sigalg SHA256withRSA \
  -validity "$VALIDITY_DAYS" \
  -dname 'CN=Nowen Video Android, OU=Release, O=Nowen, L=Shenzhen, ST=Guangdong, C=CN'

FINGERPRINT="$(keytool -exportcert -rfc \
  -keystore "$OUTPUT_PATH" \
  -alias "$KEY_ALIAS" \
  -storepass "$ANDROID_KEYSTORE_PASSWORD" 2>/dev/null | \
  openssl x509 -noout -fingerprint -sha256 | \
  sed 's/^.*=//' | tr -d ':[:space:]' | tr '[:upper:]' '[:lower:]')"
[[ "$FINGERPRINT" =~ ^[0-9a-f]{64}$ ]] || fail "unable to resolve certificate SHA-256"

cat > "$BACKUP_ENV" <<EOF
ANDROID_KEYSTORE_PASSWORD=$ANDROID_KEYSTORE_PASSWORD
ANDROID_KEY_ALIAS=$KEY_ALIAS
ANDROID_KEY_PASSWORD=$ANDROID_KEY_PASSWORD
EOF
printf '%s\n' "$FINGERPRINT" > "$FINGERPRINT_FILE"
chmod 600 "$OUTPUT_PATH" "$BACKUP_ENV" "$FINGERPRINT_FILE"

ANDROID_KEY_ALIAS="$KEY_ALIAS" \
  bash "$ROOT_DIR/scripts/android-signing-preflight.sh" \
    --version 1.2.9 \
    --keystore "$OUTPUT_PATH" \
    --alias "$KEY_ALIAS" \
    --repository "$REPOSITORY" \
    --expected-fingerprint "$FINGERPRINT" \
    --skip-git-checks

if [[ "$SET_GITHUB_SECRETS" == true ]]; then
  require_command gh
  gh auth status >/dev/null
  gh repo view "$REPOSITORY" >/dev/null
  base64 < "$OUTPUT_PATH" | tr -d '\r\n' | gh secret set ANDROID_KEYSTORE_BASE64 --repo "$REPOSITORY"
  printf '%s' "$ANDROID_KEYSTORE_PASSWORD" | gh secret set ANDROID_KEYSTORE_PASSWORD --repo "$REPOSITORY"
  printf '%s' "$KEY_ALIAS" | gh secret set ANDROID_KEY_ALIAS --repo "$REPOSITORY"
  printf '%s' "$ANDROID_KEY_PASSWORD" | gh secret set ANDROID_KEY_PASSWORD --repo "$REPOSITORY"
fi

printf '\nAndroid production signing bootstrap complete.\n'
printf 'Keystore: %s\n' "$OUTPUT_PATH"
printf 'Secret backup: %s\n' "$BACKUP_ENV"
printf 'Certificate SHA-256: %s\n' "$FINGERPRINT"
if [[ "$SET_GITHUB_SECRETS" == true ]]; then
  printf 'GitHub Actions secrets: configured for %s\n' "$REPOSITORY"
else
  printf 'GitHub Actions secrets: not changed (pass --set-github-secrets to configure them).\n'
fi
printf '\nIMPORTANT: keep at least two offline backups of the .jks and .env files.\n'
printf 'Losing this key prevents future signed upgrades of com.nowen.video.\n'
