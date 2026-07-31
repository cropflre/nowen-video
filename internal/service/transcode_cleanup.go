package service

import (
	"fmt"
	"os"
	"time"
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
	for _, task := range doneStale {
		s.mu.RLock()
		_, active := s.running[task.ID]
		s.mu.RUnlock()
		if active {
			continue
		}
		if err := os.RemoveAll(s.GetOutputDir(task.MediaID, task.Quality)); err == nil {
			dirsCleaned++
		}
		if err := s.repo.DeleteByID(task.ID); err == nil {
			recordsCleaned++
		}
	}

	failedStale, err := s.repo.ListStaleFailed(now.AddDate(0, 0, -failedRetainDays))
	if err != nil {
		return dirsCleaned, recordsCleaned, fmt.Errorf("查询过期失败任务失败: %w", err)
	}
	for _, task := range failedStale {
		if err := os.RemoveAll(s.GetOutputDir(task.MediaID, task.Quality)); err == nil {
			dirsCleaned++
		}
		if err := s.repo.DeleteByID(task.ID); err == nil {
			recordsCleaned++
		}
	}
	if dirsCleaned > 0 {
		s.InvalidateCacheDiskUsage()
	}
	s.logger.Infof("缓存清理完成: 删除 %d 个目录, %d 条 DB 记录", dirsCleaned, recordsCleaned)
	return dirsCleaned, recordsCleaned, nil
}
