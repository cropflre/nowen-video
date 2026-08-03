# Transcode Multi-Job Contention Certification

## Status

The first multi-job contention gate is implemented on `refactor/server-lite-v1`.

This phase moves the Resource Governor verification from simulated runners to real FFmpeg processes. It certifies process admission and cancellation behavior before broader NAS resource-pressure scenarios are introduced.

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

## CI gate

Workflow:

```text
.github/workflows/transcode-multi-job-contention-cert.yml
```

The workflow runs when the executor, Governor, Runtime, or contention workflow changes. It performs:

1. Governor and Runtime race tests;
2. installation and identity output for the runner FFmpeg binary;
3. real FFmpeg serialization and cancelled-waiter fixtures.

The workflow is also available through `workflow_dispatch`.

## Current boundary

This gate certifies admission behavior for multiple real software processes on an Ubuntu hosted runner. It does not yet certify:

- aggregate CPU and memory ceilings across multiple admitted jobs;
- hardware and software jobs contending at the same time;
- NAS filesystem latency spikes or blocked writes;
- network-mounted output directories;
- OOM kill and memory-pressure recovery;
- host reboot during a concurrent durable commit;
- full service-level Job, Attempt, and Artifact publication under concurrent load;
- replacement of a published Artifact while clients are reading it.

Those remain subsequent evidence gates. Until they pass, concurrency capacity must remain conservative and Artifact publication must continue to fail closed.
