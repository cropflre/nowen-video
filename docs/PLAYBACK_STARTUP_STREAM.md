# Startup Stream Playback Contract

## Status

This document defines the first production-facing Startup Stream contract on
`refactor/server-lite-v1`.

It is a formal playback method, not a hidden replacement for `transcode` and
not a client-side redirect trick.

Current protocol version:

```text
startup_stream / event_bridge_v1
```

## Decision boundary

The Playback Planner may select `startup_stream` only when all of the following
are true:

1. Direct Play, Remux, and Smart Remux are not the selected primary method.
2. A source-fingerprint-matched Startup Artifact is already `published`.
3. Its profile, duration, and authenticated bridge URL are complete.
4. The request does not carry a per-request `max_bitrate` cap.

When a bitrate cap is present, Runtime HLS remains authoritative because a
previously encoded immutable Startup Artifact cannot be retroactively capped.

Startup Stream is never selected for STRM/WebDAV resolver-specific sources in
this version.

## PlaybackPlan schema

Example:

```json
{
  "media_id": "media-id",
  "method": "startup_stream",
  "url": "/api/stream/media-id/startup-720p/stream.m3u8",
  "reason_code": "startup_artifact_ready",
  "reason": "已命中预生成启动流，并通过服务端统一时间线接续持续转码",
  "requires_transcode": true,
  "fallback_method": "transcode",
  "fallback_url": "/api/stream/media-id/master.m3u8",
  "startup_stream": {
    "profile_id": "720p",
    "duration_ms": 30000,
    "playlist_url": "/api/stream/media-id/startup-720p/stream.m3u8",
    "continuation_mode": "event_bridge_v1",
    "discontinuity_at_handoff": true
  }
}
```

The client must treat `url` as the only primary playback URL. It must not build
Artifact paths, Job IDs, Attempt IDs, or continuation URLs itself.

## EVENT bridge

The bridge is a server-generated, authenticated EVENT playlist:

```text
immutable Startup Artifact segments
  -> EXT-X-DISCONTINUITY
  -> Lease-valid Startup Continuation segments
```

The bridge itself stores no media files and never copies or mutates either
Artifact directory.

While continuation is pending, the playlist deliberately omits
`EXT-X-ENDLIST`, allowing normal HLS playlist reloads. `EXT-X-ENDLIST` is added
only after the continuation Artifact has completed.

Segment URLs use the existing authenticated stream route with a virtual profile:

```text
/api/stream/:media/startup-:profile/stream.m3u8
/api/stream/:media/startup-:profile/startup__segNNNN.ts
/api/stream/:media/startup-:profile/continuation__segNNNN.ts
```

The HTTP Adapter parses this virtual namespace and delegates file resolution to
the Artifact Store. It does not access a shared mutable media/profile directory.

## Fallback

`fallback_method` and `fallback_url` always point to ordinary Runtime HLS.
Clients may switch to the fallback once when the bridge cannot be loaded or the
handoff fails. Authentication, authorization, and DRM errors must not trigger an
automatic compatibility fallback.

Android V2 already has one-shot playback fallback behavior. Web/PC consumes the
Startup Stream URL through the common HLS path; explicit fatal-HLS fallback UX
remains a separate client hardening task.

## Timeline honesty

`discontinuity_at_handoff=true` is intentional. The current implementation
provides one stable URL and one append-only playlist, but it does not claim
sample-perfect timestamp continuity yet.

Before this field can become `false`, the following must be verified on every
encoding backend:

- video PTS/DTS alignment at the startup boundary;
- audio priming and encoder-delay alignment;
- identical track selection and channel layout;
- identical codec profile, level, pixel format, color metadata, and time base;
- keyframe and segment-boundary alignment;
- browser, ExoPlayer, mpv, Emby, and Infuse behavior without decoder reset.

The explicit discontinuity prevents the project from presenting a URL switch as
seamless playback before the media timeline has actually been proven continuous.

## Failure and recovery

- A stale Lease cannot expose continuation segments.
- A superseded Attempt cannot publish into the bridge.
- Source fingerprint or Planner Version changes invalidate Startup reuse.
- A missing Startup Artifact makes the Planner return normal `transcode`.
- A database or Artifact Store failure is returned as an infrastructure error;
  it is not silently converted into On-demand playback.
- Service restart reconstructs readability from Job, Attempt, Lease, and
  Artifact state rather than directory existence.

## Verification

Required automated coverage:

- Startup Planner projection and Runtime HLS fallback.
- Bitrate-cap exclusion.
- Virtual route parsing and path rejection.
- Startup-before-continuation ordering.
- Exactly one declared handoff discontinuity.
- Open EVENT playlist while continuation is pending.
- End List only after continuation completes.
- Web TypeScript production build.
- Android model decoding and preferred/fallback URL behavior.
- Lite and Full server build and Docker deployment smoke tests.

## Next phase

The next formal phase is timestamp-aligned continuation. It must introduce a
versioned Encoding Plan containing track selection, time-base policy, seek
policy, encoder delay, GOP layout, color conversion, and backend capability
identity. Only after that plan is shared by Startup and Continuation may the
handoff discontinuity be removed.
