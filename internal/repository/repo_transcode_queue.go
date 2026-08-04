package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func (r *TranscodeExecutionRepo) CountQueuedJobs() (int64, error) {
	var count int64
	err := r.db.Model(&model.TranscodeJobRecord{}).
		Where("active_key IS NOT NULL AND status = ? AND desired_state = ?", "queued", "running").
		Count(&count).Error
	return count, err
}

func (r *TranscodeExecutionRepo) ListQueuedJobCandidates(now time.Time, scanLimit int) ([]string, error) {
	if scanLimit <= 0 {
		scanLimit = 16
	}
	var candidateIDs []string
	err := r.db.Model(&model.TranscodeJobRecord{}).
		Where(
			"active_key IS NOT NULL AND status = ? AND desired_state = ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?)",
			"queued",
			"running",
			now,
		).
		Order("priority DESC, created_at ASC, id ASC").
		Limit(scanLimit).
		Pluck("id", &candidateIDs).Error
	return candidateIDs, err
}

// ClaimNextQueuedJob remains available for repository-level ownership tests.
// Runtime workers use ListQueuedJobCandidates so the service can acquire a
// durable storage Reservation before calling ClaimJob.
func (r *TranscodeExecutionRepo) ClaimNextQueuedJob(workerID string, now time.Time, leaseDuration time.Duration, scanLimit int) (*model.TranscodeJobRecord, bool, error) {
	candidateIDs, err := r.ListQueuedJobCandidates(now, scanLimit)
	if err != nil {
		return nil, false, err
	}
	for _, jobID := range candidateIDs {
		job, claimed, claimErr := r.ClaimJob(jobID, workerID, now, leaseDuration)
		if claimErr != nil {
			return nil, false, claimErr
		}
		if claimed {
			return job, true, nil
		}
	}
	return nil, false, nil
}

// LoadJobPayload reconstructs the process payload from durable rows. The
// transcode Job remains the scheduling authority; the legacy task is only the
// management API projection and Media is loaded by the Job's immutable ID.
func (r *TranscodeExecutionRepo) LoadJobPayload(job *model.TranscodeJobRecord) (*model.TranscodeTask, *model.Media, error) {
	if job == nil {
		return nil, nil, fmt.Errorf("transcode job is nil")
	}
	if job.LegacyTaskID == nil || strings.TrimSpace(*job.LegacyTaskID) == "" {
		return nil, nil, fmt.Errorf("transcode job %s has no legacy task projection", job.ID)
	}
	taskID := strings.TrimSpace(*job.LegacyTaskID)
	var task model.TranscodeTask
	if err := r.db.First(&task, "id = ?", taskID).Error; err != nil {
		return nil, nil, fmt.Errorf("load transcode task %s: %w", taskID, err)
	}
	if task.MediaID != job.MediaID {
		return nil, nil, fmt.Errorf("transcode job %s media mismatch: job=%s task=%s", job.ID, job.MediaID, task.MediaID)
	}
	var media model.Media
	if err := r.db.First(&media, "id = ?", job.MediaID).Error; err != nil {
		return nil, nil, fmt.Errorf("load media %s: %w", job.MediaID, err)
	}
	return &task, &media, nil
}

// CompleteUnleasedJob finalizes work cancelled or rejected before a Worker
// acquired a Lease. The predicates prevent this path from racing a successful
// Claim in another process.
func (r *TranscodeExecutionRepo) CompleteUnleasedJob(jobID, status string, completedAt time.Time) (bool, error) {
	result := r.db.Model(&model.TranscodeJobRecord{}).
		Where(
			"id = ? AND active_key IS NOT NULL AND lease_token = '' AND status IN ?",
			jobID,
			[]string{"queued", "cancel_requested"},
		).
		Updates(terminalJobUpdates(status, completedAt))
	return result.RowsAffected == 1, result.Error
}
