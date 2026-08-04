package repository

import (
	"errors"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// FinalizePublishedJobStorageReservation records the authoritative immutable
// Artifact size after publication. The operation is idempotent and verifies the
// completed Job plus its exact Attempt/Artifact pair before closing the audit
// row, so a stale Worker cannot publish calibration evidence for a replacement.
func (r *TranscodeExecutionRepo) FinalizePublishedJobStorageReservation(
	jobID,
	attemptID string,
	completedAt time.Time,
) error {
	if r == nil || r.db == nil || jobID == "" || attemptID == "" {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockTranscodeStorageLedger(tx, completedAt); err != nil {
			return err
		}
		var job model.TranscodeJobRecord
		jobResult := tx.Where(
			"id = ? AND status = ? AND active_key IS NULL AND current_attempt_id = ''",
			jobID,
			"completed",
		).First(&job)
		if errors.Is(jobResult.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		if jobResult.Error != nil {
			return jobResult.Error
		}

		var artifact model.TranscodeArtifactRecord
		artifactResult := tx.Where(
			"job_id = ? AND attempt_id = ? AND status = ?",
			jobID,
			attemptID,
			"published",
		).
			Order("published_at DESC, created_at DESC").
			First(&artifact)
		if errors.Is(artifactResult.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		if artifactResult.Error != nil {
			return artifactResult.Error
		}
		publishedAt := completedAt
		if artifact.PublishedAt != nil {
			publishedAt = *artifact.PublishedAt
		}
		return finalizeStorageReservationTx(
			tx,
			jobID,
			attemptID,
			"completed",
			artifact.SizeBytes,
			publishedAt,
		)
	})
}

// ReconcilePublishedStorageReservations repairs the narrow crash window after
// Artifact/Job publication committed but before the calibration audit update.
// Remaining terminal rows without a published Artifact are handled by the
// generic released-row reconciler.
func (r *TranscodeExecutionRepo) ReconcilePublishedStorageReservations(now time.Time) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	type candidate struct {
		JobID     string
		AttemptID string
	}
	var candidates []candidate
	if err := r.db.Table("transcode_storage_reservations AS r").
		Select("r.job_id, a.attempt_id").
		Joins("JOIN transcode_jobs AS j ON j.id = r.job_id").
		Joins("JOIN transcode_artifacts AS a ON a.job_id = r.job_id AND a.status = 'published'").
		Where(
			"r.state = ? AND j.status = ? AND j.active_key IS NULL",
			model.TranscodeStorageReservationActive,
			"completed",
		).
		Order("a.published_at DESC, a.created_at DESC").
		Scan(&candidates).Error; err != nil {
		return 0, err
	}
	seen := make(map[string]struct{}, len(candidates))
	var reconciled int64
	for _, item := range candidates {
		if _, ok := seen[item.JobID]; ok {
			continue
		}
		seen[item.JobID] = struct{}{}
		if err := r.FinalizePublishedJobStorageReservation(item.JobID, item.AttemptID, now); err != nil {
			return reconciled, err
		}
		reconciled++
	}
	return reconciled, nil
}
