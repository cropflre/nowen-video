package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

const (
	ArtifactCleanupPending                 = "pending"
	ArtifactCleanupClaimed                 = "claimed"
	ArtifactCleanupRetryWait               = "retry_wait"
	ArtifactCleanupBlocked                 = "blocked"
	ArtifactCleanupCompleted               = "completed"
	ArtifactCleanupRollbackCompleted       = "rollback_completed"
	LegacyTranscodeArtifactMigrationSource = "legacy_transcode_task_v1"
	LegacyTranscodeArtifactKind            = "legacy_hls_directory"
)

var artifactCleanupTerminalStatuses = []string{
	"failed",
	"cancelled",
	"abandoned",
	"superseded",
	"expired",
}

var artifactCleanupManagedStates = []string{
	ArtifactCleanupPending,
	ArtifactCleanupClaimed,
	ArtifactCleanupRetryWait,
	ArtifactCleanupBlocked,
}

func artifactCleanupEligibilityQuery() string {
	return `status IN ? AND (
		(COALESCE(cleanup_state, '') = '' AND updated_at < ?)
		OR (cleanup_state IN ? AND (cleanup_next_attempt_at IS NULL OR cleanup_next_attempt_at <= ?))
		OR (cleanup_state = ? AND (cleanup_lease_expires_at IS NULL OR cleanup_lease_expires_at <= ?))
	)`
}

// ListArtifactsEligibleForCleanup returns only durable cleanup work that may be
// claimed now. Initial terminal rows use the retention cutoff; retry and expired
// cleanup leases use their own persisted schedule and are not delayed again by
// later metadata updates.
func (r *TranscodeExecutionRepo) ListArtifactsEligibleForCleanup(cutoff, now time.Time, limit int) ([]model.TranscodeArtifactRecord, error) {
	if limit <= 0 {
		limit = 500
	}
	var artifacts []model.TranscodeArtifactRecord
	err := r.db.Where(
		artifactCleanupEligibilityQuery(),
		artifactCleanupTerminalStatuses,
		cutoff,
		[]string{ArtifactCleanupPending, ArtifactCleanupRetryWait},
		now,
		ArtifactCleanupClaimed,
		now,
	).
		Order("COALESCE(cleanup_next_attempt_at, updated_at) ASC, id ASC").
		Limit(limit).
		Find(&artifacts).Error
	return artifacts, err
}

// ListArtifactCleanupOperations exposes the bounded durable cleanup work set to
// the admin task center. Blocked work is ordered first so an invariant failure
// cannot disappear behind a large history of normal transcode tasks.
func (r *TranscodeExecutionRepo) ListArtifactCleanupOperations(limit int) ([]model.TranscodeArtifactRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var artifacts []model.TranscodeArtifactRecord
	err := r.db.Where("cleanup_state IN ?", artifactCleanupManagedStates).
		Order(`CASE cleanup_state
			WHEN 'blocked' THEN 0
			WHEN 'retry_wait' THEN 1
			WHEN 'claimed' THEN 2
			ELSE 3
		END ASC`).
		Order("COALESCE(cleanup_next_attempt_at, cleanup_last_attempt_at, updated_at) DESC, id ASC").
		Limit(limit).
		Find(&artifacts).Error
	return artifacts, err
}

func (r *TranscodeExecutionRepo) FindArtifactCleanupOperation(artifactID string) (*model.TranscodeArtifactRecord, error) {
	var artifact model.TranscodeArtifactRecord
	result := r.db.Where("id = ? AND cleanup_state IN ?", artifactID, artifactCleanupManagedStates).
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

// RequeueArtifactCleanup is the only operator-facing unblock operation. It does
// not bypass path validation or delete metadata directly: it merely clears the
// persisted block/retry delay and makes the Artifact eligible for the normal
// Cleanup Lease path. If the invariant is still broken, the next attempt blocks
// again with fresh evidence.
func (r *TranscodeExecutionRepo) RequeueArtifactCleanup(artifactID string, now time.Time) (*model.TranscodeArtifactRecord, bool, error) {
	result := r.db.Model(&model.TranscodeArtifactRecord{}).
		Where(
			"id = ? AND status IN ? AND cleanup_state IN ?",
			artifactID,
			artifactCleanupTerminalStatuses,
			[]string{ArtifactCleanupBlocked, ArtifactCleanupRetryWait},
		).
		Updates(map[string]any{
			"cleanup_state":            ArtifactCleanupPending,
			"cleanup_token":            "",
			"cleanup_claimed_at":       nil,
			"cleanup_lease_expires_at": nil,
			"cleanup_next_attempt_at":  now,
			"cleanup_error_code":       "",
			"cleanup_error_message":    "",
			"updated_at":               now,
		})
	if result.Error != nil || result.RowsAffected != 1 {
		return nil, false, result.Error
	}
	artifact, err := r.FindArtifactCleanupOperation(artifactID)
	if err != nil {
		return nil, false, err
	}
	return artifact, true, nil
}

// QueueArtifactCleanup makes a task-owned Artifact immediately eligible without
// overwriting a retry schedule or a live claim. A currently published Artifact
// becomes expired before filesystem deletion so no new resolver can select it.
func (r *TranscodeExecutionRepo) QueueArtifactCleanup(artifactID string, now time.Time) error {
	return r.db.Model(&model.TranscodeArtifactRecord{}).
		Where(
			"id = ? AND status IN ? AND COALESCE(cleanup_state, '') IN ?",
			artifactID,
			append(append([]string{}, artifactCleanupTerminalStatuses...), "published"),
			[]string{"", ArtifactCleanupPending},
		).
		Updates(map[string]any{
			"status":                  gorm.Expr("CASE WHEN status = ? THEN ? ELSE status END", "published", "expired"),
			"cleanup_state":           ArtifactCleanupPending,
			"cleanup_next_attempt_at": now,
			"updated_at":              now,
		}).Error
}

// ClaimArtifactCleanup is the cleanup equivalent of a Job Lease. Only one
// process may remove files and metadata for an Artifact at a time. An expired
// claim is recoverable by another server instance.
func (r *TranscodeExecutionRepo) ClaimArtifactCleanup(
	artifactID,
	token string,
	cutoff,
	now time.Time,
	leaseDuration time.Duration,
) (*model.TranscodeArtifactRecord, bool, error) {
	if leaseDuration <= 0 {
		leaseDuration = 2 * time.Minute
	}
	leaseExpiresAt := now.Add(leaseDuration)
	result := r.db.Model(&model.TranscodeArtifactRecord{}).
		Where("id = ? AND "+artifactCleanupEligibilityQuery(),
			artifactID,
			artifactCleanupTerminalStatuses,
			cutoff,
			[]string{ArtifactCleanupPending, ArtifactCleanupRetryWait},
			now,
			ArtifactCleanupClaimed,
			now,
		).
		Updates(map[string]any{
			"cleanup_state":            ArtifactCleanupClaimed,
			"cleanup_token":            token,
			"cleanup_claimed_at":       now,
			"cleanup_lease_expires_at": leaseExpiresAt,
			"cleanup_last_attempt_at":  now,
			"cleanup_attempts":         gorm.Expr("cleanup_attempts + 1"),
			"cleanup_error_code":       "",
			"cleanup_error_message":    "",
			"updated_at":               now,
		})
	if result.Error != nil || result.RowsAffected != 1 {
		return nil, false, result.Error
	}
	var artifact model.TranscodeArtifactRecord
	if err := r.db.First(&artifact, "id = ?", artifactID).Error; err != nil {
		return nil, false, err
	}
	return &artifact, true, nil
}

func (r *TranscodeExecutionRepo) ScheduleArtifactCleanupRetry(
	artifactID,
	token,
	errorCode,
	errorMessage string,
	nextAttemptAt,
	now time.Time,
) (bool, error) {
	result := r.db.Model(&model.TranscodeArtifactRecord{}).
		Where("id = ? AND cleanup_state = ? AND cleanup_token = ?", artifactID, ArtifactCleanupClaimed, token).
		Updates(map[string]any{
			"cleanup_state":            ArtifactCleanupRetryWait,
			"cleanup_token":            "",
			"cleanup_claimed_at":       nil,
			"cleanup_lease_expires_at": nil,
			"cleanup_next_attempt_at":  nextAttemptAt,
			"cleanup_error_code":       errorCode,
			"cleanup_error_message":    errorMessage,
			"updated_at":               now,
		})
	return result.RowsAffected == 1, result.Error
}

func (r *TranscodeExecutionRepo) BlockArtifactCleanup(
	artifactID,
	token,
	errorCode,
	errorMessage string,
	now time.Time,
) (bool, error) {
	result := r.db.Model(&model.TranscodeArtifactRecord{}).
		Where("id = ? AND cleanup_state = ? AND cleanup_token = ?", artifactID, ArtifactCleanupClaimed, token).
		Updates(map[string]any{
			"cleanup_state":            ArtifactCleanupBlocked,
			"cleanup_token":            "",
			"cleanup_claimed_at":       nil,
			"cleanup_lease_expires_at": nil,
			"cleanup_next_attempt_at":  nil,
			"cleanup_error_code":       errorCode,
			"cleanup_error_message":    errorMessage,
			"updated_at":               now,
		})
	return result.RowsAffected == 1, result.Error
}

// CompleteArtifactCleanupByClaim preserves a durable tombstone after the
// filesystem has been reclaimed. Runtime History therefore keeps the original
// path, byte count, completion time and disposition without treating the file
// as live storage.
func (r *TranscodeExecutionRepo) CompleteArtifactCleanupByClaim(
	artifactID,
	token,
	disposition string,
	now time.Time,
) (bool, error) {
	completed := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var artifact model.TranscodeArtifactRecord
		result := tx.Where(
			"id = ? AND cleanup_state = ? AND cleanup_token = ?",
			artifactID,
			ArtifactCleanupClaimed,
			token,
		).Limit(1).Find(&artifact)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		if err := tx.Where(
			"startup_artifact_id = ? OR continuation_artifact_id = ?",
			artifactID,
			artifactID,
		).Delete(&model.TranscodeHandoffAttestationRecord{}).Error; err != nil {
			return err
		}
		result = tx.Model(&model.TranscodeArtifactRecord{}).
			Where(
				"id = ? AND cleanup_state = ? AND cleanup_token = ?",
				artifactID,
				ArtifactCleanupClaimed,
				token,
			).
			Updates(map[string]any{
				"status":                         "deleted",
				"cleanup_state":                  ArtifactCleanupCompleted,
				"cleanup_completed_at":           now,
				"cleanup_disposition":            disposition,
				"cleanup_original_path":          artifact.Path,
				"cleanup_original_temp_path":     artifact.TempPath,
				"cleanup_original_manifest_path": artifact.ManifestPath,
				"path":                           "",
				"temp_path":                      "",
				"manifest_path":                  "",
				"cleanup_token":                  "",
				"cleanup_claimed_at":             nil,
				"cleanup_lease_expires_at":       nil,
				"cleanup_next_attempt_at":        nil,
				"cleanup_error_code":             "",
				"cleanup_error_message":          "",
				"updated_at":                     now,
			})
		if result.Error != nil {
			return result.Error
		}
		completed = result.RowsAffected == 1
		return nil
	})
	return completed, err
}

// RollbackLegacyArtifactCleanup removes a migrated legacy directory from the
// cleanup work set without changing or deleting the directory itself. Claimed
// or completed cleanup cannot be rolled back.
func (r *TranscodeExecutionRepo) RollbackLegacyArtifactCleanup(artifactID string, now time.Time) (bool, error) {
	result := r.db.Model(&model.TranscodeArtifactRecord{}).
		Where(
			"id = ? AND migration_source = ? AND cleanup_state IN ?",
			artifactID,
			LegacyTranscodeArtifactMigrationSource,
			[]string{ArtifactCleanupPending, ArtifactCleanupRetryWait, ArtifactCleanupBlocked},
		).
		Updates(map[string]any{
			"status":                   "migration_rolled_back",
			"cleanup_state":            ArtifactCleanupRollbackCompleted,
			"cleanup_completed_at":     now,
			"cleanup_disposition":      "rollback_preserved",
			"cleanup_token":            "",
			"cleanup_claimed_at":       nil,
			"cleanup_lease_expires_at": nil,
			"cleanup_next_attempt_at":  nil,
			"cleanup_error_code":       "",
			"cleanup_error_message":    "",
			"cleanup_rollback_until":   nil,
			"updated_at":               now,
		})
	return result.RowsAffected == 1, result.Error
}

func (r *TranscodeExecutionRepo) ArtifactCleanupStateCounts() (map[string]int64, error) {
	type row struct {
		State string
		Count int64
	}
	var rows []row
	if err := r.db.Model(&model.TranscodeArtifactRecord{}).
		Select("COALESCE(cleanup_state, '') AS state, COUNT(*) AS count").
		Where("COALESCE(cleanup_state, '') <> ''").
		Group("COALESCE(cleanup_state, '')").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, item := range rows {
		counts[item.State] = item.Count
	}
	return counts, nil
}
