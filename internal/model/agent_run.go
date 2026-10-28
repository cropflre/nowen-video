package model

import "time"

// AgentRun is an owner-bound preview and durable execution receipt. The nullable
// unique library key prevents two approved Agent jobs from claiming one library.
type AgentRun struct {
	ID              string     `json:"id" gorm:"primaryKey;type:text"`
	UserID          string     `json:"-" gorm:"index;type:text;not null"`
	TokenVersion    int        `json:"-"`
	Kind            string     `json:"kind" gorm:"type:text"`
	LibraryID       string     `json:"library_id" gorm:"index;type:text"`
	LibraryName     string     `json:"library_name" gorm:"type:text"`
	Snapshot        string     `json:"-" gorm:"type:text"`
	Proposal        string     `json:"-" gorm:"type:text"`
	Digest          string     `json:"approval_digest" gorm:"type:text"`
	Status          string     `json:"status" gorm:"index;type:text"`
	ActiveLibraryID *string    `json:"-" gorm:"uniqueIndex;type:text"`
	WorkerID        string     `json:"-" gorm:"type:text"`
	LeaseUntil      *time.Time `json:"-"`
	NewCount        int        `json:"new_count"`
	Message         string     `json:"message" gorm:"type:text"`
	ExpiresAt       time.Time  `json:"expires_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
