package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

func validArtifactExecutionContract(
	encodingPlanVersion,
	encodingPlanHash,
	timestampPlanVersion,
	timestampPlanHash string,
	timelineOriginMS int64,
) bool {
	return encodingPlanVersion != "" && encodingPlanHash != "" &&
		timestampPlanVersion != "" && timestampPlanHash != "" && timelineOriginMS >= 0
}

// FindPublishedArtifactByExecutionContract requires both the immutable output
// compatibility identity and the timestamp normalization identity/origin.
// Older attested Artifacts remain stored but are not eligible for the new
// Startup Bridge planners.
func (r *TranscodeExecutionRepo) FindPublishedArtifactByExecutionContract(
	mediaID,
	profileID,
	sourceFingerprint,
	plannerVersion,
	kind,
	encodingPlanVersion,
	encodingPlanHash,
	timestampPlanVersion,
	timestampPlanHash string,
	timelineOriginMS int64,
) (*model.TranscodeArtifactRecord, error) {
	if !validArtifactExecutionContract(encodingPlanVersion, encodingPlanHash, timestampPlanVersion, timestampPlanHash, timelineOriginMS) {
		return nil, gorm.ErrRecordNotFound
	}
	var artifact model.TranscodeArtifactRecord
	result := r.db.Where(
		`media_id = ? AND profile_id = ? AND source_fingerprint = ? AND planner_version = ?
		AND kind = ? AND encoding_plan_version = ? AND encoding_plan_hash = ?
		AND timestamp_plan_version = ? AND timestamp_plan_hash = ? AND timeline_origin_ms = ?
		AND status = ? AND attestation_status = ?
		AND attestation_version <> '' AND attestation_hash <> '' AND attestation_json <> ''`,
		mediaID,
		profileID,
		sourceFingerprint,
		plannerVersion,
		kind,
		encodingPlanVersion,
		encodingPlanHash,
		timestampPlanVersion,
		timestampPlanHash,
		timelineOriginMS,
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

func (r *TranscodeExecutionRepo) FindReadableArtifactByExecutionContract(
	mediaID,
	profileID,
	sourceFingerprint,
	plannerVersion,
	kind,
	encodingPlanVersion,
	encodingPlanHash,
	timestampPlanVersion,
	timestampPlanHash string,
	timelineOriginMS int64,
	now time.Time,
) (*model.TranscodeArtifactRecord, error) {
	if !validArtifactExecutionContract(encodingPlanVersion, encodingPlanHash, timestampPlanVersion, timestampPlanHash, timelineOriginMS) {
		return nil, gorm.ErrRecordNotFound
	}
	var active model.TranscodeArtifactRecord
	activeResult := r.db.Table("transcode_artifacts AS a").
		Select("a.*").
		Joins("JOIN transcode_jobs AS j ON j.id = a.job_id").
		Where(
			`a.media_id = ? AND a.profile_id = ? AND a.source_fingerprint = ? AND a.planner_version = ?
			AND a.kind = ? AND a.encoding_plan_version = ? AND a.encoding_plan_hash = ?
			AND a.timestamp_plan_version = ? AND a.timestamp_plan_hash = ? AND a.timeline_origin_ms = ?
			AND a.status IN ? AND a.attestation_status IN ?
			AND a.attestation_version <> '' AND a.attestation_hash <> '' AND a.attestation_json <> ''
			AND a.attempt_id = j.current_attempt_id
			AND j.encoding_plan_version = a.encoding_plan_version AND j.encoding_plan_hash = a.encoding_plan_hash
			AND j.timestamp_plan_version = a.timestamp_plan_version AND j.timestamp_plan_hash = a.timestamp_plan_hash
			AND j.timeline_origin_ms = a.timeline_origin_ms
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
			timestampPlanVersion,
			timestampPlanHash,
			timelineOriginMS,
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
	return r.FindPublishedArtifactByExecutionContract(
		mediaID,
		profileID,
		sourceFingerprint,
		plannerVersion,
		kind,
		encodingPlanVersion,
		encodingPlanHash,
		timestampPlanVersion,
		timestampPlanHash,
		timelineOriginMS,
	)
}

// FindActiveArtifactByExecutionContractForAttestation returns one current
// staging Artifact before it has provisional evidence. Every declarative Job
// field must match the Artifact before ffprobe can make it playback-readable.
func (r *TranscodeExecutionRepo) FindActiveArtifactByExecutionContractForAttestation(
	mediaID,
	profileID,
	sourceFingerprint,
	plannerVersion,
	kind,
	encodingPlanVersion,
	encodingPlanHash,
	timestampPlanVersion,
	timestampPlanHash string,
	timelineOriginMS int64,
	now time.Time,
) (*model.TranscodeArtifactRecord, error) {
	if !validArtifactExecutionContract(encodingPlanVersion, encodingPlanHash, timestampPlanVersion, timestampPlanHash, timelineOriginMS) {
		return nil, gorm.ErrRecordNotFound
	}
	var artifact model.TranscodeArtifactRecord
	result := r.db.Table("transcode_artifacts AS a").
		Select("a.*").
		Joins("JOIN transcode_jobs AS j ON j.id = a.job_id").
		Where(
			`a.media_id = ? AND a.profile_id = ? AND a.source_fingerprint = ? AND a.planner_version = ?
			AND a.kind = ? AND a.encoding_plan_version = ? AND a.encoding_plan_hash = ?
			AND a.timestamp_plan_version = ? AND a.timestamp_plan_hash = ? AND a.timeline_origin_ms = ?
			AND a.status = ? AND a.attempt_id = j.current_attempt_id
			AND j.encoding_plan_version = a.encoding_plan_version AND j.encoding_plan_hash = a.encoding_plan_hash
			AND j.timestamp_plan_version = a.timestamp_plan_version AND j.timestamp_plan_hash = a.timestamp_plan_hash
			AND j.timeline_origin_ms = a.timeline_origin_ms
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
			timestampPlanVersion,
			timestampPlanHash,
			timelineOriginMS,
			"staging",
			"running",
			[]string{"claimed", "running"},
			now,
		).
		Order("a.created_at DESC").
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
