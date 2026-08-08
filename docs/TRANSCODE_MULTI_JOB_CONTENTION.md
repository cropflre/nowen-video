# Transcode Multi-Job Contention Certification

## Status

The third multi-job contention gate is implemented on `refactor/server-lite-v1`.

The first gate moved Resource Governor verification from simulated runners to real FFmpeg processes. The second gate extended certification into concurrent durable Job, Attempt, Lease, filesystem, and Artifact publication. The third gate closes the client-read boundary when the same Media/Profile receives a replacement Artifact while an older playlist is still in use.

The implementation does not weaken the existing durable Job, Lease, Attempt, Artifact, attestation, publication, or cleanup boundaries.

## Production observability

The transcode statistics API exposes the current process-level resource state for every Governor pool:

```text
resource_capacity
resource_in_use
resource_waiting
resource_peak_in_use
```

The pools remain independent:

```text
software_transcode
hardware_transcode
remux
ondemand
```

`resource_waiting` shows admission pressure without counting a process as running. `resource_peak_in_use` records the maximum number of admitted processes during the current server process lifetime and must never exceed the configured capacity.

Lease release updates accounting before exposing the semaphore slot to another waiter. This ordering prevents a handoff window from falsely reporting two active leases for a capacity-one pool.

## Real FFmpeg fixtures

File:

```text
internal/transcode/runtime/ffmpeg_contention_test.go
```

The fixtures use the production `executor.ProcessRunner`, the production `runtime.Runtime`, and the production `governor.Governor`. FFmpeg reads a real-time `lavfi` source and writes isolated MP4 outputs.

The tests are opt-in locally:

```bash
NOWEN_REQUIRE_FFMPEG_CONTENTION_FIXTURE=1 \
  go test ./internal/transcode/runtime -run '^TestFFmpeg' -count=1 -v
```

### Serialized software processes

Two real software-transcode processes contend for a capacity-one pool.

The fixture requires:

- process 1 starts and owns the only software slot;
- process 2 appears in `resource_waiting`;
- process 2 does not execute while process 1 is active;
- process 2 starts only after process 1 has completed and released admission;
- both isolated output files are non-empty;
- `resource_peak_in_use.software_transcode` remains exactly `1`;
- the pool drains to zero active and zero waiting processes.

### Cancelled waiter

A second real FFmpeg attempt is submitted while the capacity-one software pool is occupied, then cancelled before admission.

The fixture requires:

- the waiter appears in `resource_waiting`;
- cancellation returns `context.Canceled`;
- the process-start callback is never invoked;
- no output file is created for the cancelled waiter;
- the active process completes normally;
- waiting and active counters return to zero;
- the process-lifetime peak remains within capacity.

## Service-level Artifact contention

File:

```text
internal/service/transcode_concurrent_artifact_test.go
```

The fixture uses a file-backed SQLite database in WAL mode, multiple database connections, the production `TranscodeExecutionRepo`, the production `artifactstore.Store`, and the production `publishCurrentHLSArtifact` service path.

### Independent concurrent publication

Two distinct Media records own separate durable Jobs, Attempts, Leases, staging Artifact rows, and HLS workspaces. Their publication calls start concurrently.

The fixture requires:

- both current Lease owners pass the prepare fence;
- both workspaces are atomically renamed into immutable Artifact directories;
- each Job and its Artifact become visible in one lease-fenced database commit;
- the two published paths are distinct;
- both active keys and Lease tokens are cleared only after completion;
- each resolver returns only its own published Artifact;
- the final Artifact status count contains exactly two published rows.

### Current owner versus stale Lease

Two independent Jobs are prepared, then one Job is explicitly requeued before both publication calls start concurrently.

The fixture requires:

- the current owner publishes successfully;
- the stale worker cannot pass `PrepareArtifactPublish`;
- the stale Artifact is marked `abandoned` with `lease_lost`;
- the stale workspace is not renamed or exposed and remains cleanup eligible;
- the stale Job remains queued and recoverable with no Lease token;
- the successful Job and Artifact are not affected by the stale worker.

## Replacement and client-read consistency

Files:

```text
internal/repository/repo_transcode_artifact_version.go
internal/service/transcode_artifact_version.go
internal/service/stream_artifacts.go
internal/handler/stream_artifacts.go
internal/service/stream_artifact_version_test.go
```

### Previous gap

Runtime HLS playlists previously returned bare segment names such as:

```text
seg0000.ts
```

Every segment request then resolved whichever Artifact was current at request time. If a replacement Artifact became published after the client downloaded the old playlist, a request for the old `seg0000.ts` could resolve the replacement directory. Depending on segment naming, this could return the wrong encode or fail because the replacement had a different segment set.

### Version-pinned playlist contract

Managed HLS playlists now bind every local media URI to the exact Artifact identity that supplied that playlist:

```text
seg0000.ts?artifact=<artifact-id>
```

The segment service resolves that explicit identity and permits only:

- a `staging` or `publishing` Artifact whose current Attempt still owns a live Job Lease;
- the immutable current `published` Artifact;
- an immutable `superseded` Artifact retained for clients holding an older playlist.

The resolver verifies Artifact ID, Media ID, profile, source fingerprint, planner version, kind, lifecycle state, and—when applicable—current Job Lease ownership.

### Fail-closed behavior

A versioned request never falls through to:

- the current replacement Artifact;
- a different Artifact with the same segment basename;
- on-demand segment generation;
- an abandoned workspace.

If the exact requested version has expired or has already been cleaned, the HTTP Adapter returns `410 Gone` with `artifact_version_unavailable`. Legacy playlists without the version query retain the previous current-version/on-demand compatibility path.

Published and retained superseded segments use immutable cache headers and expose `X-Nowen-Artifact-ID` for diagnostics.

### Retention boundary

Publishing a replacement changes the previous row to `superseded` but does not remove its immutable directory. Terminal cleanup selects superseded versions only after the configured failed/terminal retention cutoff, which defaults to seven days. This allows existing clients to complete the exact version referenced by their playlist without preventing bounded storage cleanup.

### Certification fixture

Two sequential Jobs publish the same Media/Profile and intentionally use the same segment basename with different bytes.

The fixture requires:

- playlist one contains Artifact one’s identity;
- playlist two contains only Artifact two’s identity;
- Artifact one becomes `superseded` after publication two;
- the old versioned URL still returns Artifact one’s bytes;
- the new versioned URL returns Artifact two’s bytes;
- a filename present only in Artifact two cannot be read through Artifact one’s identity;
- the freshly superseded row is not returned by cleanup queries using an older retention cutoff;
- nested and external playlist media URIs fail closed rather than bypassing the authenticated single-segment route.

## CI gate

Workflow:

```text
.github/workflows/transcode-multi-job-contention-cert.yml
```

The workflow runs when the execution schema, Lease/Artifact repositories, stream Handler, service publication/read path, Artifact store, executor, Governor, Runtime, or contention workflow changes. It performs:

1. Governor and Runtime race tests;
2. concurrent service-level Job and Artifact publication under the race detector;
3. same-Media replacement and version-pinned client-read tests under the race detector;
4. Repository Claim, Lease, and Artifact fence tests under the race detector;
5. installation and identity output for the runner FFmpeg binary;
6. real FFmpeg serialization and cancelled-waiter fixtures.

The workflow is also available through `workflow_dispatch`.

## Current boundary

This gate certifies process admission, independent publication, same-Media replacement, and retained-version client reads on an Ubuntu hosted runner with file-backed SQLite. It does not yet certify:

- aggregate CPU and memory ceilings across multiple admitted jobs;
- hardware and software jobs contending at the same time;
- NAS filesystem latency spikes or blocked writes;
- network-mounted output directories;
- OOM kill and memory-pressure recovery;
- host reboot during a concurrent durable commit;
- cleanup retry when an Artifact directory is busy or temporarily unavailable;
- client reads that remain open while cleanup removes a version after its retention window.

Those remain subsequent evidence gates. Until they pass, concurrency capacity must remain conservative and Artifact publication/read selection must continue to fail closed.
