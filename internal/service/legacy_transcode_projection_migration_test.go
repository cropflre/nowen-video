package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

func TestLegacyProjectionInventorySupportsRollback(t *testing.T) {
	service, db := newArtifactMaintenanceTestService(t)
	now := time.Now()
	path := filepath.Join(service.artifactStore.Root(), "media-legacy", "720p")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "stream.m3u8"), []byte("#EXTM3U"), 0o644); err != nil {
		t.Fatal(err)
	}
	task := model.TranscodeTask{ID: "legacy-task", MediaID: "media-legacy", Status: "done", Quality: "720p", OutputDir: path, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	report, err := service.inventoryLegacyTranscodeProjection(now)
	if err != nil {
		t.Fatal(err)
	}
	if report.ArtifactsQueued != 1 {
		t.Fatalf("report=%+v", report)
	}
	artifactID := deterministicLegacyProjectionID("artifact", task.ID)
	var artifact model.TranscodeArtifactRecord
	if err := db.First(&artifact, "id = ?", artifactID).Error; err != nil {
		t.Fatal(err)
	}
	if artifact.CleanupState != repository.ArtifactCleanupPending || artifact.CleanupRollbackUntil == nil {
		t.Fatalf("artifact=%+v", artifact)
	}
	if err := service.RollbackLegacyArtifactMigration(artifact.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&artifact, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if artifact.CleanupState != repository.ArtifactCleanupRollbackCompleted || artifact.Path != path {
		t.Fatalf("rollback=%+v", artifact)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("rollback removed directory: %v", err)
	}
}

func TestLegacyProjectionCleanupPreservesTombstone(t *testing.T) {
	service, db := newArtifactMaintenanceTestService(t)
	now := time.Now()
	path := filepath.Join(service.artifactStore.Root(), "media-expired", "480p")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "seg.ts"), []byte("segment"), 0o644); err != nil {
		t.Fatal(err)
	}
	task := model.TranscodeTask{ID: "legacy-expired", MediaID: "media-expired", Status: "failed", Quality: "480p", OutputDir: path, CreatedAt: now.Add(-30 * 24 * time.Hour), UpdatedAt: now.Add(-30 * 24 * time.Hour)}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	inventoryAt := now.Add(-8 * 24 * time.Hour)
	if _, err := service.inventoryLegacyTranscodeProjection(inventoryAt); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.cleanupTerminalArtifactBatch(now.Add(-24*time.Hour), now); err != nil {
		t.Fatal(err)
	}
	artifactID := deterministicLegacyProjectionID("artifact", task.ID)
	var artifact model.TranscodeArtifactRecord
	if err := db.First(&artifact, "id = ?", artifactID).Error; err != nil {
		t.Fatal(err)
	}
	if artifact.CleanupState != repository.ArtifactCleanupCompleted || artifact.CleanupOriginalPath != path || artifact.Path != "" {
		t.Fatalf("tombstone=%+v", artifact)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("legacy path remains: %v", err)
	}
	var legacy model.TranscodeTask
	if err := db.First(&legacy, "id = ?", task.ID).Error; err != nil {
		t.Fatalf("legacy row was mutated/deleted: %v", err)
	}
	if legacy.OutputDir != path || legacy.Status != "failed" {
		t.Fatalf("legacy row changed: %+v", legacy)
	}
}
