package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// PreprocessRepo 视频预处理任务仓储
type PreprocessRepo struct {
	db *gorm.DB
}

// Create 创建预处理任务
func (r *PreprocessRepo) Create(task *model.PreprocessTask) error {
	return r.db.Create(task).Error
}

// Update 更新预处理任务
func (r *PreprocessRepo) Update(task *model.PreprocessTask) error {
	return r.db.Save(task).Error
}

// FindByID 根据 ID 查找任务
func (r *PreprocessRepo) FindByID(id string) (*model.PreprocessTask, error) {
	var task model.PreprocessTask
	err := r.db.First(&task, "id = ?", id).Error
	return &task, err
}

// FindByMediaID 根据媒体 ID 查找最新的预处理任务
func (r *PreprocessRepo) FindByMediaID(mediaID string) (*model.PreprocessTask, error) {
	var task model.PreprocessTask
	err := r.db.Where("media_id = ?", mediaID).Order("created_at DESC").First(&task).Error
	return &task, err
}

// FindActiveByMediaID 查找媒体的活跃任务（非终态）
func (r *PreprocessRepo) FindActiveByMediaID(mediaID string) (*model.PreprocessTask, error) {
	var task model.PreprocessTask
	err := r.db.Where("media_id = ? AND status IN ?", mediaID, []string{"pending", "queued", "running", "paused"}).
		Order("created_at DESC").First(&task).Error
	return &task, err
}

// ListPending 获取待处理的任务（按优先级和创建时间排序）
func (r *PreprocessRepo) ListPending(limit int) ([]model.PreprocessTask, error) {
	var tasks []model.PreprocessTask
	err := r.db.Where("status IN ?", []string{"pending", "queued"}).
		Order("priority DESC, created_at ASC").
		Limit(limit).Find(&tasks).Error
	return tasks, err
}

// ListRunning 获取正在运行的任务
func (r *PreprocessRepo) ListRunning() ([]model.PreprocessTask, error) {
	var tasks []model.PreprocessTask
	err := r.db.Where("status = ?", "running").Find(&tasks).Error
	return tasks, err
}

// ListAll 分页获取所有任务
func (r *PreprocessRepo) ListAll(page, pageSize int, status string) ([]model.PreprocessTask, int64, error) {
	var tasks []model.PreprocessTask
	var total int64

	query := r.db.Model(&model.PreprocessTask{})
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
func (r *PreprocessRepo) CountByStatus() (map[string]int64, error) {
	type Result struct {
		Status string
		Count  int64
	}
	var results []Result
	err := r.db.Model(&model.PreprocessTask{}).
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
func (r *PreprocessRepo) DeleteByID(id string) error {
	return r.db.Delete(&model.PreprocessTask{}, "id = ?", id).Error
}

// DeleteCompletedBefore 删除指定时间之前完成的任务记录
func (r *PreprocessRepo) DeleteCompletedBefore(beforeTime string) (int64, error) {
	result := r.db.Where("status = ? AND completed_at < ?", "completed", beforeTime).
		Delete(&model.PreprocessTask{})
	return result.RowsAffected, result.Error
}

// FindNeedRetry 查找需要重试的失败任务
func (r *PreprocessRepo) FindNeedRetry(limit int) ([]model.PreprocessTask, error) {
	var tasks []model.PreprocessTask
	err := r.db.Where("status = ? AND retries < max_retry", "failed").
		Order("priority DESC, created_at ASC").
		Limit(limit).Find(&tasks).Error
	return tasks, err
}

// BatchUpdateStatus 批量更新任务状态
func (r *PreprocessRepo) BatchUpdateStatus(ids []string, status string) error {
	return r.db.Model(&model.PreprocessTask{}).
		Where("id IN ?", ids).
		Update("status", status).Error
}

// FindByIDs 根据 ID 列表批量查找任务
func (r *PreprocessRepo) FindByIDs(ids []string) ([]model.PreprocessTask, error) {
	var tasks []model.PreprocessTask
	err := r.db.Where("id IN ?", ids).Find(&tasks).Error
	return tasks, err
}

// DeleteByIDs 批量删除任务（仅删除非运行中的任务）
func (r *PreprocessRepo) DeleteByIDs(ids []string) (int64, error) {
	result := r.db.Where("id IN ? AND status != ?", ids, "running").Delete(&model.PreprocessTask{})
	return result.RowsAffected, result.Error
}
