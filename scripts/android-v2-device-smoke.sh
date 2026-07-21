#!/usr/bin/env bash
set -u

api_level="${1:-unknown}"
log_file="android-v2-device-smoke-api-${api_level}.log"

set +e
./android/gradlew -p clients/android-v2 \
  connectedDebugAndroidTest \
  -Pandroid.testInstrumentationRunnerArguments.class=com.nowen.video.v2.AppLaunchSmokeTest \
  --no-daemon --stacktrace >"$log_file" 2>&1
status=$?
set -e

cat "$log_file"
exit "$status"
