# Transcode Source-Origin and Frame-Rate Evidence

## Status

This document records the certification-only source-origin and frame-rate evidence introduced on `refactor/server-lite-v1`.

The evidence does not change production playback policy. It cannot authorize seamless Startup-to-Continuation handoff.

Current policy remains:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Why this evidence exists

The previous boundary-placement and A/V-sync matrices proved the produced-media overlap around a 30-second handoff for 30 fps CFR sources. They did not answer three separate questions:

1. Does the source really begin at zero, at a positive timestamp, or at a negative timestamp?
2. Is the source packet cadence CFR or VFR before the timestamp plan is applied?
3. Does the existing production-shaped FFmpeg path normalize those inputs consistently without silently changing the meaning of the source evidence?

Those questions must be answered independently. A boundary overlap is an output fact. A source timestamp origin is an input fact. Treating them as one value hides whether an error came from the source, input seek, encoder, muxer, or output timestamp normalization.

## Contracts

### Source-origin evidence

Schema:

```text
hls-source-origin-evidence-v1
```

The contract records:

- immutable case and fixture identity;
- source mode: `cfr` or `vfr`;
- declared frame-rate rational and milli projection;
- declared source timestamp offset and origin class;
- source video/audio first PTS and DTS;
- packet-derived minimum and maximum source durations;
- duration spread and variable-duration classification;
- FFmpeg and FFprobe identity;
- Timestamp Plan identity;
- Boundary Packet Evidence identity;
- A/V Boundary Sync Evidence identity;
- normalized Startup and Continuation first-stream timestamps;
- fail-closed playback policy.

All tick-to-microsecond projections and cadence classifications are recomputed by `Validate`. The canonical JSON is signed with SHA-256.

### Matrix report

Schema:

```text
ffmpeg-source-origin-matrix-v1
```

The matrix requires the registered cases in an exact order, one consistent toolchain identity, unique source evidence hashes, and valid bound Boundary/A/V evidence for every case.

## One source graph, two consumers

The certification uses one deterministic `lavfi` graph as the source of truth.

The exact same graph is consumed by:

1. FFprobe, to measure source PTS, DTS, and packet cadence;
2. the production FFmpeg argument builder, to produce Startup and Continuation HLS.

This is intentional. The certification does not probe one file and transcode a separately reconstructed source. That would create two independent timelines and allow an intermediate container to shift or reject timestamps before the production path sees them.

The HLS command still uses the production-shaped components:

- `service/ffmpeg.BuildHLSArgs`;
- `service/ffmpeg.WithInputSeekMicros`;
- Timestamp Plan v1 through `timestampplan.ApplyFFmpeg`;
- software H.264 zerolatency encoding;
- AAC 48 kHz output;
- MPEG-TS HLS segments;
- Produced-media Attestation;
- Boundary Packet Evidence v1;
- A/V Boundary Sync Evidence v1.

## Why an intermediate NUT fixture was rejected

The first implementation encoded the synthetic source to NUT before probing and transcoding it. FFmpeg rejected the AAC encoder priming packet because its first audio PTS was `-1024` samples:

```text
Negative pts not supported stream 1, pts -1024
```

Shifting the source with `avoid_negative_ts`, `asetpts`, or another container workaround would have made the fixture writable, but it would also have destroyed the fact being certified: the original source timestamp origin.

The final implementation therefore removes the intermediate container. The directly probeable `lavfi` graph preserves exact zero, positive, and negative PTS and is passed unchanged to the production command builder.

## Immutable case registry

| Case | Source cadence | Source origin | Declared rate | GOP |
|---|---|---:|---:|---:|
| `source-cfr-24-origin-zero-v1` | CFR | 0 s | 24 fps | 48 |
| `source-cfr-25-origin-zero-v1` | CFR | 0 s | 25 fps | 50 |
| `source-cfr-30000-1001-origin-zero-v1` | CFR | 0 s | 30000/1001 fps | 60 |
| `source-vfr-24-30-origin-zero-v1` | 20 s at 24 fps, then 20 s at 30 fps | 0 s | deterministic 27 fps mean | 60 |
| `source-cfr-30-origin-positive-5s-v1` | CFR | +5 s | 30 fps | 60 |
| `source-cfr-30-origin-negative-2s-v1` | CFR | -2 s | 30 fps | 60 |

Shared fixture policy:

- 320×180;
- 40 seconds;
- 48 kHz stereo audio;
- 30-second Startup/Continuation boundary;
- four-second HLS target segments;
- software x264 `veryfast` + `zerolatency`;
- one encoding thread;
- Timestamp Plan v1;
- discontinuity required.

## Reference environment

```text
Ubuntu 24.04.4
FFmpeg 6.1.1-3ubuntu5
FFprobe 6.1.1-3ubuntu5
```

## Source and normalized-output results

| Case | Source V/A origin | Source video min/max duration | Source spread | Produced boundary rate | Startup V/A start | Continuation V/A start |
|---|---:|---:|---:|---:|---:|---:|
| 24 CFR | 0 / 0 µs | 41,666 / 41,667 µs | 1 µs | 24.000 fps | 1421 / 1400 ms | 31,400 / 31,379 ms |
| 25 CFR | 0 / 0 µs | 40,000 / 40,000 µs | 0 µs | 25.000 fps | 1421 / 1400 ms | 31,400 / 31,379 ms |
| 30000/1001 CFR | 0 / 0 µs | 33,366 / 33,367 µs | 1 µs | 29.970 fps | 1421 / 1400 ms | 31,430 / 31,379 ms |
| VFR 24→30 | 0 / 0 µs | 33,333 / 41,667 µs | 8,334 µs | 24.000 fps at the probed boundary | 1421 / 1400 ms | 31,400 / 31,379 ms |
| 30 CFR, +5 s | 5,000,000 / 5,000,000 µs | 33,333 / 33,334 µs | 1 µs | 30.000 fps | 1421 / 1400 ms | 31,400 / 31,379 ms |
| 30 CFR, -2 s | -2,000,000 / -2,000,000 µs | 33,333 / 33,334 µs | 1 µs | 30.000 fps | 1421 / 1400 ms | 31,400 / 31,379 ms |

All source audio packet durations were stable at 21,333 µs in this reference environment.

## Boundary and A/V results

All six cases produced the same boundary classification in this software-encoder environment:

```text
video status                    = single_packet_overlap
video presentation delta        = -21,333 µs
audio status                    = multi_packet_overlap
audio presentation delta        = -58,667 µs
A/V boundary delta skew         = -37,334 µs
A/V projection residual         = +1 µs
discontinuity_required          = true
```

The evidence therefore separates two independent properties:

- input origin/cadence changed across the matrix;
- the measured software encoder/muxer overlap pattern remained unchanged.

A positive or negative source origin was not the cause of the previously observed handoff overlap.

## Findings

### Positive and negative origins are preserved as source evidence

FFprobe observed the exact declared origins for both streams:

```text
+5 s case: video = 5,000,000 µs, audio = 5,000,000 µs
-2 s case: video = -2,000,000 µs, audio = -2,000,000 µs
```

The Timestamp Plan then normalized both cases to the same produced Startup and Continuation ranges as the zero-origin 30 fps case.

This proves the current Timestamp Plan can normalize these synthetic source origins consistently in this toolchain. It does not prove all demuxers, formats, or real-world broken timestamps behave the same way.

### CFR rates remain distinguishable

The source probe and packet-derived produced boundary evidence preserve the expected distinction between 24, 25, 29.97, and 30 fps.

Integer-microsecond duration projection creates a one-microsecond spread for rational or repeating frame durations. That does not make a CFR source VFR. VFR classification requires a spread of at least 5,000 µs.

### The VFR source is real, but lossless VFR preservation is not proven

The VFR fixture has two source durations:

```text
30 fps section: approximately 33,333 µs
24 fps section: approximately 41,667 µs
spread: 8,334 µs
```

The source contract therefore correctly classifies it as VFR.

At the probed Startup-to-Continuation boundary, the produced video packet evidence reports a nominal 24 fps cadence. This means the current matrix proves that the production-shaped path accepts and normalizes the VFR source; it does not prove that the entire dynamic cadence is preserved without duplication, dropping, or local cadence collapse.

A separate full-output cadence evidence phase is required before making any VFR preservation claim.

### Frame rate does not remove the overlap

The same video/audio overlap pattern was measured for 24, 25, 29.97, VFR, and 30 fps source cases. Moving to a different source frame rate is not sufficient to authorize a seamless handoff.

## CI enforcement

`Transcode Source Origin Certification` validates:

- exact case registry and order;
- exact declared frame rates and source offsets;
- exact source first PTS for both streams;
- exact source packet-duration ranges;
- CFR/VFR classification;
- exact normalized Startup and Continuation starts;
- exact produced boundary frame-rate projection;
- exact overlap classifications and deltas;
- exact A/V delta skew and projection residual;
- canonical SHA-256 identity for Source, Boundary, and A/V evidence;
- unique evidence hashes for every case;
- one FFmpeg/FFprobe toolchain identity;
- fail-closed playback policy.

The generated Artifact contains:

```text
source-origin-matrix-v1.json
```

## Compatibility and rollback

This phase adds certification code, a dedicated CLI, a CI workflow, contracts, and documentation.

It does not:

- modify the persisted Timestamp Plan v1;
- reinterpret existing Encoding Plan or Attestation schemas;
- modify normal production transcode commands;
- change HLS playlist handoff policy;
- remove `#EXT-X-DISCONTINUITY`;
- change database schemas;
- change Web, Android, PC, Emby, or Infuse playback behavior.

Rollback consists of removing the source-origin certification package, command, workflow, and documentation. Existing production data and playback paths remain compatible.

## Non-claims

This evidence does not claim:

- seamless Startup-to-Continuation playback;
- zero packet overlap;
- lossless VFR cadence preservation;
- correctness for every container or demuxer;
- correctness for damaged or non-monotonic real-world timestamps;
- identical behavior for QSV, NVENC, or VAAPI;
- identical behavior for B-frame or open-GOP encoders;
- production readiness of Timestamp Execution Plan v2;
- permission to remove discontinuity handling.

## Next evidence phase

The next formal phase should be **VFR Output Cadence and Duplicate/Drop Evidence v1**.

It should measure the entire produced Startup and Continuation video timelines rather than only packet windows near the handoff. Required evidence includes:

- source-to-output frame-count mapping;
- duplicate PTS and duplicate-content classification;
- dropped-frame classification;
- cadence runs before and after each VFR transition;
- 23.976, 50, and 59.94 fps cases;
- long-GOP and B-frame cases;
- separate QSV, NVENC, and VAAPI identities;
- repeated-run variance;
- continued fail-closed discontinuity policy.

Only after that evidence exists should the project consider a persisted output-cadence execution policy or any production VFR-specific shaping behavior.
