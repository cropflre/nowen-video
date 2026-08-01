# Media Probe Architecture

## Purpose

`media_probe_cache` is the authoritative technical description used by the
transcode planner. `media` keeps user-facing summary fields for compatibility,
but frame rate, colour metadata, pixel depth and audio stream details must come
from the Probe record.

This boundary prevents playback planning, scanning, preprocessing and runtime
transcoding from maintaining different FFprobe parsers.

## Freshness identity

A local source is fresh only when all of the following match:

- media ID
- source path
- source file size
- source modification time in nanoseconds
- Probe parser version

The values are hashed into `source_fingerprint`. A source change creates a cache
miss and atomically replaces the record for the same media ID.

Remote sources that do not expose a stable local file identity require a
resolver-specific fingerprint. `.strm` and `webdav://` are not guessed by the
local Probe service; they continue through their dedicated input resolvers until
those resolvers expose a stable identity and FFprobe-readable URL.

## Concurrency

Concurrent playback requests for the same media and fingerprint share one
FFprobe execution through single-flight. The shared execution has its own
20-second timeout. A caller may stop waiting without cancelling work required by
another caller.

## Scan-completion warmup

Lite and Full subscribe to the existing `scan_completed` lifecycle event through
an in-process observer owned by the WebSocket Hub. The observer does not perform
media work; it only submits the completed library ID to a bounded warmup queue.

The warmup service provides:

- two Probe workers
- a 16-library bounded queue
- per-library deduplication while queued or running
- stable ID-based pagination with 64 media rows per page
- synchronization of legacy `media` technical summary columns only
- queue, run, media and failure counters under `probe_warmup`
- automatic shutdown when the durable transcode scheduler closes

The warmup uses the same persistent Probe service, source fingerprint and
single-flight group as runtime HLS. It generates technical metadata only; it
does not create Startup Stream or full-video transcode artifacts.

## Playback planning

Playback planning performs a non-blocking fresh-cache lookup. It never starts
FFprobe on the request path. When a fresh record exists, Direct Play, zero-copy
Remux and Smart Remux are recomputed from the authoritative video codec and
default audio track. The response also includes a normalized technical snapshot
for diagnostics.

A cache miss keeps the compatibility summary behavior. If HLS is selected, the
claimed transcode Worker performs the cold Probe before constructing FFmpeg
arguments.

## Normalized fields

The first video stream contributes:

- codec
- width and height
- average frame-rate rational, falling back to real frame rate
- pixel format and bit depth
- colour transfer, primaries, matrix and range
- HDR classification

All audio streams contribute codec, channels, layout, sample rate, language,
title and default disposition.

## HDR rules

Codec names never imply HDR. In particular, ordinary SDR HEVC, VP9 and AV1 must
not be tone mapped.

HDR is true only when FFprobe reports at least one of:

- SMPTE ST 2084 / PQ transfer
- ARIB STD-B67 / HLG transfer
- mastering display metadata
- content light metadata
- Dolby Vision / DOVI side data

Software transcoding uses the HDR-to-SDR filter only for confirmed HDR records.
A missing or failed Probe falls back to an SDR-safe aspect-ratio-preserving
scale and pad operation.

## Frame rate and GOP

HLS GOP size is calculated from the normalized source frame rate and target
segment duration:

```
round(source_fps * segment_seconds)
```

The value is clamped to 12–240 frames. Missing or invalid frame rate falls back
to 25 FPS for compatibility. For example, 24000/1001 FPS with two-second
segments produces a 48-frame GOP.

## Failure policy

Probe failure must not make an otherwise playable video unavailable. Runtime
transcoding logs the failure and uses compatibility parameters. Failures,
executions, cache hits and cache misses are exposed in the existing transcode
statistics response under `media_probe`.

A single media failure during scan warmup is counted and logged but does not
abort the rest of the library. Unsupported resolver-specific sources are
counted as skipped.

## Migration sequence

Completed:

1. Runtime HLS consumes the persistent Probe record.
2. Playback Planner consumes a fresh cached Probe without adding request latency.
3. Scan completion warms the Probe cache and synchronizes legacy technical summaries.

Remaining:

1. Preprocess consumes the same Probe record and source fingerprint.
2. Startup Stream generation consumes Probe and the immutable Artifact Store.
3. Resolver-specific source identities cover `.strm`, WebDAV and remote storage.
4. The legacy FFprobe parser in `scanner.go` is deleted after all callers move.
