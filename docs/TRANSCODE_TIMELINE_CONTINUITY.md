# Startup Handoff Timeline Continuity

## Status

This document defines the packet-level handoff contract between an immutable
Startup Artifact and a durable Startup Continuation Artifact.

Current schema:

```text
startup-handoff-timeline-v2
```

Current timestamp policy:

```text
hls-timestamp-normalization-v1
```

The contract records and classifies timeline evidence after binding it to the
versioned FFmpeg Timestamp Plan and each Job-owned timeline origin. It does
**not** authorize removal of `#EXT-X-DISCONTINUITY`.

## Why this is separate from other plans

Encoding Plan answers:

```text
What media format must both Artifacts produce?
```

Timestamp Plan answers:

```text
How must each execution preserve the source-relative timeline?
```

Produced-media Attestation answers:

```text
What did this individual Artifact actually contain?
```

Handoff Timeline Contract answers:

```text
What is the packet timestamp relation between the end of Startup and the
beginning of Continuation, under the declared timestamp execution policy?
```

These are distinct immutable facts. Startup and Continuation keep their own
`hls-produced-media-attestation-v1` records. The handoff contract references
both evidence identities, the shared Timestamp Plan identity, both execution
origins and the expected boundary.

## Domain model

A v2 handoff contract contains:

- Encoding Plan version and hash;
- Timestamp Plan version and hash;
- Startup timeline origin;
- Continuation timeline origin;
- expected boundary;
- Startup Produced-media Attestation version and hash;
- Continuation Produced-media Attestation version and hash;
- video PTS and DTS boundary relation;
- audio PTS and DTS boundary relation;
- exact stream time bases;
- delta values in stream ticks and microseconds;
- bounded comparison tolerance;
- aggregate status;
- explicit playback policy.

The contract canonical JSON is hashed with SHA-256. Evaluation time, database
IDs, Job IDs, Attempt IDs, Lease tokens and filesystem paths remain outside the
canonical payload.

For the current bridge:

```text
startup_timeline_origin_ms = 0
continuation_timeline_origin_ms = 30000
expected_boundary_ms = continuation_timeline_origin_ms
```

The expected boundary must equal the Continuation origin. A mismatched Job range
cannot produce a valid v2 contract.

## Timestamp relation

For each stream:

```text
startup_end_pts = startup.last.end_pts
continuation_first_pts = continuation.first.first_pts
presentation_delta = continuation_first_pts - startup_end_pts
```

Produced-media Attestation stores the final packet PTS and end PTS. The final
packet duration is therefore:

```text
final_packet_duration = end_pts - last_pts
```

The decode boundary is derived as:

```text
startup_end_dts = startup.last.last_dts + final_packet_duration
continuation_first_dts = continuation.first.first_dts
decode_delta = continuation_first_dts - startup_end_dts
```

Both deltas are converted using the actual ffprobe stream time base. The
contract never assumes a fixed MPEG-TS clock when the evidence declares a
different one.

## Classification

Each stream is classified as:

```text
aligned
gap
overlap
mixed
```

`aligned` requires both presentation and decode deltas to fall inside the
calculated tolerance. Tolerance is based on one eighth of the observed final
packet duration, clamped between 1 ms and 5 ms. This absorbs rounding noise but
does not treat a full video frame or AAC frame as continuous.

`gap` requires both deltas to be positive beyond tolerance.

`overlap` requires both deltas to be negative beyond tolerance.

`mixed` covers contradictory PTS/DTS direction or a video/audio disagreement.

The aggregate status is aligned only when both video and audio are aligned.

## Safety policy

Schema v2 has the same hard invariant as v1:

```text
seamless_allowed = false
discontinuity_required = true
```

Even an `aligned` result uses:

```text
decision_reason = client_certification_pending
```

Timestamp normalization and packet arithmetic are evidence, not a production
feature flag. Removing the discontinuity requires a new schema after real-media
and cross-client certification.

A missing, malformed, stale or unreadable contract also fails closed and keeps
the discontinuity. The current Bridge explicitly recognizes only the current
schema and cannot trust a future or unknown record that claims seamless
permission.

## Persistence

The additive table remains:

```text
transcode_handoff_attestations
```

Its internal identity remains:

```text
startup_artifact_id
continuation_artifact_id
schema_version
```

The v2 row additionally stores:

```text
timestamp_plan_version
timestamp_plan_hash
startup_timeline_origin_ms
continuation_timeline_origin_ms
expected_boundary_ms
```

The same Artifact pair is updated when a live Continuation changes from a
`provisional` first-segment attestation to a final `verified` attestation.
Normal EVENT playlist reloads read the existing row and do not rerun ffprobe or
rewrite the database when evidence identities are unchanged.

No historical v1 row is rewritten. Existing Startup v2 / Continuation v3
Artifacts and handoff rows remain stored, but Startup v3 / Continuation v4
resolvers require the complete Timestamp Plan contract and create a v2 handoff
record.

## Bridge behavior

The server-owned EVENT playlist performs this sequence:

```text
resolve published Startup Artifact by full execution contract
resolve current Lease-valid Continuation Artifact by full execution contract
validate both Produced-media Attestations
validate first packets against each Job-owned timeline origin
validate actual stream identity compatibility
validate shared Timestamp Plan identity
evaluate or reuse handoff timeline v2 contract
append Startup segments
append EXT-X-DISCONTINUITY according to the contract policy
append Continuation segments
```

The HTTP client never receives canonical timestamp or timeline JSON, packet
timestamps, Artifact IDs or internal paths.

## Expected result after normalization

Before Timestamp Plan v1, the separate Continuation process commonly restarted
near the MPEG-TS muxer origin, producing approximately a full startup-range
overlap.

Startup v3 and Continuation v4 now run under `copyts_start_at_zero`, preserve the
input seek as the Continuation timeline origin and disable automatic negative
timestamp shifting. The first-packet gate rejects a 30-second Continuation whose
evidence restarts near 1.4 seconds.

The intended result is now stable source-relative timestamps and a measurable
v2 handoff classification. It is still possible for the exact packet boundary
to be `gap`, `overlap` or `mixed` because keyframe seek, B-frame decode order,
AAC priming and MPEG-TS mux behavior remain real media properties. Those
results are retained as evidence and never hidden by removing the discontinuity.

## Migration and rollback

Migration is additive:

- add Timestamp Plan and origin columns to Job and Artifact tables;
- extend handoff persistence with timestamp identity and origin projections;
- do not alter media files;
- do not rewrite old Produced-media Attestation or v1 handoff JSON;
- do not backfill unverifiable historical evidence.

Rollback behavior:

- an older binary ignores the additive columns;
- Startup, Continuation and historical handoff rows remain unchanged;
- a later re-upgrade requires the current planner and complete execution
  contract before reuse.

When an Artifact is removed by retention cleanup, handoff rows referencing that
Artifact are removed first.

## Verification

Required automated coverage:

- deterministic v2 canonical identity;
- timestamp identity and origin included in the hash;
- exact aligned boundary classification;
- gap classification;
- overlap classification;
- stream identity mismatch rejection;
- expected boundary/origin mismatch rejection;
- schema v2 cannot authorize seamless playback;
- additive database migration;
- deterministic upsert for provisional to verified Continuation evidence;
- missing or unknown policy fails closed in the EVENT playlist;
- exactly one discontinuity remains;
- value-object, repository and service race tests;
- handoff lookup and execution-contract resolver performance baselines;
- Lite/Full build and persistent-volume restart smoke tests.

## Next phase

The next formal phase is real FFmpeg fixture certification. It must measure,
not assume:

- CFR and VFR video;
- B-frame presentation/decode order;
- 44.1 kHz and 48 kHz AAC priming and padding;
- HDR-to-SDR conversion;
- source files with non-zero initial timestamps;
- source keyframes before and after the 30-second boundary;
- software baseline and each hardware backend independently;
- restart, cancellation and fallback behavior.

Only after normalized output produces stable evidence can cross-client
certification begin. A future contract may authorize discontinuity removal only
after hls.js, Safari native HLS, ExoPlayer, mpv, Emby and Infuse all pass the
same handoff fixture suite.
