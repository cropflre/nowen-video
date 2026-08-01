package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	transcodeexecutor "github.com/nowen-video/nowen-video/internal/transcode/executor"
	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	"gorm.io/gorm"
)

type transcodeAttemptExecution struct {
	Attempt   *model.TranscodeAttemptRecord
	Artifact  *model.TranscodeArtifactRecord
	Workspace string
	Args      []string
}

func (s *TranscodeService) prepareAttemptExecution(job *TranscodeJob, attemptNumber int, backend string) (*transcodeAttemptExecution, error) {
	if s == nil || s.artifactStore == nil || s.executionRepo == nil {
		return nil, fmt.Errorf("transcode artifact store is unavailable")
	}
	if job == nil || job.ExecutionJob == nil || job.Media == nil {
		return nil, fmt.Errorf("transcode job is incomplete")
	}
	attemptID := uuid.NewString()
	artifactID := uuid.NewString()
	workspace, err := s.artifactStore.PrepareWorkspace(job.ExecutionJob.ID, attemptID)
	if err != nil {
		return nil, err
	}

	args := s.buildFFmpegArgsForBackendWithProbe(
		job.Media,
		job.Probe,
		job.Media.FilePath,
		workspace,
		job.Quality,
		job.startOffset,
		backend,
	)
	commandJSON, _ := json.Marshal(map[string]any{
		"path": s.cfg.App.FFmpegPath,
		"args": redactFFmpegArgs(args),
	})
	now := time.Now()
	attempt := &model.TranscodeAttemptRecord{
		ID:            attemptID,
		JobID:         job.ExecutionJob.ID,
		Number:        attemptNumber,
		Backend:       backend,
		Status:        "preparing",
		CommandJSON:   string(commandJSON),
		WorkspacePath: workspace,
		ExitCode:      -1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.executionRepo.CreateAttempt(attempt); err != nil {
		_ = s.artifactStore.Remove(workspace)
		return nil, err
	}

	artifact := &model.TranscodeArtifactRecord{
		ID:                artifactID,
		JobID:             job.ExecutionJob.ID,
		AttemptID:         attemptID,
		MediaID:           job.Media.ID,
		Kind:              "hls_variant",
		ProfileID:         job.Quality,
		SourceFingerprint: job.ExecutionJob.SourceFingerprint,
		PlannerVersion:    job.ExecutionJob.PlannerVersion,
		TempPath:          workspace,
		Status:            "staging",
		DurationMS:        int64(job.Media.Duration * 1000),
		SegmentDuration:   hlsTargetSegmentSeconds,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.executionRepo.CreateArtifact(artifact); err != nil {
		_ = s.executionRepo.CompleteAttempt(
			attempt.ID,
			"failed",
			-1,
			"",
			"artifact_create_failed",
			err.Error(),
			time.Now(),
		)
		_ = s.artifactStore.Remove(workspace)
		return nil, err
	}

	job.CurrentAttempt = attempt
	job.CurrentArtifact = artifact
	job.taskMu.Lock()
	job.Task.OutputDir = workspace
	updateErr := s.repo.Update(job.Task)
	job.taskMu.Unlock()
	if updateErr != nil {
		s.logger.Warnf("同步 Attempt 工作区到兼容任务失败 task=%s: %v", job.Task.ID, updateErr)
	}
	return &transcodeAttemptExecution{
		Attempt:   attempt,
		Artifact:  artifact,
		Workspace: workspace,
		Args:      args,
	}, nil
}

func (s *TranscodeService) completeAttemptArtifact(job *TranscodeJob, execution *transcodeAttemptExecution, result transcodeexecutor.Result) {
	if s == nil || s.executionRepo == nil || execution == nil || execution.Artifact == nil || result.Err == nil && !result.Cancelled && !result.TimedOut {
		return
	}
	status := "failed"
	errorCode := "process_failed"
	if result.Cancelled || result.TimedOut {
		status = "cancelled"
		errorCode = "cancelled"
		if result.TimedOut {
			errorCode = "deadline_exceeded"
		}
	}
	completedAt := result.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	owned, err := s.executionRepo.MarkOwnedArtifactTerminal(
		job.ExecutionJob.ID,
		execution.Attempt.ID,
		execution.Artifact.ID,
		job.leaseToken,
		status,
		errorCode,
		result.ErrorText(),
		completedAt,
	)
	if err != nil {
		s.logger.Warnf("完成 Attempt Artifact 状态失败 artifact=%s: %v", execution.Artifact.ID, err)
	}
	if !owned {
		_ = s.executionRepo.MarkArtifactAbandoned(
			execution.Artifact.ID,
			"lease_lost",
			"Attempt lost Job Lease before artifact terminal update",
			completedAt,
		)
	}
}

// publishCurrentHLSArtifact validates, prepares, atomically renames and commits
// one immutable HLS Artifact. Job completion happens in the same database
// transaction as Artifact visibility.
func (s *TranscodeService) publishCurrentHLSArtifact(job *TranscodeJob) (bool, error) {
	if s == nil || s.artifactStore == nil || s.executionRepo == nil {
		return false, fmt.Errorf("transcode artifact store is unavailable")
	}
	if job == nil || job.ExecutionJob == nil || job.CurrentAttempt == nil || job.CurrentArtifact == nil {
		return false, fmt.Errorf("transcode attempt artifact is missing")
	}
	validation, err := s.artifactStore.ValidateHLS(job.CurrentArtifact.TempPath)
	if err != nil {
		_, _ = s.executionRepo.MarkOwnedArtifactTerminal(
			job.ExecutionJob.ID,
			job.CurrentAttempt.ID,
			job.CurrentArtifact.ID,
			job.leaseToken,
			"failed",
			"artifact_validation_failed",
			err.Error(),
			time.Now(),
		)
		return false, fmt.Errorf("validate hls artifact: %w", err)
	}

	publishedDir, err := s.artifactStore.PublishedDir(job.Media.ID, job.Quality, job.CurrentArtifact.ID)
	if err != nil {
		return false, err
	}
	manifestPath := filepath.Join(publishedDir, "stream.m3u8")
	preparedAt := time.Now()
	prepared, err := s.executionRepo.PrepareArtifactPublish(
		job.ExecutionJob.ID,
		job.CurrentAttempt.ID,
		job.CurrentArtifact.ID,
		job.leaseToken,
		publishedDir,
		manifestPath,
		preparedAt,
	)
	if err != nil {
		return false, fmt.Errorf("prepare artifact publish: %w", err)
	}
	if !prepared {
		_ = s.executionRepo.MarkArtifactAbandoned(
			job.CurrentArtifact.ID,
			"lease_lost",
			"Job Lease changed before artifact publish",
			preparedAt,
		)
		return false, nil
	}

	if err := s.artifactStore.Publish(job.CurrentArtifact.TempPath, publishedDir); err != nil {
		_, _ = s.executionRepo.MarkOwnedArtifactTerminal(
			job.ExecutionJob.ID,
			job.CurrentAttempt.ID,
			job.CurrentArtifact.ID,
			job.leaseToken,
			"failed",
			"artifact_publish_failed",
			err.Error(),
			time.Now(),
		)
		return false, err
	}

	completedAt := time.Now()
	committed, err := s.executionRepo.CommitArtifactPublishAndCompleteJob(
		job.ExecutionJob.ID,
		job.CurrentAttempt.ID,
		job.CurrentArtifact.ID,
		job.leaseToken,
		validation.SizeBytes,
		int64(job.Media.Duration*1000),
		completedAt,
	)
	if err != nil {
		_ = s.executionRepo.MarkArtifactAbandoned(
			job.CurrentArtifact.ID,
			"publish_commit_failed",
			err.Error(),
			completedAt,
		)
		return false, fmt.Errorf("commit artifact publish: %w", err)
	}
	if !committed {
		_ = s.executionRepo.MarkArtifactAbandoned(
			job.CurrentArtifact.ID,
			"lease_lost",
			"Job Lease changed before artifact publish commit",
			completedAt,
		)
		return false, nil
	}

	job.CurrentArtifact.Status = "published"
	job.CurrentArtifact.Path = publishedDir
	job.CurrentArtifact.ManifestPath = manifestPath
	job.CurrentArtifact.TempPath = ""
	job.CurrentArtifact.SizeBytes = validation.SizeBytes
	job.CurrentArtifact.PublishedAt = &completedAt
	job.taskMu.Lock()
	job.Task.OutputDir = publishedDir
	job.taskMu.Unlock()
	s.InvalidateCacheDiskUsage()
	return true, nil
}

func (s *TranscodeService) hasPublishedHLSArtifact(media *model.Media, quality string) bool {
	if s == nil || s.executionRepo == nil || media == nil {
		return false
	}
	artifact, err := s.executionRepo.FindPublishedHLSArtifact(
		media.ID,
		quality,
		transcodeSourceFingerprint(media),
		transcodePlannerVersion,
	)
	if err != nil || artifact == nil || artifact.Path == "" {
		return false
	}
	manifestPath := artifact.ManifestPath
	if manifestPath == "" {
		manifestPath = filepath.Join(artifact.Path, "stream.m3u8")
	}
	_, err = os.Stat(manifestPath)
	return err == nil
}

// ResolveHLSOutputDir is the only runtime HLS filesystem resolver. It first
// resolves a Lease-valid staging Artifact, then an immutable published version,
// then performs one bounded legacy import for historical shared directories.
func (s *TranscodeService) ResolveHLSOutputDir(media *model.Media, quality string) (string, error) {
	if s == nil || s.executionRepo == nil || media == nil {
		return "", gorm.ErrRecordNotFound
	}
	fingerprint := transcodeSourceFingerprint(media)
	artifact, err := s.executionRepo.FindReadableHLSArtifact(
		media.ID,
		quality,
		fingerprint,
		transcodePlannerVersion,
		time.Now(),
	)
	if err == nil && artifact != nil {
		switch artifact.Status {
		case "staging":
			if artifact.TempPath != "" {
				return artifact.TempPath, nil
			}
		case "publishing":
			if artifact.Path != "" {
				if _, statErr := os.Stat(artifact.Path); statErr == nil {
					return artifact.Path, nil
				}
			}
			if artifact.TempPath != "" {
				return artifact.TempPath, nil
			}
		case "published":
			if artifact.Path != "" {
				return artifact.Path, nil
			}
		}
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	return s.importLegacyHLSArtifact(media, quality, fingerprint)
}

func (s *TranscodeService) importLegacyHLSArtifact(media *model.Media, quality, fingerprint string) (string, error) {
	legacyDir := s.GetLegacyOutputDir(media.ID, quality)
	manifestPath := filepath.Join(legacyDir, "stream.m3u8")
	content, err := os.ReadFile(manifestPath)
	if err != nil || !strings.Contains(string(content), ".ts") {
		return "", gorm.ErrRecordNotFound
	}
	artifactID := stableHash(strings.Join([]string{
		"legacy_runtime_hls_v1",
		media.ID,
		quality,
		fingerprint,
		transcodePlannerVersion,
	}, "|"))
	now := time.Now()
	artifact := &model.TranscodeArtifactRecord{
		ID:                artifactID,
		JobID:             "legacy:" + artifactID,
		MediaID:           media.ID,
		Kind:              "hls_variant",
		ProfileID:         quality,
		SourceFingerprint: fingerprint,
		PlannerVersion:    transcodePlannerVersion,
		Path:              legacyDir,
		ManifestPath:      manifestPath,
		Status:            "published",
		MigrationSource:   "legacy_runtime_hls_v1",
		DurationMS:        int64(media.Duration * 1000),
		SegmentDuration:   hlsTargetSegmentSeconds,
		PublishedAt:       &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.executionRepo.ImportLegacyHLSArtifact(artifact); err != nil {
		return "", err
	}
	s.logger.Infof("已导入历史 HLS Artifact media=%s profile=%s path=%s", media.ID, quality, legacyDir)
	return legacyDir, nil
}

func (s *TranscodeService) WaitForFirstSegmentForMedia(ctx context.Context, media *model.Media, quality string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		outputDir, resolveErr := s.ResolveHLSOutputDir(media, quality)
		if resolveErr == nil {
			manifestPath := filepath.Join(outputDir, "stream.m3u8")
			if content, readErr := os.ReadFile(manifestPath); readErr == nil && strings.Contains(string(content), ".ts") {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func normalizeAttemptBackend(backend string) string {
	if backend == "" {
		return ffmpeg.HWAccelNone
	}
	return backend
}
