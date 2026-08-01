# Transcode Fixture Certification

## Status

This pipeline turns Startup-to-Continuation handoff analysis into reproducible real-FFmpeg evidence. It remains a certification and diagnosis tool, not a production feature flag.

The current matrix is:

```text
cfr-h264-aac-48k-software-v1
cfr-h264-aac-48k-software-zerolatency-v1
cfr-h264-aac-44k1-software-zerolatency-v1
```

Every report must keep:

```text
seamless_allowed = false
discontinuity_required = true
```

No result from this matrix authorizes removal of `#EXT-X-DISCONTINUITY`.

## Why the matrix replaced one baseline

The first baseline used the shared production HLS argument builder, but it did not select the production software tuning policy. The normal server path uses:

```text
-tune zerolatency
```

The original fixture therefore measured x264's default B-frame behavior rather than the exact production software output policy. Its Ubuntu/FFmpeg 6.1.1 result was:

```text
video PTS delta = -33.333 ms
video DTS delta = -66.667 ms
audio PTS/DTS delta = -104 ms
status = overlap
```

That result remains valuable as a historical control. It is not rewritten or hidden. The matrix adds a production-equivalent 48 kHz fixture and a production-equivalent 44.1 kHz fixture so video reordering and AAC sample-rate effects can be compared independently.

## Fixture controls

All fixtures use:

- 30 fps constant-frame-rate source video;
- H.264 8-bit `yuv420p`;
- two-second HLS segments;
- a 30-second Startup boundary;
- software encoding through `libx264`;
- separate Startup and Continuation FFmpeg processes;
- the production HLS argument builder;
- `hls-timestamp-normalization-v1`.

The synthetic source explicitly contains B frames. This keeps the input realistic while allowing the output encoder policy to be the controlled variable.

### Historical control

```text
cfr-h264-aac-48k-software-v1
```

- AAC stereo at 48 kHz;
- explicit output `-bf 3`;
- no `zerolatency` tune;
- retained only as the comparison baseline.

### Production 48 kHz

```text
cfr-h264-aac-48k-software-zerolatency-v1
```

- AAC stereo at 48 kHz;
- output `-tune zerolatency`;
- matches the server's software transcode tuning policy.

### Production 44.1 kHz

```text
cfr-h264-aac-44k1-software-zerolatency-v1
```

- AAC stereo at 44.1 kHz;
- output `-tune zerolatency`;
- isolates AAC frame-duration, priming, and padding behavior from the video tuning comparison.

## Evidence chain

Each fixture creates and validates:

```text
synthetic source
  -> Startup HLS Artifact
  -> Continuation HLS Artifact
  -> two produced-media attestations
  -> timestamp-origin validation
  -> startup-handoff-timeline-v2 contract
  -> versioned fixture report
```

The matrix report contains all three fixture reports plus two arithmetic comparisons:

```text
x264_zerolatency_effect_48k
aac_sample_rate_effect_zerolatency
```

Comparison values are candidate deltas minus baseline deltas. They describe measured change only; positive or negative values are not automatically interpreted as safe or unsafe.

## Report v2 boundary evidence

`ffmpeg-handoff-fixture-report-v2` preserves the summary fields and also records the complete contract projections for video and audio:

- exact stream time base;
- Startup end PTS and DTS;
- Continuation first PTS and DTS;
- tick and microsecond deltas;
- calculated tolerance;
- per-stream status;
- Startup-end and Continuation-start offsets from the expected 30-second boundary;
- observed AAC sample rate;
- explicit fixture tuning policy.

This makes a one-frame video overlap distinguishable from AAC priming or mux-offset behavior without re-running FFprobe manually.

Historical `ffmpeg-handoff-fixture-report-v1` artifacts remain immutable evidence. They are not rewritten into v2. New certification commands emit v2 because v2 contains the raw boundary projection needed for attribution.

## Registry integrity gate

Schema validation and certification authority are separate operations:

- `Report.Validate` checks whether an existing v2 report is structurally readable;
- `ValidateCertifiedReport` additionally requires the complete Fixture metadata to match the immutable in-code registry;
- the CLI and matrix builder use the registry-backed gate before issuing JSON.

This preserves historical report readability while preventing a report with a changed description, sample rate, Tune policy, or control classification from being reissued as certified evidence.

## Observed overlap-attribution matrix

The pull-request matrix executed on Ubuntu 24.04 with FFmpeg/FFprobe 6.1.1. The measured 30-second boundary evidence was:

| Fixture | Video PTS | Video DTS | Audio PTS/DTS | Result |
|---|---:|---:|---:|---|
| historical 48 kHz control | -33.333 ms | -66.667 ms | -104.000 ms | overlap |
| production zerolatency 48 kHz | -21.333 ms | -21.333 ms | -58.667 ms | overlap |
| production zerolatency 44.1 kHz | -23.222 ms | -23.222 ms | -46.622 ms | overlap |

Controlled changes were:

```text
x264 zerolatency at 48 kHz
  video PTS change   = +12.000 ms
  video DTS change   = +45.334 ms
  audio PTS/DTS      = +45.333 ms

audio sample rate, zerolatency output
  48 kHz -> 44.1 kHz audio PTS/DTS change = +12.045 ms
  accompanying video change               = -1.889 ms
```

The matrix supports the following limited conclusions:

1. `zerolatency` substantially removes B-frame decode reordering from the production output, because video DTS moved from `-66.667 ms` to `-21.333 ms`.
2. It does not remove the whole boundary overlap; the production video path still overlaps by about `21–23 ms` in this fixture.
3. Audio overlap changes with both encoder/mux boundary behavior and sample rate. The evidence does not justify treating AAC priming as one fixed constant.
4. All three relations remain outside the current alignment tolerances, so the bridge stays fail-closed.

These values are a reproducible baseline for this FFmpeg build and fixture definition. CI verifies identities, arithmetic consistency, fail-closed policy, and the direction of the controlled zerolatency improvement; it intentionally does not hard-code every measured microsecond across future FFmpeg versions.

## Production-shape execution

Startup uses the bounded VOD projection from the server. Continuation seeks to the Job-owned 30-second origin and runs to source EOF using the EVENT/append-list shape.

Continuation deliberately does not add a relative `-t` duration. With copied source timestamps, FFmpeg evaluates that option against the retained timeline and can terminate before the first continuation packet. The fixture mirrors the durable Continuation Job instead of creating a test-only command.

## Run locally

List fixtures:

```bash
go run ./cmd/transcode-fixture-cert -list
```

Run the production 48 kHz fixture:

```bash
go run ./cmd/transcode-fixture-cert \
  -fixture cfr-h264-aac-48k-software-zerolatency-v1 \
  -output ./artifacts/production-48k.json
```

Run the complete comparison matrix:

```bash
go run ./cmd/transcode-fixture-cert \
  -all \
  -output ./artifacts/cfr-overlap-attribution-matrix-v1.json
```

Keep generated media for manual packet inspection:

```bash
go run ./cmd/transcode-fixture-cert \
  -all \
  -work-dir /tmp/nowen-transcode-fixtures \
  -output /tmp/nowen-transcode-fixtures/matrix.json
```

## CI evidence

`.github/workflows/transcode-fixture-cert.yml` runs the complete matrix for relevant branch and pull-request changes. The workflow:

1. verifies Go fixture, registry-integrity, and timestamp contracts;
2. builds the certification CLI;
3. produces all real-media fixtures;
4. validates fixture identities, nested PTS/DTS arithmetic, sample-rate identities, and fail-closed policy;
5. verifies that production `zerolatency` reduces the absolute control overlap without hard-coding exact future FFmpeg values;
6. prints the measured comparison deltas;
7. uploads the matrix JSON as a 14-day workflow artifact.

The Server Lite CI remains responsible for the full Go, Web, Docker, migration, race, and performance matrix.

## Next certification phases

The next fixtures should add, without weakening the current contract:

- exact boundary placement around the previous packet end and next packet start;
- AAC priming, padding, and encoder-delay metadata where the container exposes it;
- variable-frame-rate sources;
- open-GOP and longer B-frame chains;
- non-zero source timestamps;
- boundaries immediately before and after source keyframes;
- HDR-to-SDR conversion;
- cancellation, restart, Lease recovery, and software fallback;
- QSV, NVENC, and VAAPI as independently versioned backend contracts;
- hls.js, Safari native HLS, ExoPlayer, mpv, Emby, and Infuse playback observations.

A future seamless protocol must use a new schema and explicit client/backend certification identities. It must not reinterpret `startup-handoff-timeline-v2`.
