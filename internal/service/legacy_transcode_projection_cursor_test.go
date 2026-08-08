package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

func TestLegacyProjectionInventoryAdvancesDurableCursor(t *testing.T) {
	service, db := newArtifactMaintenanceTestService(t)
	service.legacyMigrationBatchSize = 1
	now := time.Now().UTC().Truncate(time.Millisecond)
	for index, id := range []string{"cursor-a", "cursor-b"} {
		path := filepath.Join(service.artifactStore.Root(), id, "720p")
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		task := model.TranscodeTask{ID: id, MediaID: id, Status: "done", Quality: "720p", OutputDir: path, CreatedAt: now, UpdatedAt: now.Add(time.Duration(index) * time.Second)}
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
	}
	first, err := service.inventoryLegacyTranscodeProjection(now.Add(time.Minute))
	if err != nil || first.TasksFound != 1 || !first.HasMore {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	state, err := service.executionRepo.LegacyProjectionMigrationState(repository.LegacyTranscodeArtifactMigrationSource)
	if err != nil || state == nil || state.CursorID != "cursor-a" || state.ScannedRows != 1 {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	second, err := service.inventoryLegacyTranscodeProjection(now.Add(2 * time.Minute))
	if err != nil || second.TasksFound != 1 || second.Status != repository.LegacyProjectionMigrationCompleted || second.ScannedRows != 2 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	third, err := service.inventoryLegacyTranscodeProjection(now.Add(3 * time.Minute))
	if err != nil || third.TasksFound != 0 || third.ScannedRows != 2 {
		t.Fatalf("completed generation rescanned rows: %+v err=%v", third, err)
	}
}

func TestLegacyProjectionTaskCenterProgress(t *testing.T) {
	service, _ := newArtifactMaintenanceTestService(t)
	now := time.Now()
	state := &model.LegacyTranscodeProjectionMigrationState{
		Source:      repository.LegacyTranscodeArtifactMigrationSource,
		Generation:  3,
		Status:      repository.LegacyProjectionMigrationRunning,
		TargetRows:  10,
		ScannedRows: 4,
		CreatedAt:   now.Add(-time.Minute),
		UpdatedAt:   now,
	}
	if err := service.executionRepo.DB().Create(state).Error; err != nil {
		t.Fatal(err)
	}
	task := legacyProjectionMigrationToUnifiedTask(state, now)
	if task.Kind != TaskKindLegacyProjectionMigration || task.Status != TaskStatusRunning || task.Progress != 40 {
		t.Fatalf("task=%+v", task)
	}
}

func TestLegacyProjectionSourceSnapshotPolicy(t *testing.T) {
	now := time.Now()
	highAt := now.Add(time.Hour)
	same := &repository.LegacyProjectionCursor{UpdatedAt: now, ID: "a"}
	newer := &repository.LegacyProjectionCursor{UpdatedAt: highAt, ID: "b"}
	state := &model.LegacyTranscodeProjectionMigrationState{
		Status:             repository.LegacyProjectionMigrationPending,
		HighWaterUpdatedAt: &now,
		HighWaterID:        "a",
	}
	if shouldRefreshLegacyProjectionTarget(state, newer) {
		t.Fatal("active generation must keep its frozen target")
	}
	if frozen := legacyProjectionFrozenHighWater(state); frozen == nil || frozen.ID != "a" {
		t.Fatalf("frozen=%+v", frozen)
	}
	state.Status = repository.LegacyProjectionMigrationCompleted
	if shouldRefreshLegacyProjectionTarget(state, same) {
		t.Fatal("completed generation refreshed unchanged target")
	}
	if !shouldRefreshLegacyProjectionTarget(state, newer) {
		t.Fatal("completed generation did not detect newer source high-water")
	}
	next := now.Add(time.Minute)
	state.NextSourceCheckAt = &next
	if !legacyProjectionStateDeferred(state, now) {
		t.Fatal("completed generation ignored source-check schedule")
	}
}
