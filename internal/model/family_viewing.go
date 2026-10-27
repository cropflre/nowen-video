package model

import "time"

// ChargedSeconds includes outstanding reservations. No client playhead or clock
// is used to increase or reset the daily budget.
type FamilyViewingDay struct {
	UserID         string `gorm:"primaryKey;type:text"`
	Day            string `gorm:"primaryKey;type:text"`
	ChargedSeconds int64
}

// One outstanding reservation per account, persisted across server restarts.
type FamilyViewingLease struct {
	UserID          string `gorm:"primaryKey;type:text"`
	ID              string `gorm:"type:text"`
	MediaID         string `gorm:"type:text"`
	Day             string `gorm:"type:text"`
	TokenVersion    int
	StartedAt       time.Time
	ExpiresAt       time.Time
	ReservedSeconds int64
}
