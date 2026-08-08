package service

import (
	"os"
	"path/filepath"
	"time"
)

// TranscodeStatistics is retained as the wire shape for existing diagnostics,
// but all execution counters are permanently zero because Runtime workers no
// longer exist. The live fields describe Artifact maintenance only.
type TranscodeStatistics struct {
	StatusCounts               map[string]int64             `json:"status_counts"`
	ArtifactStatusCounts       map[string]int64             `json:"artifact_status_counts"`
	ArtifactCleanupStateCounts map[string]int64             `json:"artifact_cleanup_state_counts"`
	RunningCount               int                          `json:"running_count"`
	ActiveWorkers              int                          `json:"active_workers"`
	MaxWorkers                 int                          `json:"max_workers"`
	QueueDepth                 int                          `json:"queue_depth"`
	DurableQueueDepth          int64                        `json:"durable_queue_depth"`
	Scheduler                  string                       `json:"scheduler"`
	QueuePollMS                int64                        `json:"queue_poll_ms"`
	LeaseDurationSeconds       int64                        `json:"lease_duration_seconds"`
	HWAccel                    string                       `json:"hw_accel"`
	DiskUsageBytes             int64                        `json:"disk_usage_bytes"`
	DiskUsageDir               string                       `json:"disk_usage_dir"`
	ArtifactStoreRoot          string                       `json:"artifact_store_root"`
	StorageHealth              TranscodeStorageHealthStatus `json:"storage_health"`
	DiskPressure               TranscodeDiskPressureStatus  `json:"disk_pressure"`
}

func (s *ArtifactMaintenanceService) GetStatistics() TranscodeStatistics {
	counts := map[string]int64{}
	artifactCounts, _ := s.executionRepo.ArtifactStatusCounts()
	if artifactCounts == nil {
		artifactCounts = map[string]int64{}
	}
	cleanupCounts, _ := s.executionRepo.ArtifactCleanupStateCounts()
	if cleanupCounts == nil {
		cleanupCounts = map[string]int64{}
	}
	artifactRoot := ""
	if s.artifactStore != nil {
		artifactRoot = s.artifactStore.Root()
	}
	return TranscodeStatistics{
		StatusCounts:               counts,
		ArtifactStatusCounts:       artifactCounts,
		ArtifactCleanupStateCounts: cleanupCounts,
		Scheduler:                  "artifact_maintenance_only",
		HWAccel:                    "none",
		DiskUsageBytes:             s.GetCacheDiskUsage(),
		DiskUsageDir:               filepath.Join(s.cfg.Cache.CacheDir, "transcode"),
		ArtifactStoreRoot:          artifactRoot,
		StorageHealth:              s.GetStorageHealthStatus(),
		DiskPressure:               s.GetDiskPressureStatus(),
	}
}

func (s *ArtifactMaintenanceService) GetCacheDiskUsage() int64 {
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

func (s *ArtifactMaintenanceService) InvalidateCacheDiskUsage() {
	s.diskUsageMu.Lock()
	s.diskUsageAt = time.Time{}
	s.diskUsageMu.Unlock()
}
