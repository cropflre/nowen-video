#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]

worker_test = root / "cmd/server/runtime_worker_removed_test.go"
content = worker_test.read_text(encoding="utf-8")
for line in (
    '\trequireSource(t, "../../internal/handler/admin.go", "RetiredRuntimeTranscode")\n',
    '\trequireSource(t, "../../internal/handler/stream.go", "RetiredRuntimeHLS")\n',
    '\trequireSource(t, "../server-lite/routes_core.go", "handlers.Stream.RetiredRuntimeHLS")\n',
):
    if content.count(line) != 1:
        raise RuntimeError(f"runtime worker test marker mismatch: {line!r}")
    content = content.replace(line, "", 1)
worker_test.write_text(content, encoding="utf-8")

obsolete = root / "internal/middleware/runtime_transcode_retired_test.go"
if not obsolete.exists():
    raise RuntimeError("obsolete Runtime tombstone middleware test is missing")
obsolete.unlink()

contract = root / "cmd/server/legacy_runtime_api_removed_test.go"
content = contract.read_text(encoding="utf-8")
replacements = {
    '"/stream/:id/master.m3u8",': '`GET("/stream/:id/master.m3u8"`,',
    '"/stream/:id/:quality/:segment",': '`GET("/stream/:id/:quality/:segment"`,',
    '"/stream/:id/playback",': '`POST("/stream/:id/playback"`,',
    '"/stream/:id/bandwidth",': '`POST("/stream/:id/bandwidth"`,',
    '"/stream/:id/throttle",': '`GET("/stream/:id/throttle"`,',
    '"/audio-track/:id/:trackIdx",': '`GET("/audio-track/:id/:trackIdx"`,',
}
for old, new in replacements.items():
    if content.count(old) != 1:
        raise RuntimeError(f"legacy Runtime route contract marker mismatch: {old!r}")
    content = content.replace(old, new, 1)
contract.write_text(content, encoding="utf-8")
