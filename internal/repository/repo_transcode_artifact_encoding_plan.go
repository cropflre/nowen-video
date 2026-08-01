package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// FindPublishedArtifactByEncodingPlan resolves an immutable Artifact only when
// both its declarative Encoding Plan and observed produced-media attestation
// match the formal publication contract. Historical unattested rows are kept
// for rollback and diagnostics but are intentionally excluded.
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
	result := r.db.Where(
		`media_id = ? AND profile_id = ? AND source_fingerprint = ? AND planner_version = ?
		AND kind = ? AND encoding_plan_version = ? AND encoding_plan_hash = ? AND status = ?
		AND attestation_status = ? AND attestation_version <> '' AND attestation_hash <> '' AND attestation_json <> ''`,
		mediaID,
		profileID,
		sourceFingerprint,
		plannerVersion,
		kind,
		encodingPlanVersion,
		encodingPlanHash,
		"published",
		"verified",
	).
		Order("published_at DESC, created_at DESC").
		Limit(1).
		Find(&artifact)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &artifact, nil
}

// FindReadableArtifactByEncodingPlan resolves either a Lease-valid current
// Attempt with provisional/verified first-segment evidence or the newest fully
// verified immutable Artifact. Unattested staging output is never readable.
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
	activeResult := r.db.Table("transcode_artifacts AS a").
		Select("a.*").
		Joins("JOIN transcode_jobs AS j ON j.id = a.job_id").
		Where(
			`a.media_id = ? AND a.profile_id = ? AND a.source_fingerprint = ? AND a.planner_version = ?
			AND a.kind = ? AND a.encoding_plan_version = ? AND a.encoding_plan_hash = ? AND a.status IN ?
			AND a.attestation_status IN ? AND a.attestation_version <> '' AND a.attestation_hash <> '' AND a.attestation_json <> ''
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
			[]string{"provisional", "verified"},
			"running",
			[]string{"claimed", "running"},
			now,
		).
		Order("a.created_at DESC").
		Limit(1).
		Find(&active)
	if activeResult.Error != nil {
		return nil, activeResult.Error
	}
	if activeResult.RowsAffected == 1 {
		return &active, nil
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
