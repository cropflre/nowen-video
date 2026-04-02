package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== TagRepo 标签仓储 ====================

type TagRepo struct {
	db *gorm.DB
}

func (r *TagRepo) Create(tag *model.Tag) error {
	return r.db.Create(tag).Error
}

func (r *TagRepo) FindByID(id string) (*model.Tag, error) {
	var tag model.Tag
	err := r.db.First(&tag, "id = ?", id).Error
	return &tag, err
}

func (r *TagRepo) FindByName(name string) (*model.Tag, error) {
	var tag model.Tag
	err := r.db.Where("name = ?", name).First(&tag).Error
	return &tag, err
}

func (r *TagRepo) List(category string) ([]model.Tag, error) {
	var tags []model.Tag
	query := r.db.Model(&model.Tag{})
	if category != "" {
		query = query.Where("category = ?", category)
	}
	err := query.Order("sort_order DESC, usage_count DESC, name ASC").Find(&tags).Error
	return tags, err
}

func (r *TagRepo) Update(tag *model.Tag) error {
	return r.db.Save(tag).Error
}

func (r *TagRepo) Delete(id string) error {
	// 同时删除关联
	r.db.Where("tag_id = ?", id).Delete(&model.MediaTag{})
	return r.db.Delete(&model.Tag{}, "id = ?", id).Error
}

func (r *TagRepo) IncrementUsage(tagID string) error {
	return r.db.Model(&model.Tag{}).Where("id = ?", tagID).
		UpdateColumn("usage_count", gorm.Expr("usage_count + 1")).Error
}

func (r *TagRepo) DecrementUsage(tagID string) error {
	return r.db.Model(&model.Tag{}).Where("id = ?", tagID).
		UpdateColumn("usage_count", gorm.Expr("CASE WHEN usage_count > 0 THEN usage_count - 1 ELSE 0 END")).Error
}

func (r *TagRepo) ListCategories() ([]string, error) {
	var categories []string
	err := r.db.Model(&model.Tag{}).Distinct("category").
		Where("category != '' AND category IS NOT NULL").
		Pluck("category", &categories).Error
	return categories, err
}

// ==================== MediaTagRepo 媒体标签关联仓储 ====================

type MediaTagRepo struct {
	db *gorm.DB
}

func (r *MediaTagRepo) Create(mt *model.MediaTag) error {
	return r.db.Create(mt).Error
}

func (r *MediaTagRepo) Delete(mediaID, tagID string) error {
	return r.db.Where("media_id = ? AND tag_id = ?", mediaID, tagID).Delete(&model.MediaTag{}).Error
}

func (r *MediaTagRepo) ListByMediaID(mediaID string) ([]model.MediaTag, error) {
	var tags []model.MediaTag
	err := r.db.Preload("Tag").Where("media_id = ?", mediaID).Find(&tags).Error
	return tags, err
}

func (r *MediaTagRepo) ListByTagID(tagID string, page, size int) ([]model.MediaTag, int64, error) {
	var tags []model.MediaTag
	var total int64
	query := r.db.Model(&model.MediaTag{}).Where("tag_id = ?", tagID)
	query.Count(&total)
	err := query.Preload("Tag").Order("created_at DESC").
		Offset((page - 1) * size).Limit(size).Find(&tags).Error
	return tags, total, err
}

func (r *MediaTagRepo) DeleteByMediaID(mediaID string) error {
	return r.db.Where("media_id = ?", mediaID).Delete(&model.MediaTag{}).Error
}

func (r *MediaTagRepo) Exists(mediaID, tagID string) bool {
	var count int64
	r.db.Model(&model.MediaTag{}).Where("media_id = ? AND tag_id = ?", mediaID, tagID).Count(&count)
	return count > 0
}

func (r *MediaTagRepo) BatchCreate(tags []model.MediaTag) error {
	if len(tags) == 0 {
		return nil
	}
	return r.db.Create(&tags).Error
}

// ==================== ShareLinkRepo 分享链接仓储 ====================

type ShareLinkRepo struct {
	db *gorm.DB
}

func (r *ShareLinkRepo) Create(link *model.ShareLink) error {
	return r.db.Create(link).Error
}

func (r *ShareLinkRepo) FindByID(id string) (*model.ShareLink, error) {
	var link model.ShareLink
	err := r.db.First(&link, "id = ?", id).Error
	return &link, err
}

func (r *ShareLinkRepo) FindByCode(code string) (*model.ShareLink, error) {
	var link model.ShareLink
	err := r.db.Where("code = ?", code).First(&link).Error
	return &link, err
}

func (r *ShareLinkRepo) ListByUser(userID string, page, size int) ([]model.ShareLink, int64, error) {
	var links []model.ShareLink
	var total int64
	query := r.db.Model(&model.ShareLink{}).Where("created_by = ?", userID)
	query.Count(&total)
	err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&links).Error
	return links, total, err
}

func (r *ShareLinkRepo) Update(link *model.ShareLink) error {
	return r.db.Save(link).Error
}

func (r *ShareLinkRepo) Delete(id string) error {
	return r.db.Delete(&model.ShareLink{}, "id = ?", id).Error
}

func (r *ShareLinkRepo) IncrementViewCount(id string) error {
	return r.db.Model(&model.ShareLink{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

// ==================== MatchRuleRepo 自定义匹配规则仓储 ====================

type MatchRuleRepo struct {
	db *gorm.DB
}

func (r *MatchRuleRepo) Create(rule *model.MatchRule) error {
	return r.db.Create(rule).Error
}

func (r *MatchRuleRepo) FindByID(id string) (*model.MatchRule, error) {
	var rule model.MatchRule
	err := r.db.First(&rule, "id = ?", id).Error
	return &rule, err
}

func (r *MatchRuleRepo) List(libraryID string) ([]model.MatchRule, error) {
	var rules []model.MatchRule
	query := r.db.Model(&model.MatchRule{})
	if libraryID != "" {
		query = query.Where("library_id = ? OR library_id = '' OR library_id IS NULL", libraryID)
	}
	err := query.Order("priority DESC, created_at ASC").Find(&rules).Error
	return rules, err
}

func (r *MatchRuleRepo) ListEnabled(libraryID string) ([]model.MatchRule, error) {
	var rules []model.MatchRule
	query := r.db.Model(&model.MatchRule{}).Where("enabled = ?", true)
	if libraryID != "" {
		query = query.Where("library_id = ? OR library_id = '' OR library_id IS NULL", libraryID)
	}
	err := query.Order("priority DESC, created_at ASC").Find(&rules).Error
	return rules, err
}

func (r *MatchRuleRepo) Update(rule *model.MatchRule) error {
	return r.db.Save(rule).Error
}

func (r *MatchRuleRepo) Delete(id string) error {
	return r.db.Delete(&model.MatchRule{}, "id = ?", id).Error
}

func (r *MatchRuleRepo) IncrementHitCount(id string) error {
	return r.db.Model(&model.MatchRule{}).Where("id = ?", id).
		UpdateColumn("hit_count", gorm.Expr("hit_count + 1")).Error
}
