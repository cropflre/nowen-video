package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

const (
	TranscodePriorityBackground  = 20
	TranscodePriorityRetry       = 70
	TranscodePriorityInteractive = 100

	defaultTranscodeQueuePollInterval = 500 * time.Millisecond
	defaultTranscodeQueueScanLimit    = 16
)

// transcodePriorityQueue is a database-backed Priority + FIFO scheduler. The
// channel is only a wake-up hint; transcode_jobs is the sole source of pending
// work and ClaimNextQueuedJob is the ownership boundary across processes.
type transcodePriorityQueue struct {
	executionRepo *repository.TranscodeExecutionRepo
	legacyRepo    *repository.TranscodeRepo
	logger        *zap.SugaredLogger
	capacity      int64
	pollInterval  time.Duration
	wake          chan struct{}
	done          chan struct{}
	closeOnce     sync.Once

	mu     sync.RWMutex
	closed bool
}

func newTranscodePriorityQueue(
	executionRepo *repository.TranscodeExecutionRepo,
	legacyRepo *repository.TranscodeRepo,
	capacity int,
	logger *zap.SugaredLogger,
) *transcodePriorityQueue {
	if capacity <= 0 {
		capacity = 1
	}
	return &transcodePriorityQueue{
		executionRepo: executionRepo,
		legacyRepo:    legacyRepo,
		logger:        logger,
		capacity:      int64(capacity),
		pollInterval:  defaultTranscodeQueuePollInterval,
		wake:          make(chan struct{}, 1),
		done:          make(chan struct{}),
	}
}

func (q *transcodePriorityQueue) CanAccept() bool {
	if q == nil || q.IsClosed() || q.executionRepo == nil {
		return false
	}
	depth, err := q.executionRepo.CountQueuedJobs()
	if err != nil {
		q.warnf("读取持久化转码队列深度失败: %v", err)
		return false
	}
	return depth < q.capacity
}

// Push preserves the existing submission call shape but never stores the Job
// pointer. The durable row has already been created; this only validates global
// capacity and wakes database-polling workers.
func (q *transcodePriorityQueue) Push(job *TranscodeJob) bool {
	if q == nil || job == nil || job.ExecutionJob == nil || q.IsClosed() {
		return false
	}
	depth, err := q.executionRepo.CountQueuedJobs()
	if err != nil {
		q.warnf("读取持久化转码队列深度失败: %v", err)
		return false
	}
	if depth > q.capacity {
		return false
	}
	return q.Notify()
}

func (q *transcodePriorityQueue) Notify() bool {
	if q == nil || q.IsClosed() {
		return false
	}
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return true
}

// Promote is already persisted by PromoteQueuedJob. Waking workers is enough;
// their next database selection observes the new priority atomically.
func (q *transcodePriorityQueue) Promote(_ string, _ int) bool {
	return q.Notify()
}

func (q *transcodePriorityQueue) Pop(workerID string, leaseDuration time.Duration) (*TranscodeJob, bool) {
	if q == nil || q.executionRepo == nil || q.legacyRepo == nil {
		return nil, false
	}
	for {
		if q.IsClosed() {
			return nil, false
		}

		record, claimed, err := q.executionRepo.ClaimNextQueuedJob(
			workerID,
			time.Now(),
			leaseDuration,
			defaultTranscodeQueueScanLimit,
		)
		if err != nil {
			q.warnf("数据库领取转码 Job 失败 worker=%s: %v", workerID, err)
		} else if claimed {
			if q.IsClosed() {
				q.releaseClaimAfterClose(record)
				return nil, false
			}
			job, hydrateErr := q.hydrateClaimedJob(record)
			if hydrateErr == nil {
				if q.IsClosed() {
					job.RequestCancel()
					q.releaseClaimAfterClose(record)
					return nil, false
				}
				return job, true
			}
			q.failClaimedPayload(record, hydrateErr)
			continue
		}

		if !q.waitForWork() {
			return nil, false
		}
	}
}

func (q *transcodePriorityQueue) releaseClaimAfterClose(record *model.TranscodeJobRecord) bool {
	if record == nil || record.LeaseToken == "" {
		return false
	}
	released, err := q.executionRepo.RequeueLeasedJob(record.ID, record.LeaseToken, time.Now())
	if err != nil {
		q.warnf("服务关闭释放刚领取的转码 Lease 失败 job=%s worker=%s: %v", record.ID, record.WorkerID, err)
		return false
	}
	if released {
		q.warnf("服务关闭已释放刚领取的转码 Lease job=%s worker=%s", record.ID, record.WorkerID)
	}
	return released
}

func (q *transcodePriorityQueue) hydrateClaimedJob(record *model.TranscodeJobRecord) (*TranscodeJob, error) {
	task, media, err := q.executionRepo.LoadJobPayload(record)
	if err != nil {
		return nil, err
	}
	if _, ok := qualityPresets[record.ProfileID]; !ok {
		return nil, fmt.Errorf("unknown transcode profile %q", record.ProfileID)
	}
	if task.OutputDir == "" {
		return nil, fmt.Errorf("transcode task %s has empty output directory", task.ID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &TranscodeJob{
		Task:         task,
		ExecutionJob: record,
		Media:        media,
		Quality:      record.ProfileID,
		ctx:          ctx,
		cancel:       cancel,
		workerID:     record.WorkerID,
		leaseToken:   record.LeaseToken,
		startOffset:  float64(record.StartMS) / 1000,
		leaseDone:    make(chan struct{}),
		throttleDone: make(chan struct{}),
	}, nil
}

func (q *transcodePriorityQueue) failClaimedPayload(record *model.TranscodeJobRecord, cause error) {
	if record == nil {
		return
	}
	now := time.Now()
	completed, err := q.executionRepo.CompleteLeasedJob(record.ID, record.LeaseToken, "failed", now)
	if err != nil {
		q.warnf("终结无法重建的转码 Job 失败 job=%s: %v", record.ID, err)
		return
	}
	if !completed {
		return
	}
	if record.LegacyTaskID != nil && *record.LegacyTaskID != "" {
		if task, findErr := q.legacyRepo.FindByID(*record.LegacyTaskID); findErr == nil {
			task.Status = "failed"
			task.Error = fmt.Sprintf("无法恢复持久化转码任务: %v", cause)
			task.CompletedAt = &now
			_ = q.legacyRepo.Update(task)
		}
	}
	q.warnf("持久化转码 Job 载荷无效，已终结 job=%s: %v", record.ID, cause)
}

func (q *transcodePriorityQueue) waitForWork() bool {
	timer := time.NewTimer(q.PollInterval())
	defer timer.Stop()
	select {
	case <-q.wake:
		return !q.IsClosed()
	case <-timer.C:
		return !q.IsClosed()
	case <-q.done:
		return false
	}
}

func (q *transcodePriorityQueue) Close() {
	if q == nil {
		return
	}
	q.closeOnce.Do(func() {
		q.mu.Lock()
		q.closed = true
		q.mu.Unlock()
		close(q.done)
		select {
		case q.wake <- struct{}{}:
		default:
		}
	})
}

// There are no unclaimed process-local deliveries to drain. Closing only
// stops future database Claims; queued rows remain durable for the next start.
func (q *transcodePriorityQueue) CloseAndDrain() []*TranscodeJob {
	q.Close()
	return nil
}

func (q *transcodePriorityQueue) Done() <-chan struct{} {
	if q == nil || q.done == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return q.done
}

func (q *transcodePriorityQueue) IsClosed() bool {
	if q == nil {
		return true
	}
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.closed
}

func (q *transcodePriorityQueue) Len() int {
	if q == nil || q.executionRepo == nil {
		return 0
	}
	depth, err := q.executionRepo.CountQueuedJobs()
	if err != nil {
		q.warnf("读取持久化转码队列深度失败: %v", err)
		return 0
	}
	return int(depth)
}

func (q *transcodePriorityQueue) PollInterval() time.Duration {
	if q == nil || q.pollInterval <= 0 {
		return defaultTranscodeQueuePollInterval
	}
	return q.pollInterval
}

func (q *transcodePriorityQueue) warnf(template string, args ...any) {
	if q != nil && q.logger != nil {
		q.logger.Warnf(template, args...)
	}
}
