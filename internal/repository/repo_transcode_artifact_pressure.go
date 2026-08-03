package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

type ArtifactPressureQueueResult struct {
	Queued int   `json:"queued"`
	Bytes  int64 `json:"bytes"`
}

// TouchArtifactAccess persists a bounded last-use signal in updated_at. The
// service throttles calls, so playback does not create one database write per
// segment. Cleanup-owned rows are never revived by a late reader.
func (r *TranscodeExecutionRepo) TouchArtifactAccess(artifactID string, accessedAt, writeBefore time.Time) (bool, error) {
	result := r.db.Model(&model.TranscodeArtifactRecord{}).
		Where(
			"id = ? AND status IN ? AND COALESCE(cleanup_state, '') = '' AND updated_at <= ?",
			artifactID,
			[]string{"published", "superseded"},
			writeBefore,
		).
		Update("updated_at", accessedAt)
	return result.RowsAffected == 1, result.Error
}

// QueueTerminalArtifactsForPressure shortens retention only for already
// terminal evidence. Recent explicit-version playback remains protected by the
// access grace window, and existing retry/blocked ownership is left untouched.
func (r *TranscodeExecutionRepo) QueueTerminalArtifactsForPressure(
	protectedAfter,
	now time.Time,
	limit int,
) (ArtifactPressureQueueResult, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	var candidates []model.TranscodeArtifactRecord
	if err := r.db.Where(
		"status IN ? AND COALESCE(cleanup_state, '') = '' AND updated_at <= ?",
		artifactCleanupTerminalStatuses,
		protectedAfter,
	).
		Order(`CASE status
			WHEN 'expired' THEN 0
			WHEN 'superseded' THEN 1
			WHEN 'failed' THEN 2
			WHEN 'cancelled' THEN 3
			ELSE 4
		END ASC`).
		Order("updated_at ASC, id ASC").
		Limit(limit).
		Find(&candidates).Error; err != nil {
		return ArtifactPressureQueueResult{}, err
	}

	queued := ArtifactPressureQueueResult{}
	for i := range candidates {
		candidate := &candidates[i]
		result := r.db.Model(&model.TranscodeArtifactRecord{}).
			Where(
				"id = ? AND status IN ? AND COALESCE(cleanup_state, '') = '' AND updated_at <= ?",
				candidate.ID,
				artifactCleanupTerminalStatuses,
				protectedAfter,
			).
			Updates(map[string]any{
				"cleanup_state":           ArtifactCleanupPending,
				"cleanup_next_attempt_at": now,
				"updated_at":              now,
			})
		if result.Error != nil {
			return queued, result.Error
		}
		if result.RowsAffected != 1 {
			continue
		}
		queued.Queued++
		if candidate.SizeBytes > 0 {
			queued.Bytes += candidate.SizeBytes
		}
	}
	return queued, nil
}

// ExpirePublishedArtifactsForPressure is the explicit published-cache tier.
// It never touches staging/publishing output, recently accessed artifacts, or
// cleanup-owned rows. Runtime HLS is reclaimed before startup artifacts because
// startup media provides the highest playback-start benefit per stored byte.
func (r *TranscodeExecutionRepo) ExpirePublishedArtifactsForPressure(
	protectedAfter,
	publishedBefore time.Time,
	targetBytes int64,
	now time.Time,
	limit int,
) (ArtifactPressureQueueResult, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	var candidates []model.TranscodeArtifactRecord
	if err := r.db.Where(
		`status = ? AND COALESCE(cleanup_state, '') = ''
		AND updated_at <= ?
		AND published_at IS NOT NULL AND published_at <= ?`,
		"published",
		protectedAfter,
		publishedBefore,
	).
		Order(`CASE kind
			WHEN 'hls_variant' THEN 0
			WHEN 'startup_continuation_hls' THEN 1
			WHEN 'startup_hls' THEN 2
			ELSE 3
		END ASC`).
		Order("published_at ASC, id ASC").
		Limit(limit).
		Find(&candidates).Error; err != nil {
		return ArtifactPressureQueueResult{}, err
	}

	queued := ArtifactPressureQueueResult{}
	for i := range candidates {
		if targetBytes > 0 && queued.Bytes >= targetBytes {
			break
		}
		candidate := &candidates[i]
		result := r.db.Model(&model.TranscodeArtifactRecord{}).
			Where(
				`id = ? AND status = ? AND COALESCE(cleanup_state, '') = ''
				AND updated_at <= ?
				AND published_at IS NOT NULL AND published_at <= ?`,
				candidate.ID,
				"published",
				protectedAfter,
				publishedBefore,
			).
			Updates(map[string]any{
				"status":                  "expired",
				"expires_at":              now,
				"cleanup_state":           ArtifactCleanupPending,
				"cleanup_next_attempt_at": now,
				"updated_at":              now,
			})
		if result.Error != nil {
			return queued, result.Error
		}
		if result.RowsAffected != 1 {
			continue
		}
		queued.Queued++
		if candidate.SizeBytes > 0 {
			queued.Bytes += candidate.SizeBytes
		}
	}
	return queued, nil
}
