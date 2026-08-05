package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

type TranscodeExecutionRepo struct {
	db *gorm.DB
}

func NewTranscodeExecutionRepo(db *gorm.DB) *TranscodeExecutionRepo {
	return &TranscodeExecutionRepo{db: db}
}

func (r *TranscodeExecutionRepo) DB() *gorm.DB { return r.db }

func (r *TranscodeRepo) DB() *gorm.DB { return r.db }

func (r *TranscodeExecutionRepo) CreateJob(job *model.TranscodeJobRecord) error {
	return r.db.Create(job).Error
}

func (r *TranscodeExecutionRepo) FindJobByID(id string) (*model.TranscodeJobRecord, error) {
	var job model.TranscodeJobRecord
	if err := r.db.First(&job, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *TranscodeExecutionRepo) FindActiveByKey(key string) (*model.TranscodeJobRecord, error) {
	var job model.TranscodeJobRecord
	err := r.db.Where("active_key = ?", key).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *TranscodeExecutionRepo) FindActiveByLegacyTaskID(taskID string) (*model.TranscodeJobRecord, error) {
	var job model.TranscodeJobRecord
	err := r.db.Where("legacy_task_id = ? AND active_key IS NOT NULL", taskID).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *TranscodeExecutionRepo) ListActiveJobs() ([]model.TranscodeJobRecord, error) {
	var jobs []model.TranscodeJobRecord
	err := r.db.Where("active_key IS NOT NULL").Order("priority DESC, created_at ASC").Find(&jobs).Error
	return jobs, err
}

// PromoteQueuedJob raises priority only while the job is still unclaimed. The
// greater-than predicate makes repeated interactive requests idempotent and
// prevents a late low-priority caller from lowering an existing job.
func (r *TranscodeExecutionRepo) PromoteQueuedJob(jobID string, priority int, updatedAt time.Time) (bool, error) {
	result := r.db.Model(&model.TranscodeJobRecord{}).
		Where(
			"id = ? AND status = ? AND desired_state = ? AND lease_token = '' AND active_key IS NOT NULL AND priority < ?",
			jobID,
			"queued",
			"running",
			priority,
		).
		Updates(map[string]any{
			"priority":   priority,
			"updated_at": updatedAt,
		})
	return result.RowsAffected == 1, result.Error
}

func (r *TranscodeExecutionRepo) ClaimJob(jobID, workerID string, now time.Time, leaseDuration time.Duration) (*model.TranscodeJobRecord, bool, error) {
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}
	leaseToken := uuid.NewString()
	leaseExpiresAt := now.Add(leaseDuration)
	result := r.db.Model(&model.TranscodeJobRecord{}).
		Where(
			"id = ? AND status = ? AND desired_state = ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?)",
			jobID,
			"queued",
			"running",
			now,
		).
		Updates(map[string]any{
			"status":            "claimed",
			"worker_id":         workerID,
			"lease_token":       leaseToken,
			"claimed_at":        now,
			"last_heartbeat_at": now,
			"lease_expires_at":  leaseExpiresAt,
			"updated_at":        now,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, false, nil
	}
	job, err := r.FindJobByID(jobID)
	if err != nil {
		return nil, false, err
	}
	return job, true, nil
}

func (r *TranscodeExecutionRepo) RenewJobLease(jobID, leaseToken string, now time.Time, leaseDuration time.Duration) (bool, error) {
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}
	result := r.db.Model(&model.TranscodeJobRecord{}).
		Where(
			"id = ? AND lease_token = ? AND desired_state = ? AND status IN ? AND lease_expires_at > ?",
			jobID,
			leaseToken,
			"running",
			[]string{"claimed", "running"},
			now,
		).
		Updates(map[string]any{
			"last_heartbeat_at": now,
			"lease_expires_at":  now.Add(leaseDuration),
			"updated_at":        now,
		})
	return result.RowsAffected == 1, result.Error
}

// SetJobRunning also advances CurrentAttemptID for hardware fallback attempts.
// A job may already be running, but the same lease token must still own it.
func (r *TranscodeExecutionRepo) SetJobRunning(jobID, attemptID, leaseToken string, startedAt time.Time) (bool, error) {
	result := r.db.Model(&model.TranscodeJobRecord{}).
		Where(
			"id = ? AND lease_token = ? AND status IN ? AND desired_state = ? AND lease_expires_at > ?",
			jobID,
			leaseToken,
			[]string{"claimed", "running"},
			"running",
			startedAt,
		).
		Updates(map[string]any{
			"status":             "running",
			"current_attempt_id": attemptID,
			"last_heartbeat_at":  startedAt,
			"updated_at":         startedAt,
		})
	return result.RowsAffected == 1, result.Error
}

func (r *TranscodeExecutionRepo) RequestCancellation(jobID string, requestedAt time.Time) error {
	return r.db.Model(&model.TranscodeJobRecord{}).
		Where("id = ? AND active_key IS NOT NULL", jobID).
		Updates(map[string]any{
			"status":              "cancel_requested",
			"desired_state":       "cancelled",
			"cancel_requested_at": requestedAt,
			"updated_at":          requestedAt,
		}).Error
}

func terminalJobUpdates(status string, completedAt time.Time) map[string]any {
	return map[string]any{
		"status":             status,
		"active_key":         nil,
		"current_attempt_id": "",
		"worker_id":          "",
		"lease_token":        "",
		"claimed_at":         nil,
		"last_heartbeat_at":  nil,
		"lease_expires_at":   nil,
		"completed_at":       completedAt,
		"updated_at":         completedAt,
	}
}

func queuedJobUpdates(updatedAt time.Time) map[string]any {
	return map[string]any{
		"status":              "queued",
		"desired_state":       "running",
		"current_attempt_id":  "",
		"worker_id":           "",
		"lease_token":         "",
		"claimed_at":          nil,
		"last_heartbeat_at":   nil,
		"lease_expires_at":    nil,
		"cancel_requested_at": nil,
		"completed_at":        nil,
		"updated_at":          updatedAt,
	}
}

func (r *TranscodeExecutionRepo) CompleteJob(jobID, status string, completedAt time.Time) error {
	return r.db.Model(&model.TranscodeJobRecord{}).
		Where("id = ?", jobID).
		Updates(terminalJobUpdates(status, completedAt)).Error
}

func (r *TranscodeExecutionRepo) CompleteQueuedJob(jobID, status string, completedAt time.Time) (bool, error) {
	result := r.db.Model(&model.TranscodeJobRecord{}).
		Where("id = ? AND status = ? AND lease_token = '' AND active_key IS NOT NULL", jobID, "queued").
		Updates(terminalJobUpdates(status, completedAt))
	return result.RowsAffected == 1, result.Error
}

// RequeueUnleasedJob upgrades active rows created before Lease ownership was
// introduced. Only desired_state=running rows are recoverable; cancelled rows
// must be finalized instead of being revived.
func (r *TranscodeExecutionRepo) RequeueUnleasedJob(jobID string, now time.Time) (bool, error) {
	result := r.db.Model(&model.TranscodeJobRecord{}).
		Where(
			"id = ? AND active_key IS NOT NULL AND desired_state = ? AND lease_token = '' AND status IN ?",
			jobID,
			"running",
			[]string{"claimed", "running"},
		).
		Updates(queuedJobUpdates(now))
	return result.RowsAffected == 1, result.Error
}

// RequeueLeasedJob releases ownership during graceful shutdown. The lease token
// fences a late worker result, while ActiveKey remains allocated to the same Job.
func (r *TranscodeExecutionRepo) RequeueLeasedJob(jobID, leaseToken string, now time.Time) (bool, error) {
	result := r.db.Model(&model.TranscodeJobRecord{}).
		Where(
			"id = ? AND lease_token = ? AND active_key IS NOT NULL AND desired_state = ? AND status IN ?",
			jobID,
			leaseToken,
			"running",
			[]string{"claimed", "running"},
		).
		Updates(queuedJobUpdates(now))
	return result.RowsAffected == 1, result.Error
}

// RequeueExpiredLease returns abandoned work to the durable queue instead of
// losing it as a terminal failure. Cancellation remains terminal and is handled
// by CompleteExpiredLease.
func (r *TranscodeExecutionRepo) RequeueExpiredLease(jobID, leaseToken string, now time.Time) (bool, error) {
	result := r.db.Model(&model.TranscodeJobRecord{}).
		Where(
			"id = ? AND lease_token = ? AND active_key IS NOT NULL AND desired_state = ? AND lease_expires_at IS NOT NULL AND lease_expires_at <= ? AND status IN ?",
			jobID,
			leaseToken,
			"running",
			now,
			[]string{"claimed", "running"},
		).
		Updates(queuedJobUpdates(now))
	return result.RowsAffected == 1, result.Error
}

// CompleteLeasedJob rejects both replaced and already-expired leases. Recovery
// therefore wins even when the old process exits just before the reaper runs.
func (r *TranscodeExecutionRepo) CompleteLeasedJob(jobID, leaseToken, status string, completedAt time.Time) (bool, error) {
	result := r.db.Model(&model.TranscodeJobRecord{}).
		Where(
			"id = ? AND lease_token = ? AND active_key IS NOT NULL AND lease_expires_at IS NOT NULL AND lease_expires_at > ?",
			jobID,
			leaseToken,
			completedAt,
		).
		Updates(terminalJobUpdates(status, completedAt))
	return result.RowsAffected == 1, result.Error
}

func (r *TranscodeExecutionRepo) ListExpiredLeases(now time.Time) ([]model.TranscodeJobRecord, error) {
	var jobs []model.TranscodeJobRecord
	err := r.db.Where(
		"active_key IS NOT NULL AND lease_token <> '' AND lease_expires_at IS NOT NULL AND lease_expires_at <= ? AND status IN ?",
		now,
		[]string{"claimed", "running", "cancel_requested"},
	).Order("lease_expires_at ASC").Find(&jobs).Error
	return jobs, err
}

func (r *TranscodeExecutionRepo) CompleteExpiredLease(jobID, leaseToken, status string, now time.Time) (bool, error) {
	result := r.db.Model(&model.TranscodeJobRecord{}).
		Where(
			"id = ? AND lease_token = ? AND active_key IS NOT NULL AND lease_expires_at IS NOT NULL AND lease_expires_at <= ? AND status IN ?",
			jobID,
			leaseToken,
			now,
			[]string{"claimed", "running", "cancel_requested"},
		).
		Updates(terminalJobUpdates(status, now))
	return result.RowsAffected == 1, result.Error
}

func (r *TranscodeExecutionRepo) NextAttemptNumber(jobID string) (int, error) {
	var maximum int
	err := r.db.Model(&model.TranscodeAttemptRecord{}).
		Select("COALESCE(MAX(number), 0)").
		Where("job_id = ?", jobID).
		Scan(&maximum).Error
	if err != nil {
		return 0, err
	}
	return maximum + 1, nil
}

func (r *TranscodeExecutionRepo) CreateAttempt(attempt *model.TranscodeAttemptRecord) error {
	return r.db.Create(attempt).Error
}

func (r *TranscodeExecutionRepo) MarkAttemptStarted(attemptID string, pid int, startedAt time.Time) error {
	return r.db.Model(&model.TranscodeAttemptRecord{}).Where("id = ?", attemptID).Updates(map[string]any{
		"status":       "running",
		"pid":          pid,
		"started_at":   startedAt,
		"heartbeat_at": startedAt,
		"updated_at":   startedAt,
	}).Error
}

func (r *TranscodeExecutionRepo) TouchAttempt(attemptID string, heartbeatAt time.Time) error {
	return r.db.Model(&model.TranscodeAttemptRecord{}).Where("id = ?", attemptID).Updates(map[string]any{
		"heartbeat_at": heartbeatAt,
		"updated_at":   heartbeatAt,
	}).Error
}

func (r *TranscodeExecutionRepo) CompleteAttempt(attemptID, status string, exitCode int, stderrTail, errorCode, errorMessage string, completedAt time.Time) error {
	return r.db.Model(&model.TranscodeAttemptRecord{}).Where("id = ?", attemptID).Updates(map[string]any{
		"status":        status,
		"exit_code":     exitCode,
		"stderr_tail":   stderrTail,
		"error_code":    errorCode,
		"error_message": errorMessage,
		"completed_at":  completedAt,
		"heartbeat_at":  completedAt,
		"updated_at":    completedAt,
	}).Error
}

func (r *TranscodeExecutionRepo) CreateArtifact(artifact *model.TranscodeArtifactRecord) error {
	return r.db.Create(artifact).Error
}

func (r *TranscodeExecutionRepo) DeleteArtifactByJobAndKind(jobID, kind, profileID string) error {
	return r.db.Where("job_id = ? AND kind = ? AND profile_id = ?", jobID, kind, profileID).
		Delete(&model.TranscodeArtifactRecord{}).Error
}
