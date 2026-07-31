package service

import (
	"fmt"
	"strings"
	"time"

	transcodeexecutor "github.com/nowen-video/nowen-video/internal/transcode/executor"
)

func (s *TranscodeService) recordProgress(job *TranscodeJob, progress transcodeexecutor.Progress) {
	if job == nil || job.Media == nil || job.Task == nil || job.CancellationRequested() {
		return
	}
	currentSec := progress.CurrentSec + job.startOffset
	job.transcodedPos.Store(uint64(currentSec * 100))

	percentage := 0.0
	if job.Media.Duration > 0 {
		percentage = currentSec / job.Media.Duration * 100
		if percentage > 100 {
			percentage = 100
		}
	}

	job.taskMu.Lock()
	if job.Task.Status != "running" {
		job.taskMu.Unlock()
		return
	}
	job.Task.Progress = percentage
	job.taskMu.Unlock()

	lastPersisted := float64(job.lastDBProgress.Load()) / 100
	if percentage-lastPersisted >= 5 || percentage >= 99.5 {
		job.lastDBProgress.Store(uint64(percentage * 100))
		if err := s.repo.UpdateProgress(job.Task.ID, percentage); err != nil {
			s.logger.Warnf("更新转码进度失败 task=%s: %v", job.Task.ID, err)
		}
	}

	s.broadcastTranscodeEvent(EventTranscodeProgress, &TranscodeProgressData{
		TaskID:   job.Task.ID,
		MediaID:  job.Media.ID,
		Title:    job.Media.Title,
		Quality:  job.Quality,
		Progress: percentage,
		Speed:    progress.Speed,
		Message:  fmt.Sprintf("转码中: %.1f%% (速度: %s)", percentage, progress.Speed),
	})
}

func (s *TranscodeService) markJobRunning(job *TranscodeJob) bool {
	job.taskMu.Lock()
	defer job.taskMu.Unlock()
	if job.CancellationRequested() || job.Task.Status == "cancelled" {
		return false
	}
	now := time.Now()
	job.Task.Status = "running"
	job.Task.StartedAt = &now
	job.Task.Error = ""
	if err := s.repo.Update(job.Task); err != nil {
		s.logger.Warnf("标记转码任务运行失败 task=%s: %v", job.Task.ID, err)
	}
	return true
}

func (s *TranscodeService) finalizeCancelled(job *TranscodeJob) {
	job.taskMu.Lock()
	now := time.Now()
	job.Task.Status = "cancelled"
	job.Task.Error = ""
	job.Task.CompletedAt = &now
	err := s.repo.Update(job.Task)
	job.taskMu.Unlock()
	if err != nil {
		s.logger.Warnf("持久化转码取消状态失败 task=%s: %v", job.Task.ID, err)
	}
	s.broadcastTranscodeEvent(EventTranscodeCancelled, &TranscodeProgressData{
		TaskID:  job.Task.ID,
		MediaID: job.Media.ID,
		Title:   job.Media.Title,
		Quality: job.Quality,
		Message: fmt.Sprintf("转码已取消: %s (%s)", job.Media.Title, job.Quality),
	})
}

func (s *TranscodeService) finalizeFailed(job *TranscodeJob, result transcodeexecutor.Result) {
	errorText := strings.TrimSpace(result.ErrorText())
	if errorText == "" {
		errorText = "FFmpeg 进程异常退出"
	}
	job.taskMu.Lock()
	now := time.Now()
	job.Task.Status = "failed"
	job.Task.Error = errorText
	job.Task.CompletedAt = &now
	err := s.repo.Update(job.Task)
	job.taskMu.Unlock()
	if err != nil {
		s.logger.Warnf("持久化转码失败状态失败 task=%s: %v", job.Task.ID, err)
	}
	s.broadcastTranscodeEvent(EventTranscodeFailed, &TranscodeProgressData{
		TaskID:  job.Task.ID,
		MediaID: job.Media.ID,
		Title:   job.Media.Title,
		Quality: job.Quality,
		Message: fmt.Sprintf("转码失败: %s", errorText),
	})
}

func (s *TranscodeService) finalizeCompleted(job *TranscodeJob) {
	job.taskMu.Lock()
	now := time.Now()
	job.Task.Status = "done"
	job.Task.Progress = 100
	job.Task.CompletedAt = &now
	job.Task.Error = ""
	err := s.repo.Update(job.Task)
	job.taskMu.Unlock()
	if err != nil {
		s.logger.Warnf("持久化转码完成状态失败 task=%s: %v", job.Task.ID, err)
	}
	s.broadcastTranscodeEvent(EventTranscodeCompleted, &TranscodeProgressData{
		TaskID:   job.Task.ID,
		MediaID:  job.Media.ID,
		Title:    job.Media.Title,
		Quality:  job.Quality,
		Progress: 100,
		Message:  fmt.Sprintf("转码完成: %s (%s)", job.Media.Title, job.Quality),
	})
}

func (s *TranscodeService) broadcastTranscodeEvent(eventType string, data *TranscodeProgressData) {
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(eventType, data)
	}
}
