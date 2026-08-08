# Web Browser Adaptive Playback

## Goal

The browser player should behave like a mature media server client: use the cheapest compatible path first, but do not stop permanently when the browser rejects a source that the initial capability probe considered playable.

The runtime order is strictly one-way:

```text
Direct Play -> Remux / Smart Remux -> HLS Transcode
```

Preprocessed HLS remains the preferred ready-made source. Desktop libmpv and the experimental WebCodecs engine keep their existing selection rules; when WebCodecs fails, the browser resumes at the captured position through this adaptive chain.

## Authority and state ownership

The server playback plan is authoritative. The Web client does not decide that a codec or container must be remuxed or transcoded after a runtime failure.

`AdaptiveWebVideoPlayer` only reports the failed capability back to `/api/stream/:id/plan`:

| Runtime failure | Replanning capabilities |
| --- | --- |
| Direct Play failed | `supports_direct=false`, `supports_remux=true` |
| Remux failed | `supports_direct=false`, `supports_remux=false`, `force_transcode=true` |

The returned `PlaybackPlan` becomes the single active runtime plan. A session-required transcode is handed to `SessionVideoPlayer`; a URL-based plan is handed to `VideoPlayer`.

## Runtime guarantees

### Monotonic fallback

Playback modes have an explicit rank:

```text
direct=0, remux=1, hls=2
```

A server response is rejected when it moves sideways or backwards. Failed modes are recorded for the current source generation, and a previously failed mode cannot be selected again. This prevents direct/remux loops and avoids timer-driven retries.

### State preservation

Before requesting a fallback, the client captures:

- absolute playback position;
- volume and mute state;
- playback rate;
- whether a non-zero playback position was explicitly paused.

The next player generation receives the captured start position. Volume, mute state, playback rate and play/pause intent are restored after metadata becomes available.

The plan cache may change during fallback. That cache update is not treated as a new media generation, so it cannot clear the captured snapshot. Reset only occurs when the media or real source generation changes, such as switching to a completed preprocessed asset.

Subtitle tracks continue to follow the existing player reload and auto-selection policy. Exact user-selected subtitle identity is not yet part of the fallback snapshot.

### Error boundaries

Direct and Remux native media errors trigger replanning. HLS is the final compatibility path and retains the existing hls.js/native-HLS recovery behavior. If the server cannot produce a strictly safer plan, the UI stops with the original browser error plus the planning failure instead of looping indefinitely.

Authentication and authorization failures are not converted into codec fallbacks by this controller. The server still owns access control and session creation errors.

## Resource behavior

This implementation does not force all media through FFmpeg:

1. Direct Play remains first choice when the server and browser capability probe allow it.
2. Remux changes the container without video re-encoding when the server selects it.
3. HLS transcoding is created only after cheaper compatible paths are unavailable or fail at runtime.

This keeps NAS CPU/GPU use close to Emby-style behavior while improving browser compatibility.

## Acceptance matrix

Before release, verify at least:

1. H.264/AAC MP4 plays directly without creating a transcode session.
2. A source reported as direct-playable but rejected by the browser automatically replans to Remux or HLS.
3. H.264 MKV selects Remux and continues playback.
4. Unsupported HEVC/10-bit Remux failure advances to HLS without returning to Direct Play.
5. Playback position, volume, mute state and speed survive Direct-to-Remux and Remux-to-HLS transitions.
6. WebCodecs failure resumes through the adaptive browser path at the current position.
7. Chrome/Edge use hls.js for HLS; Safari can use native HLS where applicable.
8. A missing file, invalid STRM target or failed transcode shows a terminal error and does not retry in a loop.
9. Switching to a newly completed preprocessed stream resets the source generation while preserving the switch position.
10. Navigating to the next media item clears all failed-mode history.

## Regression contract

`cmd/server/web_playback_fallback_contract_test.go` protects the critical source-level contract:

- server-authoritative replanning parameters;
- strict Direct -> Remux -> HLS ranking;
- failed-mode cycle prevention;
- playback snapshot capture and resume position;
- PlayerPage ownership through `AdaptiveWebVideoPlayer`;
- removal of the old split `remuxFailed` fallback state.
