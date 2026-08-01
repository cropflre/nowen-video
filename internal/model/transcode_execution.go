package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TranscodeJobRecord struct {
	ID                  string     `json:"id" gorm:"primaryKey;type:text"`
	LegacyTaskID        *string    `json:"legacy_task_id,omitempty" gorm:"uniqueIndex;type:text"`
	MediaID             string     `json:"media_id" gorm:"index;type:text;not null"`
	Intent              string     `json:"intent" gorm:"index;type:text;not null"`
	ProfileID           string     `json:"profile_id" gorm:"type:text"`
	AudioTrack          int        `json:"audio_track" gorm:"default:-1"`
	StartMS             int64      `json:"start_ms"`
	DurationMS          int64      `json:"duration_ms"`
	Priority            int        `json:"priority" gorm:"index;default:0"`
	Status              string     `json:"status" gorm:"index;type:text;not null"`
	DesiredState        string     `json:"desired_state" gorm:"index;type:text;not null"`
	ActiveKey           *string    `json:"active_key,omitempty" gorm:"uniqueIndex;type:text"`
	SourceFingerprint   string     `json:"source_fingerprint" gorm:"index;type:text"`
	PlanHash            string     `json:"plan_hash" gorm:"index;type:text"`
	PlannerVersion      string     `json:"planner_version" gorm:"type:text"`
	EncodingPlanVersion string     `json:"encoding_plan_version" gorm:"type:text"`
	EncodingPlanHash    string     `json:"encoding_plan_hash" gorm:"index;type:text"`
	EncodingPlanJSON    string     `json:"encoding_plan_json" gorm:"type:text"`
	SessionID           string     `json:"session_id" gorm:"index;type:text"`
	CurrentAttemptID    string     `json:"current_attempt_id" gorm:"index;type:text"`
	WorkerID            string     `json:"worker_id" gorm:"index;type:text"`
	LeaseToken          string     `json:"lease_token" gorm:"index;type:text"`
	ClaimedAt           *time.Time `json:"claimed_at"`
	LastHeartbeatAt     *time.Time `json:"last_heartbeat_at" gorm:"index"`
	LeaseExpiresAt      *time.Time `json:"lease_expires_at" gorm:"index"`
	CancelRequestedAt   *time.Time `json:"cancel_requested_at"`
	CompletedAt         *time.Time `json:"completed_at"`
	CreatedAt           time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt           time.Time  `json:"updated_at" gorm:"index"`
}

func (TranscodeJobRecord) TableName() string { return "transcode_jobs" }
func (j *TranscodeJobRecord) BeforeCreate(*gorm.DB) error {
	if j.ID == "" {
		j.ID = uuid.NewString()
	}
	return nil
}

type TranscodeAttemptRecord struct {
	ID            string     `json:"id" gorm:"primaryKey;type:text"`
	JobID         string     `json:"job_id" gorm:"uniqueIndex:idx_transcode_attempt_no;index;type:text;not null"`
	Number        int        `json:"number" gorm:"uniqueIndex:idx_transcode_attempt_no;not null"`
	Backend       string     `json:"backend" gorm:"index;type:text"`
	Status        string     `json:"status" gorm:"index;type:text;not null"`
	PID           int        `json:"pid" gorm:"column:pid"`
	CommandJSON   string     `json:"command_json" gorm:"type:text"`
	WorkspacePath string     `json:"workspace_path" gorm:"type:text"`
	StartedAt     *time.Time `json:"started_at"`
	HeartbeatAt   *time.Time `json:"heartbeat_at" gorm:"index"`
	CompletedAt   *time.Time `json:"completed_at"`
	ExitCode      int        `json:"exit_code" gorm:"default:-1"`
	Signal        string     `json:"signal" gorm:"type:text"`
	StderrTail    string     `json:"stderr_tail" gorm:"type:text"`
	ErrorCode     string     `json:"error_code" gorm:"type:text"`
	ErrorMessage  string     `json:"error_message" gorm:"type:text"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (TranscodeAttemptRecord) TableName() string { return "transcode_attempts" }
func (a *TranscodeAttemptRecord) BeforeCreate(*gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return nil
}

type TranscodeArtifactRecord struct {
	ID                  string     `json:"id" gorm:"primaryKey;type:text"`
	JobID               string     `json:"job_id" gorm:"index;type:text;not null"`
	AttemptID           string     `json:"attempt_id" gorm:"index;type:text"`
	MediaID             string     `json:"media_id" gorm:"index:idx_transcode_artifact_resolve,priority:1;index;type:text"`
	Kind                string     `json:"kind" gorm:"index;type:text;not null"`
	ProfileID           string     `json:"profile_id" gorm:"index:idx_transcode_artifact_resolve,priority:2;index;type:text"`
	SourceFingerprint   string     `json:"source_fingerprint" gorm:"index:idx_transcode_artifact_resolve,priority:3;index;type:text"`
	PlannerVersion      string     `json:"planner_version" gorm:"index:idx_transcode_artifact_resolve,priority:4;type:text"`
	EncodingPlanVersion string     `json:"encoding_plan_version" gorm:"type:text"`
	EncodingPlanHash    string     `json:"encoding_plan_hash" gorm:"index;type:text"`
	EncodingPlanJSON    string     `json:"encoding_plan_json" gorm:"type:text"`
	Path                string     `json:"path" gorm:"type:text"`
	TempPath            string     `json:"temp_path" gorm:"type:text"`
	ManifestPath        string     `json:"manifest_path" gorm:"type:text"`
	Status              string     `json:"status" gorm:"index:idx_transcode_artifact_resolve,priority:5;index;type:text;not null"`
	MigrationSource     string     `json:"migration_source" gorm:"index;type:text"`
	SizeBytes           int64      `json:"size_bytes"`
	Checksum            string     `json:"checksum" gorm:"type:text"`
	DurationMS          int64      `json:"duration_ms"`
	SegmentDuration     int        `json:"segment_duration"`
	PublishedAt         *time.Time `json:"published_at" gorm:"index"`
	ExpiresAt           *time.Time `json:"expires_at" gorm:"index"`
	ErrorCode           string     `json:"error_code" gorm:"type:text"`
	ErrorMessage        string     `json:"error_message" gorm:"type:text"`
	CreatedAt           time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func (TranscodeArtifactRecord) TableName() string { return "transcode_artifacts" }
func (a *TranscodeArtifactRecord) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if a.JobID == "" || (a.EncodingPlanVersion != "" && a.EncodingPlanHash != "" && a.EncodingPlanJSON != "") {
		return nil
	}
	var job TranscodeJobRecord
	result := tx.Select("encoding_plan_version", "encoding_plan_hash", "encoding_plan_json").
		Where("id = ?", a.JobID).
		Limit(1).
		Find(&job)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// Historical legacy imports intentionally use synthetic Job IDs.
		return nil
	}
	if a.EncodingPlanVersion == "" {
		a.EncodingPlanVersion = job.EncodingPlanVersion
	}
	if a.EncodingPlanHash == "" {
		a.EncodingPlanHash = job.EncodingPlanHash
	}
	if a.EncodingPlanJSON == "" {
		a.EncodingPlanJSON = job.EncodingPlanJSON
	}
	return nil
}

func AutoMigrateTranscodeExecution(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&TranscodeJobRecord{},
		&TranscodeAttemptRecord{},
		&TranscodeArtifactRecord{},
	); err != nil {
		return err
	}
	// Existing artifact rows were created before identity and Encoding Plan
	// fields were stored on the artifact itself. Backfill from the owning Job
	// without deleting or rewriting historical task/output data. The correlated
	// update is supported by both SQLite and PostgreSQL.
	return db.Exec(`
		UPDATE transcode_artifacts
		SET
			media_id = COALESCE(NULLIF(media_id, ''), (SELECT media_id FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			source_fingerprint = COALESCE(NULLIF(source_fingerprint, ''), (SELECT source_fingerprint FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			planner_version = COALESCE(NULLIF(planner_version, ''), (SELECT planner_version FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			encoding_plan_version = COALESCE(NULLIF(encoding_plan_version, ''), (SELECT encoding_plan_version FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			encoding_plan_hash = COALESCE(NULLIF(encoding_plan_hash, ''), (SELECT encoding_plan_hash FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			encoding_plan_json = COALESCE(NULLIF(encoding_plan_json, ''), (SELECT encoding_plan_json FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id))
		WHERE job_id <> ''
	`).Error
}
