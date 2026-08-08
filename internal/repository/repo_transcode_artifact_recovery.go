package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

// AbandonUnownedArtifacts reconciles staging/publishing rows after startup.
// A row remains active only when the owning Job still points at the same
// Attempt and has a live Lease. Valid Leases held by other instances are kept.
func (r *TranscodeExecutionRepo) AbandonUnownedArtifacts(now time.Time) (int64, error) {
	result := r.db.Model(&model.TranscodeArtifactRecord{}).
		Where(
			`status IN ? AND NOT EXISTS (
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
			[]string{"staging", "publishing"},
			"running",
			[]string{"claimed", "running"},
			now,
		).
		Updates(map[string]any{
			"status":        "abandoned",
			"error_code":    "startup_reconciliation",
			"error_message": "Artifact no longer has a current live Job Lease",
			"updated_at":    now,
		})
	return result.RowsAffected, result.Error
}

func (r *TranscodeExecutionRepo) ListArtifactsByLegacyTaskID(legacyTaskID string) ([]model.TranscodeArtifactRecord, error) {
	var artifacts []model.TranscodeArtifactRecord
	err := r.db.Table("transcode_artifacts AS a").
		Select("a.*").
		Joins("JOIN transcode_jobs AS j ON j.id = a.job_id").
		Where("j.legacy_task_id = ?", legacyTaskID).
		Order("a.created_at ASC").
		Find(&artifacts).Error
	return artifacts, err
}

func (r *TranscodeExecutionRepo) ListTerminalArtifactsBefore(cutoff time.Time, limit int) ([]model.TranscodeArtifactRecord, error) {
	if limit <= 0 {
		limit = 500
	}
	var artifacts []model.TranscodeArtifactRecord
	err := r.db.Where(
		"status IN ? AND updated_at < ?",
		[]string{"failed", "cancelled", "abandoned", "superseded", "expired"},
		cutoff,
	).
		Order("updated_at ASC, id ASC").
		Limit(limit).
		Find(&artifacts).Error
	return artifacts, err
}

func (r *TranscodeExecutionRepo) DeleteArtifactByID(artifactID string) error {
	return r.db.Delete(&model.TranscodeArtifactRecord{}, "id = ?", artifactID).Error
}
