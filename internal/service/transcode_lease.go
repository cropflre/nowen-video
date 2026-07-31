package service

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

const (
	defaultTranscodeLeaseDuration          = 30 * time.Second
	defaultTranscodeLeaseHeartbeatInterval = 8 * time.Second
	defaultTranscodeLeaseRecoveryInterval  = 10 * time.Second
)

func newTranscodeInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "nowen-video"
	}
	hostname = strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(strings.TrimSpace(hostname))
	return fmt.Sprintf("%s-%s", hostname, uuid.NewString())
}

func (s *TranscodeService) claimExecutionJob(job *TranscodeJob, workerID string) (bool, error) {
	if job == nil || job.ExecutionJob == nil {
		return false, fmt.Errorf("持久化转码 Job 不存在")
	}
	claimed, ok, err := s.executionRepo.ClaimJob(
		job.ExecutionJob.ID,
		workerID,
		time.Now(),
		s.leaseDuration,
	)
	if err != nil || !ok {
		return false, err
	}
	job.ExecutionJob = claimed
	job.workerID = claimed.WorkerID
	job.leaseToken = claimed.LeaseToken
	return true, nil
}

func (s *TranscodeService) leaseHeartbeatLoop(job *TranscodeJob) {
	if job == nil || job.ExecutionJob == nil || job.leaseToken == "" {
		return
	}
	ticker := time.NewTicker(s.leaseHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-job.leaseDone:
			return
		case <-ticker.C:
			now := time.Now()
			renewed, err := s.executionRepo.RenewJobLease(
				job.ExecutionJob.ID,
				job.leaseToken,
				now,
				s.leaseDuration,
			)
			if err != nil {
				s.logger.Warnf("转码 Lease 续租失败 job=%s worker=%s: %v", job.ExecutionJob.ID, job.workerID, err)
				continue
			}
			if !renewed {
				s.logger.Warnf("转码 Lease 已失效，终止旧 Worker job=%s worker=%s", job.ExecutionJob.ID, job.workerID)
				job.RequestCancel()
				return
			}
		}
	}
}

func (s *TranscodeService) handleUnclaimedJob(job *TranscodeJob) {
	if job == nil || job.ExecutionJob == nil {
		return
	}
	current, err := s.executionRepo.FindJobByID(job.ExecutionJob.ID)
	if err != nil {
		s.logger.Warnf("读取未 Claim 的转码 Job 失败 job=%s: %v", job.ExecutionJob.ID, err)
		return
	}
	if current.DesiredState == "cancelled" || current.Status == "cancel_requested" || job.CancellationRequested() {
		s.finalizeCancelled(job)
		return
	}
	s.logger.Warnf(
		"转码 Job 未获得执行权，跳过本地队列项 job=%s status=%s worker=%s",
		current.ID,
		current.Status,
		current.WorkerID,
	)
}

func (s *TranscodeService) failUnclaimedJob(job *TranscodeJob, reason string) {
	if job == nil || job.ExecutionJob == nil {
		return
	}
	now := time.Now()
	completed, err := s.executionRepo.CompleteQueuedJob(job.ExecutionJob.ID, "failed", now)
	if err != nil {
		s.logger.Warnf("终止未 Claim 的转码 Job 失败 job=%s: %v", job.ExecutionJob.ID, err)
		return
	}
	if !completed {
		s.handleUnclaimedJob(job)
		return
	}
	job.taskMu.Lock()
	job.Task.Status = "failed"
	job.Task.Error = reason
	job.Task.CompletedAt = &now
	legacyErr := s.repo.Update(job.Task)
	job.taskMu.Unlock()
	if legacyErr != nil {
		s.logger.Warnf("更新未 Claim 任务兼容投影失败 task=%s: %v", job.Task.ID, legacyErr)
	}
	s.broadcastTranscodeEvent(EventTranscodeFailed, &TranscodeProgressData{
		TaskID:  job.Task.ID,
		MediaID: job.Media.ID,
		Title:   job.Media.Title,
		Quality: job.Quality,
		Message: fmt.Sprintf("转码调度失败: %s", reason),
	})
}

func (s *TranscodeService) recoverPendingTasks() {
	now := time.Now()
	activeJobs, err := s.executionRepo.ListActiveJobs()
	if err != nil {
		s.logger.Warnf("读取待恢复转码 Job 失败: %v", err)
		return
	}

	recovered := 0
	for i := range activeJobs {
		job := &activeJobs[i]
		switch job.Status {
		case "queued":
			completed, completeErr := s.executionRepo.CompleteQueuedJob(job.ID, "failed", now)
			if completeErr != nil {
				s.logger.Warnf("回收重启前排队 Job 失败 job=%s: %v", job.ID, completeErr)
				continue
			}
			if completed {
				s.updateRecoveredLegacyTask(job, "failed", "服务重启前任务尚未被 Worker Claim，请重新提交", now)
				recovered++
			}
		case "claimed", "running", "cancel_requested":
			if job.LeaseExpiresAt == nil || !job.LeaseExpiresAt.After(now) {
				if s.recoverExpiredLease(job, now) {
					recovered++
				}
			}
		default:
			if err := s.executionRepo.CompleteJob(job.ID, "failed", now); err != nil {
				s.logger.Warnf("回收未知状态 Job 失败 job=%s status=%s: %v", job.ID, job.Status, err)
				continue
			}
			s.updateRecoveredLegacyTask(job, "failed", "服务重启时发现未知执行状态", now)
			recovered++
		}
	}

	legacyRows, legacyErr := s.repo.ListRunning()
	if legacyErr != nil {
		s.logger.Warnf("读取旧转码任务兼容投影失败: %v", legacyErr)
	} else {
		for i := range legacyRows {
			if _, findErr := s.executionRepo.FindActiveByLegacyTaskID(legacyRows[i].ID); !repository.IsNotFound(findErr) {
				continue
			}
			legacyRows[i].Status = "failed"
			legacyRows[i].Error = "执行 Job 已丢失，请重新提交"
			legacyRows[i].CompletedAt = &now
			if updateErr := s.repo.Update(&legacyRows[i]); updateErr != nil {
				s.logger.Warnf("修复孤立旧转码任务失败 task=%s: %v", legacyRows[i].ID, updateErr)
			}
		}
	}

	if recovered > 0 {
		s.logger.Infof("已回收 %d 个重启前遗留的转码 Job", recovered)
	}
}

func (s *TranscodeService) leaseRecoveryLoop() {
	ticker := time.NewTicker(s.leaseRecoveryInterval)
	defer ticker.Stop()
	for now := range ticker.C {
		expired, err := s.executionRepo.ListExpiredLeases(now)
		if err != nil {
			s.logger.Warnf("扫描过期转码 Lease 失败: %v", err)
			continue
		}
		for i := range expired {
			s.recoverExpiredLease(&expired[i], now)
		}
	}
}

func (s *TranscodeService) recoverExpiredLease(job *model.TranscodeJobRecord, now time.Time) bool {
	if job == nil || job.LeaseToken == "" {
		return false
	}
	terminalStatus := "failed"
	errorMessage := fmt.Sprintf("Worker Lease 已过期: %s", job.WorkerID)
	if job.DesiredState == "cancelled" || job.Status == "cancel_requested" {
		terminalStatus = "cancelled"
		errorMessage = "取消请求已确认，Worker Lease 已释放"
	}
	completed, err := s.executionRepo.CompleteExpiredLease(job.ID, job.LeaseToken, terminalStatus, now)
	if err != nil {
		s.logger.Warnf("回收过期转码 Lease 失败 job=%s worker=%s: %v", job.ID, job.WorkerID, err)
		return false
	}
	if !completed {
		return false
	}
	s.updateRecoveredLegacyTask(job, terminalStatus, errorMessage, now)
	s.logger.Warnf("已回收过期转码 Lease job=%s worker=%s status=%s", job.ID, job.WorkerID, terminalStatus)
	return true
}

func (s *TranscodeService) updateRecoveredLegacyTask(job *model.TranscodeJobRecord, status, errorMessage string, now time.Time) {
	if job == nil || job.LegacyTaskID == nil || strings.TrimSpace(*job.LegacyTaskID) == "" {
		return
	}
	task, err := s.repo.FindByID(*job.LegacyTaskID)
	if err != nil {
		return
	}
	if status == "cancelled" {
		task.Status = "cancelled"
		task.Error = ""
	} else {
		task.Status = "failed"
		task.Error = errorMessage
	}
	task.CompletedAt = &now
	if err := s.repo.Update(task); err != nil {
		s.logger.Warnf("更新 Lease 回收兼容投影失败 task=%s: %v", task.ID, err)
	}
}
