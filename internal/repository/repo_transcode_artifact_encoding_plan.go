package repository

import (
	"errors"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// FindPublishedArtifactByEncodingPlan resolves an immutable Artifact only when
// its persisted output compatibility identity matches exactly. Historical rows
// with blank Encoding Plan fields are intentionally excluded.
func (r *TranscodeExecutionRepo) FindPublishedArtifactByEncodingPlan(
	mediaID,
	profileID,
	sourceFingerprint,
	plannerVersion,
	kind,
	encodingPlanVersion,
	encodingPlanHash string,
) (*model.TranscodeArtifactRecord, error) {
	if encodingPlanVersion == "" || encodingPlanHash == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var artifact model.TranscodeArtifactRecord
	err := r.db.Where(
		`media_id = ? AND profile_id = ? AND source_fingerprint = ? AND planner_version = ?
		AND kind = ? AND encoding_plan_version = ? AND encoding_plan_hash = ? AND status = ?`,
		mediaID,
		profileID,
		sourceFingerprint,
		plannerVersion,
		kind,
		encodingPlanVersion,
		encodingPlanHash,
		"published",
	).
		Order("published_at DESC, created_at DESC").
		First(&artifact).Error
	if err != nil {
		return nil, err
	}
	return &artifact, nil
}

// FindReadableArtifactByEncodingPlan resolves either the Lease-valid current
// Attempt Artifact or the newest immutable published Artifact for one complete
// media, planner and output-compatibility identity.
func (r *TranscodeExecutionRepo) FindReadableArtifactByEncodingPlan(
	mediaID,
	profileID,
	sourceFingerprint,
	plannerVersion,
	kind,
	encodingPlanVersion,
	encodingPlanHash string,
	now time.Time,
) (*model.TranscodeArtifactRecord, error) {
	if encodingPlanVersion == "" || encodingPlanHash == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var active model.TranscodeArtifactRecord
	activeErr := r.db.Table("transcode_artifacts AS a").
		Select("a.*").
		Joins("JOIN transcode_jobs AS j ON j.id = a.job_id").
		Where(
			`a.media_id = ? AND a.profile_id = ? AND a.source_fingerprint = ? AND a.planner_version = ?
			AND a.kind = ? AND a.encoding_plan_version = ? AND a.encoding_plan_hash = ? AND a.status IN ?
			AND a.attempt_id = j.current_attempt_id
			AND j.encoding_plan_version = a.encoding_plan_version
			AND j.encoding_plan_hash = a.encoding_plan_hash
			AND j.active_key IS NOT NULL AND j.desired_state = ?
			AND j.status IN ? AND j.lease_token <> ''
			AND j.lease_expires_at IS NOT NULL AND j.lease_expires_at > ?`,
			mediaID,
			profileID,
			sourceFingerprint,
			plannerVersion,
			kind,
			encodingPlanVersion,
			encodingPlanHash,
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
	return r.FindPublishedArtifactByEncodingPlan(
		mediaID,
		profileID,
		sourceFingerprint,
		plannerVersion,
		kind,
		encodingPlanVersion,
		encodingPlanHash,
	)
}
