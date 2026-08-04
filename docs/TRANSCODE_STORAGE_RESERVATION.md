# Transcode Predictive Storage Reservation

## Status

Predictive storage Reservation is implemented on `refactor/server-lite-v1` as the admission boundary between the durable transcode queue and the Worker Lease.

Disk-pressure governance reacts to real filesystem usage. Reservation adds the complementary forward-looking guarantee: a Job must prove that its predicted peak Artifact output fits the remaining storage headroom before a Worker can claim it and start FFmpeg.

The current protocol also measures the active Attempt workspace, refunds bytes already represented by the physical disk sample, and records the final predicted-versus-actual Artifact error for estimator calibration.

## Scheduling boundary

The runtime order is:

```text
ordered queued candidate
  -> predict Artifact bytes
  -> acquire persistent storage Reservation
  -> atomically Claim Job Lease
  -> create Attempt workspace
  -> start FFmpeg
```

Reservation is deliberately not acquired when the HTTP or warmup path submits a Job. Submission remains durable and cheap; a queued Job does not monopolize physical capacity while higher-priority or already-running work completes.

If the highest-priority candidate does not fit, it stays queued. The Worker continues scanning the bounded candidate window so a smaller Job may proceed instead of allowing one oversized movie to block the complete queue.

## Estimation policy

The estimator consumes the shared runtime profile catalog, so quality bitrate policy has one authority.

For known duration:

```text
payload_bytes = (video_bitrate + audio_bitrate) * duration_seconds / 8
reserved_bytes = max(64 MiB, ceil(payload_bytes * 1.35))
```

The 35% envelope covers CRF/VBR excursions, MPEG-TS overhead, manifests, segment boundary variance, and ordinary profile/source complexity differences.

Duration selection:

1. use `transcode_jobs.duration_ms` for bounded Startup Stream or other explicitly bounded work;
2. otherwise use authoritative media duration minus `start_ms`;
3. fall back to the media runtime field when only minutes are known.

For unknown duration:

```text
reserved_bytes = max(64 MiB, ceil(source_file_size * 1.10))
```

If neither duration nor source size is known, admission fails conservatively to a 2 GiB Reservation floor. This prevents remote or incomplete metadata from bypassing storage governance with a zero-byte estimate.

## Available capacity

A Reservation may spend only the smallest of these current headrooms:

```text
filesystem high-watermark headroom
free bytes above the minimum-free-space floor
configured Artifact Store max-size headroom
```

Capacity accounting uses:

```text
physical Artifact Store usage
+ sum(max(reserved_bytes - observed_bytes, 0))
```

`observed_bytes` is already included in the physical filesystem/store sample. Subtracting the complete original Reservation after those bytes materialize would count them twice and unnecessarily suppress safe concurrency.

Disk pressure remains authoritative. A `pressure`, `critical`, or unavailable filesystem sample prevents new Reservations even when a stale calculation might otherwise appear to fit.

## Consumption observation protocol

The running Attempt workspace is measured at most once every 30 seconds. Progress parsing only schedules the observation; directory traversal and disk sampling run outside the FFmpeg progress pipe.

The refund order is deliberately strict:

```text
measure current Attempt workspace
  -> invalidate cached Store usage
  -> force a fresh filesystem and Store sample
  -> verify current Job Lease Token and Attempt ID
  -> persist observed_bytes
```

The database write is serialized through the same singleton Ledger used for Reservation acquisition. A stale Worker, replaced Attempt, expired Lease, or unavailable disk sample cannot reduce the remaining commitment.

Observation is monotonic within one Attempt. Hardware-to-software fallback resets `observed_bytes` for the new Attempt because the replacement output must receive a complete future commitment again. `peak_observed_bytes` remains monotonic across all Attempts for diagnostics.

A final synchronous observation is performed before cancellation, failure, or atomic Artifact publication.

## Persistent records

Two tables are used:

```text
transcode_storage_reservations
transcode_storage_ledger
```

A Job Reservation records:

```text
job_id
media_id
profile_id
intent
attempt_id
estimated_bytes
reserved_bytes
observed_bytes
peak_observed_bytes
final_bytes
prediction_error_bytes
actual_to_estimate_ratio
observation_count
outcome
state
acquired_at
last_observed_at
released_at
created_at
updated_at
```

States:

```text
active
released
```

The singleton ledger row is named:

```text
artifact_store
```

Updating its version is the serialization fence. The transaction obtains the database write/row lock before reading or changing active Reservation commitment, so multiple server instances cannot spend the same headroom or race a consumption refund.

## Reservation ownership and recovery

Reservation belongs to the durable Job, not to one process or Attempt.

It therefore survives:

- Worker Lease renewal;
- hardware-to-software fallback Attempts;
- expired Lease recovery;
- graceful shutdown requeue;
- server restart.

A recovered Job reuses its existing active Reservation idempotently before obtaining a new Worker Lease.

Capacity accounting joins each Reservation back to an active `transcode_jobs` row. Once a Job becomes terminal and releases `active_key`, its Reservation immediately stops consuming effective headroom.

Failed and cancelled Jobs close their Reservation audit row after the terminal transition while retaining peak workspace evidence. Successful publication records the immutable Artifact size, prediction error, and actual-to-estimate ratio.

Startup recovery first repairs the narrow crash window where Artifact/Job publication committed but calibration evidence did not. Remaining terminal audit rows are then reconciled to `released` without deleting evidence.

## Prediction calibration evidence

For a published Artifact:

```text
prediction_error_bytes = final_bytes - estimated_bytes
actual_to_estimate_ratio = final_bytes / estimated_bytes
```

A positive error means underprediction. A ratio below `1.0` means the initial envelope was conservative.

Aggregated statistics expose:

```text
calibration sample count
average actual-to-estimate ratio
average absolute relative error
underpredicted sample count
```

This phase records evidence but does not automatically mutate the shared profile catalog. Policy changes remain explicit and reviewable rather than allowing a small or biased local sample to silently weaken storage safety.

## Failure behavior

Expected capacity shortage:

- leaves the Job queued;
- does not acquire a Worker Lease;
- does not create an Attempt or workspace;
- allows the scheduler to consider smaller candidates;
- is visible through the waiting Reservation count.

Unexpected database, media metadata, profile, or filesystem sampling failures also leave the Job queued and fail closed. Observation failures retain the previous full future commitment rather than refunding uncertain bytes.

Cancellation remains available while a Job waits for capacity. A cancelled or otherwise terminal Job no longer contributes to active Reservation totals.

## Observability

`GET /api/admin/transcode-tasks/statistics` includes:

```text
storage_reservation.active_count
storage_reservation.active_bytes
storage_reservation.reserved_bytes
storage_reservation.observed_bytes
storage_reservation.remaining_bytes
storage_reservation.waiting_count
storage_reservation.available_headroom_bytes
storage_reservation.calibration_samples
storage_reservation.average_actual_to_estimate
storage_reservation.average_absolute_error
storage_reservation.underpredicted_count
```

`active_bytes` remains a compatibility alias of `remaining_bytes`.

The scheduler identifier is:

```text
database_priority_fifo_storage_reserved
```

These fields distinguish actual Artifact Store usage, total predicted output, already-materialized output, and future bytes still promised to active Jobs.

## Certification

The multi-job contention gate certifies:

- profile bitrate parsing and deterministic estimates;
- remaining-duration calculations;
- bounded Startup Stream estimates;
- unknown-duration fail-closed behavior;
- per-Job idempotency;
- observed-byte refund without physical double counting;
- Lease and Attempt fencing of observations;
- fallback Attempt observation reset with retained peak evidence;
- published predicted-versus-actual calibration;
- failed/cancelled audit release;
- terminal capacity release;
- waiting-count projection;
- concurrent ledger serialization;
- exactly one winner when two instances compete for the same final headroom;
- Reservation persistence before Worker Claim;
- coexistence with disk pressure, Job Lease, Artifact publication, cleanup and real FFmpeg contention.

Workflow:

```text
.github/workflows/transcode-multi-job-contention-cert.yml
```

## Remaining evidence

This phase does not yet provide:

- automatic estimator coefficient updates from local calibration samples;
- per-library or per-user storage quotas;
- operator controls for manually bypassing storage governance;
- real multi-host PostgreSQL/NFS contention certification on a NAS lab runner.

There is intentionally no manual bypass. Uncertain capacity remains queued rather than risking an incomplete Artifact or a full NAS volume.
