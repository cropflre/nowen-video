package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DanmakuRepo manages local bullet comments.
type DanmakuRepo struct {
	db *gorm.DB
}

func NewDanmakuRepo(db *gorm.DB) *DanmakuRepo {
	return &DanmakuRepo{db: db}
}

func (r *DanmakuRepo) UpsertMany(items []model.DanmakuComment) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "source"},
			{Name: "source_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"media_id",
			"author",
			"content",
			"rating",
			"likes",
			"position",
			"color",
			"mode",
			"imported_at",
			"updated_at",
		}),
	}).Create(&items).Error
}

func (r *DanmakuRepo) ListByMedia(mediaID string, limit int) ([]model.DanmakuComment, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var items []model.DanmakuComment
	err := r.db.Where("media_id = ?", mediaID).
		Order("position ASC, created_at ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (r *DanmakuRepo) CountByMedia(mediaID string) (int64, error) {
	var total int64
	err := r.db.Model(&model.DanmakuComment{}).Where("media_id = ?", mediaID).Count(&total).Error
	return total, err
}
