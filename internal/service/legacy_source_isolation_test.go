package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"gorm.io/gorm"
)

func TestLegacySourceIsolationRenamesAndRollsBackWithoutDeletingRows(t *testing.T) {
	retirement, db, now, taskID, plan := prepareLegacySourceIsolationPlan(t)

	request := LegacySourceIsolationRequest{
		ExpectedPlanID:       plan.ID,
		ExpectedEvidenceHash: plan.EvidenceHash,
		ExpectedSchemaHash:   plan.SchemaHash,
		Confirmation:         LegacySourceIsolationConfirmation,
		Reason:               "isolate the approved read-only source",
	}
	isolation, err := retirement.Isolate(plan.Source, request, "admin-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if isolation.Status != LegacySourceIsolationStatusIsolated || isolation.RemovalPlanID != plan.ID {
		t.Fatalf("unexpected isolation record: %+v", isolation)
	}
	assertLegacySourceTableState(t, db, false, true)
	if count := countLegacyRowsByID(t, db, repository.LegacySourceArchiveTable, taskID); count != 1 {
		t.Fatalf("archived source row count = %d, want 1", count)
	}

	// Exact retries are idempotent and do not create another isolation record.
	repeated, err := retirement.Isolate(plan.Source, request, "admin-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if repeated.ID != isolation.ID {
		t.Fatalf("isolation retry returned %s, want %s", repeated.ID, isolation.ID)
	}
	var isolationCount int64
	if err := db.Model(&model.LegacySourceIsolationRecord{}).Count(&isolationCount).Error; err != nil {
		t.Fatal(err)
	}
	if isolationCount != 1 {
		t.Fatalf("isolation records = %d, want 1", isolationCount)
	}

	// Rollback is intentionally allowed after the 24-hour plan lifetime because
	// it is the emergency downgrade path.
	retirement.clock = func() time.Time { return now.Add(48 * time.Hour) }
	rollbackRequest := LegacySourceIsolationRollbackRequest{
		ExpectedIsolationID: isolation.ID,
		ExpectedSchemaHash:  isolation.SchemaHash,
		Confirmation:        LegacySourceIsolationRollbackConfirmation,
		Reason:              "restore the legacy table for an emergency downgrade",
	}
	rollback, err := retirement.RollbackIsolation(plan.Source, rollbackRequest, "admin-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	assertLegacySourceTableState(t, db, true, false)
	if count := countLegacyRowsByID(t, db, repository.LegacySourceOriginalTable, taskID); count != 1 {
		t.Fatalf("restored source row count = %d, want 1", count)
	}

	repeatedRollback, err := retirement.RollbackIsolation(plan.Source, rollbackRequest, "admin-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if repeatedRollback.ID != rollback.ID {
		t.Fatalf("rollback retry returned %s, want %s", repeatedRollback.ID, rollback.ID)
	}
	var rollbackCount int64
	if err := db.Model(&model.LegacySourceIsolationRollbackRecord{}).Count(&rollbackCount).Error; err != nil {
		t.Fatal(err)
	}
	if rollbackCount != 1 {
		t.Fatalf("rollback records = %d, want 1", rollbackCount)
	}
}

func TestLegacySourceIsolationRejectsExpiredPlan(t *testing.T) {
	retirement, db, now, _, plan := prepareLegacySourceIsolationPlan(t)
	retirement.clock = func() time.Time { return now.Add(25 * time.Hour) }

	_, err := retirement.Isolate(plan.Source, LegacySourceIsolationRequest{
		ExpectedPlanID:       plan.ID,
		ExpectedEvidenceHash: plan.EvidenceHash,
		ExpectedSchemaHash:   plan.SchemaHash,
		Confirmation:         LegacySourceIsolationConfirmation,
		Reason:               "expired plans must fail closed",
	}, "admin-1", "admin")
	if !errors.Is(err, ErrLegacySourceIsolationBlocked) {
		t.Fatalf("expired plan error = %v, want isolation blocked", err)
	}
	assertLegacySourceTableState(t, db, true, false)
}

func TestLegacySourceIsolationRejectsSchemaDrift(t *testing.T) {
	retirement, db, _, _, plan := prepareLegacySourceIsolationPlan(t)
	if err := db.Exec("ALTER TABLE transcode_tasks ADD COLUMN isolation_drift_marker TEXT").Error; err != nil {
		t.Fatal(err)
	}

	_, err := retirement.Isolate(plan.Source, LegacySourceIsolationRequest{
		ExpectedPlanID:       plan.ID,
		ExpectedEvidenceHash: plan.EvidenceHash,
		ExpectedSchemaHash:   plan.SchemaHash,
		Confirmation:         LegacySourceIsolationConfirmation,
		Reason:               "schema drift must invalidate the handoff",
	}, "admin-1", "admin")
	if !errors.Is(err, ErrLegacySourceRetirementEvidenceStale) {
		t.Fatalf("schema drift error = %v, want stale evidence", err)
	}
	assertLegacySourceTableState(t, db, true, false)
}

func TestLegacySourceIsolationContainsNoDestructiveDDL(t *testing.T) {
	paths := []string{
		"legacy_source_isolation_service.go",
		filepath.Join("..", "repository", "repo_legacy_source_isolation.go"),
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ToUpper(string(content))
		if strings.Contains(text, "DROP TABLE") || strings.Contains(string(content), ".DropTable(") {
			t.Fatalf("destructive table deletion found in %s", path)
		}
	}
}

func prepareLegacySourceIsolationPlan(t *testing.T) (*LegacySourceRetirementService, *gorm.DB, time.Time, string, *model.LegacySourceRemovalPlanRecord) {
	t.Helper()
	_, db := newArtifactMaintenanceTestService(t)
	execution := repository.NewTranscodeExecutionRepo(db)
	retirement := NewLegacySourceRetirementService(execution)
	now := time.Date(2026, 8, 6, 6, 0, 0, 0, time.UTC)
	retirement.clock = func() time.Time { return now }

	taskID := "legacy-task-isolation"
	cursorAt := now.Add(-40 * 24 * time.Hour)
	if err := db.Create(&model.TranscodeTask{
		ID:        taskID,
		MediaID:   "media-isolation",
		Status:    "done",
		Quality:   "720p",
		OutputDir: "/tmp/legacy-task-isolation",
		CreatedAt: cursorAt,
		UpdatedAt: cursorAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TranscodeJobRecord{
		ID:           "job-isolation",
		LegacyTaskID: &taskID,
		MediaID:      "media-isolation",
		Status:       "completed",
		DesiredState: "running",
		CreatedAt:    cursorAt,
		UpdatedAt:    cursorAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	quiescentSince := now.Add(-31 * 24 * time.Hour)
	retireAfter := now.Add(-24 * time.Hour)
	completedAt := quiescentSince
	if err := db.Create(&model.LegacyTranscodeProjectionMigrationState{
		Source:             repository.LegacyTranscodeArtifactMigrationSource,
		Generation:         3,
		Status:             repository.LegacyProjectionMigrationCompleted,
		CursorUpdatedAt:    &cursorAt,
		CursorID:           taskID,
		HighWaterUpdatedAt: &cursorAt,
		HighWaterID:        taskID,
		TargetRows:         1,
		ScannedRows:        1,
		CompletedAt:        &completedAt,
		QuiescentSince:     &quiescentSince,
		SourceRetireAfter:  &retireAfter,
		CreatedAt:          cursorAt,
		UpdatedAt:          quiescentSince,
	}).Error; err != nil {
		t.Fatal(err)
	}

	report, err := retirement.Report(repository.LegacyTranscodeArtifactMigrationSource)
	if err != nil {
		t.Fatal(err)
	}
	verifiedAt := now.Add(-2 * time.Hour)
	restoreTestedAt := now.Add(-time.Hour)
	decision, err := retirement.Review(report.Source, LegacySourceRetirementReviewRequest{
		Decision:             LegacySourceRetirementDecisionApprove,
		ExpectedEvidenceHash: report.EvidenceHash,
		Reason:               "approve isolation fixture",
		Backup: LegacySourceBackupVerification{
			Verified:        true,
			VerifiedAt:      &verifiedAt,
			RestoreTestedAt: &restoreTestedAt,
			Reference:       "backup://legacy-transcode/isolation-fixture",
			Checksum:        "sha256:isolation-fixture",
		},
	}, "admin-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := retirement.PrepareRemovalPlan(report.Source, LegacySourceRemovalPlanRequest{
		ExpectedEvidenceHash: report.EvidenceHash,
		ExpectedDecisionID:   decision.ID,
		Reason:               "prepare reversible isolation fixture",
	}, "admin-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	return retirement, db, now, taskID, plan
}

func assertLegacySourceTableState(t *testing.T, db *gorm.DB, original, archive bool) {
	t.Helper()
	if got := db.Migrator().HasTable(repository.LegacySourceOriginalTable); got != original {
		t.Fatalf("original table present = %v, want %v", got, original)
	}
	if got := db.Migrator().HasTable(repository.LegacySourceArchiveTable); got != archive {
		t.Fatalf("archive table present = %v, want %v", got, archive)
	}
}

func countLegacyRowsByID(t *testing.T, db *gorm.DB, table, id string) int64 {
	t.Helper()
	var count int64
	if err := db.Table(table).Where("id = ?", id).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}
