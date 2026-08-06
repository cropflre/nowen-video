# Legacy Transcode Projection Retirement

`transcode_jobs`, `transcode_attempts` and `transcode_artifacts` are the only
maintained Runtime history model. The old `transcode_tasks` table is no longer
created by Lite or Full migrations and has no write, retry, cancel, progress or
retention code path.

Existing deployments keep the table unchanged for database rollback and audit.
Runtime History may count it when present, but production code treats it as an
optional read-only import source.

## Legacy directory inventory

Terminal legacy rows with an `output_dir` are imported idempotently into a
deterministic historical Job and a `legacy_hls_directory` Artifact. The file is
not removed immediately. The Artifact receives a seven-day rollback deadline
and enters the same durable Cleanup Lease state machine used by all other
Artifact reclamation.

Invalid or out-of-root paths are blocked with evidence and are never removed.
Missing paths become completed audit tombstones.

## Rollback

During the seven-day observation window, and only before cleanup is claimed,
an administrator can choose **保留目录** in Task Center. This changes the
Artifact to `rollback_completed` and removes it from the cleanup work set
without modifying the directory or resurrecting the old Runtime executor.

The action disappears when the persisted deadline expires. The repository
rechecks the same deadline atomically, so a stale browser cannot extend the
window. A claimed or completed cleanup cannot be rolled back because filesystem
work may already have started.

## Cleanup evidence

Successful cleanup no longer deletes `transcode_artifacts`. It clears the live
path fields and records:

- `cleanup_state=completed`
- completion time and disposition
- original path, temporary path and manifest path
- retained byte count and migration source

Runtime History therefore remains auditable while storage summaries count only
files that may still exist.

## Cursor generations and source retirement

The inventory source is no longer scanned from the beginning on every maintenance
interval. `legacy_transcode_projection_migrations` stores one durable state row
for `legacy_transcode_task_v1` with:

- a finite per-generation source high-water
- the last committed `(updated_at, id)` cursor
- cumulative row and Artifact counters
- a database Lease for multi-instance ownership
- persisted cumulative and consecutive failure evidence
- the next retry and source-tail-check timestamps
- a 30-day read-only source retirement review date

A batch advances the cursor only after every row in that batch has been imported.
Partial Job or Artifact writes are safe because their identifiers are deterministic
and the batch is replayed after a failure. If a rollback to an older server creates
newer legacy rows, the next completed-generation tail check opens a new generation
beginning at the previous high-water.

A running or failed generation uses only its frozen high-water and does not query
the legacy source again. After completion, the maintenance loop reads only the
durable state row until a 15-minute source-check deadline is reached. A new source
count is calculated only when the tail high-water has actually advanced.

The migration Lease is renewed during directory traversal and is checked again
immediately before Artifact insertion. The heartbeat uses the migration clock plus
real elapsed time, so historical upgrade recovery and production execution share
one consistent time domain. An expired owner cannot renew or commit a batch after
another instance takes over.

Retry delay is based on consecutive failures and is capped, while cumulative
failure count remains permanent audit evidence. A successful batch or explicit
administrator retry resets only consecutive failure pressure.

The retirement date is evidence for an explicit later schema-removal decision; it
does not automatically drop `transcode_tasks`.

## Legacy Source Retirement Decision

The retirement decision is a separate, explicit administrator protocol. It does
not reuse migration cursor state as an approval field and it does not execute a
schema migration. The Lite administrator API exposes:

- `GET /api/admin/legacy-source-retirement/:source`
- `POST /api/admin/legacy-source-retirement/:source/decisions`

The report recomputes a stable evidence hash from the current migration generation,
cursor/high-water, 30-day observation timestamps, source-row inventory, rows that
have no matching `transcode_jobs.legacy_task_id`, and every still-open Artifact
rollback deadline. A submitted decision must include the hash the administrator
reviewed. Any new legacy row, migration generation, cursor movement or rollback
boundary change rejects the write as stale evidence.

The protocol supports `approve`, `defer` and `reject`. `defer` and `reject` require
an administrator reason. `approve` additionally requires all of the following:

- the legacy source table still exists and is the object being reviewed
- the migration generation is completed
- `quiescent_since` and `source_retire_after` prove the 30-day observation period
- no legacy row with an output directory remains without a migrated Job
- no migrated Artifact still has an open rollback window
- a backup reference and checksum have been verified
- a restore test has been recorded with its own timestamp

Each review is appended to `legacy_source_retirement_decisions` with protocol
version, reviewer identity, immutable evidence JSON, SHA-256 evidence hash,
backup proof, decision and reason. Approval means only **eligible for a future,
separately reviewed schema-removal migration**. It does not mutate or delete a
legacy row, close an Artifact rollback window, or run `DROP TABLE`.

A future removal change must still be a normal, reversible database migration
that checks the latest approved decision against fresh evidence immediately
before DDL. This phase intentionally contains no automatic or scheduled source
deletion.

## Acceptance

- Fresh Lite and Full database profiles no longer create `transcode_tasks`.
- Existing legacy tables survive migration unchanged and are used only as a
  bounded, read-only inventory source.
- Legacy directories receive deterministic Job and Artifact records, a persisted
  seven-day observation window, path fencing and Cleanup Lease ownership.
- Rollback is available only before both the persisted deadline and cleanup
  claim; the Task Center and repository enforce the same boundary.
- Successful retention and disk-pressure cleanup reclaim files while preserving
  completed Artifact tombstones and original-path evidence.
- Migration batches are ordered by `(updated_at, id)`, commit a durable cursor,
  and freeze a finite high-water for each generation.
- Concurrent instances are fenced by a renewable database Lease; stale owners
  cannot renew, complete a batch or publish an Artifact after takeover.
- Completed generations avoid per-tick source counts and perform only a bounded
  15-minute tail check before deciding whether to open the next generation.
- Task Center and Runtime History expose generation, progress, failure evidence,
  source-check schedule and retirement-review readiness.
- Retirement reports aggregate the durable 30-day observation, direct unmigrated
  row count, rollback-window evidence and latest decision.
- Administrator decisions are append-only, hash-fenced against stale evidence and
  require verified backup plus restore-test proof before approval.
- Approval leaves `transcode_tasks`, its rows and all migration state untouched;
  this phase contains no `DROP TABLE` or automatic source-retirement worker.
- Focused migration and Lease tests, the complete Go package suite, Lite and Full
  builds, and the Web production build passed in the dedicated implementation
  gate. The normal project matrix is rerun from this acceptance commit.
