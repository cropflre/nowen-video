# Media Execution Boundaries

## Status

Runtime playback and durable administrator preprocessing are now separate
products with one shared stateless execution platform.

```text
MediaExecutionService
  ├─ PlaybackSessionService        ephemeral
  └─ PreprocessArtifactService     explicit durable output
```

The retired persistent Runtime queue is not a third path.

## MediaExecutionService

`MediaExecutionService` owns only process-local capabilities:

- FFmpeg execution runtime;
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

Lite and Full construct playback sessions through
`NewPlaybackSessionServiceWithExecution`. The compatibility adapter passed to
the current session constructor contains only the execution runtime, Probe
service, hardware backend, configuration, and logger. Repository, queue,
Artifact Store, worker registry, and Lease fields remain nil.

## Runtime playback

Runtime playback is exclusively session-scoped:

```text
cache/playback-temp/sessions/<session>/<generation>
```

A session owns its FFmpeg process, rolling HLS window, active readers, seek
generations, heartbeat, close state, and cleanup. Closing or expiring a session
removes all generated media. Runtime playback never publishes or resolves a
durable Artifact.

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
capability used by playback. It does not obtain execution capability from the
retired persistent Runtime queue.

Preprocessing output is not silently reused as a Runtime playback cache. The
Playback Planner may select a preprocessing asset only through the explicit
preprocess contract and source-fingerprint policy.

## Retired persistent Runtime queue

The historical database queue is constructed with an invariant
`runtimeRetired=true`.

It must never:

- accept a Job;
- report submission capacity;
- Claim a queued row;
- start a persistent Runtime FFmpeg worker;
- expose historical queue depth as current work;
- restore a media-keyed Runtime Job after restart.

The queue object remains lifecycle-open until shutdown only so rolling-upgrade
maintenance can continue to wake the retirement sweeper. Historical Runtime
rows are fenced and cleaned by the dedicated retirement migration, not by an
execution worker.

## Task Center

Lite Task Center no longer projects `transcode_tasks` as active work and no
longer exposes Runtime cancel/retry actions. It continues to show:

- media library scans;
- scrape tasks;
- storage incidents;
- durable Artifact cleanup operations.

Playback sessions are user playback state, not administrator background tasks.
Explicit preprocessing remains represented by its own preprocessing APIs and
models rather than being relabeled as Runtime transcode work.

## Compatibility and migration

Historical tables and records remain intact for audit, upgrade, and rollback.
No destructive schema drop is performed.

Compatibility methods may remain while callers migrate, but they must fail
closed and cannot create Jobs, Claim Leases, read Runtime Artifacts, or start
FFmpeg outside a Playback Session or explicit preprocessing task.

## Regression gates

Automated tests enforce:

- queue construction is permanently retired;
- retired queues cannot accept, Claim, or restore Runtime work;
- Playback Session construction uses `MediaExecutionService`;
- the playback compatibility adapter has no persistent repositories or worker
  state;
- Full preprocessing is bound to the shared media execution capability;
- Lite Task Center has no historical Runtime task projection or executor;
- source assembly cannot silently restore the removed execution path.
