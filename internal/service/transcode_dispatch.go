package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

const defaultTranscodeDispatchInterval = time.Second

// dispatchQueuedLoop keeps the process-local delivery heap synchronized with
// durable queued rows. ClaimJob remains the ownership boundary, so multiple
// instances may observe the same row without executing it twice.
func (s *TranscodeService) dispatchQueuedLoop() {
	ticker := time.NewTicker(defaultTranscodeDispatchInterval)
	defer ticker.Stop()
	for {
		if s.jobs.IsClosed() {
			return
		}
		select {
		case <-ticker.C:
			s.dispatchPersistedQueuedJobs()
		}
	}
}

func (s *TranscodeService) dispatchPersistedQueuedJobs() int {
	if s == nil || s.executionRepo == nil || s.jobs == nil || s.jobs.IsClosed() {
		return 0
	}
	activeJobs, err := s.executionRepo.ListActiveJobs()
	if err != nil {
		s.logger.Warnf("扫描持久化转码队列失败: %v", err)
		return 0
	}
	dispatched := 0
	for i := range activeJobs {
		record := &activeJobs[i]
		if record.Status != "queued" || record.DesiredState != "running" {
			continue
		}
		if s.enqueuePersistedJob(record) {
			dispatched++
		}
		if s.jobs.IsClosed() {
			break
		}
	}
	return dispatched
}

func (s *TranscodeService) enqueuePersistedJob(record *model.TranscodeJobRecord) bool {
	if record == nil || record.Status != "queued" || record.DesiredState != "running" || record.ActiveKey == nil {
		return false
	}
	if record.LegacyTaskID == nil || strings.TrimSpace(*record.LegacyTaskID) == "" {
		now := time.Now()
		if completed, err := s.executionRepo.CompleteQueuedJob(record.ID, "failed", now); err != nil {
			s.logger.Warnf("终结缺少兼容任务的排队 Job 失败 job=%s: %v", record.ID, err)
		} else if completed {
			s.logger.Warnf("排队 Job 缺少 legacy_task_id，已终结 job=%s", record.ID)
		}
		return false
	}

	taskID := strings.TrimSpace(*record.LegacyTaskID)
	s.mu.RLock()
	existing := s.running[taskID]
	s.mu.RUnlock()
	if existing != nil {
		if existing.ExecutionJob != nil && record.Priority > existing.ExecutionJob.Priority {
			s.jobs.Promote(record.ID, record.Priority)
		}
		return false
	}

	task, err := s.repo.FindByID(taskID)
	if err != nil {
		now := time.Now()
		_, _ = s.executionRepo.CompleteQueuedJob(record.ID, "failed", now)
		s.logger.Warnf("恢复排队 Job 时兼容任务不存在 job=%s task=%s: %v", record.ID, taskID, err)
		return false
	}
	if _, ok := qualityPresets[record.ProfileID]; !ok {
		now := time.Now()
		_, _ = s.executionRepo.CompleteQueuedJob(record.ID, "failed", now)
		task.Status = "failed"
		task.Error = fmt.Sprintf("恢复任务时发现未知转码档位: %s", record.ProfileID)
		task.CompletedAt = &now
		_ = s.repo.Update(task)
		return false
	}

	var media model.Media
	if err := s.repo.DB().First(&media, "id = ?", record.MediaID).Error; err != nil {
		now := time.Now()
		_, _ = s.executionRepo.CompleteQueuedJob(record.ID, "failed", now)
		task.Status = "failed"
		task.Error = "恢复任务时媒体记录已不存在"
		task.CompletedAt = &now
		_ = s.repo.Update(task)
		s.logger.Warnf("恢复排队 Job 时媒体不存在 job=%s media=%s: %v", record.ID, record.MediaID, err)
		return false
	}

	if task.OutputDir == "" {
		task.OutputDir = s.GetOutputDir(media.ID, record.ProfileID)
	}
	task.Status = "pending"
	task.Progress = 0
	task.Error = ""
	task.Priority = record.Priority
	task.StartedAt = nil
	task.CompletedAt = nil
	if err := s.repo.Update(task); err != nil {
		s.logger.Warnf("恢复排队 Job 的兼容投影失败 job=%s task=%s: %v", record.ID, task.ID, err)
		return false
	}

	nextAttempt, err := s.executionRepo.NextAttemptNumber(record.ID)
	if err != nil {
		s.logger.Warnf("读取恢复任务 Attempt 编号失败 job=%s: %v", record.ID, err)
		return false
	}
	if nextAttempt > 1 {
		s.cleanAttemptOutput(task.OutputDir)
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &TranscodeJob{
		Task:         task,
		ExecutionJob: record,
		Media:        &media,
		Quality:      record.ProfileID,
		ctx:          ctx,
		cancel:       cancel,
		startOffset:  float64(record.StartMS) / 1000,
		leaseDone:    make(chan struct{}),
		throttleDone: make(chan struct{}),
	}

	s.mu.Lock()
	if current := s.running[task.ID]; current != nil {
		s.mu.Unlock()
		job.RequestCancel()
		return false
	}
	s.running[task.ID] = job
	s.mu.Unlock()

	if !s.jobs.Push(job) {
		s.mu.Lock()
		if s.running[task.ID] == job {
			delete(s.running, task.ID)
		}
		s.mu.Unlock()
		job.RequestCancel()
		return false
	}

	s.logger.Infof(
		"已恢复持久化转码队列 job=%s task=%s media=%s quality=%s priority=%d output=%s",
		record.ID,
		task.ID,
		media.ID,
		record.ProfileID,
		record.Priority,
		filepath.Clean(task.OutputDir),
	)
	return true
}
