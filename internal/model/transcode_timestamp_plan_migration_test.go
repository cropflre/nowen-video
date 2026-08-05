package model

import (
	"testing"

	"gorm.io/gorm"
)

func timestampMigrationJob() *TranscodeJobRecord {
	return &TranscodeJobRecord{
		MediaID:              "media-timestamp-migration",
		Intent:               "startup_continuation_hls",
		ProfileID:            "720p",
		StartMS:              30_000,
		Status:               "completed",
		DesiredState:         "running",
		SourceFingerprint:    "source-timestamp",
		PlannerVersion:       "startup-continuation-hls-v4",
		EncodingPlanVersion:  "hls-encoding-plan-v1",
		EncodingPlanHash:     "encoding-plan",
		EncodingPlanJSON:     `{"schema_version":"hls-encoding-plan-v1"}`,
		TimestampPlanVersion: "hls-timestamp-normalization-v1",
		TimestampPlanHash:    "timestamp-plan",
		TimestampPlanJSON:    `{"schema_version":"hls-timestamp-normalization-v1"}`,
		TimelineOriginMS:     30_000,
	}
}

func TestArtifactCreateInheritsTimestampExecutionContract(t *testing.T) {
	db := openArtifactMigrationDB(t)
	job := timestampMigrationJob()
	if err := db.Create(job).Error; err != nil {
		t.Fatal(err)
	}
	artifact := &TranscodeArtifactRecord{
		JobID:     job.ID,
		Kind:      "startup_continuation_hls",
		ProfileID: "720p",
		Status:    "staging",
	}
	if err := db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	if artifact.TimestampPlanVersion != job.TimestampPlanVersion ||
		artifact.TimestampPlanHash != job.TimestampPlanHash ||
		artifact.TimestampPlanJSON != job.TimestampPlanJSON ||
		artifact.TimelineOriginMS != job.TimelineOriginMS {
		t.Fatalf("artifact did not inherit timestamp execution contract: %+v", artifact)
	}
}

func TestArtifactMigrationBackfillsTimestampIdentityWithoutFabricatingEvidence(t *testing.T) {
	db := openArtifactMigrationDB(t)
	job := timestampMigrationJob()
	if err := db.Create(job).Error; err != nil {
		t.Fatal(err)
	}
	artifact := &TranscodeArtifactRecord{
		JobID:        job.ID,
		Kind:         "startup_continuation_hls",
		ProfileID:    "720p",
		Status:       "published",
		Path:         "/cache/historical/timestamp",
		ManifestPath: "/cache/historical/timestamp/stream.m3u8",
	}
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	var stored TranscodeArtifactRecord
	if err := db.First(&stored, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TimestampPlanVersion != job.TimestampPlanVersion || stored.TimestampPlanHash != job.TimestampPlanHash ||
		stored.TimestampPlanJSON != job.TimestampPlanJSON || stored.TimelineOriginMS != job.TimelineOriginMS {
		t.Fatalf("timestamp execution identity was not backfilled: %+v", stored)
	}
	if stored.AttestationVersion != "" || stored.AttestationHash != "" || stored.AttestationJSON != "" || stored.AttestedAt != nil {
		t.Fatalf("migration fabricated produced-media evidence: %+v", stored)
	}
}

func TestMigrationDoesNotInventTimestampPlanForLegacyJob(t *testing.T) {
	db := openArtifactMigrationDB(t)
	job := encodingPlanMigrationJob()
	if err := db.Create(job).Error; err != nil {
		t.Fatal(err)
	}
	artifact := &TranscodeArtifactRecord{JobID: job.ID, Kind: "hls_variant", Status: "published"}
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	var stored TranscodeArtifactRecord
	if err := db.First(&stored, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TimestampPlanVersion != "" || stored.TimestampPlanHash != "" || stored.TimestampPlanJSON != "" {
		t.Fatalf("legacy job received a fabricated timestamp plan: %+v", stored)
	}
}
