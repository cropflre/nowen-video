# Runtime History

The persistent Runtime worker has been physically removed. The historical
SQLite tables remain as audit evidence and are exposed through a dedicated
read-only domain.

## Authority

- `transcode_jobs` is the authoritative historical execution record.
- `transcode_attempts` stores backend, exit and diagnostic evidence.
- `transcode_artifacts` stores Artifact lifecycle and cleanup evidence.
- `transcode_tasks` is a legacy compatibility projection only. It is linked by
  `legacy_task_id` and never becomes an execution source again.

## API

All endpoints require an authenticated administrator:

- `GET /api/admin/runtime-history`
- `GET /api/admin/runtime-history/summary`
- `GET /api/admin/runtime-history/jobs/:id`

The API intentionally has no POST, PUT, PATCH or DELETE operation. It cannot
submit, retry, cancel, recover, claim or lease work.

Sensitive fields are omitted from responses: FFmpeg command JSON, workspace
paths, Artifact paths, temporary paths and manifest paths.

## Retention

Execution metadata is retained indefinitely for audit and rollback evidence.
There is no automatic metadata deletion. Artifact file content remains bounded
by `ArtifactMaintenanceService`, disk-pressure reclamation and cleanup retry
state. Cleanup failures stay visible until cleanup succeeds or an operator fixes
the underlying storage problem.
