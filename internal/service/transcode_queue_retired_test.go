package service

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"go.uber.org/zap"
)

func TestPersistentRuntimeQueueIsPermanentlyRetired(t *testing.T) {
	queue := newTranscodePriorityQueue(nil, nil, 100, zap.NewNop().Sugar())
	if queue == nil || !queue.runtimeRetired {
		t.Fatal("persistent runtime queue must be constructed retired")
	}
	if queue.IsClosed() {
		t.Fatal("retired queue must remain lifecycle-open for migration sweeps")
	}
	if queue.CanAccept() {
		t.Fatal("retired queue accepted a submission")
	}
	if queue.Push(&TranscodeJob{ExecutionJob: &model.TranscodeJobRecord{ID: "runtime-job"}}) {
		t.Fatal("retired queue accepted a durable Job")
	}
	if job, ok := queue.Pop("worker", time.Minute); ok || job != nil {
		t.Fatalf("retired queue claimed work: ok=%v job=%+v", ok, job)
	}
	if got := queue.Len(); got != 0 {
		t.Fatalf("retired queue exposed depth %d", got)
	}
	if !queue.Notify() {
		t.Fatal("retirement lifecycle wake-up must remain available before shutdown")
	}
	queue.Close()
	if !queue.IsClosed() {
		t.Fatal("retired queue did not close during shutdown")
	}
}
