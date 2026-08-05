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

// transcodePriorityQueue is retained only as a migration-compatible shell.
// Runtime playback moved to PlaybackSessionService, so this queue must never
// accept submissions or Claim a database Job again. It intentionally remains
// lifecycle-open until Shutdown so the TranscodeService retirement sweeper can
// keep fencing old rolling-upgrade Leases and deleting their files.
type transcodePriorityQueue struct {
	executionRepo *repository.TranscodeExecutionRepo
	legacyRepo    *repository.TranscodeRepo
	logger        *zap.SugaredLogger
	capacity      int64
	pollInterval  time.Duration
	wake          chan struct{}
	done          chan struct{}
	closeOnce     sync.Once

	// runtimeRetired is an invariant, not a feature flag. Re-enabling this queue
	// would recreate the removed media-keyed persistent playback architecture.
	runtimeRetired bool

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
		runtimeRetired: true,
	}
}

func (q *transcodePriorityQueue) CanAccept() bool {
	if q == nil || q.runtimeRetired || q.IsClosed() || q.executionRepo == nil {
		return false
	}
	if err := transcodeQueueAdmissionError(q); err != nil {
		q.warnf("磁盘压力下拒绝新转码任务: %v", err)
		return false
	}
	depth, err := q.executionRepo.CountQueuedJobs()
	if err != nil {
		q.warnf("读取持久化转码队列深度失败: %v", err)
		return false
	}
	return depth < q.capacity
}

// Push remains only for source compatibility. Runtime Jobs are rejected before
// any wake-up or capacity query, even when an old caller constructs a Job value.
func (q *transcodePriorityQueue) Push(job *TranscodeJob) bool {
	if q == nil || q.runtimeRetired || job == nil || job.ExecutionJob == nil || q.IsClosed() {
		return false
	}
	if err := transcodeQueueAdmissionError(q); err != nil {
		q.warnf("磁盘压力在 Job 创建后关闭队列准入 job=%s: %v", job.ExecutionJob.ID, err)
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

// Notify is still available to migration and shutdown code. It does not make a
// retired queue claimable; Pop always fails closed while runtimeRetired is true.
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

// Promote is already persisted by PromoteQueuedJob. A retired queue never
// consumes the promoted row, but keeping this adapter avoids unsafe dual paths.
func (q *transcodePriorityQueue) Promote(_ string, _ int) bool {
	if q == nil || q.runtimeRetired {
		return false
	}
	return q.Notify()
}

func (q *transcodePriorityQueue) Pop(workerID string, leaseDuration time.Duration) (*TranscodeJob, bool) {
	if q == nil || q.runtimeRetired || q.executionRepo == nil || q.legacyRepo == nil {
		return nil, false
	}
	for {
		if q.IsClosed() {
			return nil, false
		}
		// Existing FFmpeg processes keep their Lease and Reservation. Only the
		// next database Claim is paused so queued work cannot deepen pressure.
		if !transcodeQueueClaimAllowed(q) {
			if !q.waitForWork() {
				return nil, false
			}
			continue
		}

		now := time.Now()
		candidateIDs, err := q.executionRepo.ListQueuedJobCandidates(now, defaultTranscodeQueueScanLimit)
		if err != nil {
			q.warnf("读取转码候选 Job 失败 worker=%s: %v", workerID, err)
			if !q.waitForWork() {
				return nil, false
			}
			continue
		}

		for _, jobID := range candidateIDs {
			reservationErr := transcodeReserveQueueCandidate(q, jobID)
			if reservationErr != nil {
				// Capacity shortage is an expected scheduling result. Keep the Job
				// queued and continue scanning so a smaller candidate can proceed.
				if !isTranscodeReservationCapacityError(reservationErr) {
					q.warnf("获取转码空间 Reservation 失败 job=%s worker=%s: %v", jobID, workerID, reservationErr)
				}
				continue
			}

			record, claimed, claimErr := q.executionRepo.ClaimJob(jobID, workerID, now, leaseDuration)
			if claimErr != nil {
				q.warnf("数据库领取转码 Job 失败 job=%s worker=%s: %v", jobID, workerID, claimErr)
				continue
			}
			if !claimed {
				continue
			}
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
	if !supportedTranscodeIntent(record) {
		return nil, fmt.Errorf("unsupported transcode intent %q", record.Intent)
	}
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
// stops the retirement lifecycle and cannot affect database queue ownership.
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
	if q == nil || q.runtimeRetired || q.executionRepo == nil {
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
