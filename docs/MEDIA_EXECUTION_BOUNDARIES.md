# Media Execution Boundaries

## Status

Runtime playback, managed remux, and durable administrator preprocessing now
share one stateless execution platform. Historical Runtime migration and
Artifact cleanup are owned by a separate maintenance service.

```text
MediaExecutionService
  ├─ PlaybackSessionService        ephemeral HLS
  ├─ StreamService                 direct play / managed remux / planning
  └─ PreprocessArtifactService     explicit durable output

ArtifactMaintenanceService
  └─ historical retirement / Artifact cleanup only
```

The retired persistent Runtime queue is not an execution path.

## MediaExecutionService

`MediaExecutionService` owns only process-local capabilities:

- FFmpeg execution runtime and resource governor;
- FFprobe cache service;
- detected hardware acceleration backend;
- configuration and structured logging.

It owns no:

- `transcode_tasks` projection;
- `transcode_jobs` queue;
- Job Lease or recovery worker;
- Runtime Artifact store;
- media/profile keyed playback cache;
- playback lifecycle state.

Lite and Full create one `MediaExecutionService` and inject that same instance
into playback sessions, playback planning, managed remux, system capability
reporting, and administrator preprocessing. The former
`playbackCompatibilityAdapter` has been removed; playback no longer constructs
or receives a `TranscodeService` object.

## Runtime playback

Runtime video transcoding is exclusively session-scoped:

```text
cache/playback-temp/sessions/<session>/<generation>
```

A session owns its FFmpeg process, rolling HLS window, active readers, seek
generations, heartbeat, close state, and cleanup. Closing or expiring a session
removes all generated media. Runtime playback never publishes or resolves a
durable Artifact.

Direct play and managed remux remain immediate stream responses. Managed remux
uses the same `MediaExecutionService` runtime and governor but creates no Job,
Lease, task projection, or reusable media Artifact.

## Administrator preprocessing

`PreprocessArtifactService` is the formal name for explicit durable media
preparation. The existing `PreprocessService` type remains as a source-compatible
alias during handler migration.

Preprocessing owns:

- administrator-created or policy-created tasks;
- serial background execution;
- pause, cancel, retry, and restart recovery;
- durable output under `cache/preprocess/<media>`;
- ABR policy, GPU safety, and VFS integration.

Full server binds preprocessing to the same `MediaExecutionService` hardware
capability used by playback. It does not obtain execution capability from
Artifact maintenance or the retired persistent Runtime queue.

Preprocessing output is not silently reused as a Runtime playback cache. The
Playback Planner may select a preprocessing asset only through the explicit
preprocess contract and source-fingerprint policy.

## ArtifactMaintenanceService

`ArtifactMaintenanceService` is the production construction boundary for the
remaining historical transcode domain. It owns only:

- startup and periodic retirement of historical Runtime intents;
- expired Lease fencing during rolling upgrades;
- durable Artifact ownership reconciliation;
- retryable Artifact filesystem cleanup;
- storage incident and cleanup evidence exposed to Task Center.

Its constructor does not create an FFprobe service, FFmpeg execution runtime,
hardware detector, or Runtime Worker. There is no `TranscodeService` alias, Runtime Job value, queue, Lease loop,
worker, attempt runner, throttle controller or submission API in the service
assembly. Historical database records remain data only and are consumed by the
retirement and cleanup sweepers.

## Retired persistent Runtime queue

The historical database queue is constructed with the invariant
`runtimeRetired=true`.

It must never:

- accept a Job;
- report submission capacity;
- Claim a queued row;
- start a persistent Runtime FFmpeg worker;
- expose historical queue depth as current work;
- restore a media-keyed Runtime Job after restart.

The queue object remains lifecycle-open until shutdown only so rolling-upgrade
maintenance can wake the retirement sweeper. Historical Runtime rows are fenced
and cleaned by migration maintenance, not by an execution worker.

## Retired stream and administrator APIs

Legacy media-keyed HLS, playback-position, bandwidth, and throttle routes remain
registered temporarily so old clients receive a deterministic migration
response. They return `410 Gone` with code
`persistent_runtime_hls_retired`. Runtime clients must create and maintain a
Playback Session instead.

After JWT validation and the administrator role check, legacy administrator
Runtime paths return `410 Gone` with code
`persistent_runtime_transcode_retired`:

- `/api/admin/transcode/*`;
- `/api/admin/transcode-tasks`;
- `/api/admin/transcode-tasks/*`;
- `/api/admin/tasks/transcode/*`.

Authorization is evaluated before the tombstone, and all tombstone responses
are `no-store`. Preprocessing routes are explicitly outside these matches and
continue to operate normally.

## Task Center

Lite Task Center no longer projects `transcode_tasks` as active work and no
longer exposes Runtime cancel/retry actions. It continues to show:

- media library scans;
- scrape tasks;
- storage incidents;
- durable Artifact cleanup operations.

Playback sessions are user playback state, not administrator background tasks.
Explicit preprocessing remains represented by its own APIs and models.

## Compatibility and migration

Historical tables and records remain intact for audit, upgrade, and rollback.
No destructive schema drop is performed.

Compatibility routes may remain during the migration window, but they resolve
only to authenticated `410 Gone` tombstones. No service compatibility method
can create a Job, Claim a Lease, read a Runtime Artifact or start FFmpeg.

## Regression gates

Automated tests enforce:

- one process-local `MediaExecutionService` runtime;
- Playback Session and Stream construction have no `TranscodeService`
  dependency;
- the playback compatibility adapter cannot return;
- media-keyed HLS service and Handler implementations cannot return;
- legacy stream and administrator routes resolve only to authenticated `410`
  tombstones;
- Artifact maintenance construction does not start a Runtime Worker;
- retired queues cannot accept, Claim, or restore Runtime work;
- Full preprocessing is bound to the shared media execution capability;
- Lite Task Center has no historical Runtime task projection or executor;
- source assembly cannot silently restore the removed execution path.
