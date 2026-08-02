# Long-Duration Drift Certification

## Status

Long-Duration Drift Certification v1 is implemented and passing on `refactor/server-lite-v1`.

The certification extends the real-media candidate evidence from short Startup and Continuation windows to a continuous 30-minute HLS output. It compares both explicit encoder time-base candidates:

```text
encoder-time-base-avtb-v1  -> 1/1000000
encoder-time-base-90k-v1   -> 1/90000
```

This phase does **not** enable seamless playback, remove playlist discontinuities, or change the production encoder time base.

Production remains fail-closed:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Execution matrix

The v1 matrix intentionally uses one timing-sensitive real-media source:

```text
real-mp4-h264-aac-cfr-30-aac-44100-v1
```

The source combines:

- MP4 container;
- H.264 video at 30 fps;
- three B-frames and closed GOP output;
- AAC audio at 44.1 kHz;
- a sample rate that does not divide evenly into the 90 kHz transport clock.

The complete matrix is:

```text
1 immutable source asset
x 2 encoder time-base candidates
x 2 independent repeats
x 30 minutes per execution
= 4 executions
= 120 encoded minutes
```

Every execution produces a complete 30-minute HLS VOD with approximately 900 two-second segments. The 40-second immutable source is expanded through the FFmpeg input loop; the output remains a single continuous transcode rather than concatenated reports or synthetic timestamp arithmetic.

## Checkpoint model

Video and audio are checked at seven relative presentation-time checkpoints:

```text
00:00
05:00
10:00
15:00
20:00
25:00
30:00
```

The evidence records, independently for video and audio:

- raw first-packet timestamp;
- raw final packet end timestamp;
- relative stream duration;
- final duration error against 30 minutes;
- nearest presentation timestamp at every checkpoint;
- checkpoint error;
- packet count and stream time base.

## Raw mux origin versus accumulated drift

MPEG-TS/HLS output can retain a deterministic positive mux origin of approximately 1.4 seconds. That origin is not accumulated drift.

The certification therefore separates two concerns:

1. the raw first-packet timestamp remains recorded and must satisfy the canonical Timestamp Plan origin window;
2. duration and checkpoint drift are calculated relative to each stream's first packet.

This prevents a legal mux origin from being misreported as 1.4 seconds of long-duration drift while retaining the original packet timing for diagnostics.

The bound Timestamp Plan is:

```text
version = hls-timestamp-normalization-v1
hash    = 0648217f7c10a055d84c6005c497f328ff02606119195e98dfe76fcae33dd937
```

## Evidence contract

Schema:

```text
long-duration-drift-evidence-v1
```

Report schema:

```text
ffmpeg-long-duration-drift-matrix-v1
```

The contract binds:

1. Real Media Corpus Spec version and SHA-256;
2. Corpus Manifest version and SHA-256;
3. source generator, FFmpeg, and FFprobe identities;
4. certification FFmpeg and FFprobe identities;
5. Timestamp Plan version and SHA-256;
6. immutable source path, file SHA-256, byte size, and canonical asset evidence SHA-256;
7. each normalized FFmpeg command SHA-256;
8. each generated playlist SHA-256;
9. Produced-media Attestation version and SHA-256;
10. segment count, stream packet evidence, checkpoints, summaries, and cross-candidate comparison.

The production policy fields are part of the canonical contract identity and cannot be omitted or changed without invalidating the report.

## Acceptance policy

The v1 limits are explicit:

```text
raw first-packet origin tolerance = 3,000,000 us
stream end-error tolerance        =    50,000 us
checkpoint-error tolerance        =    50,000 us
final A/V skew tolerance          =    50,000 us
repeat variance tolerance         =     2,000 us
cross-candidate tolerance         =     2,000 us
```

Both repeats of a candidate must satisfy every absolute limit. The candidate summary must also remain within the repeat-variance limit.

AVTB and 90 kHz are considered equivalent only when corresponding repeat metrics remain within the cross-candidate limit for:

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

Observed AVTB summary:

```text
repeats                                = 2
maximum absolute video end error       = 15,501 us
maximum absolute audio end error       = 24,800 us
maximum absolute final A/V skew        = 16,255 us
maximum absolute checkpoint error      = 24,800 us
stable                                 = true
```

Observed 90 kHz summary:

```text
repeats                                = 2
maximum absolute video end error       = 15,501 us
maximum absolute audio end error       = 24,800 us
maximum absolute final A/V skew        = 16,255 us
maximum absolute checkpoint error      = 24,800 us
stable                                 = true
```

Cross-candidate result:

```text
candidates_equivalent                  = true
maximum checkpoint difference          = 0 us
```

Canonical contract identity:

```text
763c1e269740f45619089b95c833e0e7abdd9420a544b7d1849e5aea27ffbf55
```

The generated JSON report is approximately 40 KB.

## Commands

List the profile, checkpoints, and candidates:

```bash
go run ./cmd/transcode-long-duration-drift-cert -list
```

Run certification against an existing corpus:

```bash
go run ./cmd/transcode-long-duration-drift-cert \
  -corpus-root /tmp/nowen-transcode-real-media-corpus-v1 \
  -output /tmp/long-duration-drift-v1.json
```

The command refuses missing, escaping, symlinked, size-mismatched, or SHA-mismatched corpus assets through the existing real-media corpus loader.

## CI

Workflow:

```text
.github/workflows/transcode-long-duration-drift-cert.yml
```

Independent verifier:

```text
.github/scripts/verify_long_duration_drift.py
```

The workflow has two isolated jobs:

1. compile contracts and commands, generate the deterministic corpus, verify its Manifest and media bytes, and upload an immutable corpus artifact;
2. download that exact artifact, execute the four long-duration transcodes, build the canonical evidence graph, independently recompute the report policy, and upload the final report.

Successful run:

```text
workflow run ID = 30736241467
corpus artifact = 8829682017
report artifact = 8829787189
retention       = 7 days
```

The long-duration execution and verification step completed in approximately seven minutes on the hosted runner.

## What this phase proves

This phase proves that, for the selected H.264/AAC 44.1 kHz real-media profile under deterministic software encoding:

- neither candidate accumulated material video-duration drift over 30 minutes;
- neither candidate accumulated material audio-duration drift over 30 minutes;
- final A/V skew remained within the declared limit;
- all intermediate five-minute checkpoints remained within the declared limit;
- both candidates were repeat-stable;
- AVTB and 90 kHz produced identical measured drift metrics.

## Remaining gates

Long-Duration Drift Certification v1 does not complete:

- two-hour to six-hour duration coverage;
- VFR, Opus, edit-list, and MPEG-TS long-duration profiles;
- discontinuous or damaged source timelines;
- multi-audio, subtitles, interlaced, HEVC, AV1, HDR, or Dolby Vision media;
- hardware encoder certification;
- memory, disk, thermal, and cancellation stress on NAS-class devices;
- Web, PC, Android, Emby, or Infuse playback acceptance;
- production rollout, fallback, rollback, or time-base selection authorization.

No evidence in this phase independently permits removal of `#EXT-X-DISCONTINUITY` or a production encoder time-base policy change.
