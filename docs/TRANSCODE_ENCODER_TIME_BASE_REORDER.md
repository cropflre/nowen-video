# Encoder Time-Base Reorder Certification

## Status

This phase extends Encoder Time-Base Candidate Evidence with deterministic B-frame decode-order and presentation-order certification.

It is certification-only. It does not modify the production FFmpeg builder, persisted transcode jobs, Startup or Continuation execution, HLS playlists, playback planning, or client behavior.

The production safety policy remains fail-closed:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Why this phase exists

The base encoder time-base matrix proves PTS cadence, decoded-frame preservation, Boundary evidence, and A/V evidence for the two explicit candidates:

```text
encoder-time-base-avtb-v1 = 1/1000000
encoder-time-base-90k-v1  = 1/90000
```

That matrix intentionally uses the production zero-latency software profile. A zero-latency profile does not exercise B-frame reordering, where packets are submitted to the decoder in DTS order but displayed in PTS order.

This phase removes the zero-latency tune only inside the certification command and adds deterministic closed-GOP B-frame policies. It then binds packet-order evidence back to the complete base candidate evidence for every run.

## Evidence schemas

Contract schema:

```text
encoder-time-base-reorder-evidence-v1
```

Matrix schema:

```text
ffmpeg-encoder-time-base-reorder-matrix-v1
```

Canonical contract JSON is identified by SHA-256.

## Candidate policies

The matrix keeps the same two explicit encoder time-base candidates:

| Candidate | FFmpeg option |
|---|---|
| AVTB | `-enc_time_base:v:0 1/1000000` |
| MPEG-TS clock | `-enc_time_base:v:0 1/90000` |

Both candidates use the same Timestamp Plan, output cadence policy, HLS transport, codec profile, GOP policy, and source case. Only the explicit encoder time base differs.

## B-frame policy

Every case uses deterministic software H.264 reordering:

```text
b-adapt = 0
b-pyramid = none
open-gop = 0
scenecut = 0
```

The case registry explicitly binds:

- B-frame count;
- reference-frame count;
- GOP size;
- source frame-rate policy;
- source timestamp origin;
- Startup and Continuation boundary.

Adaptive B-frame placement and open GOPs are outside v1 because they would make repeated packet-order comparison dependent on encoder heuristics.

## Case matrix

Each source is 40 seconds long with a 30-second Startup/Continuation boundary and 48 kHz audio.

| Case | Source | Reorder policy | Purpose |
|---|---|---|---|
| `reorder-cfr-24-b2-origin-zero-v1` | CFR 24 fps | 2 B-frames, 3 refs, 48-frame GOP | baseline cinematic cadence |
| `reorder-cfr-30000-1001-b3-origin-zero-v1` | CFR 30000/1001 fps | 3 B-frames, 4 refs, 60-frame GOP | rational broadcast cadence |
| `reorder-vfr-24-30-b3-origin-zero-v1` | VFR 24 → 30 fps | 3 B-frames, 4 refs, 60-frame GOP | variable input cadence |
| `reorder-cfr-30-b3-origin-positive-5s-v1` | CFR 30 fps, +5 s origin | 3 B-frames, 4 refs, 60-frame GOP | positive source origin |
| `reorder-cfr-30-b3-origin-negative-2s-v1` | CFR 30 fps, -2 s origin | 3 B-frames, 4 refs, 60-frame GOP | negative source origin |
| `reorder-cfr-30-b3-long-gop-origin-zero-v1` | CFR 30 fps | 3 B-frames, 4 refs, 300-frame GOP | ten-second GOP behavior |

Every case runs both candidates three times. Startup and Continuation are independently produced and verified.

## Bound base evidence

Each reorder run embeds the complete Encoder Time-Base Candidate run evidence, including:

- normalized Startup and Continuation command hashes;
- complete PTS cadence evidence;
- source-to-output frame mappings;
- exact decoded-pixel frame fingerprints;
- Produced-media Attestation;
- Boundary Packet Evidence;
- A/V Boundary Sync Evidence;
- fail-closed discontinuity policy.

The reorder layer cannot pass by validating DTS alone. Its packet count must equal the corresponding base cadence frame count, and the base candidate run must independently remain valid.

## Presentation-order and packet-order separation

MPEG-TS packets are emitted in decode order when B-frames are enabled. PTS therefore moves backward inside the packet sequence even when the presentation timeline is valid.

The certification pipeline keeps both views:

- PTS cadence evidence is built in presentation order by sorting the reorder-only packet set by PTS;
- packet-order evidence retains the original demux/decode order and validates DTS monotonicity, PTS inversions, composition delay, and reorder depth;
- the generic Output Cadence pipeline keeps its existing behavior and is not globally relaxed.

This prevents a valid B-frame packet sequence from being misclassified as a broken presentation timeline without hiding decode-order defects.

## Packet-order evidence

For Startup and Continuation, the contract records:

- time base and packet count;
- first and last PTS/DTS in ticks and microseconds;
- minimum and maximum composition offset (`PTS - DTS`);
- packets whose PTS is before, equal to, or after DTS;
- reordered packet count;
- adjacent PTS inversion count in decode order;
- duplicate DTS count;
- non-monotonic DTS count;
- maximum presentation reorder depth;
- complete positive DTS-delta histogram;
- complete composition-offset histogram.

The evidence is derived from FFprobe packet output for the generated HLS manifest.

## Exact pixels and perceptual frame identity

Two independently encoded lossy H.264 candidates can reconstruct slightly different pixels even when they retain the same source frame identity and presentation order. The contract therefore keeps two separate gates:

1. **Exact decoded-pixel sequence** — FFmpeg `framemd5` with SHA-256. Every candidate must reproduce the same exact sequence across its three repeats.
2. **Perceptual frame sequence** — every decoded frame is represented by a deterministic 128-bit aHash+dHash signature. Cross-candidate comparison is performed frame by frame at the same presentation index.

The perceptual gate requires:

- equal frame counts;
- one-to-one frame-index comparison;
- maximum Hamming distance no greater than `8/128` bits;
- complete repeated-run stability inside each candidate.

It does not replace exact hashes. Exact hashes remain evidence and remain mandatory for repeat stability. The perceptual sequence only prevents independent lossy encodes from being falsely rejected solely because reconstructed pixels are not byte-identical.

## Required invariants

A run fails when any of the following is true:

- DTS contains a duplicate value;
- DTS decreases or remains equal between adjacent decode-order packets;
- no packet has a non-zero composition offset;
- no adjacent PTS inversion is observed in decode order;
- maximum presentation reorder depth is zero;
- packet-order count differs from base PTS cadence frame count;
- presentation-order PTS contains near-zero, duplicate, or non-monotonic values;
- decoded output contains adjacent duplicate frames;
- exact decoded-pixel sequence changes between repeats of the same candidate;
- perceptual frame sequence changes between repeats of the same candidate;
- candidate perceptual frame distance exceeds the contract threshold;
- Boundary or A/V evidence becomes invalid;
- nested evidence attempts to authorize seamless playback.

## Repeat and comparison policy

Each case/candidate cell runs exactly three times.

A candidate is stable only when:

- base frame, cadence, Boundary, and A/V evidence is stable;
- exact decoded-pixel sequences are stable across repeats;
- perceptual frame sequences are stable across repeats;
- reordered packet counts have zero variance;
- maximum reorder depth has zero variance;
- maximum composition offsets have zero variance;
- complete Startup packet-order evidence is identical across repeats;
- complete Continuation packet-order evidence is identical across repeats;
- DTS is strictly monotonic in all windows;
- B-frame reordering is observed in all windows.

AVTB and 90 kHz must be semantically equivalent for:

- frame mapping and presentation cadence;
- Boundary and A/V evidence;
- Startup and Continuation packet-order evidence;
- Startup and Continuation perceptual frame sequences.

Exact cross-candidate pixel hashes are retained in the report but are not used as the sole semantic-equivalence criterion for independent lossy encodes.

## FFmpeg 6.1.1 exact baseline

The Ubuntu 24.04 reference run uses:

```text
ffmpeg version 6.1.1-3ubuntu5
ffprobe version 6.1.1-3ubuntu5
```

Reference contract hash:

```text
cfc8da4f17a096d0d2cc69ffea474ca5ed72b90c8b11f37387f3810ffcba2961
```

All six cases passed with:

- zero duplicate or non-monotonic DTS packets;
- stable reorder counts, reorder depth, composition offsets, cadence, Boundary, and A/V metrics;
- exact perceptual frame matches for every compared frame;
- maximum observed perceptual Hamming distance `0/128` bits;
- `seamless_allowed = false`;
- `discontinuity_required = true`.

Exact cross-candidate decoded pixels differed in seven windows:

```text
reorder-cfr-30000-1001-b3-origin-zero-v1 / Startup
reorder-cfr-30-b3-origin-positive-5s-v1 / Startup
reorder-cfr-30-b3-origin-positive-5s-v1 / Continuation
reorder-cfr-30-b3-origin-negative-2s-v1 / Startup
reorder-cfr-30-b3-origin-negative-2s-v1 / Continuation
reorder-cfr-30-b3-long-gop-origin-zero-v1 / Startup
reorder-cfr-30-b3-long-gop-origin-zero-v1 / Continuation
```

Those windows still had frame-for-frame identical perceptual signatures, identical PTS cadence, identical DTS packet-order evidence, and identical Boundary/A/V evidence. The exact differences remain visible in the base comparison rather than being discarded.

## CLI

List registered cases and candidates:

```bash
go run ./cmd/transcode-encoder-time-base-reorder-cert -list
```

Run the complete matrix:

```bash
go run ./cmd/transcode-encoder-time-base-reorder-cert \
  -output encoder-time-base-reorder-matrix-v1.json
```

Keep the diagnostic workspace:

```bash
go run ./cmd/transcode-encoder-time-base-reorder-cert \
  -work-dir /tmp/nowen-encoder-time-base-reorder \
  -keep-work-dir \
  -output /tmp/nowen-encoder-time-base-reorder/encoder-time-base-reorder-matrix-v1.json
```

## CI

Dedicated workflow:

```text
.github/workflows/transcode-encoder-time-base-reorder-cert.yml
```

Cross-version semantic verifier:

```text
.github/scripts/verify_encoder_time_base_reorder.py
```

Ubuntu 24.04 / FFmpeg 6.1.1 exact baseline verifier:

```text
.github/scripts/verify_encoder_time_base_reorder_baseline.py
```

The workflow:

1. runs the Go reorder, base candidate, output cadence, and certification tests;
2. builds the standalone certification command;
3. produces all six cases, two candidates, and three repeats;
4. verifies strict DTS and observed B-frame reordering;
5. verifies packet-order histogram arithmetic;
6. verifies presentation cadence and per-frame perceptual identity;
7. verifies exact per-candidate repeat stability;
8. verifies Boundary and A/V fail-closed policy;
9. runs the FFmpeg 6.1.1 exact numerical baseline lock;
10. uploads `encoder-time-base-reorder-matrix-v1.json` as a CI artifact.

## Production non-claims

This phase does not prove:

- representative real-file demuxer behavior;
- hardware encoder reorder behavior;
- HEVC, AV1, interlaced, HDR, edit-list, or discontinuous-source behavior;
- open-GOP or adaptive B-frame behavior;
- long-duration DTS or A/V drift behavior;
- Web, PC, Android, Emby, or Infuse playback acceptance;
- safe seamless Startup-to-Continuation handoff;
- safe removal of `#EXT-X-DISCONTINUITY`;
- that either candidate should be enabled in production.

## Next production gate

The next gate is a separately versioned **Real Media Corpus v1**. It must not overload the existing synthetic `FixtureSpec`. The corpus needs its own immutable source metadata for container, codecs, CFR/VFR policy, timestamp origin, edit-list behavior, GOP/reorder structure, audio layout, color metadata, and source digest.

It must bind real demuxer and container behavior to:

- Timestamp Plan;
- Encoder Time-Base Candidate Evidence;
- Encoder Time-Base Reorder Evidence;
- Produced-media Attestation;
- Boundary Packet Evidence;
- A/V Boundary Sync Evidence.

Production selection remains deferred until real-media, hardware, long-duration, client-acceptance, rollout, fallback, and rollback gates all pass.
