# Timestamp Execution Plan v2

## Status

`hls-timestamp-execution-plan-v2` is a certification-only candidate contract for explicit Startup-to-Continuation boundary shaping.

It does not replace or reinterpret:

- `hls-timestamp-normalization-v1`;
- `hls-encoding-plan-v1`;
- `hls-produced-media-attestation-v1`;
- `hls-boundary-packet-evidence-v1`;
- `startup-handoff-timeline-v2`.

The v2 contract binds the canonical v1 Timestamp Plan identity and evaluates one narrow execution change:

```text
Continuation video PTS shift
Continuation audio PTS shift
```

It is deliberately fail-closed:

```text
certification_only = true
seamless_allowed = false
discontinuity_required = true
certified_backends = ["none"]
```

No production Job, Artifact, planner version, database row, playback API, or bridge playlist consumes this plan yet.

## Why a new contract is required

The current production timestamp contract preserves source seek origin with:

```text
-copyts
-start_at_zero
-avoid_negative_ts disabled
-fps_mode passthrough
```

Real packet evidence proves that timestamp reset is fixed, but the 30-second Startup/Continuation boundary still contains:

- one video packet overlap;
- multiple AAC packet overlap.

Changing FFmpeg filters without a new identity would make two materially different executions appear to use the same persisted Timestamp Plan. v2 therefore records:

```text
schema and strategy
base Timestamp Plan version/hash
microsecond seek precision
video PTS shift in microseconds
audio PTS shift in microseconds
backend certification scope
fail-closed playback policy
```

Canonical JSON and SHA-256 provide deterministic identity for every candidate.

## Command Adapter

`internal/transcode/timestampexecution.ApplyContinuation` works on one complete FFmpeg argument vector.

For a candidate with:

```text
video_pts_shift_micros = 33333
audio_pts_shift_micros = 64000
```

it merges:

```text
-vf scale=640:360,setpts=PTS+0.033333/TB
-af asetpts=PTS+0.064000/TB
```

The Adapter:

- validates the complete execution plan;
- copies the caller argument slice;
- merges with an existing `-vf` or `-af` option;
- inserts a missing filter option before the output path;
- rejects duplicate filter options;
- leaves the zero-shift baseline byte-for-byte equivalent;
- never changes Startup media.

PTS shifting changes timestamps only. It does not claim to synthesize silence, duplicate frames, trim encoder priming, or preserve client decoder state.

## Registered real-media matrix

`ffmpeg-boundary-shaping-matrix-v1` uses the exact 30-second boundary and production-shaped software fixtures.

| Case | Sample rate | Video shift | Audio shift | Purpose |
| --- | ---: | ---: | ---: | --- |
| `shape-48k-baseline-v1` | 48 kHz | 0 | 0 | Reproduce current boundary |
| `shape-48k-common-video-frame-v1` | 48 kHz | 33.333 ms | 33.333 ms | Shift both streams by one video frame |
| `shape-48k-common-aac-two-v1` | 48 kHz | 42.667 ms | 42.667 ms | Shift both streams by two AAC units |
| `shape-48k-common-aac-three-v1` | 48 kHz | 64.000 ms | 64.000 ms | Shift both streams by three AAC units |
| `shape-48k-per-stream-v1` | 48 kHz | 33.333 ms | 64.000 ms | Shape each stream by its observed unit class |
| `shape-44k1-baseline-v1` | 44.1 kHz | 0 | 0 | Reproduce current boundary |
| `shape-44k1-common-aac-two-v1` | 44.1 kHz | 46.440 ms | 46.440 ms | Shift both streams by two AAC units |
| `shape-44k1-per-stream-v1` | 44.1 kHz | 33.333 ms | 46.440 ms | Shape each stream independently |

The matrix stores, for every candidate:

```text
Timestamp Execution Plan version/hash/canonical JSON
Startup and Continuation Produced-media Attestation identities
Boundary Packet Evidence version/hash
video and audio packet windows
presentation and decode deltas
AAC sample projection
FFmpeg and FFprobe versions
```

The zero-shift plans intentionally share one plan hash across 48 kHz and 44.1 kHz because the execution policy is identical. Their evidence hashes remain distinct because the produced media and fixture identities differ.

## CLI

List all registered cases:

```bash
go run ./cmd/transcode-fixture-cert -list
```

Run the candidate matrix:

```bash
go run ./cmd/transcode-fixture-cert \
  -shaping-matrix \
  -output ./artifacts/timestamp-execution-shaping-matrix-v1.json
```

The following modes are mutually exclusive:

```text
-all
-boundary-matrix
-shaping-matrix
```

## CI

`.github/workflows/transcode-shaping-cert.yml` is the dedicated ownership boundary for this phase.

It verifies:

- Timestamp Execution Plan identity and safety rules;
- command Adapter immutability and filter merging;
- immutable shaping registry order;
- all eight real-media cases;
- packet-window and tick arithmetic;
- AAC nominal packet samples;
- one consistent FFmpeg/FFprobe toolchain;
- unique Produced-media evidence identities;
- explicit fail-closed playback policy.

The workflow uploads:

```text
timestamp-execution-shaping-matrix-v1.json
```

Exact measured deltas are evidence, not permanent pass/fail constants. A toolchain upgrade may change legal packet placement while all identity and integrity rules remain valid.

## Production adoption gate

A candidate must not be copied into runtime code merely because it removes a negative delta in this synthetic matrix.

Before production adoption, the selected policy must additionally prove:

1. stable results across repeated runs and FFmpeg versions;
2. 24 fps, 25 fps, 29.97 fps, and variable-frame-rate behavior;
3. non-zero and negative source timestamp behavior;
4. open-GOP and B-frame decode-order behavior;
5. explicit A/V sync preservation across the boundary;
6. AAC priming, padding, resampling, and offset behavior;
7. QSV, NVENC, and VAAPI as separate backend identities;
8. browser, Android/ExoPlayer, PC, Emby, and Infuse playback behavior;
9. persisted Job and Artifact migration and rollback design.

Only after those gates may a later persisted production schema evaluate planner adoption.

## Non-claims

This phase does not prove:

- sample-perfect handoff;
- gapless audio;
- absence of encoder priming;
- correct silence insertion;
- decoder continuity;
- hardware encoder equivalence;
- client playback certification;
- safety of removing `#EXT-X-DISCONTINUITY`.

`#EXT-X-DISCONTINUITY` remains mandatory.
