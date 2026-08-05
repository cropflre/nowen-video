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
// by one active Job. ReservedBytes is the predicted total output size, while
// ObservedBytes is the portion already materialized in the current Attempt
// workspace and therefore already represented by disk/store usage samples.
// Capacity accounting charges only max(ReservedBytes-ObservedBytes, 0) as the
// future commitment, preventing actual bytes from being counted twice.
type TranscodeStorageReservationRecord struct {
	JobID                 string     `json:"job_id" gorm:"primaryKey;type:text"`
	MediaID               string     `json:"media_id" gorm:"index;type:text;not null"`
	ProfileID             string     `json:"profile_id" gorm:"index;type:text"`
	Intent                string     `json:"intent" gorm:"index;type:text"`
	AttemptID             string     `json:"attempt_id" gorm:"index;type:text"`
	EstimatedBytes        int64      `json:"estimated_bytes"`
	ReservedBytes         int64      `json:"reserved_bytes"`
	ObservedBytes         int64      `json:"observed_bytes"`
	PeakObservedBytes     int64      `json:"peak_observed_bytes"`
	FinalBytes            int64      `json:"final_bytes"`
	PredictionErrorBytes  int64      `json:"prediction_error_bytes"`
	ActualToEstimateRatio float64    `json:"actual_to_estimate_ratio"`
	ObservationCount      int64      `json:"observation_count"`
	Outcome               string     `json:"outcome" gorm:"index;type:text"`
	State                 string     `json:"state" gorm:"index;type:text;not null"`
	AcquiredAt            time.Time  `json:"acquired_at" gorm:"index"`
	LastObservedAt        *time.Time `json:"last_observed_at" gorm:"index"`
	ReleasedAt            *time.Time `json:"released_at" gorm:"index"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

func (TranscodeStorageReservationRecord) TableName() string {
	return "transcode_storage_reservations"
}

// TranscodeStorageLedgerRecord is a single-row serialization fence. Updating
// its version obtains the database write lock before active reservations are
// summed or observed, preventing multiple server instances from overcommitting
// the same physical headroom or racing a consumption refund.
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
