package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm/clause"
)

// ImportLegacyHLSArtifact is the bounded migration Adapter for historical
// media/profile directories. The caller supplies a deterministic primary key,
// making repeated reads idempotent. New runtime writes must never use it.
func (r *TranscodeExecutionRepo) ImportLegacyHLSArtifact(artifact *model.TranscodeArtifactRecord) error {
	if artifact == nil {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(artifact).Error
}
