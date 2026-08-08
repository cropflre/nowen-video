# Versioned Transcode Encoding Plan

## Status

This document defines the first versioned output-encoding contract shared by
Startup HLS and Startup Continuation HLS on `refactor/server-lite-v1`.

It is intentionally separate from `TranscodeJobRecord.PlanHash`:

- `PlanHash` identifies one execution request, including intent, time range,
  priority, and other scheduling inputs.
- `EncodingPlanHash` identifies the media bitstream contract that must remain
  compatible across separate Jobs and Attempts.

A Startup Artifact and its Continuation Artifact may be joined into one playback
timeline only when their Encoding Plan identity is exactly equal.

Current schema version:

```text
hls-encoding-plan-v1
```

## Problem

Startup and Continuation are separate durable Jobs. Their execution plans are
necessarily different because one covers `[0, startup_duration)` and the other
covers `[startup_duration, end)`. Comparing their ordinary Job `PlanHash`
therefore cannot prove output compatibility.

Before this phase, the EVENT Bridge matched only:

- media ID;
- profile ID;
- source fingerprint;
- Planner Version and Artifact Kind within each resolver.

Those fields prevent stale-source reuse, but they do not prove that both outputs
share the same codec, dimensions, frame-rate policy, GOP layout, pixel format,
color conversion, audio selection, or segment contract.

## Domain model

`EncodingPlan` is an immutable canonical value object. Version 1 contains:

- schema version;
- HLS container and segment format;
- profile ID and output dimensions;
- video codec and pixel-format contract;
- frame-rate policy and source frame-rate identity;
- GOP size, keyframe interval, scene-cut policy, and forced-keyframe policy;
- HDR/color conversion policy;
- audio codec, bitrate, channel layout, source-rate policy, and selected track;
- target HLS segment duration.

The canonical JSON representation is hashed with SHA-256. Field ordering is
owned by the Go type and must not depend on maps or runtime argument order.

## Persistence

The following additive fields are stored on both Job and Artifact records:

```text
encoding_plan_version
encoding_plan_hash
encoding_plan_json
```

They are copied from the Job into every Attempt Artifact before FFmpeg starts.
Published Artifacts therefore remain independently auditable after the owning
Job has completed.

Migration is non-destructive:

- columns are added through the existing AutoMigrate path;
- existing Artifact rows are backfilled from their owning Job when possible;
- historical rows without an Encoding Plan remain readable by their original
  runtime resolver, but are not eligible for Startup/Continuation timeline
  bridging;
- no old Job, Attempt, Artifact, task, or file is deleted.

Rollback to an older binary leaves the additive columns untouched and harmless.

## Startup and Continuation rules

The Startup Job builds the Encoding Plan from the authoritative cached Probe and
the selected shared profile catalog entry.

The Continuation Job does not independently invent a second plan. It receives
the Startup descriptor's exact Encoding Plan identity and persists that same
version, hash, and canonical JSON.

The following values are deliberately excluded from `EncodingPlanHash` because
they describe execution rather than output compatibility:

- Job ID, Attempt ID, Worker ID, and Lease Token;
- execution intent;
- start offset and duration;
- priority and retry count;
- temporary or published filesystem paths;
- hardware backend selected for an Attempt.

Backend independence is intentional at this stage. Hardware fallback may change
the concrete encoder implementation, but it must still satisfy the declared
output contract. Later phases may add backend capability proofs before removing
the HLS discontinuity.

## Resolver and bridge fencing

A published Startup Artifact is eligible only when:

1. its source fingerprint matches the current authoritative Probe;
2. its Startup Planner Version matches the current Startup planner;
3. its Encoding Plan version, hash, and canonical JSON match the plan rebuilt
   from the current Probe and profile catalog.

A readable Continuation Artifact is eligible only when:

1. its media/profile/source identity matches the Startup descriptor;
2. its Continuation Planner Version and Artifact Kind match;
3. its Encoding Plan version and hash exactly equal the Startup descriptor;
4. when both canonical JSON values are present, they are byte-for-byte equal.

A mismatch is treated as `not ready / not found`, never as a readable timeline.
The Bridge continues serving the immutable Startup portion and retries normal
Continuation submission/resolution on later playlist reloads.

## API diagnostics

The client-safe Startup playback contract exposes:

```json
{
  "encoding_plan_version": "hls-encoding-plan-v1",
  "encoding_plan_hash": "sha256-hex"
}
```

It does not expose canonical JSON, FFmpeg arguments, filesystem paths, Job IDs,
Attempt IDs, Artifact IDs, or Lease data.

Clients do not make compatibility decisions from the hash. The server remains
the only playback-planning authority; the fields exist for diagnostics and
cross-client incident correlation.

## State and failure semantics

Encoding Plan validation occurs before a Startup Artifact is projected into a
Playback Plan and before a Continuation Artifact is appended to the EVENT
Bridge.

Failure classes:

- missing historical Encoding Plan: Artifact is not bridge-eligible;
- current Probe/profile produces a different hash: Startup is stale and normal
  Runtime HLS is selected;
- Continuation hash mismatch: Continuation is hidden and a compatible Job is
  submitted/retried;
- malformed canonical JSON or hash generation failure: Job submission fails as
  a planning error before queue insertion;
- database or Artifact Store failure: propagated as infrastructure failure, not
  silently converted into compatibility playback.

## Observability

Structured logs and diagnostics should include:

- media ID;
- profile ID;
- Encoding Plan version and hash;
- Startup Artifact and Continuation Artifact kinds;
- mismatch reason without leaking filesystem paths or authentication data.

The Artifact table remains the long-term source for published-plan auditing.

## Verification

Required automated coverage:

- deterministic canonical JSON and hash;
- hash changes when a compatibility field changes;
- hash does not change for execution range or backend changes because those are
  not Encoding Plan fields;
- Job-to-Artifact plan propagation;
- additive migration/backfill;
- Startup descriptor rejects blank or stale plan identity;
- Continuation submission copies the Startup plan identity;
- Continuation resolver rejects mismatched plan identity;
- Playback Plan exposes only safe version/hash diagnostics;
- Go full-package, race, Lite/Full build, Web build, Android contract, and Docker
  restart smoke verification.

## Non-goals of this phase

This phase does not remove `EXT-X-DISCONTINUITY` and does not claim
sample-perfect handoff. It establishes the persisted compatibility identity
required for later work on:

- timestamp origin and PTS/DTS normalization;
- AAC priming and encoder delay;
- exact codec profile/level and color metadata verification;
- backend capability attestations;
- segment-boundary checkpoint/resume;
- discontinuity-free client validation across Web, ExoPlayer, mpv, Emby, and
  Infuse.
