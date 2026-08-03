package service

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

func TestCleanupStaleCacheRetriesBusyArtifactStorage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission semantics differ on Windows")
	}
	service, db := newConcurrentArtifactService(t)
	dir, err := service.artifactStore.PublishedDir("cleanup-media", "720p", "busy-artifact")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "seg0000.ts"), []byte("segment"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-8 * 24 * time.Hour)
	artifact := &model.TranscodeArtifactRecord{
		ID:                "busy-artifact",
		JobID:             "busy-job",
		MediaID:           "cleanup-media",
		Kind:              "hls_variant",
		ProfileID:         "720p",
		SourceFingerprint: "source",
		PlannerVersion:    "planner",
		Path:              dir,
		ManifestPath:      filepath.Join(dir, "stream.m3u8"),
		Status:            "superseded",
		CreatedAt:         old,
		UpdatedAt:         old,
	}
	if err := db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.TranscodeArtifactRecord{}).
		Where("id = ?", artifact.ID).
		Updates(map[string]any{"created_at": old, "updated_at": old}).Error; err != nil {
		t.Fatal(err)
	}

	parent := filepath.Dir(dir)
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(parent, 0o755)

	if _, _, err := service.CleanupStaleCache(30, 7); err != nil {
		t.Fatal(err)
	}
	var retry model.TranscodeArtifactRecord
	if err := db.First(&retry, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if retry.CleanupState != repository.ArtifactCleanupRetryWait || retry.CleanupAttempts != 1 {
		t.Fatalf("cleanup failure did not persist retry evidence: %+v", retry)
	}
	if retry.CleanupErrorCode != "filesystem_permission" && retry.CleanupErrorCode != "filesystem_io" {
		t.Fatalf("unexpected cleanup error classification: %+v", retry)
	}
	if retry.CleanupNextAttemptAt == nil || !retry.CleanupNextAttemptAt.After(retry.UpdatedAt) {
		t.Fatalf("cleanup retry has no future schedule: %+v", retry)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("busy Artifact was removed despite failed cleanup: %v", err)
	}

	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.TranscodeArtifactRecord{}).
		Where("id = ?", artifact.ID).
		Update("cleanup_next_attempt_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.CleanupStaleCache(30, 7); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("recovered cleanup did not remove Artifact directory: %v", err)
	}
	var count int64
	if err := db.Model(&model.TranscodeArtifactRecord{}).Where("id = ?", artifact.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("recovered cleanup did not delete Artifact row: %d", count)
	}
}

func TestArtifactCleanupFailureClassification(t *testing.T) {
	tests := []struct {
		err       error
		code      string
		retryable bool
	}{
		{errors.New("remove: device or resource busy"), "filesystem_busy", true},
		{errors.New("remove: stale file handle"), "mount_unavailable", true},
		{errors.New("remove: input/output error"), "filesystem_io", true},
		{os.ErrPermission, "filesystem_permission", true},
		{errors.New("artifact path escapes store root"), "cleanup_invariant_violation", false},
	}
	for _, test := range tests {
		code, retryable := classifyArtifactCleanupError(test.err)
		if code != test.code || retryable != test.retryable {
			t.Fatalf("classify %q: code=%q retryable=%v", test.err, code, retryable)
		}
	}
}

func TestArtifactCleanupBackoffCapsAtOneDay(t *testing.T) {
	previous := time.Duration(0)
	for attempt := 1; attempt <= 20; attempt++ {
		value := artifactCleanupBackoff(attempt)
		if value < previous {
			t.Fatalf("cleanup backoff regressed at attempt %d: %s < %s", attempt, value, previous)
		}
		if value > 24*time.Hour {
			t.Fatalf("cleanup backoff exceeded cap: %s", value)
		}
		previous = value
	}
	if got := artifactCleanupBackoff(20); got != 24*time.Hour {
		t.Fatalf("cleanup backoff cap = %s", got)
	}
}

func TestCleanupInvariantMessageRemainsOperatorVisible(t *testing.T) {
	code, retryable := classifyArtifactCleanupError(errors.New("artifact store is unavailable"))
	if retryable || code != "cleanup_invariant_violation" || !strings.Contains(code, "invariant") {
		t.Fatalf("invariant cleanup failure was not blocked: code=%q retryable=%v", code, retryable)
	}
}
