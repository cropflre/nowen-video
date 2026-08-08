# Multi-Hour Duration Scaling Certification

## Status

Multi-Hour Duration Scaling Certification v3 is implemented and passing on `refactor/server-lite-v1`.

This phase extends the 30-minute Long-Duration Drift v1 and Long-Duration Profile Matrix v2 evidence into bounded two-hour and six-hour tiers. The historical v1 and v2 schemas, commands, reports, and workflows remain independently reproducible.

This phase does **not** enable seamless playback, remove playlist discontinuities, or select a production encoder time base.

Production remains fail-closed:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Why the matrix is sharded

Serially multiplying five profiles, two candidates, repeats, and six-hour durations would create an unnecessarily expensive and fragile workflow. The v3 design separates breadth from depth:

1. every certified profile receives one continuous two-hour AVTB execution;
2. the two most timing-sensitive profiles receive six-hour AVTB and 90 kHz executions;
3. each execution is an independently verifiable shard;
4. a final aggregate contract is created only when all nine shard contracts are present and valid.

This produces useful multi-hour coverage without hiding a failed profile behind a global average or rerunning redundant combinations.

## Parameterized evidence policy

The historical 30-minute contract originally encoded duration, checkpoint interval, repeat count, and segment geometry as package constants.

v3 introduces an immutable `longdrift.Policy` value object containing:

```text
duration_micros
checkpoint_interval_micros
repeat_count
start_tolerance_micros
end_tolerance_micros
checkpoint_tolerance_micros
av_skew_tolerance_micros
repeat_variance_tolerance_micros
cross_candidate_tolerance_micros
```

The existing v1 and v2 functions remain available and delegate to `DefaultPolicy()`, which exactly reproduces the historical 30-minute values. New multi-hour commands and probes use explicit policies.

Parameterized functions include:

```text
BuildCandidateSummaryForPolicy
BuildCandidateComparisonForPolicy
CandidateEvidence.ValidateForPolicy
RunEvidence.ValidateForPolicy
StreamEvidence.ValidateForPolicy
CheckpointTargetsForPolicy
longDurationHLSArgsForPolicy
probeLongDriftStreamForPolicy
buildLongDriftStreamEvidenceForPolicy
```

The policy validates that duration is exactly divisible by the checkpoint interval. Expected HLS segment count is derived from the policy using two-second segment geometry with an explicit ten-segment tolerance.

## Scaling tiers

### Tier 1: two-hour breadth

```text
tier ID                     = multi-hour-breadth-2h-v1
duration                    = 2 hours
checkpoint interval         = 30 minutes
checkpoints per stream      = 5
repeat count                = 1
candidate                   = encoder-time-base-avtb-v1
expected segments per shard = 3,600
```

Profiles:

```text
profile-mp4-aac-44100-cfr-v1
profile-mp4-edit-list-v1
profile-mkv-vfr-24-30-v1
profile-mpegts-positive-origin-v1
profile-mkv-opus-v1
```

Purpose: ensure all five deterministic source/container/audio profiles remain inside the existing timing limits over a continuous two-hour output.

### Tier 2: six-hour depth

```text
tier ID                     = multi-hour-depth-6h-v1
duration                    = 6 hours
checkpoint interval         = 1 hour
checkpoints per stream      = 7
repeat count                = 1
candidates                  = AVTB and 90 kHz
expected segments per shard = 10,800
```

Sentinel profiles:

```text
profile-mp4-aac-44100-cfr-v1
profile-mkv-vfr-24-30-v1
```

The 44.1 kHz profile stresses projection into the 90 kHz transport clock. The VFR profile stresses long-running variable cadence. Both candidates are executed for both sentinels so their corresponding six-hour evidence can be compared directly.

## Execution size

```text
five 2-hour breadth shards      = 10 encoded hours
four 6-hour depth shards        = 24 encoded hours
aggregate                        = 34 encoded hours
encoded minutes                  = 2,040
scaling shards                   = 9
video and audio stream scans     = 18
approximate HLS media segments   = 61,200
```

Each shard is one continuous HLS transcode. Source looping remains input-scoped through `-stream_loop -1`; reports are not assembled from short timestamp simulations or concatenated media outputs.

## Shard contract

Shard schema:

```text
long-duration-scaling-shard-evidence-v3
```

Shard report schema:

```text
ffmpeg-long-duration-scaling-shard-v3
```

Each shard contract binds:

1. Real Media Corpus Spec and Manifest versions and hashes;
2. source generator, source FFmpeg, and source FFprobe identities;
3. certification FFmpeg and FFprobe identities;
4. canonical Timestamp Plan identity;
5. exact tier, profile, candidate, and shard identity;
6. source path, SHA-256, byte size, and canonical asset evidence hash;
7. normalized FFmpeg command hash;
8. generated playlist hash;
9. Produced-media Attestation identity;
10. segment count;
11. complete video and audio packet-derived stream evidence;
12. dynamic checkpoint targets and errors;
13. candidate summary;
14. fail-closed production fields.

A shard is rejected if its tier/profile/candidate combination is not in the immutable registry.

## Aggregate contract

Aggregate schema:

```text
long-duration-scaling-aggregate-evidence-v3
```

Aggregate report schema:

```text
ffmpeg-long-duration-scaling-aggregate-v3
```

The aggregate contract:

- requires exactly nine shard reports;
- rejects duplicate or missing shard IDs;
- orders shards according to the immutable registry rather than filesystem order;
- embeds every shard contract and binds its contract hash;
- requires identical Corpus and Timestamp Plan identities across every shard;
- independently rebuilds six-hour AVTB/90 kHz comparisons;
- fails when either sentinel comparison is not equivalent;
- preserves the fail-closed production policy.

The aggregate command does not trust artifact names as evidence. Every downloaded JSON report is decoded and validated before aggregation.

## Commands

List tiers and exact shard IDs:

```bash
go run ./cmd/transcode-long-duration-scaling-cert -list
```

Run one shard:

```bash
go run ./cmd/transcode-long-duration-scaling-cert \
  -corpus-root /tmp/nowen-transcode-real-media-corpus-v1 \
  -shard multi-hour-breadth-2h-v1--profile-mkv-vfr-24-30-v1--encoder-time-base-avtb-v1 \
  -output /tmp/2h-mkv-vfr-avtb.json
```

Aggregate a complete shard directory:

```bash
go run ./cmd/transcode-long-duration-scaling-cert \
  -aggregate-dir /tmp/long-duration-scaling-shards \
  -output /tmp/long-duration-scaling-aggregate-v3.json
```

The strict Real Media Corpus loader continues to reject missing, escaping, symlinked, size-mismatched, or SHA-mismatched source assets.

## Independent verification

Verifier:

```text
.github/scripts/verify_long_duration_scaling.py
```

For shard reports it independently recomputes:

- Corpus identities;
- exact tier/profile/shard registry membership;
- source traits and source asset identity;
- expected duration, checkpoint count, and segment count;
- stream duration and end errors;
- every video and audio checkpoint error;
- final A/V skew;
- candidate summary;
- canonical shard contract hash.

For the aggregate report it additionally recomputes:

- exact nine-shard order and completeness;
- each embedded shard contract hash;
- common Timestamp Plan identity;
- six-hour AVTB/90 kHz comparisons;
- canonical aggregate contract hash;
- total encoded minutes and maximum observed errors.

## CI architecture

Workflow:

```text
.github/workflows/transcode-long-duration-scaling-cert.yml
```

Jobs:

1. build contracts and commands, generate and verify one immutable Corpus artifact;
2. execute nine independent matrix jobs with `fail-fast: false` and `max-parallel: 9`;
3. download exactly nine shard artifacts, validate and aggregate them, independently verify the root report, and upload the aggregate artifact.

The aggregate job explicitly checks that the artifact directory contains exactly nine JSON files before invoking the Go aggregator.

The v1 and v2 long-duration workflow path filters were narrowed from `internal/transcode/longdrift/**` to their exact shared and version-specific files. Scaling-only changes no longer trigger redundant 30-minute media matrices, while changes to shared `Policy`, command, or probe behavior still trigger the relevant historical regression workflows.

## Successful two-hour breadth evidence

### MP4 AAC 44.1 kHz CFR

```text
segments                       = 3,600
maximum video end error        = 15,601 us
maximum audio end error        = 29,567 us
maximum final A/V skew         = 20,856 us
maximum checkpoint error       = 29,567 us
stable                         = true
contract                       = 700c04efbfbf78ddf8beae25e6dbe409782a86e45990ad802f29da94bcce9e64
artifact                       = 8834294927
artifact SHA-256               = c8530cc07e9977d27d97c92cb1de8b64acb9e7b65ebc9a708c5ead5a017bc095
```

### MP4 positive-origin edit list

```text
segments                       = 3,600
maximum video end error        = 11,866 us
maximum audio end error        = 21,333 us
maximum final A/V skew         = 10,134 us
maximum checkpoint error       = 21,333 us
stable                         = true
contract                       = b3896b1c3fe2bc332fc83ec1817e5d55598aa999c31f9f1ec6c857f53a20e031
artifact                       = 8834300341
artifact SHA-256               = 49f148b341e6af6ba5e8dadf43b74c41469870f88b1e537588edab9199c8d903
```

### Matroska VFR

```text
segments                       = 3,600
maximum video end error        =    667 us
maximum audio end error        = 25,500 us
maximum final A/V skew         = 17,500 us
maximum checkpoint error       = 25,500 us
stable                         = true
contract                       = 77cfd3606201f2aa9818f75b934cfd45b8957f5a472d06dd58aec3662f2274d4
artifact                       = 8834258496
artifact SHA-256               = 98a8ef39ca64a5cba9a97de3811a303af134985a47be909a613159e61ad9333e
```

### MPEG-TS positive transport origin

```text
segments                       = 3,600
maximum video end error        = 14,667 us
maximum audio end error        = 21,333 us
maximum final A/V skew         =  6,667 us
maximum checkpoint error       = 21,333 us
stable                         = true
contract                       = c7309d3fb8e3e0892df57bda2c58d58ad8832b562ce171e36133566342df7620
artifact                       = 8834288975
artifact SHA-256               = f14d618bdb7e4c612e9821da8ef3cd0da406a7a93e8619d8a3dd85227c8bb2a8
```

### Matroska Opus

```text
segments                       = 3,600
maximum video end error        = 32,000 us
maximum audio end error        = 34,333 us
maximum final A/V skew         = 19,000 us
maximum checkpoint error       = 34,333 us
stable                         = true
contract                       = fc70db57c5f862ee9dff16a9a2bdf42601b4d8cd87f2ef37156d433af89bb7dd
artifact                       = 8834263337
artifact SHA-256               = 30619272445a84426e775fb76c47770e2c4d1c194e82a4344df3581869305b41
```

## Successful six-hour depth evidence

### MP4 AAC 44.1 kHz

Both candidates produced:

```text
segments                       = 10,800
maximum video end error        = 31,123 us
maximum audio end error        = 42,267 us
maximum final A/V skew         = 16,834 us
maximum checkpoint error       = 42,267 us
stable                         = true
```

AVTB:

```text
contract                       = 919eff61898555ab986eb30bf121a51401406d156ed2e521aee4bb6fb7da592f
artifact                       = 8834457392
artifact SHA-256               = b1f4f12dde86987e7faa65c35e6f2e45d9506bc94983dd906264f3ca78d86018
```

90 kHz:

```text
contract                       = e41e08c8ff1726c48daf1b663b3823a8f149fd547eeb7d6db9618ed80ce3c724
artifact                       = 8834442973
artifact SHA-256               = 5e57a30c94d6ae58c7f9d1aecb2bc449edb9812c7abbfd68365d77ec1661bc5f
```

Comparison:

```text
candidates equivalent          = true
maximum checkpoint difference  = 11 us
```

The summary maxima are identical, while one corresponding checkpoint differs by eleven microseconds. This remains far below the 2,000 us cross-candidate limit.

### Matroska VFR

Both candidates produced:

```text
segments                       = 10,800
maximum video end error        =  6,333 us
maximum audio end error        = 33,500 us
maximum final A/V skew         =  2,500 us
maximum checkpoint error       = 33,500 us
stable                         = true
```

AVTB:

```text
contract                       = 66e3ef1cbf3fc96acbf19628d0a410459a7e210cc878155c75175359dafbe0e1
artifact                       = 8834324994
artifact SHA-256               = b25c0008176b73f580c6f300778b0b44c7e13c1fa69323d6931e2102fd9e6201
```

90 kHz:

```text
contract                       = 2e2592eac6005f658a92142be5891a37e88426f08159027f0a4a8c0bfa9fcc1b
artifact                       = 8834447933
artifact SHA-256               = 502bdd83e692c8945c173d7eddf397efb8991ebdfc598eab2d12ad6eb4d7b1a4
```

Comparison:

```text
candidates equivalent          = true
maximum checkpoint difference  = 0 us
```

## Aggregate evidence

Reference run:

```text
workflow run                   = 30749804750
merge-tested commit            = 9e1ed767a2332b7d5bcd13da0af52b68b6193783
branch head                    = e8cd8504396608a4e0616c2fbb84b8362c43d294
aggregate contract             = 840423bd2bfee758c6e19c8d96a0e6e01beb8ecd0bfefcfba5672bcf08ef825b
aggregate artifact             = 8834465070
aggregate artifact SHA-256     = d88b97c17b8083ffbaa4908dbf32d69d50f03573ad8eb18c32cbfb395903c1d7
aggregate JSON                 = approximately 88 KB
retention                      = 7 days
```

Immutable Corpus:

```text
corpus artifact                = 8834166740
corpus artifact SHA-256        = 09a5a3354e151eed31091f3cf2bbbc16347ef097dfdf15ff4224b59b10ceae85
compressed size                = approximately 24.5 MB
```

Aggregate verifier result:

```text
shards                         = 9
encoded minutes                = 2,040
encoded hours                  = 34
maximum observed end error     = 42,267 us
maximum candidate checkpoint   = 11 us
all shards stable              = true
all six-hour comparisons       = equivalent
```

The nine media jobs ran concurrently. The workflow completed in approximately twenty-seven minutes of wall-clock time on hosted runners.

## Acceptance limits

v3 deliberately retains the existing long-duration limits:

```text
raw first-packet origin tolerance = 3,000,000 us
stream end-error tolerance        =    50,000 us
checkpoint-error tolerance        =    50,000 us
final A/V skew tolerance          =    50,000 us
cross-candidate tolerance         =     2,000 us
```

The single-execution v3 tiers do not claim repeat variance evidence. Repeat-count validation is still explicit in the policy and remains two for the historical 30-minute v1/v2 contracts.

## What this phase proves

For the selected deterministic software-encoding profiles, this phase proves that:

- every v2 profile remains inside the declared timing limits for two continuous hours;
- MP4 AAC 44.1 kHz remains inside the limits for six continuous hours;
- Matroska VFR 24-to-30 fps remains inside the limits for six continuous hours;
- six-hour AVTB and 90 kHz outputs remain equivalent within 11 microseconds at corresponding checkpoints;
- positive MP4 edit-list and MPEG-TS origins do not accumulate material two-hour output drift;
- Matroska Opus input remains stable through the production-shaped AAC HLS output path for two hours;
- the parameterized evidence layer preserves the old 30-minute behavior while supporting longer immutable policies;
- missing, duplicate, malformed, or divergent shard evidence cannot produce a valid aggregate contract.

## Remaining gates

Multi-Hour Duration Scaling v3 does not complete:

- multi-hour repeat-variance coverage;
- six-hour coverage for edit-list, MPEG-TS, or Opus profiles;
- twelve-hour, twenty-four-hour, or multi-day execution;
- 24000/1001 long-duration coverage;
- PTS/DTS wraparound and MPEG-TS 33-bit timestamp rollover;
- discontinuous, corrupted, truncated, or damaged source timelines;
- multi-audio and language switching;
- subtitles and external subtitle timelines;
- interlaced, HEVC, AV1, HDR, or Dolby Vision media;
- hardware encoder certification;
- cancellation, retry, restart, lease recovery, memory, disk, and thermal stress during multi-hour jobs;
- NAS-class device validation;
- Web, PC, Android, Emby, or Infuse playback acceptance;
- production rollout, fallback, rollback, or encoder time-base selection authorization.

No evidence in this phase independently permits removal of `#EXT-X-DISCONTINUITY` or a production encoder time-base policy change.
