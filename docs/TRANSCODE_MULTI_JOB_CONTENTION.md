# Transcode Multi-Job Contention Certification

## Status

The second multi-job contention gate is implemented on `refactor/server-lite-v1`.

The first gate moved Resource Governor verification from simulated runners to real FFmpeg processes. The second gate extends the same certification boundary into the service persistence layer and exercises concurrent Job, Attempt, Lease, filesystem, and Artifact publication behavior.

The implementation does not weaken the existing durable Job, Lease, Attempt, Artifact, attestation, or publication boundaries.

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

## CI gate

Workflow:

```text
.github/workflows/transcode-multi-job-contention-cert.yml
```

The workflow runs when the execution schema, Lease/Artifact repositories, service publication path, Artifact store, executor, Governor, Runtime, or contention workflow changes. It performs:

1. Governor and Runtime race tests;
2. concurrent service-level Job and Artifact publication under the race detector;
3. Repository Claim, Lease, and Artifact fence tests under the race detector;
4. installation and identity output for the runner FFmpeg binary;
5. real FFmpeg serialization and cancelled-waiter fixtures.

The workflow is also available through `workflow_dispatch`.

## Current boundary

This gate certifies process admission and independent service-level Artifact publication on an Ubuntu hosted runner with file-backed SQLite. It does not yet certify:

- aggregate CPU and memory ceilings across multiple admitted jobs;
- hardware and software jobs contending at the same time;
- NAS filesystem latency spikes or blocked writes;
- network-mounted output directories;
- OOM kill and memory-pressure recovery;
- host reboot during a concurrent durable commit;
- concurrent replacement of the same Media/Profile Artifact version;
- replacement of a published Artifact while clients are reading it.

Those remain subsequent evidence gates. Until they pass, concurrency capacity must remain conservative and Artifact publication must continue to fail closed.
