package repository

import (
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// FindReadableHLSArtifactVersion resolves one explicit Artifact version.
//
// Active staging/publishing versions remain readable only while their owning
// Job still points at the same Attempt and holds a live Lease. Immutable
// published versions and retained superseded versions remain readable without
// a Job Lease so clients that already received an older playlist can finish
// that exact version during the retention window.
func (r *TranscodeExecutionRepo) FindReadableHLSArtifactVersion(
	mediaID,
	profileID,
	sourceFingerprint,
	plannerVersion,
	artifactID string,
	now time.Time,
) (*model.TranscodeArtifactRecord, error) {
	if strings.TrimSpace(artifactID) == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var active model.TranscodeArtifactRecord
	activeResult := r.db.Table("transcode_artifacts AS a").
		Select("a.*").
		Joins("JOIN transcode_jobs AS j ON j.id = a.job_id").
		Where(
			`a.id = ? AND a.media_id = ? AND a.profile_id = ?
			AND a.source_fingerprint = ? AND a.planner_version = ?
			AND a.kind = ? AND a.status IN ?
			AND a.attempt_id = j.current_attempt_id
			AND j.active_key IS NOT NULL AND j.desired_state = ?
			AND j.status IN ? AND j.lease_token <> ''
			AND j.lease_expires_at IS NOT NULL AND j.lease_expires_at > ?`,
			artifactID,
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
		Limit(1).
		Find(&active)
	if activeResult.Error != nil {
		return nil, activeResult.Error
	}
	if activeResult.RowsAffected == 1 {
		return &active, nil
	}

	var retained model.TranscodeArtifactRecord
	retainedResult := r.db.Where(
		`id = ? AND media_id = ? AND profile_id = ?
		AND source_fingerprint = ? AND planner_version = ?
		AND kind = ? AND status IN ?`,
		artifactID,
		mediaID,
		profileID,
		sourceFingerprint,
		plannerVersion,
		"hls_variant",
		[]string{"published", "superseded"},
	).
		Limit(1).
		Find(&retained)
	if retainedResult.Error != nil {
		return nil, retainedResult.Error
	}
	if retainedResult.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &retained, nil
}

// FindReadableHLSArtifactByPath binds a resolver result back to the exact
// Artifact row that supplied it. This closes the race where a new version may
// become current between path resolution and playlist rewriting: the old path
// is still identified as a retained superseded version rather than silently
// switching the playlist to the new Artifact ID.
func (r *TranscodeExecutionRepo) FindReadableHLSArtifactByPath(
	mediaID,
	profileID,
	sourceFingerprint,
	plannerVersion,
	resolvedPath string,
	now time.Time,
) (*model.TranscodeArtifactRecord, error) {
	if strings.TrimSpace(resolvedPath) == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var active model.TranscodeArtifactRecord
	activeResult := r.db.Table("transcode_artifacts AS a").
		Select("a.*").
		Joins("JOIN transcode_jobs AS j ON j.id = a.job_id").
		Where(
			`a.media_id = ? AND a.profile_id = ?
			AND a.source_fingerprint = ? AND a.planner_version = ?
			AND a.kind = ? AND a.status IN ?
			AND (a.temp_path = ? OR a.path = ?)
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
			resolvedPath,
			resolvedPath,
			"running",
			[]string{"claimed", "running"},
			now,
		).
		Order("a.created_at DESC").
		Limit(1).
		Find(&active)
	if activeResult.Error != nil {
		return nil, activeResult.Error
	}
	if activeResult.RowsAffected == 1 {
		return &active, nil
	}

	var retained model.TranscodeArtifactRecord
	retainedResult := r.db.Where(
		`media_id = ? AND profile_id = ?
		AND source_fingerprint = ? AND planner_version = ?
		AND kind = ? AND status IN ? AND path = ?`,
		mediaID,
		profileID,
		sourceFingerprint,
		plannerVersion,
		"hls_variant",
		[]string{"published", "superseded"},
		resolvedPath,
	).
		Order("published_at DESC, created_at DESC").
		Limit(1).
		Find(&retained)
	if retainedResult.Error != nil {
		return nil, retainedResult.Error
	}
	if retainedResult.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &retained, nil
}
