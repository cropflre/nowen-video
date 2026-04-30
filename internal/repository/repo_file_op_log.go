package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== 文件管理操作日志 Repository ====================

// FileOpLogRepo 文件管理操作日志仓储（持久化到数据库，解决进程重启/多副本丢失问题）
type FileOpLogRepo struct {
	db *gorm.DB
}

// NewFileOpLogRepo 创建操作日志 Repository
func NewFileOpLogRepo(db *gorm.DB) *FileOpLogRepo {
	return &FileOpLogRepo{db: db}
}

// Create 记录一条操作日志
func (r *FileOpLogRepo) Create(log *model.FileOperationLog) error {
	return r.db.Create(log).Error
}

// List 获取最近 N 条操作日志（按时间倒序）
func (r *FileOpLogRepo) List(limit int) ([]model.FileOperationLog, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	var logs []model.FileOperationLog
	err := r.db.Order("created_at DESC").Limit(limit).Find(&logs).Error
	return logs, err
}

// Count 获取日志总条数（用于统计"操作记录"指标）
func (r *FileOpLogRepo) Count() (int64, error) {
	var count int64
	err := r.db.Model(&model.FileOperationLog{}).Count(&count).Error
	return count, err
}

// CountSince 获取指定时间点之后的日志条数
func (r *FileOpLogRepo) CountSince(sqliteInterval string) (int64, error) {
	var count int64
	err := r.db.Model(&model.FileOperationLog{}).
		Where("created_at >= datetime('now', ?)", sqliteInterval).
		Count(&count).Error
	return count, err
}

// Cleanup 清理超过 keepCount 条的旧日志（防止无限增长）
func (r *FileOpLogRepo) Cleanup(keepCount int) (int64, error) {
	if keepCount <= 0 {
		return 0, nil
	}
	// 查询第 keepCount 条的时间，删除之前的所有记录
	var cutoff model.FileOperationLog
	if err := r.db.Order("created_at DESC").Offset(keepCount).Limit(1).First(&cutoff).Error; err != nil {
		// 没有超过 keepCount 条
		return 0, nil
	}
	result := r.db.Unscoped().Where("created_at < ?", cutoff.CreatedAt).Delete(&model.FileOperationLog{})
	return result.RowsAffected, result.Error
}
