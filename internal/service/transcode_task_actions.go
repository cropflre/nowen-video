package service

import (
	"fmt"
	"os"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

// CancelTranscode writes desired_state=cancelled before signalling the process
// context. A queued task therefore cannot lose cancellation, and recovery will
// not revive it after a restart.
func (s *TranscodeService) CancelTranscode(taskID string) error {
	s.mu.RLock()
	job, exists := s.running[taskID]
	s.mu.RUnlock()
	if !exists {
		return fmt.Errorf("转码任务不存在或已完成: %s", taskID)
	}

	job.taskMu.Lock()
	if job.Task.Status == "done" || job.Task.Status == "failed" || job.Task.Status == "cancelled" {
		job.taskMu.Unlock()
		return fmt.Errorf("转码任务已经结束: %s", taskID)
	}
	now := time.Now()
	if err := s.persistCancellation(job, now); err != nil {
		job.taskMu.Unlock()
		return fmt.Errorf("持久化转码取消意图失败: %w", err)
	}
	job.Task.Status = "cancelled"
	job.Task.Error = ""
	job.Task.CompletedAt = &now
	err := s.repo.Update(job.Task)
	job.taskMu.Unlock()
	if err != nil {
		return fmt.Errorf("持久化取消状态失败: %w", err)
	}

	job.RequestCancel()
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
