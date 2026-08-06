package service

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

func TestLegacyProjectionImportsAuditJobWithoutOutputArtifact(t *testing.T) {
	maintenance, db := newArtifactMaintenanceTestService(t)
	now := time.Date(2026, 8, 6, 2, 0, 0, 0, time.UTC)
	task := model.TranscodeTask{
		ID:        "legacy-no-output",
		MediaID:   "media-no-output",
		Status:    "failed",
		OutputDir: "",
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}

	report, err := maintenance.inventoryLegacyTranscodeProjection(now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if report.JobsImported != 1 || report.ArtifactsQueued != 0 || report.ArtifactsBlocked != 0 || report.MissingPaths != 0 {
		t.Fatalf("unexpected audit projection report: %+v", report)
	}

	var jobs int64
	if err := db.Model(&model.TranscodeJobRecord{}).Where("legacy_task_id = ?", task.ID).Count(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	if jobs != 1 {
		t.Fatalf("expected one deterministic audit Job, got %d", jobs)
	}
	var artifacts int64
	if err := db.Model(&model.TranscodeArtifactRecord{}).Where("job_id = ?", deterministicLegacyProjectionID("job", task.ID)).Count(&artifacts).Error; err != nil {
		t.Fatal(err)
	}
	if artifacts != 0 {
		t.Fatalf("empty output path must not create an Artifact, got %d", artifacts)
	}

	inventory, err := maintenance.executionRepo.LegacySourceRetirementInventory(repository.LegacyTranscodeArtifactMigrationSource, now.Add(8*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if inventory.SourceRows != 1 || inventory.UnmigratedRows != 0 {
		t.Fatalf("retirement inventory did not recognize the audit Job: %+v", inventory)
	}

	second, err := maintenance.inventoryLegacyTranscodeProjection(now.Add(16 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if second.JobsImported != 0 || second.TasksFound != 0 {
		t.Fatalf("completed full-source generation was not idempotent: %+v", second)
	}
	if err := db.Model(&model.TranscodeJobRecord{}).Where("legacy_task_id = ?", task.ID).Count(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	if jobs != 1 {
		t.Fatalf("replay created duplicate audit Jobs: %d", jobs)
	}
}

func TestLegacyProjectionReopensCompletedDirectoryOnlyGeneration(t *testing.T) {
	maintenance, db := newArtifactMaintenanceTestService(t)
	now := time.Date(2026, 8, 6, 3, 0, 0, 0, time.UTC)
	older := now.Add(-2 * time.Hour)
	newer := now.Add(-time.Hour)

	missing := model.TranscodeTask{
		ID:        "legacy-backfill-no-output",
		MediaID:   "media-backfill-no-output",
		Status:    "failed",
		OutputDir: "",
		CreatedAt: older,
		UpdatedAt: older,
	}
	projected := model.TranscodeTask{
		ID:        "legacy-backfill-existing",
		MediaID:   "media-backfill-existing",
		Status:    "done",
		OutputDir: "",
		CreatedAt: newer,
		UpdatedAt: newer,
	}
	if err := db.Create(&missing).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&projected).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := ensureLegacyProjectionJob(db, &projected); err != nil {
		t.Fatal(err)
	}

	completedAt := now.Add(-31 * 24 * time.Hour)
	retireAfter := now.Add(-24 * time.Hour)
	nextSourceCheck := now.Add(-time.Minute)
	if err := db.Create(&model.LegacyTranscodeProjectionMigrationState{
		Source:             repository.LegacyTranscodeArtifactMigrationSource,
		Generation:         1,
		Status:             repository.LegacyProjectionMigrationCompleted,
		CursorUpdatedAt:    &newer,
		CursorID:           projected.ID,
		HighWaterUpdatedAt: &newer,
		HighWaterID:        projected.ID,
		TargetRows:         1,
		ScannedRows:        1,
		ImportedJobs:       1,
		BatchSize:          250,
		CompletedAt:        &completedAt,
		QuiescentSince:     &completedAt,
		SourceRetireAfter:  &retireAfter,
		NextSourceCheckAt:  &nextSourceCheck,
		CreatedAt:          completedAt,
		UpdatedAt:          completedAt,
	}).Error; err != nil {
		t.Fatal(err)
	}

	report, err := maintenance.inventoryLegacyTranscodeProjection(now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != repository.LegacyProjectionMigrationCompleted || report.TargetRows != 2 || report.ScannedRows != 2 {
		t.Fatalf("full-source backfill generation did not complete: %+v", report)
	}
	state, err := maintenance.executionRepo.LegacyProjectionMigrationState(repository.LegacyTranscodeArtifactMigrationSource)
	if err != nil {
		t.Fatal(err)
	}
	if state == nil || state.Generation != 2 || state.TargetRows != 2 || state.ScannedRows != 2 || state.QuiescentSince == nil || state.SourceRetireAfter == nil {
		t.Fatalf("completed v1 state was not replaced by a fresh observation generation: %+v", state)
	}
	var jobs int64
	if err := db.Model(&model.TranscodeJobRecord{}).Where("legacy_task_id IN ?", []string{missing.ID, projected.ID}).Count(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	if jobs != 2 {
		t.Fatalf("expected both legacy rows to have audit Jobs, got %d", jobs)
	}
	var artifacts int64
	if err := db.Model(&model.TranscodeArtifactRecord{}).Count(&artifacts).Error; err != nil {
		t.Fatal(err)
	}
	if artifacts != 0 {
		t.Fatalf("rows without output paths created cleanup Artifacts: %d", artifacts)
	}
}
