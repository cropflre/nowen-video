# Startup Stream Playback Contract

## Status

This document defines the production-facing Startup Stream contract on
`refactor/server-lite-v1`.

It is a formal playback method, not a hidden replacement for `transcode` and
not a client-side redirect trick.

Current protocol and planners:

```text
startup_stream / event_bridge_v1
startup-hls-v3
startup-continuation-hls-v4
```

Current server-owned media contracts:

```text
hls-encoding-plan-v1
hls-timestamp-normalization-v1
hls-produced-media-attestation-v1
startup-handoff-timeline-v2
```

`#EXT-X-DISCONTINUITY` remains mandatory.

## Decision boundary

The Playback Planner may select `startup_stream` only when all of the following
are true:

1. Direct Play, Remux, and Smart Remux are not the selected primary method.
2. A source-fingerprint-matched Startup Artifact is already `published`.
3. Its profile, duration and authenticated bridge URL are complete.
4. Its persisted Encoding Plan exactly matches the current authoritative Probe
   and shared profile catalog.
5. Its Timestamp Plan exactly matches the current server policy and its
   `timeline_origin_ms` is zero.
6. It carries a complete, identity-verified Produced-media Attestation.
7. ffprobe-observed media matches the Encoding Plan.
8. ffprobe-observed first video and audio packets match the Timestamp Plan's
   bounded origin window.
9. The request does not carry a per-request `max_bitrate` cap.

When a bitrate cap is present, Runtime HLS remains authoritative because an
immutable Startup Artifact cannot be retroactively capped.

Startup Stream is not selected for STRM/WebDAV resolver-specific sources in
this version.

## PlaybackPlan schema

Example:

```json
{
  "media_id": "media-id",
  "method": "startup_stream",
  "url": "/api/stream/media-id/startup-720p/stream.m3u8",
  "reason_code": "startup_artifact_ready",
  "reason": "已命中预生成启动流，并通过服务端统一时间线接续持续转码",
  "requires_transcode": true,
  "fallback_method": "transcode",
  "fallback_url": "/api/stream/media-id/master.m3u8",
  "startup_stream": {
    "profile_id": "720p",
    "duration_ms": 30000,
    "playlist_url": "/api/stream/media-id/startup-720p/stream.m3u8",
    "continuation_mode": "event_bridge_v1",
    "discontinuity_at_handoff": true,
    "encoding_plan_version": "hls-encoding-plan-v1",
    "encoding_plan_hash": "sha256-hex"
  }
}
```

The client treats `url` as the only primary playback URL. It must not build
Artifact paths, Job IDs, Attempt IDs, continuation URLs, timestamp offsets or
attestation state.

`encoding_plan_version` and `encoding_plan_hash` remain read-only diagnostics.
Timestamp Plan JSON/hash, origins, packet timestamps, ffprobe output, Artifact
IDs, Lease tokens and handoff JSON remain inside the server boundary.

## Full execution contract

Startup and Continuation are separate durable Jobs. Their ordinary Plan Hashes
differ because their execution ranges differ, but the Bridge requires:

```text
same encoding_plan_version/hash/json
same timestamp_plan_version/hash/json
startup timeline_origin_ms = 0
continuation timeline_origin_ms = startup duration_ms
```

Current default boundary:

```text
startup origin = 0 ms
continuation origin = 30000 ms
```

The Encoding Plan defines transport, codec, dimensions, GOP/keyframe policy,
pixel format, color conversion and audio policy.

The Timestamp Plan defines seek and output timestamp behavior:

```text
-copyts
-start_at_zero
-ss <continuation origin>
-avoid_negative_ts disabled
-fps_mode passthrough
```

The Timestamp Plan does not change Encoding Plan compatibility identity. The
per-Job origin is execution range, not output codec identity.

Full details:

- `docs/TRANSCODE_ENCODING_PLAN.md`
- `docs/TRANSCODE_TIMESTAMP_NORMALIZATION.md`

## Backend policy

Timestamp Plan v1 certifies software encoding only.

QSV, NVENC and VAAPI remain available for ordinary Runtime HLS. A
Startup/Continuation hardware candidate is rejected before workspace creation,
Attempt persistence and FFmpeg start; the existing orchestrator then runs the
software fallback Attempt.

This is intentional fail-closed behavior. Hardware backends must earn a future
contract version through measured fixtures rather than being assumed compatible.

## Produced-media Attestation

Before Startup or Continuation becomes readable, ffprobe records:

- actual video Codec, Profile, Level, dimensions, Pixel Format, colors, frame
  rate and Time Base;
- actual audio Codec, channel count, sample rate and Time Base;
- first and last PTS/DTS packet checkpoints;
- final packet end timestamp and millisecond bounds.

A final Artifact enters publication only with a verified complete Attestation.
A live Continuation remains invisible until the first materialized segment has
a Lease-valid provisional Attestation.

For Timestamp Plan v1, first video and audio packets must be within:

```text
origin - 250 ms
origin + 3000 ms
```

Therefore a 30-second Continuation starting near 31.4 seconds is eligible, while
one resetting near 1.4 seconds is rejected before Bridge exposure.

Full details are in `docs/TRANSCODE_PRODUCED_MEDIA_ATTESTATION.md`.

## EVENT bridge

The bridge is a server-generated authenticated EVENT playlist:

```text
immutable verified Startup segments
  -> contract-governed EXT-X-DISCONTINUITY
  -> Lease-valid, attested Continuation segments
```

The bridge stores no media files and never mutates either Artifact directory.
It remains open without `EXT-X-ENDLIST` while Continuation is pending and adds
`EXT-X-ENDLIST` only after Continuation completes.

Virtual authenticated routes remain:

```text
/api/stream/:media/startup-:profile/stream.m3u8
/api/stream/:media/startup-:profile/startup__segNNNN.ts
/api/stream/:media/startup-:profile/continuation__segNNNN.ts
```

## Handoff Timeline Contract

Before appending Continuation, the server verifies:

- same Encoding Plan;
- same Timestamp Plan;
- exact Startup and Continuation origins;
- actual video/audio stream identity compatibility;
- first-packet origin compliance;
- packet-level PTS and DTS relation at the boundary.

The persisted v2 contract contains:

```text
startup-handoff-timeline-v2
encoding plan identity
timestamp plan identity
startup origin
continuation origin
expected boundary
both produced-media attestation identities
video/audio presentation and decode deltas
aligned / gap / overlap / mixed
```

The canonical contract is reused across playlist reloads. It is recomputed when
a provisional Continuation Attestation becomes verified or either evidence
identity changes.

Schema v2 has a hard invariant:

```text
seamless_allowed = false
discontinuity_required = true
```

Even `aligned` uses `decision_reason=client_certification_pending`. Missing,
malformed, stale or unknown handoff state also fails closed.

Full details are in `docs/TRANSCODE_TIMELINE_CONTINUITY.md`.

## Migration and rollback

All database changes are additive.

Upgrade behavior:

- Startup planner advances from v2 to v3;
- Continuation planner advances from v3 to v4;
- old Artifact rows and files remain stored;
- old rows without a current Timestamp Plan are not reused by the new Bridge;
- timestamp identity is copied only from an owning new Job;
- Produced-media and handoff evidence is never fabricated;
- old v1 handoff rows are retained and v2 rows are created from current evidence;
- no shared directory or compatibility copy is introduced.

Rollback behavior:

- older binaries ignore additive columns and v2 handoff projections;
- files and historical rows are preserved;
- a later re-upgrade again requires exact planner, Encoding Plan, Timestamp Plan,
  origin and Produced-media evidence.

Artifact retention cleanup removes handoff rows referencing a deleted Startup or
Continuation Artifact.

## Fallback

`fallback_method` and `fallback_url` point to ordinary Runtime HLS.

Clients may switch once when the bridge cannot load or handoff fails.
Authentication, authorization and DRM errors must not trigger automatic
compatibility fallback.

Android V2 already has one-shot fallback. Web/PC uses the common HLS path;
explicit fatal-HLS fallback UX remains a separate client hardening task.

## Timeline honesty

The new Timestamp Plan prevents the previous full-range reset from being
silently accepted. It does not prove sample-perfect continuity.

A real handoff can still classify as `gap`, `overlap` or `mixed` because of:

- keyframe position and accurate-seek decode behavior;
- B-frame PTS/DTS ordering;
- MPEG-TS mux delay;
- AAC priming, encoder delay and padding;
- source files with non-zero initial timestamps;
- backend-specific packet behavior.

`discontinuity_at_handoff=true` therefore remains intentional.

Before a future protocol can set it to false, the project must complete real
FFmpeg fixtures and browser, native HLS, ExoPlayer, mpv, Emby and Infuse
certification. Packet arithmetic is evidence, not permission.

## Failure and recovery

- A stale Lease cannot attach provisional or verified evidence.
- A superseded Attempt cannot publish.
- Planner, source fingerprint, Encoding Plan, Timestamp Plan or origin changes
  invalidate reuse.
- An uncertified backend cannot create a current Startup/Continuation Attempt.
- A reset first-packet origin prevents provisional visibility and publication.
- Unsafe Manifest URI or ffprobe failure prevents publication.
- Codec, Pixel Format, color, channel, sample-rate or Time Base mismatch prevents
  Bridge append.
- Missing or malformed handoff state keeps the discontinuity.
- A missing Startup Artifact returns normal Runtime HLS planning.
- Restart reconstructs readability from Job, Attempt, Lease, Artifact, Encoding
  Plan, Timestamp Plan, Produced-media Attestation and handoff state rather than
  directory existence.

## Verification

Automated gates include:

- deterministic Encoding Plan, Timestamp Plan, Produced-media Attestation and
  handoff contract identities;
- Timestamp Plan mutation and backend certification rejection;
- exact FFmpeg option ordering;
- Job origin equals seek start;
- reset Continuation first-packet rejection;
- Job-to-Artifact inheritance and additive migration;
- exact execution-contract repository resolution;
- provisional and final Lease fencing;
- aligned, gap, overlap and mixed handoff classification;
- v2 cannot authorize seamless playback;
- exactly one discontinuity and fail-closed unknown policy;
- value-object, repository and service race tests;
- old and timestamp-fenced resolver performance baselines;
- Go full-package tests;
- Web build;
- Lite/Full build and persistent-volume restart smoke tests;
- Android unit/build gates and API 26/33/35 launch smoke tests.

## Next phase

The next formal phase is deterministic real-media fixture certification.
Software FFmpeg must be measured across CFR/VFR, B-frames, audio sample rates,
HDR conversion, non-zero source timestamps and keyframe positions. Hardware
backends remain uncertified until they pass independently.

The discontinuity remains until a future contract version is backed by both
stable media evidence and cross-client certification.
