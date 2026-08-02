# Long-Duration Profile Matrix Certification

## Status

Long-Duration Profile Matrix Certification v2 is implemented and passing on `refactor/server-lite-v1`.

This phase extends the single-profile Long-Duration Drift Certification v1 into a profile-scoped matrix covering container, cadence, source-origin, edit-list, audio-codec, and audio-sample-rate differences.

The v1 schema and workflow remain intact and independently reproducible. The v2 matrix is additive; it does not rewrite historical v1 evidence.

This phase does **not** enable seamless playback, remove playlist discontinuities, or change the production encoder time base.

Production remains fail-closed:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Profile registry

The v2 contract binds five profiles to exact Real Media Corpus cases.

### 1. MP4 AAC 44.1 kHz CFR

```text
profile ID     = profile-mp4-aac-44100-cfr-v1
source case    = real-mp4-h264-aac-cfr-30-aac-44100-v1
container      = MP4
video cadence  = CFR 30 fps
B-frames       = 3
audio          = AAC 44.1 kHz stereo
source origin  = 0 us
edit list      = false
```

Purpose: preserve the original v1 stress case where 44.1 kHz audio does not divide evenly into the 90 kHz transport clock.

### 2. MP4 positive-origin edit list

```text
profile ID     = profile-mp4-edit-list-v1
source case    = real-mp4-h264-aac-cfr-30000-1001-edit-list-v1
container      = MP4
video cadence  = CFR 30000/1001 fps
B-frames       = 3
audio          = AAC 48 kHz stereo
source origin  = 5,000,000 us
edit list      = true
```

Purpose: certify that a positive MP4 source origin and edit-list metadata do not create accumulated output drift after canonical timestamp normalization.

### 3. Matroska VFR 24 to 30 fps

```text
profile ID     = profile-mkv-vfr-24-30-v1
source case    = real-mkv-h264-aac-vfr-24-30-v1
container      = Matroska
video cadence  = VFR 24 -> 30 fps
B-frames       = 3
audio          = AAC 48 kHz stereo
source origin  = 0 us
edit list      = false
```

Purpose: certify a file-backed variable-cadence timeline without MP4 edit-list semantics.

### 4. MPEG-TS positive transport origin

```text
profile ID     = profile-mpegts-positive-origin-v1
source case    = real-mpegts-h264-aac-cfr-30-b3-v1
container      = MPEG-TS
video cadence  = CFR 30 fps
B-frames       = 3
audio          = AAC 48 kHz stereo
source origin  = 1,400,000 us
edit list      = false
```

Purpose: certify transport-stream packet timestamps and positive source-origin handling before HLS re-encoding.

### 5. Matroska Opus

```text
profile ID     = profile-mkv-opus-v1
source case    = real-mkv-h264-opus-cfr-25-v1
container      = Matroska
video cadence  = CFR 25 fps
B-frames       = 2
audio          = Opus 48 kHz stereo
source origin  = 0 us
edit list      = false
```

Purpose: certify a non-AAC input codec and Matroska audio time base through the production-shaped AAC HLS output path.

## Execution matrix

Each profile executes both explicit encoder time-base candidates:

```text
encoder-time-base-avtb-v1  -> 1/1000000
encoder-time-base-90k-v1   -> 1/90000
```

Each candidate executes twice against the same immutable source bytes:

```text
5 profiles
x 2 encoder time-base candidates
x 2 independent repeats
x 30 minutes per execution
= 20 executions
= 600 encoded minutes
```

Each execution produces a complete 30-minute HLS VOD with approximately 900 two-second segments. Across the full matrix this is approximately 18,000 media segments.

The 40-second immutable sources are expanded through FFmpeg input-level looping. Each result is one continuous transcode timeline rather than concatenated reports or synthetic timestamp arithmetic.

## Reused execution path

The v2 runner does not implement a second FFmpeg policy.

It reuses the v1 long-duration execution path for:

- production-shaped `BuildHLSArgs` generation;
- input-scoped `-stream_loop -1`;
- explicit encoder time base;
- deterministic B-frame policy;
- 180p software encoding;
- two-second HLS segmentation;
- canonical Timestamp Plan application;
- complete Produced-media Attestation;
- video and audio packet probing;
- stream-relative duration calculation;
- seven checkpoint calculations;
- repeat stability and cross-candidate comparison.

The v2 layer only adds profile identity, profile ordering, source-trait binding, and a root profile matrix contract.

## Checkpoint model

Every video and audio stream is checked at seven relative presentation-time checkpoints:

```text
00:00
05:00
10:00
15:00
20:00
25:00
30:00
```

The evidence records:

- raw first-packet timestamp;
- raw final packet end timestamp;
- relative stream duration;
- final duration error against 30 minutes;
- nearest presentation timestamp at every checkpoint;
- checkpoint error;
- packet count and stream time base;
- final A/V skew.

Raw mux origin and accumulated drift remain separate. The raw first-packet timestamp is preserved and checked against the Timestamp Plan, while duration and checkpoints are calculated relative to each stream's first packet.

Bound Timestamp Plan:

```text
version = hls-timestamp-normalization-v1
hash    = 0648217f7c10a055d84c6005c497f328ff02606119195e98dfe76fcae33dd937
```

## Evidence contract

Contract schema:

```text
long-duration-profile-matrix-evidence-v2
```

Report schema:

```text
ffmpeg-long-duration-profile-matrix-v2
```

The root contract binds:

1. Real Media Corpus Spec version and SHA-256;
2. Corpus Manifest version and SHA-256;
3. source generator, FFmpeg, and FFprobe identities;
4. certification FFmpeg and FFprobe identities;
5. Timestamp Plan version and SHA-256;
6. canonical profile order and exact source traits;
7. each source path, SHA-256, size, and canonical asset evidence hash;
8. each normalized FFmpeg command SHA-256;
9. each generated playlist SHA-256;
10. each Produced-media Attestation identity;
11. segment counts, stream packet evidence, checkpoints, summaries, and per-profile candidate comparisons;
12. fail-closed production policy.

### Profile-scoped comparison

Candidates are compared only within the same profile.

A VFR profile is never numerically compared to an Opus or edit-list profile. This prevents legitimate container or codec behavior from being flattened into one global average.

The root contract succeeds only when all five profile contracts are stable and all five AVTB/90 kHz comparisons are equivalent.

### Run-bound identity

The Contract SHA-256 is intentionally bound to the concrete outputs of one certification run. Playlist and Produced-media Attestation hashes are part of the evidence graph, so later successful runs can have different Contract hashes while retaining identical policy and measured drift metrics.

A Contract SHA must be interpreted together with its workflow run and artifacts.

## Acceptance policy

The v2 matrix retains the explicit v1 limits:

```text
raw first-packet origin tolerance = 3,000,000 us
stream end-error tolerance        =    50,000 us
checkpoint-error tolerance        =    50,000 us
final A/V skew tolerance          =    50,000 us
repeat variance tolerance         =     2,000 us
cross-candidate tolerance         =     2,000 us
```

Both repeats of every candidate must satisfy every absolute limit. Each candidate must remain within the repeat-variance limit.

AVTB and 90 kHz are equivalent within a profile only when corresponding repeat metrics remain within the cross-candidate limit for:

- video final duration error;
- audio final duration error;
- final A/V skew;
- every video and audio checkpoint error.

## Successful CI evidence

Environment:

```text
Ubuntu 24.04.4
Go 1.25.0
FFmpeg / FFprobe 6.1.1-3ubuntu5
```

### MP4 AAC 44.1 kHz CFR

Both AVTB and 90 kHz produced:

```text
maximum video end error       = 15,501 us
maximum audio end error       = 24,800 us
maximum final A/V skew        = 16,255 us
maximum checkpoint error      = 24,800 us
stable                        = true
candidate checkpoint delta    = 0 us
candidates equivalent         = true
```

### MP4 edit list

Both AVTB and 90 kHz produced:

```text
maximum video end error       = 11,034 us
maximum audio end error       = 21,333 us
maximum final A/V skew        = 33,034 us
maximum checkpoint error      = 21,333 us
stable                        = true
candidate checkpoint delta    = 0 us
candidates equivalent         = true
```

### Matroska VFR

Both AVTB and 90 kHz produced:

```text
maximum video end error       =  1,333 us
maximum audio end error       = 27,833 us
maximum final A/V skew        = 13,167 us
maximum checkpoint error      = 27,833 us
stable                        = true
candidate checkpoint delta    = 0 us
candidates equivalent         = true
```

### MPEG-TS positive origin

Both AVTB and 90 kHz produced:

```text
maximum video end error       =  5,333 us
maximum audio end error       = 21,333 us
maximum final A/V skew        = 26,667 us
maximum checkpoint error      = 21,333 us
stable                        = true
candidate checkpoint delta    = 0 us
candidates equivalent         = true
```

### Matroska Opus

Both AVTB and 90 kHz produced:

```text
maximum video end error       = 32,000 us
maximum audio end error       = 23,833 us
maximum final A/V skew        = 29,500 us
maximum checkpoint error      = 32,000 us
stable                        = true
candidate checkpoint delta    = 0 us
candidates equivalent         = true
```

### Matrix summary

```text
profiles                              = 5
executions                            = 20
encoded minutes                       = 600
checkpoints per stream                = 7
maximum observed stream end error     = 32,000 us
maximum cross-candidate checkpoint    = 0 us
all candidates stable                 = true
all profile comparisons equivalent    = true
```

Reference-run Contract identity:

```text
workflow run = 30738228052
contract SHA = 58987d8dbbf1b89045e4ea56c82f3b826663632c62072bde42bd5f84f2d9d806
```

The generated JSON report is approximately 116 KB.

## Commands

List all profiles, checkpoints, and candidates:

```bash
go run ./cmd/transcode-long-duration-profile-cert -list
```

Run the complete matrix against an existing corpus:

```bash
go run ./cmd/transcode-long-duration-profile-cert \
  -corpus-root /tmp/nowen-transcode-real-media-corpus-v1 \
  -output /tmp/long-duration-profile-matrix-v2.json
```

The command reuses the strict real-media corpus loader and rejects missing, escaping, symlinked, size-mismatched, or SHA-mismatched source assets.

## CI

Workflow:

```text
.github/workflows/transcode-long-duration-profile-cert.yml
```

Independent verifier:

```text
.github/scripts/verify_long_duration_profile_matrix.py
```

The workflow has two isolated jobs:

1. compile profile contracts and commands, generate the deterministic corpus, verify its Manifest and media bytes, and upload an immutable corpus artifact;
2. download that exact artifact, execute all twenty long-duration transcodes, build the profile-scoped evidence graph, independently recompute every policy result, and upload the final report.

Reference successful run:

```text
workflow run ID         = 30738228052
corpus artifact         = 8830408255
corpus artifact SHA-256 = 79079d9ada27565212693cdfb12737b216b151c94d0bec46588b78e19e715e2f
report artifact         = 8830798770
report artifact SHA-256 = 008b726456c59272486e03a3869d8756fb2c805f82ddb251e7074c432c7a3404
retention               = 7 days
```

The twenty executions, packet scans, contract construction, and independent verification completed in approximately 34 minutes on the hosted runner.

## What this phase proves

For the selected deterministic software-encoding profiles, this phase proves that:

- 44.1 kHz AAC input remains stable over 30 minutes;
- MP4 positive origin and edit-list metadata do not accumulate material output drift;
- the VFR 24-to-30 fps source remains inside the declared checkpoint and end-error limits;
- MPEG-TS positive transport origin is normalized without accumulated drift;
- Matroska Opus input remains stable through the AAC HLS output path;
- each profile is repeat-stable;
- AVTB and 90 kHz produce identical measured drift metrics for every profile;
- no profile exceeded the 50 ms end, checkpoint, or final A/V skew limits.

## Remaining gates

Long-Duration Profile Matrix Certification v2 does not complete:

- two-hour to six-hour duration coverage;
- 24000/1001 long-duration coverage;
- discontinuous, damaged, or timestamp-wrapping source timelines;
- multi-audio and audio-language switching;
- subtitle burn-in, passthrough, and external subtitle timelines;
- interlaced, HEVC, AV1, HDR, or Dolby Vision media;
- hardware encoder certification;
- memory, disk, thermal, cancellation, and recovery stress on NAS-class devices;
- Web, PC, Android, Emby, or Infuse playback acceptance;
- production rollout, fallback, rollback, or encoder time-base selection authorization.

No evidence in this phase independently permits removal of `#EXT-X-DISCONTINUITY` or a production encoder time-base policy change.
