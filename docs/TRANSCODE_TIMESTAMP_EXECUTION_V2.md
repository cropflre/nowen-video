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
-vf scale=320:180,setpts=PTS+0.033333/TB
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

## Measured Ubuntu 24.04 / FFmpeg 6.1.1 evidence

The dedicated CI matrix produced the following presentation-boundary evidence:

| Case | Video result | Video delta | Audio result | Audio delta |
| --- | --- | ---: | --- | ---: |
| `shape-48k-baseline-v1` | single-packet overlap | -21.333 ms | multi-packet overlap | -58.667 ms / -2816 samples |
| `shape-48k-common-video-frame-v1` | single-packet gap | +12.000 ms | multi-packet overlap | -25.356 ms / -1217 samples |
| `shape-48k-common-aac-two-v1` | single-packet gap | +12.000 ms | single-packet overlap | -16.000 ms / -768 samples |
| `shape-48k-common-aac-three-v1` | multi-packet gap | +45.333 ms | single-packet gap | +5.333 ms / +256 samples |
| `shape-48k-per-stream-v1` | single-packet gap | +12.000 ms | single-packet gap | +5.333 ms / +256 samples |
| `shape-44k1-baseline-v1` | single-packet overlap | -23.222 ms | multi-packet overlap | -46.622 ms / -2056 samples |
| `shape-44k1-common-aac-two-v1` | single-packet gap | +10.111 ms | aligned | -0.178 ms / -8 samples |
| `shape-44k1-per-stream-v1` | single-packet gap | +10.111 ms | aligned | -0.178 ms / -8 samples |

The measurements establish several useful facts:

1. Timestamp shaping predictably moves produced packet boundaries; the negative baseline is not immutable muxer noise.
2. A common 48 kHz shift of one video frame does not resolve the AAC overlap.
3. A common 48 kHz shift of three AAC access units over-corrects video to a larger multi-packet gap.
4. The 48 kHz per-stream candidate yields small positive gaps for both streams, but it changes their relative boundary placement and therefore still requires A/V sync and client certification.
5. At 44.1 kHz, two AAC access units reduce the measured audio boundary to eight overlapping samples while video becomes a small positive gap.
6. The equal and per-stream 44.1 kHz candidates produce the same packet result in this fixture, showing encoder/muxer quantization can collapse distinct requested shifts into the same materialized boundary.

None of these results selects a production winner. A small positive gap is not automatically safer than a small overlap, and packet-level alignment is not equivalent to decoder continuity or gapless playback.

CI run:

```text
Transcode Timestamp Execution Certification #6
Run ID: 30692387724
Artifact ID: 8816120517
Artifact ZIP SHA-256: a7e8962068e94e418ab04a591cbc4447dc8ffadaf4b418f6886eafabc736c3fc
```

## Certified backend scheduling

Timestamp-normalized Startup and Continuation Jobs already require the software backend under `hls-timestamp-normalization-v1`.

The Worker now calls `preferredAttemptBackend(job)` before creating an Attempt or workspace. Therefore:

```text
certified timestamp Job + detected QSV/NVENC/VAAPI
  -> software Attempt #N directly
```

It no longer creates an impossible hardware Attempt #N followed by software Attempt #N+1. Runtime Jobs without a timestamp certification contract retain the detected hardware candidate and existing hardware-to-software fallback.

This scheduling correction does not adopt Timestamp Execution Plan v2. It only makes the already-deployed v1 backend certification authoritative before durable Attempt creation.

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
