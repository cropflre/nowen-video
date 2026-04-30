package repository

import (
	"fmt"
	"strings"
	"unicode"

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

// FindByIDWithMedia 根据 ID 查找合集并预加载关联的电影（按首映日期排序）
func (r *MovieCollectionRepo) FindByIDWithMedia(id string) (*model.MovieCollection, error) {
	var coll model.MovieCollection
	err := r.db.Preload("Media", func(db *gorm.DB) *gorm.DB {
		return db.Order("CASE WHEN premiered != '' THEN 0 ELSE 1 END ASC, premiered ASC, year ASC, title ASC")
	}).First(&coll, "id = ?", id).Error
	return &coll, err
}

// FindByName 根据名称精确查找合集（返回第一个匹配的）
func (r *MovieCollectionRepo) FindByName(name string) (*model.MovieCollection, error) {
	var coll model.MovieCollection
	err := r.db.Where("name = ?", name).First(&coll).Error
	return &coll, err
}

// FindByNameFuzzy 根据名称模糊查找合集
// 先尝试精确匹配，失败后通过标准化名称（去除空格、标点、全半角差异）进行匹配
// 这样 "逃学威龙" 和 "逃学威龙 " 或 "逃学威龙·" 都能匹配到同一合集
func (r *MovieCollectionRepo) FindByNameFuzzy(name string) (*model.MovieCollection, error) {
	// 1. 先尝试精确匹配
	if coll, err := r.FindByName(name); err == nil {
		return coll, nil
	}

	// 2. 精确匹配失败，使用标准化名称在所有合集中查找
	var allColls []model.MovieCollection
	if err := r.db.Find(&allColls).Error; err != nil {
		return nil, err
	}

	normalized := normalizeForMatch(name)
	for i := range allColls {
		if normalizeForMatch(allColls[i].Name) == normalized {
			return &allColls[i], nil
		}
	}

	return nil, gorm.ErrRecordNotFound
}

// normalizeForMatch 标准化名称用于模糊匹配
// 去除空格、常见标点、全半角差异，统一为小写
func normalizeForMatch(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		// 跳过空格和常见标点
		if unicode.IsSpace(r) || isIgnoredPunct(r) {
			continue
		}
		// 全角转半角
		switch {
		case r >= 'Ａ' && r <= 'Ｚ':
			r = r - 'Ａ' + 'A'
		case r >= 'ａ' && r <= 'ｚ':
			r = r - 'ａ' + 'a'
		case r >= '０' && r <= '９':
			r = r - '０' + '0'
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// isIgnoredPunct 判断是否为在比较时应忽略的标点符号
// 使用 unicode.IsPunct 判断，覆盖所有标点类别
func isIgnoredPunct(r rune) bool {
	return unicode.IsPunct(r)
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
	return r.ListWithOptions(page, size, "created_desc", "")
}

// ListWithOptions 支持排序和来源筛选的分页查询
// sort: created_desc | created_asc | updated_desc | updated_asc | name_asc | name_desc | count_desc | count_asc
// autoFilter: "" 全部 | "true" 自动匹配 | "false" 手动创建
func (r *MovieCollectionRepo) ListWithOptions(page, size int, sort, autoFilter string) ([]model.MovieCollection, int64, error) {
	var colls []model.MovieCollection
	var total int64

	query := r.db.Model(&model.MovieCollection{}).Where("media_count > 0")
	if autoFilter == "true" {
		query = query.Where("auto_matched = ?", true)
	} else if autoFilter == "false" {
		query = query.Where("auto_matched = ?", false)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 白名单映射：防止 SQL 注入
	orderExpr := "created_at DESC"
	switch sort {
	case "created_asc":
		orderExpr = "created_at ASC"
	case "updated_desc":
		orderExpr = "updated_at DESC"
	case "updated_asc":
		orderExpr = "updated_at ASC"
	case "name_asc":
		orderExpr = "name ASC"
	case "name_desc":
		orderExpr = "name DESC"
	case "count_desc":
		orderExpr = "media_count DESC"
	case "count_asc":
		orderExpr = "media_count ASC"
	case "created_desc":
		fallthrough
	default:
		orderExpr = "created_at DESC"
	}

	err := query.
		Order(orderExpr).
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

// GetMediaByCollectionID 获取合集下的所有电影（按首映日期排序）
func (r *MovieCollectionRepo) GetMediaByCollectionID(collectionID string) ([]model.Media, error) {
	var media []model.Media
	err := r.db.Where("collection_id = ?", collectionID).
		Order("CASE WHEN premiered != '' THEN 0 ELSE 1 END ASC, premiered ASC, year ASC, title ASC").
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

// UpdateMediaCount 更新合集的电影数量和年份范围
// media_count 按 version_group 去重（同一部电影的不同版本算 1 部），贴合前端"系列里有几部电影"的语义；
// file_count 为原始文件总数（每个版本各算一个）。
func (r *MovieCollectionRepo) UpdateMediaCount(collectionID string) error {
	var fileCount int64
	r.db.Model(&model.Media{}).Where("collection_id = ?", collectionID).Count(&fileCount)

	// 去重电影数：有 version_group 的按 version_group 聚合成 1 部；
	// 无 version_group（空字符串）的每条独立算 1 部，用 id 兜底保证互不合并。
	var movieCount int64
	r.db.Model(&model.Media{}).
		Where("collection_id = ?", collectionID).
		Distinct("CASE WHEN version_group IS NULL OR version_group = '' THEN id ELSE version_group END").
		Count(&movieCount)

	// 计算年份范围
	yearRange := r.computeYearRange(collectionID)

	return r.db.Model(&model.MovieCollection{}).Where("id = ?", collectionID).
		Updates(map[string]interface{}{
			"media_count": movieCount,
			"file_count":  fileCount,
			"year_range":  yearRange,
		}).Error
}

// computeYearRange 根据合集中电影的年份计算年份范围
// 如果所有电影同年返回 "2020"，否则返回 "1991-1993"
func (r *MovieCollectionRepo) computeYearRange(collectionID string) string {
	var minYear, maxYear int
	// 使用 sql.NullInt64 兜底，避免合集内无有效年份数据时 Scan 失败
	row := r.db.Model(&model.Media{}).
		Where("collection_id = ? AND year > 0", collectionID).
		Select("MIN(year), MAX(year)").
		Row()
	if err := row.Scan(&minYear, &maxYear); err != nil {
		// 没有数据或 Scan 出错时，直接返回空字符串（不抛出异常，让上层使用默认值）
		return ""
	}

	if minYear == 0 && maxYear == 0 {
		return ""
	}
	if minYear == maxYear {
		return fmt.Sprintf("%d", minYear)
	}
	return fmt.Sprintf("%d-%d", minYear, maxYear)
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

// ClearAutoMatchedAssociations 清除所有自动匹配合集的电影关联（将 collection_id 置空）
// 保留手动创建的合集（auto_matched = false）及其关联
func (r *MovieCollectionRepo) ClearAutoMatchedAssociations() (int64, error) {
	// 获取所有自动匹配的合集 ID
	var autoMatchedIDs []string
	if err := r.db.Model(&model.MovieCollection{}).
		Where("auto_matched = true").
		Pluck("id", &autoMatchedIDs).Error; err != nil {
		return 0, err
	}

	if len(autoMatchedIDs) == 0 {
		return 0, nil
	}

	// 清除这些合集下电影的 collection_id
	result := r.db.Model(&model.Media{}).
		Where("collection_id IN ?", autoMatchedIDs).
		Update("collection_id", "")
	return result.RowsAffected, result.Error
}

// DeleteAutoMatchedCollections 删除所有自动匹配的合集记录
// 仅删除 auto_matched = true 的合集，保留手动创建的
func (r *MovieCollectionRepo) DeleteAutoMatchedCollections() (int64, error) {
	result := r.db.Where("auto_matched = true").Delete(&model.MovieCollection{})
	return result.RowsAffected, result.Error
}
