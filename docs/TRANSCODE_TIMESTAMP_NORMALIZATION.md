# FFmpeg Timestamp Normalization

## Status

This document defines the versioned execution-time timestamp policy used by the
Startup Stream and Startup Continuation planners.

Current schema:

```text
hls-timestamp-normalization-v1
```

Current planners:

```text
startup-hls-v3
startup-continuation-hls-v4
```

This phase makes the desired packet timestamp origin explicit and verifies it
against produced media. It does **not** authorize removal of
`#EXT-X-DISCONTINUITY`.

## Domain boundary

Encoding Plan answers:

```text
What output media format must both Artifacts produce?
```

Timestamp Plan answers:

```text
How must each FFmpeg execution preserve the shared media timeline?
```

Job-owned timeline origin answers:

```text
At what source-media millisecond must this Artifact begin?
```

These are separate identities:

- Startup and Continuation share one Timestamp Plan hash;
- Startup has `timeline_origin_ms = 0`;
- Continuation has `timeline_origin_ms = start_ms`, currently 30000;
- the Job's ordinary Plan Hash includes the timestamp identity and its own
  origin;
- the Encoding Plan hash remains independent of seek, duration and execution
  range.

## Canonical policy

Schema v1 requires:

```text
strategy = copyts_start_at_zero
seek_mode = input_accurate
copy_timestamps = true
start_at_zero = true
avoid_negative_ts = disabled
fps_mode = passthrough
certified_backends = [none]
```

The generated FFmpeg command therefore contains:

```text
-copyts
-start_at_zero
-ss <job start, when non-zero>
-avoid_negative_ts disabled
-fps_mode passthrough
```

`-copyts` prevents FFmpeg from discarding the source timestamp relationship.
`-start_at_zero` establishes one source-relative zero point. With an input seek,
the resulting Continuation timestamps retain that seek position instead of
restarting at the muxer's local origin. `avoid_negative_ts=disabled` prevents
the output muxer from silently applying a second global shift. Passthrough FPS
mode prevents timestamp normalization from being hidden by frame duplication or
dropping.

Input-side accurate seek remains intentional for bounded startup latency. The
produced-media verifier, not the command line alone, is the final authority.

## Backend certification

Version v1 certifies software H.264/AAC encoding only:

```text
backend = none
```

QSV, NVENC and VAAPI remain supported for ordinary Runtime HLS, but they cannot
produce Startup/Continuation Artifacts under this timestamp schema.

When the service detects a hardware backend for a timestamp-normalized Job, the
candidate is rejected before workspace creation, Attempt persistence and
FFmpeg process start. The existing orchestrator then starts the normal software
fallback Attempt.

A later hardware-capable timestamp schema must be based on measured fixtures for
each backend. It must not silently add a backend to the v1 allowlist.

## Persistence

The following additive fields exist on both `transcode_jobs` and
`transcode_artifacts`:

```text
timestamp_plan_version
timestamp_plan_hash
timestamp_plan_json
timeline_origin_ms
```

An Artifact inherits all four fields from its Job before FFmpeg starts.
Repository resolvers require exact Job/Artifact equality in addition to the
Encoding Plan, source fingerprint, planner, Attempt, Lease and Produced-media
Attestation predicates.

Historical Startup v2 and Continuation v3 rows remain stored. They do not carry
the new timestamp contract and are excluded from Startup v3 / Continuation v4
resolution.

## Produced-media gate

A command line is only intent. Before an Artifact becomes provisional or
published, ffprobe-observed first-packet timestamps must match the Job-owned
origin.

Schema v1 accepts first video and audio packet timestamps within this bounded
window:

```text
origin - 250 ms
origin + 3000 ms
```

The lower allowance covers bounded encoder priming. The upper allowance covers
the observed MPEG-TS mux start delay without allowing an entire startup range to
be lost.

For a 30-second Continuation, evidence near 31.4 seconds is eligible. Evidence
near 1.4 seconds is rejected as a reset timeline. A rejected Artifact cannot:

- receive provisional playback visibility;
- be published;
- be resolved by the Startup Bridge;
- participate in a handoff contract.

The final packet-to-packet relation is still evaluated separately by
`startup-handoff-timeline-v2`.

## Measured software fixture

Server Lite CI installs the Ubuntu 24.04 FFmpeg 6.1.1 package and executes a
required real-media fixture. The test generates an eight-second H.264/AAC source,
encodes a bounded 0-4 second Startup HLS Artifact, then encodes a production-like
Continuation from an input seek at four seconds to source EOF.

The measured first-packet timestamps on the accepted fixture were:

```text
startup video       1.421333 s
continuation video  5.400000 s
video delta         3.978667 s

startup audio       1.400000 s
continuation audio  5.378667 s
audio delta         3.978667 s
```

The result proves that the four-second seek relationship survives the separate
FFmpeg execution for both streams. It also demonstrates that MPEG-TS still has a
roughly 1.4-second mux-origin delay. The implementation therefore verifies the
relative Job-owned origin rather than pretending that the first packet must be
exactly zero or exactly the seek value.

This is one deterministic software fixture, not a complete media or backend
certification matrix.

## Migration and rollback

Migration is additive:

- new columns are created on Job and Artifact tables;
- declarative timestamp identity is copied from an owning new Job to its
  Artifact;
- historical Jobs without a Timestamp Plan remain blank;
- no Produced-media Attestation or handoff result is fabricated;
- no media file or Artifact directory is rewritten or deleted.

Rollback behavior:

- an older binary ignores the additive columns;
- old and new Artifact files remain available for rollback diagnostics;
- a later re-upgrade requires the current planner and exact timestamp identity
  before reuse.

## Failure handling

The execution fails closed when:

- Timestamp Plan identity is missing or non-canonical;
- Job origin is negative or differs from `start_ms`;
- an uncertified backend is selected;
- Job and Artifact timestamp fields disagree;
- ffprobe first-packet timestamps are outside the origin window;
- a historical Artifact has no current timestamp identity;
- a handoff pair declares different Timestamp Plans or origins.

Runtime HLS remains the playback fallback. The server does not reinterpret an
invalid Startup/Continuation Artifact as valid output.

## Verification

Required automated coverage:

- deterministic Timestamp Plan canonical JSON and SHA-256;
- policy mutation changes identity;
- v1 rejects hardware backend certification;
- generated FFmpeg option ordering;
- required real FFmpeg/ffprobe origin-preservation fixture;
- Job origin must equal seek start;
- normalized first-packet evidence accepted;
- reset Continuation evidence rejected;
- Job-to-Artifact inheritance and migration;
- no timestamp identity fabricated for historical Jobs;
- exact published and Lease-valid Artifact resolution;
- Job/Artifact timestamp mismatch rejection;
- value-object and service race tests;
- previous and timestamp-fenced resolver performance baselines;
- Lite/Full build and persistent-volume restart smoke tests.

## Non-claims

This phase does not yet prove:

- sample-perfect audio continuity;
- identical AAC priming or padding across executions;
- zero packet gap or overlap at the exact handoff;
- timestamp behavior on QSV, NVENC or VAAPI;
- safe decoder continuation without an HLS discontinuity;
- cross-client certification.

## Next phase

The next formal phase is deterministic media fixture certification. It should:

1. generate source fixtures covering CFR/VFR, B-frames, 44.1/48 kHz audio, HDR
   conversion and non-zero source start times;
2. execute Startup and Continuation with real FFmpeg;
3. require `startup-handoff-timeline-v2` to classify both audio and video;
4. record P50/P95/P99 origin and handoff deltas per backend;
5. keep software as the only certified backend until each hardware backend
   passes independently;
6. retain `#EXT-X-DISCONTINUITY` until client certification is complete.
