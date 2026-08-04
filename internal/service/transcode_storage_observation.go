package service

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"
	"time"
)

const transcodeStorageObservationInterval = 30 * time.Second

type transcodeStorageObservationState struct {
	mu            sync.Mutex
	attemptID     string
	lastSampledAt time.Time
	observedBytes int64
}

var transcodeStorageObservationStates sync.Map

func storageObservationState(job *TranscodeJob) *transcodeStorageObservationState {
	if job == nil {
		return &transcodeStorageObservationState{}
	}
	state := &transcodeStorageObservationState{}
	actual, _ := transcodeStorageObservationStates.LoadOrStore(job, state)
	return actual.(*transcodeStorageObservationState)
}

func forgetStorageObservation(job *TranscodeJob) {
	if job != nil {
		transcodeStorageObservationStates.Delete(job)
	}
}

func currentStorageObservationTarget(job *TranscodeJob) (string, string) {
	if job == nil || job.CurrentAttempt == nil {
		return "", ""
	}
	workspace := job.CurrentAttempt.WorkspacePath
	if job.CurrentArtifact != nil && job.CurrentArtifact.TempPath != "" {
		workspace = job.CurrentArtifact.TempPath
	}
	return job.CurrentAttempt.ID, workspace
}

// observeJobStorageReservation refunds only materialized bytes that are already
// represented by a fresh Artifact Store disk sample. The ordering is critical:
// workspace scan -> invalidate/force store sample -> persist observed bytes.
// Sampling failure keeps the original full commitment and therefore fails
// closed instead of creating a transient overcommit window.
func (s *TranscodeService) observeJobStorageReservation(job *TranscodeJob, now time.Time, force bool) {
	if s == nil || s.executionRepo == nil || job == nil || job.ExecutionJob == nil || job.leaseToken == "" {
		return
	}
	attemptID, workspace := currentStorageObservationTarget(job)
	if attemptID == "" || workspace == "" {
		return
	}
	state := storageObservationState(job)
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.attemptID != attemptID {
		state.attemptID = attemptID
		state.lastSampledAt = time.Time{}
		state.observedBytes = 0
	}
	if !force && !state.lastSampledAt.IsZero() && now.Sub(state.lastSampledAt) < transcodeStorageObservationInterval {
		return
	}

	observedBytes, err := transcodeDirectorySize(workspace)
	if err != nil {
		if s.logger != nil {
			s.logger.Debugf("读取转码 Workspace 占用失败 attempt=%s path=%s: %v", attemptID, workspace, err)
		}
		state.lastSampledAt = now
		return
	}
	if !force && observedBytes <= state.observedBytes {
		state.lastSampledAt = now
		return
	}

	s.InvalidateCacheDiskUsage()
	pressure := s.runDiskPressureGovernorTick(now, true)
	if diskPressureSampleUnavailable(pressure) {
		if s.logger != nil {
			s.logger.Warnf("磁盘样本不可用，保留完整转码 Reservation job=%s attempt=%s", job.ExecutionJob.ID, attemptID)
		}
		state.lastSampledAt = now
		return
	}
	updated, err := s.executionRepo.ObserveOwnedJobStorageReservation(
		job.ExecutionJob.ID,
		attemptID,
		job.leaseToken,
		observedBytes,
		now,
	)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("持久化转码 Reservation 实际占用失败 job=%s attempt=%s: %v", job.ExecutionJob.ID, attemptID, err)
		}
		state.lastSampledAt = now
		return
	}
	state.lastSampledAt = now
	if updated {
		state.observedBytes = observedBytes
		if s.logger != nil {
			s.logger.Debugf("已回补转码 Reservation job=%s attempt=%s observed=%d", job.ExecutionJob.ID, attemptID, observedBytes)
		}
	}
}

func diskPressureSampleUnavailable(status TranscodeDiskPressureStatus) bool {
	for _, reason := range status.Reasons {
		if reason == "disk_sample_unavailable" {
			return true
		}
	}
	return status.SampledAt.IsZero()
}

func transcodeDirectorySize(root string) (int64, error) {
	if root == "" {
		return 0, fmt.Errorf("workspace path is empty")
	}
	var total int64
	err := filepath.WalkDir(root, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func (s *TranscodeService) releaseJobStorageReservation(job *TranscodeJob, outcome string, releasedAt time.Time) {
	defer forgetStorageObservation(job)
	if s == nil || s.executionRepo == nil || job == nil || job.ExecutionJob == nil {
		return
	}
	if err := s.executionRepo.ReleaseJobStorageReservation(job.ExecutionJob.ID, outcome, releasedAt); err != nil && s.logger != nil {
		s.logger.Warnf("释放转码 Reservation 审计失败 job=%s outcome=%s: %v", job.ExecutionJob.ID, outcome, err)
	}
}

func (s *TranscodeService) finalizePublishedStorageReservation(job *TranscodeJob, completedAt time.Time) {
	defer forgetStorageObservation(job)
	if s == nil || s.executionRepo == nil || job == nil || job.ExecutionJob == nil || job.CurrentAttempt == nil {
		return
	}
	if err := s.executionRepo.FinalizePublishedJobStorageReservation(
		job.ExecutionJob.ID,
		job.CurrentAttempt.ID,
		completedAt,
	); err != nil && s.logger != nil {
		s.logger.Warnf("写入转码 Reservation 发布校准失败 job=%s attempt=%s: %v", job.ExecutionJob.ID, job.CurrentAttempt.ID, err)
	}
}
