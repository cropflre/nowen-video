#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (root / path).read_text(encoding="utf-8")


def write(path: str, content: str) -> None:
    target = root / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")


def replace_once(path: str, old: str, new: str) -> None:
    content = read(path)
    count = content.count(old)
    if count != 1:
        raise RuntimeError(f"{path}: anchor count {count}, expected 1\n{old[:160]}")
    write(path, content.replace(old, new, 1))


# 1. Durable migration state model.
path = "internal/model/transcode_execution.go"
content = read(path)
anchor = "func AutoMigrateTranscodeExecution(db *gorm.DB) error {\n"
state_model = r'''type LegacyTranscodeProjectionMigrationState struct {
	Source              string     `json:"source" gorm:"primaryKey;type:text"`
	Generation          int64      `json:"generation" gorm:"default:0"`
	Status              string     `json:"status" gorm:"index;type:text;not null"`
	CursorUpdatedAt     *time.Time `json:"cursor_updated_at,omitempty" gorm:"index"`
	CursorID            string     `json:"cursor_id" gorm:"type:text"`
	HighWaterUpdatedAt  *time.Time `json:"high_water_updated_at,omitempty" gorm:"index"`
	HighWaterID         string     `json:"high_water_id" gorm:"type:text"`
	TargetRows          int64      `json:"target_rows"`
	ScannedRows         int64      `json:"scanned_rows"`
	ImportedJobs        int64      `json:"imported_jobs"`
	ArtifactsQueued     int64      `json:"artifacts_queued"`
	ArtifactsBlocked    int64      `json:"artifacts_blocked"`
	MissingPaths        int64      `json:"missing_paths"`
	BatchSize           int        `json:"batch_size"`
	FailureCount        int        `json:"failure_count"`
	LastErrorCode       string     `json:"last_error_code" gorm:"type:text"`
	LastErrorMessage    string     `json:"last_error_message" gorm:"type:text"`
	LastBatchStartedAt  *time.Time `json:"last_batch_started_at,omitempty"`
	LastBatchCompletedAt *time.Time `json:"last_batch_completed_at,omitempty"`
	NextAttemptAt       *time.Time `json:"next_attempt_at,omitempty" gorm:"index"`
	LeaseOwner          string     `json:"lease_owner" gorm:"type:text"`
	LeaseToken          string     `json:"lease_token" gorm:"index;type:text"`
	LeaseExpiresAt      *time.Time `json:"lease_expires_at,omitempty" gorm:"index"`
	CompletedAt         *time.Time `json:"completed_at,omitempty" gorm:"index"`
	QuiescentSince      *time.Time `json:"quiescent_since,omitempty" gorm:"index"`
	SourceRetireAfter   *time.Time `json:"source_retire_after,omitempty" gorm:"index"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at" gorm:"index"`
}

func (LegacyTranscodeProjectionMigrationState) TableName() string {
	return "legacy_transcode_projection_migrations"
}

'''
if state_model not in content:
    if content.count(anchor) != 1:
        raise RuntimeError("transcode execution migration anchor missing")
    content = content.replace(anchor, state_model + anchor, 1)
old = '''		&TranscodeArtifactRecord{},
		&TranscodeHandoffAttestationRecord{},
'''
new = '''		&TranscodeArtifactRecord{},
		&LegacyTranscodeProjectionMigrationState{},
		&TranscodeHandoffAttestationRecord{},
'''
if new not in content:
    if content.count(old) != 1:
        raise RuntimeError("AutoMigrate state anchor mismatch")
    content = content.replace(old, new, 1)
write(path, content)

# 2. Cursor-based read-only source gateway.
path = "internal/repository/repo_stats.go"
content = read(path)
start = content.index("// ListLegacyTerminalWithOutput returns a bounded inventory source.")
end = content.index("// ==================== PlaybackStatsRepo", start)
replacement = r'''type LegacyProjectionCursor struct {
	UpdatedAt time.Time
	ID        string
}

func LegacyProjectionCursorAfter(left, right LegacyProjectionCursor) bool {
	if left.UpdatedAt.Equal(right.UpdatedAt) {
		return left.ID > right.ID
	}
	return left.UpdatedAt.After(right.UpdatedAt)
}

func (r *TranscodeRepo) legacyTerminalWithOutputQuery() *gorm.DB {
	return r.db.Model(&model.TranscodeTask{}).Where(
		"status IN ? AND TRIM(COALESCE(output_dir, '')) <> ''",
		[]string{"done", "completed", "failed", "cancelled"},
	)
}

func (r *TranscodeRepo) LegacyProjectionHighWater() (*LegacyProjectionCursor, error) {
	if !r.LegacyTableExists() {
		return nil, nil
	}
	var row struct {
		UpdatedAt time.Time
		ID        string
	}
	result := r.legacyTerminalWithOutputQuery().
		Select("updated_at", "id").
		Order("updated_at DESC, id DESC").
		Limit(1).
		Find(&row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &LegacyProjectionCursor{UpdatedAt: row.UpdatedAt, ID: row.ID}, nil
}

func (r *TranscodeRepo) CountLegacyTerminalWithOutputThrough(highWater LegacyProjectionCursor) (int64, error) {
	if !r.LegacyTableExists() {
		return 0, nil
	}
	var count int64
	err := r.legacyTerminalWithOutputQuery().
		Where("updated_at < ? OR (updated_at = ? AND id <= ?)", highWater.UpdatedAt, highWater.UpdatedAt, highWater.ID).
		Count(&count).Error
	return count, err
}

func (r *TranscodeRepo) ListLegacyTerminalWithOutputAfter(after *LegacyProjectionCursor, highWater LegacyProjectionCursor, limit int) ([]model.TranscodeTask, error) {
	if !r.LegacyTableExists() {
		return []model.TranscodeTask{}, nil
	}
	if limit <= 0 {
		limit = 250
	}
	if limit > 2000 {
		limit = 2000
	}
	query := r.legacyTerminalWithOutputQuery().
		Where("updated_at < ? OR (updated_at = ? AND id <= ?)", highWater.UpdatedAt, highWater.UpdatedAt, highWater.ID)
	if after != nil {
		query = query.Where("updated_at > ? OR (updated_at = ? AND id > ?)", after.UpdatedAt, after.UpdatedAt, after.ID)
	}
	var tasks []model.TranscodeTask
	err := query.Order("updated_at ASC, id ASC").Limit(limit).Find(&tasks).Error
	return tasks, err
}

// ListLegacyTerminalWithOutput remains as a bounded compatibility reader for
// diagnostics. Migration code uses the explicit cursor/high-water API above.
func (r *TranscodeRepo) ListLegacyTerminalWithOutput(limit int) ([]model.TranscodeTask, error) {
	highWater, err := r.LegacyProjectionHighWater()
	if err != nil || highWater == nil {
		return []model.TranscodeTask{}, err
	}
	return r.ListLegacyTerminalWithOutputAfter(nil, *highWater, limit)
}

'''
content = content[:start] + replacement + content[end:]
content = content.replace('import (\n\t"github.com/nowen-video/nowen-video/internal/model"', 'import (\n\t"time"\n\n\t"github.com/nowen-video/nowen-video/internal/model"', 1)
write(path, content)

# 3. Durable migration repository state machine.
write("internal/repository/repo_legacy_transcode_projection_migration.go", r'''package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	LegacyProjectionMigrationPending   = "pending"
	LegacyProjectionMigrationRunning   = "running"
	LegacyProjectionMigrationCompleted = "completed"
	LegacyProjectionMigrationFailed    = "failed"
)

type LegacyProjectionBatchDelta struct {
	ScannedRows      int64
	ImportedJobs     int64
	ArtifactsQueued  int64
	ArtifactsBlocked int64
	MissingPaths     int64
}

func (r *TranscodeExecutionRepo) LegacyProjectionMigrationState(source string) (*model.LegacyTranscodeProjectionMigrationState, error) {
	var state model.LegacyTranscodeProjectionMigrationState
	result := r.db.Where("source = ?", source).Limit(1).Find(&state)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &state, nil
}

func (r *TranscodeExecutionRepo) prepareLegacyProjectionState(tx *gorm.DB, source string, batchSize int, now time.Time) error {
	if batchSize <= 0 {
		batchSize = 250
	}
	state := &model.LegacyTranscodeProjectionMigrationState{
		Source: source, Status: LegacyProjectionMigrationPending, BatchSize: batchSize,
		CreatedAt: now, UpdatedAt: now,
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(state).Error
}

// PrepareLegacyProjectionMigration freezes one finite source high-water per
// generation. A completed generation reopens only when the read-only source has
// a strictly newer terminal row.
func (r *TranscodeExecutionRepo) PrepareLegacyProjectionMigration(
	source string,
	highWater *LegacyProjectionCursor,
	targetRows int64,
	batchSize int,
	now time.Time,
	retirementWindow time.Duration,
) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {
	var state model.LegacyTranscodeProjectionMigrationState
	changed := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := r.prepareLegacyProjectionState(tx, source, batchSize, now); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&state, "source = ?", source).Error; err != nil {
			return err
		}
		updates := map[string]any{}
		if highWater == nil {
			if state.HighWaterUpdatedAt == nil && state.Status != LegacyProjectionMigrationCompleted {
				retireAfter := now.Add(retirementWindow)
				updates = map[string]any{
					"status": LegacyProjectionMigrationCompleted,
					"target_rows": int64(0),
					"completed_at": now,
					"quiescent_since": now,
					"source_retire_after": retireAfter,
					"next_attempt_at": nil,
					"updated_at": now,
				}
			}
		} else if state.HighWaterUpdatedAt == nil {
			updates = map[string]any{
				"generation": int64(1),
				"status": LegacyProjectionMigrationPending,
				"high_water_updated_at": highWater.UpdatedAt,
				"high_water_id": highWater.ID,
				"target_rows": targetRows,
				"batch_size": batchSize,
				"next_attempt_at": now,
				"completed_at": nil,
				"quiescent_since": nil,
				"source_retire_after": nil,
				"updated_at": now,
			}
		} else {
			currentHigh := LegacyProjectionCursor{UpdatedAt: *state.HighWaterUpdatedAt, ID: state.HighWaterID}
			if state.Status == LegacyProjectionMigrationCompleted && LegacyProjectionCursorAfter(*highWater, currentHigh) {
				updates = map[string]any{
					"generation": state.Generation + 1,
					"status": LegacyProjectionMigrationPending,
					"cursor_updated_at": currentHigh.UpdatedAt,
					"cursor_id": currentHigh.ID,
					"high_water_updated_at": highWater.UpdatedAt,
					"high_water_id": highWater.ID,
					"target_rows": targetRows,
					"batch_size": batchSize,
					"next_attempt_at": now,
					"last_error_code": "",
					"last_error_message": "",
					"completed_at": nil,
					"quiescent_since": nil,
					"source_retire_after": nil,
					"updated_at": now,
				}
			}
		}
		if len(updates) > 0 {
			if err := tx.Model(&model.LegacyTranscodeProjectionMigrationState{}).Where("source = ?", source).Updates(updates).Error; err != nil {
				return err
			}
			changed = true
			return tx.First(&state, "source = ?", source).Error
		}
		return nil
	})
	return &state, changed, err
}

func (r *TranscodeExecutionRepo) ClaimLegacyProjectionMigration(source, owner, token string, now time.Time, leaseDuration time.Duration) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {
	if leaseDuration <= 0 {
		leaseDuration = 2 * time.Minute
	}
	result := r.db.Model(&model.LegacyTranscodeProjectionMigrationState{}).
		Where("source = ? AND high_water_updated_at IS NOT NULL", source).
		Where("status IN ?", []string{LegacyProjectionMigrationPending, LegacyProjectionMigrationFailed, LegacyProjectionMigrationRunning}).
		Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
		Where("lease_token = '' OR lease_expires_at IS NULL OR lease_expires_at <= ?", now).
		Updates(map[string]any{
			"status": LegacyProjectionMigrationRunning,
			"lease_owner": owner,
			"lease_token": token,
			"lease_expires_at": now.Add(leaseDuration),
			"last_batch_started_at": now,
			"updated_at": now,
		})
	if result.Error != nil || result.RowsAffected != 1 {
		return nil, false, result.Error
	}
	state, err := r.LegacyProjectionMigrationState(source)
	return state, state != nil, err
}

func (r *TranscodeExecutionRepo) CompleteLegacyProjectionMigrationBatch(source, token string, cursor LegacyProjectionCursor, delta LegacyProjectionBatchDelta, completed bool, now time.Time, retirementWindow time.Duration) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {
	updates := map[string]any{
		"cursor_updated_at": cursor.UpdatedAt,
		"cursor_id": cursor.ID,
		"scanned_rows": gorm.Expr("scanned_rows + ?", delta.ScannedRows),
		"imported_jobs": gorm.Expr("imported_jobs + ?", delta.ImportedJobs),
		"artifacts_queued": gorm.Expr("artifacts_queued + ?", delta.ArtifactsQueued),
		"artifacts_blocked": gorm.Expr("artifacts_blocked + ?", delta.ArtifactsBlocked),
		"missing_paths": gorm.Expr("missing_paths + ?", delta.MissingPaths),
		"last_batch_completed_at": now,
		"last_error_code": "",
		"last_error_message": "",
		"lease_owner": "",
		"lease_token": "",
		"lease_expires_at": nil,
		"updated_at": now,
	}
	if completed {
		updates["status"] = LegacyProjectionMigrationCompleted
		updates["completed_at"] = now
		updates["quiescent_since"] = now
		updates["source_retire_after"] = now.Add(retirementWindow)
		updates["next_attempt_at"] = nil
	} else {
		updates["status"] = LegacyProjectionMigrationPending
		updates["completed_at"] = nil
		updates["quiescent_since"] = nil
		updates["source_retire_after"] = nil
		updates["next_attempt_at"] = now
	}
	result := r.db.Model(&model.LegacyTranscodeProjectionMigrationState{}).
		Where("source = ? AND status = ? AND lease_token = ?", source, LegacyProjectionMigrationRunning, token).
		Updates(updates)
	if result.Error != nil || result.RowsAffected != 1 {
		return nil, false, result.Error
	}
	state, err := r.LegacyProjectionMigrationState(source)
	return state, state != nil, err
}

func (r *TranscodeExecutionRepo) FailLegacyProjectionMigrationBatch(source, token, code, message string, nextAttemptAt, now time.Time) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {
	result := r.db.Model(&model.LegacyTranscodeProjectionMigrationState{}).
		Where("source = ? AND status = ? AND lease_token = ?", source, LegacyProjectionMigrationRunning, token).
		Updates(map[string]any{
			"status": LegacyProjectionMigrationFailed,
			"failure_count": gorm.Expr("failure_count + 1"),
			"last_error_code": code,
			"last_error_message": message,
			"next_attempt_at": nextAttemptAt,
			"last_batch_completed_at": now,
			"lease_owner": "",
			"lease_token": "",
			"lease_expires_at": nil,
			"updated_at": now,
		})
	if result.Error != nil || result.RowsAffected != 1 {
		return nil, false, result.Error
	}
	state, err := r.LegacyProjectionMigrationState(source)
	return state, state != nil, err
}

func (r *TranscodeExecutionRepo) RetryLegacyProjectionMigration(source string, now time.Time) (bool, error) {
	result := r.db.Model(&model.LegacyTranscodeProjectionMigrationState{}).
		Where("source = ? AND status = ?", source, LegacyProjectionMigrationFailed).
		Updates(map[string]any{
			"status": LegacyProjectionMigrationPending,
			"next_attempt_at": now,
			"last_error_code": "",
			"last_error_message": "",
			"lease_owner": "",
			"lease_token": "",
			"lease_expires_at": nil,
			"updated_at": now,
		})
	return result.RowsAffected == 1, result.Error
}
''')

# 4. Replace the migration service with a cursor/Lease implementation.
write("internal/service/legacy_transcode_projection_migration.go", r'''package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	legacyProjectionRollbackWindow         = 7 * 24 * time.Hour
	legacyProjectionSourceRetirementWindow = 30 * 24 * time.Hour
	legacyProjectionMigrationLease         = 2 * time.Minute
	legacyProjectionDefaultBatchSize       = 250
)

type legacyProjectionInventoryReport struct {
	TasksFound       int
	JobsImported     int
	ArtifactsQueued  int
	ArtifactsBlocked int
	MissingPaths     int
	Generation       int64
	ScannedRows      int64
	TargetRows       int64
	Status           string
	HasMore          bool
}

func (r legacyProjectionInventoryReport) Changed() bool {
	return r.TasksFound > 0 || r.JobsImported > 0 || r.ArtifactsQueued > 0 || r.ArtifactsBlocked > 0 || r.MissingPaths > 0
}

func (s *ArtifactMaintenanceService) inventoryLegacyTranscodeProjection(now time.Time) (legacyProjectionInventoryReport, error) {
	report := legacyProjectionInventoryReport{}
	if s == nil || s.repo == nil || s.repo.DB() == nil || s.cfg == nil || s.executionRepo == nil {
		return report, nil
	}
	batchSize := s.legacyMigrationBatchSize
	if batchSize <= 0 {
		batchSize = legacyProjectionDefaultBatchSize
	}

	highWater, err := s.repo.LegacyProjectionHighWater()
	if err != nil {
		return report, fmt.Errorf("read legacy projection high-water: %w", err)
	}
	var targetRows int64
	if highWater != nil {
		targetRows, err = s.repo.CountLegacyTerminalWithOutputThrough(*highWater)
		if err != nil {
			return report, fmt.Errorf("count legacy projection target rows: %w", err)
		}
	}
	state, _, err := s.executionRepo.PrepareLegacyProjectionMigration(
		repository.LegacyTranscodeArtifactMigrationSource,
		highWater,
		targetRows,
		batchSize,
		now,
		legacyProjectionSourceRetirementWindow,
	)
	if err != nil {
		return report, fmt.Errorf("prepare legacy projection migration: %w", err)
	}
	report = legacyProjectionReportFromState(state)
	if state == nil || state.Status == repository.LegacyProjectionMigrationCompleted || state.HighWaterUpdatedAt == nil {
		return report, nil
	}

	token := uuid.NewString()
	claimed, ok, err := s.executionRepo.ClaimLegacyProjectionMigration(
		state.Source,
		s.legacyMigrationOwner,
		token,
		now,
		legacyProjectionMigrationLease,
	)
	if err != nil {
		return report, fmt.Errorf("claim legacy projection migration: %w", err)
	}
	if !ok || claimed == nil {
		return report, nil
	}
	state = claimed

	after := legacyProjectionCursor(state.CursorUpdatedAt, state.CursorID)
	through := repository.LegacyProjectionCursor{UpdatedAt: *state.HighWaterUpdatedAt, ID: state.HighWaterID}
	tasks, err := s.repo.ListLegacyTerminalWithOutputAfter(after, through, state.BatchSize)
	if err != nil {
		return s.failLegacyProjectionBatch(state, token, now, fmt.Errorf("list legacy migration batch: %w", err))
	}

	delta := repository.LegacyProjectionBatchDelta{ScannedRows: int64(len(tasks))}
	for index := range tasks {
		item, importErr := s.importLegacyProjectionTask(&tasks[index], now)
		if importErr != nil {
			return s.failLegacyProjectionBatch(state, token, now, importErr)
		}
		delta.ImportedJobs += int64(item.JobsImported)
		delta.ArtifactsQueued += int64(item.ArtifactsQueued)
		delta.ArtifactsBlocked += int64(item.ArtifactsBlocked)
		delta.MissingPaths += int64(item.MissingPaths)
	}

	cursor := through
	if len(tasks) > 0 {
		last := tasks[len(tasks)-1]
		cursor = repository.LegacyProjectionCursor{UpdatedAt: last.UpdatedAt, ID: last.ID}
	}
	completed := len(tasks) == 0 || !repository.LegacyProjectionCursorAfter(through, cursor)
	stored, updated, err := s.executionRepo.CompleteLegacyProjectionMigrationBatch(
		state.Source,
		token,
		cursor,
		delta,
		completed,
		now,
		legacyProjectionSourceRetirementWindow,
	)
	if err != nil {
		return report, fmt.Errorf("complete legacy migration batch: %w", err)
	}
	if !updated || stored == nil {
		return report, fmt.Errorf("legacy migration Lease was lost before batch completion")
	}
	report = legacyProjectionReportFromState(stored)
	report.TasksFound = len(tasks)
	report.JobsImported = int(delta.ImportedJobs)
	report.ArtifactsQueued = int(delta.ArtifactsQueued)
	report.ArtifactsBlocked = int(delta.ArtifactsBlocked)
	report.MissingPaths = int(delta.MissingPaths)
	s.broadcastLegacyProjectionMigration(stored)
	return report, nil
}

func (s *ArtifactMaintenanceService) failLegacyProjectionBatch(state *model.LegacyTranscodeProjectionMigrationState, token string, now time.Time, cause error) (legacyProjectionInventoryReport, error) {
	if state == nil {
		return legacyProjectionInventoryReport{}, cause
	}
	backoff := legacyProjectionRetryBackoff(state.FailureCount + 1)
	stored, _, persistErr := s.executionRepo.FailLegacyProjectionMigrationBatch(
		state.Source,
		token,
		"legacy_projection_batch_failed",
		cause.Error(),
		now.Add(backoff),
		now,
	)
	if persistErr != nil {
		return legacyProjectionReportFromState(state), fmt.Errorf("%v; persist migration failure: %w", cause, persistErr)
	}
	if stored != nil {
		s.broadcastLegacyProjectionMigration(stored)
	}
	return legacyProjectionReportFromState(stored), cause
}

func legacyProjectionRetryBackoff(failureCount int) time.Duration {
	if failureCount < 1 {
		failureCount = 1
	}
	backoff := 30 * time.Second
	for i := 1; i < failureCount && backoff < 30*time.Minute; i++ {
		backoff *= 2
	}
	if backoff > 30*time.Minute {
		return 30 * time.Minute
	}
	return backoff
}

func legacyProjectionCursor(updatedAt *time.Time, id string) *repository.LegacyProjectionCursor {
	if updatedAt == nil {
		return nil
	}
	return &repository.LegacyProjectionCursor{UpdatedAt: *updatedAt, ID: id}
}

func legacyProjectionReportFromState(state *model.LegacyTranscodeProjectionMigrationState) legacyProjectionInventoryReport {
	if state == nil {
		return legacyProjectionInventoryReport{}
	}
	return legacyProjectionInventoryReport{
		Generation: state.Generation,
		ScannedRows: state.ScannedRows,
		TargetRows: state.TargetRows,
		Status: state.Status,
		HasMore: state.Status == repository.LegacyProjectionMigrationPending || state.Status == repository.LegacyProjectionMigrationRunning,
	}
}

func (s *ArtifactMaintenanceService) broadcastLegacyProjectionMigration(state *model.LegacyTranscodeProjectionMigrationState) {
	if s == nil || s.wsHub == nil || state == nil {
		return
	}
	s.wsHub.BroadcastEvent(EventTaskUpdated, map[string]any{
		"kind": TaskKindLegacyProjectionMigration,
		"status": state.Status,
		"source_id": state.Source,
		"generation": state.Generation,
		"scanned_rows": state.ScannedRows,
		"target_rows": state.TargetRows,
	})
}

func (s *ArtifactMaintenanceService) importLegacyProjectionTask(task *model.TranscodeTask, now time.Time) (legacyProjectionInventoryReport, error) {
	report := legacyProjectionInventoryReport{TasksFound: 1}
	if task == nil {
		return report, nil
	}
	root := filepath.Join(s.cfg.Cache.CacheDir, "transcode")
	db := s.repo.DB()
	job, imported, jobErr := ensureLegacyProjectionJob(db, task)
	if jobErr != nil {
		return report, jobErr
	}
	if imported {
		report.JobsImported++
	}

	artifactID := deterministicLegacyProjectionID("artifact", task.ID)
	var existing int64
	if err := db.Model(&model.TranscodeArtifactRecord{}).Where("id = ?", artifactID).Count(&existing).Error; err != nil {
		return report, fmt.Errorf("check legacy artifact %s: %w", artifactID, err)
	}
	if existing > 0 {
		return report, nil
	}

	rollbackUntil := now.Add(legacyProjectionRollbackWindow)
	outputDir := filepath.Clean(strings.TrimSpace(task.OutputDir))
	artifact := &model.TranscodeArtifactRecord{
		ID: artifactID, JobID: job.ID, MediaID: task.MediaID,
		Kind: repository.LegacyTranscodeArtifactKind, ProfileID: task.Quality,
		Path: outputDir, Status: "expired",
		MigrationSource: repository.LegacyTranscodeArtifactMigrationSource,
		CleanupState: repository.ArtifactCleanupPending,
		CleanupNextAttemptAt: &rollbackUntil,
		CleanupRollbackUntil: &rollbackUntil,
		CreatedAt: task.CreatedAt, UpdatedAt: now,
	}
	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = now
	}

	if !legacyProjectionPathAllowed(root, outputDir) {
		artifact.CleanupState = repository.ArtifactCleanupBlocked
		artifact.CleanupNextAttemptAt = nil
		artifact.CleanupErrorCode = "legacy_path_outside_store"
		artifact.CleanupErrorMessage = "legacy output directory is outside the managed transcode store"
		report.ArtifactsBlocked++
	} else if info, statErr := os.Stat(outputDir); errors.Is(statErr, os.ErrNotExist) {
		artifact.Status = "deleted"
		artifact.Path = ""
		artifact.CleanupState = repository.ArtifactCleanupCompleted
		artifact.CleanupCompletedAt = &now
		artifact.CleanupDisposition = "missing_at_inventory"
		artifact.CleanupOriginalPath = outputDir
		artifact.CleanupNextAttemptAt = nil
		artifact.CleanupRollbackUntil = nil
		report.MissingPaths++
	} else if statErr != nil {
		artifact.CleanupState = repository.ArtifactCleanupRetryWait
		artifact.CleanupErrorCode = "legacy_path_unavailable"
		artifact.CleanupErrorMessage = statErr.Error()
		report.ArtifactsBlocked++
	} else if !info.IsDir() {
		artifact.CleanupState = repository.ArtifactCleanupBlocked
		artifact.CleanupNextAttemptAt = nil
		artifact.CleanupErrorCode = "legacy_path_not_directory"
		artifact.CleanupErrorMessage = "legacy output path is not a directory"
		report.ArtifactsBlocked++
	} else {
		artifact.SizeBytes, _ = directorySize(outputDir)
		manifest := filepath.Join(outputDir, "stream.m3u8")
		if _, manifestErr := os.Stat(manifest); manifestErr == nil {
			artifact.ManifestPath = manifest
		}
		report.ArtifactsQueued++
	}

	if err := s.executionRepo.ImportLegacyHLSArtifact(artifact); err != nil {
		return report, fmt.Errorf("import legacy transcode artifact %s: %w", artifact.ID, err)
	}
	return report, nil
}

func ensureLegacyProjectionJob(db *gorm.DB, task *model.TranscodeTask) (*model.TranscodeJobRecord, bool, error) {
	var existing model.TranscodeJobRecord
	result := db.Where("legacy_task_id = ?", task.ID).Limit(1).Find(&existing)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		return &existing, false, nil
	}
	legacyID := task.ID
	completedAt := task.CompletedAt
	if completedAt == nil {
		value := task.UpdatedAt
		if value.IsZero() {
			value = time.Now()
		}
		completedAt = &value
	}
	status := "cancelled"
	switch strings.ToLower(strings.TrimSpace(task.Status)) {
	case "done", "completed":
		status = "completed"
	case "failed":
		status = "failed"
	}
	job := &model.TranscodeJobRecord{
		ID: deterministicLegacyProjectionID("job", task.ID), LegacyTaskID: &legacyID,
		MediaID: task.MediaID, Intent: "legacy_history_import", ProfileID: task.Quality,
		AudioTrack: -1, Status: status, DesiredState: "cancelled",
		CompletedAt: completedAt, CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt,
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = *completedAt
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = *completedAt
	}
	create := db.Clauses(clause.OnConflict{DoNothing: true}).Create(job)
	if create.Error != nil {
		return nil, false, fmt.Errorf("create legacy history job: %w", create.Error)
	}
	if create.RowsAffected == 0 {
		if err := db.Where("legacy_task_id = ?", task.ID).First(&existing).Error; err != nil {
			return nil, false, err
		}
		return &existing, false, nil
	}
	return job, true, nil
}

func deterministicLegacyProjectionID(kind, legacyTaskID string) string {
	return "legacy-" + kind + "-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(legacyTaskID)).String()
}

func legacyProjectionPathAllowed(root, candidate string) bool {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false
	}
	candidateAbs, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	first := strings.Split(relative, string(filepath.Separator))[0]
	switch first {
	case "artifacts", "workspaces", "ondemand":
		return false
	default:
		return true
	}
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.Walk(root, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
''')

# 5. Maintenance owner, batch size and structured logs.
path = "internal/service/artifact_maintenance.go"
content = read(path)
content = content.replace('"github.com/nowen-video/nowen-video/internal/config"', '"github.com/google/uuid"\n\t"github.com/nowen-video/nowen-video/internal/config"', 1)
old = '''	artifactStore *transcodeartifactstore.Store

	diskUsageMu'''
new = '''	artifactStore *transcodeartifactstore.Store

	legacyMigrationOwner     string
	legacyMigrationBatchSize int

	diskUsageMu'''
if new not in content:
    if content.count(old) != 1:
        raise RuntimeError("artifact maintenance field anchor mismatch")
    content = content.replace(old, new, 1)
old = '''		artifactStore: artifactStore,
		diskUsageTTL:  30 * time.Second,
'''
new = '''		artifactStore:             artifactStore,
		legacyMigrationOwner:     "artifact-maintenance-" + uuid.NewString(),
		legacyMigrationBatchSize: legacyProjectionDefaultBatchSize,
		diskUsageTTL:              30 * time.Second,
'''
if new not in content:
    if content.count(old) != 1:
        raise RuntimeError("artifact maintenance constructor anchor mismatch")
    content = content.replace(old, new, 1)
content = content.replace('logger.Infof("启动登记 Legacy 转码目录 tasks=%d jobs=%d queued=%d blocked=%d missing=%d", report.TasksFound, report.JobsImported, report.ArtifactsQueued, report.ArtifactsBlocked, report.MissingPaths)', 'logger.Infof("启动登记 Legacy 转码目录 generation=%d status=%s scanned=%d/%d batch=%d jobs=%d queued=%d blocked=%d missing=%d", report.Generation, report.Status, report.ScannedRows, report.TargetRows, report.TasksFound, report.JobsImported, report.ArtifactsQueued, report.ArtifactsBlocked, report.MissingPaths)', 1)
content = content.replace('s.logger.Infof("周期登记 Legacy 转码目录 tasks=%d jobs=%d queued=%d blocked=%d missing=%d", report.TasksFound, report.JobsImported, report.ArtifactsQueued, report.ArtifactsBlocked, report.MissingPaths)', 's.logger.Infof("周期登记 Legacy 转码目录 generation=%d status=%s scanned=%d/%d batch=%d jobs=%d queued=%d blocked=%d missing=%d", report.Generation, report.Status, report.ScannedRows, report.TargetRows, report.TasksFound, report.JobsImported, report.ArtifactsQueued, report.ArtifactsBlocked, report.MissingPaths)', 1)
write(path, content)

# 6. Operator retry command for failed migration batches.
path = "internal/service/transcode_cleanup_admin.go"
content = read(path)
append = r'''

var ErrLegacyProjectionMigrationNotRetryable = errors.New("legacy projection migration is not retryable")

func (s *ArtifactMaintenanceService) RetryLegacyProjectionMigration(source string) error {
	if s == nil || s.executionRepo == nil {
		return fmt.Errorf("Legacy Projection 迁移服务不可用")
	}
	if strings.TrimSpace(source) != repository.LegacyTranscodeArtifactMigrationSource {
		return fmt.Errorf("%w: source=%s", ErrLegacyProjectionMigrationNotRetryable, source)
	}
	requeued, err := s.executionRepo.RetryLegacyProjectionMigration(source, time.Now())
	if err != nil {
		return fmt.Errorf("重新排队 Legacy Projection 迁移失败: %w", err)
	}
	if !requeued {
		return fmt.Errorf("%w: source=%s", ErrLegacyProjectionMigrationNotRetryable, source)
	}
	_, runErr := s.inventoryLegacyTranscodeProjection(time.Now())
	if runErr != nil {
		stored, lookupErr := s.executionRepo.LegacyProjectionMigrationState(source)
		if lookupErr == nil && stored != nil && stored.Status == repository.LegacyProjectionMigrationFailed {
			s.logger.Warnf("管理员重试 Legacy Projection 迁移后仍失败 source=%s: %v", source, runErr)
			return nil
		}
		return runErr
	}
	return nil
}
'''
if "RetryLegacyProjectionMigration" not in content:
    content = content.replace('"fmt"\n\t"time"', '"fmt"\n\t"strings"\n\t"time"', 1)
    content += append
write(path, content)

# 7. Task Center exposes the migration generation itself.
path = "internal/service/task_center.go"
content = read(path)
content = content.replace('TaskKindLegacyArtifactMigration = "legacy_artifact_migration"', 'TaskKindLegacyArtifactMigration   = "legacy_artifact_migration"\n\tTaskKindLegacyProjectionMigration = "legacy_projection_migration"', 1)
anchor = '''	if s.executionRepo != nil {
		incidents, err := s.executionRepo.ListActiveStorageIncidents(limit)
'''
insert = '''	if s.executionRepo != nil {
		migration, err := s.executionRepo.LegacyProjectionMigrationState(repository.LegacyTranscodeArtifactMigrationSource)
		if err != nil {
			return nil, fmt.Errorf("read legacy projection migration: %w", err)
		}
		if migration != nil && (migration.TargetRows > 0 || migration.Status == repository.LegacyProjectionMigrationFailed || migration.Status == repository.LegacyProjectionMigrationRunning) {
			task := legacyProjectionMigrationToUnifiedTask(migration, now)
			if !activeOnly || isTaskActive(task.Status) {
				tasks = append(tasks, task)
			}
		}

		incidents, err := s.executionRepo.ListActiveStorageIncidents(limit)
'''
if insert not in content:
    if content.count(anchor) != 1:
        raise RuntimeError("task center migration insert anchor mismatch")
    content = content.replace(anchor, insert, 1)
func_anchor = 'func artifactCleanupToUnifiedTask(artifact *model.TranscodeArtifactRecord) UnifiedTask {'
new_func = r'''func legacyProjectionMigrationToUnifiedTask(state *model.LegacyTranscodeProjectionMigrationState, now time.Time) UnifiedTask {
	if state == nil {
		return UnifiedTask{}
	}
	status := TaskStatusQueued
	switch state.Status {
	case repository.LegacyProjectionMigrationRunning:
		status = TaskStatusRunning
	case repository.LegacyProjectionMigrationFailed:
		status = TaskStatusFailed
	case repository.LegacyProjectionMigrationCompleted:
		status = TaskStatusCompleted
	}
	progress := float64(0)
	if state.TargetRows > 0 {
		progress = float64(state.ScannedRows) / float64(state.TargetRows) * 100
		if progress > 100 {
			progress = 100
		}
	} else if status == TaskStatusCompleted {
		progress = 100
	}
	subtitle := fmt.Sprintf("第 %d 代 · %d/%d", state.Generation, state.ScannedRows, state.TargetRows)
	message := "按持久游标登记旧转码目录"
	if state.Status == repository.LegacyProjectionMigrationFailed {
		message = strings.TrimSpace(strings.Join([]string{state.LastErrorCode, state.LastErrorMessage}, " · "))
	} else if state.Status == repository.LegacyProjectionMigrationCompleted && state.SourceRetireAfter != nil {
		if now.Before(*state.SourceRetireAfter) {
			message = "迁移完成；旧表只读观察至 " + state.SourceRetireAfter.Format("2006-01-02 15:04")
		} else {
			message = "迁移完成；旧表已达到人工废弃评审时间"
		}
	} else if state.CursorUpdatedAt != nil {
		message = "当前游标 " + state.CursorUpdatedAt.Format("2006-01-02 15:04:05") + " / " + state.CursorID
	}
	return UnifiedTask{
		ID: TaskKindLegacyProjectionMigration + ":" + state.Source,
		Kind: TaskKindLegacyProjectionMigration,
		Status: status,
		Title: "旧转码历史登记",
		Subtitle: subtitle,
		Message: message,
		Progress: progress,
		SourceID: state.Source,
		CreatedAt: timePtr(state.CreatedAt),
		UpdatedAt: timePtr(state.UpdatedAt),
		StartedAt: state.LastBatchStartedAt,
		CompletedAt: state.CompletedAt,
	}
}

'''
if new_func not in content:
    if content.count(func_anchor) != 1:
        raise RuntimeError("task center function anchor mismatch")
    content = content.replace(func_anchor, new_func + func_anchor, 1)
write(path, content)

# 8. Task action protocol for failed migration generation.
path = "internal/service/task_actions.go"
content = read(path)
old = '''type artifactCleanupActions interface {
	RetryArtifactCleanup(artifactID string) error
	RollbackLegacyArtifactMigration(artifactID string) error
}
'''
new = old + '''
type legacyProjectionActions interface {
	RetryLegacyProjectionMigration(source string) error
}
'''
if new not in content:
    if content.count(old) != 1:
        raise RuntimeError("task action interface anchor mismatch")
    content = content.replace(old, new, 1)
content = content.replace('\tartifactCleanup artifactCleanupActions\n', '\tartifactCleanup artifactCleanupActions\n\tlegacyProjection legacyProjectionActions\n', 1)
old = '''	if maintenance != nil {
		dispatcher.artifactCleanup = maintenance
		dispatcher.artifactLookup = maintenance.executionRepo
	}
'''
new = '''	if maintenance != nil {
		dispatcher.artifactCleanup = maintenance
		dispatcher.legacyProjection = maintenance
		dispatcher.artifactLookup = maintenance.executionRepo
	}
'''
if new not in content:
    if content.count(old) != 1:
        raise RuntimeError("task action constructor anchor mismatch")
    content = content.replace(old, new, 1)
anchor = '''	case TaskKindLegacyArtifactMigration:
		switch normalizedStatus {
'''
insert = '''	case TaskKindLegacyProjectionMigration:
		if normalizedStatus == TaskStatusFailed {
			return []string{TaskActionRetry}
		}
	case TaskKindLegacyArtifactMigration:
		switch normalizedStatus {
'''
if insert not in content:
    if content.count(anchor) != 1:
        raise RuntimeError("task available actions anchor mismatch")
    content = content.replace(anchor, insert, 1)
anchor = '''	case TaskKindLegacyArtifactMigration:
		err = d.executeArtifactCleanup(sourceID, action, true)
	case TaskKindScan, TaskKindStorageIncident:
'''
insert = '''	case TaskKindLegacyArtifactMigration:
		err = d.executeArtifactCleanup(sourceID, action, true)
	case TaskKindLegacyProjectionMigration:
		err = d.executeLegacyProjectionMigration(sourceID, action)
	case TaskKindScan, TaskKindStorageIncident:
'''
if insert not in content:
    if content.count(anchor) != 1:
        raise RuntimeError("task execute switch anchor mismatch")
    content = content.replace(anchor, insert, 1)
func_anchor = 'func (d *TaskActionDispatcher) executeArtifactCleanup(sourceID, action string, legacyMigration bool) error {'
new_func = r'''func (d *TaskActionDispatcher) executeLegacyProjectionMigration(sourceID, action string) error {
	if action != TaskActionRetry {
		return fmt.Errorf("%w: legacy projection action=%s", ErrTaskActionUnsupported, action)
	}
	if d.legacyProjection == nil || d.artifactLookup == nil {
		return fmt.Errorf("Legacy Projection 迁移执行器不可用")
	}
	state, err := d.artifactLookup.(*repository.TranscodeExecutionRepo).LegacyProjectionMigrationState(sourceID)
	if err != nil || state == nil {
		return fmt.Errorf("%w: legacy projection %s", ErrTaskNotFound, sourceID)
	}
	if state.Status != repository.LegacyProjectionMigrationFailed {
		return fmt.Errorf("%w: legacy projection status=%s", ErrTaskActionConflict, state.Status)
	}
	if err := d.legacyProjection.RetryLegacyProjectionMigration(sourceID); err != nil {
		if errors.Is(err, ErrLegacyProjectionMigrationNotRetryable) {
			return fmt.Errorf("%w: legacy projection status changed", ErrTaskActionConflict)
		}
		return fmt.Errorf("重试 Legacy Projection 迁移失败: %w", err)
	}
	return nil
}

'''
# Avoid unsafe type assertion by extending lookup interface instead.
content = content.replace('type artifactCleanupLookup interface {\n\tFindArtifactCleanupOperation(id string) (*model.TranscodeArtifactRecord, error)\n}', 'type artifactCleanupLookup interface {\n\tFindArtifactCleanupOperation(id string) (*model.TranscodeArtifactRecord, error)\n\tLegacyProjectionMigrationState(source string) (*model.LegacyTranscodeProjectionMigrationState, error)\n}', 1)
new_func = new_func.replace('state, err := d.artifactLookup.(*repository.TranscodeExecutionRepo).LegacyProjectionMigrationState(sourceID)', 'state, err := d.artifactLookup.LegacyProjectionMigrationState(sourceID)')
if new_func not in content:
    if content.count(func_anchor) != 1:
        raise RuntimeError("task action method anchor mismatch")
    content = content.replace(func_anchor, new_func + func_anchor, 1)
# Update task action message switch if present.
content = content.replace('case TaskKindLegacyArtifactMigration:', 'case TaskKindLegacyProjectionMigration:\n\t\tif action == TaskActionRetry {\n\t\t\treturn "旧转码历史登记已重新排队"\n\t\t}\n\tcase TaskKindLegacyArtifactMigration:', 1) if 'func taskActionMessage' in content else content
write(path, content)

# Existing fake lookup must satisfy the expanded lookup interface.
path = "internal/service/task_actions_test.go"
content = read(path)
anchor = '''func (f *fakeArtifactLookup) FindArtifactCleanupOperation(string) (*model.TranscodeArtifactRecord, error) {
'''
method = '''func (f *fakeArtifactLookup) LegacyProjectionMigrationState(string) (*model.LegacyTranscodeProjectionMigrationState, error) {
	return nil, errors.New("not found")
}

'''
if method not in content:
    if content.count(anchor) != 1:
        raise RuntimeError("fake artifact lookup anchor mismatch")
    content = content.replace(anchor, method + anchor, 1)
write(path, content)

# 9. Runtime History exposes migration progress and source retirement window.
path = "internal/repository/runtime_history.go"
content = read(path)
content = content.replace('\tNewestAt          *time.Time\n}', '\tNewestAt          *time.Time\n\tLegacyMigration   *model.LegacyTranscodeProjectionMigrationState\n}', 1)
anchor = '''	if r.db.Migrator().HasTable(&model.TranscodeTask{}) {
'''
insert = '''	if migration, err := NewTranscodeExecutionRepo(r.db).LegacyProjectionMigrationState(LegacyTranscodeArtifactMigrationSource); err != nil {
		return nil, err
	} else {
		counts.LegacyMigration = migration
	}
	if r.db.Migrator().HasTable(&model.TranscodeTask{}) {
'''
if insert not in content:
    if content.count(anchor) != 1:
        raise RuntimeError("runtime history migration anchor mismatch")
    content = content.replace(anchor, insert, 1)
write(path, content)

path = "internal/service/runtime_history.go"
content = read(path)
summary_anchor = 'type RuntimeHistorySummary struct {\n'
summary_type = r'''type RuntimeHistoryLegacyMigration struct {
	Source             string     `json:"source"`
	Generation         int64      `json:"generation"`
	Status             string     `json:"status"`
	TargetRows         int64      `json:"target_rows"`
	ScannedRows        int64      `json:"scanned_rows"`
	ImportedJobs       int64      `json:"imported_jobs"`
	ArtifactsQueued    int64      `json:"artifacts_queued"`
	ArtifactsBlocked   int64      `json:"artifacts_blocked"`
	MissingPaths       int64      `json:"missing_paths"`
	FailureCount       int        `json:"failure_count"`
	LastErrorCode      string     `json:"last_error_code,omitempty"`
	LastErrorMessage   string     `json:"last_error_message,omitempty"`
	CursorUpdatedAt    *time.Time `json:"cursor_updated_at,omitempty"`
	CursorID           string     `json:"cursor_id,omitempty"`
	HighWaterUpdatedAt *time.Time `json:"high_water_updated_at,omitempty"`
	HighWaterID        string     `json:"high_water_id,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	SourceRetireAfter  *time.Time `json:"source_retire_after,omitempty"`
	RetirementEligible bool       `json:"retirement_eligible"`
}

'''
if summary_type not in content:
    if content.count(summary_anchor) != 1:
        raise RuntimeError("runtime history summary type anchor mismatch")
    content = content.replace(summary_anchor, summary_type + summary_anchor, 1)
content = content.replace('\tRetention         RuntimeHistoryRetentionPolicy `json:"retention"`\n}', '\tRetention         RuntimeHistoryRetentionPolicy `json:"retention"`\n\tLegacyMigration   *RuntimeHistoryLegacyMigration `json:"legacy_migration,omitempty"`\n}', 1)
old = '''	return &RuntimeHistorySummary{
		Jobs: counts.Jobs, Attempts: counts.Attempts, Artifacts: counts.Artifacts,
		LegacyTasks: counts.LegacyTasks, OrphanLegacyTasks: counts.OrphanLegacyTasks,
		ArtifactBytes: counts.ArtifactBytes, ByStatus: counts.ByStatus,
		OldestAt: counts.OldestAt, NewestAt: counts.NewestAt,
		Generated: time.Now(), Retention: RuntimeHistoryRetention(),
	}, nil
'''
new = '''	summary := &RuntimeHistorySummary{
		Jobs: counts.Jobs, Attempts: counts.Attempts, Artifacts: counts.Artifacts,
		LegacyTasks: counts.LegacyTasks, OrphanLegacyTasks: counts.OrphanLegacyTasks,
		ArtifactBytes: counts.ArtifactBytes, ByStatus: counts.ByStatus,
		OldestAt: counts.OldestAt, NewestAt: counts.NewestAt,
		Generated: time.Now(), Retention: RuntimeHistoryRetention(),
	}
	if migration := counts.LegacyMigration; migration != nil {
		summary.LegacyMigration = &RuntimeHistoryLegacyMigration{
			Source: migration.Source, Generation: migration.Generation, Status: migration.Status,
			TargetRows: migration.TargetRows, ScannedRows: migration.ScannedRows,
			ImportedJobs: migration.ImportedJobs, ArtifactsQueued: migration.ArtifactsQueued,
			ArtifactsBlocked: migration.ArtifactsBlocked, MissingPaths: migration.MissingPaths,
			FailureCount: migration.FailureCount, LastErrorCode: migration.LastErrorCode,
			LastErrorMessage: truncateRuntimeHistoryText(migration.LastErrorMessage),
			CursorUpdatedAt: migration.CursorUpdatedAt, CursorID: migration.CursorID,
			HighWaterUpdatedAt: migration.HighWaterUpdatedAt, HighWaterID: migration.HighWaterID,
			CompletedAt: migration.CompletedAt, SourceRetireAfter: migration.SourceRetireAfter,
			RetirementEligible: migration.SourceRetireAfter != nil && !time.Now().Before(*migration.SourceRetireAfter),
		}
	}
	return summary, nil
'''
if new not in content:
    if content.count(old) != 1:
        raise RuntimeError("runtime history summary mapping anchor mismatch")
    content = content.replace(old, new, 1)
write(path, content)

# 10. Web contracts and labels.
path = "web/src/api/tasks.ts"
content = read(path)
content = content.replace("'legacy_artifact_migration' | 'storage_incident'", "'legacy_artifact_migration' | 'legacy_projection_migration' | 'storage_incident'", 1)
write(path, content)

path = "web/src/components/TaskCenter.tsx"
content = read(path)
content = content.replace("legacy_artifact_migration: '旧转码目录迁移',", "legacy_artifact_migration: '旧转码目录迁移',\n  legacy_projection_migration: '旧转码历史登记',", 1)
content = content.replace("if (kind === 'artifact_cleanup' || kind === 'legacy_artifact_migration') return <HardDrive size={17} />", "if (kind === 'artifact_cleanup' || kind === 'legacy_artifact_migration') return <HardDrive size={17} />\n  if (kind === 'legacy_projection_migration') return <Database size={17} />", 1)
content = content.replace("const cleanupTask = task.kind === 'artifact_cleanup' || task.kind === 'legacy_artifact_migration'", "const cleanupTask = task.kind === 'artifact_cleanup' || task.kind === 'legacy_artifact_migration'\n  const projectionMigrationTask = task.kind === 'legacy_projection_migration'", 1)
content = content.replace("{cleanupTask ? '立即重试' : '重试'}", "{cleanupTask ? '立即重试' : projectionMigrationTask ? '重新登记' : '重试'}", 1)
write(path, content)

path = "web/src/api/runtimeHistory.ts"
content = read(path)
legacy_type = r'''export interface RuntimeHistoryLegacyMigration {
  source: string
  generation: number
  status: string
  target_rows: number
  scanned_rows: number
  imported_jobs: number
  artifacts_queued: number
  artifacts_blocked: number
  missing_paths: number
  failure_count: number
  last_error_code?: string
  last_error_message?: string
  cursor_updated_at?: string
  cursor_id?: string
  high_water_updated_at?: string
  high_water_id?: string
  completed_at?: string
  source_retire_after?: string
  retirement_eligible: boolean
}

'''
anchor = 'export interface RuntimeHistorySummary {\n'
if legacy_type not in content:
    if content.count(anchor) != 1:
        raise RuntimeError("web runtime history type anchor mismatch")
    content = content.replace(anchor, legacy_type + anchor, 1)
content = content.replace('  retention: RuntimeHistoryRetentionPolicy\n}', '  retention: RuntimeHistoryRetentionPolicy\n  legacy_migration?: RuntimeHistoryLegacyMigration\n}', 1)
write(path, content)

# 11. Focused tests.
write("internal/model/legacy_transcode_projection_state_test.go", r'''package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestTranscodeExecutionMigrationCreatesLegacyProjectionStateOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:legacy-projection-state?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&LegacyTranscodeProjectionMigrationState{}) {
		t.Fatal("migration state table missing")
	}
	if db.Migrator().HasTable(&TranscodeTask{}) {
		t.Fatal("execution migration recreated legacy transcode_tasks")
	}
}
''')

write("internal/repository/repo_legacy_transcode_projection_migration_test.go", r'''package repository

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
''')

write("internal/service/legacy_transcode_projection_cursor_test.go", r'''package service

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
		Source: repository.LegacyTranscodeArtifactMigrationSource,
		Generation: 3,
		Status: repository.LegacyProjectionMigrationRunning,
		TargetRows: 10,
		ScannedRows: 4,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
	}
	if err := service.executionRepo.DB().Create(state).Error; err != nil {
		t.Fatal(err)
	}
	task := legacyProjectionMigrationToUnifiedTask(state, now)
	if task.Kind != TaskKindLegacyProjectionMigration || task.Status != TaskStatusRunning || task.Progress != 40 {
		t.Fatalf("task=%+v", task)
	}
}
''')

# Add DB accessor only if repository does not already expose it.
path = "internal/repository/repo_transcode_execution.go"
content = read(path)
if 'func (r *TranscodeExecutionRepo) DB() *gorm.DB' not in content:
    anchor = 'func NewTranscodeExecutionRepo(db *gorm.DB) *TranscodeExecutionRepo {'
    idx = content.index(anchor)
    # insert after constructor block by locating its closing pattern
    constructor_end = content.index('\n}\n', idx) + 3
    content = content[:constructor_end] + '\nfunc (r *TranscodeExecutionRepo) DB() *gorm.DB { return r.db }\n' + content[constructor_end:]
write(path, content)

# 12. Source-level regression and docs.
write("cmd/server/legacy_projection_cursor_test.go", r'''package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyProjectionMigrationKeepsDurableCursorBoundary(t *testing.T) {
	root := filepath.Join("..", "..")
	checks := map[string][]string{
		"internal/model/transcode_execution.go": {"legacy_transcode_projection_migrations", "HighWaterUpdatedAt", "SourceRetireAfter"},
		"internal/repository/repo_legacy_transcode_projection_migration.go": {"ClaimLegacyProjectionMigration", "CompleteLegacyProjectionMigrationBatch", "RetryLegacyProjectionMigration"},
		"internal/service/legacy_transcode_projection_migration.go": {"ListLegacyTerminalWithOutputAfter", "legacyProjectionSourceRetirementWindow", "legacyProjectionMigrationLease"},
	}
	for name, needles := range checks {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		for _, needle := range needles {
			if !strings.Contains(text, needle) {
				t.Fatalf("%s missing %s", name, needle)
			}
		}
	}
	serviceContent, err := os.ReadFile(filepath.Join(root, "internal/service/legacy_transcode_projection_migration.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serviceContent), "ListLegacyTerminalWithOutput(500)") {
		t.Fatal("legacy migration reintroduced head-of-table rescans")
	}
}
''')

path = "docs/LEGACY_TRANSCODE_PROJECTION_RETIREMENT.md"
content = read(path)
section = r'''

## Cursor generations and source retirement

The inventory source is no longer scanned from the beginning on every maintenance
interval. `legacy_transcode_projection_migrations` stores one durable state row
for `legacy_transcode_task_v1` with:

- a finite per-generation source high-water
- the last committed `(updated_at, id)` cursor
- cumulative row and Artifact counters
- a database Lease for multi-instance ownership
- persisted failure evidence and exponential retry time
- a 30-day read-only source retirement review date

A batch advances the cursor only after every row in that batch has been imported.
Partial Job or Artifact writes are safe because their identifiers are deterministic
and the batch is replayed after a failure. A completed generation performs only a
high-water check. If a rollback to an older server creates newer legacy rows, the
next upgrade opens a new generation beginning at the previous high-water.

The retirement date is evidence for an explicit later schema-removal decision; it
does not automatically drop `transcode_tasks`.
'''
if "## Cursor generations and source retirement" not in content:
    content += section
write(path, content)
