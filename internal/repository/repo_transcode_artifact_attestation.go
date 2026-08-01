package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// RecordOwnedArtifactAttestation persists evidence produced by the current
// Worker only while it still owns the Job Lease and Attempt. Final verification
// therefore cannot be attached by a stale or recovered Worker.
func (r *TranscodeExecutionRepo) RecordOwnedArtifactAttestation(
	jobID,
	attemptID,
	artifactID,
	leaseToken,
	version,
	status,
	hash,
	canonical string,
	timelineStartMS,
	timelineEndMS int64,
	attestedAt time.Time,
) (bool, error) {
	if version == "" || hash == "" || canonical == "" || (status != "provisional" && status != "verified") {
		return false, nil
	}
	result := r.db.Model(&model.TranscodeArtifactRecord{}).
		Where(
			`id = ? AND job_id = ? AND attempt_id = ? AND status IN ?
			AND EXISTS (
				SELECT 1 FROM transcode_jobs
				WHERE transcode_jobs.id = transcode_artifacts.job_id
				AND transcode_jobs.lease_token = ?
				AND transcode_jobs.current_attempt_id = ?
				AND transcode_jobs.active_key IS NOT NULL
				AND transcode_jobs.desired_state = ?
				AND transcode_jobs.status IN ?
				AND transcode_jobs.lease_expires_at IS NOT NULL
				AND transcode_jobs.lease_expires_at > ?
			)`,
			artifactID,
			jobID,
			attemptID,
			[]string{"staging", "publishing"},
			leaseToken,
			attemptID,
			"running",
			[]string{"claimed", "running"},
			attestedAt,
		).
		Updates(map[string]any{
			"attestation_version": version,
			"attestation_status":  status,
			"attestation_hash":    hash,
			"attestation_json":    canonical,
			"timeline_start_ms":   timelineStartMS,
			"timeline_end_ms":     timelineEndMS,
			"attested_at":         attestedAt,
			"updated_at":          attestedAt,
		})
	return result.RowsAffected == 1, result.Error
}

// RecordCurrentArtifactAttestation is the playback readiness gate for a live
// continuation. It may attach provisional first-segment evidence only to the
// database-current Attempt with an unexpired Lease. It cannot overwrite final
// verification or change Artifact lifecycle state.
func (r *TranscodeExecutionRepo) RecordCurrentArtifactAttestation(
	artifactID,
	version,
	status,
	hash,
	canonical string,
	timelineStartMS,
	timelineEndMS int64,
	attestedAt time.Time,
) (bool, error) {
	if version == "" || hash == "" || canonical == "" || status != "provisional" {
		return false, nil
	}
	result := r.db.Model(&model.TranscodeArtifactRecord{}).
		Where(
			`id = ? AND status = ? AND attestation_status IN ?
			AND EXISTS (
				SELECT 1 FROM transcode_jobs
				WHERE transcode_jobs.id = transcode_artifacts.job_id
				AND transcode_jobs.current_attempt_id = transcode_artifacts.attempt_id
				AND transcode_jobs.active_key IS NOT NULL
				AND transcode_jobs.desired_state = ?
				AND transcode_jobs.status IN ?
				AND transcode_jobs.lease_token <> ''
				AND transcode_jobs.lease_expires_at IS NOT NULL
				AND transcode_jobs.lease_expires_at > ?
			)`,
			artifactID,
			"staging",
			[]string{"", "provisional"},
			"running",
			[]string{"claimed", "running"},
			attestedAt,
		).
		Updates(map[string]any{
			"attestation_version": version,
			"attestation_status":  status,
			"attestation_hash":    hash,
			"attestation_json":    canonical,
			"timeline_start_ms":   timelineStartMS,
			"timeline_end_ms":     timelineEndMS,
			"attested_at":         attestedAt,
			"updated_at":          attestedAt,
		})
	return result.RowsAffected == 1, result.Error
}

// FindActiveArtifactByEncodingPlanForAttestation returns an otherwise valid
// current Attempt before it has first-segment evidence. Callers must verify and
// persist provisional evidence before exposing any media from this Artifact.
func (r *TranscodeExecutionRepo) FindActiveArtifactByEncodingPlanForAttestation(
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
	var artifact model.TranscodeArtifactRecord
	result := r.db.Table("transcode_artifacts AS a").
		Select("a.*").
		Joins("JOIN transcode_jobs AS j ON j.id = a.job_id").
		Where(
			`a.media_id = ? AND a.profile_id = ? AND a.source_fingerprint = ? AND a.planner_version = ?
			AND a.kind = ? AND a.encoding_plan_version = ? AND a.encoding_plan_hash = ? AND a.status = ?
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
