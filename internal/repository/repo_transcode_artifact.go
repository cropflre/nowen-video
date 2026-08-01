package repository

import (
	"errors"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

var errArtifactOwnershipLost = errors.New("artifact publish ownership lost")

func (r *TranscodeExecutionRepo) UpdateAttemptWorkspaceAndCommand(attemptID, workspacePath, commandJSON string, updatedAt time.Time) error {
	return r.db.Model(&model.TranscodeAttemptRecord{}).
		Where("id = ? AND status = ?", attemptID, "preparing").
		Updates(map[string]any{
			"workspace_path": workspacePath,
			"command_json":   commandJSON,
			"updated_at":     updatedAt,
		}).Error
}

// FindReadableHLSArtifact resolves the only Artifact that may currently be read
// for a media/profile/source/plan tuple. A staging Artifact is returned only
// while it is the Job's current Attempt and the owning Lease is still valid.
func (r *TranscodeExecutionRepo) FindReadableHLSArtifact(
	mediaID,
	profileID,
	sourceFingerprint,
	plannerVersion string,
	now time.Time,
) (*model.TranscodeArtifactRecord, error) {
	var active model.TranscodeArtifactRecord
	activeErr := r.db.Table("transcode_artifacts AS a").
		Select("a.*").
		Joins("JOIN transcode_jobs AS j ON j.id = a.job_id").
		Where(
			`a.media_id = ? AND a.profile_id = ? AND a.source_fingerprint = ? AND a.planner_version = ?
			AND a.kind = ? AND a.status IN ?
			AND a.attempt_id = j.current_attempt_id
			AND j.active_key IS NOT NULL AND j.desired_state = ?
			AND j.status IN ? AND j.lease_token <> ''
			AND j.lease_expires_at IS NOT NULL AND j.lease_expires_at > ?`,
			mediaID,
			profileID,
			sourceFingerprint,
			plannerVersion,
			"hls_variant",
			[]string{"staging", "publishing"},
			"running",
			[]string{"claimed", "running"},
			now,
		).
		Order("a.created_at DESC").
		First(&active).Error
	if activeErr == nil {
		return &active, nil
	}
	if !errors.Is(activeErr, gorm.ErrRecordNotFound) {
		return nil, activeErr
	}

	var published model.TranscodeArtifactRecord
	publishedErr := r.db.Where(
		"media_id = ? AND profile_id = ? AND source_fingerprint = ? AND planner_version = ? AND kind = ? AND status = ?",
		mediaID,
		profileID,
		sourceFingerprint,
		plannerVersion,
		"hls_variant",
		"published",
	).
		Order("published_at DESC, created_at DESC").
		First(&published).Error
	if publishedErr != nil {
		return nil, publishedErr
	}
	return &published, nil
}

func (r *TranscodeExecutionRepo) FindPublishedHLSArtifact(
	mediaID,
	profileID,
	sourceFingerprint,
	plannerVersion string,
) (*model.TranscodeArtifactRecord, error) {
	var artifact model.TranscodeArtifactRecord
	err := r.db.Where(
		"media_id = ? AND profile_id = ? AND source_fingerprint = ? AND planner_version = ? AND kind = ? AND status = ?",
		mediaID,
		profileID,
		sourceFingerprint,
		plannerVersion,
		"hls_variant",
		"published",
	).
		Order("published_at DESC, created_at DESC").
		First(&artifact).Error
	if err != nil {
		return nil, err
	}
	return &artifact, nil
}

// PrepareArtifactPublish fences the filesystem rename behind the current Job
// Lease and Attempt. It writes the immutable target path but does not expose the
// Artifact as published yet.
func (r *TranscodeExecutionRepo) PrepareArtifactPublish(
	jobID,
	attemptID,
	artifactID,
	leaseToken,
	publishedPath,
	manifestPath string,
	now time.Time,
) (bool, error) {
	result := r.db.Model(&model.TranscodeArtifactRecord{}).
		Where(
			`id = ? AND job_id = ? AND attempt_id = ? AND status = ?
			AND EXISTS (
				SELECT 1 FROM transcode_jobs
				WHERE transcode_jobs.id = transcode_artifacts.job_id
				AND transcode_jobs.lease_token = ?
				AND transcode_jobs.current_attempt_id = ?
				AND transcode_jobs.active_key IS NOT NULL
				AND transcode_jobs.desired_state = ?
				AND transcode_jobs.status = ?
				AND transcode_jobs.lease_expires_at IS NOT NULL
				AND transcode_jobs.lease_expires_at > ?
			)`,
			artifactID,
			jobID,
			attemptID,
			"staging",
			leaseToken,
			attemptID,
			"running",
			"running",
			now,
		).
		Updates(map[string]any{
			"status":        "publishing",
			"path":          publishedPath,
			"manifest_path": manifestPath,
			"updated_at":    now,
		})
	return result.RowsAffected == 1, result.Error
}

// CommitArtifactPublishAndCompleteJob is the single database commit point for
// Artifact visibility and Job completion. Any ownership change rolls back the
// transaction, leaving a filesystem orphan that is not returned by Resolver.
func (r *TranscodeExecutionRepo) CommitArtifactPublishAndCompleteJob(
	jobID,
	attemptID,
	artifactID,
	leaseToken string,
	sizeBytes,
	durationMS int64,
	completedAt time.Time,
) (bool, error) {
	committed := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		artifactResult := tx.Model(&model.TranscodeArtifactRecord{}).
			Where(
				`id = ? AND job_id = ? AND attempt_id = ? AND status = ?
				AND EXISTS (
					SELECT 1 FROM transcode_jobs
					WHERE transcode_jobs.id = transcode_artifacts.job_id
					AND transcode_jobs.lease_token = ?
					AND transcode_jobs.current_attempt_id = ?
					AND transcode_jobs.active_key IS NOT NULL
					AND transcode_jobs.desired_state = ?
					AND transcode_jobs.status = ?
					AND transcode_jobs.lease_expires_at IS NOT NULL
					AND transcode_jobs.lease_expires_at > ?
				)`,
				artifactID,
				jobID,
				attemptID,
				"publishing",
				leaseToken,
				attemptID,
				"running",
				"running",
				completedAt,
			).
			Updates(map[string]any{
				"status":        "published",
				"temp_path":     "",
				"size_bytes":    sizeBytes,
				"duration_ms":   durationMS,
				"published_at":  completedAt,
				"error_code":    "",
				"error_message": "",
				"updated_at":    completedAt,
			})
		if artifactResult.Error != nil {
			return artifactResult.Error
		}
		if artifactResult.RowsAffected != 1 {
			return errArtifactOwnershipLost
		}

		jobResult := tx.Model(&model.TranscodeJobRecord{}).
			Where(
				"id = ? AND lease_token = ? AND current_attempt_id = ? AND active_key IS NOT NULL AND desired_state = ? AND status = ? AND lease_expires_at IS NOT NULL AND lease_expires_at > ?",
				jobID,
				leaseToken,
				attemptID,
				"running",
				"running",
				completedAt,
			).
			Updates(terminalJobUpdates("completed", completedAt))
		if jobResult.Error != nil {
			return jobResult.Error
		}
		if jobResult.RowsAffected != 1 {
			return errArtifactOwnershipLost
		}

		var artifact model.TranscodeArtifactRecord
		if err := tx.First(&artifact, "id = ?", artifactID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.TranscodeArtifactRecord{}).
			Where(
				"id <> ? AND media_id = ? AND profile_id = ? AND source_fingerprint = ? AND planner_version = ? AND kind = ? AND status = ?",
				artifactID,
				artifact.MediaID,
				artifact.ProfileID,
				artifact.SourceFingerprint,
				artifact.PlannerVersion,
				artifact.Kind,
				"published",
			).
			Updates(map[string]any{"status": "superseded", "updated_at": completedAt}).Error; err != nil {
			return err
		}
		committed = true
		return nil
	})
	if errors.Is(err, errArtifactOwnershipLost) {
		return false, nil
	}
	return committed, err
}

func (r *TranscodeExecutionRepo) MarkOwnedArtifactTerminal(
	jobID,
	attemptID,
	artifactID,
	leaseToken,
	status,
	errorCode,
	errorMessage string,
	completedAt time.Time,
) (bool, error) {
	result := r.db.Model(&model.TranscodeArtifactRecord{}).
		Where(
			`id = ? AND job_id = ? AND attempt_id = ? AND status IN ?
			AND EXISTS (
				SELECT 1 FROM transcode_jobs
				WHERE transcode_jobs.id = transcode_artifacts.job_id
				AND transcode_jobs.lease_token = ?
				AND transcode_jobs.current_attempt_id = ?
				AND transcode_jobs.active_key IS NOT NULL
			)`,
			artifactID,
			jobID,
			attemptID,
			[]string{"staging", "publishing"},
			leaseToken,
			attemptID,
		).
		Updates(map[string]any{
			"status":        status,
			"error_code":    errorCode,
			"error_message": errorMessage,
			"updated_at":    completedAt,
		})
	return result.RowsAffected == 1, result.Error
}

func (r *TranscodeExecutionRepo) MarkArtifactAbandoned(artifactID, errorCode, errorMessage string, updatedAt time.Time) error {
	return r.db.Model(&model.TranscodeArtifactRecord{}).
		Where("id = ? AND status IN ?", artifactID, []string{"staging", "publishing"}).
		Updates(map[string]any{
			"status":        "abandoned",
			"error_code":    errorCode,
			"error_message": errorMessage,
			"updated_at":    updatedAt,
		}).Error
}

func (r *TranscodeExecutionRepo) AbandonArtifactsForAttempt(attemptID, errorCode, errorMessage string, updatedAt time.Time) error {
	if attemptID == "" {
		return nil
	}
	return r.db.Model(&model.TranscodeArtifactRecord{}).
		Where("attempt_id = ? AND status IN ?", attemptID, []string{"staging", "publishing"}).
		Updates(map[string]any{
			"status":        "abandoned",
			"error_code":    errorCode,
			"error_message": errorMessage,
			"updated_at":    updatedAt,
		}).Error
}

func (r *TranscodeExecutionRepo) ArtifactStatusCounts() (map[string]int64, error) {
	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	if err := r.db.Model(&model.TranscodeArtifactRecord{}).
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, item := range rows {
		counts[item.Status] = item.Count
	}
	return counts, nil
}
