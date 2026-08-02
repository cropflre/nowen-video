# Real Media Corpus Candidate Certification

## Status

Real Media Corpus Candidate Certification is implemented and passing on `refactor/server-lite-v1`.

The certification consumes the immutable files produced by Real Media Corpus v1 and executes both encoder time-base candidates through the existing production-shaped HLS pipeline:

```text
encoder-time-base-avtb-v1  -> 1/1000000
encoder-time-base-90k-v1   -> 1/90000
```

This evidence does **not** enable seamless playback, remove playlist discontinuities, or change the production encoder time base.

Production remains fail-closed:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Execution matrix

The matrix contains:

```text
6 immutable source assets
x 2 encoder time-base candidates
x 3 independent repeats
x 2 output windows: Startup and Continuation
= 72 HLS transcode executions
```

Every execution uses the file bytes identified by:

```text
case_id + source_file_sha256
```

The candidate job does not regenerate media. It downloads the corpus artifact emitted by the previous CI job and re-verifies every path, file size, and SHA-256 before certification.

## Covered media

| Case | Source | Reorder depth observed |
|---|---|---:|
| `real-mp4-h264-aac-cfr-24000-1001-v1` | MP4, H.264/AAC, CFR 24000/1001 | 2 |
| `real-mp4-h264-aac-cfr-30000-1001-edit-list-v1` | MP4, H.264/AAC, CFR 30000/1001, edit list, positive origin | 3 |
| `real-mkv-h264-aac-vfr-24-30-v1` | Matroska, H.264/AAC, VFR 24 to 30 | 3 |
| `real-mpegts-h264-aac-cfr-30-b3-v1` | MPEG-TS, H.264/AAC, CFR 30, positive transport origin | 3 |
| `real-mkv-h264-opus-cfr-25-v1` | Matroska, H.264/Opus, CFR 25 | 2 |
| `real-mp4-h264-aac-cfr-30-aac-44100-v1` | MP4, H.264/AAC 44.1 kHz, CFR 30 | 3 |

Both candidates were stable across all three repeats for Startup and Continuation, and both candidates were semantically equivalent for all six cases.

## Evidence graph

Schema:

```text
real-media-corpus-candidate-evidence-v1
```

Each case binds the following identities:

1. corpus Spec version and SHA-256;
2. corpus Manifest version and SHA-256;
3. canonical Manifest asset evidence SHA-256;
4. immutable media file SHA-256 and byte size;
5. Timestamp Plan version and SHA-256;
6. semantic time-base case version and SHA-256;
7. reorder candidate case version and SHA-256;
8. produced-media attestation identities for Startup and Continuation;
9. Boundary Packet and A/V Boundary Sync identities.

The semantic time-base sub-contract is:

```text
encoder-time-base-semantic-candidate-evidence-v1
```

The historical exact candidate contract remains unchanged. Existing synthetic evidence still requires exact decoded frame sequence identity. The real-media semantic contract requires equivalent:

- input/output frame mapping;
- output cadence;
- A/V boundary sync;
- decoded perceptual frame sequence;
- packet-order semantics.

Raw decoded frame hashes remain in the report for diagnostics, but codec quantization differences do not masquerade as timeline or semantic failure.

## Presentation order and decode order

B-frame sources expose two valid orders:

- cadence and frame mapping are evaluated in **presentation order**, sorted by PTS;
- packet reorder is evaluated in original demux/decode order, preserving DTS and composition offsets.

The packet gate still requires:

```text
DTS non-monotonic count = 0
DTS duplicate count     = 0
reordered packet count  > 0
PTS inversion count     > 0
reorder depth           > 0
```

This prevents legal B-frame PTS reordering from being misclassified as a broken cadence while preserving strict decode-order evidence.

## One-tick packet quantization policy

The two candidates may quantize a packet timestamp to adjacent ticks in a 90 kHz stream. Real-media cross-candidate packet comparison therefore declares:

```text
packet_order_comparison_tolerance_ticks = 1
```

The tolerance is real-media-specific. The existing synthetic Reorder contract continues to use zero-tick equality.

Within the one-tick policy, the following must remain exactly equal:

- packet count;
- reordered packet count;
- PTS-before-DTS, PTS-after-DTS, and PTS-equals-DTS counts;
- adjacent PTS inversion count;
- DTS monotonicity and duplicate counts;
- maximum presentation reorder depth;
- decoded perceptual frame sequence.

Only corresponding timestamp values may differ by at most one tick. Independent Python verification expands the DTS-delta and composition-offset histograms sample by sample and enforces the same limit.

The successful CI run observed:

```text
maximum_observed_difference_ticks = 1
packet_comparisons                 = 36
```

## Decoded frame comparison policy

The report explicitly records:

```text
decoded_frame_comparison_policy = perceptual_frame_sequence_v1
```

This policy requires the same decoded frame count and perceptual sequence across candidates and repeats. It does not treat an encoder-time-base-dependent pixel-level quantization difference as a timeline decision.

## Commands

List cases and candidates:

```bash
go run ./cmd/transcode-real-media-candidate-cert -list
```

Run certification against an existing corpus:

```bash
go run ./cmd/transcode-real-media-candidate-cert \
  -corpus-root /tmp/nowen-transcode-real-media-corpus-v1 \
  -output /tmp/real-media-corpus-candidate-v1.json
```

The command refuses missing, escaping, symlinked, size-mismatched, or SHA-mismatched assets.

## CI

Workflow:

```text
.github/workflows/transcode-real-media-corpus-generate.yml
```

Independent verifiers:

```text
.github/scripts/verify_real_media_candidate.py
.github/scripts/verify_real_media_packet_tolerance.py
```

The workflow is split into two jobs:

1. compile contracts and commands, generate the corpus twice, verify Manifest and media bytes, upload the corpus artifact;
2. download that exact artifact, run the 72-execution candidate matrix, independently recompute the evidence graph and packet tolerance, upload the candidate report.

Successful evidence from Ubuntu 24.04 and FFmpeg 6.1.1:

```text
cases          = 6
candidates     = 2
repeats        = 3
report size    = 3.5 MB
manifest hash  = de2ef70ce71b31f25f53de33b359f3967ca52e2ac3f6d7ead4b64b9724e29075
contract hash  = efa92b220f487427f3dcdb86605dded1ad5e2390180d005b53db8b611039aab0
```

The CI artifact is retained for seven days.

## Remaining gates

This phase proves deterministic software-encoder equivalence for the current six-case real-media registry. It does not complete:

- curated external media provenance and licensing;
- long-duration drift certification;
- hardware encoder certification;
- HEVC, AV1, HDR, interlaced, subtitles, multi-audio, or discontinuous-source cases;
- Web, PC, Android, Emby, or Infuse playback acceptance;
- production rollout, fallback, or rollback authorization.

No evidence in this phase independently permits removal of `#EXT-X-DISCONTINUITY` or a production time-base policy change.
