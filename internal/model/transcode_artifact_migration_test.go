package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestArtifactMigrationBackfillsJobIdentityWithoutDeletingHistory(t *testing.T) {
	dsn := fmt.Sprintf("file:artifact-migration-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	job := &TranscodeJobRecord{
		MediaID:             "media-migration",
		Intent:              "runtime_hls",
		ProfileID:           "1080p",
		Status:              "completed",
		DesiredState:        "running",
		SourceFingerprint:   "source-fingerprint",
		PlannerVersion:      "runtime-hls-v2",
		EncodingPlanVersion: "hls-encoding-plan-v1",
		EncodingPlanHash:    "encoding-plan-hash",
		EncodingPlanJSON:    `{"schema_version":"hls-encoding-plan-v1"}`,
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatal(err)
	}
	artifact := &TranscodeArtifactRecord{
		JobID:     job.ID,
		Kind:      "hls_variant",
		ProfileID: "1080p",
		Path:      "/cache/legacy/media/1080p",
		Status:    "published",
	}
	if err := db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}

	// Re-running the migration represents an upgrade from the schema where
	// Artifact identity and Encoding Plan lived only on transcode_jobs.
	if err := AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	var stored TranscodeArtifactRecord
	if err := db.First(&stored, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.MediaID != job.MediaID || stored.SourceFingerprint != job.SourceFingerprint || stored.PlannerVersion != job.PlannerVersion {
		t.Fatalf("artifact identity was not backfilled: %+v", stored)
	}
	if stored.EncodingPlanVersion != job.EncodingPlanVersion || stored.EncodingPlanHash != job.EncodingPlanHash || stored.EncodingPlanJSON != job.EncodingPlanJSON {
		t.Fatalf("artifact encoding plan was not backfilled: %+v", stored)
	}
	var jobCount int64
	var attemptCount int64
	var artifactCount int64
	if err := db.Model(&TranscodeJobRecord{}).Count(&jobCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&TranscodeAttemptRecord{}).Count(&attemptCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&TranscodeArtifactRecord{}).Count(&artifactCount).Error; err != nil {
		t.Fatal(err)
	}
	if jobCount != 1 || attemptCount != 0 || artifactCount != 1 {
		t.Fatalf("migration deleted historical rows: jobs=%d attempts=%d artifacts=%d", jobCount, attemptCount, artifactCount)
	}
}
