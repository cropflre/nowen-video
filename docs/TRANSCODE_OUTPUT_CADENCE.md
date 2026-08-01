# Transcode Output Cadence and Frame-Count Evidence

## Status

This document records the certification-only output-cadence evidence introduced on `refactor/server-lite-v1`.

The evidence does not change production FFmpeg arguments, playback planning, persisted transcode state, or client behavior. It cannot authorize seamless Startup-to-Continuation handoff.

Current policy remains:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Why this evidence exists

Source-Origin Evidence v1 proved the source timestamp origin and source packet cadence before the production Timestamp Plan was applied. Boundary Packet Evidence and A/V Boundary Sync Evidence measured the first packets around the 30-second handoff.

Those contracts did not inspect the complete produced-video timeline. In particular, they could not answer:

1. Did Startup or Continuation preserve the source frame count?
2. Did produced PTS remain strictly increasing?
3. Did VFR input preserve its source-window cadence distribution?
4. Did FFmpeg represent extra frames by dropping frames, repeating PTS, or compressing frame intervals?
5. Did a positive or negative source timestamp origin change the output cadence result?

Output Cadence Evidence v1 answers those questions without changing the meaning of the previous contracts.

## Contracts

### Output cadence evidence

Schema:

```text
hls-output-cadence-evidence-v1
```

The contract binds these upstream identities:

- Source-Origin Evidence v1;
- Timestamp Plan v1;
- Boundary Packet Evidence v1;
- A/V Boundary Sync Evidence v1;
- FFmpeg and FFprobe toolchain identity.

It records five complete video timelines:

```text
source_full
source_startup
source_continuation
output_startup
output_continuation
```

Each timeline records:

- first and last PTS;
- frame count;
- minimum and maximum positive PTS delta;
- a deterministic, ordered delta histogram;
- dominant delta and occurrence count;
- duplicate-PTS and non-monotonic-PTS counts;
- raw variable-duration classification;
- significant cadence buckets;
- low-frequency outlier count;
- near-zero delta count;
- material variable-duration classification.

The source-window frame counts are bound to the produced Startup and Continuation frame counts through explicit mappings.

### Matrix report

Schema:

```text
ffmpeg-output-cadence-matrix-v1
```

The matrix requires:

- all registered Source-Origin cases in exact order;
- one consistent FFmpeg/FFprobe toolchain;
- unique output-cadence hashes;
- valid nested Source-Origin, Boundary, and A/V evidence;
- canonical SHA-256 identity for every output-cadence contract;
- fail-closed playback policy for every case.

## Classification rules

### Frame-count projection

The mapping reports:

```text
aligned
within_tolerance
duplicate_projection
drop_projection
```

A positive frame-count delta is only a `duplicate_projection`; a negative delta is only a `drop_projection`. These names describe arithmetic differences between source and output frame counts.

They are not content-level duplicate-frame detection.

V1 therefore fixes:

```text
content_duplicate_classification = not_measured
```

Content equality requires decoded-frame fingerprints and is intentionally outside this contract.

### Raw and material cadence

`variable_duration` is derived from the complete minimum-to-maximum delta spread. It preserves every measured extreme.

`material_variable_duration` uses only histogram buckets whose occurrence count is at least:

```text
max(2, ceil(positive_delta_count / 100))
```

This separates a rare muxer boundary artifact from a recurring cadence pattern without deleting the rare value from evidence.

A positive delta below 1,000 microseconds is additionally counted as a near-zero delta:

```text
near_zero_delta_threshold = 1000us
```

Near-zero deltas are not treated as ordinary outliers when they occur frequently enough to form a significant histogram bucket.

### Preservation status

The contract reports:

```text
preserved_exact
preserved_with_cadence_outliers
preserved_with_count_tolerance
changed
```

`preserved_exact` requires:

- exact source/output frame counts;
- strictly increasing, non-duplicate PTS;
- matching source-window and output material cadence;
- dominant source/output deltas within 1,000 microseconds;
- no newly introduced near-zero deltas;
- no output cadence outliers.

A recurring near-zero bucket or a material source/output cadence mismatch produces `changed`, even when the frame counts are identical.

## Certified cases

The matrix reuses the immutable Source-Origin registry:

| Case | Source mode | Source origin | Startup frames | Continuation frames |
|---|---|---:|---:|---:|
| 24 fps | CFR | 0 s | 720 | 240 |
| 25 fps | CFR | 0 s | 750 | 250 |
| 30000/1001 fps | CFR | 0 s | 900 | 299 |
| 24-to-30 fps | VFR | 0 s | 780 | 300 |
| 30 fps | CFR | +5 s | 900 | 300 |
| 30 fps | CFR | -2 s | 900 | 300 |

Every case uses:

- the same deterministic source graph consumed by FFprobe and production-shaped FFmpeg;
- software H.264/AAC encoding;
- a 30-second Startup/Continuation boundary;
- the production HLS argument builder;
- the production Timestamp Plan;
- complete produced-media attestation, Boundary Evidence, and A/V Sync Evidence.

## Reference results

Reference environment:

```text
Ubuntu 24.04.4
FFmpeg 6.1.1-3ubuntu5
FFprobe 6.1.1-3ubuntu5
```

### CFR cases

All CFR cases produced:

- exact source/output frame counts;
- zero projected duplicate frames;
- zero projected dropped frames;
- zero duplicate PTS;
- zero non-monotonic PTS;
- zero near-zero deltas;
- `preserved_exact`.

| Case | Source-window dominant delta | Output dominant delta | Result |
|---|---:|---:|---|
| 24 fps | 41,667 us | 41,667 us | preserved_exact |
| 25 fps | 40,000 us | 40,000 us | preserved_exact |
| 30000/1001 fps | 33,367 us | 33,367 us | preserved_exact |
| 30 fps, +5 s origin | 33,333 us | 33,333 us | preserved_exact |
| 30 fps, -2 s origin | 33,333 us | 33,333 us | preserved_exact |

Positive and negative source timestamp origins therefore do not change the certified output cadence result after Timestamp Plan normalization.

### VFR case

The VFR source is:

```text
0s  - 20s: 24 fps
20s - 40s: 30 fps
```

Source Startup window histogram:

| Delta | Count |
|---:|---:|
| 33,333 us | 199 |
| 33,334 us | 100 |
| 41,666 us | 160 |
| 41,667 us | 320 |

Source Continuation window histogram:

| Delta | Count |
|---:|---:|
| 33,333 us | 199 |
| 33,334 us | 100 |

Produced Startup histogram:

| Delta | Count |
|---:|---:|
| 11 us | 60 |
| 41,656 us | 60 |
| 41,667 us | 659 |

Produced Continuation histogram:

| Delta | Count |
|---:|---:|
| 11 us | 60 |
| 41,656 us | 60 |
| 41,667 us | 179 |

The frame counts remain exact:

```text
Startup:      780 source -> 780 output
Continuation: 300 source -> 300 output
```

PTS remains strictly increasing, so there are no equal-PTS duplicates. However, the 30 fps source window has 60 more frames than a 24 fps timeline over ten seconds. The produced timeline retains those 60 frames by assigning exactly 60 near-zero 11-microsecond intervals while the dominant output cadence remains approximately 41.667 milliseconds.

This is a material cadence change, not a rare outlier:

```text
preservation_status = changed
near_zero_delta_count = 60 per 10-second 30 fps window
projected_duplicate_frames = 0
projected_dropped_frames = 0
duplicate_pts_count = 0
non_monotonic_pts_count = 0
```

The evidence proves that frame-count equality alone is insufficient to claim VFR preservation.

## Interpretation boundary

The matrix proves the produced packet timeline under the certified software toolchain.

It does not yet prove which individual layer creates the VFR cadence compression. The remaining candidates include:

- filtergraph/output time-base selection;
- encoder time-base behavior;
- H.264 packet timestamp generation;
- MPEG-TS/HLS muxer time-base projection;
- an interaction between those layers.

The current evidence strongly constrains the failure mode, but attributing it to one layer requires an isolation matrix that changes one layer at a time.

The matrix also does not prove:

- decoded-frame content equality;
- client-visible smoothness;
- hardware encoder behavior;
- behavior on 23.976, 50, 59.94, or high-frame-rate VFR sources;
- safe seamless playback.

## CLI

List registered cases:

```bash
go run ./cmd/transcode-output-cadence-cert -list
```

Run the matrix:

```bash
go run ./cmd/transcode-output-cadence-cert \
  -output output-cadence-matrix-v1.json
```

Keep a diagnostic workspace:

```bash
go run ./cmd/transcode-output-cadence-cert \
  -work-dir /tmp/nowen-output-cadence \
  -keep-work-dir \
  -output /tmp/nowen-output-cadence/output-cadence-matrix-v1.json
```

## CI

Dedicated workflow:

```text
.github/workflows/transcode-output-cadence-cert.yml
```

The workflow:

1. tests the output-cadence and upstream evidence contracts;
2. builds the certification CLI;
3. produces all six real-media cases;
4. recomputes histogram arithmetic and canonical hashes;
5. verifies exact frame-count mappings and fail-closed policy;
6. verifies the recurring VFR near-zero cadence pattern;
7. uploads `output-cadence-matrix-v1.json` as a CI Artifact.

## Production gate

This phase is evidence-only.

A production VFR policy change requires a separate layer-isolation matrix and must prove:

1. the selected command policy removes recurring near-zero deltas;
2. source-window cadence is preserved or deliberately normalized under an explicit contract;
3. frame fingerprints prove whether frames are duplicated, dropped, or reordered;
4. A/V boundary sync remains bounded;
5. real Web, PC, Android, Emby, and Infuse playback is acceptable;
6. rollback and fallback behavior remain fail closed.

Until that gate is complete:

```text
seamless_allowed = false
discontinuity_required = true
```
