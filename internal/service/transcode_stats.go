package service

import (
	"os"
	"path/filepath"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodegovernor "github.com/nowen-video/nowen-video/internal/transcode/governor"
	transcodeprobe "github.com/nowen-video/nowen-video/internal/transcode/probe"
)

type TranscodeStatistics struct {
	StatusCounts               map[string]int64                  `json:"status_counts"`
	ArtifactStatusCounts       map[string]int64                  `json:"artifact_status_counts"`
	ArtifactCleanupStateCounts map[string]int64                  `json:"artifact_cleanup_state_counts"`
	RunningCount               int                               `json:"running_count"`
	ActiveWorkers              int                               `json:"active_workers"`
	MaxWorkers                 int                               `json:"max_workers"`
	QueueDepth                 int                               `json:"queue_depth"`
	DurableQueueDepth          int64                             `json:"durable_queue_depth"`
	Scheduler                  string                            `json:"scheduler"`
	QueuePollMS                int64                             `json:"queue_poll_ms"`
	LeaseDurationSeconds       int64                             `json:"lease_duration_seconds"`
	HWAccel                    string                            `json:"hw_accel"`
	MediaProbe                 transcodeprobe.Stats              `json:"media_probe"`
	ProbeWarmup                MediaProbeWarmupStats             `json:"probe_warmup"`
	DiskUsageBytes             int64                             `json:"disk_usage_bytes"`
	DiskUsageDir               string                            `json:"disk_usage_dir"`
	ArtifactStoreRoot          string                            `json:"artifact_store_root"`
	DiskPressure               TranscodeDiskPressureStatus       `json:"disk_pressure"`
	StorageReservation         TranscodeStorageReservationStatus `json:"storage_reservation"`
	ResourceCapacity           map[transcodegovernor.Kind]int    `json:"resource_capacity,omitempty"`
	ResourceInUse              map[transcodegovernor.Kind]int    `json:"resource_in_use,omitempty"`
	ResourceWaiting            map[transcodegovernor.Kind]int    `json:"resource_waiting,omitempty"`
	ResourcePeakInUse          map[transcodegovernor.Kind]int    `json:"resource_peak_in_use,omitempty"`
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
	artifactCounts, artifactErr := s.executionRepo.ArtifactStatusCounts()
	if artifactErr != nil {
		s.logger.Debugf("读取转码 Artifact 状态统计失败: %v", artifactErr)
	}
	if artifactCounts == nil {
		artifactCounts = map[string]int64{}
	}
	cleanupCounts, cleanupErr := s.executionRepo.ArtifactCleanupStateCounts()
	if cleanupErr != nil {
		s.logger.Debugf("读取转码 Artifact 清理状态统计失败: %v", cleanupErr)
	}
	if cleanupCounts == nil {
		cleanupCounts = map[string]int64{}
	}
	active := 0
	s.mu.RLock()
	for _, job := range s.running {
		if job.currentProcess() != nil {
			active++
		}
	s.mu.RUnlock()
	durableQueueDepth, err := s.executionRepo.CountQueuedJobs()
	if err != nil {
		s.logger.Debugf("读取持久化转码队列深度失败: %v", err)
	}
	snapshot := s.executionRuntime.Snapshot()
	artifactRoot := ""
	if s.artifactStore != nil {
		artifactRoot = s.artifactStore.Root()
	}
	pressure := s.GetDiskPressureStatus()
	reservation := s.GetStorageReservationStatus()
	return TranscodeStatistics{
		StatusCounts:               counts,
		ArtifactStatusCounts:       artifactCounts,
		ArtifactCleanupStateCounts: cleanupCounts,
		RunningCount:               active,
		ActiveWorkers:              active,
		MaxWorkers:                 s.workerCount,
		QueueDepth:                 s.jobs.Len(),
		DurableQueueDepth:          durableQueueDepth,
		Scheduler:                  "database_priority_fifo_storage_reserved",
		QueuePollMS:                s.jobs.PollInterval().Milliseconds(),
		LeaseDurationSeconds:       int64(s.leaseDuration / time.Second),
		HWAccel:                    s.hwAccel,
		MediaProbe:                 s.GetMediaProbeStats(),
		ProbeWarmup:                s.GetMediaProbeWarmupStats(),
		DiskUsageBytes:             s.GetCacheDiskUsage(),
		DiskUsageDir:               filepath.Join(s.cfg.Cache.CacheDir, "transcode"),
		ArtifactStoreRoot:          artifactRoot,
		DiskPressure:               pressure,
		StorageReservation:         reservation,
		ResourceCapacity:           snapshot.Capacity,
		ResourceInUse:              snapshot.InUse,
		ResourceWaiting:            snapshot.Waiting,
		ResourcePeakInUse:          snapshot.PeakInUse,
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
