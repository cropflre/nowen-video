package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	TranscodeStorageIncidentActive    = "active"
	TranscodeStorageIncidentRecovered = "recovered"
)

// TranscodeStorageIncidentRecord is durable operational evidence for an
// Artifact Store outage. ActiveKey is unique only while the incident is active;
// recovery clears it so a later recurrence creates a new history row.
type TranscodeStorageIncidentRecord struct {
	ID               string     `json:"id" gorm:"primaryKey;type:text"`
	ActiveKey        *string    `json:"active_key,omitempty" gorm:"uniqueIndex;type:text"`
	Code             string     `json:"code" gorm:"index;type:text;not null"`
	Severity         string     `json:"severity" gorm:"index;type:text;not null"`
	Operation        string     `json:"operation" gorm:"index;type:text;not null"`
	Path             string     `json:"path" gorm:"type:text"`
	Message          string     `json:"message" gorm:"type:text"`
	Retryable        bool       `json:"retryable"`
	AdmissionBlocked bool       `json:"admission_blocked"`
	QueuePaused      bool       `json:"queue_paused"`
	Occurrences      int64      `json:"occurrences"`
	FirstSeenAt      time.Time  `json:"first_seen_at" gorm:"index"`
	LastSeenAt       time.Time  `json:"last_seen_at" gorm:"index"`
	RecoveredAt      *time.Time `json:"recovered_at" gorm:"index"`
	Status           string     `json:"status" gorm:"index;type:text;not null"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (TranscodeStorageIncidentRecord) TableName() string {
	return "transcode_storage_incidents"
}

func (record *TranscodeStorageIncidentRecord) BeforeCreate(*gorm.DB) error {
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	return nil
}

func AutoMigrateTranscodeStorageIncidents(db *gorm.DB) error {
	return db.AutoMigrate(&TranscodeStorageIncidentRecord{})
}
