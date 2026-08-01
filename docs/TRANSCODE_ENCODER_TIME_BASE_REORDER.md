# Encoder Time-Base Reorder Certification

## Status

This phase certifies deterministic B-frame decode and presentation ordering for the two explicit encoder time-base candidates:

```text
encoder-time-base-avtb-v1 = 1/1000000
encoder-time-base-90k-v1  = 1/90000
```

It is evidence-only. It does not change the production FFmpeg builder, persisted jobs, Startup/Continuation execution, HLS playlists, playback planning, or client behavior.

Production remains fail-closed:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Why this phase exists

The base Encoder Time-Base Candidate matrix proves presentation cadence, frame mapping, decoded-frame stability, Boundary evidence, and A/V evidence for the software zero-latency profile. Zero latency does not exercise B-frame reordering, where packets are decoded in DTS order and displayed in PTS order.

Reorder certification removes the zero-latency tune only inside the certification command and adds deterministic closed-GOP B-frame policies. Every reorder run embeds and validates the complete base candidate evidence.

## Schemas

```text
encoder-time-base-reorder-evidence-v1
ffmpeg-encoder-time-base-reorder-matrix-v1
```

Canonical contract JSON is identified by SHA-256.

## Deterministic encoder policy

```text
b-adapt = 0
b-pyramid = none
open-gop = 0
scenecut = 0
```

Every case explicitly binds B-frame count, reference-frame count, GOP size, source cadence, timestamp origin, and the Startup/Continuation boundary. Adaptive B-frame placement and open GOPs are outside v1.

## Matrix

Every source is 40 seconds long with a 30-second boundary and 48 kHz audio.

| Case | Source | Reorder policy |
|---|---|---|
| `reorder-cfr-24-b2-origin-zero-v1` | CFR 24 fps | B2, 3 refs, GOP 48 |
| `reorder-cfr-30000-1001-b3-origin-zero-v1` | CFR 30000/1001 | B3, 4 refs, GOP 60 |
| `reorder-vfr-24-30-b3-origin-zero-v1` | VFR 24 → 30 | B3, 4 refs, GOP 60 |
| `reorder-cfr-30-b3-origin-positive-5s-v1` | CFR 30, +5 s origin | B3, 4 refs, GOP 60 |
| `reorder-cfr-30-b3-origin-negative-2s-v1` | CFR 30, -2 s origin | B3, 4 refs, GOP 60 |
| `reorder-cfr-30-b3-long-gop-origin-zero-v1` | CFR 30 | B3, 4 refs, GOP 300 |

Each case runs both candidates three times. Startup and Continuation are produced and verified independently.

## Bound base evidence

Every run retains:

- normalized Startup and Continuation command hashes;
- presentation cadence and source-to-output mappings;
- exact decoded-pixel frame fingerprints;
- Produced-media Attestation;
- Boundary Packet Evidence;
- A/V Boundary Sync Evidence;
- fail-closed discontinuity policy.

The reorder layer cannot pass by validating DTS alone. Packet counts must equal base cadence frame counts and the nested base evidence must remain valid.

## Presentation order versus decode order

MPEG-TS packets with B-frames are emitted in decode order. PTS therefore moves backward inside the packet sequence even when the presentation timeline is valid.

The certification pipeline keeps both views:

- reorder-only cadence evidence is sorted by PTS and validates presentation order;
- packet-order evidence retains original demux order and validates DTS, PTS inversions, composition delay, and reorder depth;
- generic Output Cadence behavior is unchanged.

## Packet-order evidence

Startup and Continuation record:

- packet count and time base;
- first and last PTS/DTS;
- duplicate and non-monotonic DTS counts;
- PTS-before/equal/after-DTS counts;
- adjacent PTS inversion count;
- reordered packet count;
- maximum presentation reorder depth;
- minimum and maximum `PTS - DTS`;
- complete DTS-delta and composition-offset histograms.

A run fails if DTS is not strictly increasing, no B-frame reorder is observed, packet counts differ from cadence evidence, or any Boundary/A/V/fail-closed invariant fails.

## Exact pixels and perceptual frame identity

Independent lossy H.264 encodes can reconstruct slightly different pixels while preserving the same source frame identity and order. Two separate gates are therefore retained:

1. **Exact decoded-pixel sequence** — FFmpeg `framemd5` with SHA-256. Each candidate must reproduce the same exact sequence across its three repeats.
2. **Perceptual frame sequence** — each decoded frame has a deterministic 128-bit aHash+dHash signature and candidates are compared frame by frame at the same presentation index.

The perceptual gate requires equal frame counts, repeat-stable per-frame signatures, and a maximum Hamming distance of at most `8/128` bits. It does not replace exact hashes; it only prevents independent lossy encodes from being rejected solely because reconstructed pixels are not byte-identical.

## Repeat and candidate comparison

A candidate is stable only when exact pixel sequences, perceptual sequences, cadence, frame mapping, packet order, reorder depth, composition offsets, Boundary, and A/V evidence are stable across all three repeats.

AVTB and 90 kHz must be semantically equivalent for:

- frame mapping and presentation cadence;
- Boundary and A/V evidence;
- Startup and Continuation packet order;
- Startup and Continuation perceptual frame sequence.

Exact cross-candidate pixel hashes remain visible evidence but are not the sole semantic-equivalence criterion for two independent lossy encodes.

## FFmpeg 6.1.1 exact semantic baseline

Reference toolchain:

```text
ffmpeg version 6.1.1-3ubuntu5
ffprobe version 6.1.1-3ubuntu5
```

Reference contract hash:

```text
cfc8da4f17a096d0d2cc69ffea474ca5ed72b90c8b11f37387f3810ffcba2961
```

The exact verifier locks:

- the six-case registry and toolchain identity;
- frame counts and dominant cadence;
- video/audio boundary deltas and A/V skew metrics;
- reorder counts, reorder depth, and maximum composition offsets;
- exact-pixel equivalence classifications;
- perceptual maximum distances;
- a canonical semantic SHA-256 for every case, candidate, and repeat.

All reference perceptual comparisons have maximum Hamming distance `0/128`.

Exact cross-candidate decoded pixels differ in seven windows:

```text
reorder-cfr-30000-1001-b3-origin-zero-v1 / Startup
reorder-cfr-30-b3-origin-positive-5s-v1 / Startup
reorder-cfr-30-b3-origin-positive-5s-v1 / Continuation
reorder-cfr-30-b3-origin-negative-2s-v1 / Startup
reorder-cfr-30-b3-origin-negative-2s-v1 / Continuation
reorder-cfr-30-b3-long-gop-origin-zero-v1 / Startup
reorder-cfr-30-b3-long-gop-origin-zero-v1 / Continuation
```

Those windows still have frame-for-frame identical perceptual signatures, identical presentation cadence, identical DTS packet evidence, and identical Boundary/A/V evidence.

## CLI

```bash
go run ./cmd/transcode-encoder-time-base-reorder-cert -list

go run ./cmd/transcode-encoder-time-base-reorder-cert \
  -output encoder-time-base-reorder-matrix-v1.json
```

Keep diagnostics:

```bash
go run ./cmd/transcode-encoder-time-base-reorder-cert \
  -work-dir /tmp/nowen-encoder-time-base-reorder \
  -keep-work-dir \
  -output /tmp/nowen-encoder-time-base-reorder/encoder-time-base-reorder-matrix-v1.json
```

## CI

Workflow:

```text
.github/workflows/transcode-encoder-time-base-reorder-cert.yml
```

Cross-version semantic verifier:

```text
.github/scripts/verify_encoder_time_base_reorder.py
```

Ubuntu 24.04 / FFmpeg 6.1.1 exact semantic verifier:

```text
.github/scripts/verify_encoder_time_base_reorder_exact.py
```

The workflow runs Go contract tests, builds the standalone command, produces the complete matrix, verifies semantic invariants, verifies the exact FFmpeg 6.1.1 baseline, and uploads the matrix artifact.

## Production non-claims

This phase does not prove representative real-file demuxer behavior, hardware encoders, HEVC/AV1/HDR/interlaced/edit-list behavior, open-GOP/adaptive-B behavior, long-duration drift, client acceptance, safe seamless handoff, or safe removal of `#EXT-X-DISCONTINUITY`.

Neither candidate is enabled in production by this evidence.

## Next gate

The next gate is the separately versioned **Real Media Corpus v1**. It owns immutable container, codec, cadence, timestamp-origin, edit-list, GOP/reorder, audio, color, and source-digest metadata rather than expanding the historical synthetic `FixtureSpec`.

It must bind real file demuxer behavior to Timestamp Plan, candidate/reorder evidence, Produced-media Attestation, Boundary Packet Evidence, and A/V Boundary Sync Evidence. Hardware, long-duration, client-acceptance, rollout, fallback, and rollback gates remain required before production selection.
