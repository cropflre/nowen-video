package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	TranscodeStorageLedgerArtifactStore = "artifact_store"
	TranscodeStorageReservationActive   = "active"
	TranscodeStorageReservationReleased = "released"
)

// TranscodeStorageReservationRecord persists the predicted peak storage owned
// by one active Job. The reservation remains attached across Lease recovery and
// graceful requeue. Capacity accounting joins back to transcode_jobs, so a
// terminal Job immediately stops consuming headroom even if this audit row has
// not yet been reconciled to released.
type TranscodeStorageReservationRecord struct {
	JobID          string     `json:"job_id" gorm:"primaryKey;type:text"`
	MediaID        string     `json:"media_id" gorm:"index;type:text;not null"`
	ProfileID      string     `json:"profile_id" gorm:"index;type:text"`
	Intent         string     `json:"intent" gorm:"index;type:text"`
	EstimatedBytes int64      `json:"estimated_bytes"`
	ReservedBytes  int64      `json:"reserved_bytes"`
	State          string     `json:"state" gorm:"index;type:text;not null"`
	AcquiredAt     time.Time  `json:"acquired_at" gorm:"index"`
	ReleasedAt     *time.Time `json:"released_at" gorm:"index"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (TranscodeStorageReservationRecord) TableName() string {
	return "transcode_storage_reservations"
}

// TranscodeStorageLedgerRecord is a single-row serialization fence. Updating
// its version obtains the database write lock before active reservations are
// summed, preventing multiple server instances from overcommitting the same
// physical headroom.
type TranscodeStorageLedgerRecord struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	Version   uint64    `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (TranscodeStorageLedgerRecord) TableName() string {
	return "transcode_storage_ledger"
}

func AutoMigrateTranscodeStorageReservation(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&TranscodeStorageReservationRecord{},
		&TranscodeStorageLedgerRecord{},
	); err != nil {
		return err
	}
	ledger := TranscodeStorageLedgerRecord{
		ID:        TranscodeStorageLedgerArtifactStore,
		UpdatedAt: time.Now(),
	}
	return db.Where("id = ?", TranscodeStorageLedgerArtifactStore).
		FirstOrCreate(&ledger).Error
}
