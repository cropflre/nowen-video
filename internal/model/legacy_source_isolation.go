package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const LegacySourceIsolationProtocolVersion = "legacy-source-isolation/v1"

// LegacySourceIsolationRecord is the append-only audit result of moving a
// retired source out of its runtime table name. Isolation preserves every row
// and is reversible; it never drops the archived table.
type LegacySourceIsolationRecord struct {
	ID                     string    `json:"id" gorm:"primaryKey;type:text"`
	ProtocolVersion        string    `json:"protocol_version" gorm:"index;type:text;not null"`
	Source                 string    `json:"source" gorm:"index:idx_legacy_source_isolation,priority:1;type:text;not null"`
	Generation             int64     `json:"generation" gorm:"index:idx_legacy_source_isolation,priority:2;not null"`
	Status                 string    `json:"status" gorm:"index;type:text;not null"`
	RemovalPlanID          string    `json:"removal_plan_id" gorm:"uniqueIndex;type:text;not null"`
	RetirementDecisionID   string    `json:"retirement_decision_id" gorm:"index;type:text;not null"`
	RetirementDecisionHash string    `json:"retirement_decision_hash" gorm:"type:text;not null"`
	EvidenceHash           string    `json:"evidence_hash" gorm:"index;type:text;not null"`
	SchemaHash             string    `json:"schema_hash" gorm:"index;type:text;not null"`
	SourceRows             int64     `json:"source_rows"`
	OriginalTable          string    `json:"original_table" gorm:"type:text;not null"`
	ArchiveTable           string    `json:"archive_table" gorm:"index;type:text;not null"`
	BackupReference        string    `json:"backup_reference" gorm:"type:text;not null"`
	BackupChecksum         string    `json:"backup_checksum" gorm:"type:text;not null"`
	ReviewerID             string    `json:"reviewer_id" gorm:"index;type:text;not null"`
	ReviewerName           string    `json:"reviewer_name" gorm:"type:text"`
	Reason                 string    `json:"reason" gorm:"type:text;not null"`
	IsolatedAt             time.Time `json:"isolated_at" gorm:"index;not null"`
	CreatedAt              time.Time `json:"created_at"`
}

func (LegacySourceIsolationRecord) TableName() string {
	return "legacy_source_isolations"
}

func (r *LegacySourceIsolationRecord) BeforeCreate(*gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.ProtocolVersion == "" {
		r.ProtocolVersion = LegacySourceIsolationProtocolVersion
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	return nil
}

// LegacySourceIsolationRollbackRecord is append-only evidence that an isolated
// table was restored to its original runtime name.
type LegacySourceIsolationRollbackRecord struct {
	ID              string    `json:"id" gorm:"primaryKey;type:text"`
	ProtocolVersion string    `json:"protocol_version" gorm:"index;type:text;not null"`
	Source          string    `json:"source" gorm:"index;type:text;not null"`
	IsolationID     string    `json:"isolation_id" gorm:"uniqueIndex;type:text;not null"`
	RemovalPlanID   string    `json:"removal_plan_id" gorm:"index;type:text;not null"`
	SchemaHash      string    `json:"schema_hash" gorm:"type:text;not null"`
	SourceRows      int64     `json:"source_rows"`
	OriginalTable   string    `json:"original_table" gorm:"type:text;not null"`
	ArchiveTable    string    `json:"archive_table" gorm:"type:text;not null"`
	ReviewerID      string    `json:"reviewer_id" gorm:"index;type:text;not null"`
	ReviewerName    string    `json:"reviewer_name" gorm:"type:text"`
	Reason          string    `json:"reason" gorm:"type:text;not null"`
	RolledBackAt    time.Time `json:"rolled_back_at" gorm:"index;not null"`
	CreatedAt       time.Time `json:"created_at"`
}

func (LegacySourceIsolationRollbackRecord) TableName() string {
	return "legacy_source_isolation_rollbacks"
}

func (r *LegacySourceIsolationRollbackRecord) BeforeCreate(*gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.ProtocolVersion == "" {
		r.ProtocolVersion = LegacySourceIsolationProtocolVersion
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	return nil
}
