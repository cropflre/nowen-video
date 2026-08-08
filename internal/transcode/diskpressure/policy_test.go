package diskpressure

import (
	"testing"
	"time"
)

func TestEvaluateNormalPressureCriticalAndRecovery(t *testing.T) {
	cfg := Config{
		HighWatermarkPct:  90,
		LowWatermarkPct:   80,
		MinFreeBytes:      200,
		CriticalFreeBytes: 50,
		MaxStoreBytes:     500,
	}
	now := time.Now()

	normal := Evaluate(LevelNormal, Sample{TotalBytes: 1000, FreeBytes: 300, StoreBytes: 300, SampledAt: now}, cfg)
	if normal.Level != LevelNormal || normal.AdmissionBlocked || normal.QueuePaused {
		t.Fatalf("unexpected normal snapshot: %+v", normal)
	}

	pressure := Evaluate(LevelNormal, Sample{TotalBytes: 1000, FreeBytes: 100, StoreBytes: 520, SampledAt: now}, cfg)
	if pressure.Level != LevelPressure || !pressure.AdmissionBlocked || pressure.ReclaimTargetBytes == 0 {
		t.Fatalf("unexpected pressure snapshot: %+v", pressure)
	}

	critical := Evaluate(LevelPressure, Sample{TotalBytes: 1000, FreeBytes: 40, StoreBytes: 520, SampledAt: now}, cfg)
	if critical.Level != LevelCritical {
		t.Fatalf("expected critical pressure, got %+v", critical)
	}

	hysteresis := Evaluate(LevelPressure, Sample{TotalBytes: 1000, FreeBytes: 230, StoreBytes: 400, SampledAt: now}, cfg)
	if hysteresis.Level != LevelPressure {
		t.Fatalf("pressure must remain until low-watermark recovery: %+v", hysteresis)
	}

	recovered := Evaluate(LevelPressure, Sample{TotalBytes: 1000, FreeBytes: 300, StoreBytes: 400, SampledAt: now}, cfg)
	if recovered.Level != LevelNormal || recovered.AdmissionBlocked {
		t.Fatalf("expected recovered state: %+v", recovered)
	}
}

func TestEvaluateUsesStoreLimitWithoutFilesystemPressure(t *testing.T) {
	cfg := Config{
		HighWatermarkPct:  90,
		LowWatermarkPct:   80,
		MinFreeBytes:      100,
		CriticalFreeBytes: 25,
		MaxStoreBytes:     400,
	}
	snapshot := Evaluate(LevelNormal, Sample{
		TotalBytes: 1000,
		FreeBytes:  500,
		StoreBytes: 450,
		SampledAt:  time.Now(),
	}, cfg)
	if snapshot.Level != LevelPressure {
		t.Fatalf("store limit must trigger pressure: %+v", snapshot)
	}
	if snapshot.ReclaimTargetBytes != 110 {
		t.Fatalf("reclaim target = %d, want 110", snapshot.ReclaimTargetBytes)
	}
}
