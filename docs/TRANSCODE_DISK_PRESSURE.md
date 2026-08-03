# Transcode Artifact Store Disk Pressure Protocol

## Status

Disk-pressure governance is implemented on `refactor/server-lite-v1` for both Lite and Full server profiles.

The Artifact Store remains the single filesystem boundary for managed HLS workspaces and immutable published versions. Disk pressure is handled inside the transcode orchestrator rather than by an external directory sweeper, so admission, durable Job claims, Artifact state, Cleanup Leases, playback reads, and observability all use the same source of truth.

## Objectives

The protocol prevents a nearly full NAS volume from entering a write-amplification loop where queued FFmpeg Jobs continue to start while cleanup cannot recover enough space.

It guarantees:

- existing FFmpeg processes keep their current Job Lease;
- no new transcode submission is accepted while pressure remains active;
- queued database Jobs remain durable but are not claimed by Workers;
- staging and publishing Artifacts are never pressure-cleanup candidates;
- recently read published or superseded Artifacts are protected;
- every filesystem removal continues through the durable Cleanup Lease and retry state machine;
- failure to sample the filesystem fails closed instead of assuming capacity is available.

## Policy

The built-in defaults are:

```text
filesystem high watermark: 90%
filesystem low watermark:  80%
minimum free space:         2 GiB
critical free space:       512 MiB
active playback grace:      15 minutes
published cache grace:      24 hours
```

`cache.max_disk_usage_mb`, when configured, is also enforced as the Artifact Store byte limit. A value of `0` keeps the existing unlimited-store setting, while filesystem high-watermark and free-space protection remain active.

The policy exposes three levels:

```text
normal
pressure
critical
```

Pressure reasons are stable machine-readable values:

```text
filesystem_high_watermark
minimum_free_space
artifact_store_limit
critical_free_space
recovery_hysteresis
disk_sample_unavailable
```

## Hysteresis

The governor does not resume work immediately after crossing just below the high watermark.

Recovery requires all configured conditions:

- filesystem usage is at or below the low watermark;
- free space is at least 125% of the minimum-free threshold;
- when a store limit is configured, Artifact Store usage is at or below 85% of that limit.

This prevents Workers from repeatedly stopping and restarting near one threshold.

## Admission and scheduling

The pressure state is sampled before startup recovery can launch Workers.

While the state is `pressure` or `critical`:

- `CanAccept` rejects new submissions;
- the database-backed Priority/FIFO queue does not claim another queued Job;
- already claimed/running Jobs continue under their current Lease;
- cancellation and shutdown behavior remain available;
- queued rows remain `queued` and recover automatically after the governor returns to `normal`.

The same rule applies to runtime HLS, Startup Stream warmup, Startup Continuation, background batch submission, and retries because all paths enter the same durable queue.

## Reclaim tiers

Pressure reclaim is ordered and bounded.

### Tier 1: terminal evidence

The governor first queues old terminal Artifacts:

```text
expired
superseded
failed
cancelled
abandoned
```

Priority is:

1. expired;
2. superseded;
3. failed;
4. cancelled;
5. abandoned.

Rows already owned by cleanup, waiting for a retry, or blocked by an invariant are not overwritten.

### Tier 2: published cache

If pressure remains after Tier 1, published cache older than 24 hours may be expired.

Priority is:

1. runtime `hls_variant`;
2. `startup_continuation_hls`;
3. `startup_hls`.

Startup media is retained longest because it provides the highest playback-start benefit for its stored duration.

The governor processes bounded batches and re-samples the real filesystem between batches. It stops when the low-watermark recovery condition is reached or no safe candidate remains.

## Active playback protection

Managed HLS reads emit a throttled durable access signal for:

- runtime manifests;
- runtime segments;
- Startup Bridge discovery and playlists;
- Startup Stream segments;
- Startup Continuation manifests and segments.

The signal is written asynchronously and at most once per Artifact every 30 seconds. Pressure cleanup excludes any Artifact accessed within the previous 15 minutes.

This complements the existing open-file protection:

- Unix readers keep an already-open descriptor after unlink;
- Windows/SMB readers can cause a busy deletion, which enters the Cleanup retry schedule;
- the access grace protects the playback session before cleanup reaches filesystem deletion.

## Cleanup ownership

Pressure governance never calls an unrestricted `RemoveAll` path.

Every selected Artifact is converted to the normal cleanup lifecycle:

```text
pending -> claimed -> deleted
                 \-> retry_wait
                 \-> blocked
```

Cleanup still validates the path against the Artifact Store root, obtains a two-minute Cleanup Lease, fences metadata deletion with the cleanup token, and persists NAS/mount failures with capped backoff.

## Observability

`GET /api/admin/transcode-tasks/statistics` includes:

```text
disk_pressure.level
disk_pressure.reasons
disk_pressure.total_bytes
disk_pressure.free_bytes
disk_pressure.used_bytes
disk_pressure.used_percent
disk_pressure.store_bytes
disk_pressure.max_store_bytes
disk_pressure.high_watermark_percent
disk_pressure.low_watermark_percent
disk_pressure.min_free_bytes
disk_pressure.critical_free_bytes
disk_pressure.reclaim_target_bytes
disk_pressure.admission_blocked
disk_pressure.queue_paused
disk_pressure.last_reclaim_at
disk_pressure.last_reclaimed_bytes
disk_pressure.last_reclaimed_rows
disk_pressure.last_error
```

The existing Artifact status and cleanup-state counters remain available in the same response.

## Certification

The multi-job contention gate runs disk policy, service, repository, Cleanup Lease, and real FFmpeg tests under the existing certification workflow:

```text
.github/workflows/transcode-multi-job-contention-cert.yml
```

It certifies:

- high/low watermark hysteresis;
- minimum-free and store-limit pressure;
- bounded reclaim targets;
- old terminal and published Artifact selection;
- recent-access exclusion;
- actual filesystem and metadata deletion;
- admission rejection;
- durable queue Claim pause;
- Cleanup Lease and stale-token fencing;
- real FFmpeg multi-job contention.

## Remaining evidence boundary

This phase does not yet certify:

- filling a real NFS/SMB NAS volume to each watermark in CI;
- filesystem quotas that report capacity differently from the host mount;
- coordinated pressure sampling across multiple hosts that mount the same remote share;
- product UI for editing built-in watermark and reserve defaults;
- predictive space reservation based on estimated output size before a long transcode starts.

The runtime remains fail-closed when storage capacity cannot be measured or when safe candidates cannot recover the configured reserve.
