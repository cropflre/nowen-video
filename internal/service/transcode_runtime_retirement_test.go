package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

func TestRetiredRuntimeArtifactUsesCleanupTombstone(t *testing.T) {
	service, db := newArtifactMaintenanceTestService(t)
	now := time.Now()
	path := filepath.Join(service.artifactStore.Root(), "artifacts", "media", "720p", "runtime-old")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "seg.ts"), []byte("segment"), 0o644); err != nil {
		t.Fatal(err)
	}
	job := model.TranscodeJobRecord{ID: "runtime-old", MediaID: "media", Intent: retiredRuntimePlaybackIntents[0], Status: "completed", DesiredState: "cancelled", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	artifact := model.TranscodeArtifactRecord{ID: "artifact-old", JobID: job.ID, MediaID: job.MediaID, Kind: "hls_variant", ProfileID: "720p", Path: path, Status: "published", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	report, err := service.retirePersistentRuntimePlayback(now)
	if err != nil {
		t.Fatal(err)
	}
	if report.ArtifactsDeleted != 1 {
		t.Fatalf("report=%+v", report)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("artifact path still exists: %v", err)
	}
	var stored model.TranscodeArtifactRecord
	if err := db.First(&stored, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CleanupState != repository.ArtifactCleanupCompleted || stored.Path != "" || stored.CleanupOriginalPath != path {
		t.Fatalf("unexpected tombstone: %+v", stored)
	}
}

func TestRuntimeRetirementPathFenceProtectsRoots(t *testing.T) {
	root := t.TempDir()
	if runtimeRetirementPathAllowed(root, root) {
		t.Fatal("root must be protected")
	}
	if runtimeRetirementPathAllowed(root, filepath.Join(root, "artifacts")) {
		t.Fatal("artifact namespace root must be protected")
	}
	if !runtimeRetirementPathAllowed(root, filepath.Join(root, "workspaces", "job", "attempt")) {
		t.Fatal("attempt workspace must be removable")
	}
}
