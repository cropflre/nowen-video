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

1. verifies Go fixture and timestamp contracts;
2. builds the certification CLI;
3. produces all real-media fixtures;
4. validates fixture identities and fail-closed policy;
5. prints the measured comparison deltas;
6. uploads the matrix JSON as a 14-day workflow artifact.

The Server Lite CI remains responsible for the full Go, Web, Docker, migration, race, and performance matrix.

## Next certification phases

The next fixtures should add, without weakening the current contract:

- variable-frame-rate sources;
- open-GOP and longer B-frame chains;
- non-zero source timestamps;
- boundaries immediately before and after source keyframes;
- HDR-to-SDR conversion;
- cancellation, restart, Lease recovery, and software fallback;
- QSV, NVENC, and VAAPI as independently versioned backend contracts;
- hls.js, Safari native HLS, ExoPlayer, mpv, Emby, and Infuse playback observations.

A future seamless protocol must use a new schema and explicit client/backend certification identities. It must not reinterpret `startup-handoff-timeline-v2`.
