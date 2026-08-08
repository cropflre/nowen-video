package repository

import (
	"errors"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// FindReadableArtifactByKind resolves either the Lease-valid current staging
// artifact or the newest immutable published artifact for one complete media
// planning identity. Callers must supply Kind so startup, continuation and
// ordinary runtime artifacts cannot shadow one another.
func (r *TranscodeExecutionRepo) FindReadableArtifactByKind(
	mediaID,
	profileID,
	sourceFingerprint,
	plannerVersion,
	kind string,
	now time.Time,
) (*model.TranscodeArtifactRecord, error) {
	var active model.TranscodeArtifactRecord
	activeErr := r.db.Table("transcode_artifacts AS a").
		Select("a.*").
		Joins("JOIN transcode_jobs AS j ON j.id = a.job_id").
		Where(
			`a.media_id = ? AND a.profile_id = ? AND a.source_fingerprint = ? AND a.planner_version = ?
			AND a.kind = ? AND a.status IN ?
			AND a.attempt_id = j.current_attempt_id
			AND j.active_key IS NOT NULL AND j.desired_state = ?
			AND j.status IN ? AND j.lease_token <> ''
			AND j.lease_expires_at IS NOT NULL AND j.lease_expires_at > ?`,
			mediaID,
			profileID,
			sourceFingerprint,
			plannerVersion,
			kind,
			[]string{"staging", "publishing"},
			"running",
			[]string{"claimed", "running"},
			now,
		).
		Order("a.created_at DESC").
		First(&active).Error
	if activeErr == nil {
		return &active, nil
	}
	if !errors.Is(activeErr, gorm.ErrRecordNotFound) {
		return nil, activeErr
	}
	return r.FindPublishedArtifact(mediaID, profileID, sourceFingerprint, plannerVersion, kind)
}
