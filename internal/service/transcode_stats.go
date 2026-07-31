package service

import (
	"os"
	"path/filepath"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodegovernor "github.com/nowen-video/nowen-video/internal/transcode/governor"
)

type TranscodeStatistics struct {
	StatusCounts         map[string]int64               `json:"status_counts"`
	RunningCount         int                            `json:"running_count"`
	ActiveWorkers        int                            `json:"active_workers"`
	MaxWorkers           int                            `json:"max_workers"`
	QueueDepth           int                            `json:"queue_depth"`
	DurableQueueDepth    int64                          `json:"durable_queue_depth"`
	Scheduler            string                         `json:"scheduler"`
	QueuePollMS          int64                          `json:"queue_poll_ms"`
	LeaseDurationSeconds int64                          `json:"lease_duration_seconds"`
	HWAccel              string                         `json:"hw_accel"`
	DiskUsageBytes       int64                          `json:"disk_usage_bytes"`
	DiskUsageDir         string                         `json:"disk_usage_dir"`
	ResourceCapacity     map[transcodegovernor.Kind]int `json:"resource_capacity,omitempty"`
	ResourceInUse        map[transcodegovernor.Kind]int `json:"resource_in_use,omitempty"`
}

func (s *TranscodeService) ListTasks(page, pageSize int, status string) ([]model.TranscodeTask, int64, error) {
	tasks, total, err := s.repo.ListAll(page, pageSize, status)
	if err != nil {
		return tasks, total, err
	}
	for i := range tasks {
		if tasks[i].Media.ID != "" {
			tasks[i].MediaTitle = tasks[i].Media.DescriptiveTitle()
		}
	}
	return tasks, total, nil
}

func (s *TranscodeService) GetStatistics() TranscodeStatistics {
	counts, _ := s.repo.CountByStatus()
	if counts == nil {
		counts = map[string]int64{}
	}
	active := 0
	s.mu.RLock()
	for _, job := range s.running {
		if job.currentProcess() != nil {
			active++
		}
	}
	s.mu.RUnlock()
	durableQueueDepth, err := s.executionRepo.CountQueuedJobs()
	if err != nil {
		s.logger.Debugf("读取持久化转码队列深度失败: %v", err)
	}
	snapshot := s.executionRuntime.Snapshot()
	return TranscodeStatistics{
		StatusCounts:         counts,
		RunningCount:         active,
		ActiveWorkers:        active,
		MaxWorkers:           s.workerCount,
		QueueDepth:           s.jobs.Len(),
		DurableQueueDepth:    durableQueueDepth,
		Scheduler:            "database_priority_fifo",
		QueuePollMS:          s.jobs.PollInterval().Milliseconds(),
		LeaseDurationSeconds: int64(s.leaseDuration / time.Second),
		HWAccel:              s.hwAccel,
		DiskUsageBytes:       s.GetCacheDiskUsage(),
		DiskUsageDir:         filepath.Join(s.cfg.Cache.CacheDir, "transcode"),
		ResourceCapacity:     snapshot.Capacity,
		ResourceInUse:        snapshot.InUse,
	}
}

func (s *TranscodeService) GetCacheDiskUsage() int64 {
	ttl := s.diskUsageTTL
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	s.diskUsageMu.RLock()
	if !s.diskUsageAt.IsZero() && time.Since(s.diskUsageAt) < ttl {
		value := s.diskUsageBytes
		s.diskUsageMu.RUnlock()
		return value
	}
	s.diskUsageMu.RUnlock()

	dir := filepath.Join(s.cfg.Cache.CacheDir, "transcode")
	var total int64
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		_ = filepath.Walk(dir, func(_ string, fileInfo os.FileInfo, walkErr error) error {
			if walkErr == nil && fileInfo != nil && !fileInfo.IsDir() {
				total += fileInfo.Size()
			}
			return nil
		})
	}
	s.diskUsageMu.Lock()
	s.diskUsageBytes = total
	s.diskUsageAt = time.Now()
	s.diskUsageMu.Unlock()
	return total
}

func (s *TranscodeService) InvalidateCacheDiskUsage() {
	s.diskUsageMu.Lock()
	s.diskUsageAt = time.Time{}
	s.diskUsageMu.Unlock()
}
