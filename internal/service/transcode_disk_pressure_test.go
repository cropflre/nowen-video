package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodediskpressure "github.com/nowen-video/nowen-video/internal/transcode/diskpressure"
	"gorm.io/gorm"
)

func TestDiskPressureGovernorReclaimsOldPublishedArtifact(t *testing.T) {
	service, db := newArtifactMaintenanceTestService(t)
	service.cfg.Cache.MaxDiskUsageMB = 1
	service.diskUsageTTL = time.Nanosecond
	artifact, path := createDiskPressureArtifact(t, service, db, "pressure-old", time.Now().Add(-48*time.Hour), 2*1024*1024)

	status := service.runDiskPressureGovernorTick(time.Now(), true)
	if status.LastReclaimedRows == 0 || status.LastReclaimedBytes < artifact.SizeBytes {
		t.Fatalf("pressure reclaim evidence missing: %+v", status)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("artifact directory survived pressure reclaim: %v", err)
	}
	var count int64
	if err := db.Model(&model.TranscodeArtifactRecord{}).Where("id = ?", artifact.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("artifact metadata survived pressure cleanup: %d", count)
	}
}

func TestDiskPressureGovernorProtectsRecentArtifact(t *testing.T) {
	service, db := newArtifactMaintenanceTestService(t)
	service.cfg.Cache.MaxDiskUsageMB = 1
	service.diskUsageTTL = time.Nanosecond
	artifact, path := createDiskPressureArtifact(t, service, db, "pressure-active", time.Now(), 2*1024*1024)

	status := service.runDiskPressureGovernorTick(time.Now(), true)
	if status.Level == transcodediskpressure.LevelNormal || !status.AdmissionBlocked || !status.QueuePaused {
		t.Fatalf("recent artifact should keep pressure visible: %+v", status)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("recently accessed artifact was removed: %v", err)
	}
	var stored model.TranscodeArtifactRecord
	if err := db.First(&stored, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "published" || stored.CleanupState != "" {
		t.Fatalf("recent artifact entered cleanup: %+v", stored)
	}
}

func createDiskPressureArtifact(
	t *testing.T,
	service *ArtifactMaintenanceService,
	db *gorm.DB,
	id string,
	updatedAt time.Time,
	size int,
) (*model.TranscodeArtifactRecord, string) {
	t.Helper()
	path, err := service.artifactStore.PublishedDir("pressure-media", "720p", id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, size)
	if err := os.WriteFile(filepath.Join(path, "seg00001.ts"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	publishedAt := time.Now().Add(-48 * time.Hour)
	artifact := &model.TranscodeArtifactRecord{
		ID:                "artifact-" + id,
		JobID:             "job-" + id,
		MediaID:           "pressure-media",
		Kind:              "hls_variant",
		ProfileID:         "720p",
		SourceFingerprint: "source",
		PlannerVersion:    "planner",
		Status:            "published",
		Path:              path,
		SizeBytes:         int64(size),
		PublishedAt:       &publishedAt,
		CreatedAt:         publishedAt,
		UpdatedAt:         updatedAt,
	}
	if err := db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	return artifact, path
}
