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
startup-continuation-hls-v2
```

Current output compatibility schema:

```text
hls-encoding-plan-v1
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
5. The request does not carry a per-request `max_bitrate` cap.

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
Artifact paths, Job IDs, Attempt IDs, or continuation URLs itself.

`encoding_plan_version` and `encoding_plan_hash` are read-only diagnostics for
incident correlation. Clients must not use them to reproduce server planning or
make their own compatibility decision. Canonical Encoding Plan JSON remains
inside the server boundary.

## Encoding Plan fencing

Startup and Continuation are separate durable Jobs, so their ordinary
`PlanHash` values are expected to differ. The first covers the bounded startup
range and the second covers the remaining timeline.

Both Jobs must nevertheless persist the same immutable output compatibility
identity:

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
starts. The repository resolver filters on plan version and hash before an
Artifact becomes a readable candidate. The service additionally verifies the
canonical JSON when it is present on both sides.

A missing or mismatched plan is handled as not bridge-eligible. It is never
appended optimistically.

Full details are defined in `docs/TRANSCODE_ENCODING_PLAN.md`.

## EVENT bridge

The bridge is a server-generated, authenticated EVENT playlist:

```text
immutable Startup Artifact segments
  -> EXT-X-DISCONTINUITY
  -> Lease-valid, Encoding-Plan-matched Startup Continuation segments
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

## Migration and rollback

The v2 Startup planners do not delete or rewrite v1 Artifacts. Database changes
are additive and old rows remain available for historical inspection and normal
retention cleanup.

Upgrade behavior:

- existing Job plan fields are copied into old Artifact rows when available;
- historical Startup/Continuation rows without an Encoding Plan remain stored;
- v1 Planner Version rows or blank-plan rows are not eligible for the v2 Bridge;
- the next normal warm-up creates a v2 Startup Artifact with a current Encoding
  Plan;
- no shared directory or compatibility copy is introduced.

Rollback behavior:

- an older binary ignores the additive columns;
- files and historical rows are preserved;
- the v2 Planner Version prevents a newer binary from confusing old and new
  output identities after a later re-upgrade.

## Fallback

`fallback_method` and `fallback_url` always point to ordinary Runtime HLS.
Clients may switch to the fallback once when the bridge cannot be loaded or the
handoff fails. Authentication, authorization, and DRM errors must not trigger an
automatic compatibility fallback.

Android V2 already has one-shot playback fallback behavior. Web/PC consumes the
Startup Stream URL through the common HLS path; explicit fatal-HLS fallback UX
remains a separate client hardening task.

## Timeline honesty

`discontinuity_at_handoff=true` remains intentional. The current implementation
provides one stable URL, one append-only playlist, and a persisted shared output
contract, but it does not claim sample-perfect timestamp continuity yet.

Before this field can become `false`, the following must be verified on every
encoding backend:

- video PTS/DTS origin and alignment at the startup boundary;
- audio priming and encoder-delay alignment;
- exact selected source tracks and channel layout;
- actual codec profile, level, pixel format, color metadata, and time base;
- keyframe and segment-boundary checkpoint alignment;
- browser, ExoPlayer, mpv, Emby, and Infuse behavior without decoder reset.

The Encoding Plan is a declared compatibility contract. A later phase must also
attest that produced media conforms to that declaration before the explicit HLS
discontinuity can be removed.

## Failure and recovery

- A stale Lease cannot expose continuation segments.
- A superseded Attempt cannot publish into the bridge.
- Source fingerprint, Planner Version, Encoding Plan version, or Encoding Plan
  hash changes invalidate reuse.
- A canonical plan mismatch hides the candidate even when its shorter hash
  lookup matched.
- A missing Startup Artifact makes the Planner return normal `transcode`.
- A database or Artifact Store failure is returned as an infrastructure error;
  it is not silently converted into On-demand playback.
- Service restart reconstructs readability from Job, Attempt, Lease, Artifact,
  and Encoding Plan state rather than directory existence.

## Verification

Required automated coverage:

- deterministic canonical Encoding Plan JSON and SHA-256 identity;
- compatibility-field hash changes;
- Job-to-Artifact plan inheritance and additive migration backfill;
- repository rejection of mismatched active and published Artifacts;
- Startup Planner projection and Runtime HLS fallback;
- bitrate-cap exclusion;
- virtual route parsing and path rejection;
- Startup-before-continuation ordering;
- exactly one declared handoff discontinuity;
- open EVENT playlist while continuation is pending;
- End List only after continuation completes;
- safe version/hash decoding in Web/PC and Android;
- Go full-package and race tests;
- Lite and Full server build and Docker persistent-volume restart smoke tests.

## Next phase

The next formal phase is produced-media attestation and timestamp checkpointing.
It must record and verify actual output codec/profile/level, stream time bases,
first/last PTS/DTS, AAC priming information, and segment boundary checkpoints.
Only after the declared Encoding Plan and produced-media attestation agree on
all supported backends and clients may the handoff discontinuity be considered
for removal.
