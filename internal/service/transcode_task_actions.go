package service

import (
	"fmt"
	"os"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

// CancelTranscode persists desired_state=cancelled before signalling any local
// process. Unlike the legacy implementation it also handles queued rows that
// were reconstructed from the database and therefore have no local job yet.
func (s *TranscodeService) CancelTranscode(taskID string) error {
	task, err := s.repo.FindByID(taskID)
	if err != nil {
		return fmt.Errorf("转码任务不存在: %w", err)
	}
	if task.Status == "done" || task.Status == "failed" || task.Status == "cancelled" {
		return fmt.Errorf("转码任务已经结束: %s", taskID)
	}

	executionJob, err := s.executionRepo.FindActiveByLegacyTaskID(taskID)
	if err != nil {
		if repository.IsNotFound(err) {
			return fmt.Errorf("转码执行 Job 不存在或已完成: %s", taskID)
		}
		return fmt.Errorf("读取转码执行 Job 失败: %w", err)
	}

	now := time.Now()
	if err := s.executionRepo.RequestCancellation(executionJob.ID, now); err != nil {
		return fmt.Errorf("持久化转码取消意图失败: %w", err)
	}

	task.Status = "cancelled"
	task.Error = ""
	task.CompletedAt = &now
	if err := s.repo.Update(task); err != nil {
		return fmt.Errorf("持久化取消状态失败: %w", err)
	}

	s.mu.RLock()
	localJob := s.running[taskID]
	s.mu.RUnlock()
	if localJob != nil {
		localJob.RequestCancel()
	}

	// A queued row has no process and no Lease owner to publish its terminal
	// state. Finalize it immediately, but only if Claim did not win the race.
	if executionJob.Status == "queued" && executionJob.LeaseToken == "" {
		completed, completeErr := s.executionRepo.CompleteUnleasedJob(executionJob.ID, "cancelled", now)
		if completeErr != nil {
			return fmt.Errorf("确认排队任务取消失败: %w", completeErr)
		}
		if completed {
			s.mu.Lock()
			if current := s.running[taskID]; current != nil {
				current.RequestCancel()
				delete(s.running, taskID)
			}
			s.mu.Unlock()
			s.broadcastTranscodeEvent(EventTranscodeCancelled, &TranscodeProgressData{
				TaskID:  task.ID,
				MediaID: task.MediaID,
				Title:   task.MediaTitle,
				Quality: task.Quality,
				Message: fmt.Sprintf("转码已取消: %s (%s)", task.MediaTitle, task.Quality),
			})
		}
	}

	s.jobs.Notify()
	s.logger.Infof("转码取消意图与兼容任务状态已持久化: %s", taskID)
	return nil
}

func (s *TranscodeService) DeleteTask(taskID string) error {
	task, err := s.repo.FindByID(taskID)
	if err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}
	if task.Status == "running" || task.Status == "pending" {
		return fmt.Errorf("运行中的任务不可删除，请先取消")
	}
	if task.OutputDir != "" {
		if err := os.RemoveAll(task.OutputDir); err != nil {
			s.logger.Warnf("删除转码输出目录失败 %s: %v", task.OutputDir, err)
		}
		s.InvalidateCacheDiskUsage()
	}
	return s.repo.DeleteByID(taskID)
}

func (s *TranscodeService) RetryTask(taskID string, mediaResolver func(mediaID string) (*model.Media, error)) error {
	task, err := s.repo.FindByID(taskID)
	if err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}
	if task.Status == "running" || task.Status == "pending" {
		return fmt.Errorf("任务正在运行中，无需重试")
	}
	if mediaResolver == nil {
		return fmt.Errorf("缺少媒体解析器")
	}
	media, err := mediaResolver(task.MediaID)
	if err != nil {
		return fmt.Errorf("查找媒体失败: %w", err)
	}
	task.Retries++
	task.Error = ""
	if err := s.repo.Update(task); err != nil {
		return fmt.Errorf("更新重试计数失败: %w", err)
	}
	if _, err := s.startTranscodeWithPriority(media, task.Quality, 0, TranscodePriorityRetry); err != nil {
		return fmt.Errorf("重新启动转码失败: %w", err)
	}
	return nil
}

func (s *TranscodeService) BatchCancelTasks(taskIDs []string) (int, error) {
	cancelled := 0
	for _, id := range taskIDs {
		if err := s.CancelTranscode(id); err == nil {
			cancelled++
		}
	}
	return cancelled, nil
}

func (s *TranscodeService) BatchDeleteTasks(taskIDs []string) (int64, error) {
	if len(taskIDs) == 0 {
		return 0, nil
	}
	cleared := false
	for _, id := range taskIDs {
		task, err := s.repo.FindByID(id)
		if err != nil || task.OutputDir == "" || task.Status == "running" || task.Status == "pending" {
			continue
		}
		if err := os.RemoveAll(task.OutputDir); err != nil {
			s.logger.Warnf("删除转码输出目录失败 %s: %v", task.OutputDir, err)
		}
		cleared = true
	}
	if cleared {
		s.InvalidateCacheDiskUsage()
	}
	return s.repo.DeleteByIDs(taskIDs)
}

func (s *TranscodeService) BatchRetryTasks(taskIDs []string, mediaResolver func(mediaID string) (*model.Media, error)) (int, error) {
	retried := 0
	for _, id := range taskIDs {
		if err := s.RetryTask(id, mediaResolver); err == nil {
			retried++
		}
	}
	return retried, nil
}

func (s *TranscodeService) BatchSubmitByMediaIDs(mediaIDs, qualities []string, mediaResolver func(string) (*model.Media, error)) (submitted int, skipped int, tasks []*model.TranscodeTask, errs []string) {
	if len(mediaIDs) == 0 {
		return 0, 0, nil, []string{"media_ids 不能为空"}
	}
	if len(qualities) == 0 {
		qualities = []string{"720p"}
	}
	for _, mediaID := range mediaIDs {
		media, err := mediaResolver(mediaID)
		if err != nil || media == nil {
			skipped++
			errs = append(errs, fmt.Sprintf("%s: 媒体不存在", mediaID))
			continue
		}
		available := s.GetAvailableQualities(media)
		availableSet := make(map[string]struct{}, len(available))
		for _, quality := range available {
			availableSet[quality] = struct{}{}
		}
		effective := make([]string, 0, len(qualities))
		for _, quality := range qualities {
			if _, ok := availableSet[quality]; ok {
				effective = append(effective, quality)
			}
		}
		if len(effective) == 0 && len(available) > 0 {
			effective = []string{available[0]}
		}
		mediaSubmitted := 0
		for _, quality := range effective {
			task, err := s.startTranscodeWithPriority(media, quality, 0, TranscodePriorityBackground)
			if err != nil {
				s.logger.Warnf("BatchSubmitByMediaIDs: %s/%s 启动失败: %v", mediaID, quality, err)
				continue
			}
			tasks = append(tasks, task)
			mediaSubmitted++
		}
		if mediaSubmitted > 0 {
			submitted++
		} else {
			skipped++
			errs = append(errs, fmt.Sprintf("%s: 所有档位启动均失败", mediaID))
		}
	}
	return submitted, skipped, tasks, errs
}
