package service

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
)

func (s *TranscodeService) CleanupStaleCache(doneRetainDays, failedRetainDays int) (int, int, error) {
	if doneRetainDays <= 0 {
		doneRetainDays = 30
	}
	if failedRetainDays <= 0 {
		failedRetainDays = 7
	}

	now := time.Now()
	dirsCleaned := 0
	recordsCleaned := 0

	doneStale, err := s.repo.ListStaleDone(now.AddDate(0, 0, -doneRetainDays))
	if err != nil {
		return 0, 0, fmt.Errorf("查询过期完成任务失败: %w", err)
	}
	for i := range doneStale {
		task := &doneStale[i]
		s.mu.RLock()
		_, active := s.running[task.ID]
		s.mu.RUnlock()
		if active {
			continue
		}
		removedDirs, removedArtifacts, cleanupErr := s.cleanupArtifactsForTask(task)
		if cleanupErr != nil {
			s.logger.Warnf("清理完成任务 Artifact 延期 task=%s: %v", task.ID, cleanupErr)
			continue
		}
		dirsCleaned += removedDirs
		recordsCleaned += removedArtifacts
		if err := s.repo.DeleteByID(task.ID); err == nil {
			recordsCleaned++
		}
	}

	failedStale, err := s.repo.ListStaleFailed(now.AddDate(0, 0, -failedRetainDays))
	if err != nil {
		return dirsCleaned, recordsCleaned, fmt.Errorf("查询过期失败任务失败: %w", err)
	}
	for i := range failedStale {
		task := &failedStale[i]
		removedDirs, removedArtifacts, cleanupErr := s.cleanupArtifactsForTask(task)
		if cleanupErr != nil {
			s.logger.Warnf("清理失败任务 Artifact 延期 task=%s: %v", task.ID, cleanupErr)
			continue
		}
		dirsCleaned += removedDirs
		recordsCleaned += removedArtifacts
		if err := s.repo.DeleteByID(task.ID); err == nil {
			recordsCleaned++
		}
	}

	// Attempt terminal evidence remains in the database, while terminal Artifact
	// files are storage cache. Every deletion is protected by a durable cleanup
	// Lease so concurrent servers cannot remove the same version twice. Failed
	// NAS or network-mount operations persist their own retry schedule.
	terminalCutoff := now.AddDate(0, 0, -failedRetainDays)
	terminalDirs, terminalRecords, terminalErr := s.cleanupTerminalArtifactBatch(terminalCutoff, now)
	if terminalErr != nil {
		return dirsCleaned, recordsCleaned, fmt.Errorf("清理过期终态 Artifact 失败: %w", terminalErr)
	}
	dirsCleaned += terminalDirs
	recordsCleaned += terminalRecords

	if dirsCleaned > 0 {
		s.InvalidateCacheDiskUsage()
	}
	s.logger.Infof("Artifact 缓存清理完成: 删除 %d 个目录, %d 条 DB 记录", dirsCleaned, recordsCleaned)
	return dirsCleaned, recordsCleaned, nil
}

func (s *TranscodeService) cleanupArtifactsForTask(task *model.TranscodeTask) (int, int, error) {
	if task == nil {
		return 0, 0, nil
	}
	artifacts, err := s.executionRepo.ListArtifactsByLegacyTaskID(task.ID)
	if err != nil {
		return 0, 0, err
	}
	removedDirs := 0
	removedRecords := 0
	for i := range artifacts {
		removed, removeErr := s.cleanupArtifactRecord(&artifacts[i])
		if removeErr != nil {
			return removedDirs + removed, removedRecords, removeErr
		}
		removedDirs += removed
		removedRecords++
	}
	if len(artifacts) == 0 {
		// Historical task without an imported Artifact: remove only the bounded
		// legacy directory, never resolve and delete a newer Artifact version.
		if err := os.RemoveAll(s.GetLegacyOutputDir(task.MediaID, task.Quality)); err == nil {
			removedDirs++
		}
	}
	return removedDirs, removedRecords, nil
}

func (s *TranscodeService) cleanupArtifactRecord(artifact *model.TranscodeArtifactRecord) (int, error) {
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
		return removed, fmt.Errorf("artifact cleanup did not delete record: %s", artifact.ID)
	}
	return removed, nil
}
