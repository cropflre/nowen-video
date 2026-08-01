package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// MediaProbeWarmupRepo is a narrow repository used by the scan-completion
// background service. It avoids constructing the entire Repositories aggregate
// from inside the transcode subsystem.
type MediaProbeWarmupRepo struct {
	db *gorm.DB
}

func NewMediaProbeWarmupRepo(db *gorm.DB) *MediaProbeWarmupRepo {
	return &MediaProbeWarmupRepo{db: db}
}

func listProbeCandidatesByLibrary(db *gorm.DB, libraryID, afterID string, limit int) ([]model.Media, error) {
	if limit <= 0 || limit > 500 {
		limit = 64
	}
	query := db.Where("library_id = ?", libraryID)
	if afterID != "" {
		query = query.Where("id > ?", afterID)
	}
	var rows []model.Media
	err := query.Order("id ASC").Limit(limit).Find(&rows).Error
	return rows, err
}

func updateProbeTechnicalSummary(db *gorm.DB, mediaID, videoCodec, audioCodec, resolution string, duration float64, fileSize int64) error {
	updates := map[string]any{
		"video_codec": videoCodec,
		"audio_codec": audioCodec,
		"resolution":  resolution,
		"duration":    duration,
	}
	if fileSize > 0 {
		updates["file_size"] = fileSize
	}
	return db.Model(&model.Media{}).Where("id = ?", mediaID).Updates(updates).Error
}

// ListProbeCandidatesByLibrary returns a stable ID-ordered page so a scan
// completion can warm technical metadata without loading a large NAS library
// into memory. Soft-deleted rows are excluded by GORM's model scope.
func (r *MediaProbeWarmupRepo) ListProbeCandidatesByLibrary(libraryID, afterID string, limit int) ([]model.Media, error) {
	return listProbeCandidatesByLibrary(r.db, libraryID, afterID, limit)
}

// UpdateTechnicalSummary keeps legacy API fields synchronized with the
// authoritative media_probe_cache record. It intentionally updates only
// technical columns so metadata scraping and user edits cannot be overwritten
// by a background Probe.
func (r *MediaProbeWarmupRepo) UpdateTechnicalSummary(mediaID, videoCodec, audioCodec, resolution string, duration float64, fileSize int64) error {
	return updateProbeTechnicalSummary(r.db, mediaID, videoCodec, audioCodec, resolution, duration, fileSize)
}

// MediaRepo adapters keep the same operations available to other services and
// tests without duplicating SQL.
func (r *MediaRepo) ListProbeCandidatesByLibrary(libraryID, afterID string, limit int) ([]model.Media, error) {
	return listProbeCandidatesByLibrary(r.db, libraryID, afterID, limit)
}

func (r *MediaRepo) UpdateTechnicalSummary(mediaID, videoCodec, audioCodec, resolution string, duration float64, fileSize int64) error {
	return updateProbeTechnicalSummary(r.db, mediaID, videoCodec, audioCodec, resolution, duration, fileSize)
}
