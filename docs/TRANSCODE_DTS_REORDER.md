# Transcode DTS Reorder Evidence v1

## Status

This phase adds a dedicated B-frame and DTS-reorder certification gate for the two explicit encoder time-base candidates:

```text
encoder-time-base-avtb-v1 = 1/1000000
encoder-time-base-90k-v1  = 1/90000
```

It is an evidence layer only. It does not change the production Encoding Plan, Startup Stream, Continuation Stream, HLS playlist policy, or client playback behavior.

The safety posture remains:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Why this phase exists

The Encoder Time-Base Candidate matrix proved equivalent presentation cadence for AVTB and 90 kHz under software H.264/AAC, but its production-shaped command used the zero-latency x264 path. That path does not prove behavior when encoded packets are emitted in decode order and presented in a different order.

B-frame media introduces a second timeline:

- **PTS** controls presentation order;
- **DTS** controls decoder submission order;
- `PTS - DTS` is the composition delay;
- packet order may contain PTS backsteps even while DTS remains strictly increasing.

A candidate that looks correct in PTS-only evidence can still be unsafe if it duplicates DTS, emits non-monotonic decode timestamps, changes reorder depth, or diverges from the alternate time-base candidate.

## Matrix

The v1 matrix uses deterministic libx264 B-frame structure:

```text
bframes = 3
b-adapt = 0
open-gop = 0
scenecut = 0
threads = 1
```

Cases:

| Case | Frame rate | GOP | Startup | Continuation |
|---|---:|---:|---:|---:|
| `dts-cfr-24000-1001-b3-v1` | 24000/1001 | 48 | `[0s, 8s)` | `[8s, 12s)` |
| `dts-cfr-30000-1001-b3-v1` | 30000/1001 | 60 | `[0s, 8s)` | `[8s, 12s)` |
| `dts-cfr-60-b3-v1` | 60 | 120 | `[0s, 8s)` | `[8s, 12s)` |

Each case runs:

```text
2 candidates × 3 repeats × 2 windows = 12 executions
3 cases = 36 executions
```

The output container is MPEG-TS so both candidates are evaluated after the same 90 kHz muxing boundary used by HLS transport streams.

## Evidence contract

The generated report uses:

```text
schema_version = encoder-time-base-dts-reorder-evidence-v1
```

Every Startup and Continuation window records:

- packet count and key-packet count;
- first and last PTS/DTS in ticks and microseconds;
- duplicate PTS and duplicate DTS counts;
- non-increasing DTS transitions;
- PTS backsteps observed in decode order;
- positive and negative composition-offset counts;
- minimum and maximum `PTS - DTS`;
- maximum decode-to-presentation displacement;
- presentation cadence histogram;
- decode DTS cadence histogram;
- composition-offset histogram;
- normalized presentation sequence hash;
- normalized decode-DTS sequence hash;
- decode-to-presentation rank hash;
- composition-offset sequence hash.

The report also records the Continuation-to-Startup first-PTS delta relative to the declared eight-second boundary.

## Required invariants

A window fails certification when any of the following occurs:

- duplicate PTS;
- duplicate DTS;
- equal or decreasing DTS;
- no PTS backstep, because B-frame reorder would be unproven;
- no positive composition delay;
- a negative `PTS - DTS` value;
- no decode/presentation displacement;
- reorder displacement larger than the configured three-B-frame bound plus one anchor frame;
- non-increasing presentation time after sorting by PTS;
- non-increasing decode time.

A candidate fails when any semantic window projection changes across three repeats. A case fails when AVTB and 90 kHz differ in presentation sequence, decode order, composition-offset sequence, cadence, or boundary start behavior.

Absolute first/last packet timestamps are retained as evidence but excluded from repeat equivalence. The normalized packet structure is the stable contract.

## Reference run

A local reference run with FFmpeg 7.1.3 completed all 36 executions and produced:

```text
all_cases_stable = true
all_candidates_equivalent = true
seamless_allowed = false
discontinuity_required = true
```

Observed first-run packet counts and reorder depth were:

| Case | Startup packets | Continuation packets | Max forward reorder | Max backward reorder | Boundary delta |
|---|---:|---:|---:|---:|---:|
| 24000/1001 | 192 | 96 | 3 | 2 | 0 us |
| 30000/1001 | 240 | 120 | 3 | 2 | 0 us |
| 60 | 480 | 240 | 3 | 2 | 0 us |

These values are not yet an exact cross-version baseline. CI validates the semantic invariants and candidate equivalence against the runner's installed FFmpeg version.

## Commands

Run analyzer unit tests:

```bash
python3 .github/scripts/test_verify_dts_reorder.py
```

Produce the matrix:

```bash
python3 .github/scripts/verify_dts_reorder.py \
  --output /tmp/dts-reorder-evidence-v1.json
```

Keep generated transport streams for investigation:

```bash
python3 .github/scripts/verify_dts_reorder.py \
  --keep-work-dir \
  --output /tmp/dts-reorder-evidence-v1.json
```

CI workflow:

```text
.github/workflows/transcode-dts-reorder-cert.yml
```

Artifact:

```text
dts-reorder-evidence-v1.json
```

## Non-claims

This phase does not prove:

- representative real-file demuxer behavior;
- hardware encoder reorder behavior;
- HEVC, AV1, interlaced, HDR, edit-list, or discontinuous-source behavior;
- long-duration DTS or A/V drift behavior;
- HLS segment-boundary decoder acceptance;
- Web, PC, Android, Emby, or Infuse acceptance;
- safe removal of `#EXT-X-DISCONTINUITY`;
- that AVTB or 90 kHz should be enabled as the production encoder time base.

## Next gate

The next production gate is **representative real-media fixture certification**. It must bind real demuxer/container behavior to the existing:

- Timestamp Plan;
- Encoder Time-Base Candidate evidence;
- DTS Reorder evidence;
- Produced-media Attestation;
- Boundary and A/V evidence.

Production selection remains deferred until real-media, hardware, long-duration, client-acceptance, rollout, fallback, and rollback gates all pass.
