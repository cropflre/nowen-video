# Transcode Artifact Cleanup Protocol

## Status

The first durable Artifact cleanup protocol is implemented on `refactor/server-lite-v1`.

Artifact publication and playback are immutable/versioned. Cleanup is therefore a separate lifecycle with its own ownership, retry schedule, error evidence, and observability. A cleanup failure must never silently delete metadata, expose a partially removed version, or cause multiple server instances to remove the same Artifact concurrently.

## Cleanup lifecycle

`transcode_artifacts` persists these fields:

```text
cleanup_state
cleanup_attempts
cleanup_token
cleanup_claimed_at
cleanup_lease_expires_at
cleanup_next_attempt_at
cleanup_last_attempt_at
cleanup_error_code
cleanup_error_message
```

Cleanup states:

```text
pending
claimed
retry_wait
blocked
```

A successful cleanup deletes the Artifact row, so there is no long-lived `completed` row.

## Cleanup ownership

Cleanup uses a two-minute database Lease.

A candidate may be claimed only when:

- it is a terminal Artifact whose retention period has elapsed;
- it is `pending` or `retry_wait` and its persisted next-attempt time is due; or
- it is `claimed` but the previous cleanup Lease has expired.

The claim atomically stores a new token, claim time, Lease expiry, last-attempt time, and increments the attempt counter. Filesystem deletion and metadata deletion are allowed only while the token remains current.

An expired cleanup Lease can be recovered by another server instance. A stale token cannot delete the Artifact row or dependent handoff attestations.

## Task-owned cleanup

When an old compatibility task is removed:

- a currently `published` Artifact becomes `expired` before deletion;
- new Resolver requests can no longer select that Artifact;
- the Artifact enters the same durable cleanup state machine;
- the compatibility task remains in the database while cleanup is deferred or owned elsewhere;
- the task row is deleted only after all tracked Artifact rows have been removed.

Historical legacy directories without imported Artifact rows remain bounded to the legacy media/profile directory and keep the previous compatibility cleanup behavior.

## Failure classification

Retryable failures include:

```text
filesystem_busy
mount_unavailable
filesystem_io
filesystem_permission
cleanup_io
```

Examples include open-file sharing violations, busy directories, stale NFS handles, disconnected transport endpoints, temporary network failures, read-only remounts, and ordinary filesystem I/O errors.

Invariant failures become `blocked` and require operator intervention:

```text
cleanup_invariant_violation
```

Examples include a path escaping the configured Artifact Store root or an unavailable/uninitialized Artifact Store.

## Backoff

Cleanup retries use the persisted schedule:

```text
1 minute
5 minutes
15 minutes
1 hour
6 hours
12 hours
24 hours
```

Later attempts remain capped at 24 hours. Retry metadata updates do not restart the Artifact retention period; retry eligibility is driven by `cleanup_next_attempt_at`.

## Metadata commit

After all tracked paths are absent, cleanup removes dependent handoff attestations and the Artifact row in one database transaction. The transaction rechecks the cleanup token before deleting either record type.

Partial filesystem progress is safe. If one path was removed before a later path or database operation failed, the next attempt treats the already-missing path as success and continues from the remaining evidence.

## In-flight readers

Managed HLS segment responses open the segment file before sending response headers and serve from that open handle.

- On Unix, an in-flight descriptor remains readable after the path is unlinked.
- On Windows and mounts that reject deletion of an open file, removal fails as busy and enters the durable retry schedule.

This protects an active segment transfer without keeping retired Artifact versions indefinitely. New requests to an expired or removed explicit Artifact version continue to fail closed.

## Observability

The transcode statistics response includes:

```text
artifact_cleanup_state_counts
```

Operators can distinguish pending, currently claimed, retrying, and blocked cleanup work. Per-Artifact records retain attempt count, next attempt, last error code, and last error message until successful deletion.

## Certification

The multi-job contention workflow runs the cleanup tests under the race detector:

```text
.github/workflows/transcode-multi-job-contention-cert.yml
```

The gate certifies:

- exclusive cleanup claims;
- stale-token fencing;
- expired cleanup-Lease recovery;
- persisted retry scheduling;
- retry timing independent from retention timestamps;
- busy storage recovery;
- invariant failure classification;
- capped backoff;
- versioned read consistency and real FFmpeg contention.

## Remaining boundary

This phase does not yet certify:

- real NFS/SMB fault injection on a NAS runner;
- host reboot during the filesystem-delete/database-delete boundary;
- cleanup throughput limits for millions of Artifact rows;
- administrative retry/unblock operations for `blocked` rows;
- disk-pressure-triggered retention shortening;
- distributed cleanup scheduling across multiple simultaneously active servers.

Those remain subsequent evidence gates. Cleanup continues to fail closed and retains database evidence whenever ownership or invariants are uncertain.
