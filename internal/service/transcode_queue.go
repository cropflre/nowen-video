package service

import (
	"container/heap"
	"sync"
)

const (
	TranscodePriorityBackground  = 20
	TranscodePriorityRetry       = 70
	TranscodePriorityInteractive = 100
)

type queuedTranscodeJob struct {
	job      *TranscodeJob
	priority int
	sequence uint64
	index    int
}

type transcodeJobHeap []*queuedTranscodeJob

func (h transcodeJobHeap) Len() int { return len(h) }
func (h transcodeJobHeap) Less(i, j int) bool {
	if h[i].priority == h[j].priority {
		return h[i].sequence < h[j].sequence
	}
	return h[i].priority > h[j].priority
}
func (h transcodeJobHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *transcodeJobHeap) Push(value any) {
	item := value.(*queuedTranscodeJob)
	item.index = len(*h)
	*h = append(*h, item)
}
func (h *transcodeJobHeap) Pop() any {
	old := *h
	last := len(old) - 1
	item := old[last]
	old[last] = nil
	item.index = -1
	*h = old[:last]
	return item
}

// transcodePriorityQueue is the process-local delivery layer. Priority is also
// persisted on transcode_jobs, so replacing this queue with database polling
// later will not change the scheduling contract.
type transcodePriorityQueue struct {
	mu       sync.Mutex
	cond     *sync.Cond
	items    transcodeJobHeap
	capacity int
	sequence uint64
	closed   bool
}

func newTranscodePriorityQueue(capacity int) *transcodePriorityQueue {
	if capacity <= 0 {
		capacity = 1
	}
	queue := &transcodePriorityQueue{capacity: capacity}
	queue.cond = sync.NewCond(&queue.mu)
	heap.Init(&queue.items)
	return queue
}

func (q *transcodePriorityQueue) Push(job *TranscodeJob) bool {
	if q == nil || job == nil {
		return false
	}
	priority := TranscodePriorityInteractive
	if job.ExecutionJob != nil {
		priority = job.ExecutionJob.Priority
	} else if job.Task != nil {
		priority = job.Task.Priority
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || len(q.items) >= q.capacity {
		return false
	}
	q.sequence++
	heap.Push(&q.items, &queuedTranscodeJob{
		job:      job,
		priority: priority,
		sequence: q.sequence,
	})
	q.cond.Signal()
	return true
}

func (q *transcodePriorityQueue) Pop() (*TranscodeJob, bool) {
	if q == nil {
		return nil, false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return nil, false
	}
	item := heap.Pop(&q.items).(*queuedTranscodeJob)
	return item.job, true
}

func (q *transcodePriorityQueue) Close() {
	if q == nil {
		return
	}
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

func (q *transcodePriorityQueue) Len() int {
	if q == nil {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
