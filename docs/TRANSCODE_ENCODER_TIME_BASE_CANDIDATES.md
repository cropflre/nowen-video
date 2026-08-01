# Encoder Time-Base Candidate Expansion and Variance Evidence

## Status

This phase is certification-only. It does not modify the production FFmpeg builder, persisted transcode state, playback planning, HLS manifests, or any Web, PC, Android, Emby, or Infuse client behavior.

The production safety policy remains:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Why this phase exists

VFR Layer-Isolation Evidence v1 identified the automatic encoder time-base path as the layer that first compresses a 30 fps continuation window onto an approximately 24 fps packet timestamp grid. In the certified synthetic case:

- automatic encoder time base produced 60 equal packet PTS values before MPEG-TS;
- MPEG-TS represented those equal values as 60 one-clock-tick intervals of approximately 11 microseconds;
- `fps_mode=vfr` removed the near-zero intervals by dropping 59 decoded frames;
- `fps_mode=cfr` created hundreds of real adjacent duplicate frames;
- explicit `enc_time_base=1/1000000` and `enc_time_base=1/90000` preserved the 300-frame decoded sequence and source cadence.

That result was only one VFR pattern and one execution. A production policy cannot be selected from one successful cell. This phase expands the candidate surface and adds repeated-run variance, decoded-frame, packet-cadence, boundary, and A/V-sync gates.

## Evidence schemas

Contract schema:

```text
encoder-time-base-candidate-evidence-v1
```

Matrix schema:

```text
ffmpeg-encoder-time-base-candidate-matrix-v1
```

The contract binds:

- FFmpeg and FFprobe toolchain identity;
- the exact source case registry;
- the exact candidate registry;
- three executions per case and candidate;
- source Startup and Continuation cadence;
- complete output PTS cadence for both windows;
- source-to-output frame mappings;
- decoded-frame fingerprints;
- Startup and Continuation command identities;
- Boundary Packet Evidence v1;
- A/V Boundary Sync Evidence v1;
- repeated-run metric ranges;
- cross-candidate sequence, cadence, mapping, and A/V comparison;
- fail-closed playback policy.

Canonical contract JSON is identified by SHA-256.

## Candidate policies

The matrix compares exactly two candidates:

| Candidate | FFmpeg option |
|---|---|
| AVTB | `-enc_time_base:v:0 1/1000000` |
| MPEG-TS clock | `-enc_time_base:v:0 1/90000` |

Both retain the production Timestamp Plan and its existing `fps_mode=passthrough` policy. No `fps_mode` replacement is tested as a candidate because the previous layer-isolation phase proved that `vfr` drops frames and `cfr` duplicates frames for the certified VFR source.

## Source case matrix

Each source is 40 seconds long with a 30-second Startup/Continuation boundary and 48 kHz audio.

### CFR rates

- 24000/1001 fps;
- 24 fps;
- 25 fps;
- 30000/1001 fps;
- 30 fps;
- 50 fps;
- 60000/1001 fps.

### VFR patterns

Each VFR source uses one 20-second cadence followed by another 20-second cadence:

- 24 fps to 30 fps;
- 25 fps to 30 fps;
- 30000/1001 fps to 60000/1001 fps.

The 30-second boundary deliberately falls inside the second cadence. Startup therefore contains both rates, while Continuation isolates the higher-rate tail.

### Source origins

The matrix also repeats 30 fps CFR with:

- a positive five-second source origin;
- a negative two-second source origin.

This verifies that candidate behavior is independent of input timestamp origin after the production Timestamp Plan normalizes output.

## Repeat policy

Every case/candidate cell runs exactly three times.

A candidate is stable only when all three runs have:

- identical decoded Startup sequence hashes;
- identical decoded Continuation sequence hashes;
- identical packet-cadence signatures;
- identical frame mappings;
- no near-zero intervals;
- no duplicate or non-monotonic packet PTS;
- no adjacent identical decoded frames;
- frame-count preservation;
- A/V metrics with at most one microsecond repeated-run span.

## Packet cadence and frame-content evidence

Every output window records the complete ordered positive PTS-delta histogram and validates:

```text
positive PTS deltas
+ duplicate PTS
+ non-monotonic PTS
= frame count - 1
```

Decoded content is measured with FFmpeg `framemd5` using SHA-256. Each window records:

- decoded frame count;
- unique decoded frame count;
- adjacent identical-frame count;
- ordered frame-sequence SHA-256;
- first-frame SHA-256;
- last-frame SHA-256.

The candidate gate requires zero actual adjacent duplicate frames and exact frame-count alignment. Packet-count projections are not treated as content-level duplicate detection.

## Boundary and A/V evidence

Each repeated run produces independent Startup and Continuation HLS artifacts and binds:

- Produced Media Attestation;
- Boundary Packet Evidence;
- A/V Boundary Sync Evidence.

Repeated-run variance is measured for:

- video boundary delta;
- audio boundary delta;
- Startup end skew;
- Continuation start skew;
- boundary delta skew;
- skew transition;
- projection residual.

The two candidates are also compared ordinal-by-ordinal. Their decoded sequences, frame mappings, and cadence signatures must be equivalent, while all A/V metric differences must remain within one millisecond.

## CLI

List cases and candidates:

```bash
go run ./cmd/transcode-encoder-time-base-cert -list
```

Run the complete matrix:

```bash
go run ./cmd/transcode-encoder-time-base-cert \
  -output encoder-time-base-candidate-matrix-v1.json
```

Keep the diagnostic workspace:

```bash
go run ./cmd/transcode-encoder-time-base-cert \
  -work-dir /tmp/nowen-encoder-time-base \
  -keep-work-dir \
  -output /tmp/nowen-encoder-time-base/encoder-time-base-candidate-matrix-v1.json
```

## CI

Dedicated workflow:

```text
.github/workflows/transcode-encoder-time-base-cert.yml
```

Semantic verifier:

```text
.github/scripts/verify_encoder_time_base.py
```

The workflow:

1. tests the candidate, cadence, Boundary, and A/V contracts;
2. builds the standalone CLI;
3. produces all 12 cases, two candidates, and three repeats;
4. validates complete PTS arithmetic and frame mappings;
5. decodes every Startup and Continuation output;
6. verifies repeated sequence and cadence determinism;
7. verifies Boundary and A/V variance;
8. verifies cross-candidate equivalence;
9. uploads `encoder-time-base-candidate-matrix-v1.json` as a CI Artifact.

## Production non-claims

This phase does not prove:

- hardware encoder safety;
- B-frame and DTS-reorder safety;
- HDR, interlaced, edit-list, or discontinuous-source behavior;
- long-duration drift behavior;
- real-file demuxer behavior beyond deterministic lavfi sources;
- Web, PC, Android, Emby, or Infuse playback acceptance;
- safe seamless Startup-to-Continuation handoff;
- that either candidate should be enabled in production.

## Next production gate

A production Encoder Time-Base Policy may only be proposed after this matrix succeeds and a separate phase adds:

1. B-frame and DTS-reorder cases;
2. representative real-media fixtures;
3. hardware backends;
4. long-duration A/V drift evidence;
5. client playback acceptance;
6. explicit capability, rollout, fallback, and rollback policy.

Until then:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```
