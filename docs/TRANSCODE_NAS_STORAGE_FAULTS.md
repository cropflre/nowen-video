# Transcode NAS Storage Fault Protocol

## Status

The NAS storage fault protocol is implemented on `refactor/server-lite-v1` for the shared Lite and Full transcode runtime.

Disk-pressure governance answers whether enough capacity remains. This protocol answers the independent operational question: can the Artifact Store still complete the metadata operations required for a safe immutable publication?

A mounted SMB/NFS path may continue to return filesystem statistics while writes fail because the share became read-only, the mount is stale, credentials changed, or the backend disappeared. Capacity sampling alone is therefore not treated as proof of writability.

## Real write probe

Every 30 seconds the service verifies the complete Artifact Store metadata path:

```text
stat store root
  -> create .health namespace
  -> create temporary file
  -> write payload
  -> fsync file
  -> close file
  -> atomic rename
  -> remove committed probe
```

The probe uses the same filesystem root as Attempt workspaces and published Artifacts. It does not write into the database volume and does not infer health from configuration or process permissions.

Successful probe files are removed immediately. One probe is also executed during startup before recovered Jobs may acquire a new storage Reservation.

## Normalized fault codes

Kernel and NAS errors are normalized into stable operational codes:

```text
no_space
read_only
permission_denied
unavailable
io_error
unknown
```

Representative mappings:

```text
ENOSPC                         -> no_space
EROFS                          -> read_only
EACCES / EPERM                 -> permission_denied
ENOENT / ENOTDIR / ENODEV      -> unavailable
ENXIO / stale file handle      -> unavailable
transport endpoint disconnected -> unavailable
EIO                            -> io_error
```

`no_space`, `unavailable`, and `io_error` are considered retryable because capacity or connectivity may recover without changing configuration. `read_only` and `permission_denied` require an administrator to repair mount mode, ownership, ACLs, or container volume permissions.

## Admission and execution behavior

When the write probe fails:

```text
storage_health.state = critical or degraded
storage_health.writable = false
storage_health.admission_blocked = true
storage_health.queue_paused = true
```

The failure is checked before a Job obtains its predictive storage Reservation and before Worker Claim. Therefore:

- new work may remain durably queued;
- queued Jobs do not obtain a Worker Lease;
- no new Attempt workspace is created;
- no new FFmpeg process starts;
- existing running FFmpeg processes retain their current Lease;
- cancellation and graceful shutdown remain available;
- published Artifacts that are still readable remain eligible for playback.

The protocol does not terminate existing FFmpeg processes solely because a probe failed. Their own process result, Lease, Artifact publication fence, and recovery workflow remain authoritative.

## Persistent incident evidence

Operational failures are stored in:

```text
transcode_storage_incidents
```

An incident records:

```text
code
severity
operation
path
message
retryable
admission_blocked
queue_paused
occurrences
first_seen_at
last_seen_at
recovered_at
status
```

States:

```text
active
recovered
```

The active key is derived from operation, path, and normalized code. Repeated probes of the same outage increment `occurrences` and update `last_seen_at` instead of creating a new row every 30 seconds.

Recovery clears the active key and preserves the row as history. If the same NAS outage occurs again later, a new incident row is created so separate operational episodes are not collapsed.

## Recovery boundary

An incident is not recovered merely because:

- `stat` succeeds;
- disk-usage sampling succeeds;
- the mount path exists again;
- an administrator manually dismisses a warning.

Recovery requires a successful full write probe including fsync, atomic rename, and removal. Once that succeeds:

1. active `artifact_store_probe` incidents are marked `recovered`;
2. the Task Center warning disappears on the next authoritative snapshot;
3. Reservation admission reopens when disk-pressure policy also permits it;
4. queued Jobs can be claimed normally.

There is intentionally no manual bypass. A false-positive pause is safer than starting an unpublishable transcode on a degraded NAS volume.

## Administrator visibility

`GET /api/admin/transcode-tasks/statistics` includes:

```text
storage_health.state
storage_health.code
storage_health.severity
storage_health.operation
storage_health.path
storage_health.message
storage_health.retryable
storage_health.writable
storage_health.admission_blocked
storage_health.queue_paused
storage_health.incident_id
storage_health.occurrences
storage_health.probe_latency_ms
storage_health.last_checked_at
storage_health.last_successful_at
storage_health.active_incidents
storage_health.critical_incidents
storage_health.recovered_count
storage_health.last_error
```

The Lite Task Center projects every active incident as:

```text
kind = storage_incident
status = failed
```

Storage incidents appear before ordinary cleanup failures and use the global red operational badge. The card displays the normalized cause, raw error, affected path, occurrence count, and whether automatic recovery is expected.

Storage incidents expose no `retry` or bypass action. The correct recovery action is to repair the underlying NAS mount, capacity, ownership, or ACL; the service then verifies it automatically.

## WebSocket lifecycle

Storage-health transitions broadcast:

```text
storage_health_updated
```

The event is mapped into the existing generic `task_updated` invalidation envelope:

```text
kind = storage_incident
status = failed | completed
source_id = incident_id
```

Clients refresh the authoritative Task Center snapshot rather than reconstructing incident state from event payloads.

## Certification

Dedicated workflow:

```text
.github/workflows/transcode-storage-fault-cert.yml
```

The gate certifies:

- wrapped kernel errno classification;
- a real Linux `/dev/full` write producing ENOSPC;
- real create/write/fsync/rename/remove probes;
- read-only directory detection and recovery;
- disconnected-root detection and recovery;
- active incident deduplication;
- occurrence counting;
- recovery history and recurrent-outage separation;
- Reservation/Worker admission pause;
- Task Center projection without unsafe actions;
- race safety across probe, repository, and service state.

The existing multi-job contention and Server Lite CI workflows continue to certify coexistence with Disk Pressure, Reservation, Job Lease, Artifact publication, Cleanup, Web build, Lite/Full binaries, and container startup.

## Remaining lab evidence

The deterministic CI gate does not claim to emulate every NAS implementation. Remaining physical-lab evidence includes:

- Linux NFS hard/soft mount timeout behavior during an in-flight FFmpeg write;
- SMB credential expiry and reconnect behavior;
- server-side share removal while file handles remain open;
- multipath failover and stale attribute-cache behavior;
- PostgreSQL plus shared NFS with multiple hosts;
- vendor NAS filesystem snapshots and quota exhaustion.

These scenarios should use the same normalized incident and recovery protocol rather than introducing vendor-specific runtime branches.
