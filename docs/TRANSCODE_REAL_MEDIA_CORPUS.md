# Real Media Corpus v1

## Status

Real Media Corpus v1 is a separately versioned, file-backed source domain for transcode certification. It does not reuse or expand the historical synthetic `FixtureSpec`.

The current implementation includes:

1. immutable Spec and Manifest contracts;
2. a six-case deterministic source registry;
3. a real MP4, Matroska, and MPEG-TS asset generator;
4. two-pass byte determinism verification;
5. FFprobe and MP4 box evidence bound into the Manifest;
6. independent Spec and asset-generation CI workflows.

It does not yet run the generated files through both encoder time-base candidates. Production remains fail-closed:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Why a separate domain is required

The historical fixture matrix is deliberately small and synthetic. Adding MP4 edit-list behavior, Matroska VFR, MPEG-TS timestamp origins, Opus, color metadata, and immutable file digests to that structure would make old evidence ambiguous.

Real Media Corpus therefore owns two explicit identities:

```text
real-media-corpus-spec-v1
real-media-corpus-manifest-v1
```

## Spec contract

Every case declares:

- stable case ID, description, purpose, and tier;
- source container;
- video codec, profile, pixel format, dimensions, and BT.709 metadata;
- CFR or VFR policy and exact rational rates;
- GOP, B-frame, reference-frame, and open-GOP policy;
- audio codec, sample rate, layout, and track count;
- duration, timestamp origin, edit-list, and discontinuity policy;
- Startup/Continuation boundary;
- the evidence set required before the source can influence production decisions.

Required downstream evidence:

```text
timestamp_plan
output_cadence
packet_order
produced_media_attestation
boundary_packet
av_boundary_sync
```

The canonical Spec JSON is hashed with SHA-256. Case ordering is part of the identity.

Current Spec SHA-256:

```text
ae9623f2c051868115a926b3a5cf881fbb58cc3408ff82a52ace1905332267fc
```

## Manifest contract

The Manifest binds one generator execution to real files. It records:

- Spec version and SHA-256;
- generator, FFmpeg, and FFprobe versions;
- exactly two generation repeats;
- canonical asset order;
- normalized FFmpeg command SHA-256;
- final file SHA-256 and byte size;
- the SHA-256 from both independent generation repeats;
- observed container, codecs, profile, pixel format, dimensions, and color metadata;
- observed start, duration, frame count, CFR/VFR rates, and stream time bases;
- observed key-frame count and maximum key-frame interval;
- observed B-frame reorder depth and maximum composition offset;
- audio codec, sample rate, channel count, track count, and time base;
- observed MP4 edit-list presence.

Relative paths are diagnostic. The authoritative source identity is:

```text
case_id + file_sha256
```

Absolute paths, root escapes, asset reordering, non-deterministic repeat hashes, and probe/Spec mismatches are rejected.

## Corpus tiers

| Tier | Meaning |
|---|---|
| `deterministic_container` | media bytes produced by the pinned generator and consumed through real file demuxers |
| `curated_external` | immutable externally sourced media with provenance, licensing review, and SHA-256 |

The initial registry contains deterministic container-backed cases only.

## Initial registry

Every source is 40 seconds long with a 30-second certification boundary.

| Case | Container | Video | Audio | Timeline purpose |
|---|---|---|---|---|
| `real-mp4-h264-aac-cfr-24000-1001-v1` | MP4 | H.264 CFR 24000/1001, B2 | AAC 48 kHz | cinematic rational cadence, no edit list |
| `real-mp4-h264-aac-cfr-30000-1001-edit-list-v1` | MP4 | H.264 CFR 30000/1001, B3 | AAC 48 kHz | +5 s origin and explicit edit list |
| `real-mkv-h264-aac-vfr-24-30-v1` | Matroska | H.264 VFR 24 → 30, B3 | AAC 48 kHz | real file-backed VFR demuxing |
| `real-mpegts-h264-aac-cfr-30-b3-v1` | MPEG-TS | H.264 CFR 30, B3 | AAC 48 kHz | +1.4 s transport origin |
| `real-mkv-h264-opus-cfr-25-v1` | Matroska | H.264 CFR 25, B2 | Opus 48 kHz | non-AAC input audio |
| `real-mp4-h264-aac-cfr-30-aac-44100-v1` | MP4 | H.264 CFR 30, B3 | AAC 44.1 kHz | input audio resampling boundary |

All v1 video sources are progressive SDR BT.709 H.264 High Profile `yuv420p`. Interlaced, HDR, HEVC, AV1, open-GOP, discontinuous-source, multi-audio, subtitle, and curated external cases remain later phases.

## Deterministic generator

Execution package:

```text
internal/transcode/corpusgenerator
```

The domain package remains execution-free:

```text
internal/transcode/realmediacorpus
```

For every case the generator:

1. builds a normalized FFmpeg command from the immutable Spec;
2. writes two independent files into an isolated staging directory;
3. requires both file SHA-256 values to be identical;
4. probes the first file with FFprobe;
5. analyzes presentation-order PTS, cadence, key frames, B-frame reorder depth, and composition offsets;
6. parses MP4 boxes directly to prove or reject `edts` edit-list presence;
7. validates the complete observed evidence against the Spec;
8. constructs and validates the canonical Manifest;
9. only then moves assets into `assets/` and writes the Manifest;
10. removes the staging directory.

The generator never mutates the Spec to match observed output. A mismatch is a failed build.

### Determinism controls

The generated H.264 policy uses:

```text
threads = 1
lookahead-threads = 1
sliced-threads = 0
b-adapt = 0
b-pyramid = none
open-gop = 0
scenecut = 0
```

Additional controls include bitexact mux/codec flags, fixed metadata, MP4 `use_editlist`, Matroska CRC policy, explicit MPEG-TS mux delay, deterministic color VUI, fixed GOP/ref/B-frame policy, and normalized output-path-independent command hashing.

## CLI

List and emit the immutable Spec:

```bash
go run ./cmd/transcode-real-media-corpus-spec -list

go run ./cmd/transcode-real-media-corpus-spec \
  -output /tmp/real-media-corpus-spec-v1.json
```

Generate assets and Manifest:

```bash
go run ./cmd/transcode-real-media-corpus-generate \
  -output-dir /tmp/real-media-corpus-v1
```

Output layout:

```text
/tmp/real-media-corpus-v1/
├── real-media-corpus-manifest-v1.json
└── assets/
    ├── real-mp4-h264-aac-cfr-24000-1001-v1.mp4
    ├── real-mp4-h264-aac-cfr-30000-1001-edit-list-v1.mp4
    ├── real-mkv-h264-aac-vfr-24-30-v1.mkv
    ├── real-mpegts-h264-aac-cfr-30-b3-v1.ts
    ├── real-mkv-h264-opus-cfr-25-v1.mkv
    └── real-mp4-h264-aac-cfr-30-aac-44100-v1.mp4
```

## CI

Spec workflow:

```text
.github/workflows/transcode-real-media-corpus-spec.yml
.github/scripts/verify_real_media_corpus_spec.py
```

Asset generation workflow:

```text
.github/workflows/transcode-real-media-corpus-generate.yml
.github/scripts/verify_real_media_corpus_manifest.py
```

The asset workflow:

1. runs domain and generator Go tests;
2. builds the standalone generator;
3. installs the Ubuntu 24.04 FFmpeg toolchain;
4. generates every asset twice;
5. requires exact byte equality between repeats;
6. validates the Manifest and recomputes every file SHA-256 and size;
7. verifies exact case order, containers, CFR/VFR rates, frame counts, GOPs, B-frame depth, audio policy, origins, color metadata, and edit-list policy;
8. verifies no staging directory survives;
9. uploads assets and Manifest for seven days.

## Next gate: corpus candidate certification

The next phase must consume the generated Manifest as an immutable input and run every file through both explicit encoder time-base candidates. It must bind:

- source file SHA-256;
- Timestamp Plan;
- Output Cadence Evidence;
- Encoder Time-Base Candidate Evidence;
- Encoder Time-Base Reorder Evidence;
- Produced-media Attestation;
- Boundary Packet Evidence;
- A/V Boundary Sync Evidence.

The certification runner must fail if the source file bytes, Manifest identity, observed demuxer behavior, candidate evidence, or fail-closed policy changes.

Independent later gates are still required for curated real-world media, hardware encoders, long-duration drift, HEVC/AV1/HDR/interlaced/subtitles/multi-audio, client acceptance, rollout, fallback, and rollback.

No Real Media Corpus evidence can remove `#EXT-X-DISCONTINUITY` or enable a production encoder time-base candidate by itself.
