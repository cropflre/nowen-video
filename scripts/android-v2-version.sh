#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/android-v2-version.sh VERSION_NAME
  scripts/android-v2-version.sh --self-test

Supported versions:
  MAJOR.MINOR.PATCH
  MAJOR.MINOR.PATCH-alpha.N
  MAJOR.MINOR.PATCH-beta.N
  MAJOR.MINOR.PATCH-rc.N

The resulting Android versionCode is monotonic within one semantic version:
  alpha < beta < rc < stable
EOF
}

android_version_code() {
  local version_name="$1"
  local core prerelease major minor patch channel sequence base offset

  if [[ ! "$version_name" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)(-(alpha|beta|rc)\.([0-9]+))?$ ]]; then
    echo "Unsupported Android V2 version: $version_name" >&2
    return 1
  fi

  major="${BASH_REMATCH[1]}"
  minor="${BASH_REMATCH[2]}"
  patch="${BASH_REMATCH[3]}"
  prerelease="${BASH_REMATCH[4]:-}"
  channel="${BASH_REMATCH[5]:-}"
  sequence="${BASH_REMATCH[6]:-}"

  # Force base-10 parsing so values such as 08 are not treated as octal.
  major=$((10#$major))
  minor=$((10#$minor))
  patch=$((10#$patch))

  if (( major > 199 || minor > 99 || patch > 99 )); then
    echo "Android V2 version components exceed limits: major<=199, minor<=99, patch<=99" >&2
    return 1
  fi

  base=$((major * 10000000 + minor * 100000 + patch * 1000))

  if [[ -z "$prerelease" ]]; then
    offset=999
  else
    sequence=$((10#$sequence))
    if (( sequence < 1 || sequence > 99 )); then
      echo "Android V2 prerelease sequence must be between 1 and 99" >&2
      return 1
    fi
    case "$channel" in
      alpha) offset=$((100 + sequence)) ;;
      beta)  offset=$((300 + sequence)) ;;
      rc)    offset=$((500 + sequence)) ;;
      *)
        echo "Unsupported Android V2 prerelease channel: $channel" >&2
        return 1
        ;;
    esac
  fi

  local version_code=$((base + offset))
  if (( version_code < 1 || version_code > 2100000000 )); then
    echo "Android V2 versionCode is outside Android's supported range: $version_code" >&2
    return 1
  fi

  printf '%s\n' "$version_code"
}

self_test() {
  local failures=0 actual

  assert_code() {
    local version="$1" expected="$2"
    actual="$(android_version_code "$version")" || {
      echo "FAIL: $version should resolve to $expected" >&2
      failures=$((failures + 1))
      return
    }
    if [[ "$actual" != "$expected" ]]; then
      echo "FAIL: $version resolved to $actual, expected $expected" >&2
      failures=$((failures + 1))
    fi
  }

  assert_invalid() {
    local version="$1"
    if android_version_code "$version" >/dev/null 2>&1; then
      echo "FAIL: $version should be rejected" >&2
      failures=$((failures + 1))
    fi
  }

  assert_code "0.1.0-alpha.1" "100101"
  assert_code "0.1.0-beta.1" "100301"
  assert_code "0.1.0-rc.1" "100501"
  assert_code "0.1.0" "100999"
  assert_code "1.2.3-rc.4" "10203504"
  assert_code "1.2.3" "10203999"
  assert_code "199.99.99" "1999999999"

  assert_invalid "1.2"
  assert_invalid "1.2.3-preview.1"
  assert_invalid "1.2.3-rc.0"
  assert_invalid "1.2.3-rc.100"
  assert_invalid "200.0.0"
  assert_invalid "1.100.0"
  assert_invalid "1.0.100"

  if (( failures > 0 )); then
    echo "Android V2 version policy self-test failed: $failures case(s)" >&2
    return 1
  fi
  echo "Android V2 version policy self-test passed."
}

case "${1:-}" in
  --self-test)
    self_test
    ;;
  -h|--help|"")
    usage
    [[ -n "${1:-}" ]] || exit 1
    ;;
  *)
    android_version_code "$1"
    ;;
esac
