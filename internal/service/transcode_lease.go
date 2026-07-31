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

func (s *TranscodeService) recoverPendingTasks() {
	now := time.Now()
	activeJobs, err := s.executionRepo.ListActiveJobs()
	if err != nil {
		s.logger.Warnf("读取待恢复转码 Job 失败: %v", err)
		return
	}

	recovered := 0
	queued := 0
	for i := range activeJobs {
		job := &activeJobs[i]
		switch job.Status {
		case "queued":
			if job.DesiredState == "cancelled" {
				completed, completeErr := s.executionRepo.CompleteQueuedJob(job.ID, "cancelled", now)
				if completeErr != nil {
					s.logger.Warnf("确认重启前排队取消失败 job=%s: %v", job.ID, completeErr)
					continue
				}
				if completed {
					s.updateRecoveredLegacyTask(job, "cancelled", "", now)
					recovered++
				}
				continue
			}
			s.updateRecoveredLegacyTask(job, "queued", "", now)
			queued++

		case "claimed", "running", "cancel_requested":
			if job.DesiredState == "cancelled" || job.Status == "cancel_requested" {
				if job.LeaseToken == "" || job.LeaseExpiresAt == nil {
					if completeErr := s.executionRepo.CompleteJob(job.ID, "cancelled", now); completeErr != nil {
						s.logger.Warnf("确认无租约取消 Job 失败 job=%s: %v", job.ID, completeErr)
						continue
					}
					s.updateRecoveredLegacyTask(job, "cancelled", "", now)
					recovered++
					continue
				}
				if !job.LeaseExpiresAt.After(now) && s.recoverExpiredLease(job, now) {
					recovered++
				}
				continue
			}

			// Active rows written before Lease ownership existed are safe to
			// return to queued at startup, before this process starts Workers.
			if job.LeaseToken == "" || job.LeaseExpiresAt == nil {
				requeued, requeueErr := s.executionRepo.RequeueUnleasedJob(job.ID, now)
				if requeueErr != nil {
					s.logger.Warnf("恢复无租约 Job 失败 job=%s: %v", job.ID, requeueErr)
					continue
				}
				if requeued {
					s.updateRecoveredLegacyTask(job, "queued", "", now)
					queued++
				}
				continue
			}
			if !job.LeaseExpiresAt.After(now) && s.recoverExpiredLease(job, now) {
				recovered++
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

	if queued > 0 {
		s.jobs.Notify()
		s.logger.Infof("已保留并唤醒 %d 个数据库排队转码 Job", queued)
	}
	if recovered > 0 {
		s.logger.Infof("已恢复或回收 %d 个重启前转码 Job", recovered)
	}
}

func (s *TranscodeService) leaseRecoveryLoop() {
	ticker := time.NewTicker(s.leaseRecoveryInterval)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			if s.jobs.IsClosed() {
				return
			}
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
}

func (s *TranscodeService) recoverExpiredLease(job *model.TranscodeJobRecord, now time.Time) bool {
	if job == nil || job.LeaseToken == "" {
		return false
	}
	if job.DesiredState == "cancelled" || job.Status == "cancel_requested" {
		completed, err := s.executionRepo.CompleteExpiredLease(job.ID, job.LeaseToken, "cancelled", now)
		if err != nil {
			s.logger.Warnf("确认过期 Lease 取消失败 job=%s worker=%s: %v", job.ID, job.WorkerID, err)
			return false
		}
		if !completed {
			return false
		}
		s.updateRecoveredLegacyTask(job, "cancelled", "", now)
		s.logger.Warnf("已确认过期取消 Job job=%s worker=%s", job.ID, job.WorkerID)
		return true
	}

	requeued, err := s.executionRepo.RequeueExpiredLease(job.ID, job.LeaseToken, now)
	if err != nil {
		s.logger.Warnf("重新排队过期转码 Lease 失败 job=%s worker=%s: %v", job.ID, job.WorkerID, err)
		return false
	}
	if !requeued {
		return false
	}
	s.updateRecoveredLegacyTask(job, "queued", "", now)
	s.jobs.Notify()
	s.logger.Warnf("Worker Lease 已过期，任务重新排队 job=%s worker=%s", job.ID, job.WorkerID)
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
	switch status {
	case "queued":
		task.Status = "pending"
		task.Progress = 0
		task.Error = ""
		task.Priority = job.Priority
		task.StartedAt = nil
		task.CompletedAt = nil
	case "cancelled":
		task.Status = "cancelled"
		task.Error = ""
		task.CompletedAt = &now
	default:
		task.Status = "failed"
		task.Error = errorMessage
		task.CompletedAt = &now
	}
	if err := s.repo.Update(task); err != nil {
		s.logger.Warnf("更新 Lease 恢复兼容投影失败 task=%s: %v", task.ID, err)
	}
}
