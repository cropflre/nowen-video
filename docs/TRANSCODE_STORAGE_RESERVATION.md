# Transcode Predictive Storage Reservation

## Status

Predictive storage Reservation is implemented on `refactor/server-lite-v1` as the admission boundary between the durable transcode queue and the Worker Lease.

Disk-pressure governance reacts to real filesystem usage. Reservation adds the complementary forward-looking guarantee: a Job must prove that its predicted peak Artifact output fits the remaining storage headroom before a Worker can claim it and start FFmpeg.

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

Existing active Reservations are subtracted inside the serialized database transaction.

Disk pressure remains authoritative. A `pressure`, `critical`, or unavailable filesystem sample prevents new Reservations even when a stale calculation might otherwise appear to fit.

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
estimated_bytes
reserved_bytes
state
acquired_at
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

Updating its version is the serialization fence. The transaction obtains the database write/row lock before reading the active Reservation total, so two server instances cannot both observe and consume the same final headroom.

## Reservation ownership and recovery

Reservation belongs to the durable Job, not to one process or Attempt.

It therefore survives:

- Worker Lease renewal;
- hardware-to-software fallback Attempts;
- expired Lease recovery;
- graceful shutdown requeue;
- server restart.

A recovered Job reuses its existing active Reservation idempotently before obtaining a new Worker Lease.

Capacity accounting joins each Reservation back to an active `transcode_jobs` row. Once a Job becomes terminal and releases `active_key`, its Reservation immediately stops consuming effective headroom even if the audit row has not yet been reconciled from `active` to `released`.

Startup reconciliation marks such terminal audit rows `released` and records `released_at` without deleting their evidence.

## Interaction with physical usage

Reservation is a conservative peak commitment. Real workspace bytes are still included in filesystem usage, while the original Reservation remains active until Job terminal state. This intentionally favors preventing NAS exhaustion over maximizing concurrency.

A later refinement may persist consumed workspace bytes and reduce the remaining commitment dynamically. The current protocol does not guess consumption from progress percentages because FFmpeg progress is time-based and is not a reliable byte-allocation signal for VBR media.

## Failure behavior

Expected capacity shortage:

- leaves the Job queued;
- does not acquire a Worker Lease;
- does not create an Attempt or workspace;
- allows the scheduler to consider smaller candidates;
- is visible through the waiting Reservation count.

Unexpected database, media metadata, profile, or filesystem sampling failures also leave the Job queued and fail closed. The scheduler logs the durable reason rather than starting unreserved work.

Cancellation remains available while a Job waits for capacity. A cancelled or otherwise terminal Job no longer contributes to active Reservation totals.

## Observability

`GET /api/admin/transcode-tasks/statistics` includes:

```text
storage_reservation.active_count
storage_reservation.active_bytes
storage_reservation.waiting_count
storage_reservation.available_headroom_bytes
```

The scheduler identifier is:

```text
database_priority_fifo_storage_reserved
```

These fields distinguish actual Artifact Store usage from future bytes already promised to active Jobs.

## Certification

The multi-job contention gate certifies:

- profile bitrate parsing and deterministic estimates;
- remaining-duration calculations;
- bounded Startup Stream estimates;
- unknown-duration fail-closed behavior;
- per-Job idempotency;
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

- dynamic reduction of Reservation by measured workspace bytes;
- historical estimator calibration from predicted versus actual Artifact size;
- per-library or per-user storage quotas;
- operator controls for manually bypassing storage governance;
- real multi-host PostgreSQL/NFS contention certification on a NAS lab runner.

There is intentionally no manual bypass in this phase. Uncertain capacity remains queued rather than risking an incomplete Artifact or a full NAS volume.
