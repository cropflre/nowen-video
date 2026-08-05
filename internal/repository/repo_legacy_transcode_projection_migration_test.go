package repository

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

func newLegacyProjectionRepoTestDB(t *testing.T) (*gorm.DB, *TranscodeExecutionRepo) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TranscodeTask{}); err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	return db, NewTranscodeExecutionRepo(db)
}

func TestLegacyProjectionCursorAndGenerationLease(t *testing.T) {
	db, execution := newLegacyProjectionRepoTestDB(t)
	source := &TranscodeRepo{db: db}
	now := time.Now().UTC().Truncate(time.Millisecond)
	rows := []model.TranscodeTask{
		{ID: "a", Status: "done", OutputDir: "/tmp/a", UpdatedAt: now},
		{ID: "b", Status: "failed", OutputDir: "/tmp/b", UpdatedAt: now},
		{ID: "c", Status: "completed", OutputDir: "/tmp/c", UpdatedAt: now.Add(time.Second)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	high, err := source.LegacyProjectionHighWater()
	if err != nil || high == nil || high.ID != "c" {
		t.Fatalf("high=%+v err=%v", high, err)
	}
	target, err := source.CountLegacyTerminalWithOutputThrough(*high)
	if err != nil || target != 3 {
		t.Fatalf("target=%d err=%v", target, err)
	}
	batch, err := source.ListLegacyTerminalWithOutputAfter(nil, *high, 2)
	if err != nil || len(batch) != 2 || batch[0].ID != "a" || batch[1].ID != "b" {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	state, changed, err := execution.PrepareLegacyProjectionMigration(LegacyTranscodeArtifactMigrationSource, high, target, 2, now, 30*24*time.Hour)
	if err != nil || !changed || state.Generation != 1 {
		t.Fatalf("prepare=%+v changed=%v err=%v", state, changed, err)
	}
	claimed, ok, err := execution.ClaimLegacyProjectionMigration(state.Source, "one", "token-one", now, time.Minute)
	if err != nil || !ok || claimed.Status != LegacyProjectionMigrationRunning {
		t.Fatalf("claim=%+v ok=%v err=%v", claimed, ok, err)
	}
	if _, ok, err := execution.ClaimLegacyProjectionMigration(state.Source, "two", "token-two", now, time.Minute); err != nil || ok {
		t.Fatalf("second claim ok=%v err=%v", ok, err)
	}
	cursor := LegacyProjectionCursor{UpdatedAt: batch[1].UpdatedAt, ID: batch[1].ID}
	state, ok, err = execution.CompleteLegacyProjectionMigrationBatch(state.Source, "token-one", cursor, LegacyProjectionBatchDelta{ScannedRows: 2}, false, now, 30*24*time.Hour)
	if err != nil || !ok || state.Status != LegacyProjectionMigrationPending || state.ScannedRows != 2 {
		t.Fatalf("batch state=%+v ok=%v err=%v", state, ok, err)
	}
}

func TestCompletedLegacyProjectionReopensOnlyForNewHighWater(t *testing.T) {
	db, execution := newLegacyProjectionRepoTestDB(t)
	source := &TranscodeRepo{db: db}
	now := time.Now().UTC().Truncate(time.Millisecond)
	if err := db.Create(&model.TranscodeTask{ID: "a", Status: "done", OutputDir: "/tmp/a", UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	high, _ := source.LegacyProjectionHighWater()
	state, _, err := execution.PrepareLegacyProjectionMigration(LegacyTranscodeArtifactMigrationSource, high, 1, 10, now, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	_, ok, err := execution.ClaimLegacyProjectionMigration(state.Source, "one", "token", now, time.Minute)
	if err != nil || !ok {
		t.Fatal(err)
	}
	state, ok, err = execution.CompleteLegacyProjectionMigrationBatch(state.Source, "token", *high, LegacyProjectionBatchDelta{ScannedRows: 1}, true, now, 30*24*time.Hour)
	if err != nil || !ok || state.Generation != 1 {
		t.Fatalf("complete=%+v err=%v", state, err)
	}
	state, changed, err := execution.PrepareLegacyProjectionMigration(state.Source, high, 1, 10, now.Add(time.Minute), 30*24*time.Hour)
	if err != nil || changed || state.Generation != 1 {
		t.Fatalf("same high-water reopened state=%+v changed=%v err=%v", state, changed, err)
	}
	if err := db.Create(&model.TranscodeTask{ID: "b", Status: "done", OutputDir: "/tmp/b", UpdatedAt: now.Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	high, _ = source.LegacyProjectionHighWater()
	state, changed, err = execution.PrepareLegacyProjectionMigration(state.Source, high, 2, 10, now.Add(time.Hour), 30*24*time.Hour)
	if err != nil || !changed || state.Generation != 2 || state.CursorID != "a" {
		t.Fatalf("reopen=%+v changed=%v err=%v", state, changed, err)
	}
}
