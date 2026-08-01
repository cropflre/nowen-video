# Startup Stream Playback Contract

## Status

This document defines the production-facing Startup Stream contract on
`refactor/server-lite-v1`.

It is a formal playback method, not a hidden replacement for `transcode` and
not a client-side redirect trick.

Current playback protocol version:

```text
startup_stream / event_bridge_v1
```

Current Job planners:

```text
startup-hls-v2
startup-continuation-hls-v3
```

Current output compatibility schema:

```text
hls-encoding-plan-v1
```

Current produced-media evidence schema:

```text
hls-produced-media-attestation-v1
```

## Decision boundary

The Playback Planner may select `startup_stream` only when all of the following
are true:

1. Direct Play, Remux, and Smart Remux are not the selected primary method.
2. A source-fingerprint-matched Startup Artifact is already `published`.
3. Its profile, duration, authenticated bridge URL, Encoding Plan version, and
   Encoding Plan hash are complete.
4. Its persisted Encoding Plan exactly matches the plan rebuilt from the current
   authoritative Probe and shared profile catalog.
5. The Artifact carries a complete, identity-verified Produced-media
   Attestation with `attestation_status = verified`.
6. ffprobe-observed output matches the persisted Encoding Plan.
7. The request does not carry a per-request `max_bitrate` cap.

When a bitrate cap is present, Runtime HLS remains authoritative because a
previously encoded immutable Startup Artifact cannot be retroactively capped.

Startup Stream is never selected for STRM/WebDAV resolver-specific sources in
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

The client must treat `url` as the only primary playback URL. It must not build
Artifact paths, Job IDs, Attempt IDs, continuation URLs, or attestation state
itself.

`encoding_plan_version` and `encoding_plan_hash` remain read-only diagnostics
for incident correlation. Clients must not use them to reproduce server
planning or make their own compatibility decision.

Attestation JSON, packet timestamps, filesystem paths, ffprobe output, Job IDs,
Attempt IDs, Artifact IDs, and Lease tokens remain inside the server boundary.
The external schema intentionally does not depend on provisional attestation
state while that protocol is still evolving.

## Encoding Plan fencing

Startup and Continuation are separate durable Jobs, so their ordinary
`PlanHash` values are expected to differ. The first covers the bounded startup
range and the second covers the remaining timeline.

Both Jobs must persist the same immutable output compatibility identity:

```text
encoding_plan_version
encoding_plan_hash
encoding_plan_json
```

The Encoding Plan contains the shared transport, codec, dimensions, source
frame-rate identity, GOP/keyframe policy, pixel-format contract, color
conversion policy, audio policy, and target segment duration. It deliberately
excludes start offset, duration, priority, Worker, Lease, Attempt, paths, and
hardware backend.

Every Attempt Artifact inherits these fields from its owning Job before FFmpeg
starts. A missing or mismatched plan is handled as not bridge-eligible. It is
never appended optimistically.

Full details are defined in `docs/TRANSCODE_ENCODING_PLAN.md`.

## Produced-media Attestation

The declared Encoding Plan is not sufficient evidence that a hardware or
software encoder actually emitted compatible media.

Before an immutable Startup or Continuation Artifact may be published, the
server runs ffprobe against the first and last MPEG-TS segments and records:

- actual video Codec, Profile, Level, dimensions, Pixel Format, colors, frame
  rate, and Time Base;
- actual audio Codec, channel count, sample rate, and Time Base;
- first and last PTS/DTS packet checkpoints;
- the last packet end timestamp and millisecond bounds.

A planned Artifact can enter `publishing` only when its Attestation is
`verified`. This rule is enforced both in the service and again in the database
publish transaction.

A live Continuation remains invisible until its first materialized segment has
a Lease-valid `provisional` Attestation. The final publication path replaces
that provisional proof with a complete verified proof.

Full details are defined in
`docs/TRANSCODE_PRODUCED_MEDIA_ATTESTATION.md`.

## EVENT bridge

The bridge is a server-generated, authenticated EVENT playlist:

```text
immutable verified Startup Artifact segments
  -> EXT-X-DISCONTINUITY
  -> Lease-valid, attested and stream-compatible Continuation segments
```

The bridge itself stores no media files and never copies or mutates either
Artifact directory.

While continuation is pending, the playlist deliberately omits
`EXT-X-ENDLIST`, allowing normal HLS playlist reloads. `EXT-X-ENDLIST` is added
only after the continuation Artifact has completed.

Segment URLs use the existing authenticated stream route with a virtual profile:

```text
/api/stream/:media/startup-:profile/stream.m3u8
/api/stream/:media/startup-:profile/startup__segNNNN.ts
/api/stream/:media/startup-:profile/continuation__segNNNN.ts
```

The HTTP Adapter parses this virtual namespace and delegates file resolution to
the Artifact Store. It does not access a shared mutable media/profile directory.

## Actual stream compatibility

Before the bridge appends Continuation segments, it compares the verified
Startup tail evidence with the Continuation first-segment evidence.

The following must match:

- Encoding Plan version and hash
- video Codec and Time Base
- dimensions and Pixel Format
- color primaries, transfer, and matrix
- audio Codec and Time Base
- channel count and sample rate

This is stronger than comparing declared plans, but it still does not prove
that timestamps are continuous across independently started FFmpeg processes.

## Migration and rollback

Database changes are additive. Existing Artifact rows and files are not deleted
or rewritten.

Upgrade behavior:

- historical rows keep blank Attestation fields;
- no Attestation is fabricated from old metadata or directory existence;
- historical Startup/Continuation Artifacts without verified evidence remain
  stored but are excluded from the new Resolver;
- `startup-continuation-hls-v3` prevents continuation-v2 output from being
  confused with the attested execution contract;
- the next normal warm-up or playback submission generates a new Artifact;
- no shared directory or compatibility copy is introduced.

Rollback behavior:

- an older binary ignores the additive columns;
- files and historical rows are preserved;
- a later re-upgrade again excludes unattested rows from the current bridge.

## Fallback

`fallback_method` and `fallback_url` always point to ordinary Runtime HLS.
Clients may switch to the fallback once when the bridge cannot be loaded or the
handoff fails. Authentication, authorization, and DRM errors must not trigger an
automatic compatibility fallback.

Android V2 already has one-shot playback fallback behavior. Web/PC consumes the
Startup Stream URL through the common HLS path; explicit fatal-HLS fallback UX
remains a separate client hardening task.

## Timeline honesty

`discontinuity_at_handoff=true` remains intentional.

The current implementation proves actual stream identity and records packet
checkpoints, but does not yet define the legal relationship between the Startup
last packet and Continuation first packet. In particular, independent FFmpeg
processes may reset or offset timestamps differently.

Before this field can become `false`, the following must be verified on every
encoding backend:

- video PTS/DTS origin and alignment at the startup boundary;
- MPEG-TS timestamp wrap and offset rules;
- audio priming, encoder delay, and padding;
- keyframe and segment-boundary checkpoint alignment;
- actual Sample Description compatibility;
- browser, ExoPlayer, mpv, Emby, and Infuse behavior without decoder reset.

Produced-media Attestation is evidence collection and output-contract
verification. It is not yet Timeline Continuity Attestation.

## Failure and recovery

- A stale Lease cannot attach provisional or verified evidence.
- A superseded Attempt cannot publish into the bridge.
- Source fingerprint, Planner Version, Encoding Plan version, or Encoding Plan
  hash changes invalidate reuse.
- A missing or malformed Attestation hides the candidate.
- An unsafe Manifest URI or ffprobe failure prevents publication.
- A Codec, Pixel Format, color, channel, sample-rate, or Time Base mismatch
  prevents bridge append.
- A missing Startup Artifact makes the Planner return normal `transcode`.
- A database or Artifact Store failure is returned as an infrastructure error;
  it is not silently converted into On-demand playback.
- Service restart reconstructs readability from Job, Attempt, Lease, Artifact,
  Encoding Plan, and Attestation state rather than directory existence.

## Verification

Required automated coverage:

- deterministic Encoding Plan and Attestation canonical JSON/SHA-256 identity;
- fake-ffprobe first/last segment extraction;
- unsafe Manifest URI rejection;
- observed output mismatch rejection;
- Job-to-Artifact plan inheritance and additive migration;
- repository rejection of unattested active and published Artifacts;
- Lease-fenced provisional and final proof persistence;
- Startup Planner projection and Runtime HLS fallback;
- bitrate-cap exclusion;
- virtual route parsing and path rejection;
- Startup-before-continuation ordering;
- exactly one declared handoff discontinuity;
- open EVENT playlist while continuation is pending;
- End List only after continuation completes;
- Go full-package and race tests;
- Lite and Full server build and Docker persistent-volume restart smoke tests.

## Next phase

The next formal phase is Timeline Continuity Attestation. It must define and
verify the mathematical timestamp relationship between Startup and Continuation,
including audio priming and encoder delay. Only after that proof succeeds across
all supported backends and clients may removal of `EXT-X-DISCONTINUITY` be
considered.
