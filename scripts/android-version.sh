#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/android-version.sh VERSION_NAME
  scripts/android-version.sh --self-test

Supported versions:
  MAJOR.MINOR.PATCH
  MAJOR.MINOR.PATCH-alpha.N
  MAJOR.MINOR.PATCH-beta.N
  MAJOR.MINOR.PATCH-rc.N
EOF
}

android_version_code() {
  local version_name="$1" major minor patch prerelease channel sequence base offset

  if [[ ! "$version_name" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)(-(alpha|beta|rc)\.([0-9]+))?$ ]]; then
    echo "Unsupported Android version: $version_name" >&2
    return 1
  fi

  major=$((10#${BASH_REMATCH[1]}))
  minor=$((10#${BASH_REMATCH[2]}))
  patch=$((10#${BASH_REMATCH[3]}))
  prerelease="${BASH_REMATCH[4]:-}"
  channel="${BASH_REMATCH[5]:-}"
  sequence="${BASH_REMATCH[6]:-}"

  if (( major > 199 || minor > 99 || patch > 99 )); then
    echo "Android version components exceed limits: major<=199, minor<=99, patch<=99" >&2
    return 1
  fi

  base=$((major * 10000000 + minor * 100000 + patch * 1000))

  if [[ -z "$prerelease" ]]; then
    offset=999
  else
    sequence=$((10#$sequence))
    if (( sequence < 1 || sequence > 99 )); then
      echo "Android prerelease sequence must be between 1 and 99" >&2
      return 1
    fi
    case "$channel" in
      alpha) offset=$((100 + sequence)) ;;
      beta)  offset=$((300 + sequence)) ;;
      rc)    offset=$((500 + sequence)) ;;
      *) return 1 ;;
    esac
  fi

  local version_code=$((base + offset))
  if (( version_code < 1 || version_code > 2100000000 )); then
    echo "Android versionCode is outside Android's supported range: $version_code" >&2
    return 1
  fi
  printf '%s\n' "$version_code"
}

self_test() {
  local failures=0 actual
  assert_code() {
    local version="$1" expected="$2"
    actual="$(android_version_code "$version")" || { failures=$((failures + 1)); return; }
    [[ "$actual" == "$expected" ]] || {
      echo "FAIL: $version resolved to $actual, expected $expected" >&2
      failures=$((failures + 1))
    }
  }
  assert_invalid() {
    android_version_code "$1" >/dev/null 2>&1 && failures=$((failures + 1)) || true
  }

  assert_code "0.1.0-alpha.1" "100101"
  assert_code "0.1.0-beta.1" "100301"
  assert_code "0.1.0-rc.1" "100501"
  assert_code "0.1.0" "100999"
  assert_code "1.2.3-rc.4" "10203504"
  assert_code "1.2.3" "10203999"
  assert_code "1.2.9" "10209999"
  assert_code "199.99.99" "1999999999"
  assert_invalid "1.2"
  assert_invalid "1.2.3-preview.1"
  assert_invalid "1.2.3-rc.0"
  assert_invalid "1.2.3-rc.100"
  assert_invalid "200.0.0"

  (( failures == 0 )) || { echo "$failures Android version self-tests failed" >&2; return 1; }
  echo "Android version self-test passed"
}

case "${1:-}" in
  --self-test) self_test ;;
  -h|--help|'') usage ;;
  *) android_version_code "$1" ;;
esac
