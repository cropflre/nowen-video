package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
)

// FindPublishedArtifact resolves an immutable artifact by its complete planning
// identity. Kind is part of the identity so startup_hls and runtime_hls can use
// the same media/profile without shadowing each other.
func (r *TranscodeExecutionRepo) FindPublishedArtifact(
	mediaID,
	profileID,
	sourceFingerprint,
	plannerVersion,
	kind string,
) (*model.TranscodeArtifactRecord, error) {
	var artifact model.TranscodeArtifactRecord
	err := r.db.Where(
		"media_id = ? AND profile_id = ? AND source_fingerprint = ? AND planner_version = ? AND kind = ? AND status = ?",
		mediaID,
		profileID,
		sourceFingerprint,
		plannerVersion,
		kind,
		"published",
	).
		Order("published_at DESC, created_at DESC").
		First(&artifact).Error
	if err != nil {
		return nil, err
	}
	return &artifact, nil
}
