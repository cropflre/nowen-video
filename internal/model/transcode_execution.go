package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TranscodeJobRecord struct {
	ID                string     `json:"id" gorm:"primaryKey;type:text"`
	LegacyTaskID      *string    `json:"legacy_task_id,omitempty" gorm:"uniqueIndex;type:text"`
	MediaID           string     `json:"media_id" gorm:"index;type:text;not null"`
	Intent            string     `json:"intent" gorm:"index;type:text;not null"`
	ProfileID         string     `json:"profile_id" gorm:"type:text"`
	AudioTrack        int        `json:"audio_track" gorm:"default:-1"`
	StartMS           int64      `json:"start_ms"`
	Priority          int        `json:"priority" gorm:"index;default:0"`
	Status            string     `json:"status" gorm:"index;type:text;not null"`
	DesiredState      string     `json:"desired_state" gorm:"index;type:text;not null"`
	ActiveKey         *string    `json:"active_key,omitempty" gorm:"uniqueIndex;type:text"`
	SourceFingerprint string     `json:"source_fingerprint" gorm:"index;type:text"`
	PlanHash          string     `json:"plan_hash" gorm:"index;type:text"`
	PlannerVersion    string     `json:"planner_version" gorm:"type:text"`
	SessionID         string     `json:"session_id" gorm:"index;type:text"`
	CurrentAttemptID  string     `json:"current_attempt_id" gorm:"index;type:text"`
	WorkerID          string     `json:"worker_id" gorm:"index;type:text"`
	LeaseToken        string     `json:"lease_token" gorm:"index;type:text"`
	ClaimedAt         *time.Time `json:"claimed_at"`
	LastHeartbeatAt   *time.Time `json:"last_heartbeat_at" gorm:"index"`
	LeaseExpiresAt    *time.Time `json:"lease_expires_at" gorm:"index"`
	CancelRequestedAt *time.Time `json:"cancel_requested_at"`
	CompletedAt       *time.Time `json:"completed_at"`
	CreatedAt         time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt         time.Time  `json:"updated_at" gorm:"index"`
}

func (TranscodeJobRecord) TableName() string { return "transcode_jobs" }
func (j *TranscodeJobRecord) BeforeCreate(*gorm.DB) error {
	if j.ID == "" {
		j.ID = uuid.NewString()
	}
	return nil
}

type TranscodeAttemptRecord struct {
	ID           string     `json:"id" gorm:"primaryKey;type:text"`
	JobID        string     `json:"job_id" gorm:"uniqueIndex:idx_transcode_attempt_no;index;type:text;not null"`
	Number       int        `json:"number" gorm:"uniqueIndex:idx_transcode_attempt_no;not null"`
	Backend      string     `json:"backend" gorm:"index;type:text"`
	Status       string     `json:"status" gorm:"index;type:text;not null"`
	PID          int        `json:"pid" gorm:"column:pid"`
	CommandJSON  string     `json:"command_json" gorm:"type:text"`
	StartedAt    *time.Time `json:"started_at"`
	HeartbeatAt  *time.Time `json:"heartbeat_at" gorm:"index"`
	CompletedAt  *time.Time `json:"completed_at"`
	ExitCode     int        `json:"exit_code" gorm:"default:-1"`
	Signal       string     `json:"signal" gorm:"type:text"`
	StderrTail   string     `json:"stderr_tail" gorm:"type:text"`
	ErrorCode    string     `json:"error_code" gorm:"type:text"`
	ErrorMessage string     `json:"error_message" gorm:"type:text"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (TranscodeAttemptRecord) TableName() string { return "transcode_attempts" }
func (a *TranscodeAttemptRecord) BeforeCreate(*gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return nil
}

type TranscodeArtifactRecord struct {
	ID              string     `json:"id" gorm:"primaryKey;type:text"`
	JobID           string     `json:"job_id" gorm:"index;type:text;not null"`
	AttemptID       string     `json:"attempt_id" gorm:"index;type:text"`
	Kind            string     `json:"kind" gorm:"index;type:text;not null"`
	ProfileID       string     `json:"profile_id" gorm:"index;type:text"`
	Path            string     `json:"path" gorm:"type:text"`
	TempPath        string     `json:"temp_path" gorm:"type:text"`
	Status          string     `json:"status" gorm:"index;type:text;not null"`
	SizeBytes       int64      `json:"size_bytes"`
	Checksum        string     `json:"checksum" gorm:"type:text"`
	DurationMS      int64      `json:"duration_ms"`
	SegmentDuration int        `json:"segment_duration"`
	ExpiresAt       *time.Time `json:"expires_at" gorm:"index"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (TranscodeArtifactRecord) TableName() string { return "transcode_artifacts" }
func (a *TranscodeArtifactRecord) BeforeCreate(*gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return nil
}

func AutoMigrateTranscodeExecution(db *gorm.DB) error {
	return db.AutoMigrate(
		&TranscodeJobRecord{},
		&TranscodeAttemptRecord{},
		&TranscodeArtifactRecord{},
	)
}
