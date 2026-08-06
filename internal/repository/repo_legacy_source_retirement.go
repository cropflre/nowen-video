package repository

import (
	"sort"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

type LegacySourceRetirementInventory struct {
	SourceTablePresent    bool       `json:"source_table_present"`
	SourceRows            int64      `json:"source_rows"`
	UnmigratedRows        int64      `json:"unmigrated_rows"`
	RollbackOpenArtifacts int64      `json:"rollback_open_artifacts"`
	RollbackLatestUntil   *time.Time `json:"rollback_latest_until,omitempty"`
}

type LegacySourceColumnSnapshot struct {
	Name          string `json:"name"`
	DatabaseType  string `json:"database_type"`
	ColumnType    string `json:"column_type,omitempty"`
	PrimaryKey    *bool  `json:"primary_key,omitempty"`
	AutoIncrement *bool  `json:"auto_increment,omitempty"`
	Nullable      *bool  `json:"nullable,omitempty"`
	Unique        *bool  `json:"unique,omitempty"`
	Length        *int64 `json:"length,omitempty"`
	Precision     *int64 `json:"precision,omitempty"`
	Scale         *int64 `json:"scale,omitempty"`
	DefaultValue  string `json:"default_value,omitempty"`
}

type LegacySourceSchemaSnapshot struct {
	TableName    string                       `json:"table_name"`
	TablePresent bool                         `json:"table_present"`
	Columns      []LegacySourceColumnSnapshot `json:"columns"`
}

func (r *TranscodeExecutionRepo) EnsureLegacySourceRetirementSchema() error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return model.AutoMigrateLegacySourceRetirement(r.db)
}

// LegacySourceRetirementInventory reads current evidence directly from both
// sides of the migration boundary. Every source row must have a matching Job;
// rows without output directories still carry rollback and audit meaning.
// The query is read-only and never mutates or drops the legacy source table.
func (r *TranscodeExecutionRepo) LegacySourceRetirementInventory(source string, now time.Time) (LegacySourceRetirementInventory, error) {
	inventory := LegacySourceRetirementInventory{}
	if r == nil || r.db == nil {
		return inventory, gorm.ErrInvalidDB
	}

	inventory.SourceTablePresent = r.db.Migrator().HasTable(&model.TranscodeTask{})
	if inventory.SourceTablePresent {
		if err := r.db.Table("transcode_tasks").Count(&inventory.SourceRows).Error; err != nil {
			return inventory, err
		}
		if err := r.db.Raw(`
			SELECT COUNT(*)
			FROM transcode_tasks AS legacy
			WHERE NOT EXISTS (
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

// LegacySourceSchemaSnapshot returns a portable, stable description of the
// legacy table shape. It is evidence for a future removal migration, not DDL.
func (r *TranscodeExecutionRepo) LegacySourceSchemaSnapshot() (LegacySourceSchemaSnapshot, error) {
	snapshot := LegacySourceSchemaSnapshot{
		TableName: "transcode_tasks",
		Columns:   make([]LegacySourceColumnSnapshot, 0),
	}
	if r == nil || r.db == nil {
		return snapshot, gorm.ErrInvalidDB
	}
	snapshot.TablePresent = r.db.Migrator().HasTable(&model.TranscodeTask{})
	if !snapshot.TablePresent {
		return snapshot, nil
	}
	columnTypes, err := r.db.Migrator().ColumnTypes("transcode_tasks")
	if err != nil {
		return snapshot, err
	}
	for _, column := range columnTypes {
		item := LegacySourceColumnSnapshot{
			Name:         column.Name(),
			DatabaseType: column.DatabaseTypeName(),
		}
		if value, ok := column.ColumnType(); ok {
			item.ColumnType = value
		}
		if value, ok := column.PrimaryKey(); ok {
			item.PrimaryKey = boolPointer(value)
		}
		if value, ok := column.AutoIncrement(); ok {
			item.AutoIncrement = boolPointer(value)
		}
		if value, ok := column.Nullable(); ok {
			item.Nullable = boolPointer(value)
		}
		if value, ok := column.Unique(); ok {
			item.Unique = boolPointer(value)
		}
		if value, ok := column.Length(); ok {
			item.Length = int64Pointer(value)
		}
		if precision, scale, ok := column.DecimalSize(); ok {
			item.Precision = int64Pointer(precision)
			item.Scale = int64Pointer(scale)
		}
		if value, ok := column.DefaultValue(); ok {
			item.DefaultValue = value
		}
		snapshot.Columns = append(snapshot.Columns, item)
	}
	sort.Slice(snapshot.Columns, func(i, j int) bool {
		return snapshot.Columns[i].Name < snapshot.Columns[j].Name
	})
	return snapshot, nil
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

func (r *TranscodeExecutionRepo) CreateLegacySourceRemovalPlan(record *model.LegacySourceRemovalPlanRecord) error {
	if record == nil {
		return nil
	}
	if err := r.EnsureLegacySourceRetirementSchema(); err != nil {
		return err
	}
	return r.db.Create(record).Error
}

func (r *TranscodeExecutionRepo) LatestLegacySourceRemovalPlan(source string) (*model.LegacySourceRemovalPlanRecord, error) {
	if err := r.EnsureLegacySourceRetirementSchema(); err != nil {
		return nil, err
	}
	var record model.LegacySourceRemovalPlanRecord
	result := r.db.Where("source = ?", source).
		Order("prepared_at DESC, created_at DESC, id DESC").
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

func boolPointer(value bool) *bool {
	return &value
}

func int64Pointer(value int64) *int64 {
	return &value
}
