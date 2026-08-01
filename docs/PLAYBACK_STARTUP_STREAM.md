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

Current handoff timeline schema:

```text
startup-handoff-timeline-v1
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
The external schema intentionally does not depend on provisional attestation or
handoff evaluation state while those protocols evolve.

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
  -> contract-governed EXT-X-DISCONTINUITY
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

This is stronger than comparing declared plans, but it still does not by itself
prove that timestamps are continuous across independently started FFmpeg
processes.

## Timeline Continuity Attestation

The server now builds an immutable packet-boundary contract for every readable
Startup/Continuation Artifact pair.

For video and audio independently it records:

```text
startup final packet end PTS
continuation first PTS
presentation delta
startup derived end DTS
continuation first DTS
decode delta
stream time base
tolerance
```

The result is classified as:

```text
aligned
gap
overlap
mixed
```

The contract is persisted in `transcode_handoff_attestations` and identified by:

```text
startup_artifact_id
continuation_artifact_id
startup-handoff-timeline-v1
```

A normal EVENT playlist reload reuses the stored contract. If the Continuation
changes from a provisional to a verified Produced-media Attestation, the same
handoff row is reevaluated and updated rather than duplicated.

Schema v1 has a hard safety invariant:

```text
seamless_allowed = false
discontinuity_required = true
```

Therefore even a mathematically `aligned` boundary retains
`#EXT-X-DISCONTINUITY` with `decision_reason=client_certification_pending`.
Missing, malformed, stale or unreadable handoff state also fails closed.

Full details are defined in
`docs/TRANSCODE_TIMELINE_CONTINUITY.md`.

## Migration and rollback

Database changes are additive. Existing Artifact rows and files are not deleted
or rewritten.

Upgrade behavior:

- historical rows keep blank Produced-media Attestation fields;
- no Produced-media or handoff Attestation is fabricated from old metadata or
  directory existence;
- historical Startup/Continuation Artifacts without verified evidence remain
  stored but are excluded from the current Resolver;
- `startup-continuation-hls-v3` prevents continuation-v2 output from being
  confused with the attested execution contract;
- `transcode_handoff_attestations` is created empty and populated only from
  current verified evidence;
- the next normal warm-up or playback submission generates current Artifacts;
- no shared directory or compatibility copy is introduced.

Rollback behavior:

- an older binary ignores the additive columns and handoff table;
- files and historical rows are preserved;
- a later re-upgrade reevaluates the boundary from persisted Artifact evidence.

Artifact retention cleanup removes handoff rows that reference the deleted
Startup or Continuation Artifact.

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

The current implementation proves actual stream identity and now records the
mathematical packet relationship between the Startup tail and Continuation
head. It still does not normalize timestamps across independently started
FFmpeg processes or certify decoder behavior without reset.

The current Continuation path uses input-side seeking in a separate process, so
its timestamps may restart near the muxer origin. The expected evidence on many
backends is therefore `overlap`, not alignment.

Before a future contract can authorize `discontinuity_at_handoff=false`, the
following must be completed:

- versioned FFmpeg timestamp-origin and offset policy;
- video PTS/DTS normalization at the startup boundary;
- MPEG-TS timestamp wrap and mux-delay rules;
- audio priming, encoder delay, and padding evidence;
- keyframe and segment-boundary checkpoint alignment;
- actual Sample Description compatibility;
- browser, ExoPlayer, mpv, Emby, and Infuse fixture certification without
  decoder reset.

Packet arithmetic is evidence, not permission.

## Failure and recovery

- A stale Lease cannot attach provisional or verified evidence.
- A superseded Attempt cannot publish into the bridge.
- Source fingerprint, Planner Version, Encoding Plan version, or Encoding Plan
  hash changes invalidate reuse.
- A missing or malformed Produced-media Attestation hides the candidate.
- An unsafe Manifest URI or ffprobe failure prevents publication.
- A Codec, Pixel Format, color, channel, sample-rate, or Time Base mismatch
  prevents bridge append.
- A missing or malformed handoff contract keeps the HLS discontinuity.
- A changed Continuation Attestation hash invalidates the cached handoff
  projection and triggers reevaluation.
- A missing Startup Artifact makes the Planner return normal `transcode`.
- A database or Artifact Store failure is returned as an infrastructure error;
  it is not silently converted into On-demand playback.
- Service restart reconstructs readability from Job, Attempt, Lease, Artifact,
  Encoding Plan, Produced-media Attestation, and handoff contract state rather
  than directory existence.

## Verification

Required automated coverage:

- deterministic Encoding Plan, Produced-media Attestation, and handoff contract
  canonical JSON/SHA-256 identity;
- fake-ffprobe first/last segment extraction;
- unsafe Manifest URI rejection;
- observed output mismatch rejection;
- aligned, gap, overlap, and mixed timestamp classification;
- schema v1 cannot authorize seamless playback;
- Job-to-Artifact plan inheritance and additive migration;
- handoff table migration and provisional-to-verified deterministic upsert;
- repository rejection of unattested active and published Artifacts;
- Lease-fenced provisional and final proof persistence;
- Startup Planner projection and Runtime HLS fallback;
- bitrate-cap exclusion;
- virtual route parsing and path rejection;
- Startup-before-continuation ordering;
- exactly one contract-governed handoff discontinuity;
- missing handoff policy fails closed;
- open EVENT playlist while continuation is pending;
- End List only after continuation completes;
- value-object and repository race tests;
- Artifact Resolver and handoff lookup performance baselines;
- Go full-package tests;
- Lite and Full server build and Docker persistent-volume restart smoke tests.

## Next phase

The next formal phase is versioned FFmpeg timestamp normalization. It must make
Startup and Continuation emit a deliberate shared timestamp origin across every
supported software and hardware backend, while retaining the current
contract-governed discontinuity path as rollback.

Only after normalized output repeatedly produces aligned evidence can the
cross-client fixture certification phase begin. Discontinuity removal requires a
new contract schema; it cannot be enabled by changing a boolean in v1.
