package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// legacyProjectionSourceQuery covers every legacy row. Jobs are the complete
// audit projection; Artifact creation remains conditional on a real output path.
func (r *TranscodeRepo) legacyProjectionSourceQuery() *gorm.DB {
	return r.db.Model(&model.TranscodeTask{})
}

func (r *TranscodeRepo) LegacyProjectionSourceHighWater() (*LegacyProjectionCursor, error) {
	if !r.LegacyTableExists() {
		return nil, nil
	}
	var row struct {
		UpdatedAt time.Time
		ID        string
	}
	result := r.legacyProjectionSourceQuery().
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

func (r *TranscodeRepo) CountLegacyProjectionSourceThrough(highWater LegacyProjectionCursor) (int64, error) {
	if !r.LegacyTableExists() {
		return 0, nil
	}
	var count int64
	err := r.legacyProjectionSourceQuery().
		Where("updated_at < ? OR (updated_at = ? AND id <= ?)", highWater.UpdatedAt, highWater.UpdatedAt, highWater.ID).
		Count(&count).Error
	return count, err
}

func (r *TranscodeRepo) ListLegacyProjectionSourceAfter(after *LegacyProjectionCursor, highWater LegacyProjectionCursor, limit int) ([]model.TranscodeTask, error) {
	if !r.LegacyTableExists() {
		return []model.TranscodeTask{}, nil
	}
	if limit <= 0 {
		limit = 250
	}
	if limit > 2000 {
		limit = 2000
	}
	query := r.legacyProjectionSourceQuery().
		Where("updated_at < ? OR (updated_at = ? AND id <= ?)", highWater.UpdatedAt, highWater.UpdatedAt, highWater.ID)
	if after != nil {
		query = query.Where("updated_at > ? OR (updated_at = ? AND id > ?)", after.UpdatedAt, after.UpdatedAt, after.ID)
	}
	var tasks []model.TranscodeTask
	err := query.Order("updated_at ASC, id ASC").Limit(limit).Find(&tasks).Error
	return tasks, err
}

// ReopenLegacyProjectionForFullSource upgrades a completed v1 directory-only
// projection to the full-row audit projection. Deterministic Job and Artifact
// IDs make the one-time replay safe. The legacy table remains read-only.
func (r *TranscodeExecutionRepo) ReopenLegacyProjectionForFullSource(
	source string,
	highWater *LegacyProjectionCursor,
	targetRows int64,
	batchSize int,
	now time.Time,
) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, gorm.ErrInvalidDB
	}
	if batchSize <= 0 {
		batchSize = 250
	}
	var state model.LegacyTranscodeProjectionMigrationState
	reopened := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("source = ?", source).
			Limit(1).
			Find(&state)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		if state.Status != LegacyProjectionMigrationCompleted || state.TargetRows == targetRows {
			return nil
		}

		updates := map[string]any{
			"generation":              state.Generation + 1,
			"status":                  LegacyProjectionMigrationPending,
			"cursor_updated_at":       nil,
			"cursor_id":               "",
			"target_rows":             targetRows,
			"scanned_rows":            int64(0),
			"imported_jobs":           int64(0),
			"artifacts_queued":        int64(0),
			"artifacts_blocked":       int64(0),
			"missing_paths":           int64(0),
			"batch_size":              batchSize,
			"next_attempt_at":         now,
			"next_source_check_at":    nil,
			"consecutive_failures":    int64(0),
			"last_error_code":         "",
			"last_error_message":      "",
			"completed_at":            nil,
			"quiescent_since":         nil,
			"source_retire_after":     nil,
			"lease_owner":             "",
			"lease_token":             "",
			"lease_expires_at":        nil,
			"last_batch_started_at":   nil,
			"last_batch_completed_at": nil,
			"updated_at":              now,
		}
		if highWater == nil {
			updates["high_water_updated_at"] = nil
			updates["high_water_id"] = ""
		} else {
			updates["high_water_updated_at"] = highWater.UpdatedAt
			updates["high_water_id"] = highWater.ID
		}
		result = tx.Model(&model.LegacyTranscodeProjectionMigrationState{}).
			Where("source = ? AND status = ? AND target_rows = ?", source, LegacyProjectionMigrationCompleted, state.TargetRows).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		reopened = true
		return tx.Where("source = ?", source).First(&state).Error
	})
	return &state, reopened, err
}
