package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== MovieCollectionRepo ====================

// MovieCollectionRepo 电影系列合集仓储
type MovieCollectionRepo struct {
	db *gorm.DB
}

// Create 创建合集
func (r *MovieCollectionRepo) Create(coll *model.MovieCollection) error {
	return r.db.Create(coll).Error
}

// Update 更新合集
func (r *MovieCollectionRepo) Update(coll *model.MovieCollection) error {
	return r.db.Save(coll).Error
}

// FindByID 根据 ID 查找合集
func (r *MovieCollectionRepo) FindByID(id string) (*model.MovieCollection, error) {
	var coll model.MovieCollection
	err := r.db.First(&coll, "id = ?", id).Error
	return &coll, err
}

// FindByIDWithMedia 根据 ID 查找合集并预加载关联的电影（按年份排序）
func (r *MovieCollectionRepo) FindByIDWithMedia(id string) (*model.MovieCollection, error) {
	var coll model.MovieCollection
	err := r.db.Preload("Media", func(db *gorm.DB) *gorm.DB {
		return db.Order("year ASC, title ASC")
	}).First(&coll, "id = ?", id).Error
	return &coll, err
}

// FindByName 根据名称精确查找合集（返回第一个匹配的）
func (r *MovieCollectionRepo) FindByName(name string) (*model.MovieCollection, error) {
	var coll model.MovieCollection
	err := r.db.Where("name = ?", name).First(&coll).Error
	return &coll, err
}

// FindAllByName 根据名称精确查找所有同名合集
func (r *MovieCollectionRepo) FindAllByName(name string) ([]model.MovieCollection, error) {
	var colls []model.MovieCollection
	err := r.db.Where("name = ?", name).Order("created_at ASC").Find(&colls).Error
	return colls, err
}

// MergeCollections 将多个合集合并到目标合集（将源合集的电影迁移到目标合集，然后删除源合集）
func (r *MovieCollectionRepo) MergeCollections(targetID string, sourceIDs []string) error {
	if len(sourceIDs) == 0 {
		return nil
	}
	// 将源合集下的所有电影迁移到目标合集
	if err := r.db.Model(&model.Media{}).Where("collection_id IN ?", sourceIDs).
		Update("collection_id", targetID).Error; err != nil {
		return err
	}
	// 删除源合集记录
	if err := r.db.Where("id IN ?", sourceIDs).Delete(&model.MovieCollection{}).Error; err != nil {
		return err
	}
	// 更新目标合集的电影数量
	return r.UpdateMediaCount(targetID)
}

// CleanupEmptyCollections 清理所有 media_count 为 0 的空壳合集
// 返回被清理的合集数量
func (r *MovieCollectionRepo) CleanupEmptyCollections() (int64, error) {
	// 先重新统计所有合集的实际电影数量（防止 media_count 字段不准确）
	var allColls []model.MovieCollection
	if err := r.db.Find(&allColls).Error; err != nil {
		return 0, err
	}
	for _, c := range allColls {
		r.UpdateMediaCount(c.ID)
	}

	// 删除实际无关联电影的空壳合集
	result := r.db.Where("media_count = 0").Delete(&model.MovieCollection{})
	return result.RowsAffected, result.Error
}

// FindDuplicateNames 查找所有存在重复的合集名称
func (r *MovieCollectionRepo) FindDuplicateNames() ([]string, error) {
	var names []string
	err := r.db.Model(&model.MovieCollection{}).
		Select("name").
		Group("name").
		Having("COUNT(*) > 1").
		Pluck("name", &names).Error
	return names, err
}

// FindByTMDbCollID 根据 TMDb Collection ID 查找
func (r *MovieCollectionRepo) FindByTMDbCollID(tmdbCollID int) (*model.MovieCollection, error) {
	var coll model.MovieCollection
	err := r.db.Where("tmdb_coll_id = ?", tmdbCollID).First(&coll).Error
	return &coll, err
}

// List 分页获取合集列表（排除空壳合集）
func (r *MovieCollectionRepo) List(page, size int) ([]model.MovieCollection, int64, error) {
	var colls []model.MovieCollection
	var total int64

	r.db.Model(&model.MovieCollection{}).Where("media_count > 0").Count(&total)
	err := r.db.Where("media_count > 0").
		Order("updated_at DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&colls).Error
	return colls, total, err
}

// ListAll 获取所有合集（用于后台管理）
func (r *MovieCollectionRepo) ListAll() ([]model.MovieCollection, error) {
	var colls []model.MovieCollection
	err := r.db.Order("name ASC").Find(&colls).Error
	return colls, err
}

// Delete 删除合集（不删除关联的电影，只清除关联关系）
func (r *MovieCollectionRepo) Delete(id string) error {
	// 先清除所有关联电影的 collection_id
	r.db.Model(&model.Media{}).Where("collection_id = ?", id).Update("collection_id", "")
	return r.db.Delete(&model.MovieCollection{}, "id = ?", id).Error
}

// GetMediaByCollectionID 获取合集下的所有电影（按年份排序）
func (r *MovieCollectionRepo) GetMediaByCollectionID(collectionID string) ([]model.Media, error) {
	var media []model.Media
	err := r.db.Where("collection_id = ?", collectionID).
		Order("year ASC, title ASC").
		Find(&media).Error
	return media, err
}

// FindCollectionByMediaID 根据媒体 ID 查找其所属的合集
func (r *MovieCollectionRepo) FindCollectionByMediaID(mediaID string) (*model.MovieCollection, error) {
	var media model.Media
	if err := r.db.Select("collection_id").First(&media, "id = ?", mediaID).Error; err != nil {
		return nil, err
	}
	if media.CollectionID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	return r.FindByIDWithMedia(media.CollectionID)
}

// UpdateMediaCount 更新合集的电影数量
func (r *MovieCollectionRepo) UpdateMediaCount(collectionID string) error {
	var count int64
	r.db.Model(&model.Media{}).Where("collection_id = ?", collectionID).Count(&count)
	return r.db.Model(&model.MovieCollection{}).Where("id = ?", collectionID).
		Update("media_count", count).Error
}

// ListMoviesWithoutCollection 获取没有合集的电影列表（用于自动匹配）
func (r *MovieCollectionRepo) ListMoviesWithoutCollection() ([]model.Media, error) {
	var media []model.Media
	err := r.db.Where("media_type = 'movie' AND (collection_id = '' OR collection_id IS NULL)").
		Where("title != ''").
		Order("title ASC").
		Find(&media).Error
	return media, err
}

// Search 搜索合集（排除空壳合集）
func (r *MovieCollectionRepo) Search(keyword string, limit int) ([]model.MovieCollection, error) {
	var colls []model.MovieCollection
	err := r.db.Where("name LIKE ? AND media_count > 0", "%"+keyword+"%").
		Order("media_count DESC").
		Limit(limit).
		Find(&colls).Error
	return colls, err
}
