#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]

stream = root / "web/src/api/stream.ts"
content = stream.read_text(encoding="utf-8")
for old, new in (
    ("  getPlaybackFallbackUrl:  getPlaybackFallbackUrl:", "  getPlaybackFallbackUrl:"),
    ("  checkSTRM:  checkSTRM:", "  checkSTRM:"),
):
    if content.count(old) != 1:
        raise RuntimeError(f"stream API migration boundary mismatch: {old!r}")
    content = content.replace(old, new, 1)
stream.write_text(content, encoding="utf-8")

player = root / "web/src/components/VideoPlayer.tsx"
content = player.read_text(encoding="utf-8")
old = "        hls.on(Hls.Events.ERROR,        hls.on(Hls.Events.ERROR,"
if content.count(old) != 1:
    raise RuntimeError("VideoPlayer HLS error handler migration boundary mismatch")
content = content.replace(old, "        hls.on(Hls.Events.ERROR,", 1)

obsolete_import = "import { streamApi } from '@/api/stream'\n"
if content.count(obsolete_import) != 1:
    raise RuntimeError("VideoPlayer obsolete streamApi import marker mismatch")
content = content.replace(obsolete_import, "", 1)
player.write_text(content, encoding="utf-8")
