package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

type TranscodeExecutionRepo struct {
	db *gorm.DB
}

func NewTranscodeExecutionRepo(db *gorm.DB) *TranscodeExecutionRepo {
	return &TranscodeExecutionRepo{db: db}
}

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

func (r *TranscodeExecutionRepo) SetJobRunning(jobID, attemptID string, startedAt time.Time) error {
	return r.db.Model(&model.TranscodeJobRecord{}).Where("id = ?", jobID).Updates(map[string]any{
		"status":             "running",
		"desired_state":      "running",
		"current_attempt_id": attemptID,
		"updated_at":         startedAt,
	}).Error
}

func (r *TranscodeExecutionRepo) RequestCancellation(jobID string, requestedAt time.Time) error {
	return r.db.Model(&model.TranscodeJobRecord{}).Where("id = ?", jobID).Updates(map[string]any{
		"status":              "cancel_requested",
		"desired_state":       "cancelled",
		"cancel_requested_at": requestedAt,
		"updated_at":          requestedAt,
	}).Error
}

func (r *TranscodeExecutionRepo) CompleteJob(jobID, status string, completedAt time.Time) error {
	return r.db.Model(&model.TranscodeJobRecord{}).Where("id = ?", jobID).Updates(map[string]any{
		"status":             status,
		"active_key":         nil,
		"current_attempt_id": "",
		"completed_at":       completedAt,
		"updated_at":         completedAt,
	}).Error
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
