# Transcode Packet Boundary Evidence

## Status

`hls-boundary-packet-evidence-v1` is a diagnostic certification contract for the packet-level relationship between one Startup HLS Artifact and one Continuation HLS Artifact.

It does not replace:

- `hls-encoding-plan-v1`, which states intended output identity;
- `hls-timestamp-normalization-v1`, which states FFmpeg timestamp execution policy;
- `hls-produced-media-attestation-v1`, which proves the first and last materialized segments;
- `startup-handoff-timeline-v2`, which classifies the high-level cross-Artifact relationship.

Boundary Evidence answers a narrower question:

> Which actual video frames and audio packets occupy the Startup tail and Continuation head, and how many nominal packets separate or overlap them?

The v1 contract is evidence only. It can never authorize removal of the HLS discontinuity.

```text
seamless_allowed = false
discontinuity_required = true
```

## Why this is a separate contract

Produced-media Attestation is part of durable Artifact identity. Expanding its canonical payload with packet windows or FFprobe-specific side data would change every Artifact hash and require a migration of persisted evidence.

Boundary Evidence therefore references, but does not rewrite, both Attestation identities:

```text
Startup Attestation version/hash
Continuation Attestation version/hash
Encoding Plan version/hash
Timestamp Plan version/hash
FFmpeg and FFprobe versions
```

The certification JSON is uploaded as CI evidence. No database table, Artifact column, backfill, or historical proof fabrication is introduced in this phase.

## Registered boundary matrix

`ffmpeg-boundary-placement-matrix-v1` uses immutable registered cases around a 30-second reference boundary.

| Case | Fixture | Exact boundary |
| --- | --- | ---: |
| `boundary-48k-keyframe-v1` | zerolatency, AAC 48 kHz | 30.000000 s |
| `boundary-48k-video-before-v1` | zerolatency, AAC 48 kHz | 29.966667 s |
| `boundary-48k-video-after-v1` | zerolatency, AAC 48 kHz | 30.033333 s |
| `boundary-48k-audio-before-v1` | zerolatency, AAC 48 kHz | 29.978667 s |
| `boundary-48k-audio-after-v1` | zerolatency, AAC 48 kHz | 30.021333 s |
| `boundary-44k1-keyframe-v1` | zerolatency, AAC 44.1 kHz | 30.000000 s |
| `boundary-44k1-audio-before-v1` | zerolatency, AAC 44.1 kHz | 29.976780 s |
| `boundary-44k1-audio-after-v1` | zerolatency, AAC 44.1 kHz | 30.023220 s |

The matrix deliberately uses only production-shaped software fixtures. The historical default-B-frame fixture remains in the overlap-attribution matrix and is not mixed into boundary-placement certification.

## Observed CI baseline

The first complete matrix was produced on Ubuntu 24.04 with FFmpeg/FFprobe 6.1.1. These values are reproducible evidence for that toolchain and runner image, not cross-version constants.

| Case | Video relation | Video delta | Audio relation | Audio delta | Audio samples |
| --- | --- | ---: | --- | ---: | ---: |
| 48k keyframe | single-packet overlap | -21.333 ms | multi-packet overlap | -58.667 ms | -2816 |
| 48k one video frame before | single-packet overlap | -21.333 ms | multi-packet overlap | -49.333 ms | -2368 |
| 48k one video frame after | single-packet overlap | -21.333 ms | multi-packet overlap | -46.667 ms | -2240 |
| 48k one AAC packet before | single-packet overlap | -21.333 ms | multi-packet overlap | -58.667 ms | -2816 |
| 48k one AAC packet after | single-packet overlap | -21.333 ms | multi-packet overlap | -58.667 ms | -2816 |
| 44.1k keyframe | single-packet overlap | -23.222 ms | multi-packet overlap | -46.622 ms | -2056 |
| 44.1k one AAC packet before | single-packet overlap | -23.222 ms | multi-packet overlap | -46.622 ms | -2056 |
| 44.1k one AAC packet after | single-packet overlap | -23.222 ms | multi-packet overlap | -46.611 ms | -2056 |

The packet-derived video rate is 30,000 milli-fps in all eight cases. The video overlap is approximately 0.64 of one nominal frame at 48 kHz and 0.697 of one nominal frame at 44.1 kHz. The small difference between the two fixtures follows a produced-media packet-duration difference rather than a declared source-rate change.

Boundary movement did not remove the video overlap. Moving by one AAC packet also did not move the 48 kHz audio result, while moving by one video frame changed the 48 kHz audio overlap. This indicates that the observed boundary is governed by the combined encoder/muxer packet schedule, not by a simple independent `-ss` offset per stream.

FFprobe exposed generic `MPEGTS Stream ID` packet side data, but no `skip_samples` or `discard_padding` evidence in the retained packet windows. The contract therefore reports:

```text
side_data_observed = false
```

This means relevant encoder-delay metadata was not observed. It does not mean encoder delay or padding is zero.

## Exact seek Adapter

The shared FFmpeg builder historically formats input-side `-ss` with two decimal places. That is sufficient for normal whole-second requests, but it quantizes:

- one 30 fps frame, approximately 33.333 ms;
- one AAC 48 kHz packet, approximately 21.333 ms;
- one AAC 44.1 kHz packet, approximately 23.220 ms.

Certification uses `ffmpeg.WithInputSeekMicros` after the shared production builder has constructed the command. The Adapter:

- copies the argument vector instead of mutating the caller;
- keeps legacy values such as `30.00` unchanged;
- preserves exact values such as `30.033333` and `29.978667`;
- replaces an existing input-side `-ss` or inserts it before `-i`;
- leaves sub-500 ms inputs unchanged, matching the existing builder threshold.

This phase does not apply that Adapter to normal runtime transcode Jobs. Production adoption would change execution identity and must be introduced through a new persisted Timestamp Execution Plan rather than as an unversioned command mutation.

## Packet acquisition

For each matrix case, certification creates:

```text
synthetic H.264/AAC source
  -> bounded Startup VOD Artifact
  -> Continuation EVENT Artifact
  -> Startup Produced-media Attestation
  -> Continuation Produced-media Attestation
  -> Startup last TS packet probe
  -> Continuation first TS packet probe
  -> hls-boundary-packet-evidence-v1
```

FFprobe supplies:

- stream index and type;
- stream time base;
- audio sample rate;
- packet PTS and DTS;
- packet duration;
- keyframe flag;
- packet side data when exposed by the container and FFprobe build.

FFprobe also reports stream-level average and real frame rates. Short or partial TS segments can report different average-rate values for the same CFR stream, so those fields are retained only as probe observations and are not authoritative contract identity. The canonical video rate is derived from median packet duration and time base:

```text
frame_rate_milli = round(time_base_denominator * 1000 /
                         (time_base_numerator * packet_duration_ticks))
```

The contract validates this projection independently.

The contract retains at most:

```text
last 6 Startup packets
first 6 Continuation packets
```

It also stores full-segment packet counts and range summaries, so the selected window can be verified against the source segment.

## Presentation and decode boundaries

For each stream, the packet probe calculates robust segment endpoints instead of assuming that the last demuxed packet also has the greatest PTS.

```text
Startup presentation end = max(packet PTS + packet duration)
Continuation presentation start = min(packet PTS)
Presentation delta = Continuation start - Startup end
```

```text
Startup decode end = max(packet DTS + packet duration)
Continuation decode start = min(packet DTS)
Decode delta = Continuation decode start - Startup decode end
```

The contract stores both tick and microsecond forms. Validation recomputes every derived delta from the packet summaries.

This matters for B-frame sources because presentation order and decode order are different domains. A PTS-only endpoint cannot explain decoder reordering.

## Nominal packet units

The nominal packet duration is the median positive duration across the Startup-tail and Continuation-head segments.

The presentation delta is projected into packet units:

```text
boundary_units_milli = round(delta_ticks * 1000 / nominal_duration_ticks)
```

Examples:

```text
-1000 = one nominal packet overlap
  000 = aligned within tolerance
+1000 = one nominal packet gap
```

The status vocabulary is:

```text
aligned
single_packet_gap
multi_packet_gap
single_packet_overlap
multi_packet_overlap
```

Tolerance is derived from one nominal packet duration and bounded to 1–5 ms. It is diagnostic tolerance only and is not a seamless-playback allowance.

## AAC delay and padding evidence

For audio, the contract additionally projects ticks into samples using the observed sample rate:

```text
samples = round(ticks * time_base * sample_rate)
```

It records:

- nominal packet samples;
- boundary delta samples;
- Startup and Continuation `skip_samples` totals;
- Startup and Continuation `discard_padding` totals;
- whether relevant encoder-delay or padding side data was observed.

AAC-LC normally represents 1024 samples per access unit, but MPEG-TS uses a 90 kHz clock and may alternate integer packet durations. The contract therefore derives the value from produced media instead of hard-coding a timestamp duration.

All packet side data remains in the raw packet windows. `side_data_observed` is narrower: it becomes true only for `skip_samples`, `discard_padding`, or an explicitly named skip/discard side-data record. Generic MPEG-TS stream metadata does not count as encoder-delay evidence.

Absence of relevant FFprobe side data is represented as:

```text
side_data_observed = false
```

It is not interpreted as proof that encoder delay or padding is zero.

## Identity and integrity

Every evidence contract has deterministic canonical JSON and SHA-256 identity.

The containing case report validates that:

- the case metadata exactly matches the immutable registry;
- case, fixture, and exact boundary identities match the evidence;
- the evidence hash can be recomputed;
- every packet microsecond projection matches its ticks and time base;
- packet ordinals match the head or tail window position;
- packet durations and segment summaries are positive and consistent;
- video rate matches packet duration and time base;
- AAC sample projections match the packet timeline;
- generic container side data cannot become encoder-delay evidence;
- v1 remains fail-closed.

The matrix additionally requires all registered cases in canonical order and one consistent FFmpeg/FFprobe toolchain identity.

## CLI

List registered fixtures and boundary cases:

```bash
go run ./cmd/transcode-fixture-cert -list
```

Run the existing overlap-attribution matrix:

```bash
go run ./cmd/transcode-fixture-cert \
  -all \
  -output ./artifacts/cfr-overlap-attribution-matrix-v1.json
```

Run the packet boundary matrix:

```bash
go run ./cmd/transcode-fixture-cert \
  -boundary-matrix \
  -output ./artifacts/boundary-placement-matrix-v1.json
```

`-all` and `-boundary-matrix` are intentionally mutually exclusive so one output path cannot silently change schema.

## CI verification

`.github/workflows/transcode-fixture-cert.yml` produces and uploads both matrices. CI verifies:

- Go contracts and the precise seek Adapter;
- all registered case identities and exact boundaries;
- Produced-media and Timestamp Plan identities;
- deterministic contract hashes;
- packet-window size and ordinal rules;
- presentation and decode tick arithmetic;
- positive nominal durations;
- packet-derived video rate;
- AAC nominal sample range;
- explicit relevant-side-data observation state;
- fail-closed seamless/discontinuity policy.

Exact measured overlap values are printed and stored as evidence, not embedded as permanent pass/fail constants. FFmpeg versions, muxer behavior, and runner images can lawfully change the values while the contract remains internally valid.

## Compatibility and rollback

The change is additive:

- no database migration;
- no Produced-media Attestation schema change;
- no Timeline v2 reinterpretation;
- no playback API change;
- no bridge playlist change;
- no production FFmpeg command change.

Older binaries ignore the new CI JSON. Rolling back removes the boundary package, CLI mode, workflow steps, and documentation without transforming media or database rows.

## Non-claims

This phase does not prove:

- sample-perfect Startup-to-Continuation handoff;
- absence of AAC encoder priming;
- client decoder continuity;
- browser, ExoPlayer, mpv, Emby, or Infuse no-reset behavior;
- hardware encoder equivalence;
- VFR, open-GOP, HDR, non-zero source timestamp, or 33-bit MPEG-TS wrap behavior.

`#EXT-X-DISCONTINUITY` remains mandatory.

## Next evidence phases

After this CFR software baseline is stable, certification should add:

1. 24 fps and 25 fps boundaries;
2. variable-frame-rate inputs;
3. non-zero and negative source timestamps;
4. open-GOP and longer B-frame chains;
5. explicit audio offset and resampling fixtures;
6. QSV, NVENC, and VAAPI as independent backend identities;
7. repeated-run variance and FFmpeg-version comparison;
8. client-specific playback observations.

Only a later schema with explicit backend and client certification may evaluate whether a seamless protocol is safe. It must not reinterpret Boundary Evidence v1 or Timeline v2.
