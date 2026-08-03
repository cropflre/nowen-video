# Transcode Recovery and Resource Stress Certification v4

## Status

Recovery and Resource Stress Certification v4 is implemented and passing on `refactor/server-lite-v1`.

The phase validates production-shaped FFmpeg execution against cancellation, process death, disk-write failure, constrained NAS resources, and stale Lease finalization. It uses the durable `transcode_jobs` / `transcode_attempts` state model, real Artifact rows, real Lease tokens, isolated workspaces, production HLS command construction, the production process executor, and independent evidence verification.

This phase does **not** enable seamless playback, remove playlist discontinuities, or relax Artifact publication rules.

Production remains fail-closed:

```text
seamless_allowed = false
discontinuity_required = true
#EXT-X-DISCONTINUITY = mandatory
```

## Scenarios

The immutable v4 registry contains five scenarios:

```text
cancel-active-segment-write-v1
sigkill-lease-requeue-restart-v1
enospc-segment-write-v1
bounded-one-core-512m-v1
stale-lease-finalize-fence-v1
```

### Active segment cancellation

A 20-minute logical HLS attempt is cancelled after progress crosses five minutes and completed segment files exist.

The scenario proves:

- the durable desired state changes to `cancelled`;
- the owning process context is cancelled;
- the current Lease finalizes the Job and Artifact as `cancelled`;
- the partial workspace is quarantined;
- no readable Artifact is exposed;
- the workspace remains cleanup eligible.

### SIGKILL and Lease recovery

The first worker is killed after progress crosses five minutes. Its Lease is expired and the same durable Job is returned to `queued`. A replacement worker claims a new Lease and starts Attempt 2.

The scenario proves:

- the killed process cannot publish;
- the old Lease cannot prepare or commit final publication;
- Attempt 1 becomes abandoned/quarantined;
- the replacement Lease token differs from the original token;
- Attempt 2 publishes atomically;
- only the replacement Artifact becomes readable.

### ENOSPC segment write

The first real HLS segment path is bind-mounted to Linux `/dev/full`. A startup probe writes to that exact path and requires the kernel to return `ENOSPC` before FFmpeg is started.

The scenario uncovered an important production behavior: FFmpeg 6.1.1 can emit explicit HLS write errors such as `No space left on device`, continue producing a partial playlist, and still return process exit code `0`.

Relying only on the process exit code could therefore publish a damaged or incomplete HLS Artifact.

The production `ProcessRunner` now scans the complete stderr stream and latches classified fatal output errors. ENOSPC is represented as:

```text
fatal_output_detected = true
fatal_output_code     = write_enospc
```

The real process exit code remains unchanged in evidence. A successful exit code combined with fatal classified output is returned as a typed `FatalOutputError`, causing the production execution path to fail closed.

The classifier observes the complete stderr stream rather than only the bounded diagnostic tail, so later FFmpeg logs cannot evict an earlier fatal write error from detection.

The scenario proves:

- the kernel ENOSPC backend is bound to the actual first HLS segment;
- stderr contains an independently verified ENOSPC marker;
- the executor detects `write_enospc` even when FFmpeg exits with `0`;
- the Job and Artifact finish as `failed`;
- the partial playlist and segments are never made readable;
- the workspace remains quarantined and cleanup eligible.

### One CPU and 512 MiB

A 30-minute logical HLS run executes under a dedicated cgroup v2 controller:

```text
cpu.max         = 100000 100000
CPU affinity    = one allowed CPU
memory.max      = 536870912
memory.swap.max = 0
```

A small privileged helper creates the cgroup, moves only the FFmpeg child into it, applies CPU affinity, drops back to the original uid/gid, executes FFmpeg, waits for completion, records kernel `memory.peak`, and removes the cgroup.

The scenario proves:

- the process completes under a real one-core CPU quota;
- the process completes under a real 512 MiB memory ceiling;
- kernel `memory.peak` remains inside the configured limit;
- no fatal output classification occurs;
- the Artifact publishes atomically and becomes readable.

### Stale Lease finalization

Attempt 1 completes its FFmpeg process, but its Lease is expired before publication. The Job is requeued and claimed under Lease generation 2. Attempt 2 completes normally.

The scenario proves:

- the old Lease is rejected by both Prepare and Commit publication fences;
- the old Artifact is abandoned and quarantined;
- the replacement Lease token differs from the original;
- only Attempt 2 publishes;
- the durable Job completes exactly once under the current Lease.

## Evidence schemas

Scenario contract:

```text
transcode-recovery-resource-scenario-evidence-v4
```

Scenario report:

```text
ffmpeg-recovery-resource-scenario-v4
```

Aggregate contract:

```text
transcode-recovery-resource-aggregate-evidence-v4
```

Aggregate report:

```text
ffmpeg-recovery-resource-aggregate-v4
```

Each scenario contract binds:

1. Real Media Corpus Spec and Manifest identities;
2. source generator, source FFmpeg, and source FFprobe identities;
3. certification FFmpeg and FFprobe identities;
4. canonical Timestamp Plan identity;
5. exact immutable scenario specification;
6. source path, SHA-256, byte size, and canonical asset evidence hash;
7. every durable state transition;
8. every process command hash, exit code, signal, cancellation state, progress and segment evidence;
9. stderr hash and classified markers;
10. fatal-output classification evidence;
11. cgroup resource controller and kernel memory-peak evidence;
12. Lease token hashes and stale-worker publication fences;
13. final Job and Artifact state;
14. readable Artifact identity or explicit absence;
15. partial-output quarantine and cleanup eligibility;
16. fail-closed production fields.

The root aggregate requires exactly five canonical scenario reports, rejects duplicates or omissions, verifies common Corpus and Timestamp Plan identities, embeds every scenario contract and hash, recomputes total process/segment/resource metrics, and fails unless all five scenarios pass.

## Production executor hardening

Files:

```text
internal/transcode/executor/fatal_output.go
internal/transcode/executor/runner.go
internal/transcode/executor/runner_test.go
```

`executor.Result` now preserves both process status and classified output status:

```text
ExitCode
Err
FatalOutputCode
FatalOutputLine
StderrTail
```

The first fatal stderr line is latched while the stream is consumed. The bounded stderr tail remains diagnostic only.

Current classified fatal output:

```text
write_enospc
```

The implementation deliberately does not fabricate a non-zero FFmpeg exit code. It preserves the real status and adds a typed error so callers can distinguish:

- process failure;
- cancellation or timeout;
- process exit `0` with a known fatal media-output error.

Unit tests execute a helper that emits ENOSPC, writes more than 50 subsequent diagnostic lines so the ENOSPC line is evicted from the configured tail, then exits with status `0`. The runner must still return `FatalOutputError{Code: write_enospc}`.

## Fault backends

### Kernel ENOSPC backend

```text
backend = dev-full-bind
```

The harness:

1. mounts an isolated tmpfs workspace;
2. creates the real first HLS segment target;
3. bind-mounts `/dev/full` over that target;
4. probes the target and requires a kernel `ENOSPC` result;
5. starts the normal production-shaped FFmpeg command;
6. unmounts the segment bind and workspace after evidence collection.

This avoids LD_PRELOAD interception and avoids estimating when a low-bitrate fixture will exhaust a filesystem.

### cgroup v2 resource backend

```text
resource_controller = cgroup-v2
```

The bounded process evidence records:

```text
cpu_count_limit
memory_limit_bytes
max_rss_bytes
```

For the bounded scenario, `max_rss_bytes` is the kernel cgroup `memory.peak`, not a user-space estimate.

## Independent verification

Verifier:

```text
.github/scripts/verify_recovery_stress.py
```

For every scenario it independently verifies:

- exact scenario registry membership and JSON identity;
- Corpus Spec and Manifest hashes;
- source asset identity;
- Timestamp Plan identity;
- process count and Attempt ordering;
- command and stderr SHA-256 values;
- state-transition sequence;
- final Job and Artifact states;
- Lease replacement and stale-worker fences;
- readable Artifact rules;
- quarantine and cleanup rules;
- fatal-output field consistency;
- ENOSPC backend `dev-full-bind` and `write_enospc` classification;
- cgroup v2 identity, one-CPU limit, 512 MiB limit, and memory peak;
- canonical scenario contract hash.

For the aggregate it additionally verifies:

- exact five-scenario order and completeness;
- every embedded scenario contract hash;
- common Corpus and Timestamp Plan identities;
- total process count;
- total observed segment count;
- maximum memory peak;
- canonical aggregate contract hash.

## CI architecture

Workflow:

```text
.github/workflows/transcode-recovery-stress-cert.yml
```

Jobs:

1. inspect the latest branch commit and skip expensive execution for unrelated changes;
2. run fail-fast contract, executor, helper, CLI and Python checks;
3. generate and independently verify one immutable Real Media Corpus;
4. execute five independent scenario jobs with `fail-fast: false`;
5. independently verify and upload every scenario report;
6. download exactly five reports, build the root contract, independently verify it, and upload the aggregate report.

Manual `workflow_dispatch` continues to force the complete certification.

## Reference environment

```text
workflow run        = 30780413504
branch head         = cd8728a74657e6513b3eed739830932434d32345
PR merge commit     = e1fa7f21507f25ce88f35b668d41898cecca65ec
operating system    = Ubuntu 24.04
Go                  = 1.25.0
FFmpeg / FFprobe    = 6.1.1-3ubuntu5
```

Timestamp Plan:

```text
version = hls-timestamp-normalization-v1
hash    = 0648217f7c10a055d84c6005c497f328ff02606119195e98dfe76fcae33dd937
```

Source:

```text
case                 = real-mp4-h264-aac-cfr-30-aac-44100-v1
SHA-256              = f08b0196d636e555bddc50d1b5d88b9cc5c8a184f8bcd4216e50517f4d6e44a0
size                 = 4,627,950 bytes
asset evidence hash  = 04f4e141085bc0aabc9c1d0bc40e9a89de856145608c315ab67c35372d7c9548
```

## Measured scenario evidence

### Active segment cancellation

```text
contract                    = 051fc1e58606931348ccfb52074d81e31f79fb18cb6f27c76f005cf971a5aa54
real process exit code      = -1
cancelled                   = true
trigger progress            = 307,247,000 us
observed segments           = 153
manifest completed          = false
memory peak                 = 88,391,680 bytes
elapsed                     = 17,099 ms
final Job                   = cancelled
final Artifact              = cancelled
readable Artifact           = none
partial workspace           = quarantined
cleanup eligible            = true
```

### SIGKILL Lease recovery

Attempt 1:

```text
signal                      = SIGKILL
real process exit code      = -1
trigger progress            = 306,202,000 us
observed segments           = 152
memory peak                 = 88,109,056 bytes
elapsed                     = 18,097 ms
```

Attempt 2:

```text
real process exit code      = 0
logical progress            = 1,199,882,000 us
observed segments           = 600
memory peak                 = 96,821,248 bytes
elapsed                     = 69,607 ms
published                   = true
```

Outcome:

```text
contract                    = 30d38bd6000cdd0b2f0adc9fddc490c40495c070b547bece41648883b511c971
Lease expired and requeued  = true
old Prepare rejected        = true
old Commit rejected         = true
replacement Lease differs   = true
replacement publish         = committed
final Job                   = completed
final Artifact              = published
old workspace               = quarantined
```

### ENOSPC segment write

```text
contract                    = 14d22805abba7f177dfa16558ba7d8c86f0348dad517e463ec6c01ee75c6c14d
fault backend               = dev-full-bind
real process exit code      = 0
fatal output detected       = true
fatal output code           = write_enospc
stderr marker               = ENOSPC
logical progress            = 599,889,000 us
non-empty regular segments  = 12
manifest exists             = true
memory peak                 = 90,087,424 bytes
elapsed                     = 34,529 ms
final Job                   = failed
final Artifact              = failed
readable Artifact           = none
partial workspace           = quarantined
cleanup eligible            = true
```

The `12` observed files demonstrate why process exit code alone is insufficient: FFmpeg continued after the first segment write failed and produced a partial output tree, but the executor classification prevented publication.

### One CPU and 512 MiB

```text
contract                    = d86cb7d48976c6204c5f4aa0e3a9a56654a0893d6cdcdae940a81b2550f85e37
resource controller         = cgroup-v2
CPU count limit             = 1
memory limit                = 536,870,912 bytes
kernel memory peak          = 127,733,760 bytes
logical progress            = 1,793,845,000 us
observed segments           = 900
real process exit code      = 0
fatal output detected       = false
elapsed                     = 123,451 ms
final Job                   = completed
final Artifact              = published
cleanup eligible            = false
```

The measured peak is approximately 121.8 MiB, leaving substantial headroom below the 512 MiB ceiling for this deterministic software-transcode profile.

### Stale Lease finalize fence

Attempt 1:

```text
real process exit code      = 0
logical progress            = 599,889,000 us
observed segments           = 300
memory peak                 = 90,226,688 bytes
elapsed                     = 34,086 ms
published                   = false
```

Attempt 2:

```text
real process exit code      = 0
logical progress            = 599,889,000 us
observed segments           = 300
memory peak                 = 90,345,472 bytes
elapsed                     = 33,818 ms
published                   = true
```

Outcome:

```text
contract                    = c5d18a9db5b8131d3ed6553d4d0184df1f0c5e1314022cf1518397684a14b3fa
Lease expired and requeued  = true
old Prepare rejected        = true
old Commit rejected         = true
replacement Lease differs   = true
replacement publish         = committed
final Job                   = completed
final Artifact              = published
old workspace               = quarantined
```

## Aggregate evidence

```text
aggregate contract          = 023838f6ea26d5eec4f894f951606e9a22a27bd1341e20dd1937e1e415470ba4
scenarios                   = 5
processes                   = 7
observed segments           = 2,417
maximum memory peak         = 127,733,760 bytes
all scenarios passed        = true
seamless allowed            = false
discontinuity required      = true
```

Aggregate Artifact:

```text
Artifact ID                 = 8843545891
compressed size             = 6,653 bytes
ZIP SHA-256                 = 0a503bc3477ea440479432cf52a05fdd65fb99070f633e4851ec36b8e3d3cd26
aggregate JSON              = 56,764 bytes
retention                   = 7 days
```

Scenario Artifacts:

```text
cancel active write         = 8843505087 / 1b83547be7e07a55e0617bd43973954f1428574227dfd458c3389eab2caebf19
ENOSPC write                = 8843510841 / 4e4af14cdae79fc9b9d54c3a18d374236ddf622de9f55572b1a37804f62322d3
stale Lease fence           = 8843520653 / a2453c27ec391c88a2fd7f16da07191a2035a90d7c75123b78d83d1676dfebaf
SIGKILL recovery            = 8843521766 / 432a8b5527d27f473f370797a4ae8d1aa86c8d37c881ad8c24fad40b1e79f354
bounded resources           = 8843532309 / 43bb251dfb2e15dafe005902c0e228f9ade5a3c0247d72a4ee5cb7c1ea3fb03c
```

Immutable Corpus Artifact:

```text
Artifact ID                 = 8843488855
compressed size             = 24,531,360 bytes
ZIP SHA-256                 = 4e421b4cc9d66df186a2ad99c1a2b3dfa7acdf165d3fb7960e821a2c62bf83d8
retention                   = 7 days
```

## Verification status

The reference branch head passed:

```text
Recovery v4 latest-commit preflight                  PASS
Executor fatal-output stream-latching tests          PASS
Recovery contract tests                              PASS
/dev/full kernel ENOSPC probe                        PASS
cgroup v2 helper compilation                         PASS
Recovery CLI build                                   PASS
Python verifier syntax                               PASS
Immutable Corpus generation and verification         PASS
Five recovery/resource scenarios                     PASS
Five independent scenario verifications              PASS
Aggregate contract                                   PASS
Independent aggregate verification                   PASS
Go full package suite                                PASS
Real FFmpeg fixture tests                            PASS
Lease / filesystem race tests                       PASS
Resolver performance baselines                       PASS
Web TypeScript / Vite build                          PASS
Lite server build                                    PASS
Full server build                                    PASS
Lite Docker persistent-volume smoke                  PASS
Full Docker persistent-volume smoke                  PASS
```

## Non-goals and remaining gates

v4 certifies a deterministic software-transcode profile on Ubuntu hosted runners. It does not yet certify:

- hardware encoder death or device reset;
- NAS filesystem latency spikes and hung I/O;
- network-mounted output filesystems;
- Linux OOM kill and memory-pressure recovery;
- host reboot during durable state commit;
- concurrent multi-job resource contention;
- cleanup retry after a locked or busy mount;
- client playback behavior while a published Artifact is replaced.

These remain separate evidence gates. No v4 result authorizes removal of discontinuities or weakening of Lease and Artifact publication fences.
