package repository

import "gorm.io/gorm"

// PulseRepo is a compatibility tombstone kept only because the legacy Full
// composition still exposes a Pulse field while old routes are phased out.
//
// Pulse was removed from the product. The repository intentionally exposes no
// query methods and performs no database work. Existing SQLite data is left
// untouched so upgrading or rolling back never destroys user data.
type PulseRepo struct {
	db *gorm.DB
}
