package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
)

func (s *ArtifactMaintenanceService) CleanupStaleCache(doneRetainDays, failedRetainDays int) (int, int, error) {
	_ = doneRetainDays
	if failedRetainDays <= 0 {
		failedRetainDays = 7
	}
	now := time.Now()
	terminalCutoff := now.AddDate(0, 0, -failedRetainDays)
	dirsCleaned, recordsCompleted, err := s.cleanupTerminalArtifactBatch(terminalCutoff, now)
	if err != nil {
		return dirsCleaned, recordsCompleted, fmt.Errorf("清理过期终态 Artifact 失败: %w", err)
	}
	if dirsCleaned > 0 {
		s.InvalidateCacheDiskUsage()
	}
	s.logger.Infof("Artifact 缓存清理完成: 删除 %d 个目录, 完成 %d 条清理墓碑", dirsCleaned, recordsCompleted)
	return dirsCleaned, recordsCompleted, nil
}

func (s *ArtifactMaintenanceService) cleanupArtifactRecord(artifact *model.TranscodeArtifactRecord) (int, error) {
	if artifact == nil {
		return 0, nil
	}
	now := time.Now()
	if err := s.executionRepo.QueueArtifactCleanup(artifact.ID, now); err != nil {
		return 0, err
	}
	token := uuid.NewString()
	claimed, ok, err := s.executionRepo.ClaimArtifactCleanup(
		artifact.ID,
		token,
		now,
		now,
		artifactCleanupLeaseDuration,
	)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("artifact cleanup is deferred or owned elsewhere: %s", artifact.ID)
	}
	removed, deleted, cleanupErr := s.cleanupClaimedArtifact(claimed, token, now)
	if cleanupErr != nil {
		return removed, cleanupErr
	}
	if !deleted {
		return 0, fmt.Errorf("artifact cleanup did not complete tombstone: %s", artifact.ID)
	}
	return removed, nil
}
