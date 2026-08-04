package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrTranscodeStorageReservationCapacity = errors.New("insufficient transcode storage reservation capacity")

type TranscodeStorageReservationCapacityError struct {
	JobID          string
	RequestedBytes int64
	ActiveBytes    int64
	AvailableBytes int64
}

func (e *TranscodeStorageReservationCapacityError) Error() string {
	return fmt.Sprintf(
		"%v: job=%s requested=%d active=%d available=%d",
		ErrTranscodeStorageReservationCapacity,
		e.JobID,
		e.RequestedBytes,
		e.ActiveBytes,
		e.AvailableBytes,
	)
}

func (e *TranscodeStorageReservationCapacityError) Unwrap() error {
	return ErrTranscodeStorageReservationCapacity
}

type TranscodeStorageReservationBudget struct {
	AvailableBytes int64
	SampledAt      time.Time
}

type TranscodeStorageReservationSummary struct {
	ActiveCount  int64 `json:"active_count"`
	ActiveBytes  int64 `json:"active_bytes"`
	WaitingCount int64 `json:"waiting_count"`
}

var activeReservationJobStatuses = []string{"queued", "claimed", "running", "cancel_requested"}

// AcquireJobStorageReservation serializes capacity allocation through the
// singleton ledger row. The UPDATE is intentionally performed before the SUM:
// SQLite obtains its write lock and PostgreSQL obtains a row lock, so two
// server instances cannot both observe and spend the same remaining headroom.
func (r *TranscodeExecutionRepo) AcquireJobStorageReservation(
	jobID string,
	estimatedBytes int64,
	budget TranscodeStorageReservationBudget,
	now time.Time,
) (*model.TranscodeStorageReservationRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("transcode reservation repository is unavailable")
	}
	if jobID == "" || estimatedBytes <= 0 {
		return nil, fmt.Errorf("invalid storage reservation request job=%q bytes=%d", jobID, estimatedBytes)
	}
	if budget.AvailableBytes < 0 {
		budget.AvailableBytes = 0
	}

	var acquired model.TranscodeStorageReservationRecord
	err := r.db.Transaction(func(tx *gorm.DB) error {
		ledgerUpdate := tx.Model(&model.TranscodeStorageLedgerRecord{}).
			Where("id = ?", model.TranscodeStorageLedgerArtifactStore).
			Updates(map[string]any{
				"version":    gorm.Expr("version + 1"),
				"updated_at": now,
			})
		if ledgerUpdate.Error != nil {
			return ledgerUpdate.Error
		}
		if ledgerUpdate.RowsAffected != 1 {
			return fmt.Errorf("transcode storage ledger is missing")
		}

		var job model.TranscodeJobRecord
		if err := tx.First(&job, "id = ?", jobID).Error; err != nil {
			return err
		}
		if job.ActiveKey == nil || job.DesiredState != "running" || !containsReservationJobStatus(job.Status) {
			return fmt.Errorf("transcode job is not reservable: job=%s status=%s desired=%s", job.ID, job.Status, job.DesiredState)
		}

		var existing model.TranscodeStorageReservationRecord
		existingResult := tx.First(&existing, "job_id = ?", jobID)
		if existingResult.Error == nil && existing.State == model.TranscodeStorageReservationActive && existing.ReservedBytes > 0 {
			acquired = existing
			return nil
		}
		if existingResult.Error != nil && !errors.Is(existingResult.Error, gorm.ErrRecordNotFound) {
			return existingResult.Error
		}

		var activeBytes int64
		if err := tx.Table("transcode_storage_reservations AS r").
			Joins("JOIN transcode_jobs AS j ON j.id = r.job_id").
			Where(
				"r.state = ? AND r.job_id <> ? AND j.active_key IS NOT NULL AND j.desired_state = ? AND j.status IN ?",
				model.TranscodeStorageReservationActive,
				jobID,
				"running",
				activeReservationJobStatuses,
			).
			Select("COALESCE(SUM(r.reserved_bytes), 0)").
			Scan(&activeBytes).Error; err != nil {
			return err
		}
		if estimatedBytes > budget.AvailableBytes-activeBytes {
			return &TranscodeStorageReservationCapacityError{
				JobID:          jobID,
				RequestedBytes: estimatedBytes,
				ActiveBytes:    activeBytes,
				AvailableBytes: budget.AvailableBytes,
			}
		}

		acquired = model.TranscodeStorageReservationRecord{
			JobID:          job.ID,
			MediaID:        job.MediaID,
			ProfileID:      job.ProfileID,
			Intent:         job.Intent,
			EstimatedBytes: estimatedBytes,
			ReservedBytes:  estimatedBytes,
			State:          model.TranscodeStorageReservationActive,
			AcquiredAt:     now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "job_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"media_id":        acquired.MediaID,
				"profile_id":      acquired.ProfileID,
				"intent":          acquired.Intent,
				"estimated_bytes": acquired.EstimatedBytes,
				"reserved_bytes":  acquired.ReservedBytes,
				"state":           acquired.State,
				"acquired_at":     acquired.AcquiredAt,
				"released_at":     nil,
				"updated_at":      acquired.UpdatedAt,
			}),
		}).Create(&acquired).Error
	})
	if err != nil {
		return nil, err
	}
	return &acquired, nil
}

func (r *TranscodeExecutionRepo) HasActiveJobStorageReservation(jobID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.TranscodeStorageReservationRecord{}).
		Where("job_id = ? AND state = ? AND reserved_bytes > 0", jobID, model.TranscodeStorageReservationActive).
		Count(&count).Error
	return count == 1, err
}

func (r *TranscodeExecutionRepo) StorageReservationSummary() (TranscodeStorageReservationSummary, error) {
	var summary TranscodeStorageReservationSummary
	row := r.db.Table("transcode_storage_reservations AS r").
		Joins("JOIN transcode_jobs AS j ON j.id = r.job_id").
		Where(
			"r.state = ? AND j.active_key IS NOT NULL AND j.desired_state = ? AND j.status IN ?",
			model.TranscodeStorageReservationActive,
			"running",
			activeReservationJobStatuses,
		).
		Select("COUNT(*) AS active_count, COALESCE(SUM(r.reserved_bytes), 0) AS active_bytes").
		Scan(&summary)
	if row.Error != nil {
		return summary, row.Error
	}
	waiting, err := r.CountQueuedJobsAwaitingStorageReservation()
	if err != nil {
		return summary, err
	}
	summary.WaitingCount = waiting
	return summary, nil
}

func (r *TranscodeExecutionRepo) CountQueuedJobsAwaitingStorageReservation() (int64, error) {
	var count int64
	err := r.db.Model(&model.TranscodeJobRecord{}).
		Where(
			`active_key IS NOT NULL AND status = ? AND desired_state = ?
			AND NOT EXISTS (
				SELECT 1 FROM transcode_storage_reservations AS r
				WHERE r.job_id = transcode_jobs.id AND r.state = ? AND r.reserved_bytes > 0
			)`,
			"queued",
			"running",
			model.TranscodeStorageReservationActive,
		).
		Count(&count).Error
	return count, err
}

func (r *TranscodeExecutionRepo) ReconcileReleasedStorageReservations(now time.Time) (int64, error) {
	result := r.db.Model(&model.TranscodeStorageReservationRecord{}).
		Where(
			`state = ? AND NOT EXISTS (
				SELECT 1 FROM transcode_jobs AS j
				WHERE j.id = transcode_storage_reservations.job_id
				AND j.active_key IS NOT NULL
				AND j.desired_state = ?
				AND j.status IN ?
			)`,
			model.TranscodeStorageReservationActive,
			"running",
			activeReservationJobStatuses,
		).
		Updates(map[string]any{
			"state":       model.TranscodeStorageReservationReleased,
			"released_at": now,
			"updated_at":  now,
		})
	return result.RowsAffected, result.Error
}

func containsReservationJobStatus(status string) bool {
	for _, candidate := range activeReservationJobStatuses {
		if status == candidate {
			return true
		}
	}
	return false
}
