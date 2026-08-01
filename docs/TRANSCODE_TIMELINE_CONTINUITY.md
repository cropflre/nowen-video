# Startup Handoff Timeline Continuity

## Status

This document defines the first packet-level handoff contract between an
immutable Startup Artifact and a durable Startup Continuation Artifact.

Current schema:

```text
startup-handoff-timeline-v1
```

This phase records and classifies timeline evidence. It does **not** authorize
removal of `#EXT-X-DISCONTINUITY`.

## Why this is separate from Produced-media Attestation

Produced-media Attestation answers:

```text
What did this individual Artifact actually contain?
```

The handoff contract answers:

```text
What is the packet timestamp relation between the end of Startup and the
beginning of Continuation?
```

These are distinct immutable facts. Startup and Continuation keep their own
`hls-produced-media-attestation-v1` records. The handoff contract references the
version and hash of both records and is recomputed when either identity changes.

## Domain model

A handoff contract contains:

- Encoding Plan version and hash;
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
IDs, Job IDs, Attempt IDs, Lease tokens and filesystem paths are outside the
canonical payload.

## Timestamp relation

For each stream:

```text
startup_end_pts = startup.last.end_pts
continuation_first_pts = continuation.first.first_pts
presentation_delta = continuation_first_pts - startup_end_pts
```

Produced-media Attestation v1 stores the final packet PTS and end PTS. The final
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

Schema v1 has a hard invariant:

```text
seamless_allowed = false
discontinuity_required = true
```

Even an `aligned` result uses:

```text
decision_reason = client_certification_pending
```

This prevents packet arithmetic alone from becoming an accidental production
feature flag. Removing the discontinuity requires a new schema version after
client certification.

A missing, malformed, stale or unreadable contract also fails closed and keeps
the discontinuity.

## Persistence

The additive table is:

```text
transcode_handoff_attestations
```

Its internal identity is:

```text
startup_artifact_id
continuation_artifact_id
schema_version
```

The row stores:

- canonical contract JSON and hash;
- both Produced-media Attestation identities;
- aggregate status and decision reason;
- video/audio presentation and decode deltas;
- seamless/discontinuity policy;
- evaluation timestamp.

The same Artifact pair is updated when a live Continuation changes from a
`provisional` first-segment attestation to a final `verified` attestation.
Normal EVENT playlist reloads read the existing row and do not rerun ffprobe or
rewrite the database.

No historical row is synthesized during migration. Existing Startup and
Continuation Artifacts remain stored, but a handoff contract is created only
when both sides have valid Produced-media Attestation evidence.

## Bridge behavior

The server-owned EVENT playlist performs this sequence:

```text
resolve published Startup Artifact
resolve current Lease-valid Continuation Artifact
validate both Produced-media Attestations
validate actual stream identity compatibility
evaluate or reuse handoff timeline contract
append Startup segments
append EXT-X-DISCONTINUITY according to the contract policy
append Continuation segments
```

The HTTP client never receives canonical timeline JSON, packet timestamps,
Artifact IDs or internal paths.

## Current expected result

The current Continuation encoder seeks with an input-side `-ss` and starts a
separate FFmpeg process. MPEG-TS timestamps may therefore restart near the muxer
origin rather than continue from the Startup final packet.

The expected production evidence for the current implementation is commonly:

```text
overlap
```

That result is useful: it proves why the discontinuity must remain and provides
a measurable baseline for the next FFmpeg timestamp-normalization phase.

## Migration and rollback

Migration is additive:

- create `transcode_handoff_attestations`;
- do not alter media files;
- do not rewrite Produced-media Attestation JSON;
- do not backfill unverifiable historical decisions.

Rollback behavior:

- an older binary ignores the new table;
- Startup and Continuation Artifacts remain unchanged;
- a later re-upgrade can reevaluate the boundary from their persisted
  Produced-media Attestations.

When an Artifact is removed by retention cleanup, handoff rows referencing that
Artifact must also be removed.

## Verification

Required automated coverage:

- deterministic canonical identity;
- exact aligned boundary classification;
- gap classification;
- overlap classification;
- stream identity mismatch rejection;
- schema v1 cannot authorize seamless playback;
- additive database migration;
- deterministic upsert for provisional to verified Continuation evidence;
- missing policy fails closed in the EVENT playlist;
- exactly one discontinuity remains for the v1 contract;
- value-object and repository race tests;
- handoff lookup performance baseline;
- Lite/Full build and persistent-volume restart smoke tests.

## Next phase

The next implementation phase is FFmpeg timestamp normalization under a new
planner version. It must define, per backend:

- input seek semantics;
- output timestamp origin;
- video PTS/DTS offset;
- audio priming, encoder delay and padding evidence;
- MPEG-TS mux delay and initial timestamp behavior;
- keyframe boundary identity;
- rollback to the current discontinuity path.

Only after normalized output produces stable aligned evidence can cross-client
certification begin. A future contract version may authorize discontinuity
removal only after hls.js, Safari native HLS, ExoPlayer, mpv, Emby and Infuse
all pass the same handoff fixture suite.
