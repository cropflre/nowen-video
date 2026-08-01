package service

import (
	"fmt"
	"os"
	"time"

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
			s.logger.Warnf("清理完成任务 Artifact 失败 task=%s: %v", task.ID, cleanupErr)
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
			s.logger.Warnf("清理失败任务 Artifact 失败 task=%s: %v", task.ID, cleanupErr)
			continue
		}
		dirsCleaned += removedDirs
		recordsCleaned += removedArtifacts
		if err := s.repo.DeleteByID(task.ID); err == nil {
			recordsCleaned++
		}
	}

	// Attempt terminal evidence remains in the database, but terminal Artifact
	// files are storage cache. Remove old failed/cancelled/abandoned/superseded
	// versions even when a historical compatibility task is no longer present.
	terminalCutoff := now.AddDate(0, 0, -failedRetainDays)
	for {
		artifacts, listErr := s.executionRepo.ListTerminalArtifactsBefore(terminalCutoff, 500)
		if listErr != nil {
			return dirsCleaned, recordsCleaned, fmt.Errorf("查询过期终态 Artifact 失败: %w", listErr)
		}
		if len(artifacts) == 0 {
			break
		}
		removedInBatch := 0
		for i := range artifacts {
			removed, removeErr := s.cleanupArtifactRecord(&artifacts[i])
			if removeErr != nil {
				s.logger.Warnf("清理终态 Artifact 失败 artifact=%s: %v", artifacts[i].ID, removeErr)
				continue
			}
			dirsCleaned += removed
			recordsCleaned++
			removedInBatch++
		}
		if removedInBatch == 0 || len(artifacts) < 500 {
			break
		}
	}

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
			return removedDirs, removedRecords, removeErr
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
	removed := 0
	seen := make(map[string]struct{}, 2)
	for _, path := range []string{artifact.TempPath, artifact.Path} {
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		if err := s.artifactStore.Remove(path); err != nil {
			return removed, err
		}
		removed++
	}
	if err := s.executionRepo.DeleteHandoffAttestationsForArtifact(artifact.ID, time.Now()); err != nil {
		return removed, err
	}
	if err := s.executionRepo.DeleteArtifactByID(artifact.ID); err != nil {
		return removed, err
	}
	return removed, nil
}
