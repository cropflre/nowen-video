package repository

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

func TestArtifactCleanupClaimRetryAndRecovery(t *testing.T) {
	db := newArtifactCleanupTestDB(t)
	repo := NewTranscodeExecutionRepo(db)
	now := time.Now().UTC().Truncate(time.Second)
	old := now.Add(-8 * 24 * time.Hour)
	artifact := &model.TranscodeArtifactRecord{
		ID:                  "cleanup-artifact",
		JobID:               "cleanup-job",
		MediaID:             "cleanup-media",
		Kind:                "hls_variant",
		ProfileID:           "720p",
		SourceFingerprint:   "source",
		PlannerVersion:      "planner",
		Status:              "superseded",
		CleanupAttempts:     0,
		CleanupState:        "",
		CleanupErrorCode:    "",
		CleanupErrorMessage: "",
		CreatedAt:           old,
		UpdatedAt:           old,
	}
	if err := db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.TranscodeArtifactRecord{}).
		Where("id = ?", artifact.ID).
		Updates(map[string]any{"created_at": old, "updated_at": old}).Error; err != nil {
		t.Fatal(err)
	}

	eligible, err := repo.ListArtifactsEligibleForCleanup(now.Add(-7*24*time.Hour), now, 10)
	if err != nil || len(eligible) != 1 {
		t.Fatalf("initial cleanup eligibility: len=%d err=%v", len(eligible), err)
	}

	claimed, ok, err := repo.ClaimArtifactCleanup(
		artifact.ID,
		"token-a",
		now.Add(-7*24*time.Hour),
		now,
		time.Minute,
	)
	if err != nil || !ok {
		t.Fatalf("claim cleanup: ok=%v err=%v", ok, err)
	}
	if claimed.CleanupState != ArtifactCleanupClaimed || claimed.CleanupAttempts != 1 || claimed.CleanupToken != "token-a" {
		t.Fatalf("unexpected claimed cleanup record: %+v", claimed)
	}
	if _, ok, err := repo.ClaimArtifactCleanup(
		artifact.ID,
		"token-b",
		now.Add(-7*24*time.Hour),
		now,
		time.Minute,
	); err != nil || ok {
		t.Fatalf("live cleanup lease was not exclusive: ok=%v err=%v", ok, err)
	}

	nextAttempt := now.Add(5 * time.Minute)
	if scheduled, err := repo.ScheduleArtifactCleanupRetry(
		artifact.ID,
		"token-a",
		"filesystem_busy",
		"device or resource busy",
		nextAttempt,
		now,
	); err != nil || !scheduled {
		t.Fatalf("schedule cleanup retry: scheduled=%v err=%v", scheduled, err)
	}
	eligible, err = repo.ListArtifactsEligibleForCleanup(now.Add(-7*24*time.Hour), now.Add(4*time.Minute), 10)
	if err != nil || len(eligible) != 0 {
		t.Fatalf("cleanup retry became eligible too early: len=%d err=%v", len(eligible), err)
	}
	eligible, err = repo.ListArtifactsEligibleForCleanup(now.Add(-7*24*time.Hour), now.Add(6*time.Minute), 10)
	if err != nil || len(eligible) != 1 {
		t.Fatalf("cleanup retry did not become eligible: len=%d err=%v", len(eligible), err)
	}

	claimed, ok, err = repo.ClaimArtifactCleanup(
		artifact.ID,
		"token-b",
		now.Add(-7*24*time.Hour),
		now.Add(6*time.Minute),
		time.Minute,
	)
	if err != nil || !ok {
		t.Fatalf("recover cleanup retry: ok=%v err=%v", ok, err)
	}
	if claimed.CleanupAttempts != 2 || claimed.CleanupToken != "token-b" {
		t.Fatalf("retry claim did not preserve attempt evidence: %+v", claimed)
	}
	if deleted, err := repo.CompleteArtifactCleanupByClaim(artifact.ID, "token-a", "test_cleanup", now.Add(6*time.Minute)); err != nil || deleted {
		t.Fatalf("stale cleanup token deleted artifact: deleted=%v err=%v", deleted, err)
	}
	if deleted, err := repo.CompleteArtifactCleanupByClaim(artifact.ID, "token-b", "test_cleanup", now.Add(6*time.Minute)); err != nil || !deleted {
		t.Fatalf("current cleanup token failed to delete artifact: deleted=%v err=%v", deleted, err)
	}
	var count int64
	if err := db.Model(&model.TranscodeArtifactRecord{}).Where("id = ?", artifact.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("artifact cleanup tombstone missing: %d", count)
	}
	var tombstone model.TranscodeArtifactRecord
	if err := db.First(&tombstone, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if tombstone.CleanupState != ArtifactCleanupCompleted || tombstone.CleanupDisposition != "test_cleanup" {
		t.Fatalf("unexpected cleanup tombstone: %+v", tombstone)
	}
}

func TestExpiredCleanupClaimCanBeRecovered(t *testing.T) {
	db := newArtifactCleanupTestDB(t)
	repo := NewTranscodeExecutionRepo(db)
	now := time.Now().UTC().Truncate(time.Second)
	old := now.Add(-8 * 24 * time.Hour)
	artifact := &model.TranscodeArtifactRecord{
		ID:                "expired-cleanup-artifact",
		JobID:             "expired-cleanup-job",
		MediaID:           "cleanup-media",
		Kind:              "hls_variant",
		ProfileID:         "720p",
		SourceFingerprint: "source",
		PlannerVersion:    "planner",
		Status:            "abandoned",
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
	if _, ok, err := repo.ClaimArtifactCleanup(
		artifact.ID,
		"dead-instance",
		now.Add(-7*24*time.Hour),
		now,
		time.Minute,
	); err != nil || !ok {
		t.Fatalf("initial cleanup claim: ok=%v err=%v", ok, err)
	}

	recovered, ok, err := repo.ClaimArtifactCleanup(
		artifact.ID,
		"recovery-instance",
		now.Add(-7*24*time.Hour),
		now.Add(2*time.Minute),
		time.Minute,
	)
	if err != nil || !ok {
		t.Fatalf("expired cleanup lease was not recoverable: ok=%v err=%v", ok, err)
	}
	if recovered.CleanupAttempts != 2 || recovered.CleanupToken != "recovery-instance" {
		t.Fatalf("unexpected recovered cleanup claim: %+v", recovered)
	}
}

func newArtifactCleanupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", filepath.Join(t.TempDir(), "cleanup.db"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	return db
}
