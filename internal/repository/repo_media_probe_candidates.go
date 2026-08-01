package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
)

// ListProbeCandidatesByLibrary returns a stable ID-ordered page so a scan
// completion can warm technical metadata without loading a large NAS library
// into memory. Soft-deleted rows are excluded by GORM's model scope.
func (r *MediaRepo) ListProbeCandidatesByLibrary(libraryID, afterID string, limit int) ([]model.Media, error) {
	if limit <= 0 || limit > 500 {
		limit = 64
	}
	query := r.db.Where("library_id = ?", libraryID)
	if afterID != "" {
		query = query.Where("id > ?", afterID)
	}
	var rows []model.Media
	err := query.Order("id ASC").Limit(limit).Find(&rows).Error
	return rows, err
}

// UpdateTechnicalSummary keeps legacy API fields synchronized with the
// authoritative media_probe_cache record. It intentionally updates only
// technical columns so metadata scraping and user edits cannot be overwritten
// by a background Probe.
func (r *MediaRepo) UpdateTechnicalSummary(mediaID, videoCodec, audioCodec, resolution string, duration float64, fileSize int64) error {
	updates := map[string]any{
		"video_codec": videoCodec,
		"audio_codec": audioCodec,
		"resolution":  resolution,
		"duration":    duration,
	}
	if fileSize > 0 {
		updates["file_size"] = fileSize
	}
	return r.db.Model(&model.Media{}).Where("id = ?", mediaID).Updates(updates).Error
}
