# Transcode VFR Layer-Isolation Evidence

## Status

This document records the certification-only VFR timing-layer isolation introduced on `refactor/server-lite-v1`.

The phase does not change production FFmpeg arguments, persisted transcode state, playback planning, HLS manifests, or client behavior. Experimental command variants exist only in `internal/transcode/certification`.

Current production safety policy remains:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Why this phase exists

Output Cadence Evidence v1 proved that the existing production-shaped software HLS path retains all 300 decoded frames in a ten-second 30 fps VFR window, but represents 60 of the frame transitions with 11-microsecond PTS intervals while the dominant output cadence remains approximately 41.667 milliseconds.

That evidence constrained the failure mode but could not identify which layer first produced it. The remaining candidates were:

1. output synchronization policy;
2. encoder time-base selection;
3. encoded packet timestamp generation;
4. Matroska or MPEG-TS time-base projection;
5. HLS segmentation;
6. an interaction between those layers.

VFR Layer-Isolation Evidence v1 changes one layer at a time and records both packet cadence and decoded frame content.

## Evidence contracts

### Layer-isolation contract

Schema:

```text
vfr-layer-isolation-evidence-v1
```

The contract binds:

- the immutable VFR source case and 30-to-40-second continuation window;
- FFmpeg and FFprobe toolchain identity;
- the complete Output Cadence Evidence v1 identity;
- the source continuation cadence;
- the exact ordered experiment registry;
- a normalized SHA-256 identity for every command vector;
- complete output PTS cadence for every variant;
- source-to-output frame-count projection;
- decoded-frame fingerprints;
- parent/child sequence identity for copy-only remux experiments;
- fail-closed playback policy.

Every timeline is validated through the full Output Cadence arithmetic validator. The contract recomputes:

```text
positive PTS deltas
+ duplicate PTS
+ non-monotonic PTS
= frame count - 1
```

It also recomputes the ordered histogram, dominant delta, near-zero count, significant cadence buckets, outliers, frame mapping and final cadence classification.

Canonical JSON is identified by SHA-256.

### Matrix report

Schema:

```text
ffmpeg-vfr-layer-isolation-matrix-v1
```

The report requires the complete variant registry in exact order and one valid layer-isolation contract identity.

## Certified source window

The matrix reuses the immutable source case:

```text
source-vfr-24-30-origin-zero-v1
```

Source structure:

```text
0s  - 20s: 24 fps
20s - 40s: 30 fps
```

Only the 30-to-40-second continuation window is isolated. This avoids mixing the earlier 24 fps section into the layer comparison.

Source continuation evidence:

| PTS delta | Count |
|---:|---:|
| 33,333 us | 199 |
| 33,334 us | 100 |

The source window contains:

```text
frames = 300
duplicate_pts = 0
non_monotonic_pts = 0
near_zero_delta = 0
```

## Decoded-frame fingerprint

Every output is decoded with FFmpeg `framemd5` using SHA-256.

The evidence records:

- decoded frame count;
- unique decoded frame count;
- adjacent identical-frame count;
- SHA-256 of the ordered frame-hash sequence;
- first decoded-frame hash;
- last decoded-frame hash.

The sequence digest is used for two different comparisons:

1. encoded variants are compared with the production HLS baseline as diagnostic evidence;
2. copy-only remux variants must match their Matroska parent exactly or the contract fails.

A packet frame-count projection is not treated as content-level duplicate detection. Actual adjacent duplicate frames are derived from decoded-frame hashes.

## Isolation variants

The registry is fixed in this order:

| Variant | Isolated change | Container |
|---|---|---|
| `production-hls-v1` | Production baseline | HLS MPEG-TS |
| `fps-mode-vfr-hls-v1` | `fps_mode=vfr` | HLS MPEG-TS |
| `fps-mode-cfr-hls-v1` | `fps_mode=cfr` | HLS MPEG-TS |
| `encoder-time-base-avtb-hls-v1` | `enc_time_base=1/1000000` | HLS MPEG-TS |
| `encoder-time-base-90k-hls-v1` | `enc_time_base=1/90000` | HLS MPEG-TS |
| `matroska-default-v1` | Replace HLS MPEG-TS container with Matroska | Matroska |
| `matroska-avtb-v1` | Matroska plus `enc_time_base=1/1000000` | Matroska |
| `matroska-remux-mpegts-v1` | Copy Matroska packets to MPEG-TS | MPEG-TS |
| `matroska-remux-hls-v1` | Copy Matroska packets to HLS MPEG-TS | HLS MPEG-TS |

The production Timestamp Plan already uses:

```text
fps_mode = passthrough
```

Therefore `passthrough` is the baseline rather than an experimental candidate.

## Reference environment

```text
Ubuntu 24.04.4
FFmpeg 6.1.1-3ubuntu5
FFprobe 6.1.1-3ubuntu5
software H.264 / AAC
```

## Reference results

### Summary

| Variant | Frames | Packet projection | Duplicate PTS | Near-zero PTS | Adjacent duplicate frames | Unique frames | Result |
|---|---:|---|---:|---:|---:|---:|---|
| Production HLS | 300 | aligned | 0 | 60 | 0 | 300 | changed |
| `fps_mode=vfr` | 241 | drop 59 | 0 | 0 | 0 | 241 | changed |
| `fps_mode=cfr` | 962 | duplicate 662 | 0 | 0 | 645 | 260 | changed |
| HLS + AVTB | 300 | aligned | 0 | 0 | 0 | 300 | preserved |
| HLS + 90 kHz | 300 | aligned | 0 | 0 | 0 | 300 | preserved |
| Matroska auto TB | 300 | aligned | 60 | 0 | 0 | 300 | changed |
| Matroska + AVTB | 300 | aligned | 0 | 0 | 0 | 300 | preserved |
| Matroska to MPEG-TS copy | 300 | aligned | 0 | 60 | 0 | 300 | changed |
| Matroska to HLS copy | 300 | aligned | 0 | 60 | 0 | 300 | changed |

### Production HLS baseline

Histogram:

| PTS delta | Count |
|---:|---:|
| 11 us | 60 |
| 41,656 us | 60 |
| 41,667 us | 179 |

The output has 300 unique decoded frames and no adjacent identical frames. The near-zero PTS intervals are therefore timestamp compression, not content duplication.

### `fps_mode=vfr`

Histogram:

| PTS delta | Count |
|---:|---:|
| 41,667 us | 240 |

Result:

```text
300 source frames -> 241 output frames
projected dropped frames = 59
```

Changing only `fps_mode` to `vfr` removes the near-zero intervals by discarding 59 decoded frames. It is not a valid preservation strategy.

### `fps_mode=cfr`

Histogram:

| PTS delta | Count |
|---:|---:|
| 41,667 us | 961 |

Result:

```text
300 source frames -> 962 output frames
projected duplicate frames = 662
adjacent identical decoded frames = 645
unique decoded frames = 260
```

Changing only `fps_mode` to `cfr` creates severe real decoded-frame duplication. It is not a valid preservation strategy.

### Explicit HLS encoder time bases

Both candidates produce the same cadence evidence:

| PTS delta | Count |
|---:|---:|
| 33,333 us | 299 |

For both `1/1000000` and `1/90000`:

```text
frames = 300
unique decoded frames = 300
adjacent duplicate frames = 0
duplicate PTS = 0
near-zero PTS = 0
sequence matches production baseline = true
cadence classification = preserved
```

Under this exact synthetic software toolchain, explicit encoder time bases preserve the decoded frame sequence while removing the cadence compression.

This is candidate evidence, not production authorization.

### Matroska with automatic encoder time base

Histogram of positive PTS deltas:

| PTS delta | Count |
|---:|---:|
| 41,000 us | 80 |
| 42,000 us | 159 |

The remaining 60 transitions have equal PTS:

```text
duplicate_pts = 60
near_zero_delta = 0
frames = 300
unique decoded frames = 300
adjacent duplicate frames = 0
```

This proves that the timing defect exists before MPEG-TS or HLS segmentation. The automatic encoder time-base path has already quantized 300 unique frames onto an approximately 24 fps timestamp grid, producing 60 equal packet PTS values.

### Matroska with AVTB

Histogram:

| PTS delta | Count |
|---:|---:|
| 33,000 us | 199 |
| 34,000 us | 100 |

The one-millisecond Matroska time base projects the source 33.333-millisecond cadence as an expected 33/34-millisecond pattern without equal PTS, dropped frames, or decoded duplicates.

### Copy-only Matroska to MPEG-TS and HLS

Both copy-only variants preserve the Matroska decoded frame sequence exactly.

Histogram:

| PTS delta | Count |
|---:|---:|
| 11 us | 60 |
| 40,989 us | 20 |
| 41,000 us | 60 |
| 41,989 us | 40 |
| 42,000 us | 119 |

The parent Matroska file has 60 equal PTS values. MPEG-TS requires monotonic timestamp representation and projects those equal values into 60 one-clock-tick intervals:

```text
1 / 90000 second = approximately 11 microseconds
```

HLS segmentation does not add a second transformation: raw MPEG-TS and HLS MPEG-TS produce the same cadence histogram.

## Root-cause boundary

The certified evidence supports this layer model:

```text
VFR source, 300 unique frames at 30 fps
        |
        v
encoder time base = auto
        |
        +-- Matroska: 60 equal packet PTS
        |
        v
MPEG-TS timestamp projection
        |
        +-- 60 equal PTS become 60 one-tick / 11-us intervals
        |
        v
HLS: same MPEG-TS cadence pattern
```

Primary cause within the certified matrix:

```text
automatic encoder time-base selection / encoded packet timestamp quantization
```

Secondary representation effect:

```text
MPEG-TS converts equal packet PTS into one-clock-tick spacing
```

HLS segmentation itself is not the originating layer.

## What the phase proves

The phase proves, under the reference software toolchain:

- the production baseline retains all 300 unique decoded frames;
- the 11-microsecond pattern is not decoded-frame duplication;
- `fps_mode=vfr` drops 59 frames;
- `fps_mode=cfr` creates hundreds of real adjacent duplicate frames;
- automatic encoder time base creates 60 equal PTS values before MPEG-TS;
- MPEG-TS converts those equal PTS values into 11-microsecond intervals;
- HLS and raw MPEG-TS have the same copy-remux cadence result;
- explicit AVTB and 90 kHz encoder time bases preserve the source-window cadence and decoded sequence in the certified case.

## What the phase does not prove

It does not prove:

- that AVTB or 90 kHz is safe for production;
- behavior across repeated executions and different FFmpeg builds;
- behavior for other VFR patterns or rates;
- behavior for B-frames, non-zero DTS reorder, HDR, interlaced media, or edit lists;
- A/V boundary sync after changing encoder time base;
- hardware encoder behavior;
- acceptable playback on Web, PC, Android, Emby, or Infuse;
- safe seamless playback.

## CLI

List variants:

```bash
go run ./cmd/transcode-vfr-isolation-cert -list
```

Run the matrix:

```bash
go run ./cmd/transcode-vfr-isolation-cert \
  -output vfr-layer-isolation-matrix-v1.json
```

Keep the diagnostic workspace:

```bash
go run ./cmd/transcode-vfr-isolation-cert \
  -work-dir /tmp/nowen-vfr-isolation \
  -keep-work-dir \
  -output /tmp/nowen-vfr-isolation/vfr-layer-isolation-matrix-v1.json
```

## CI

Dedicated workflow:

```text
.github/workflows/transcode-vfr-isolation-cert.yml
```

Exact semantic validator:

```text
.github/scripts/verify_vfr_isolation.py
```

The workflow:

1. tests the bound evidence contracts;
2. builds the isolation CLI;
3. produces the bound Output Cadence baseline;
4. produces all nine real-media variants;
5. decodes every output and computes SHA-256 frame fingerprints;
6. validates complete cadence arithmetic and command identities;
7. verifies exact frame counts, duplicate PTS, near-zero intervals and histograms;
8. verifies actual adjacent decoded-frame duplication;
9. requires copy-only remux sequence identity;
10. uploads `vfr-layer-isolation-matrix-v1.json` as a CI Artifact.

## Production adoption gate

A production encoder time-base policy must be introduced through a separate contract and migration phase. At minimum it must prove:

1. three-run or greater deterministic variance across all selected cases;
2. 24/30, 25/30, 29.97/59.94 and additional real VFR patterns;
3. positive, negative and edit-list source origins;
4. exact frame fingerprints with no drops or decoded duplicates;
5. bounded A/V skew and boundary delta after applying the candidate;
6. correct DTS ordering for B-frame content;
7. software and hardware backend behavior;
8. Web, PC, Android, Emby and Infuse playback acceptance;
9. rollback and fallback behavior;
10. continued fail-closed discontinuity policy until all gates pass.

Until that phase is complete:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```
