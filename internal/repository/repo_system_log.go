package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// SystemLogRepo 系统日志仓库
type SystemLogRepo struct {
	db *gorm.DB
}

// Create 创建日志记录
func (r *SystemLogRepo) Create(log *model.SystemLog) error {
	return r.db.Create(log).Error
}

// BatchCreate 批量创建日志（高性能写入）
func (r *SystemLogRepo) BatchCreate(logs []*model.SystemLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.CreateInBatches(logs, 100).Error
}

// SystemLogFilter 日志查询过滤条件
type SystemLogFilter struct {
	Type      string // api / playback / system
	Level     string // debug / info / warn / error
	Keyword   string // 模糊搜索（message / path / detail）
	Method    string // HTTP 方法
	Path      string // 请求路径前缀
	StartTime *time.Time
	EndTime   *time.Time
	UserID    string
	MediaID   string
	MinStatus int // 最小状态码
	MaxStatus int // 最大状态码
}

// List 分页查询日志
func (r *SystemLogRepo) List(page, size int, filter *SystemLogFilter) ([]model.SystemLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 50
	}

	q := r.db.Model(&model.SystemLog{})
	q = r.applyFilter(q, filter)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.SystemLog
	err := q.Order("created_at DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&logs).Error
	return logs, total, err
}

// ListForExport 导出查询（不分页，限制最大条数）
func (r *SystemLogRepo) ListForExport(maxRows int, filter *SystemLogFilter) ([]model.SystemLog, error) {
	if maxRows <= 0 || maxRows > 10000 {
		maxRows = 5000
	}

	q := r.db.Model(&model.SystemLog{})
	q = r.applyFilter(q, filter)

	var logs []model.SystemLog
	err := q.Order("created_at DESC").
		Limit(maxRows).
		Find(&logs).Error
	return logs, err
}

// GetStats 获取日志统计信息
func (r *SystemLogRepo) GetStats() (*SystemLogStats, error) {
	stats := &SystemLogStats{}

	// 总数
	r.db.Model(&model.SystemLog{}).Count(&stats.Total)

	// 按类型统计
	var typeCounts []struct {
		Type  string
		Count int64
	}
	r.db.Model(&model.SystemLog{}).
		Select("type, count(*) as count").
		Group("type").
		Find(&typeCounts)
	stats.TypeCounts = make(map[string]int64)
	for _, tc := range typeCounts {
		stats.TypeCounts[tc.Type] = tc.Count
	}

	// 按级别统计
	var levelCounts []struct {
		Level string
		Count int64
	}
	r.db.Model(&model.SystemLog{}).
		Select("level, count(*) as count").
		Group("level").
		Find(&levelCounts)
	stats.LevelCounts = make(map[string]int64)
	for _, lc := range levelCounts {
		stats.LevelCounts[lc.Level] = lc.Count
	}

	// 今日日志数
	today := time.Now().Truncate(24 * time.Hour)
	r.db.Model(&model.SystemLog{}).Where("created_at >= ?", today).Count(&stats.TodayCount)

	// 今日错误数
	r.db.Model(&model.SystemLog{}).Where("created_at >= ? AND level = ?", today, model.LogLevelError).Count(&stats.TodayErrors)

	return stats, nil
}

// CleanOlderThan 清理超过指定天数的日志
func (r *SystemLogRepo) CleanOlderThan(days int) (int64, error) {
	if days <= 0 {
		return 0, nil
	}
	threshold := time.Now().AddDate(0, 0, -days)
	result := r.db.Where("created_at < ?", threshold).Delete(&model.SystemLog{})
	return result.RowsAffected, result.Error
}

// applyFilter 应用过滤条件
func (r *SystemLogRepo) applyFilter(q *gorm.DB, filter *SystemLogFilter) *gorm.DB {
	if filter == nil {
		return q
	}
	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}
	if filter.Level != "" {
		q = q.Where("level = ?", filter.Level)
	}
	if filter.Method != "" {
		q = q.Where("method = ?", strings.ToUpper(filter.Method))
	}
	if filter.Path != "" {
		q = q.Where("path LIKE ?", filter.Path+"%")
	}
	if filter.UserID != "" {
		q = q.Where("user_id = ?", filter.UserID)
	}
	if filter.MediaID != "" {
		q = q.Where("media_id = ?", filter.MediaID)
	}
	if filter.StartTime != nil {
		q = q.Where("created_at >= ?", *filter.StartTime)
	}
	if filter.EndTime != nil {
		q = q.Where("created_at <= ?", *filter.EndTime)
	}
	if filter.MinStatus > 0 {
		q = q.Where("status_code >= ?", filter.MinStatus)
	}
	if filter.MaxStatus > 0 {
		q = q.Where("status_code <= ?", filter.MaxStatus)
	}
	if filter.Keyword != "" {
		kw := fmt.Sprintf("%%%s%%", filter.Keyword)
		q = q.Where("message LIKE ? OR path LIKE ? OR detail LIKE ?", kw, kw, kw)
	}
	return q
}

// SystemLogStats 日志统计
type SystemLogStats struct {
	Total       int64            `json:"total"`
	TodayCount  int64            `json:"today_count"`
	TodayErrors int64            `json:"today_errors"`
	TypeCounts  map[string]int64 `json:"type_counts"`
	LevelCounts map[string]int64 `json:"level_counts"`
}
