package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ScrapeTaskRepo 刮削任务仓储
type ScrapeTaskRepo struct {
	db *gorm.DB
}

func (r *ScrapeTaskRepo) Create(task *model.ScrapeTask) error {
	return r.db.Create(task).Error
}

func (r *ScrapeTaskRepo) FindByID(id string) (*model.ScrapeTask, error) {
	var task model.ScrapeTask
	err := r.db.First(&task, "id = ?", id).Error
	return &task, err
}

func (r *ScrapeTaskRepo) Update(task *model.ScrapeTask) error {
	return r.db.Save(task).Error
}

func (r *ScrapeTaskRepo) Delete(id string) error {
	return r.db.Unscoped().Delete(&model.ScrapeTask{}, "id = ?", id).Error
}

func (r *ScrapeTaskRepo) List(page, size int, status, source string) ([]model.ScrapeTask, int64, error) {
	var tasks []model.ScrapeTask
	var total int64

	query := r.db.Model(&model.ScrapeTask{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if source != "" {
		query = query.Where("source = ?", source)
	}

	query.Count(&total)
	err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&tasks).Error
	return tasks, total, err
}

func (r *ScrapeTaskRepo) FindByURL(url string) (*model.ScrapeTask, error) {
	var task model.ScrapeTask
	err := r.db.Where("url = ?", url).First(&task).Error
	return &task, err
}

func (r *ScrapeTaskRepo) BatchDelete(ids []string) (int64, error) {
	result := r.db.Unscoped().Where("id IN ?", ids).Delete(&model.ScrapeTask{})
	return result.RowsAffected, result.Error
}

func (r *ScrapeTaskRepo) CountByStatus() (map[string]int64, error) {
	type StatusCount struct {
		Status string
		Count  int64
	}
	var results []StatusCount
	err := r.db.Model(&model.ScrapeTask{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64)
	for _, r := range results {
		counts[r.Status] = r.Count
	}
	return counts, nil
}

// ScrapeHistoryRepo 刮削历史仓储
type ScrapeHistoryRepo struct {
	db *gorm.DB
}

func (r *ScrapeHistoryRepo) Create(history *model.ScrapeHistory) error {
	return r.db.Create(history).Error
}

func (r *ScrapeHistoryRepo) ListByTaskID(taskID string, limit int) ([]model.ScrapeHistory, error) {
	var histories []model.ScrapeHistory
	err := r.db.Where("task_id = ?", taskID).Order("created_at DESC").Limit(limit).Find(&histories).Error
	return histories, err
}

func (r *ScrapeHistoryRepo) ListRecent(limit int) ([]model.ScrapeHistory, error) {
	var histories []model.ScrapeHistory
	err := r.db.Order("created_at DESC").Limit(limit).Find(&histories).Error
	return histories, err
}
