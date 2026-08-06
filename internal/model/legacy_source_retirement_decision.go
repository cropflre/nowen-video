package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	LegacySourceRetirementProtocolVersion  = "legacy-source-retirement/v1"
	LegacySourceRemovalPlanProtocolVersion = "legacy-source-removal-plan/v1"
)

// LegacySourceRetirementDecisionRecord is an append-only administrator review
// of a legacy source. Approval is evidence for a future schema migration; it
// never executes DDL or changes the legacy source itself.
type LegacySourceRetirementDecisionRecord struct {
	ID                    string     `json:"id" gorm:"primaryKey;type:text"`
	ProtocolVersion       string     `json:"protocol_version" gorm:"index;type:text;not null"`
	Source                string     `json:"source" gorm:"index:idx_legacy_source_retirement_review,priority:1;type:text;not null"`
	Generation            int64      `json:"generation" gorm:"index:idx_legacy_source_retirement_review,priority:2;not null"`
	Decision              string     `json:"decision" gorm:"index;type:text;not null"`
	EvidenceHash          string     `json:"evidence_hash" gorm:"index;type:text;not null"`
	EvidenceJSON          string     `json:"evidence_json" gorm:"type:text;not null"`
	ObservationStartedAt  *time.Time `json:"observation_started_at,omitempty"`
	ObservationEligibleAt *time.Time `json:"observation_eligible_at,omitempty"`
	MigrationStatus       string     `json:"migration_status" gorm:"type:text"`
	TargetRows            int64      `json:"target_rows"`
	ScannedRows           int64      `json:"scanned_rows"`
	UnmigratedRows        int64      `json:"unmigrated_rows"`
	RollbackOpenArtifacts int64      `json:"rollback_open_artifacts"`
	RollbackLatestUntil   *time.Time `json:"rollback_latest_until,omitempty"`
	BackupVerified        bool       `json:"backup_verified"`
	BackupVerifiedAt      *time.Time `json:"backup_verified_at,omitempty"`
	BackupRestoreTestedAt *time.Time `json:"backup_restore_tested_at,omitempty"`
	BackupReference       string     `json:"backup_reference" gorm:"type:text"`
	BackupChecksum        string     `json:"backup_checksum" gorm:"type:text"`
	ReviewerID            string     `json:"reviewer_id" gorm:"index;type:text"`
	ReviewerName          string     `json:"reviewer_name" gorm:"type:text"`
	Reason                string     `json:"reason" gorm:"type:text"`
	ReviewedAt            time.Time  `json:"reviewed_at" gorm:"index"`
	CreatedAt             time.Time  `json:"created_at"`
}

func (LegacySourceRetirementDecisionRecord) TableName() string {
	return "legacy_source_retirement_decisions"
}

func (r *LegacySourceRetirementDecisionRecord) BeforeCreate(*gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.ProtocolVersion == "" {
		r.ProtocolVersion = LegacySourceRetirementProtocolVersion
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	return nil
}

// LegacySourceRemovalPlanRecord is an immutable handoff package for a later,
// separately deployed schema migration. Creating a plan never executes DDL.
type LegacySourceRemovalPlanRecord struct {
	ID                     string     `json:"id" gorm:"primaryKey;type:text"`
	ProtocolVersion        string     `json:"protocol_version" gorm:"index;type:text;not null"`
	Source                 string     `json:"source" gorm:"index:idx_legacy_source_removal_plan,priority:1;type:text;not null"`
	Generation             int64      `json:"generation" gorm:"index:idx_legacy_source_removal_plan,priority:2;not null"`
	Status                 string     `json:"status" gorm:"index;type:text;not null"`
	RetirementDecisionID   string     `json:"retirement_decision_id" gorm:"index;type:text;not null"`
	RetirementDecisionHash string     `json:"retirement_decision_hash" gorm:"type:text;not null"`
	EvidenceHash           string     `json:"evidence_hash" gorm:"index;type:text;not null"`
	EvidenceJSON           string     `json:"evidence_json" gorm:"type:text;not null"`
	SchemaHash             string     `json:"schema_hash" gorm:"index;type:text;not null"`
	SchemaJSON             string     `json:"schema_json" gorm:"type:text;not null"`
	SourceRows             int64      `json:"source_rows"`
	ReviewerID             string     `json:"reviewer_id" gorm:"index;type:text;not null"`
	ReviewerName           string     `json:"reviewer_name" gorm:"type:text"`
	Reason                 string     `json:"reason" gorm:"type:text;not null"`
	PreparedAt             time.Time  `json:"prepared_at" gorm:"index;not null"`
	ExpiresAt              *time.Time `json:"expires_at,omitempty" gorm:"index"`
	CreatedAt              time.Time  `json:"created_at"`
}

func (LegacySourceRemovalPlanRecord) TableName() string {
	return "legacy_source_removal_plans"
}

func (r *LegacySourceRemovalPlanRecord) BeforeCreate(*gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.ProtocolVersion == "" {
		r.ProtocolVersion = LegacySourceRemovalPlanProtocolVersion
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	return nil
}

func AutoMigrateLegacySourceRetirement(db *gorm.DB) error {
	return db.AutoMigrate(
		&LegacySourceRetirementDecisionRecord{},
		&LegacySourceRemovalPlanRecord{},
	)
}
