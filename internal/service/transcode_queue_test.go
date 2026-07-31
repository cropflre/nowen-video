package service

import (
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func queueTestJob(id string, priority int) *TranscodeJob {
	return &TranscodeJob{
		Task: &model.TranscodeTask{ID: id, Priority: priority},
		ExecutionJob: &model.TranscodeJobRecord{
			ID:       "execution-" + id,
			Priority: priority,
		},
	}
}

func TestTranscodePriorityQueueOrdersByPriority(t *testing.T) {
	queue := newTranscodePriorityQueue(4)
	background := queueTestJob("background", TranscodePriorityBackground)
	interactive := queueTestJob("interactive", TranscodePriorityInteractive)
	retry := queueTestJob("retry", TranscodePriorityRetry)

	if !queue.Push(background) || !queue.Push(interactive) || !queue.Push(retry) {
		t.Fatal("failed to enqueue test jobs")
	}

	first, ok := queue.Pop()
	if !ok || first.Task.ID != "interactive" {
		t.Fatalf("interactive job must run first: %+v", first)
	}
	second, ok := queue.Pop()
	if !ok || second.Task.ID != "retry" {
		t.Fatalf("retry job must run second: %+v", second)
	}
	third, ok := queue.Pop()
	if !ok || third.Task.ID != "background" {
		t.Fatalf("background job must run last: %+v", third)
	}
}

func TestTranscodePriorityQueuePromotesReusedBackgroundJob(t *testing.T) {
	queue := newTranscodePriorityQueue(3)
	firstBackground := queueTestJob("first-background", TranscodePriorityBackground)
	playbackTarget := queueTestJob("playback-target", TranscodePriorityBackground)
	retry := queueTestJob("retry", TranscodePriorityRetry)
	if !queue.Push(firstBackground) || !queue.Push(playbackTarget) || !queue.Push(retry) {
		t.Fatal("failed to enqueue promotion test jobs")
	}
	if !queue.Promote(playbackTarget.ExecutionJob.ID, TranscodePriorityInteractive) {
		t.Fatal("queued background job was not promoted")
	}
	if queue.Promote(playbackTarget.ExecutionJob.ID, TranscodePriorityRetry) {
		t.Fatal("lower priority must not demote a promoted job")
	}
	if playbackTarget.ExecutionJob.Priority != TranscodePriorityInteractive || playbackTarget.Task.Priority != TranscodePriorityInteractive {
		t.Fatalf("job projections did not receive promoted priority: %+v %+v", playbackTarget.ExecutionJob, playbackTarget.Task)
	}

	first, ok := queue.Pop()
	if !ok || first.Task.ID != "playback-target" {
		t.Fatalf("promoted playback target must run first: %+v", first)
	}
	second, ok := queue.Pop()
	if !ok || second.Task.ID != "retry" {
		t.Fatalf("retry must remain ahead of background work: %+v", second)
	}
}

func TestTranscodePriorityQueuePreservesFIFOWithinPriority(t *testing.T) {
	queue := newTranscodePriorityQueue(3)
	for _, id := range []string{"first", "second", "third"} {
		if !queue.Push(queueTestJob(id, TranscodePriorityInteractive)) {
			t.Fatalf("failed to enqueue %s", id)
		}
	}
	for _, expected := range []string{"first", "second", "third"} {
		job, ok := queue.Pop()
		if !ok || job.Task.ID != expected {
			t.Fatalf("expected %s, got %+v", expected, job)
		}
	}
}

func TestTranscodePriorityQueueCapacityAndClose(t *testing.T) {
	queue := newTranscodePriorityQueue(1)
	if !queue.Push(queueTestJob("first", TranscodePriorityBackground)) {
		t.Fatal("first enqueue failed")
	}
	if queue.Push(queueTestJob("overflow", TranscodePriorityInteractive)) {
		t.Fatal("queue accepted more than its capacity")
	}
	queue.Close()
	job, ok := queue.Pop()
	if !ok || job.Task.ID != "first" {
		t.Fatalf("close must preserve already queued work: %+v", job)
	}
	if job, ok := queue.Pop(); ok || job != nil {
		t.Fatalf("closed empty queue must stop workers: %+v", job)
	}
	if queue.Push(queueTestJob("late", TranscodePriorityInteractive)) {
		t.Fatal("closed queue accepted new work")
	}
}

func TestTranscodePriorityQueueCloseAndDrainPreservesDurableOrder(t *testing.T) {
	queue := newTranscodePriorityQueue(3)
	background := queueTestJob("background", TranscodePriorityBackground)
	interactive := queueTestJob("interactive", TranscodePriorityInteractive)
	retry := queueTestJob("retry", TranscodePriorityRetry)
	if !queue.Push(background) || !queue.Push(interactive) || !queue.Push(retry) {
		t.Fatal("failed to enqueue drain test jobs")
	}

	drained := queue.CloseAndDrain()
	if len(drained) != 3 {
		t.Fatalf("expected three drained jobs, got %d", len(drained))
	}
	for index, expected := range []string{"interactive", "retry", "background"} {
		if drained[index].Task.ID != expected {
			t.Fatalf("expected drained[%d]=%s, got %s", index, expected, drained[index].Task.ID)
		}
	}
	if !queue.IsClosed() || queue.Len() != 0 {
		t.Fatalf("drained queue must be closed and empty: closed=%v len=%d", queue.IsClosed(), queue.Len())
	}
	if job, ok := queue.Pop(); ok || job != nil {
		t.Fatalf("drained queue returned work: %+v", job)
	}
	if queue.Push(queueTestJob("late", TranscodePriorityInteractive)) {
		t.Fatal("drained queue accepted new work")
	}
}
