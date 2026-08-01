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

## Migration sequence

1. Runtime HLS consumes the persistent Probe record.
2. Scanner writes media summaries by applying the same Probe record.
3. Playback Planner consumes audio/video capabilities from Probe.
4. Preprocess and Startup Stream use the same record and source fingerprint.
5. The legacy FFprobe parser in `scanner.go` is deleted after all callers move.
