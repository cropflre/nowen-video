package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	transcodeexecutor "github.com/nowen-video/nowen-video/internal/transcode/executor"
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
	if _, err := validateTimestampExecution(job.ExecutionJob, backend); err != nil {
		return nil, fmt.Errorf("validate timestamp execution: %w", err)
	}
	attemptID := uuid.NewString()
	artifactID := uuid.NewString()
	workspace, err := s.artifactStore.PrepareWorkspace(job.ExecutionJob.ID, attemptID)
	if err != nil {
		s.reportStorageOperationFailure(storageOperationPrepareWorkspace, s.artifactStore.Root(), err, time.Now())
		return nil, err
	}

	args, err := s.buildJobFFmpegArgsChecked(job, workspace, backend)
	if err != nil {
		_ = s.artifactStore.Remove(workspace)
		return nil, fmt.Errorf("build ffmpeg execution plan: %w", err)
	}
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
		ID:                   artifactID,
		JobID:                job.ExecutionJob.ID,
		AttemptID:            attemptID,
		MediaID:              job.Media.ID,
		Kind:                 transcodeArtifactKind(job),
		ProfileID:            job.Quality,
		SourceFingerprint:    job.ExecutionJob.SourceFingerprint,
		PlannerVersion:       job.ExecutionJob.PlannerVersion,
		EncodingPlanVersion:  job.ExecutionJob.EncodingPlanVersion,
		EncodingPlanHash:     job.ExecutionJob.EncodingPlanHash,
		EncodingPlanJSON:     job.ExecutionJob.EncodingPlanJSON,
		TimestampPlanVersion: job.ExecutionJob.TimestampPlanVersion,
		TimestampPlanHash:    job.ExecutionJob.TimestampPlanHash,
		TimestampPlanJSON:    job.ExecutionJob.TimestampPlanJSON,
		TimelineOriginMS:     job.ExecutionJob.TimelineOriginMS,
		TempPath:             workspace,
		Status:               "staging",
		DurationMS:           transcodeArtifactDurationMS(job),
		SegmentDuration:      hlsTargetSegmentSeconds,
		CreatedAt:            now,
		UpdatedAt:            now,
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

// publishCurrentHLSArtifact validates, attests, prepares, atomically renames and
// commits one immutable HLS Artifact. This primitive is retained for explicit
// administrator workflows and historical execution certification; runtime
// playback has no read path into the published Artifact store.
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
	if err := s.attestOwnedArtifactForPublish(job); err != nil {
		_, _ = s.executionRepo.MarkOwnedArtifactTerminal(
			job.ExecutionJob.ID,
			job.CurrentAttempt.ID,
			job.CurrentArtifact.ID,
			job.leaseToken,
			"failed",
			"artifact_attestation_failed",
			err.Error(),
			time.Now(),
		)
		return false, fmt.Errorf("attest hls artifact: %w", err)
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
		failedAt := time.Now()
		s.reportStorageOperationFailure(storageOperationPublishArtifact, publishedDir, err, failedAt)
		_, _ = s.executionRepo.MarkOwnedArtifactTerminal(
			job.ExecutionJob.ID,
			job.CurrentAttempt.ID,
			job.CurrentArtifact.ID,
			job.leaseToken,
			"failed",
			"artifact_publish_failed",
			err.Error(),
			failedAt,
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
		transcodeArtifactDurationMS(job),
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

func normalizeAttemptBackend(backend string) string {
	if backend == "" {
		return ffmpeg.HWAccelNone
	}
	return backend
}
