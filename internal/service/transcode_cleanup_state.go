package service

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
)

const artifactCleanupLeaseDuration = 2 * time.Minute

var artifactCleanupBackoffSchedule = []time.Duration{
	time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	time.Hour,
	6 * time.Hour,
	12 * time.Hour,
	24 * time.Hour,
}

func artifactCleanupBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	index := attempt - 1
	if index >= len(artifactCleanupBackoffSchedule) {
		index = len(artifactCleanupBackoffSchedule) - 1
	}
	return artifactCleanupBackoffSchedule[index]
}

// classifyArtifactCleanupError separates retryable filesystem and mount errors
// from invariant violations that require operator intervention. Unknown I/O is
// retried with a capped schedule because NAS and network mounts frequently
// recover without a configuration change.
func classifyArtifactCleanupError(err error) (code string, retryable bool) {
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return "", false
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "escapes store root"),
		strings.Contains(message, "artifact store is unavailable"),
		strings.Contains(message, "artifact store root is required"):
		return "cleanup_invariant_violation", false
	case strings.Contains(message, "device or resource busy"),
		strings.Contains(message, "sharing violation"),
		strings.Contains(message, "file is being used"),
		strings.Contains(message, "directory not empty"):
		return "filesystem_busy", true
	case strings.Contains(message, "stale file handle"),
		strings.Contains(message, "transport endpoint is not connected"),
		strings.Contains(message, "network is unreachable"),
		strings.Contains(message, "host is unreachable"),
		strings.Contains(message, "connection timed out"):
		return "mount_unavailable", true
	case strings.Contains(message, "input/output error"),
		strings.Contains(message, "read-only file system"):
		return "filesystem_io", true
	case errors.Is(err, os.ErrPermission):
		return "filesystem_permission", true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return "filesystem_io", true
	}
	return "cleanup_io", true
}

func (s *TranscodeService) cleanupClaimedArtifact(
	artifact *model.TranscodeArtifactRecord,
	token string,
	now time.Time,
) (removedDirs int, deleted bool, cleanupErr error) {
	if artifact == nil {
		return 0, false, nil
	}
	seen := make(map[string]struct{}, 2)
	for _, path := range []string{artifact.TempPath, artifact.Path} {
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		if err := s.artifactStore.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.reportStorageOperationFailure(storageOperationCleanupArtifact, path, err, now)
			return removedDirs, false, s.persistArtifactCleanupFailure(artifact, token, err, now)
		}
		removedDirs++
	}

	deleted, err := s.executionRepo.DeleteArtifactByCleanupClaim(artifact.ID, token)
	if err != nil {
		return removedDirs, false, s.persistArtifactCleanupFailure(
			artifact,
			token,
			fmt.Errorf("delete artifact cleanup metadata: %w", err),
			now,
		)
	}
	if !deleted {
		return removedDirs, false, fmt.Errorf("artifact cleanup ownership lost: %s", artifact.ID)
	}
	return removedDirs, true, nil
}

func (s *TranscodeService) persistArtifactCleanupFailure(
	artifact *model.TranscodeArtifactRecord,
	token string,
	cause error,
	now time.Time,
) error {
	code, retryable := classifyArtifactCleanupError(cause)
	if !retryable {
		blocked, err := s.executionRepo.BlockArtifactCleanup(
			artifact.ID,
			token,
			code,
			cause.Error(),
			now,
		)
		if err != nil {
			return fmt.Errorf("block artifact cleanup after %v: %w", cause, err)
		}
		if !blocked {
			return fmt.Errorf("artifact cleanup ownership lost while blocking: %s", artifact.ID)
		}
		return fmt.Errorf("artifact cleanup blocked (%s): %w", code, cause)
	}

	nextAttemptAt := now.Add(artifactCleanupBackoff(artifact.CleanupAttempts))
	scheduled, err := s.executionRepo.ScheduleArtifactCleanupRetry(
		artifact.ID,
		token,
		code,
		cause.Error(),
		nextAttemptAt,
		now,
	)
	if err != nil {
		return fmt.Errorf("schedule artifact cleanup retry after %v: %w", cause, err)
	}
	if !scheduled {
		return fmt.Errorf("artifact cleanup ownership lost while scheduling retry: %s", artifact.ID)
	}
	return fmt.Errorf("artifact cleanup retry scheduled at %s (%s): %w", nextAttemptAt.Format(time.RFC3339), code, cause)
}

func (s *TranscodeService) cleanupTerminalArtifactBatch(cutoff, now time.Time) (int, int, error) {
	dirsCleaned := 0
	recordsCleaned := 0
	for {
		artifacts, err := s.executionRepo.ListArtifactsEligibleForCleanup(cutoff, now, 500)
		if err != nil {
			return dirsCleaned, recordsCleaned, err
		}
		if len(artifacts) == 0 {
			break
		}
		claimedCount := 0
		for i := range artifacts {
			token := uuid.NewString()
			claimed, ok, claimErr := s.executionRepo.ClaimArtifactCleanup(
				artifacts[i].ID,
				token,
				cutoff,
				now,
				artifactCleanupLeaseDuration,
			)
			if claimErr != nil {
				s.logger.Warnf("认领 Artifact 清理失败 artifact=%s: %v", artifacts[i].ID, claimErr)
				continue
			}
			if !ok {
				continue
			}
			claimedCount++
			removed, deleted, cleanupErr := s.cleanupClaimedArtifact(claimed, token, now)
			dirsCleaned += removed
			if deleted {
				recordsCleaned++
			}
			if cleanupErr != nil {
				s.logger.Warnf("Artifact 清理延期 artifact=%s: %v", artifacts[i].ID, cleanupErr)
			}
		}
		if claimedCount == 0 || len(artifacts) < 500 {
			break
		}
	}
	return dirsCleaned, recordsCleaned, nil
}
