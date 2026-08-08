package runtime

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/transcode/executor"
	resourcegovernor "github.com/nowen-video/nowen-video/internal/transcode/governor"
)

const ffmpegContentionFixtureEnv = "NOWEN_REQUIRE_FFMPEG_CONTENTION_FIXTURE"

type ffmpegContentionOutcome struct {
	name    string
	started time.Time
	result  executor.Result
}

type ffmpegContentionStart struct {
	name string
	at   time.Time
}

func TestFFmpegSoftwarePoolSerializesRealProcesses(t *testing.T) {
	ffmpegPath := requireFFmpegContentionFixture(t)
	resourceGovernor := resourcegovernor.New(resourcegovernor.Config{
		SoftwareTranscodes: 1,
		HardwareTranscodes: 1,
		RemuxStreams:       1,
		OnDemandSegments:   1,
	})
	runtime := New(executor.NewProcessRunner(), resourceGovernor)
	workspace := t.TempDir()
	starts := make(chan ffmpegContentionStart, 2)

	firstOutput := filepath.Join(workspace, "first.mp4")
	secondOutput := filepath.Join(workspace, "second.mp4")
	first := runFFmpegContentionAttempt(runtime, ffmpegPath, "first", "2", firstOutput, starts)
	firstStart := waitForFFmpegContentionStart(t, starts, "first")

	second := runFFmpegContentionAttempt(runtime, ffmpegPath, "second", "1", secondOutput, starts)
	waitForFFmpegContentionSnapshot(t, runtime, time.Second, func(snapshot resourcegovernor.Snapshot) bool {
		return snapshot.InUse[resourcegovernor.KindSoftwareTranscode] == 1 &&
			snapshot.Waiting[resourcegovernor.KindSoftwareTranscode] == 1 &&
			snapshot.PeakInUse[resourcegovernor.KindSoftwareTranscode] == 1
	})

	select {
	case unexpected := <-starts:
		t.Fatalf("contending process %q started before the active lease was released", unexpected.name)
	case <-time.After(150 * time.Millisecond):
	}

	firstOutcome := waitForFFmpegContentionOutcome(t, first, "first")
	secondStart := waitForFFmpegContentionStart(t, starts, "second")
	secondOutcome := waitForFFmpegContentionOutcome(t, second, "second")

	if firstOutcome.started != firstStart.at {
		t.Fatalf("first start evidence mismatch: callback=%s outcome=%s", firstStart.at, firstOutcome.started)
	}
	if secondOutcome.started != secondStart.at {
		t.Fatalf("second start evidence mismatch: callback=%s outcome=%s", secondStart.at, secondOutcome.started)
	}
	if secondStart.at.Before(firstOutcome.result.CompletedAt) {
		t.Fatalf("second process started before first completion: first_completed=%s second_started=%s", firstOutcome.result.CompletedAt, secondStart.at)
	}
	assertFFmpegContentionOutput(t, firstOutput)
	assertFFmpegContentionOutput(t, secondOutput)

	snapshot := runtime.Snapshot()
	if snapshot.InUse[resourcegovernor.KindSoftwareTranscode] != 0 || snapshot.Waiting[resourcegovernor.KindSoftwareTranscode] != 0 {
		t.Fatalf("software pool did not drain: %+v", snapshot)
	}
	if snapshot.PeakInUse[resourcegovernor.KindSoftwareTranscode] != 1 {
		t.Fatalf("software pool exceeded capacity: %+v", snapshot)
	}
}

func TestFFmpegCancelledWaiterNeverStarts(t *testing.T) {
	ffmpegPath := requireFFmpegContentionFixture(t)
	resourceGovernor := resourcegovernor.New(resourcegovernor.Config{SoftwareTranscodes: 1})
	runtime := New(executor.NewProcessRunner(), resourceGovernor)
	workspace := t.TempDir()
	starts := make(chan ffmpegContentionStart, 2)

	activeOutput := filepath.Join(workspace, "active.mp4")
	cancelledOutput := filepath.Join(workspace, "cancelled.mp4")
	active := runFFmpegContentionAttempt(runtime, ffmpegPath, "active", "2", activeOutput, starts)
	waitForFFmpegContentionStart(t, starts, "active")

	ctx, cancel := context.WithCancel(context.Background())
	cancelled := runFFmpegContentionAttemptWithContext(runtime, ctx, ffmpegPath, "cancelled", "1", cancelledOutput, starts)
	waitForFFmpegContentionSnapshot(t, runtime, time.Second, func(snapshot resourcegovernor.Snapshot) bool {
		return snapshot.InUse[resourcegovernor.KindSoftwareTranscode] == 1 &&
			snapshot.Waiting[resourcegovernor.KindSoftwareTranscode] == 1
	})
	cancel()

	cancelledOutcome := waitForFFmpegContentionOutcomeAllowError(t, cancelled, "cancelled")
	if !errors.Is(cancelledOutcome.result.Err, context.Canceled) || !cancelledOutcome.result.Cancelled {
		t.Fatalf("expected cancelled admission, got %+v", cancelledOutcome.result)
	}
	if !cancelledOutcome.started.IsZero() {
		t.Fatalf("cancelled waiter unexpectedly started at %s", cancelledOutcome.started)
	}
	if _, err := os.Stat(cancelledOutput); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled waiter produced output: stat_err=%v", err)
	}
	select {
	case unexpected := <-starts:
		t.Fatalf("cancelled waiter unexpectedly emitted start event for %q", unexpected.name)
	case <-time.After(100 * time.Millisecond):
	}

	waitForFFmpegContentionOutcome(t, active, "active")
	assertFFmpegContentionOutput(t, activeOutput)
	waitForFFmpegContentionSnapshot(t, runtime, time.Second, func(snapshot resourcegovernor.Snapshot) bool {
		return snapshot.InUse[resourcegovernor.KindSoftwareTranscode] == 0 &&
			snapshot.Waiting[resourcegovernor.KindSoftwareTranscode] == 0 &&
			snapshot.PeakInUse[resourcegovernor.KindSoftwareTranscode] == 1
	})
}

func requireFFmpegContentionFixture(t *testing.T) string {
	t.Helper()
	if os.Getenv(ffmpegContentionFixtureEnv) != "1" {
		t.Skipf("set %s=1 to run the real FFmpeg contention fixture", ffmpegContentionFixtureEnv)
	}
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatalf("ffmpeg is required: %v", err)
	}
	return path
}

func runFFmpegContentionAttempt(runtime *Runtime, ffmpegPath, name, duration, output string, starts chan<- ffmpegContentionStart) <-chan ffmpegContentionOutcome {
	return runFFmpegContentionAttemptWithContext(runtime, context.Background(), ffmpegPath, name, duration, output, starts)
}

func runFFmpegContentionAttemptWithContext(runtime *Runtime, ctx context.Context, ffmpegPath, name, duration, output string, starts chan<- ffmpegContentionStart) <-chan ffmpegContentionOutcome {
	result := make(chan ffmpegContentionOutcome, 1)
	go func() {
		var started time.Time
		processResult := runtime.Run(ctx, resourcegovernor.KindSoftwareTranscode, executor.Command{
			Path: ffmpegPath,
			Args: []string{
				"-hide_banner",
				"-nostdin",
				"-loglevel", "error",
				"-re",
				"-f", "lavfi",
				"-i", "testsrc2=size=320x180:rate=30",
				"-t", duration,
				"-an",
				"-c:v", "mpeg4",
				"-q:v", "5",
				"-progress", "pipe:2",
				"-nostats",
				"-y",
				output,
			},
			StderrTail: 20,
		}, executor.Callbacks{
			OnStarted: func(*os.Process) {
				started = time.Now()
				starts <- ffmpegContentionStart{name: name, at: started}
			},
		})
		result <- ffmpegContentionOutcome{name: name, started: started, result: processResult}
	}()
	return result
}

func waitForFFmpegContentionStart(t *testing.T, starts <-chan ffmpegContentionStart, expected string) ffmpegContentionStart {
	t.Helper()
	select {
	case start := <-starts:
		if start.name != expected {
			t.Fatalf("expected %q to start, got %q", expected, start.name)
		}
		return start
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %q FFmpeg start", expected)
		return ffmpegContentionStart{}
	}
}

func waitForFFmpegContentionOutcome(t *testing.T, outcomes <-chan ffmpegContentionOutcome, expected string) ffmpegContentionOutcome {
	t.Helper()
	outcome := waitForFFmpegContentionOutcomeAllowError(t, outcomes, expected)
	if outcome.result.Err != nil {
		t.Fatalf("FFmpeg attempt %q failed: %v\nstderr=%v", expected, outcome.result.Err, outcome.result.StderrTail)
	}
	if outcome.result.ExitCode != 0 {
		t.Fatalf("FFmpeg attempt %q returned exit code %d", expected, outcome.result.ExitCode)
	}
	return outcome
}

func waitForFFmpegContentionOutcomeAllowError(t *testing.T, outcomes <-chan ffmpegContentionOutcome, expected string) ffmpegContentionOutcome {
	t.Helper()
	select {
	case outcome := <-outcomes:
		if outcome.name != expected {
			t.Fatalf("expected outcome %q, got %q", expected, outcome.name)
		}
		return outcome
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %q FFmpeg outcome", expected)
		return ffmpegContentionOutcome{}
	}
}

func waitForFFmpegContentionSnapshot(t *testing.T, runtime *Runtime, timeout time.Duration, predicate func(resourcegovernor.Snapshot) bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate(runtime.Snapshot()) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("runtime snapshot did not reach expected contention state: %+v", runtime.Snapshot())
}

func assertFFmpegContentionOutput(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected FFmpeg output %s: %v", path, err)
	}
	if info.Size() <= 0 {
		t.Fatalf("FFmpeg output %s is empty", path)
	}
}
