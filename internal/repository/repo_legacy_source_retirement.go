package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

type LegacySourceRetirementInventory struct {
	SourceTablePresent      bool       `json:"source_table_present"`
	SourceRows              int64      `json:"source_rows"`
	UnmigratedRows          int64      `json:"unmigrated_rows"`
	RollbackOpenArtifacts   int64      `json:"rollback_open_artifacts"`
	RollbackLatestUntil     *time.Time `json:"rollback_latest_until,omitempty"`
}

func (r *TranscodeExecutionRepo) EnsureLegacySourceRetirementSchema() error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return model.AutoMigrateLegacySourceRetirement(r.db)
}

// LegacySourceRetirementInventory reads current evidence directly from both
// sides of the migration boundary. The query is deliberately read-only and
// never mutates or drops the legacy source table.
func (r *TranscodeExecutionRepo) LegacySourceRetirementInventory(source string, now time.Time) (LegacySourceRetirementInventory, error) {
	inventory := LegacySourceRetirementInventory{}
	if r == nil || r.db == nil {
		return inventory, gorm.ErrInvalidDB
	}

	inventory.SourceTablePresent = r.db.Migrator().HasTable(&model.TranscodeTask{})
	if inventory.SourceTablePresent {
		if err := r.db.Table((model.TranscodeTask{}).TableName()).Count(&inventory.SourceRows).Error; err != nil {
			return inventory, err
		}
		if err := r.db.Raw(`
			SELECT COUNT(*)
			FROM transcode_tasks AS legacy
			WHERE TRIM(COALESCE(legacy.output_dir, '')) <> ''
			  AND NOT EXISTS (
				SELECT 1
				FROM transcode_jobs AS job
				WHERE job.legacy_task_id = legacy.id
			  )
		`).Scan(&inventory.UnmigratedRows).Error; err != nil {
			return inventory, err
		}
	}

	if err := r.db.Model(&model.TranscodeArtifactRecord{}).
		Where("migration_source = ?", source).
		Where("cleanup_rollback_until IS NOT NULL AND cleanup_rollback_until >= ?", now).
		Where("cleanup_state NOT IN ?", []string{ArtifactCleanupCompleted, ArtifactCleanupRollbackCompleted}).
		Count(&inventory.RollbackOpenArtifacts).Error; err != nil {
		return inventory, err
	}

	type latestRollback struct {
		Latest *time.Time `gorm:"column:latest"`
	}
	var latest latestRollback
	if err := r.db.Model(&model.TranscodeArtifactRecord{}).
		Select("MAX(cleanup_rollback_until) AS latest").
		Where("migration_source = ?", source).
		Scan(&latest).Error; err != nil {
		return inventory, err
	}
	inventory.RollbackLatestUntil = latest.Latest
	return inventory, nil
}

func (r *TranscodeExecutionRepo) CreateLegacySourceRetirementDecision(record *model.LegacySourceRetirementDecisionRecord) error {
	if record == nil {
		return nil
	}
	if err := r.EnsureLegacySourceRetirementSchema(); err != nil {
		return err
	}
	return r.db.Create(record).Error
}

func (r *TranscodeExecutionRepo) LatestLegacySourceRetirementDecision(source string) (*model.LegacySourceRetirementDecisionRecord, error) {
	if err := r.EnsureLegacySourceRetirementSchema(); err != nil {
		return nil, err
	}
	var record model.LegacySourceRetirementDecisionRecord
	result := r.db.Where("source = ?", source).
		Order("reviewed_at DESC, created_at DESC, id DESC").
		Limit(1).
		Find(&record)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &record, nil
}

func (r *TranscodeExecutionRepo) ListLegacySourceRetirementDecisions(source string, limit int) ([]model.LegacySourceRetirementDecisionRecord, error) {
	if err := r.EnsureLegacySourceRetirementSchema(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	var records []model.LegacySourceRetirementDecisionRecord
	err := r.db.Where("source = ?", source).
		Order("reviewed_at DESC, created_at DESC, id DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}
