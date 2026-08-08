package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/transcode/executor"
	resourcegovernor "github.com/nowen-video/nowen-video/internal/transcode/governor"
)

type blockingRunner struct {
	started chan struct{}
	release chan struct{}

	mu     sync.Mutex
	active int
	peak   int
}

func newBlockingRunner() *blockingRunner {
	return &blockingRunner{
		started: make(chan struct{}, 2),
		release: make(chan struct{}, 2),
	}
}

func (r *blockingRunner) Run(ctx context.Context, _ executor.Command, _ executor.Callbacks) executor.Result {
	r.mu.Lock()
	r.active++
	if r.active > r.peak {
		r.peak = r.active
	}
	r.mu.Unlock()

	r.started <- struct{}{}
	result := executor.Result{}
	select {
	case <-r.release:
	case <-ctx.Done():
		result.Err = ctx.Err()
		result.Cancelled = ctx.Err() == context.Canceled
		result.TimedOut = ctx.Err() == context.DeadlineExceeded
	}

	r.mu.Lock()
	r.active--
	r.mu.Unlock()
	return result
}

func (r *blockingRunner) peakActive() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.peak
}

func TestRuntimeQueuesContendingProcessKinds(t *testing.T) {
	runner := newBlockingRunner()
	resourceGovernor := resourcegovernor.New(resourcegovernor.Config{SoftwareTranscodes: 1})
	runtime := New(runner, resourceGovernor)

	firstResult := make(chan executor.Result, 1)
	go func() {
		firstResult <- runtime.Run(context.Background(), resourcegovernor.KindSoftwareTranscode, executor.Command{}, executor.Callbacks{})
	}()
	waitForRuntimeSignal(t, runner.started, "first runner start")

	secondResult := make(chan executor.Result, 1)
	go func() {
		secondResult <- runtime.Run(context.Background(), resourcegovernor.KindSoftwareTranscode, executor.Command{}, executor.Callbacks{})
	}()

	waitForRuntimeSnapshot(t, runtime, time.Second, func(snapshot resourcegovernor.Snapshot) bool {
		return snapshot.InUse[resourcegovernor.KindSoftwareTranscode] == 1 &&
			snapshot.Waiting[resourcegovernor.KindSoftwareTranscode] == 1 &&
			snapshot.PeakInUse[resourcegovernor.KindSoftwareTranscode] == 1
	})

	select {
	case <-runner.started:
		t.Fatal("second runner bypassed governor admission")
	case <-time.After(50 * time.Millisecond):
	}

	runner.release <- struct{}{}
	waitForRuntimeResult(t, firstResult, "first runner completion")
	waitForRuntimeSignal(t, runner.started, "second runner start")

	snapshot := runtime.Snapshot()
	if snapshot.InUse[resourcegovernor.KindSoftwareTranscode] != 1 || snapshot.Waiting[resourcegovernor.KindSoftwareTranscode] != 0 {
		t.Fatalf("unexpected snapshot after handoff: %+v", snapshot)
	}
	if snapshot.PeakInUse[resourcegovernor.KindSoftwareTranscode] != 1 {
		t.Fatalf("software pool was oversubscribed: %+v", snapshot)
	}

	runner.release <- struct{}{}
	waitForRuntimeResult(t, secondResult, "second runner completion")

	finalSnapshot := runtime.Snapshot()
	if finalSnapshot.InUse[resourcegovernor.KindSoftwareTranscode] != 0 || finalSnapshot.Waiting[resourcegovernor.KindSoftwareTranscode] != 0 {
		t.Fatalf("runtime resources did not drain: %+v", finalSnapshot)
	}
	if runner.peakActive() != 1 {
		t.Fatalf("runner observed %d concurrent executions", runner.peakActive())
	}
}

func waitForRuntimeSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitForRuntimeResult(t *testing.T, result <-chan executor.Result, description string) executor.Result {
	t.Helper()
	select {
	case value := <-result:
		if value.Err != nil {
			t.Fatalf("%s failed: %v", description, value.Err)
		}
		return value
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", description)
		return executor.Result{}
	}
}

func waitForRuntimeSnapshot(t *testing.T, runtime *Runtime, timeout time.Duration, predicate func(resourcegovernor.Snapshot) bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate(runtime.Snapshot()) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("runtime snapshot did not reach expected state: %+v", runtime.Snapshot())
}
