package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodeexecutor "github.com/nowen-video/nowen-video/internal/transcode/executor"
	"gorm.io/gorm"
)

const transcodePlannerVersion = "runtime-hls-v2"

func transcodeActiveKey(media *model.Media, quality string, startOffset float64) string {
	return stableHash(fmt.Sprintf(
		"%s|runtime_hls|%s|%.3f|%s|%s",
		media.ID,
		quality,
		startOffset,
		transcodeSourceFingerprint(media),
		transcodePlannerVersion,
	))
}

func (s *TranscodeService) createExecutionJob(media *model.Media, quality string, startOffset float64, legacyTaskID string) (*model.TranscodeJobRecord, error) {
	fingerprint := transcodeSourceFingerprint(media)
	planHash := stableHash(fmt.Sprintf("%s|%s|%.3f|%s", transcodePlannerVersion, quality, startOffset, s.hwAccel))
	activeKey := transcodeActiveKey(media, quality, startOffset)
	legacyID := strings.TrimSpace(legacyTaskID)
	job := &model.TranscodeJobRecord{
		MediaID:           media.ID,
		Intent:            "runtime_hls",
		ProfileID:         quality,
		AudioTrack:        -1,
		StartMS:           int64(startOffset * 1000),
		Priority:          100,
		Status:            "queued",
		DesiredState:      "running",
		ActiveKey:         &activeKey,
		SourceFingerprint: fingerprint,
		PlanHash:          planHash,
		PlannerVersion:    transcodePlannerVersion,
	}
	if legacyID != "" {
		job.LegacyTaskID = &legacyID
	}
	if err := s.executionRepo.CreateJob(job); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *TranscodeService) findActiveExecutionTask(media *model.Media, quality string, startOffset float64) (*model.TranscodeTask, error) {
	if s.executionRepo == nil || media == nil {
		return nil, gorm.ErrRecordNotFound
	}
	job, err := s.executionRepo.FindActiveByKey(transcodeActiveKey(media, quality, startOffset))
	if err != nil {
		return nil, err
	}
	if job.LegacyTaskID == nil || strings.TrimSpace(*job.LegacyTaskID) == "" {
		if job.Status == "queued" {
			_, _ = s.executionRepo.CompleteQueuedJob(job.ID, "failed", time.Now())
		}
		return nil, gorm.ErrRecordNotFound
	}
	task, err := s.repo.FindByID(*job.LegacyTaskID)
	if err != nil {
		if job.Status == "queued" {
			_, _ = s.executionRepo.CompleteQueuedJob(job.ID, "failed", time.Now())
		}
		return nil, gorm.ErrRecordNotFound
	}
	if task.Status != "pending" && task.Status != "running" {
		if job.Status == "queued" {
			_, _ = s.executionRepo.CompleteQueuedJob(job.ID, "failed", time.Now())
		}
		return nil, gorm.ErrRecordNotFound
	}
	return task, nil
}

func (s *TranscodeService) persistCancellation(job *TranscodeJob, requestedAt time.Time) error {
	if s.executionRepo == nil || job == nil || job.ExecutionJob == nil {
		return nil
	}
	return s.executionRepo.RequestCancellation(job.ExecutionJob.ID, requestedAt)
}

// persistJobTerminal returns false when this worker no longer owns the lease.
// Callers must not update the legacy projection or publish artifacts in that
// case, otherwise a stale worker could overwrite timeout recovery.
func (s *TranscodeService) persistJobTerminal(job *TranscodeJob, status string, completedAt time.Time) bool {
	if s.executionRepo == nil || job == nil || job.ExecutionJob == nil {
		return true
	}
	if job.leaseToken != "" {
		completed, err := s.executionRepo.CompleteLeasedJob(job.ExecutionJob.ID, job.leaseToken, status, completedAt)
		if err != nil {
			s.logger.Warnf("更新租约转码 Job 终态失败 job=%s status=%s: %v", job.ExecutionJob.ID, status, err)
			return false
		}
		if !completed {
			s.logger.Warnf("拒绝旧 Worker 写入终态 job=%s worker=%s status=%s", job.ExecutionJob.ID, job.workerID, status)
			return false
		}
		return true
	}
	if err := s.executionRepo.CompleteJob(job.ExecutionJob.ID, status, completedAt); err != nil {
		s.logger.Warnf("更新未租约转码 Job 终态失败 job=%s status=%s: %v", job.ExecutionJob.ID, status, err)
		return false
	}
	return true
}

func (s *TranscodeService) createAttempt(job *TranscodeJob, number int, backend string, args []string) (*model.TranscodeAttemptRecord, error) {
	if s.executionRepo == nil || job == nil || job.ExecutionJob == nil {
		return nil, nil
	}
	commandJSON, _ := json.Marshal(map[string]any{
		"path": s.cfg.App.FFmpegPath,
		"args": redactFFmpegArgs(args),
	})
	attempt := &model.TranscodeAttemptRecord{
		JobID:       job.ExecutionJob.ID,
		Number:      number,
		Backend:     backend,
		Status:      "preparing",
		CommandJSON: string(commandJSON),
		ExitCode:    -1,
	}
	if err := s.executionRepo.CreateAttempt(attempt); err != nil {
		return nil, err
	}
	job.ExecutionJob.CurrentAttemptID = attempt.ID
	return attempt, nil
}

func (s *TranscodeService) markAttemptStarted(job *TranscodeJob, attempt *model.TranscodeAttemptRecord, pid int, startedAt time.Time) {
	if s.executionRepo == nil || attempt == nil {
		return
	}
	if err := s.executionRepo.MarkAttemptStarted(attempt.ID, pid, startedAt); err != nil {
		s.logger.Warnf("标记 Attempt 启动失败 attempt=%s: %v", attempt.ID, err)
	}
	if job == nil || job.ExecutionJob == nil {
		return
	}
	running, err := s.executionRepo.SetJobRunning(job.ExecutionJob.ID, attempt.ID, job.leaseToken, startedAt)
	if err != nil {
		s.logger.Warnf("标记 Job 运行失败 job=%s: %v", job.ExecutionJob.ID, err)
		job.RequestCancel()
		return
	}
	if !running {
		s.logger.Warnf("Job Lease 已失效，拒绝进入运行态 job=%s worker=%s", job.ExecutionJob.ID, job.workerID)
		job.RequestCancel()
	}
}

func (s *TranscodeService) touchAttempt(attempt *model.TranscodeAttemptRecord, at time.Time) {
	if s.executionRepo == nil || attempt == nil {
		return
	}
	if err := s.executionRepo.TouchAttempt(attempt.ID, at); err != nil {
		s.logger.Debugf("更新 Attempt 心跳失败 attempt=%s: %v", attempt.ID, err)
	}
}

func (s *TranscodeService) completeAttempt(attempt *model.TranscodeAttemptRecord, result transcodeexecutor.Result) {
	if s.executionRepo == nil || attempt == nil {
		return
	}
	status := "completed"
	errorCode := ""
	if result.Cancelled || result.TimedOut {
		status = "cancelled"
		if result.TimedOut {
			errorCode = "deadline_exceeded"
		} else {
			errorCode = "cancelled"
		}
	} else if result.Err != nil {
		status = "failed"
		errorCode = "process_failed"
	}
	completedAt := result.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	if err := s.executionRepo.CompleteAttempt(
		attempt.ID,
		status,
		result.ExitCode,
		strings.Join(result.StderrTail, "\n"),
		errorCode,
		result.ErrorText(),
		completedAt,
	); err != nil {
		s.logger.Warnf("完成 Attempt 记录失败 attempt=%s: %v", attempt.ID, err)
	}
}

func (s *TranscodeService) publishHLSArtifact(job *TranscodeJob) {
	if s.executionRepo == nil || job == nil || job.ExecutionJob == nil {
		return
	}
	masterPath := filepath.Join(job.Task.OutputDir, "stream.m3u8")
	info, err := os.Stat(masterPath)
	if err != nil {
		s.logger.Warnf("转码完成但 HLS manifest 不存在 job=%s path=%s: %v", job.ExecutionJob.ID, masterPath, err)
		return
	}
	_ = s.executionRepo.DeleteArtifactByJobAndKind(job.ExecutionJob.ID, "hls_variant", job.Quality)
	artifact := &model.TranscodeArtifactRecord{
		JobID:           job.ExecutionJob.ID,
		AttemptID:       job.ExecutionJob.CurrentAttemptID,
		Kind:            "hls_variant",
		ProfileID:       job.Quality,
		Path:            job.Task.OutputDir,
		Status:          "published",
		SizeBytes:       info.Size(),
		DurationMS:      int64(job.Media.Duration * 1000),
		SegmentDuration: hlsTargetSegmentSeconds,
	}
	if err := s.executionRepo.CreateArtifact(artifact); err != nil {
		s.logger.Warnf("记录 HLS 产物失败 job=%s: %v", job.ExecutionJob.ID, err)
	}
}

func transcodeSourceFingerprint(media *model.Media) string {
	if media == nil {
		return ""
	}
	parts := []string{
		media.ID,
		media.FilePath,
		media.StreamURL,
		strings.ToLower(media.VideoCodec),
		strings.ToLower(media.AudioCodec),
		fmt.Sprintf("%.3f", media.Duration),
		media.Resolution,
	}
	if media.FilePath != "" && !IsWebDAVPath(media.FilePath) {
		if info, err := os.Stat(media.FilePath); err == nil {
			parts = append(parts, fmt.Sprintf("%d", info.Size()), info.ModTime().UTC().Format(time.RFC3339Nano))
		}
	}
	return stableHash(strings.Join(parts, "|"))
}

func stableHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func redactFFmpegArgs(args []string) []string {
	result := make([]string, len(args))
	for index, arg := range args {
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			result[index] = SprintSafeFFmpegURL(arg)
		} else {
			result[index] = arg
		}
	}
	return result
}
