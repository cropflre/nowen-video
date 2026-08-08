package repository

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestArtifactPressureReclaimHonorsRecentAccess(t *testing.T) {
	db := newArtifactCleanupTestDB(t)
	repo := NewTranscodeExecutionRepo(db)
	now := time.Now().UTC().Truncate(time.Second)
	old := now.Add(-48 * time.Hour)
	recent := now.Add(-5 * time.Minute)

	rows := []model.TranscodeArtifactRecord{
		{
			ID: "terminal-old", JobID: "job-terminal", MediaID: "media-terminal",
			Kind: "hls_variant", ProfileID: "720p", Status: "superseded",
			SizeBytes: 100, CreatedAt: old, UpdatedAt: old,
		},
		{
			ID: "terminal-active", JobID: "job-active", MediaID: "media-active",
			Kind: "hls_variant", ProfileID: "720p", Status: "superseded",
			SizeBytes: 200, CreatedAt: old, UpdatedAt: recent,
		},
		{
			ID: "published-old", JobID: "job-published", MediaID: "media-published",
			Kind: "hls_variant", ProfileID: "720p", Status: "published",
			SizeBytes: 300, PublishedAt: &old, CreatedAt: old, UpdatedAt: old,
		},
		{
			ID: "published-active", JobID: "job-published-active", MediaID: "media-published-active",
			Kind: "startup_hls", ProfileID: "720p", Status: "published",
			SizeBytes: 400, PublishedAt: &old, CreatedAt: old, UpdatedAt: recent,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	terminal, err := repo.QueueTerminalArtifactsForPressure(now.Add(-15*time.Minute), now, 100)
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Queued != 1 || terminal.Bytes != 100 {
		t.Fatalf("unexpected terminal pressure queue: %+v", terminal)
	}

	published, err := repo.ExpirePublishedArtifactsForPressure(
		now.Add(-15*time.Minute),
		now.Add(-24*time.Hour),
		1,
		now,
		100,
	)
	if err != nil {
		t.Fatal(err)
	}
	if published.Queued != 1 || published.Bytes != 300 {
		t.Fatalf("unexpected published pressure queue: %+v", published)
	}

	var terminalActive model.TranscodeArtifactRecord
	if err := db.First(&terminalActive, "id = ?", "terminal-active").Error; err != nil {
		t.Fatal(err)
	}
	if terminalActive.CleanupState != "" || terminalActive.Status != "superseded" {
		t.Fatalf("recent terminal artifact was reclaimed: %+v", terminalActive)
	}
	var publishedActive model.TranscodeArtifactRecord
	if err := db.First(&publishedActive, "id = ?", "published-active").Error; err != nil {
		t.Fatal(err)
	}
	if publishedActive.CleanupState != "" || publishedActive.Status != "published" {
		t.Fatalf("recent published artifact was reclaimed: %+v", publishedActive)
	}
}

func TestTouchArtifactAccessProtectsOldPublishedArtifact(t *testing.T) {
	db := newArtifactCleanupTestDB(t)
	repo := NewTranscodeExecutionRepo(db)
	now := time.Now().UTC().Truncate(time.Second)
	old := now.Add(-72 * time.Hour)
	artifact := model.TranscodeArtifactRecord{
		ID: "touch-pressure", JobID: "touch-job", MediaID: "touch-media",
		Kind: "hls_variant", ProfileID: "720p", Status: "published",
		SizeBytes: 123, PublishedAt: &old, CreatedAt: old, UpdatedAt: old,
	}
	if err := db.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	updated, err := repo.TouchArtifactAccess(artifact.ID, now, now.Add(-30*time.Second))
	if err != nil || !updated {
		t.Fatalf("touch artifact access: updated=%v err=%v", updated, err)
	}
	queued, err := repo.ExpirePublishedArtifactsForPressure(
		now.Add(-15*time.Minute),
		now.Add(-24*time.Hour),
		1,
		now,
		100,
	)
	if err != nil {
		t.Fatal(err)
	}
	if queued.Queued != 0 {
		t.Fatalf("recently touched artifact was reclaimed: %+v", queued)
	}
}
