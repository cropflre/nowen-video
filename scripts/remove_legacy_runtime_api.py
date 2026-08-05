#!/usr/bin/env python3
from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def load(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def save(path: str, content: str) -> None:
    (ROOT / path).write_text(content, encoding="utf-8")


def replace_once(path: str, old: str, new: str) -> None:
    content = load(path)
    count = content.count(old)
    if count != 1:
        raise RuntimeError(f"{path}: expected one marker, found {count}: {old[:100]!r}")
    save(path, content.replace(old, new, 1))


def remove_lines_containing(path: str, tokens: list[str]) -> None:
    content = load(path)
    lines = content.splitlines(keepends=True)
    removed = {token: 0 for token in tokens}
    kept: list[str] = []
    for line in lines:
        matched = [token for token in tokens if token in line]
        if matched:
            for token in matched:
                removed[token] += 1
            continue
        kept.append(line)
    missing = [token for token, count in removed.items() if count == 0]
    if missing:
        raise RuntimeError(f"{path}: route markers not found: {missing}")
    save(path, "".join(kept))


def remove_regex_once(path: str, pattern: str, replacement: str = "") -> None:
    content = load(path)
    updated, count = re.subn(pattern, replacement, content, count=1, flags=re.DOTALL)
    if count != 1:
        raise RuntimeError(f"{path}: regex marker not found: {pattern[:120]!r}")
    save(path, updated)


def replace_between(path: str, start_marker: str, end_marker: str, replacement: str) -> None:
    content = load(path)
    start = content.find(start_marker)
    if start < 0:
        raise RuntimeError(f"{path}: start marker not found: {start_marker!r}")
    end = content.find(end_marker, start)
    if end < 0:
        raise RuntimeError(f"{path}: end marker not found: {end_marker!r}")
    save(path, content[:start] + replacement + content[end:])


def remove_balanced_js_expression(path: str, marker: str) -> None:
    content = load(path)
    marker_at = content.find(marker)
    if marker_at < 0:
        raise RuntimeError(f"{path}: JSX marker not found: {marker!r}")
    start = content.rfind("{", 0, marker_at + 1)
    if start < 0:
        raise RuntimeError(f"{path}: opening brace not found for {marker!r}")

    depth = 0
    quote: str | None = None
    escaped = False
    end = -1
    i = start
    while i < len(content):
        ch = content[i]
        if quote is not None:
            if escaped:
                escaped = False
            elif ch == "\\":
                escaped = True
            elif ch == quote:
                quote = None
            i += 1
            continue
        if ch in ("'", '"', "`"):
            quote = ch
        elif ch == "{":
            depth += 1
        elif ch == "}":
            depth -= 1
            if depth == 0:
                end = i + 1
                break
        i += 1
    if end < 0:
        raise RuntimeError(f"{path}: closing brace not found for {marker!r}")

    line_start = content.rfind("\n", 0, start) + 1
    line_end = end
    if line_end < len(content) and content[line_end] == "\n":
        line_end += 1
    save(path, content[:line_start] + content[line_end:])


stream_routes = [
    '"/stream/:id/master.m3u8"',
    '"/stream/:id/:quality/:segment"',
    '"/stream/:id/playback"',
    '"/stream/:id/bandwidth"',
    '"/stream/:id/throttle"',
    '"/audio-track/:id/:trackIdx"',
    '"/audio-track/:id/:trackIdx/:seg"',
]
admin_routes = [
    '"/transcode/status"',
    '"/transcode/throttle"',
    '"/transcode/:taskId/cancel"',
    '"/transcode-tasks"',
    '"/transcode-tasks/statistics"',
    '"/transcode-tasks/batch-cancel"',
    '"/transcode-tasks/batch-delete"',
    '"/transcode-tasks/batch-retry"',
    '"/transcode-tasks/batch-submit"',
    '"/transcode-tasks/:id/cancel"',
    '"/transcode-tasks/:id/retry"',
    '"/transcode-tasks/:id"',
]

remove_lines_containing("cmd/server-lite/routes_core.go", stream_routes)
remove_lines_containing("cmd/server/main.go", stream_routes + admin_routes)
remove_lines_containing("cmd/server-lite/routes_admin.go", admin_routes)

remove_regex_once(
    "internal/handler/stream.go",
    r'\n// RetiredRuntimeHLS is the authenticated compatibility tombstone.*?\nfunc \(h \*StreamHandler\) RetiredRuntimeHLS\(c \*gin\.Context\) \{.*?\n\}\n',
    "\n",
)
remove_regex_once(
    "internal/handler/admin.go",
    r'\n// RetiredRuntimeTranscode is the sole compatibility handler.*?\nfunc \(h \*AdminHandler\) RetiredRuntimeTranscode\(c \*gin\.Context\) \{.*?\n\}\n',
    "\n",
)

replace_once("web/src/api/admin.ts", "  TranscodeJob,\n", "")
replace_between(
    "web/src/api/admin.ts",
    "  transcodeStatus: () =>\n",
    "  // TMDb 配置管理\n",
    "  // TMDb 配置管理\n",
)

replace_once(
    "web/src/pages/PreprocessPage.tsx",
    "import TranscodeJobsPanel from '@/components/preprocess/TranscodeJobsPanel'\n",
    "",
)
replace_between(
    "web/src/pages/PreprocessPage.tsx",
    "  // 主区域 Tab 切换：",
    "  const [stats, setStats]",
    "  // 主区域 Tab 切换：'submit' = 影视文件列表；'tasks' = 持久预处理任务。\n"
    "  const [mainTab, setMainTab] = useState<'submit' | 'tasks'>(() => {\n"
    "    const hash = typeof window !== 'undefined' ? window.location.hash.replace('#', '') : ''\n"
    "    return hash === 'submit' ? 'submit' : 'tasks'\n"
    "  })\n",
)
replace_once(
    "web/src/pages/PreprocessPage.tsx",
    "      {/* 主区域 Tab 切换：选源提交 / 处理进度 / 转码任务 —— 避免多块同时铺开造成页面过长 */}",
    "      {/* 主区域 Tab 切换：选源提交 / 持久预处理进度 */}",
)
replace_once(
    "web/src/pages/PreprocessPage.tsx",
    "          { key: 'transcode', label: '转码任务', count: 0 },\n",
    "",
)
remove_balanced_js_expression("web/src/pages/PreprocessPage.tsx", "mainTab === 'transcode' && (")

panel = ROOT / "web/src/components/preprocess/TranscodeJobsPanel.tsx"
if not panel.exists():
    raise RuntimeError("retired TranscodeJobsPanel is already missing")
panel.unlink()

replace_once(
    "web/src/api/stream.ts",
    "import { getPlaybackSessionRuntime } from '@/playback/sessionRuntime'\n",
    "",
)
replace_between(
    "web/src/api/stream.ts",
    "  getMasterUrl: (mediaId: string) => {\n",
    "  getPlaybackFallbackUrl:",
    "  getMasterUrl: (mediaId: string) => {\n"
    "    const plan = playbackPlanCache.get(mediaId)\n"
    "    const plannedHls = plan?.method === 'transcode' || plan?.method === 'startup_stream'\n"
    "    if (!plannedHls || !plan.url) {\n"
    "      throw new Error('播放计划未提供可用的 HLS 地址')\n"
    "    }\n"
    "    return withToken(plan.url)\n"
    "  },\n\n"
    "  getPlaybackFallbackUrl:",
)
replace_between(
    "web/src/api/stream.ts",
    "  reportPlayback: (mediaId: string, position: number) => {\n",
    "  checkSTRM:",
    "  checkSTRM:",
)

replace_once(
    "web/src/pages/PlayerPage.tsx",
    "  // src 选择：WebCodecs 模式使用 remux URL（拿到 fMP4 流）或 direct URL（MP4 源）\n",
    "  const requiresSessionTranscode =\n"
    "    mode === 'hls' &&\n"
    "    !isPreprocessed &&\n"
    "    (remuxFailed || streamApi.requiresPlaybackSession(id))\n\n"
    "  // src 只能来自直放、Remux、持久预处理或服务端播放计划。\n"
    "  // Session 模式由 SessionVideoPlayer 创建 Generation 后提供实际 playlist。\n",
)
replace_once(
    "web/src/pages/PlayerPage.tsx",
    "          : streamApi.getMasterUrl(id)\n\n  const requiresSessionTranscode =\n    mode === 'hls' &&\n    !isPreprocessed &&\n    (remuxFailed || streamApi.requiresPlaybackSession(id))\n",
    "          : requiresSessionTranscode\n            ? ''\n            : streamApi.getMasterUrl(id)\n",
)
replace_once("web/src/pages/PlayerPage.tsx", "          fallbackSrc={src}\n", "")

replace_once("web/src/components/SessionVideoPlayer.tsx", "  fallbackSrc: string\n", "")
replace_once("web/src/components/SessionVideoPlayer.tsx", "  fallbackSrc,\n", "")
replace_once("web/src/components/SessionVideoPlayer.tsx", "    fallbackSource: fallbackSrc,\n", "")

replace_once("web/src/hooks/usePlaybackSessionSource.ts", "  fallbackSource: string\n", "")
replace_once("web/src/hooks/usePlaybackSessionSource.ts", "  fallbackSource,\n", "")
replace_once(
    "web/src/hooks/usePlaybackSessionSource.ts",
    "  const [source, setSource] = useState(enabled ? '' : fallbackSource)\n",
    "  const [source, setSource] = useState('')\n",
)
replace_once("web/src/hooks/usePlaybackSessionSource.ts", "      setSource(fallbackSource)\n", "      setSource('')\n")
replace_once(
    "web/src/hooks/usePlaybackSessionSource.ts",
    "  }, [enabled, mediaId, fallbackSource, startPosition, close])\n",
    "  }, [enabled, mediaId, startPosition, close])\n",
)

video_path = "web/src/components/VideoPlayer.tsx"
video = load(video_path)
video = video.replace("  // 实时转码/带宽状态（Settings 面板展示）\n", "  // HLS 本地播放指标（不再调用媒体级 Runtime 遥测接口）\n", 1)
for call in (
    "        streamApi.reportPlayback(mediaId, pos).catch(() => {})\n",
    "      streamApi.reportPlayback(mediaId, actualTime).catch(() => {})\n",
    "    streamApi.reportPlayback(mediaId, targetTime).catch(() => {})\n",
):
    if video.count(call) != 1:
        raise RuntimeError(f"{video_path}: playback telemetry call marker mismatch: {call!r}")
    video = video.replace(call, "", 1)
save(video_path, video)

replace_between(
    video_path,
    "        // hls.js 内部维护了 EWMA",
    "        hls.on(Hls.Events.ERROR,",
    "        // hls.js 的 EWMA 仅用于本地诊断展示，不再上报媒体级 Runtime。\n"
    "        const updateBandwidthEstimate = () => {\n"
    "          const bw = Math.round((hls as unknown as { bandwidthEstimate: number }).bandwidthEstimate || 0)\n"
    "          if (bw > 0) setBandwidthEstimate(bw)\n"
    "        }\n"
    "        hls.on(Hls.Events.FRAG_LOADED, updateBandwidthEstimate)\n"
    "        hls.on(Hls.Events.ERROR,",
)
remove_regex_once(
    video_path,
    r'\n  const \[throttleStatus, setThrottleStatus\] = useState<\{.*?\n  \} \| null>\(null\)\n',
    "\n",
)
remove_regex_once(
    video_path,
    r'\n  // 节流状态轮询：仅 HLS 转码模式下.*?\n  \}, \[mediaId, mode\]\)\n',
    "\n",
)
remove_balanced_js_expression(video_path, "throttleStatus && (")

replace_once(
    "web/src/components/WebCodecsPlayerShell.tsx",
    " *   - 进度上报与 VideoPlayer 行为对齐（reportPlayback / updateProgress）\n",
    " *   - 观看历史上报与 VideoPlayer 行为对齐\n",
)
replace_once("web/src/components/WebCodecsPlayerShell.tsx", "import { streamApi } from '@/api/stream'\n", "")
replace_once(
    "web/src/components/WebCodecsPlayerShell.tsx",
    "  // 进度上报（3 秒一次，每 5 次写一次观看历史）\n",
    "  // 观看历史每 15 秒写入一次；WebCodecs 不依赖媒体级 Runtime 遥测。\n",
)
replace_once(
    "web/src/components/WebCodecsPlayerShell.tsx",
    "      streamApi.reportPlayback(mediaId, currentTime).catch(() => {})\n",
    "",
)

contract = r'''package main

import (
    "os"
    "strings"
    "testing"
)

func TestLegacyRuntimeCompatibilitySurfaceIsPhysicallyRemoved(t *testing.T) {
    sources := []string{
        "main.go",
        "../server-lite/routes_core.go",
        "../server-lite/routes_admin.go",
        "../../internal/handler/stream.go",
        "../../internal/handler/admin.go",
        "../../web/src/api/stream.ts",
        "../../web/src/api/admin.ts",
        "../../web/src/pages/PlayerPage.tsx",
        "../../web/src/pages/PreprocessPage.tsx",
        "../../web/src/components/VideoPlayer.tsx",
        "../../web/src/components/SessionVideoPlayer.tsx",
        "../../web/src/components/WebCodecsPlayerShell.tsx",
        "../../web/src/hooks/usePlaybackSessionSource.ts",
    }
    forbidden := []string{
        "RetiredRuntimeHLS",
        "RetiredRuntimeTranscode",
        "/stream/:id/master.m3u8",
        "/stream/:id/:quality/:segment",
        "/stream/:id/playback",
        "/stream/:id/bandwidth",
        "/stream/:id/throttle",
        "/audio-track/:id/:trackIdx",
        "/admin/transcode/status",
        "/admin/transcode-tasks",
        "reportPlayback(",
        "reportBandwidth(",
        "getThrottleStatus(",
        "fallbackSrc",
        "fallbackSource",
        "TranscodeJobsPanel",
    }
    for _, path := range sources {
        content, err := os.ReadFile(path)
        if err != nil {
            t.Fatalf("read %s: %v", path, err)
        }
        for _, marker := range forbidden {
            if strings.Contains(string(content), marker) {
                t.Fatalf("%s still exposes retired Runtime marker %q", path, marker)
            }
        }
    }
    if _, err := os.Stat("../../web/src/components/preprocess/TranscodeJobsPanel.tsx"); !os.IsNotExist(err) {
        t.Fatalf("retired TranscodeJobsPanel still exists: %v", err)
    }
}

func TestModernPlaybackAndHistoryContractsRemainRegistered(t *testing.T) {
    checks := map[string][]string{
        "main.go": {
            `api.GET("/stream/:id/direct"`,
            `api.GET("/stream/:id/remux"`,
            `playbackRuntime.Register(api, guardByMediaID)`,
            `admin.GET("/runtime-history"`,
            `api.GET("/preprocess/media/:id/master.m3u8"`,
        },
        "../server-lite/routes_core.go": {
            `api.GET("/stream/:id/info"`,
            `api.GET("/stream/:id/plan"`,
            `api.GET("/stream/:id/direct"`,
            `api.GET("/stream/:id/remux"`,
            `api.POST("/playback/sessions"`,
            `api.GET("/playback/sessions/:sessionID/generations/:generationID/stream.m3u8"`,
        },
        "../server-lite/routes_admin.go": {
            `admin.GET("/runtime-history"`,
            `admin.GET("/runtime-history/summary"`,
            `admin.GET("/runtime-history/jobs/:id"`,
        },
    }
    for path, markers := range checks {
        content, err := os.ReadFile(path)
        if err != nil {
            t.Fatalf("read %s: %v", path, err)
        }
        for _, marker := range markers {
            if !strings.Contains(string(content), marker) {
                t.Fatalf("%s lost required modern contract %q", path, marker)
            }
        }
    }
}
'''
save("cmd/server/legacy_runtime_api_removed_test.go", contract)

doc = """# Legacy Runtime API Removal\n\nThe media-keyed persistent Runtime executor and Artifact playback source were\nremoved in earlier phases. This phase removes their remaining HTTP and Web UI\ncompatibility surface. The retired URLs are no longer protocol endpoints and\ntherefore resolve through the normal router 404 path instead of returning a\ncustom 410 tombstone.\n\n## Removed surfaces\n\n- Media-keyed HLS master, quality playlist, segment and audio-track routes.\n- Media-keyed playback position, bandwidth and throttle telemetry routes.\n- Legacy Runtime transcode status, task list and mutation routes.\n- The obsolete Web transcode task panel and its batch submit/actions.\n- Client-side guessed `/stream/:id/master.m3u8` fallback URLs.\n\n## Preserved contracts\n\n- Direct play and managed Remux.\n- STRM proxy and health endpoints.\n- Ephemeral Playback Sessions and Generation playlist/segment reads.\n- Durable administrator preprocessing and its playback endpoints.\n- Read-only Runtime History APIs over retained Job, Attempt, Artifact and legacy\n  task metadata.\n\n## Client rule\n\nAn incompatible source must obtain a server playback plan and create an\nephemeral Playback Session. A client must never infer a media-keyed Runtime URL.\nPlayback Session heartbeat is the only Runtime liveness/position protocol;\nnormal watch history remains a separate user-progress write.\n\n## Data and rollback\n\nNo historical table or row is deleted. Existing `transcode_jobs`,\n`transcode_attempts`, `transcode_artifacts` and `transcode_tasks` data remains\navailable through Runtime History and remains compatible with database rollback.\n"""
save("docs/LEGACY_RUNTIME_API_REMOVAL.md", doc)

# Production source must not retain an obsolete URL or handler symbol.
production_roots = [ROOT / "cmd", ROOT / "internal", ROOT / "web" / "src"]
forbidden_source = [
    "RetiredRuntimeHLS",
    "RetiredRuntimeTranscode",
    "/admin/transcode-tasks",
    "/admin/transcode/status",
    "/stream/${mediaId}/playback",
    "/stream/${mediaId}/bandwidth",
    "/stream/${mediaId}/throttle",
    "/api/stream/${mediaId}/master.m3u8",
    "TranscodeJobsPanel",
    "fallbackSrc",
    "fallbackSource",
]
for root in production_roots:
    for path in root.rglob("*"):
        if not path.is_file() or path.suffix not in {".go", ".ts", ".tsx"} or path.name.endswith("_test.go"):
            continue
        content = path.read_text(encoding="utf-8")
        for marker in forbidden_source:
            if marker in content:
                raise RuntimeError(f"{path.relative_to(ROOT)} still contains retired marker {marker!r}")

print("legacy Runtime API compatibility surface removed")
