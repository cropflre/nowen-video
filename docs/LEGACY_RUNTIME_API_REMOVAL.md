# Legacy Runtime API Removal

The media-keyed persistent Runtime executor and Artifact playback source were
removed in earlier phases. This phase removes their remaining HTTP and Web UI
compatibility surface. The retired URLs are no longer protocol endpoints and
therefore resolve through the normal router 404 path instead of returning a
custom 410 tombstone.

## Removed surfaces

- Media-keyed HLS master, quality playlist, segment and audio-track routes.
- Media-keyed playback position, bandwidth and throttle telemetry routes.
- Legacy Runtime transcode status, task list and mutation routes.
- The obsolete Web transcode task panel and its batch submit/actions.
- Client-side guessed `/stream/:id/master.m3u8` fallback URLs.

## Preserved contracts

- Direct play and managed Remux.
- STRM proxy and health endpoints.
- Ephemeral Playback Sessions and Generation playlist/segment reads.
- Durable administrator preprocessing and its playback endpoints.
- Read-only Runtime History APIs over retained Job, Attempt, Artifact and legacy
  task metadata.

## Client rule

An incompatible source must obtain a server playback plan and create an
ephemeral Playback Session. A client must never infer a media-keyed Runtime URL.
Playback Session heartbeat is the only Runtime liveness/position protocol;
normal watch history remains a separate user-progress write.

## Data and rollback

No historical table or row is deleted. Existing `transcode_jobs`,
`transcode_attempts`, `transcode_artifacts` and `transcode_tasks` data remains
available through Runtime History and remains compatible with database rollback.

## Acceptance

- The dedicated migration gate passed the complete Go package suite, Lite and
  Full server builds, Web TypeScript compilation, Vite production build and
  retired-URL production-asset scan.
- Source contracts reject reintroduction of the removed routes, tombstone
  handlers, middleware special cases, guessed client URLs and obsolete Web task
  panel.
- Build-only changes to tracked `server` and `server-lite` binaries were restored
  before final acceptance; this phase contains source, tests and documentation
  changes only.
- The normal project CI matrix is rerun from this documentation commit so Docker
  startup and database upgrade/rollback checks execute on the final tree.
