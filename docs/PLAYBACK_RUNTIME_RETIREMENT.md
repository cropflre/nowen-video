# Runtime Playback Storage Retirement

Runtime video transcoding is now owned exclusively by `PlaybackSessionService`.
The server no longer creates, resolves, publishes, recovers, or reuses durable
Runtime HLS / Startup Stream Artifacts.

## Storage ownership

| Capability | Storage root | Lifetime |
| --- | --- | --- |
| Direct play | source media | request only |
| Managed Remux / Smart Remux | pipe / HTTP response | request only |
| Runtime transcode | `cache/playback-temp/sessions/<session>/<generation>` | playback session |
| Administrator preprocessing | `cache/preprocess/<media>` | explicit persistent asset |
| Historical transcode evidence | database rows without playable files | audit only |

A runtime Generation contains its playlist, video segments, and selected audio
track in one directory. Seek creates a new Generation. Closing or expiring the
session cancels FFmpeg, drains readers, and removes every Generation directory.
Rolling HLS limits the active directory to a bounded segment window.

## Retired HTTP paths

The following compatibility paths return `410 Gone` with
`playback_session_required` and never enter Artifact or media-scoped cache code:

- `/api/stream/:id/master.m3u8`
- `/api/stream/:id/:quality/:segment`
- `/api/audio-track/:id/:trackIdx`
- `/api/audio-track/:id/:trackIdx/:segment`
- Startup Bridge playlist and segment paths
- Emby quality-scoped `hls1/:quality` playlist and segment paths

Current clients create a session through `POST /api/playback/sessions`. Emby and
Infuse bind HLS to their external `PlaySessionId`.

## Startup behavior

Media scan completion still submits FFprobe warm-up. It does not submit Startup
Stream or Runtime HLS jobs and therefore does not create playback files before
a user starts watching.

At server startup and every lease-recovery interval, the retirement sweep:

1. Finds jobs with runtime, startup, continuation, on-demand video, or on-demand
   audio intents.
2. Clears `active_key`, requests cancellation, and prevents recovery.
3. Defers filesystem deletion while another instance owns a valid Lease.
4. Cancels and retires the job after the Lease expires.
5. Removes linked runtime Artifact, Attempt workspace, legacy task output,
   media-scoped audio, quality, and on-demand directories.
6. Changes the cleaned job intent to `retired_runtime_playback` so the sweep is
   idempotent while preserving historical execution evidence.

## Protected persistent data

The retirement sweep does not remove:

- `cache/preprocess`
- `cache/playback-temp`
- the `cache/transcode/artifacts` namespace root
- the `cache/transcode/workspaces` namespace root
- unrelated Artifact kinds or administrator assets

Only validated child paths below `cache/transcode` are eligible for deletion.
The exact transcode root and namespace roots are rejected by the path boundary.

## Rolling upgrade behavior

When an older server instance still owns a runtime Job Lease, the new instance
only changes the job to `cancel_requested`; it does not remove the workspace.
The older worker fails its next Lease renewal and exits. A later sweep removes
the workspace after the Lease expiration time. This prevents the new instance
from deleting files still being written by a valid worker.

Runtime playback sessions themselves are intentionally not recovered after a
server restart. `PlaybackSessionService` removes orphan session directories on
startup, and clients create a new session.
