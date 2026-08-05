package diskpressure

import (
	"math"
	"sort"
	"time"
)

type Level string

const (
	LevelNormal   Level = "normal"
	LevelPressure Level = "pressure"
	LevelCritical Level = "critical"
)

const (
	ReasonFilesystemHigh = "filesystem_high_watermark"
	ReasonMinimumFree    = "minimum_free_space"
	ReasonStoreLimit     = "artifact_store_limit"
	ReasonCriticalFree   = "critical_free_space"
	ReasonHysteresis     = "recovery_hysteresis"
)

type Config struct {
	HighWatermarkPct  float64
	LowWatermarkPct   float64
	MinFreeBytes      uint64
	CriticalFreeBytes uint64
	MaxStoreBytes     uint64
}

func DefaultConfig() Config {
	return Config{
		HighWatermarkPct:  90,
		LowWatermarkPct:   80,
		MinFreeBytes:      2 * 1024 * 1024 * 1024,
		CriticalFreeBytes: 512 * 1024 * 1024,
	}
}

func (c Config) Normalized() Config {
	defaults := DefaultConfig()
	if c.HighWatermarkPct <= 0 || c.HighWatermarkPct > 100 {
		c.HighWatermarkPct = defaults.HighWatermarkPct
	}
	if c.LowWatermarkPct <= 0 || c.LowWatermarkPct >= c.HighWatermarkPct {
		c.LowWatermarkPct = defaults.LowWatermarkPct
	}
	if c.MinFreeBytes == 0 {
		c.MinFreeBytes = defaults.MinFreeBytes
	}
	if c.CriticalFreeBytes == 0 || c.CriticalFreeBytes >= c.MinFreeBytes {
		c.CriticalFreeBytes = defaults.CriticalFreeBytes
		if c.CriticalFreeBytes >= c.MinFreeBytes {
			c.CriticalFreeBytes = c.MinFreeBytes / 4
		}
	}
	return c
}

type Sample struct {
	TotalBytes uint64    `json:"total_bytes"`
	FreeBytes  uint64    `json:"free_bytes"`
	UsedBytes  uint64    `json:"used_bytes"`
	StoreBytes uint64    `json:"store_bytes"`
	SampledAt  time.Time `json:"sampled_at"`
}

type Snapshot struct {
	Level              Level     `json:"level"`
	Reasons            []string  `json:"reasons"`
	TotalBytes         uint64    `json:"total_bytes"`
	FreeBytes          uint64    `json:"free_bytes"`
	UsedBytes          uint64    `json:"used_bytes"`
	UsedPercent        float64   `json:"used_percent"`
	StoreBytes         uint64    `json:"store_bytes"`
	MaxStoreBytes      uint64    `json:"max_store_bytes,omitempty"`
	HighWatermarkPct   float64   `json:"high_watermark_percent"`
	LowWatermarkPct    float64   `json:"low_watermark_percent"`
	MinFreeBytes       uint64    `json:"min_free_bytes"`
	CriticalFreeBytes  uint64    `json:"critical_free_bytes"`
	ReclaimTargetBytes uint64    `json:"reclaim_target_bytes"`
	AdmissionBlocked   bool      `json:"admission_blocked"`
	QueuePaused        bool      `json:"queue_paused"`
	SampledAt          time.Time `json:"sampled_at"`
}

func Evaluate(previous Level, sample Sample, cfg Config) Snapshot {
	cfg = cfg.Normalized()
	used := sample.UsedBytes
	if used == 0 && sample.TotalBytes >= sample.FreeBytes {
		used = sample.TotalBytes - sample.FreeBytes
	}
	usedPct := 0.0
	if sample.TotalBytes > 0 {
		usedPct = float64(used) / float64(sample.TotalBytes) * 100
	}

	reasons := make([]string, 0, 4)
	if usedPct >= cfg.HighWatermarkPct {
		reasons = append(reasons, ReasonFilesystemHigh)
	}
	if sample.FreeBytes <= cfg.MinFreeBytes {
		reasons = append(reasons, ReasonMinimumFree)
	}
	if cfg.MaxStoreBytes > 0 && sample.StoreBytes >= cfg.MaxStoreBytes {
		reasons = append(reasons, ReasonStoreLimit)
	}

	critical := sample.FreeBytes <= cfg.CriticalFreeBytes || usedPct >= 97
	if critical {
		reasons = append(reasons, ReasonCriticalFree)
	}

	level := LevelNormal
	if critical {
		level = LevelCritical
	} else if len(reasons) > 0 {
		level = LevelPressure
	} else if previous == LevelPressure || previous == LevelCritical {
		recovered := usedPct <= cfg.LowWatermarkPct &&
			sample.FreeBytes >= recoveryFreeBytes(cfg.MinFreeBytes) &&
			(cfg.MaxStoreBytes == 0 || sample.StoreBytes <= uint64(float64(cfg.MaxStoreBytes)*0.85))
		if !recovered {
			level = LevelPressure
			reasons = append(reasons, ReasonHysteresis)
		}
	}

	sort.Strings(reasons)
	return Snapshot{
		Level:              level,
		Reasons:            reasons,
		TotalBytes:         sample.TotalBytes,
		FreeBytes:          sample.FreeBytes,
		UsedBytes:          used,
		UsedPercent:        usedPct,
		StoreBytes:         sample.StoreBytes,
		MaxStoreBytes:      cfg.MaxStoreBytes,
		HighWatermarkPct:   cfg.HighWatermarkPct,
		LowWatermarkPct:    cfg.LowWatermarkPct,
		MinFreeBytes:       cfg.MinFreeBytes,
		CriticalFreeBytes:  cfg.CriticalFreeBytes,
		ReclaimTargetBytes: reclaimTarget(sample, used, cfg),
		AdmissionBlocked:   level != LevelNormal,
		QueuePaused:        level != LevelNormal,
		SampledAt:          sample.SampledAt,
	}
}

func reclaimTarget(sample Sample, used uint64, cfg Config) uint64 {
	var target uint64
	if sample.TotalBytes > 0 {
		lowUsed := uint64(math.Floor(float64(sample.TotalBytes) * cfg.LowWatermarkPct / 100))
		if used > lowUsed {
			target = used - lowUsed
		}
	}
	freeGoal := recoveryFreeBytes(cfg.MinFreeBytes)
	if sample.FreeBytes < freeGoal && freeGoal-sample.FreeBytes > target {
		target = freeGoal - sample.FreeBytes
	}
	if cfg.MaxStoreBytes > 0 {
		storeGoal := uint64(float64(cfg.MaxStoreBytes) * 0.85)
		if sample.StoreBytes > storeGoal && sample.StoreBytes-storeGoal > target {
			target = sample.StoreBytes - storeGoal
		}
	}
	return target
}

func recoveryFreeBytes(minFree uint64) uint64 {
	return minFree + minFree/4
}
