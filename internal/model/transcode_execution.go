package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TranscodeJobRecord struct {
	ID                   string     `json:"id" gorm:"primaryKey;type:text"`
	LegacyTaskID         *string    `json:"legacy_task_id,omitempty" gorm:"uniqueIndex;type:text"`
	MediaID              string     `json:"media_id" gorm:"index;type:text;not null"`
	Intent               string     `json:"intent" gorm:"index;type:text;not null"`
	ProfileID            string     `json:"profile_id" gorm:"type:text"`
	AudioTrack           int        `json:"audio_track" gorm:"default:-1"`
	StartMS              int64      `json:"start_ms"`
	DurationMS           int64      `json:"duration_ms"`
	Priority             int        `json:"priority" gorm:"index;default:0"`
	Status               string     `json:"status" gorm:"index;type:text;not null"`
	DesiredState         string     `json:"desired_state" gorm:"index;type:text;not null"`
	ActiveKey            *string    `json:"active_key,omitempty" gorm:"uniqueIndex;type:text"`
	SourceFingerprint    string     `json:"source_fingerprint" gorm:"index;type:text"`
	PlanHash             string     `json:"plan_hash" gorm:"index;type:text"`
	PlannerVersion       string     `json:"planner_version" gorm:"type:text"`
	EncodingPlanVersion  string     `json:"encoding_plan_version" gorm:"type:text"`
	EncodingPlanHash     string     `json:"encoding_plan_hash" gorm:"index;type:text"`
	EncodingPlanJSON     string     `json:"encoding_plan_json" gorm:"type:text"`
	TimestampPlanVersion string     `json:"timestamp_plan_version" gorm:"type:text"`
	TimestampPlanHash    string     `json:"timestamp_plan_hash" gorm:"index;type:text"`
	TimestampPlanJSON    string     `json:"timestamp_plan_json" gorm:"type:text"`
	TimelineOriginMS     int64      `json:"timeline_origin_ms" gorm:"index"`
	SessionID            string     `json:"session_id" gorm:"index;type:text"`
	CurrentAttemptID     string     `json:"current_attempt_id" gorm:"index;type:text"`
	WorkerID             string     `json:"worker_id" gorm:"index;type:text"`
	LeaseToken           string     `json:"lease_token" gorm:"index;type:text"`
	ClaimedAt            *time.Time `json:"claimed_at"`
	LastHeartbeatAt      *time.Time `json:"last_heartbeat_at" gorm:"index"`
	LeaseExpiresAt       *time.Time `json:"lease_expires_at" gorm:"index"`
	CancelRequestedAt    *time.Time `json:"cancel_requested_at"`
	CompletedAt          *time.Time `json:"completed_at"`
	CreatedAt            time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt            time.Time  `json:"updated_at" gorm:"index"`
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
	ID                          string     `json:"id" gorm:"primaryKey;type:text"`
	JobID                       string     `json:"job_id" gorm:"index;type:text;not null"`
	AttemptID                   string     `json:"attempt_id" gorm:"index;type:text"`
	MediaID                     string     `json:"media_id" gorm:"index:idx_transcode_artifact_resolve,priority:1;index;type:text"`
	Kind                        string     `json:"kind" gorm:"index;type:text;not null"`
	ProfileID                   string     `json:"profile_id" gorm:"index:idx_transcode_artifact_resolve,priority:2;index;type:text"`
	SourceFingerprint           string     `json:"source_fingerprint" gorm:"index:idx_transcode_artifact_resolve,priority:3;index;type:text"`
	PlannerVersion              string     `json:"planner_version" gorm:"index:idx_transcode_artifact_resolve,priority:4;type:text"`
	EncodingPlanVersion         string     `json:"encoding_plan_version" gorm:"type:text"`
	EncodingPlanHash            string     `json:"encoding_plan_hash" gorm:"index;type:text"`
	EncodingPlanJSON            string     `json:"encoding_plan_json" gorm:"type:text"`
	TimestampPlanVersion        string     `json:"timestamp_plan_version" gorm:"type:text"`
	TimestampPlanHash           string     `json:"timestamp_plan_hash" gorm:"index;type:text"`
	TimestampPlanJSON           string     `json:"timestamp_plan_json" gorm:"type:text"`
	TimelineOriginMS            int64      `json:"timeline_origin_ms" gorm:"index"`
	AttestationVersion          string     `json:"attestation_version" gorm:"type:text"`
	AttestationStatus           string     `json:"attestation_status" gorm:"index;type:text"`
	AttestationHash             string     `json:"attestation_hash" gorm:"index;type:text"`
	AttestationJSON             string     `json:"attestation_json" gorm:"type:text"`
	TimelineStartMS             int64      `json:"timeline_start_ms"`
	TimelineEndMS               int64      `json:"timeline_end_ms"`
	AttestedAt                  *time.Time `json:"attested_at" gorm:"index"`
	Path                        string     `json:"path" gorm:"type:text"`
	TempPath                    string     `json:"temp_path" gorm:"type:text"`
	ManifestPath                string     `json:"manifest_path" gorm:"type:text"`
	Status                      string     `json:"status" gorm:"index:idx_transcode_artifact_resolve,priority:5;index;type:text;not null"`
	MigrationSource             string     `json:"migration_source" gorm:"index;type:text"`
	SizeBytes                   int64      `json:"size_bytes"`
	Checksum                    string     `json:"checksum" gorm:"type:text"`
	DurationMS                  int64      `json:"duration_ms"`
	SegmentDuration             int        `json:"segment_duration"`
	PublishedAt                 *time.Time `json:"published_at" gorm:"index"`
	ExpiresAt                   *time.Time `json:"expires_at" gorm:"index"`
	ErrorCode                   string     `json:"error_code" gorm:"type:text"`
	ErrorMessage                string     `json:"error_message" gorm:"type:text"`
	CleanupState                string     `json:"cleanup_state" gorm:"index;type:text"`
	CleanupAttempts             int        `json:"cleanup_attempts" gorm:"default:0"`
	CleanupToken                string     `json:"cleanup_token" gorm:"index;type:text"`
	CleanupClaimedAt            *time.Time `json:"cleanup_claimed_at"`
	CleanupLeaseExpiresAt       *time.Time `json:"cleanup_lease_expires_at" gorm:"index"`
	CleanupNextAttemptAt        *time.Time `json:"cleanup_next_attempt_at" gorm:"index"`
	CleanupLastAttemptAt        *time.Time `json:"cleanup_last_attempt_at"`
	CleanupErrorCode            string     `json:"cleanup_error_code" gorm:"index;type:text"`
	CleanupErrorMessage         string     `json:"cleanup_error_message" gorm:"type:text"`
	CleanupCompletedAt          *time.Time `json:"cleanup_completed_at" gorm:"index"`
	CleanupDisposition          string     `json:"cleanup_disposition" gorm:"index;type:text"`
	CleanupOriginalPath         string     `json:"cleanup_original_path" gorm:"type:text"`
	CleanupOriginalTempPath     string     `json:"cleanup_original_temp_path" gorm:"type:text"`
	CleanupOriginalManifestPath string     `json:"cleanup_original_manifest_path" gorm:"type:text"`
	CleanupRollbackUntil        *time.Time `json:"cleanup_rollback_until" gorm:"index"`
	CreatedAt                   time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt                   time.Time  `json:"updated_at"`
}

func (TranscodeArtifactRecord) TableName() string { return "transcode_artifacts" }
func (a *TranscodeArtifactRecord) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	encodingComplete := a.EncodingPlanVersion != "" && a.EncodingPlanHash != "" && a.EncodingPlanJSON != ""
	timestampComplete := a.TimestampPlanVersion != "" && a.TimestampPlanHash != "" && a.TimestampPlanJSON != ""
	if a.JobID == "" || (encodingComplete && timestampComplete) {
		return nil
	}
	var job TranscodeJobRecord
	result := tx.Select(
		"encoding_plan_version",
		"encoding_plan_hash",
		"encoding_plan_json",
		"timestamp_plan_version",
		"timestamp_plan_hash",
		"timestamp_plan_json",
		"timeline_origin_ms",
	).
		Where("id = ?", a.JobID).
		Limit(1).
		Find(&job)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
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
	if a.TimestampPlanVersion == "" {
		a.TimestampPlanVersion = job.TimestampPlanVersion
	}
	if a.TimestampPlanHash == "" {
		a.TimestampPlanHash = job.TimestampPlanHash
	}
	if a.TimestampPlanJSON == "" {
		a.TimestampPlanJSON = job.TimestampPlanJSON
	}
	if a.TimestampPlanVersion != "" {
		a.TimelineOriginMS = job.TimelineOriginMS
	}
	return nil
}

type LegacyTranscodeProjectionMigrationState struct {
	Source               string     `json:"source" gorm:"primaryKey;type:text"`
	Generation           int64      `json:"generation" gorm:"default:0"`
	Status               string     `json:"status" gorm:"index;type:text;not null"`
	CursorUpdatedAt      *time.Time `json:"cursor_updated_at,omitempty" gorm:"index"`
	CursorID             string     `json:"cursor_id" gorm:"type:text"`
	HighWaterUpdatedAt   *time.Time `json:"high_water_updated_at,omitempty" gorm:"index"`
	HighWaterID          string     `json:"high_water_id" gorm:"type:text"`
	TargetRows           int64      `json:"target_rows"`
	ScannedRows          int64      `json:"scanned_rows"`
	ImportedJobs         int64      `json:"imported_jobs"`
	ArtifactsQueued      int64      `json:"artifacts_queued"`
	ArtifactsBlocked     int64      `json:"artifacts_blocked"`
	MissingPaths         int64      `json:"missing_paths"`
	BatchSize            int        `json:"batch_size"`
	FailureCount         int        `json:"failure_count"`
	ConsecutiveFailures  int        `json:"consecutive_failures"`
	LastErrorCode        string     `json:"last_error_code" gorm:"type:text"`
	LastErrorMessage     string     `json:"last_error_message" gorm:"type:text"`
	LastBatchStartedAt   *time.Time `json:"last_batch_started_at,omitempty"`
	LastBatchCompletedAt *time.Time `json:"last_batch_completed_at,omitempty"`
	NextAttemptAt        *time.Time `json:"next_attempt_at,omitempty" gorm:"index"`
	NextSourceCheckAt    *time.Time `json:"next_source_check_at,omitempty" gorm:"index"`
	LeaseOwner           string     `json:"lease_owner" gorm:"type:text"`
	LeaseToken           string     `json:"lease_token" gorm:"index;type:text"`
	LeaseExpiresAt       *time.Time `json:"lease_expires_at,omitempty" gorm:"index"`
	CompletedAt          *time.Time `json:"completed_at,omitempty" gorm:"index"`
	QuiescentSince       *time.Time `json:"quiescent_since,omitempty" gorm:"index"`
	SourceRetireAfter    *time.Time `json:"source_retire_after,omitempty" gorm:"index"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at" gorm:"index"`
}

func (LegacyTranscodeProjectionMigrationState) TableName() string {
	return "legacy_transcode_projection_migrations"
}

func AutoMigrateTranscodeExecution(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&TranscodeJobRecord{},
		&TranscodeAttemptRecord{},
		&TranscodeArtifactRecord{},
		&LegacyTranscodeProjectionMigrationState{},
		&TranscodeHandoffAttestationRecord{},
	); err != nil {
		return err
	}
	return db.Exec(`
		UPDATE transcode_artifacts
		SET
			media_id = COALESCE(NULLIF(media_id, ''), (SELECT media_id FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			source_fingerprint = COALESCE(NULLIF(source_fingerprint, ''), (SELECT source_fingerprint FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			planner_version = COALESCE(NULLIF(planner_version, ''), (SELECT planner_version FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			encoding_plan_version = COALESCE(NULLIF(encoding_plan_version, ''), (SELECT encoding_plan_version FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			encoding_plan_hash = COALESCE(NULLIF(encoding_plan_hash, ''), (SELECT encoding_plan_hash FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			encoding_plan_json = COALESCE(NULLIF(encoding_plan_json, ''), (SELECT encoding_plan_json FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			timestamp_plan_version = COALESCE(NULLIF(timestamp_plan_version, ''), (SELECT timestamp_plan_version FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			timestamp_plan_hash = COALESCE(NULLIF(timestamp_plan_hash, ''), (SELECT timestamp_plan_hash FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			timestamp_plan_json = COALESCE(NULLIF(timestamp_plan_json, ''), (SELECT timestamp_plan_json FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id)),
			timeline_origin_ms = CASE
				WHEN timestamp_plan_version <> '' THEN timeline_origin_ms
				ELSE COALESCE((SELECT timeline_origin_ms FROM transcode_jobs WHERE transcode_jobs.id = transcode_artifacts.job_id), timeline_origin_ms)
			END
		WHERE job_id <> ''
	`).Error
}
