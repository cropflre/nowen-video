package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== 扫描后处理：虚拟归类与命名映射仓储 ====================
//
// 该仓储只操作 media_classifications 表，与磁盘文件无关。

// ScanClassificationRepo 扫描后处理产出仓储
type ScanClassificationRepo struct {
	db *gorm.DB
}

// NewScanClassificationRepo 构造函数
func NewScanClassificationRepo(db *gorm.DB) *ScanClassificationRepo {
	return &ScanClassificationRepo{db: db}
}

// DB 暴露底层连接（按需）
func (r *ScanClassificationRepo) DB() *gorm.DB {
	return r.db
}

// Upsert 按 MediaID 覆盖写入：存在则更新，不存在则创建。
func (r *ScanClassificationRepo) Upsert(c *model.MediaClassification) error {
	if c == nil || c.MediaID == "" {
		return gorm.ErrInvalidData
	}
	var existing model.MediaClassification
	err := r.db.Where("media_id = ?", c.MediaID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(c).Error
	}
	if err != nil {
		return err
	}
	// 保留 ID/CreatedAt，其他字段全替换
	c.ID = existing.ID
	c.CreatedAt = existing.CreatedAt
	return r.db.Save(c).Error
}

// MarkRunning 把若干 MediaID 标为 running（异步队列状态推进）
func (r *ScanClassificationRepo) MarkRunning(mediaID string) error {
	now := time.Now()
	return r.db.Model(&model.MediaClassification{}).
		Where("media_id = ?", mediaID).
		Updates(map[string]interface{}{
			"status":     model.ClassificationStatusRunning,
			"updated_at": now,
		}).Error
}

// MarkFailed 标记失败状态
func (r *ScanClassificationRepo) MarkFailed(mediaID, errMsg string) error {
	now := time.Now()
	return r.db.Model(&model.MediaClassification{}).
		Where("media_id = ?", mediaID).
		Updates(map[string]interface{}{
			"status":       model.ClassificationStatusFailed,
			"error_msg":    errMsg,
			"processed_at": &now,
			"updated_at":   now,
		}).Error
}

// FindByMediaID 按 MediaID 查询单条
func (r *ScanClassificationRepo) FindByMediaID(mediaID string) (*model.MediaClassification, error) {
	var c model.MediaClassification
	err := r.db.Where("media_id = ?", mediaID).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListFilter 列表查询过滤参数
type ClassificationListFilter struct {
	LibraryID string // 可选
	Status    string // 可选 pending/running/processed/partial/failed
	Category  string // 可选 movie/tvshow/...
	Region    string // 可选 CN/JP/...
	Decade    string // 可选 2020s/2010s/...
	Keyword   string // 可选：模糊匹配 ParsedTitle/SuggestedName
	MinScore  float64 // 可选：confidence >=
	Page      int
	Size      int
}

// List 按过滤条件分页查询，返回记录与总数
func (r *ScanClassificationRepo) List(f ClassificationListFilter) ([]model.MediaClassification, int64, error) {
	q := r.db.Model(&model.MediaClassification{})
	if f.LibraryID != "" {
		q = q.Where("library_id = ?", f.LibraryID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Category != "" {
		q = q.Where("category = ?", f.Category)
	}
	if f.Region != "" {
		q = q.Where("region = ?", f.Region)
	}
	if f.Decade != "" {
		q = q.Where("decade = ?", f.Decade)
	}
	if f.Keyword != "" {
		like := "%" + f.Keyword + "%"
		q = q.Where("parsed_title LIKE ? OR suggested_name LIKE ?", like, like)
	}
	if f.MinScore > 0 {
		q = q.Where("confidence >= ?", f.MinScore)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := f.Page
	if page <= 0 {
		page = 1
	}
	size := f.Size
	if size <= 0 || size > 500 {
		size = 50
	}
	var list []model.MediaClassification
	err := q.Order("updated_at DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&list).Error
	return list, total, err
}

// StatsItem 单个统计桶
type ClassificationStatsItem struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// Stats 聚合统计：按 status/category/region/decade 分别返回桶
type ClassificationStats struct {
	Total      int64                     `json:"total"`
	ByStatus   []ClassificationStatsItem `json:"by_status"`
	ByCategory []ClassificationStatsItem `json:"by_category"`
	ByRegion   []ClassificationStatsItem `json:"by_region"`
	ByDecade   []ClassificationStatsItem `json:"by_decade"`
}

// Stats 计算汇总
func (r *ScanClassificationRepo) Stats(libraryID string) (*ClassificationStats, error) {
	out := &ClassificationStats{}
	base := r.db.Model(&model.MediaClassification{})
	if libraryID != "" {
		base = base.Where("library_id = ?", libraryID)
	}
	if err := base.Count(&out.Total).Error; err != nil {
		return nil, err
	}

	bucket := func(field string) ([]ClassificationStatsItem, error) {
		var rows []struct {
			Key   string `gorm:"column:key"`
			Count int64  `gorm:"column:count"`
		}
		q := r.db.Model(&model.MediaClassification{}).
			Select(field+" as key, COUNT(*) as count").
			Where(field + " != ''")
		if libraryID != "" {
			q = q.Where("library_id = ?", libraryID)
		}
		if err := q.Group(field).Order("count DESC").Scan(&rows).Error; err != nil {
			return nil, err
		}
		items := make([]ClassificationStatsItem, 0, len(rows))
		for _, r := range rows {
			items = append(items, ClassificationStatsItem{Key: r.Key, Count: r.Count})
		}
		return items, nil
	}

	var err error
	if out.ByStatus, err = bucket("status"); err != nil {
		return nil, err
	}
	if out.ByCategory, err = bucket("category"); err != nil {
		return nil, err
	}
	if out.ByRegion, err = bucket("region"); err != nil {
		return nil, err
	}
	if out.ByDecade, err = bucket("decade"); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteByMediaID 删除（按需级联清理时使用）
func (r *ScanClassificationRepo) DeleteByMediaID(mediaID string) error {
	return r.db.Unscoped().Where("media_id = ?", mediaID).Delete(&model.MediaClassification{}).Error
}

// DeleteByLibraryID 整库清理（重跑前可选）
func (r *ScanClassificationRepo) DeleteByLibraryID(libraryID string) (int64, error) {
	res := r.db.Unscoped().Where("library_id = ?", libraryID).Delete(&model.MediaClassification{})
	return res.RowsAffected, res.Error
}

// DeleteAll 清空所有分类记录（危险操作，调用方应确认）
func (r *ScanClassificationRepo) DeleteAll() (int64, error) {
	res := r.db.Unscoped().Where("1 = 1").Delete(&model.MediaClassification{})
	return res.RowsAffected, res.Error
}
