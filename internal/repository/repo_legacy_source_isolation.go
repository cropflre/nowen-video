package repository

import (
	"fmt"
	"sort"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	LegacySourceOriginalTable = "transcode_tasks"
	LegacySourceArchiveTable  = "legacy_transcode_tasks_retired_v1"
)

type LegacySourceTableState struct {
	OriginalTablePresent bool `json:"original_table_present"`
	ArchiveTablePresent  bool `json:"archive_table_present"`
}

func (r *TranscodeExecutionRepo) EnsureLegacySourceIsolationSchema() error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return r.db.AutoMigrate(
		&model.LegacySourceIsolationRecord{},
		&model.LegacySourceIsolationRollbackRecord{},
	)
}

func (r *TranscodeExecutionRepo) InTransaction(fn func(*TranscodeExecutionRepo) error) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	if fn == nil {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(NewTranscodeExecutionRepo(tx))
	})
}

func (r *TranscodeExecutionRepo) LegacySourceTableState() (LegacySourceTableState, error) {
	state := LegacySourceTableState{}
	if r == nil || r.db == nil {
		return state, gorm.ErrInvalidDB
	}
	state.OriginalTablePresent = r.db.Migrator().HasTable(LegacySourceOriginalTable)
	state.ArchiveTablePresent = r.db.Migrator().HasTable(LegacySourceArchiveTable)
	return state, nil
}

func (r *TranscodeExecutionRepo) LegacySourceRowCount(tableName string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, gorm.ErrInvalidDB
	}
	if tableName != LegacySourceOriginalTable && tableName != LegacySourceArchiveTable {
		return 0, fmt.Errorf("invalid legacy source table %q", tableName)
	}
	if !r.db.Migrator().HasTable(tableName) {
		return 0, nil
	}
	var count int64
	if err := r.db.Table(tableName).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// LegacySourceArchiveSchemaSnapshot reads the archived table but normalizes the
// table name to transcode_tasks so its hash can be compared with the removal
// plan captured before isolation.
func (r *TranscodeExecutionRepo) LegacySourceArchiveSchemaSnapshot() (LegacySourceSchemaSnapshot, error) {
	return r.legacySourceSchemaSnapshotForTable(LegacySourceArchiveTable, LegacySourceOriginalTable)
}

func (r *TranscodeExecutionRepo) legacySourceSchemaSnapshotForTable(actualTable, canonicalTable string) (LegacySourceSchemaSnapshot, error) {
	snapshot := LegacySourceSchemaSnapshot{
		TableName: canonicalTable,
		Columns:   make([]LegacySourceColumnSnapshot, 0),
	}
	if r == nil || r.db == nil {
		return snapshot, gorm.ErrInvalidDB
	}
	if actualTable != LegacySourceOriginalTable && actualTable != LegacySourceArchiveTable {
		return snapshot, fmt.Errorf("invalid legacy source table %q", actualTable)
	}
	snapshot.TablePresent = r.db.Migrator().HasTable(actualTable)
	if !snapshot.TablePresent {
		return snapshot, nil
	}
	columnTypes, err := r.db.Migrator().ColumnTypes(actualTable)
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

func (r *TranscodeExecutionRepo) LegacySourceRemovalPlanByID(source, id string, lock bool) (*model.LegacySourceRemovalPlanRecord, error) {
	query := r.db.Where("source = ? AND id = ?", source, id)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var record model.LegacySourceRemovalPlanRecord
	result := query.Limit(1).Find(&record)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &record, nil
}

func (r *TranscodeExecutionRepo) LatestLegacySourceRemovalPlanForIsolation(source string, lock bool) (*model.LegacySourceRemovalPlanRecord, error) {
	query := r.db.Where("source = ?", source).
		Order("prepared_at DESC, created_at DESC, id DESC")
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var record model.LegacySourceRemovalPlanRecord
	result := query.Limit(1).Find(&record)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &record, nil
}

func (r *TranscodeExecutionRepo) LegacySourceIsolationByPlanID(source, planID string) (*model.LegacySourceIsolationRecord, error) {
	var record model.LegacySourceIsolationRecord
	result := r.db.Where("source = ? AND removal_plan_id = ?", source, planID).
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

func (r *TranscodeExecutionRepo) LegacySourceIsolationByID(source, isolationID string, lock bool) (*model.LegacySourceIsolationRecord, error) {
	query := r.db.Where("source = ? AND id = ?", source, isolationID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var record model.LegacySourceIsolationRecord
	result := query.Limit(1).Find(&record)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &record, nil
}

func (r *TranscodeExecutionRepo) LatestLegacySourceIsolation(source string) (*model.LegacySourceIsolationRecord, error) {
	var record model.LegacySourceIsolationRecord
	result := r.db.Where("source = ?", source).
		Order("isolated_at DESC, created_at DESC, id DESC").
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

func (r *TranscodeExecutionRepo) CreateLegacySourceIsolation(record *model.LegacySourceIsolationRecord) error {
	if record == nil {
		return nil
	}
	return r.db.Create(record).Error
}

func (r *TranscodeExecutionRepo) LegacySourceIsolationRollbackByIsolationID(source, isolationID string) (*model.LegacySourceIsolationRollbackRecord, error) {
	var record model.LegacySourceIsolationRollbackRecord
	result := r.db.Where("source = ? AND isolation_id = ?", source, isolationID).
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

func (r *TranscodeExecutionRepo) LatestLegacySourceIsolationRollback(source string) (*model.LegacySourceIsolationRollbackRecord, error) {
	var record model.LegacySourceIsolationRollbackRecord
	result := r.db.Where("source = ?", source).
		Order("rolled_back_at DESC, created_at DESC, id DESC").
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

func (r *TranscodeExecutionRepo) CreateLegacySourceIsolationRollback(record *model.LegacySourceIsolationRollbackRecord) error {
	if record == nil {
		return nil
	}
	return r.db.Create(record).Error
}

func (r *TranscodeExecutionRepo) RenameLegacySourceToArchive() error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return r.db.Migrator().RenameTable(LegacySourceOriginalTable, LegacySourceArchiveTable)
}

func (r *TranscodeExecutionRepo) RestoreLegacySourceFromArchive() error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return r.db.Migrator().RenameTable(LegacySourceArchiveTable, LegacySourceOriginalTable)
}
