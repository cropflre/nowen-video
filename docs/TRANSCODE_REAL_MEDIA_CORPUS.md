# Real Media Corpus v1

## Status

Real Media Corpus v1 introduces a separately versioned, file-backed source domain for transcode certification.

It does not reuse or expand the historical synthetic `FixtureSpec`. The old fixture matrix remains backward-compatible and keeps its existing meaning. The new corpus owns its own source intent, byte identity, observed probe metadata, and future certification output.

Phase 1 in this document implements the immutable **Spec** and **Manifest** contracts. It does not claim that the media assets or their certification reports already exist.

Production policy remains fail-closed:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Why a separate domain is required

The existing fixture certification is deliberately small and synthetic:

- one generated MP4 source shape;
- fixed CFR behavior;
- limited audio variants;
- no independent container registry;
- no immutable source digest contract.

Adding MP4 edit-list behavior, Matroska VFR, MPEG-TS source timestamps, Opus, and additional timeline origins to that structure would turn one fixture type into several unrelated models and make old evidence ambiguous.

Real Media Corpus therefore has two explicit identities:

```text
real-media-corpus-spec-v1
real-media-corpus-manifest-v1
```

## Spec contract

The Spec is the immutable intent for the corpus. Every case declares:

- stable case ID, description, purpose, and tier;
- source container;
- video codec, profile, pixel format, dimensions, and color metadata;
- CFR or VFR policy and exact rational rates;
- GOP, B-frame, reference-frame, and open-GOP policy;
- audio codec, sample rate, channel layout, and track count;
- duration, timestamp origin, edit-list policy, and discontinuity policy;
- Startup/Continuation boundary;
- the complete evidence set required before the source can influence production decisions.

The v1 required evidence set is:

```text
timestamp_plan
output_cadence
packet_order
produced_media_attestation
boundary_packet
av_boundary_sync
```

The canonical Spec JSON is hashed with SHA-256. Case ordering is part of the contract identity.

## Manifest contract

The Manifest binds one resolved corpus execution to actual files. Every asset must contain:

- the exact Spec version and Spec SHA-256;
- a stable case ID;
- a corpus-root-relative path;
- file SHA-256 and byte size;
- generator, FFmpeg, and FFprobe versions;
- observed container and codec identities;
- observed duration and start timestamp;
- video dimensions, pixel format, frame-rate mode, rational rates, and time base;
- audio codec, sample rate, channel count, track count, and time base;
- observed B-frame reorder classification.

Relative paths are diagnostic. The authoritative source identity is:

```text
case_id + file_sha256
```

Absolute paths and paths that escape the corpus root are rejected.

## Corpus tiers

v1 defines two tiers:

| Tier | Meaning |
|---|---|
| `deterministic_container` | media bytes produced by the pinned corpus generator and then consumed through real file demuxers |
| `curated_external` | immutable externally sourced media with recorded provenance and SHA-256 |

The initial registry contains deterministic container-backed cases. Curated external assets require a separate provenance and licensing review before inclusion.

## Initial case registry

Every initial source is 40 seconds long with a 30-second certification boundary.

| Case | Container | Video | Audio | Timeline purpose |
|---|---|---|---|---|
| `real-mp4-h264-aac-cfr-24000-1001-v1` | MP4 | H.264 CFR 24000/1001, B2 | AAC 48 kHz | cinematic rational cadence |
| `real-mp4-h264-aac-cfr-30000-1001-edit-list-v1` | MP4 | H.264 CFR 30000/1001, B3 | AAC 48 kHz | positive origin and edit-list behavior |
| `real-mkv-h264-aac-vfr-24-30-v1` | Matroska | H.264 VFR 24 → 30, B3 | AAC 48 kHz | file-backed VFR demuxing |
| `real-mpegts-h264-aac-cfr-30-b3-v1` | MPEG-TS | H.264 CFR 30, B3 | AAC 48 kHz | transport timestamp origin and packet ordering |
| `real-mkv-h264-opus-cfr-25-v1` | Matroska | H.264 CFR 25, B2 | Opus 48 kHz | non-AAC input audio and Matroska time bases |
| `real-mp4-h264-aac-cfr-30-aac-44100-v1` | MP4 | H.264 CFR 30, B3 | AAC 44.1 kHz | input audio resampling and boundary projection |

All v1 video sources are progressive SDR BT.709 H.264 High Profile `yuv420p`. Interlaced, HDR, HEVC, AV1, open-GOP, discontinuous-source, multi-audio, and subtitle cases remain explicit later phases rather than hidden extensions to v1.

## Package boundaries

Domain package:

```text
internal/transcode/realmediacorpus
```

Responsibilities:

- immutable Spec and Manifest models;
- canonical SHA-256 identity;
- default case registry;
- validation of source plans, resolved assets, probe evidence, and fail-closed policy.

The package does not execute FFmpeg, write media files, or choose a production encoder time base.

## CLI

List the registry:

```bash
go run ./cmd/transcode-real-media-corpus-spec -list
```

Write the canonical Spec report:

```bash
go run ./cmd/transcode-real-media-corpus-spec \
  -output /tmp/real-media-corpus-spec-v1.json
```

The report includes:

```text
schema_version = real-media-corpus-spec-report-v1
spec_version   = real-media-corpus-spec-v1
spec_hash      = <sha256>
```

## CI

Workflow:

```text
.github/workflows/transcode-real-media-corpus-spec.yml
```

Semantic verifier:

```text
.github/scripts/verify_real_media_corpus_spec.py
```

The workflow:

1. runs the real-media corpus Go contract tests;
2. builds the standalone Spec command;
3. produces canonical Spec JSON;
4. verifies exact case ordering and required evidence;
5. verifies MP4, Matroska, and MPEG-TS coverage;
6. verifies CFR/VFR, AAC/Opus, 44.1/48 kHz, edit-list, and non-zero-origin coverage;
7. verifies fail-closed seamless/discontinuity policy;
8. uploads the canonical Spec as a CI artifact.

## Phase 2: deterministic asset generator

The next implementation phase must add a generator that:

1. creates each source in an isolated directory;
2. uses pinned, normalized FFmpeg commands;
3. writes real MP4, Matroska, or MPEG-TS files;
4. computes byte size and SHA-256;
5. probes every file with FFprobe;
6. verifies observed metadata against the Spec;
7. writes `real-media-corpus-manifest-v1`;
8. rejects any asset whose observed identity differs from intent;
9. uploads the assets and Manifest as short-lived CI artifacts.

The generator must never mutate the Spec to match observed output. A mismatch is a failed build, not a dynamic contract update.

## Later gates

After deterministic assets exist, each source must run through both explicit encoder time-base candidates and bind:

- Timestamp Plan;
- Output Cadence Evidence;
- Encoder Time-Base Candidate Evidence;
- Encoder Time-Base Reorder Evidence;
- Produced-media Attestation;
- Boundary Packet Evidence;
- A/V Boundary Sync Evidence.

Further independent gates are still required for:

- curated real-world media;
- hardware encoders;
- long-duration drift;
- HEVC, AV1, HDR, interlaced, subtitles, multi-audio, and discontinuous sources;
- Web, PC, Android, Emby, and Infuse client acceptance;
- rollout, fallback, and rollback.

No Real Media Corpus evidence can remove `#EXT-X-DISCONTINUITY` or enable a production encoder time-base candidate by itself.
