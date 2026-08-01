# Transcode Fixture Certification

## Status

This phase turns the Startup-to-Continuation handoff from a synthetic value-object test into a reproducible real-FFmpeg evidence pipeline.

The first fixture is:

```text
cfr-h264-aac-48k-software-v1
```

It is a baseline only. It does **not** authorize removal of `#EXT-X-DISCONTINUITY`.

## What the baseline measures

The fixture generates a finite source with:

- 30 fps constant-frame-rate video;
- H.264 8-bit `yuv420p`;
- AAC stereo at 48 kHz;
- two-second HLS segments;
- a 30-second Startup boundary;
- software encoding through `libx264`.

Startup and Continuation are produced by independent FFmpeg processes. Both use the production HLS argument builder and the canonical `hls-timestamp-normalization-v1` command policy.

The Startup process is projected to the production bounded VOD shape. The Continuation process seeks to the 30-second Job origin and runs to source EOF using the production EVENT/append-list shape.

Continuation deliberately does not add a relative `-t` duration. With copied source timestamps, FFmpeg evaluates that option against the retained timeline and a small value can terminate before the first continuation packet. The fixture therefore mirrors the real durable Continuation Job instead of creating a test-only command.

## Evidence chain

One run creates and validates this chain:

```text
synthetic source
  -> Startup HLS Artifact
  -> Continuation HLS Artifact
  -> two produced-media attestations
  -> timestamp-origin validation
  -> startup-handoff-timeline-v2 contract
  -> JSON certification report
```

The report includes:

- FFmpeg and FFprobe version lines;
- Encoding Plan version and hash;
- Timestamp Plan version and hash;
- Startup and Continuation attestation identities;
- observed video/audio start and end times;
- handoff contract identity;
- video/audio PTS and DTS deltas;
- aggregate handoff status and decision reason;
- the fail-closed playback policy.

## Result interpretation

`aligned`, `gap`, `overlap`, and `mixed` are measured classifications, not CI pass/fail labels.

The certification command fails only when it cannot build real media, cannot verify the declared encoding contract, loses the Job-owned timestamp origin, cannot construct a valid handoff contract, or attempts to authorize seamless playback without certification.

A successful report still requires:

```text
seamless_allowed = false
discontinuity_required = true
```

This prevents one favorable Linux/FFmpeg measurement from silently changing production playback policy.

## Run locally

```bash
go run ./cmd/transcode-fixture-cert \
  -output ./artifacts/cfr-h264-aac-48k-software-v1.json
```

Useful diagnostic options:

```bash
go run ./cmd/transcode-fixture-cert \
  -work-dir /tmp/nowen-transcode-fixture \
  -output /tmp/nowen-transcode-fixture/report.json
```

An automatically created workspace is deleted after the run unless `-keep-work-dir` is set.

## CI evidence

`.github/workflows/transcode-fixture-cert.yml` runs the baseline on every relevant branch or pull-request change. It uploads the JSON report as a versioned workflow artifact instead of relying only on log text.

The existing Server Lite CI remains responsible for the complete Go, Web, Docker, Android, migration, race, and performance verification matrix. The fixture workflow is a focused media-evidence lane.

## Remaining certification matrix

The next fixtures must add, without weakening the current contract:

- variable-frame-rate sources;
- B-frame-heavy and open-GOP sources;
- AAC at 44.1 kHz;
- non-zero source timestamps;
- boundaries before and after source keyframes;
- HDR-to-SDR conversion;
- cancellation, restart, Lease recovery, and software fallback;
- QSV, NVENC, and VAAPI as independently versioned backend contracts;
- hls.js, Safari native HLS, ExoPlayer, mpv, Emby, and Infuse playback observations.

A future seamless protocol must use a new schema and explicit client/backend certification identities. It must not reinterpret `startup-handoff-timeline-v2`.
