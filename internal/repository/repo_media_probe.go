package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MediaProbeRepo struct {
	db *gorm.DB
}

func NewMediaProbeRepo(db *gorm.DB) *MediaProbeRepo {
	return &MediaProbeRepo{db: db}
}

func (r *MediaProbeRepo) AutoMigrate() error {
	return r.db.AutoMigrate(&model.MediaProbeRecord{})
}

func (r *MediaProbeRepo) FindFresh(mediaID, fingerprint, probeVersion string) (*model.MediaProbeRecord, error) {
	var record model.MediaProbeRecord
	err := r.db.Where(
		"media_id = ? AND source_fingerprint = ? AND probe_version = ?",
		mediaID,
		fingerprint,
		probeVersion,
	).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *MediaProbeRepo) FindByMediaID(mediaID string) (*model.MediaProbeRecord, error) {
	var record model.MediaProbeRecord
	if err := r.db.First(&record, "media_id = ?", mediaID).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *MediaProbeRepo) Upsert(record *model.MediaProbeRecord) error {
	if record.ProbedAt.IsZero() {
		record.ProbedAt = time.Now()
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "media_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"source_fingerprint",
			"source_path",
			"source_size",
			"source_mod_time_ns",
			"probe_version",
			"format_name",
			"duration_ms",
			"width",
			"height",
			"frame_rate_num",
			"frame_rate_den",
			"video_codec",
			"pixel_format",
			"bit_depth",
			"color_transfer",
			"color_primaries",
			"color_space",
			"color_range",
			"hdr",
			"audio_streams_json",
			"probed_at",
			"updated_at",
		}),
	}).Create(record).Error
}

func (r *MediaProbeRepo) DeleteByMediaID(mediaID string) error {
	return r.db.Delete(&model.MediaProbeRecord{}, "media_id = ?", mediaID).Error
}

func (r *MediaProbeRepo) DeleteByMediaIDs(mediaIDs []string) (int64, error) {
	if len(mediaIDs) == 0 {
		return 0, nil
	}
	result := r.db.Where("media_id IN ?", mediaIDs).Delete(&model.MediaProbeRecord{})
	return result.RowsAffected, result.Error
}

// CleanOrphaned 删除媒体主体已不存在的探测缓存。
func (r *MediaProbeRepo) CleanOrphaned() (int64, error) {
	result := r.db.Where("media_id NOT IN (SELECT id FROM media WHERE deleted_at IS NULL)").
		Delete(&model.MediaProbeRecord{})
	return result.RowsAffected, result.Error
}
