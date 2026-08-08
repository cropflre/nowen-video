package service

import (
	"errors"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

func TestLegacySourceRetirementDecisionRequiresAllEvidence(t *testing.T) {
	_, db := newArtifactMaintenanceTestService(t)
	execution := repository.NewTranscodeExecutionRepo(db)
	retirement := NewLegacySourceRetirementService(execution)
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	retirement.clock = func() time.Time { return now }

	legacyTaskID := "legacy-task-1"
	if err := db.Create(&model.TranscodeTask{
		ID:        legacyTaskID,
		MediaID:   "media-1",
		Status:    "done",
		Quality:   "720p",
		OutputDir: "/tmp/legacy-task-1",
		CreatedAt: now.Add(-60 * 24 * time.Hour),
		UpdatedAt: now.Add(-40 * 24 * time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TranscodeJobRecord{
		ID:           "job-1",
		LegacyTaskID: &legacyTaskID,
		MediaID:      "media-1",
		Status:       "completed",
		DesiredState: "running",
		CreatedAt:    now.Add(-40 * 24 * time.Hour),
		UpdatedAt:    now.Add(-40 * 24 * time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	quiescentSince := now.Add(-31 * 24 * time.Hour)
	retireAfter := now.Add(-24 * time.Hour)
	completedAt := quiescentSince
	cursorAt := now.Add(-40 * 24 * time.Hour)
	if err := db.Create(&model.LegacyTranscodeProjectionMigrationState{
		Source:             repository.LegacyTranscodeArtifactMigrationSource,
		Generation:         1,
		Status:             repository.LegacyProjectionMigrationCompleted,
		CursorUpdatedAt:    &cursorAt,
		CursorID:           legacyTaskID,
		HighWaterUpdatedAt: &cursorAt,
		HighWaterID:        legacyTaskID,
		TargetRows:         1,
		ScannedRows:        1,
		CompletedAt:        &completedAt,
		QuiescentSince:     &quiescentSince,
		SourceRetireAfter:  &retireAfter,
		CreatedAt:          now.Add(-40 * 24 * time.Hour),
		UpdatedAt:          now.Add(-31 * 24 * time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}

	rollbackUntil := now.Add(time.Hour)
	if err := db.Create(&model.TranscodeArtifactRecord{
		ID:                   "artifact-1",
		JobID:                "job-1",
		MediaID:              "media-1",
		Kind:                 repository.LegacyTranscodeArtifactKind,
		Status:               "expired",
		MigrationSource:      repository.LegacyTranscodeArtifactMigrationSource,
		CleanupState:         repository.ArtifactCleanupPending,
		CleanupRollbackUntil: &rollbackUntil,
		CreatedAt:            now.Add(-31 * 24 * time.Hour),
		UpdatedAt:            now.Add(-31 * 24 * time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}

	report, err := retirement.Report(repository.LegacyTranscodeArtifactMigrationSource)
	if err != nil {
		t.Fatal(err)
	}
	if report.ReadyForBackupReview || report.RollbackOpenArtifacts != 1 || !containsRetirementBlocker(report.Blockers, "rollback_window_open") {
		t.Fatalf("report should be blocked by rollback window: %+v", report)
	}

	expiredRollback := now.Add(-time.Hour)
	if err := db.Model(&model.TranscodeArtifactRecord{}).
		Where("id = ?", "artifact-1").
		Update("cleanup_rollback_until", expiredRollback).Error; err != nil {
		t.Fatal(err)
	}
	report, err = retirement.Report(repository.LegacyTranscodeArtifactMigrationSource)
	if err != nil {
		t.Fatal(err)
	}
	if !report.ReadyForBackupReview || !report.ObservationSatisfied || report.UnmigratedRows != 0 {
		t.Fatalf("report should be ready for backup review: %+v", report)
	}

	_, err = retirement.Review(report.Source, LegacySourceRetirementReviewRequest{
		Decision:             LegacySourceRetirementDecisionApprove,
		ExpectedEvidenceHash: report.EvidenceHash,
	}, "admin-1", "admin")
	if !errors.Is(err, ErrLegacySourceRetirementInvalid) {
		t.Fatalf("approval without backup should fail validation: %v", err)
	}

	verifiedAt := now.Add(-2 * time.Hour)
	restoreTestedAt := now.Add(-time.Hour)
	record, err := retirement.Review(report.Source, LegacySourceRetirementReviewRequest{
		Decision:             LegacySourceRetirementDecisionApprove,
		ExpectedEvidenceHash: report.EvidenceHash,
		Reason:               "30-day evidence and restore test accepted",
		Backup: LegacySourceBackupVerification{
			Verified:        true,
			VerifiedAt:      &verifiedAt,
			RestoreTestedAt: &restoreTestedAt,
			Reference:       "backup://legacy-transcode/2026-08-06",
			Checksum:        "sha256:0123456789abcdef",
		},
	}, "admin-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if record.Decision != LegacySourceRetirementDecisionApprove || !record.BackupVerified || record.EvidenceJSON == "" {
		t.Fatalf("unexpected decision record: %+v", record)
	}
	if !db.Migrator().HasTable(&model.LegacySourceRetirementDecisionRecord{}) {
		t.Fatal("decision table was not created")
	}
	if !db.Migrator().HasTable(&model.TranscodeTask{}) {
		t.Fatal("review must not drop the legacy source table")
	}
	var legacy model.TranscodeTask
	if err := db.First(&legacy, "id = ?", legacyTaskID).Error; err != nil {
		t.Fatalf("review mutated or deleted legacy row: %v", err)
	}

	plan, err := retirement.PrepareRemovalPlan(report.Source, LegacySourceRemovalPlanRequest{
		ExpectedEvidenceHash: report.EvidenceHash,
		ExpectedDecisionID:   record.ID,
		Reason:               "prepare explicit schema-migration handoff",
	}, "admin-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != LegacySourceRemovalPlanStatusPrepared || plan.SchemaHash == "" || plan.SchemaJSON == "" || plan.ExpiresAt == nil {
		t.Fatalf("unexpected removal plan: %+v", plan)
	}
	if !db.Migrator().HasTable(&model.LegacySourceRemovalPlanRecord{}) {
		t.Fatal("removal plan table was not created")
	}
	if !db.Migrator().HasTable(&model.TranscodeTask{}) {
		t.Fatal("removal planning must not drop the legacy source table")
	}

	// Rows without output directories are still part of the legacy audit source
	// and must invalidate both approval and any prepared removal-plan evidence.
	if err := db.Create(&model.TranscodeTask{
		ID:        "legacy-task-unmigrated",
		MediaID:   "media-2",
		Status:    "failed",
		OutputDir: "",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	changedReport, err := retirement.Report(report.Source)
	if err != nil {
		t.Fatal(err)
	}
	if changedReport.UnmigratedRows != 1 || !containsRetirementBlocker(changedReport.Blockers, "unmigrated_rows_present") {
		t.Fatalf("all legacy rows must be counted as migration evidence: %+v", changedReport)
	}
	_, err = retirement.Review(report.Source, LegacySourceRetirementReviewRequest{
		Decision:             LegacySourceRetirementDecisionApprove,
		ExpectedEvidenceHash: report.EvidenceHash,
		Backup: LegacySourceBackupVerification{
			Verified:        true,
			VerifiedAt:      &verifiedAt,
			RestoreTestedAt: &restoreTestedAt,
			Reference:       "backup://legacy-transcode/2026-08-06",
			Checksum:        "sha256:0123456789abcdef",
		},
	}, "admin-1", "admin")
	if !errors.Is(err, ErrLegacySourceRetirementEvidenceStale) {
		t.Fatalf("changed source evidence should reject stale approval: %v", err)
	}
}

func TestLegacySourceRetirementReviewRecomputesCompleteSnapshot(t *testing.T) {
	_, db := newArtifactMaintenanceTestService(t)
	retirement := NewLegacySourceRetirementService(repository.NewTranscodeExecutionRepo(db))
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	legacyTaskID := "legacy-task-complete-snapshot"
	cursorAt := now.Add(-40 * 24 * time.Hour)
	quiescentSince := now.Add(-31 * 24 * time.Hour)
	retireAfter := now.Add(-24 * time.Hour)
	if err := db.Create(&model.TranscodeTask{
		ID:        legacyTaskID,
		MediaID:   "media-complete-snapshot",
		Status:    "done",
		CreatedAt: cursorAt,
		UpdatedAt: cursorAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TranscodeJobRecord{
		ID:           "job-complete-snapshot",
		LegacyTaskID: &legacyTaskID,
		MediaID:      "media-complete-snapshot",
		Status:       "completed",
		DesiredState: "running",
		CreatedAt:    cursorAt,
		UpdatedAt:    cursorAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.LegacyTranscodeProjectionMigrationState{
		Source:             repository.LegacyTranscodeArtifactMigrationSource,
		Generation:         1,
		Status:             repository.LegacyProjectionMigrationCompleted,
		CursorUpdatedAt:    &cursorAt,
		CursorID:           legacyTaskID,
		HighWaterUpdatedAt: &cursorAt,
		HighWaterID:        legacyTaskID,
		TargetRows:         1,
		ScannedRows:        1,
		QuiescentSince:     &quiescentSince,
		SourceRetireAfter:  &retireAfter,
		CreatedAt:          cursorAt,
		UpdatedAt:          quiescentSince,
	}).Error; err != nil {
		t.Fatal(err)
	}
	retirement.clock = func() time.Time { return now }
	report, err := retirement.Report(repository.LegacyTranscodeArtifactMigrationSource)
	if err != nil {
		t.Fatal(err)
	}

	calls := 0
	retirement.clock = func() time.Time {
		calls++
		if calls == 2 {
			if err := db.Model(&model.LegacyTranscodeProjectionMigrationState{}).
				Where("source = ?", repository.LegacyTranscodeArtifactMigrationSource).
				Update("target_rows", 2).Error; err != nil {
				t.Fatalf("change migration evidence: %v", err)
			}
		}
		return now
	}
	_, err = retirement.Review(report.Source, LegacySourceRetirementReviewRequest{
		Decision:             LegacySourceRetirementDecisionDefer,
		ExpectedEvidenceHash: report.EvidenceHash,
		Reason:               "exercise second snapshot",
	}, "admin-1", "admin")
	if !errors.Is(err, ErrLegacySourceRetirementEvidenceStale) {
		t.Fatalf("complete second snapshot should reject changed target rows: %v", err)
	}
}

func TestLegacySourceRetirementObservationWindowBlocksApproval(t *testing.T) {
	_, db := newArtifactMaintenanceTestService(t)
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	retirement := NewLegacySourceRetirementService(repository.NewTranscodeExecutionRepo(db))
	retirement.clock = func() time.Time { return now }
	quiescentSince := now.Add(-10 * 24 * time.Hour)
	retireAfter := now.Add(20 * 24 * time.Hour)
	if err := db.Create(&model.LegacyTranscodeProjectionMigrationState{
		Source:            repository.LegacyTranscodeArtifactMigrationSource,
		Generation:        1,
		Status:            repository.LegacyProjectionMigrationCompleted,
		QuiescentSince:    &quiescentSince,
		SourceRetireAfter: &retireAfter,
		CreatedAt:         quiescentSince,
		UpdatedAt:         quiescentSince,
	}).Error; err != nil {
		t.Fatal(err)
	}
	report, err := retirement.Report(repository.LegacyTranscodeArtifactMigrationSource)
	if err != nil {
		t.Fatal(err)
	}
	if report.ObservationSatisfied || report.ReadyForBackupReview || !containsRetirementBlocker(report.Blockers, "observation_window_open") {
		t.Fatalf("observation window should block review: %+v", report)
	}
}

func containsRetirementBlocker(blockers []string, expected string) bool {
	for _, blocker := range blockers {
		if blocker == expected {
			return true
		}
	}
	return false
}
