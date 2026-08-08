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
	ActiveCount             int64   `json:"active_count"`
	ActiveBytes             int64   `json:"active_bytes"`
	ReservedBytes           int64   `json:"reserved_bytes"`
	ObservedBytes           int64   `json:"observed_bytes"`
	RemainingBytes          int64   `json:"remaining_bytes"`
	WaitingCount            int64   `json:"waiting_count"`
	CalibrationSamples      int64   `json:"calibration_samples"`
	AverageActualToEstimate float64 `json:"average_actual_to_estimate"`
	AverageAbsoluteError    float64 `json:"average_absolute_error"`
	UnderpredictedCount     int64   `json:"underpredicted_count"`
}

var activeReservationJobStatuses = []string{"queued", "claimed", "running", "cancel_requested"}

func lockTranscodeStorageLedger(tx *gorm.DB, now time.Time) error {
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
	return nil
}

func remainingReservationBytes(reservedBytes, observedBytes int64) int64 {
	if observedBytes >= reservedBytes {
		return 0
	}
	return reservedBytes - observedBytes
}

// AcquireJobStorageReservation serializes capacity allocation through the
// singleton ledger row. ActiveBytes represents only future commitment:
// materialized workspace bytes are already included in the fresh disk/store
// sample and must not be charged a second time.
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
		if err := lockTranscodeStorageLedger(tx, now); err != nil {
			return err
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
			Select("COALESCE(SUM(CASE WHEN r.reserved_bytes > r.observed_bytes THEN r.reserved_bytes - r.observed_bytes ELSE 0 END), 0)").
			Scan(&activeBytes).Error; err != nil {
			return err
		}
		remainingCapacity := budget.AvailableBytes - activeBytes
		if remainingCapacity < 0 || estimatedBytes > remainingCapacity {
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
				"media_id":                 acquired.MediaID,
				"profile_id":               acquired.ProfileID,
				"intent":                   acquired.Intent,
				"attempt_id":               "",
				"estimated_bytes":          acquired.EstimatedBytes,
				"reserved_bytes":           acquired.ReservedBytes,
				"observed_bytes":           0,
				"peak_observed_bytes":      0,
				"final_bytes":              0,
				"prediction_error_bytes":   0,
				"actual_to_estimate_ratio": 0,
				"observation_count":        0,
				"outcome":                  "",
				"state":                    acquired.State,
				"acquired_at":              acquired.AcquiredAt,
				"last_observed_at":         nil,
				"released_at":              nil,
				"updated_at":               acquired.UpdatedAt,
			}),
		}).Create(&acquired).Error
	})
	if err != nil {
		return nil, err
	}
	return &acquired, nil
}

// ObserveOwnedJobStorageReservation refunds only bytes that are known to be
// present in a freshly sampled Artifact Store. The Job Lease and current
// Attempt fence reject stale Workers. Switching Attempt resets ObservedBytes so
// a software fallback receives a complete future commitment again; peak usage
// across attempts is retained for diagnostics.
func (r *TranscodeExecutionRepo) ObserveOwnedJobStorageReservation(
	jobID,
	attemptID,
	leaseToken string,
	observedBytes int64,
	now time.Time,
) (bool, error) {
	if r == nil || r.db == nil || jobID == "" || attemptID == "" || leaseToken == "" || observedBytes < 0 {
		return false, fmt.Errorf("invalid storage reservation observation")
	}
	observed := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockTranscodeStorageLedger(tx, now); err != nil {
			return err
		}
		var job model.TranscodeJobRecord
		result := tx.Where(
			`id = ? AND lease_token = ? AND current_attempt_id = ?
			AND active_key IS NOT NULL AND desired_state = ? AND status IN ?
			AND lease_expires_at IS NOT NULL AND lease_expires_at > ?`,
			jobID,
			leaseToken,
			attemptID,
			"running",
			[]string{"claimed", "running"},
			now,
		).First(&job)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		if result.Error != nil {
			return result.Error
		}

		var reservation model.TranscodeStorageReservationRecord
		reservationResult := tx.First(&reservation, "job_id = ? AND state = ?", jobID, model.TranscodeStorageReservationActive)
		if errors.Is(reservationResult.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		if reservationResult.Error != nil {
			return reservationResult.Error
		}

		currentObserved := reservation.ObservedBytes
		if reservation.AttemptID != attemptID {
			currentObserved = observedBytes
		} else if observedBytes > currentObserved {
			currentObserved = observedBytes
		}
		peakObserved := reservation.PeakObservedBytes
		if observedBytes > peakObserved {
			peakObserved = observedBytes
		}
		update := tx.Model(&model.TranscodeStorageReservationRecord{}).
			Where("job_id = ? AND state = ?", jobID, model.TranscodeStorageReservationActive).
			Updates(map[string]any{
				"attempt_id":          attemptID,
				"observed_bytes":      currentObserved,
				"peak_observed_bytes": peakObserved,
				"observation_count":   gorm.Expr("observation_count + 1"),
				"last_observed_at":    now,
				"updated_at":          now,
			})
		if update.Error != nil {
			return update.Error
		}
		observed = update.RowsAffected == 1
		return nil
	})
	return observed, err
}

func (r *TranscodeExecutionRepo) HasActiveJobStorageReservation(jobID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.TranscodeStorageReservationRecord{}).
		Where("job_id = ? AND state = ? AND reserved_bytes > 0", jobID, model.TranscodeStorageReservationActive).
		Count(&count).Error
	return count == 1, err
}

func finalizeStorageReservationTx(
	tx *gorm.DB,
	jobID,
	attemptID,
	outcome string,
	finalBytes int64,
	completedAt time.Time,
) error {
	var reservation model.TranscodeStorageReservationRecord
	result := tx.First(&reservation, "job_id = ?", jobID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil
	}
	if result.Error != nil {
		return result.Error
	}
	observedBytes := reservation.ObservedBytes
	peakObservedBytes := reservation.PeakObservedBytes
	predictionErrorBytes := int64(0)
	actualToEstimateRatio := float64(0)
	if finalBytes > 0 {
		observedBytes = finalBytes
		if finalBytes > peakObservedBytes {
			peakObservedBytes = finalBytes
		}
		predictionErrorBytes = finalBytes - reservation.EstimatedBytes
		if reservation.EstimatedBytes > 0 {
			actualToEstimateRatio = float64(finalBytes) / float64(reservation.EstimatedBytes)
		}
	}
	updates := map[string]any{
		"state":                    model.TranscodeStorageReservationReleased,
		"outcome":                  outcome,
		"observed_bytes":           observedBytes,
		"peak_observed_bytes":      peakObservedBytes,
		"final_bytes":              finalBytes,
		"prediction_error_bytes":   predictionErrorBytes,
		"actual_to_estimate_ratio": actualToEstimateRatio,
		"released_at":              completedAt,
		"updated_at":               completedAt,
	}
	if attemptID != "" {
		updates["attempt_id"] = attemptID
	}
	return tx.Model(&model.TranscodeStorageReservationRecord{}).
		Where("job_id = ?", jobID).
		Updates(updates).Error
}

// ReleaseJobStorageReservation closes failed/cancelled audit rows after the Job
// terminal transition. Published output is finalized atomically by
// CommitArtifactPublishAndCompleteJob with the authoritative Artifact size.
func (r *TranscodeExecutionRepo) ReleaseJobStorageReservation(jobID, outcome string, releasedAt time.Time) error {
	if r == nil || r.db == nil || jobID == "" {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockTranscodeStorageLedger(tx, releasedAt); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.TranscodeJobRecord{}).
			Where("id = ? AND active_key IS NULL AND status = ?", jobID, outcome).
			Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return nil
		}
		return finalizeStorageReservationTx(tx, jobID, "", outcome, 0, releasedAt)
	})
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
		Select(`COUNT(*) AS active_count,
			COALESCE(SUM(r.reserved_bytes), 0) AS reserved_bytes,
			COALESCE(SUM(r.observed_bytes), 0) AS observed_bytes,
			COALESCE(SUM(CASE WHEN r.reserved_bytes > r.observed_bytes THEN r.reserved_bytes - r.observed_bytes ELSE 0 END), 0) AS remaining_bytes`).
		Scan(&summary)
	if row.Error != nil {
		return summary, row.Error
	}
	summary.ActiveBytes = summary.RemainingBytes

	waiting, err := r.CountQueuedJobsAwaitingStorageReservation()
	if err != nil {
		return summary, err
	}
	summary.WaitingCount = waiting

	type calibrationAggregate struct {
		CalibrationSamples      int64
		AverageActualToEstimate float64
		AverageAbsoluteError    float64
		UnderpredictedCount     int64
	}
	var calibration calibrationAggregate
	if err := r.db.Model(&model.TranscodeStorageReservationRecord{}).
		Where("state = ? AND outcome = ? AND final_bytes > 0 AND estimated_bytes > 0", model.TranscodeStorageReservationReleased, "completed").
		Select(`COUNT(*) AS calibration_samples,
			COALESCE(AVG(actual_to_estimate_ratio), 0) AS average_actual_to_estimate,
			COALESCE(AVG(ABS(prediction_error_bytes) * 1.0 / estimated_bytes), 0) AS average_absolute_error,
			COALESCE(SUM(CASE WHEN prediction_error_bytes > 0 THEN 1 ELSE 0 END), 0) AS underpredicted_count`).
		Scan(&calibration).Error; err != nil {
		return summary, err
	}
	summary.CalibrationSamples = calibration.CalibrationSamples
	summary.AverageActualToEstimate = calibration.AverageActualToEstimate
	summary.AverageAbsoluteError = calibration.AverageAbsoluteError
	summary.UnderpredictedCount = calibration.UnderpredictedCount
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
			"outcome":     gorm.Expr("CASE WHEN outcome = '' THEN 'reconciled' ELSE outcome END"),
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
