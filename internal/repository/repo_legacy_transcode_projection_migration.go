package repository

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
	if r == nil || r.db == nil || !r.db.Migrator().HasTable(&model.LegacyTranscodeProjectionMigrationState{}) {
		return nil, nil
	}
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
					"status":              LegacyProjectionMigrationCompleted,
					"target_rows":         int64(0),
					"completed_at":        now,
					"quiescent_since":     now,
					"source_retire_after": retireAfter,
					"next_attempt_at":     nil,
					"updated_at":          now,
				}
			}
		} else if state.HighWaterUpdatedAt == nil {
			updates = map[string]any{
				"generation":            int64(1),
				"status":                LegacyProjectionMigrationPending,
				"high_water_updated_at": highWater.UpdatedAt,
				"high_water_id":         highWater.ID,
				"target_rows":           targetRows,
				"batch_size":            batchSize,
				"next_attempt_at":       now,
				"completed_at":          nil,
				"quiescent_since":       nil,
				"source_retire_after":   nil,
				"updated_at":            now,
			}
		} else {
			currentHigh := LegacyProjectionCursor{UpdatedAt: *state.HighWaterUpdatedAt, ID: state.HighWaterID}
			if state.Status == LegacyProjectionMigrationCompleted && LegacyProjectionCursorAfter(*highWater, currentHigh) {
				updates = map[string]any{
					"generation":            state.Generation + 1,
					"status":                LegacyProjectionMigrationPending,
					"cursor_updated_at":     currentHigh.UpdatedAt,
					"cursor_id":             currentHigh.ID,
					"high_water_updated_at": highWater.UpdatedAt,
					"high_water_id":         highWater.ID,
					"target_rows":           targetRows,
					"batch_size":            batchSize,
					"next_attempt_at":       now,
					"last_error_code":       "",
					"last_error_message":    "",
					"completed_at":          nil,
					"quiescent_since":       nil,
					"source_retire_after":   nil,
					"updated_at":            now,
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
			"status":                LegacyProjectionMigrationRunning,
			"lease_owner":           owner,
			"lease_token":           token,
			"lease_expires_at":      now.Add(leaseDuration),
			"last_batch_started_at": now,
			"updated_at":            now,
		})
	if result.Error != nil || result.RowsAffected != 1 {
		return nil, false, result.Error
	}
	state, err := r.LegacyProjectionMigrationState(source)
	return state, state != nil, err
}

func (r *TranscodeExecutionRepo) CompleteLegacyProjectionMigrationBatch(source, token string, cursor LegacyProjectionCursor, delta LegacyProjectionBatchDelta, completed bool, now time.Time, retirementWindow time.Duration) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {
	updates := map[string]any{
		"cursor_updated_at":       cursor.UpdatedAt,
		"cursor_id":               cursor.ID,
		"scanned_rows":            gorm.Expr("scanned_rows + ?", delta.ScannedRows),
		"imported_jobs":           gorm.Expr("imported_jobs + ?", delta.ImportedJobs),
		"artifacts_queued":        gorm.Expr("artifacts_queued + ?", delta.ArtifactsQueued),
		"artifacts_blocked":       gorm.Expr("artifacts_blocked + ?", delta.ArtifactsBlocked),
		"missing_paths":           gorm.Expr("missing_paths + ?", delta.MissingPaths),
		"last_batch_completed_at": now,
		"last_error_code":         "",
		"last_error_message":      "",
		"lease_owner":             "",
		"lease_token":             "",
		"lease_expires_at":        nil,
		"updated_at":              now,
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
			"status":                  LegacyProjectionMigrationFailed,
			"failure_count":           gorm.Expr("failure_count + 1"),
			"last_error_code":         code,
			"last_error_message":      message,
			"next_attempt_at":         nextAttemptAt,
			"last_batch_completed_at": now,
			"lease_owner":             "",
			"lease_token":             "",
			"lease_expires_at":        nil,
			"updated_at":              now,
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
			"status":             LegacyProjectionMigrationPending,
			"next_attempt_at":    now,
			"last_error_code":    "",
			"last_error_message": "",
			"lease_owner":        "",
			"lease_token":        "",
			"lease_expires_at":   nil,
			"updated_at":         now,
		})
	return result.RowsAffected == 1, result.Error
}
