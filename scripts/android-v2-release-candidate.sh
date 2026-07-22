#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION_NAME="0.1.0-rc.1"
KEYSTORE_PATH=""
KEY_ALIAS="${ANDROID_V2_KEY_ALIAS:-nowen-video-android-v2}"
REPOSITORY="cropflre/nowen-video"
OUTPUT_DIR="$ROOT_DIR/dist/android-v2/candidate"
VERIFY_DIR=""
EXPECTED_COMMIT=""
EXPECTED_FINGERPRINT="${ANDROID_V2_CERTIFICATE_SHA256:-}"
GENERATE_KEYSTORE=false
CONFIGURE_SECRETS=false
DISPATCH=false
CREATE_TAG=false
SKIP_SIGNATURE_VERIFICATION=false
SELF_TEST=false

usage() {
  cat <<'USAGE'
Usage:
  scripts/android-v2-release-candidate.sh --keystore PATH [options]
  scripts/android-v2-release-candidate.sh --verify-dir PATH [options]
  scripts/android-v2-release-candidate.sh --self-test

Safe defaults:
  - Runs local signing preflight only.
  - Does not trigger GitHub Actions unless --dispatch is supplied.
  - Does not create or push a tag unless --create-tag is supplied.
  - Never publishes a GitHub Release.

Options:
  --version VERSION                 versionName (default: 0.1.0-rc.1)
  --keystore PATH                   Long-term Android V2 release keystore
  --alias ALIAS                     Private-key alias
  --repository OWNER/REPO           GitHub repository
  --output-dir PATH                 Candidate artifact directory
  --generate-keystore               Generate the keystore when PATH does not exist
  --configure-secrets               Configure the five Android V2 Actions secrets
  --dispatch                        Trigger release-android-v2, wait, download and verify
  --verify-dir PATH                 Verify an already downloaded candidate directory
  --expected-commit SHA             Required source commit for verify-only mode
  --expected-fingerprint SHA256     Required signing certificate SHA-256
  --create-tag                      After successful verification, create and push the RC tag
  --self-test                       Run a completely offline fixture test
  -h, --help                        Show this help

Passwords are read from ANDROID_V2_KEYSTORE_PASSWORD and
ANDROID_V2_KEY_PASSWORD. When running interactively, missing values are read
with hidden terminal input. Passwords and keystore Base64 are never printed.
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
  [[ "$value" =~ ^[0-9a-f]{64}$ ]] || return 1
  printf '%s\n' "$value"
}

read_secret() {
  local variable_name="$1"
  local prompt="$2"
  local confirm="${3:-false}"
  local value confirmation

  value="${!variable_name:-}"
  if [[ -n "$value" ]]; then
    return 0
  fi
  [[ -t 0 ]] || fail "$variable_name is required in non-interactive mode"

  read -r -s -p "$prompt: " value
  printf '\n'
  [[ -n "$value" ]] || fail "$variable_name must not be empty"
  if [[ "$confirm" == true ]]; then
    read -r -s -p "Confirm $prompt: " confirmation
    printf '\n'
    [[ "$value" == "$confirmation" ]] || fail "$prompt values do not match"
  fi
  printf -v "$variable_name" '%s' "$value"
  export "$variable_name"
}

ensure_passwords() {
  local confirm=false
  [[ "$GENERATE_KEYSTORE" == true && ! -f "$KEYSTORE_PATH" ]] && confirm=true
  read_secret ANDROID_V2_KEYSTORE_PASSWORD "Keystore password" "$confirm"
  read_secret ANDROID_V2_KEY_PASSWORD "Key password" "$confirm"
}

generate_keystore_if_requested() {
  [[ "$GENERATE_KEYSTORE" == true ]] || return 0
  [[ -n "$KEYSTORE_PATH" ]] || fail "--keystore is required with --generate-keystore"
  if [[ -e "$KEYSTORE_PATH" ]]; then
    printf 'Reusing existing keystore: %s\n' "$KEYSTORE_PATH"
    return 0
  fi

  require_command keytool
  umask 077
  mkdir -p "$(dirname "$KEYSTORE_PATH")"
  keytool -genkeypair -noprompt \
    -keystore "$KEYSTORE_PATH" \
    -storepass:env ANDROID_V2_KEYSTORE_PASSWORD \
    -keypass:env ANDROID_V2_KEY_PASSWORD \
    -alias "$KEY_ALIAS" \
    -keyalg RSA \
    -keysize 4096 \
    -validity 10000 \
    -dname 'CN=Nowen Video Android V2, OU=Release, O=Nowen, L=Shenzhen, ST=Guangdong, C=CN'
  chmod 600 "$KEYSTORE_PATH"
  printf 'Generated long-term Android V2 keystore: %s\n' "$KEYSTORE_PATH"
  printf 'Back up this file to an encrypted offline or separate-disk location before release.\n'
}

resolve_git_source() {
  require_command git
  git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail "not inside a Git worktree"
  git -C "$ROOT_DIR" fetch --quiet origin main
  local branch head remote
  branch="$(git -C "$ROOT_DIR" branch --show-current)"
  head="$(git -C "$ROOT_DIR" rev-parse HEAD)"
  remote="$(git -C "$ROOT_DIR" rev-parse origin/main)"
  [[ "$branch" == main ]] || fail "release helper must run on main; current branch: ${branch:-detached}"
  [[ -z "$(git -C "$ROOT_DIR" status --porcelain)" ]] || fail "working tree is not clean"
  [[ "$head" == "$remote" ]] || fail "HEAD does not match origin/main"
  printf '%s\n' "$head"
}

run_signing_preflight() {
  local report_path="$1"
  local args=(
    --version "$VERSION_NAME"
    --keystore "$KEYSTORE_PATH"
    --alias "$KEY_ALIAS"
    --repository "$REPOSITORY"
    --report "$report_path"
  )
  [[ -n "$EXPECTED_FINGERPRINT" ]] && args+=(--expected-fingerprint "$EXPECTED_FINGERPRINT")
  [[ "$CONFIGURE_SECRETS" == true ]] && args+=(--set-github-secrets)
  bash "$ROOT_DIR/scripts/android-v2-signing-preflight.sh" "${args[@]}"

  EXPECTED_FINGERPRINT="$(python3 - "$report_path" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as handle:
    print(json.load(handle)["signing"]["certificate_sha256"])
PY
)"
}

list_existing_run_ids() {
  gh run list \
    --repo "$REPOSITORY" \
    --workflow release-android-v2.yml \
    --event workflow_dispatch \
    --limit 100 \
    --json databaseId \
    --jq '.[].databaseId'
}

find_dispatched_run() {
  local source_commit="$1"
  local before_file="$2"
  local attempt json run_id
  for attempt in $(seq 1 90); do
    json="$(gh run list \
      --repo "$REPOSITORY" \
      --workflow release-android-v2.yml \
      --event workflow_dispatch \
      --limit 30 \
      --json databaseId,headSha,event,status,createdAt)"
    run_id="$(python3 - "$source_commit" "$before_file" <<'PY' <<<"$json"
import json, pathlib, sys
source_commit, before_path = sys.argv[1:]
before = set(pathlib.Path(before_path).read_text(encoding="utf-8").split())
for run in json.load(sys.stdin):
    if str(run["databaseId"]) not in before and run.get("headSha") == source_commit:
        print(run["databaseId"])
        break
PY
)"
    if [[ -n "$run_id" ]]; then
      printf '%s\n' "$run_id"
      return 0
    fi
    sleep 2
  done
  return 1
}

prepare_output_dir() {
  local directory="$1"
  if [[ -d "$directory" && -n "$(find "$directory" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
    fail "output directory is not empty: $directory"
  fi
  mkdir -p "$directory"
}

dispatch_download_candidate() {
  local source_commit="$1"
  require_command gh
  require_command python3
  gh auth status >/dev/null
  gh repo view "$REPOSITORY" >/dev/null

  prepare_output_dir "$OUTPUT_DIR"
  local before_file run_id
  before_file="$(mktemp)"
  list_existing_run_ids > "$before_file"

  printf 'Dispatching release-android-v2 for %s at %s\n' "$VERSION_NAME" "$source_commit"
  gh workflow run release-android-v2.yml \
    --repo "$REPOSITORY" \
    --ref main \
    -f "version_name=$VERSION_NAME"

  run_id="$(find_dispatched_run "$source_commit" "$before_file")" || {
    rm -f "$before_file"
    fail "unable to identify the newly dispatched workflow run"
  }
  rm -f "$before_file"
  printf 'Watching workflow run %s\n' "$run_id"
  gh run watch "$run_id" --repo "$REPOSITORY" --exit-status

  gh run download "$run_id" \
    --repo "$REPOSITORY" \
    --name nowen-video-android-v2-release \
    --dir "$OUTPUT_DIR"
  printf '%s\n' "$run_id" > "$OUTPUT_DIR/workflow-run-id.txt"
  printf 'Downloaded candidate artifact to %s\n' "$OUTPUT_DIR"
}

find_apksigner() {
  local candidate
  if command -v apksigner >/dev/null 2>&1; then
    command -v apksigner
    return 0
  fi
  for root in "${ANDROID_HOME:-}" "${ANDROID_SDK_ROOT:-}"; do
    [[ -n "$root" && -d "$root/build-tools" ]] || continue
    candidate="$(find "$root/build-tools" -type f -name apksigner | sort -V | tail -n 1)"
    [[ -x "$candidate" ]] && { printf '%s\n' "$candidate"; return 0; }
  done
  return 1
}

verify_candidate_dir() {
  local directory="$1"
  local source_commit="$2"
  local expected_fingerprint="$3"
  local version_code
  version_code="$(bash "$ROOT_DIR/scripts/android-v2-version.sh" "$VERSION_NAME")"
  expected_fingerprint="$(normalize_fingerprint "$expected_fingerprint")" || fail "invalid expected certificate SHA-256"

  local apk="$directory/nowen-video-android-v2-${VERSION_NAME}.apk"
  local aab="$directory/nowen-video-android-v2-${VERSION_NAME}.aab"
  local sums="$directory/SHA256SUMS.txt"
  local manifest="$directory/release-manifest.json"
  local preflight="$directory/signing-preflight.json"
  local notes="$directory/RELEASE_NOTES.md"
  local required
  for required in "$apk" "$aab" "$sums" "$manifest" "$preflight" "$notes"; do
    [[ -f "$required" ]] || fail "required candidate file is missing: $required"
  done

  (cd "$directory" && sha256sum -c SHA256SUMS.txt)
  python3 - "$manifest" "$preflight" "$notes" "$VERSION_NAME" "$version_code" "$source_commit" "$expected_fingerprint" <<'PY'
import json, pathlib, sys
manifest_path, preflight_path, notes_path, version_name, version_code, commit, fingerprint = sys.argv[1:]
manifest = json.loads(pathlib.Path(manifest_path).read_text(encoding="utf-8"))
preflight = json.loads(pathlib.Path(preflight_path).read_text(encoding="utf-8"))
notes = pathlib.Path(notes_path).read_text(encoding="utf-8")
errors = []

def expect(actual, expected, label):
    if actual != expected:
        errors.append(f"{label}: expected {expected!r}, got {actual!r}")

expect(manifest["version"]["name"], version_name, "manifest versionName")
expect(int(manifest["version"]["code"]), int(version_code), "manifest versionCode")
expect(manifest["application"]["id"], "com.nowen.video.v2", "applicationId")
expect(int(manifest["application"]["min_sdk"]), 26, "minSdk")
expect(int(manifest["application"]["target_sdk"]), 35, "targetSdk")
expect(manifest["source"]["commit"], commit, "source commit")
expect(manifest["signing"]["certificate_sha256"].replace(":", "").lower(), fingerprint, "manifest certificate")
expect(preflight["source"]["commit"], commit, "preflight commit")
expect(preflight["version"]["name"], version_name, "preflight versionName")
expect(preflight["signing"]["certificate_sha256"].replace(":", "").lower(), fingerprint, "preflight certificate")
artifact_names = {entry["name"] for entry in manifest["artifacts"]}
expected_names = {
    f"nowen-video-android-v2-{version_name}.apk",
    f"nowen-video-android-v2-{version_name}.aab",
}
expect(artifact_names, expected_names, "manifest artifact names")
for token, label in ((version_name, "version"), (commit, "commit"), (fingerprint, "certificate")):
    if token not in notes:
        errors.append(f"release notes missing {label}: {token}")
if "{{" in notes or "}}" in notes:
    errors.append("release notes contain unresolved template placeholders")
if errors:
    raise SystemExit("Candidate verification failed:\n- " + "\n- ".join(errors))
print("Candidate metadata verification passed")
PY

  if [[ "$SKIP_SIGNATURE_VERIFICATION" != true ]]; then
    local apksigner
    if apksigner="$(find_apksigner)"; then
      "$apksigner" verify --verbose --print-certs "$apk" >/dev/null
      printf 'APK signature verification passed\n'
    else
      printf 'warning: apksigner not found; APK cryptographic verification skipped\n' >&2
    fi
    if command -v jarsigner >/dev/null 2>&1; then
      jarsigner -verify "$aab" >/dev/null
      printf 'AAB signature verification passed\n'
    else
      printf 'warning: jarsigner not found; AAB cryptographic verification skipped\n' >&2
    fi
  fi

  python3 - "$directory/candidate-verification.json" "$VERSION_NAME" "$version_code" "$REPOSITORY" "$source_commit" "$expected_fingerprint" <<'PY'
import datetime, json, pathlib, sys
output, version_name, version_code, repository, commit, fingerprint = sys.argv[1:]
payload = {
    "schema_version": 1,
    "product": "Nowen Video Android V2",
    "verified_at_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "repository": repository,
    "version": {"name": version_name, "code": int(version_code)},
    "source_commit": commit,
    "certificate_sha256": fingerprint,
    "checksums_verified": True,
    "metadata_verified": True,
    "sensitive_values_included": False,
}
pathlib.Path(output).write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
PY
  printf 'Candidate verification report: %s/candidate-verification.json\n' "$directory"
}

create_and_push_tag() {
  local source_commit="$1"
  local tag="android-v2-v${VERSION_NAME}"
  require_command git
  if git -C "$ROOT_DIR" show-ref --verify --quiet "refs/tags/$tag"; then
    fail "local tag already exists: $tag"
  fi
  if git -C "$ROOT_DIR" ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null 2>&1; then
    fail "remote tag already exists: $tag"
  fi
  git -C "$ROOT_DIR" tag "$tag" "$source_commit"
  git -C "$ROOT_DIR" push origin "$tag"
  printf 'Pushed tag %s. The tag workflow will create a draft prerelease.\n' "$tag"
}

run_self_test() {
  require_command python3
  require_command sha256sum
  local temp_dir fixture fingerprint commit
  temp_dir="$(mktemp -d)"
  fixture="$temp_dir/candidate"
  mkdir -p "$fixture"
  fingerprint="$(printf 'ab%.0s' {1..32})"
  commit="$(printf 'c%.0s' {1..40})"
  VERSION_NAME="0.1.0-rc.1"
  local version_code apk_name aab_name
  version_code="$(bash "$ROOT_DIR/scripts/android-v2-version.sh" "$VERSION_NAME")"
  apk_name="nowen-video-android-v2-${VERSION_NAME}.apk"
  aab_name="nowen-video-android-v2-${VERSION_NAME}.aab"
  printf 'fixture-apk' > "$fixture/$apk_name"
  printf 'fixture-aab' > "$fixture/$aab_name"
  (cd "$fixture" && sha256sum "$apk_name" "$aab_name" > SHA256SUMS.txt)
  python3 - "$fixture" "$VERSION_NAME" "$version_code" "$commit" "$fingerprint" "$apk_name" "$aab_name" <<'PY'
import hashlib, json, pathlib, sys
root, version, code, commit, fingerprint, apk_name, aab_name = sys.argv[1:]
root = pathlib.Path(root)
def artifact(name, kind):
    path = root / name
    return {"type": kind, "name": name, "size_bytes": path.stat().st_size, "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}
manifest = {
    "schema_version": 1,
    "product": "Nowen Video Android V2",
    "channel": "rc",
    "version": {"name": version, "code": int(code)},
    "application": {"id": "com.nowen.video.v2", "min_sdk": 26, "target_sdk": 35},
    "source": {"repository": "cropflre/nowen-video", "commit": commit, "ref": "refs/heads/main"},
    "workflow": {"event": "workflow_dispatch", "run_id": "1", "run_attempt": "1"},
    "signing": {"certificate_sha256": fingerprint},
    "artifacts": [artifact(apk_name, "apk"), artifact(aab_name, "aab")],
}
preflight = {
    "schema_version": 1,
    "product": "Nowen Video Android V2",
    "repository": "cropflre/nowen-video",
    "source": {"commit": commit, "branch": "main"},
    "version": {"name": version, "code": int(code)},
    "signing": {"key_alias": "fixture", "certificate_sha256": fingerprint},
    "sensitive_values_included": False,
}
(root / "release-manifest.json").write_text(json.dumps(manifest), encoding="utf-8")
(root / "signing-preflight.json").write_text(json.dumps(preflight), encoding="utf-8")
(root / "RELEASE_NOTES.md").write_text(f"# {version}\nCommit: {commit}\nCertificate: {fingerprint}\n", encoding="utf-8")
PY

  SKIP_SIGNATURE_VERIFICATION=true
  verify_candidate_dir "$fixture" "$commit" "$fingerprint" >/dev/null

  python3 - "$fixture/release-manifest.json" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
data = json.loads(path.read_text(encoding="utf-8"))
data["source"]["commit"] = "d" * 40
path.write_text(json.dumps(data), encoding="utf-8")
PY
  if (verify_candidate_dir "$fixture" "$commit" "$fingerprint" >/dev/null 2>&1); then
    rm -rf "$temp_dir"
    fail "self-test must reject an incorrect manifest commit"
  fi
  rm -rf "$temp_dir"
  printf 'Android V2 release candidate helper self-test passed\n'
}

while (($# > 0)); do
  case "$1" in
    --version) VERSION_NAME="${2:-}"; shift 2 ;;
    --keystore) KEYSTORE_PATH="${2:-}"; shift 2 ;;
    --alias) KEY_ALIAS="${2:-}"; shift 2 ;;
    --repository) REPOSITORY="${2:-}"; shift 2 ;;
    --output-dir) OUTPUT_DIR="${2:-}"; shift 2 ;;
    --generate-keystore) GENERATE_KEYSTORE=true; shift ;;
    --configure-secrets) CONFIGURE_SECRETS=true; shift ;;
    --dispatch) DISPATCH=true; shift ;;
    --verify-dir) VERIFY_DIR="${2:-}"; shift 2 ;;
    --expected-commit) EXPECTED_COMMIT="${2:-}"; shift 2 ;;
    --expected-fingerprint) EXPECTED_FINGERPRINT="${2:-}"; shift 2 ;;
    --create-tag) CREATE_TAG=true; shift ;;
    --self-test) SELF_TEST=true; shift ;;
    --skip-signature-verification) SKIP_SIGNATURE_VERIFICATION=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown argument: $1" ;;
  esac
done

if [[ "$SELF_TEST" == true ]]; then
  run_self_test
  exit 0
fi

require_command python3
require_command sha256sum

if [[ -n "$VERIFY_DIR" ]]; then
  [[ -n "$EXPECTED_COMMIT" ]] || fail "--expected-commit is required with --verify-dir"
  [[ -n "$EXPECTED_FINGERPRINT" ]] || fail "--expected-fingerprint is required with --verify-dir"
  [[ "$CREATE_TAG" != true ]] || fail "--create-tag is only allowed after a dispatched build"
  verify_candidate_dir "$VERIFY_DIR" "$EXPECTED_COMMIT" "$EXPECTED_FINGERPRINT"
  exit 0
fi

[[ -n "$KEYSTORE_PATH" ]] || fail "--keystore is required"
ensure_passwords
generate_keystore_if_requested
[[ -f "$KEYSTORE_PATH" ]] || fail "keystore not found: $KEYSTORE_PATH"

SOURCE_COMMIT="$(resolve_git_source)"
prepare_output_dir "$OUTPUT_DIR"
run_signing_preflight "$OUTPUT_DIR/signing-preflight-local.json"

if [[ "$DISPATCH" == true && "$CONFIGURE_SECRETS" != true ]]; then
  fail "--dispatch requires --configure-secrets to avoid using stale signing Secrets"
fi

if [[ "$DISPATCH" == true ]]; then
  dispatch_download_candidate "$SOURCE_COMMIT"
  verify_candidate_dir "$OUTPUT_DIR" "$SOURCE_COMMIT" "$EXPECTED_FINGERPRINT"
fi

if [[ "$CREATE_TAG" == true ]]; then
  [[ "$DISPATCH" == true ]] || fail "--create-tag requires a successfully dispatched and verified candidate"
  create_and_push_tag "$SOURCE_COMMIT"
fi

printf 'Android V2 RC candidate helper completed safely.\n'
