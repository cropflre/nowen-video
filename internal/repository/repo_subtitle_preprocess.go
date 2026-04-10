package repository

import (
	"fmt"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// SubtitlePreprocessRepo 字幕预处理任务仓储
type SubtitlePreprocessRepo struct {
	db *gorm.DB
}

// Create 创建字幕预处理任务
func (r *SubtitlePreprocessRepo) Create(task *model.SubtitlePreprocessTask) error {
	return r.db.Create(task).Error
}

// Update 更新字幕预处理任务
func (r *SubtitlePreprocessRepo) Update(task *model.SubtitlePreprocessTask) error {
	return r.db.Save(task).Error
}

// FindByID 根据 ID 查找任务
func (r *SubtitlePreprocessRepo) FindByID(id string) (*model.SubtitlePreprocessTask, error) {
	var task model.SubtitlePreprocessTask
	err := r.db.First(&task, "id = ?", id).Error
	return &task, err
}

// FindByMediaID 根据媒体 ID 查找最新的字幕预处理任务
func (r *SubtitlePreprocessRepo) FindByMediaID(mediaID string) (*model.SubtitlePreprocessTask, error) {
	var task model.SubtitlePreprocessTask
	err := r.db.Where("media_id = ?", mediaID).Order("created_at DESC").First(&task).Error
	return &task, err
}

// FindActiveByMediaID 查找媒体的活跃任务（非终态）
func (r *SubtitlePreprocessRepo) FindActiveByMediaID(mediaID string) (*model.SubtitlePreprocessTask, error) {
	var task model.SubtitlePreprocessTask
	err := r.db.Where("media_id = ? AND status IN ?", mediaID, []string{"pending", "running"}).
		Order("created_at DESC").First(&task).Error
	return &task, err
}

// ListPending 获取待处理的任务
func (r *SubtitlePreprocessRepo) ListPending(limit int) ([]model.SubtitlePreprocessTask, error) {
	var tasks []model.SubtitlePreprocessTask
	err := r.db.Where("status = ?", "pending").
		Order("created_at ASC").
		Limit(limit).Find(&tasks).Error
	return tasks, err
}

// ListRunning 获取正在运行的任务
func (r *SubtitlePreprocessRepo) ListRunning() ([]model.SubtitlePreprocessTask, error) {
	var tasks []model.SubtitlePreprocessTask
	err := r.db.Where("status = ?", "running").Find(&tasks).Error
	return tasks, err
}

// ListAll 分页获取所有任务
func (r *SubtitlePreprocessRepo) ListAll(page, pageSize int, status string) ([]model.SubtitlePreprocessTask, int64, error) {
	var tasks []model.SubtitlePreprocessTask
	var total int64

	query := r.db.Model(&model.SubtitlePreprocessTask{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	err := query.Preload("Media").Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&tasks).Error
	return tasks, total, err
}

// CountByStatus 按状态统计任务数量
func (r *SubtitlePreprocessRepo) CountByStatus() (map[string]int64, error) {
	type Result struct {
		Status string
		Count  int64
	}
	var results []Result
	err := r.db.Model(&model.SubtitlePreprocessTask{}).
		Select("status, COUNT(*) as count").
		Group("status").Scan(&results).Error
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int64)
	for _, r := range results {
		counts[r.Status] = r.Count
	}
	return counts, nil
}

// DeleteByID 删除任务
func (r *SubtitlePreprocessRepo) DeleteByID(id string) error {
	return r.db.Delete(&model.SubtitlePreprocessTask{}, "id = ?", id).Error
}

// FindByIDs 根据 ID 列表批量查找任务
func (r *SubtitlePreprocessRepo) FindByIDs(ids []string) ([]model.SubtitlePreprocessTask, error) {
	var tasks []model.SubtitlePreprocessTask
	err := r.db.Where("id IN ?", ids).Find(&tasks).Error
	return tasks, err
}

// DeleteByIDs 批量删除任务（仅删除非运行中的任务）
func (r *SubtitlePreprocessRepo) DeleteByIDs(ids []string) (int64, error) {
	result := r.db.Where("id IN ? AND status != ?", ids, "running").Delete(&model.SubtitlePreprocessTask{})
	return result.RowsAffected, result.Error
}

// RetryAllFailed 将所有失败任务重置为 pending
func (r *SubtitlePreprocessRepo) RetryAllFailed() ([]model.SubtitlePreprocessTask, error) {
	// 先查出所有失败任务
	var tasks []model.SubtitlePreprocessTask
	if err := r.db.Where("status = ?", "failed").Find(&tasks).Error; err != nil {
		return nil, err
	}
	// 批量更新状态
	if len(tasks) > 0 {
		r.db.Model(&model.SubtitlePreprocessTask{}).Where("status = ?", "failed").
			Updates(map[string]interface{}{"status": "pending", "error": "", "message": "重试中..."})
	}
	return tasks, nil
}

// DeleteByStatus 按状态批量删除任务（不允许删除 running 状态）
func (r *SubtitlePreprocessRepo) DeleteByStatus(status string) (int64, error) {
	if status == "running" {
		return 0, fmt.Errorf("不允许删除运行中的任务")
	}
	result := r.db.Where("status = ?", status).Delete(&model.SubtitlePreprocessTask{})
	return result.RowsAffected, result.Error
}
