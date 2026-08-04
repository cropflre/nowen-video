# Startup Stream Retirement Record

## Status

Startup Stream is retired from production playback.

The historical protocol names remain documented only for database migration,
rollback analysis and incident archaeology:

```text
startup_stream / event_bridge_v1
startup-hls-v3
startup-continuation-hls-v4
```

No current planner selects `startup_stream`. No current service creates a
Startup or Continuation Job, publishes a Startup Artifact, reads one back, or
builds an EVENT bridge.

## Current playback behavior

Playback selection is now:

```text
Direct Play
  -> Managed Remux / Smart Remux
  -> ephemeral Playback Session Generation
```

A transcode plan returns `session_required=true` and a session template. The
client creates the session through:

```text
POST /api/playback/sessions
```

The Session Generation owns its playlist, selected audio track and rolling HLS
segments below `cache/playback-temp`. Closing or expiring the session cancels
FFmpeg, drains active readers and removes the Generation directory.

## Removed production implementation

The following runtime implementation has been physically removed:

- Startup eligibility, profile and active-key planning;
- bounded 30-second Startup VOD command rewriting;
- Startup Continuation Job creation and queue submission;
- Startup and Continuation Artifact resolution;
- Produced-media handoff attestation orchestration;
- EVENT bridge playlist parsing and construction;
- virtual Startup profile and segment parsing;
- immutable Startup / live Continuation segment serving;
- scan-time Startup submission hooks and counters.

The durable transcode executor no longer contains Startup-specific Artifact
kinds or FFmpeg argument branches.

## Compatibility boundaries

Historical JSON payloads may still contain the `startup_stream` field, so the
response value object remains decodable. New plans always leave it empty.

Historical database rows are still recognized by these Artifact kinds during
the retirement sweep:

```text
startup_hls
startup_continuation_hls
```

The constants exist only for cleanup. They are not executable capabilities.

Cached Startup Bridge URLs continue to return:

```http
410 Gone
Cache-Control: no-store
```

with error code:

```text
playback_session_required
```

This tombstone response prevents old clients from silently entering a missing
or partially removed playback path.

## Upgrade behavior

At startup and during Lease recovery, the runtime retirement sweep:

1. fences historical Startup and Continuation jobs;
2. clears active keys and requests cancellation;
3. waits for any valid old-instance Lease to expire;
4. removes linked runtime files and Artifact rows;
5. preserves execution evidence by changing the cleaned Job intent to
   `retired_runtime_playback`.

The sweep does not remove administrator preprocessing assets or active Playback
Session directories.

## Rollback boundary

Database schema changes remain additive, but a binary that still expects the
old Startup protocol must not be rolled forward beside this version and assume
shared runtime files remain available. The new version intentionally removes
those files after Lease-safe retirement.

A rollback therefore restores software compatibility only; it does not restore
retired Runtime/Startup media files. Clients will create a fresh Playback
Session or use an explicit administrator-generated preprocessing asset.

## Verification

Automated source gates prevent reintroduction of:

- Startup Bridge service files;
- handoff attestation runtime orchestration;
- on-demand FFmpeg segment generation;
- Runtime Artifact filesystem resolution and legacy import;
- Probe Warmup Startup submission callbacks.

Normal Go, Web, Lite/Full container and Android gates continue to validate the
Session-only playback architecture.
