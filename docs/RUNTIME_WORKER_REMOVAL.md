# Persistent Runtime Worker Removal

## Status

The media-keyed persistent Runtime execution path has been physically removed
from the Lite and Full server source tree.

Runtime playback is now exclusively owned by `PlaybackSessionService`, while
administrator preprocessing remains an explicit durable product. Historical
Runtime records are data for retirement, cleanup, audit and rollback only.

## Removed execution surface

The refactor deletes the former execution implementation rather than keeping a
passive compatibility shell. Removed source includes:

- `TranscodeService` and its compatibility constructor;
- `TranscodeJob` and the in-memory priority queue;
- Job Claim, Lease heartbeat, expired-Lease recovery and Worker loops;
- FFmpeg Attempt execution, progress persistence and hardware fallback;
- media-level playback position, throttle and process suspension state;
- startup stream and continuation execution;
- Runtime Artifact publication, recovery and version resolution;
- persistent Runtime process-shutdown signal bridge;
- queue admission and storage reservation code tied to Runtime Jobs.

The old stream and administrator URLs remain only as authenticated `410 Gone`
tombstones during the client migration window. They cannot reach a repository,
Artifact reader or FFmpeg process.

## Remaining services

### MediaExecutionService

Owns process-local execution capabilities shared by:

- ephemeral Playback Sessions;
- direct play planning and Managed Remux;
- explicit administrator preprocessing.

It contains no persistent Runtime queue or media-keyed playback cache.

### ArtifactMaintenanceService

Owns only:

- historical Runtime intent retirement;
- old Runtime Artifact and workspace cleanup;
- Artifact storage health probes and incident evidence;
- disk-pressure reclamation of durable historical Artifacts;
- maintenance projections consumed by Task Center;
- ordered shutdown of the maintenance loop.

It contains no FFmpeg runtime, Worker, Job value, queue, Lease owner, playback
position or hardware backend.

## Data compatibility

The migration does not drop `transcode_tasks`, `transcode_jobs`,
`transcode_attempts`, Artifact or storage incident tables. Existing rows remain
available for:

- audit and incident investigation;
- cleanup and retirement evidence;
- Full → Lite → Full rollback certification;
- older-version rollback when explicitly required.

No new Runtime Job can be submitted, claimed or restored by the current server.

## Regression gates

Automated source and behavior tests require:

- old Worker, Queue, Lease, progress, throttle, startup and signal bridge files
  to be absent;
- no `TranscodeService`, `TranscodeJob` or `NewTranscodeService` declaration;
- Playback Session and Stream services to depend only on
  `MediaExecutionService`;
- Full and Lite assembly to expose only `ArtifactMaintenanceService`;
- old Runtime stream and administrator routes to resolve to `410` tombstones;
- Artifact cleanup, storage health, disk pressure and retirement tests to run
  without any Runtime queue fixture;
- Go package tests and both server binaries to build after physical deletion.

## Validation performed

The one-shot deletion gate completed successfully before committing the source
change:

- `go test ./...`;
- `go build ./cmd/server-lite`;
- `go build ./cmd/server`;
- explicit filesystem assertions for removed Runtime files;
- source scans rejecting `TranscodeService`, `TranscodeJob` and
  `NewTranscodeService`.

Regular Web, Go, Docker, SQLite upgrade/rollback, storage-fault, concurrency,
media timing and Android matrices are triggered again from this documentation
commit so the final non-bot head receives the repository's complete validation
suite.
