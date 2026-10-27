package model

import "time"

type DeviceSession struct {
	ID           string     `json:"id" gorm:"primaryKey;type:text"`
	UserID       string     `json:"-" gorm:"index;type:text;not null"`
	TokenVersion int        `json:"-"`
	Name         string     `json:"name" gorm:"type:text"`
	CreatedAt    time.Time  `json:"created_at"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	ExpiresAt    time.Time  `json:"expires_at" gorm:"index"`
	RevokedAt    *time.Time `json:"-"`
}

// Only hashes of the short code and the requester secret are persisted.
// Neither the short code nor request ID can exchange authorization alone.
type DevicePairing struct {
	ID                   string `gorm:"primaryKey;type:text"`
	CodeHash             string `gorm:"uniqueIndex;type:text"`
	SecretHash           string `gorm:"type:text"`
	DeviceName           string `gorm:"type:text"`
	CreatedAt            time.Time
	ExpiresAt            time.Time `gorm:"index"`
	ApprovedUserID       string    `gorm:"type:text"`
	ApprovedSessionID    string    `gorm:"type:text"`
	ApprovedTokenVersion int
	ConsumedAt           *time.Time
	CancelledAt          *time.Time
}
