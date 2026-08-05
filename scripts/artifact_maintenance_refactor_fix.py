#!/usr/bin/env python3
from __future__ import annotations

import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def write(path: str, content: str) -> None:
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")


retirement_path = "internal/service/transcode_runtime_retirement.go"
retirement = read(retirement_path)
marker = 'const retiredRuntimePlaybackIntent = "retired_runtime_playback"\n'
replacement = '''const (
\tretiredRuntimePlaybackIntent       = "retired_runtime_playback"
\tstartupStreamArtifactKind          = "startup_hls"
\tstartupContinuationArtifactKind    = "startup_continuation_hls"
)
'''
if marker not in retirement:
    raise RuntimeError("runtime retirement constant marker missing")
write(retirement_path, retirement.replace(marker, replacement))

write(
    "internal/service/media_resolution.go",
    '''package service

import (
\t"strconv"
\t"strings"
)

// parseResolutionHeight normalizes the compact resolution labels stored on a
// Media row. Playback planning owns this helper; it has no relationship with
// the retired persistent Runtime worker.
func parseResolutionHeight(resolution string) int {
\tswitch strings.TrimSpace(resolution) {
\tcase "4K":
\t\treturn 2160
\tcase "2K":
\t\treturn 1440
\tcase "1080p":
\t\treturn 1080
\tcase "720p":
\t\treturn 720
\tcase "480p":
\t\treturn 480
\tcase "360p":
\t\treturn 360
\tdefault:
\t\tvalue := strings.TrimSpace(resolution)
\t\tif strings.HasSuffix(value, "p") {
\t\t\theight, err := strconv.Atoi(strings.TrimSuffix(value, "p"))
\t\t\tif err == nil && height > 0 {
\t\t\t\treturn height
\t\t\t}
\t\t}
\t\treturn 0
\t}
}
''',
)

lifecycle_path = "internal/service/task_lifecycle_events.go"
lifecycle = read(lifecycle_path)
dead_case = '''\tcase EventTranscodeCancelled:
\t\tupdate.Kind = TaskKindTranscode
\t\tupdate.Status = TaskStatusCancelled
'''
if dead_case not in lifecycle:
    raise RuntimeError("retired transcode cancelled event case missing")
write(lifecycle_path, lifecycle.replace(dead_case, ""))
