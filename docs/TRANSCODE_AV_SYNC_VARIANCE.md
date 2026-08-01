# A/V Boundary Sync Evidence and Repeated-run Variance

## Status

This phase introduces two certification-only schemas:

```text
hls-av-boundary-sync-evidence-v1
ffmpeg-av-boundary-sync-variance-matrix-v1
```

They are diagnostic contracts layered on top of:

```text
hls-boundary-packet-evidence-v1
hls-timestamp-execution-plan-v2
```

They do not replace or reinterpret the persisted production Timestamp Plan, Timeline contract, Artifact identity, playback planner, or bridge playlist.

The safety policy remains:

```text
certification_only = true
seamless_allowed = false
discontinuity_required = true
```

`#EXT-X-DISCONTINUITY` remains mandatory.

## Sign convention

All A/V skew fields use:

```text
audio timestamp - video timestamp
```

Therefore:

- a positive value means audio is later than video;
- a negative value means audio is earlier than video;
- a value near zero means the two stream boundaries are aligned within the packet-derived tolerance.

The contract measures two independent positions:

```text
startup_end_skew = startup audio end - startup video end
continuation_start_skew = continuation audio start - continuation video start
```

It then records the change in relative A/V position:

```text
skew_transition = continuation_start_skew - startup_end_skew
```

The same change can also be projected from the independent stream handoff deltas:

```text
boundary_delta_skew = audio boundary delta - video boundary delta
```

## Integer-microsecond projection residual

Packet timestamps originate in stream-specific rational time bases. Each timestamp and each stream delta is independently projected to integer microseconds.

Consequently, two mathematically equivalent formulas can differ by a small rounding residue:

```text
projection_residual = skew_transition - boundary_delta_skew
```

The contract does not hide this difference by widening the repeated-run stability threshold. It records the residual as first-class evidence and enforces:

```text
absolute projection residual <= 2 microseconds
```

Repeated-run determinism remains stricter:

```text
maximum span across three repeats <= 1 microsecond
```

## Registered matrix

The matrix executes four real-media cases three times each:

| Case | Sample rate | Video shift | Audio shift | Role |
| --- | ---: | ---: | ---: | --- |
| `shape-48k-baseline-v1` | 48 kHz | 0 ms | 0 ms | Current production-shaped baseline |
| `shape-48k-per-stream-v1` | 48 kHz | 33.333 ms | 64.000 ms | Best 48 kHz candidate from Timestamp Execution v2 |
| `shape-44k1-baseline-v1` | 44.1 kHz | 0 ms | 0 ms | Current production-shaped baseline |
| `shape-44k1-common-aac-two-v1` | 44.1 kHz | 46.440 ms | 46.440 ms | Best 44.1 kHz candidate from Timestamp Execution v2 |

Every repeat stores:

- complete Timestamp Execution Plan version, hash, and canonical JSON;
- complete packet-level Boundary Evidence version and hash;
- complete A/V Boundary Sync Evidence version and hash;
- FFmpeg and FFprobe identities;
- Startup and Continuation Produced-media Attestation identities;
- fail-closed playback policy.

## Ubuntu 24.04 / FFmpeg 6.1.1 evidence

Reference environment:

```text
Ubuntu 24.04
FFmpeg 6.1.1-3ubuntu5
FFprobe 6.1.1-3ubuntu5
30 fps CFR
30-second Startup-to-Continuation boundary
```

All observed ranges had a zero-microsecond span across three independent executions.

| Case | Startup end skew | Continuation start skew | Boundary delta skew | Skew transition | Projection residual | Three-run span |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 48 kHz baseline | +16.000 ms | -21.333 ms | -37.334 ms | -37.333 ms | +0.001 ms | 0 ms |
| 48 kHz per-stream | +16.000 ms | +9.334 ms | -6.667 ms | -6.666 ms | +0.001 ms | 0 ms |
| 44.1 kHz baseline | +0.178 ms | -23.222 ms | -23.400 ms | -23.400 ms | 0 ms | 0 ms |
| 44.1 kHz common two-AAC | +0.178 ms | -10.111 ms | -10.289 ms | -10.289 ms | 0 ms | 0 ms |

The comparison gate measures the maximum absolute change in relative A/V position:

| Comparison | Baseline | Candidate | Improvement |
| --- | ---: | ---: | ---: |
| 48 kHz per-stream vs baseline | 37.334 ms | 6.667 ms | 30.667 ms |
| 44.1 kHz common two-AAC vs baseline | 23.400 ms | 10.289 ms | 13.111 ms |

## Interpretation

The results establish the following narrow facts for this exact synthetic source and toolchain:

1. The four selected executions are deterministic at integer-microsecond resolution across three repeats.
2. The 48 kHz per-stream candidate substantially reduces the relative A/V jump across the boundary.
3. The 44.1 kHz common two-AAC candidate also reduces the relative A/V jump, but leaves audio approximately 10.111 ms earlier than video at Continuation start.
4. Timestamp shaping changes relative stream placement; it does not prove decoder continuity or gapless playback.
5. A small or positive packet gap is not automatically safer than a small overlap.
6. Integer-microsecond projection residual is deterministic evidence, not runtime jitter.

## CLI

Run the matrix:

```bash
go run ./cmd/transcode-fixture-cert \
  -av-sync-variance-matrix \
  -output ./artifacts/av-boundary-sync-variance-matrix-v1.json
```

List the registered cases:

```bash
go run ./cmd/transcode-fixture-cert -list
```

The following matrix modes are mutually exclusive:

```text
-all
-boundary-matrix
-shaping-matrix
-av-sync-variance-matrix
```

## CI ownership

`.github/workflows/transcode-av-sync-cert.yml` verifies:

- immutable case order;
- exactly three repeats per case;
- one stable Timestamp Execution Plan identity per case;
- one consistent FFmpeg/FFprobe toolchain across the matrix;
- Boundary Evidence and A/V Evidence canonical SHA-256 identities;
- exact A/V skew arithmetic;
- explicit projection residual arithmetic and bound;
- no more than one microsecond repeated-run span;
- positive relative-skew improvement for both selected candidates;
- `seamless_allowed=false` and `discontinuity_required=true` at every layer.

The workflow uploads:

```text
av-boundary-sync-variance-matrix-v1.json
```

## Production adoption gate

These results are necessary evidence, not production authorization. Before any Timestamp Execution v2 candidate can be persisted or consumed by runtime playback, later phases must cover:

1. 24 fps, 25 fps, 29.97 fps, and variable-frame-rate sources;
2. non-zero and negative source timestamp origins;
3. PTS and DTS A/V relationships around B-frames and open GOPs;
4. AAC priming, padding, resampling, and explicit silence/trim behavior;
5. QSV, NVENC, and VAAPI as separate execution identities;
6. browser, Android/ExoPlayer, PC, Emby, and Infuse playback certification;
7. repeated runs across multiple FFmpeg versions;
8. persisted Job/Artifact migration, rollback, and resolver compatibility.

## Non-claims

This phase does not prove:

- sample-perfect handoff;
- gapless audio;
- decoder-state continuity;
- correct silence synthesis or packet trimming;
- hardware encoder equivalence;
- client playback safety;
- safety of removing `#EXT-X-DISCONTINUITY`;
- safety of enabling `seamless_allowed`.
