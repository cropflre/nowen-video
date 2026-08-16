package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== V3: VideoChapterRepo ====================

type VideoChapterRepo struct {
	db *gorm.DB
}

func (r *VideoChapterRepo) Create(chapter *model.VideoChapter) error { return r.db.Create(chapter).Error }
func (r *VideoChapterRepo) ListByMediaID(mediaID string) ([]model.VideoChapter, error) {
	var chapters []model.VideoChapter
	err := r.db.Where("media_id = ?", mediaID).Order("start_time ASC").Find(&chapters).Error
	return chapters, err
}
func (r *VideoChapterRepo) DeleteByMediaID(mediaID string) error { return r.db.Where("media_id = ?", mediaID).Delete(&model.VideoChapter{}).Error }
func (r *VideoChapterRepo) FindByID(id string) (*model.VideoChapter, error) {
	var chapter model.VideoChapter
	err := r.db.First(&chapter, "id = ?", id).Error
	return &chapter, err
}
func (r *VideoChapterRepo) Update(chapter *model.VideoChapter) error { return r.db.Save(chapter).Error }
func (r *VideoChapterRepo) Delete(id string) error { return r.db.Delete(&model.VideoChapter{}, "id = ?", id).Error }

// ==================== V3: VideoHighlightRepo ====================

type VideoHighlightRepo struct { db *gorm.DB }
func (r *VideoHighlightRepo) Create(highlight *model.VideoHighlight) error { return r.db.Create(highlight).Error }
func (r *VideoHighlightRepo) ListByMediaID(mediaID string) ([]model.VideoHighlight, error) {
	var highlights []model.VideoHighlight
	err := r.db.Where("media_id = ?", mediaID).Order("score DESC, start_time ASC").Find(&highlights).Error
	return highlights, err
}
func (r *VideoHighlightRepo) FindByID(id string) (*model.VideoHighlight, error) {
	var highlight model.VideoHighlight
	err := r.db.First(&highlight, "id = ?", id).Error
	return &highlight, err
}
func (r *VideoHighlightRepo) Update(highlight *model.VideoHighlight) error { return r.db.Save(highlight).Error }
func (r *VideoHighlightRepo) DeleteByMediaID(mediaID string) error { return r.db.Where("media_id = ?", mediaID).Delete(&model.VideoHighlight{}).Error }
func (r *VideoHighlightRepo) Delete(id string) error { return r.db.Delete(&model.VideoHighlight{}, "id = ?", id).Error }

// ReplaceByMediaID 原子替换一个媒体的全部精彩片段。
func (r *VideoHighlightRepo) ReplaceByMediaID(mediaID string, highlights []model.VideoHighlight) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("media_id = ?", mediaID).Delete(&model.VideoHighlight{}).Error; err != nil { return err }
		if len(highlights) == 0 { return nil }
		return tx.Create(&highlights).Error
	})
}

// ==================== V3: AIAnalysisTaskRepo ====================

type AIAnalysisTaskRepo struct { db *gorm.DB }
func (r *AIAnalysisTaskRepo) Create(task *model.AIAnalysisTask) error { return r.db.Create(task).Error }
func (r *AIAnalysisTaskRepo) FindByID(id string) (*model.AIAnalysisTask, error) {
	var task model.AIAnalysisTask
	err := r.db.First(&task, "id = ?", id).Error
	return &task, err
}
func (r *AIAnalysisTaskRepo) Update(task *model.AIAnalysisTask) error { return r.db.Save(task).Error }
func (r *AIAnalysisTaskRepo) ListByMediaID(mediaID string) ([]model.AIAnalysisTask, error) {
	var tasks []model.AIAnalysisTask
	err := r.db.Where("media_id = ?", mediaID).Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}
func (r *AIAnalysisTaskRepo) DeleteByMediaID(mediaID string) error { return r.db.Where("media_id = ?", mediaID).Delete(&model.AIAnalysisTask{}).Error }
func (r *AIAnalysisTaskRepo) ListByStatus(status string, limit int) ([]model.AIAnalysisTask, error) {
	var tasks []model.AIAnalysisTask
	err := r.db.Where("status = ?", status).Order("created_at ASC").Limit(limit).Find(&tasks).Error
	return tasks, err
}
func (r *AIAnalysisTaskRepo) FindActiveByMediaAndType(mediaID, taskType string) (*model.AIAnalysisTask, error) {
	var task model.AIAnalysisTask
	err := r.db.Where("media_id = ? AND task_type = ? AND status IN ?", mediaID, taskType, []string{"pending", "running"}).Order("created_at DESC").First(&task).Error
	return &task, err
}
func (r *AIAnalysisTaskRepo) MarkRunningInterrupted(taskType string) error {
	return r.db.Model(&model.AIAnalysisTask{}).
		Where("task_type = ? AND status IN ?", taskType, []string{"pending", "running"}).
		Updates(map[string]any{
			"status": "interrupted", "stage": "interrupted", "error": "服务重启导致任务中断，请重新分析",
		}).Error
}

// ==================== V3: CoverCandidateRepo ====================

type CoverCandidateRepo struct { db *gorm.DB }
func (r *CoverCandidateRepo) Create(candidate *model.CoverCandidate) error { return r.db.Create(candidate).Error }
func (r *CoverCandidateRepo) ListByMediaID(mediaID string) ([]model.CoverCandidate, error) {
	var candidates []model.CoverCandidate
	err := r.db.Where("media_id = ?", mediaID).Order("score DESC").Find(&candidates).Error
	return candidates, err
}
func (r *CoverCandidateRepo) DeleteByMediaID(mediaID string) error { return r.db.Where("media_id = ?", mediaID).Delete(&model.CoverCandidate{}).Error }
func (r *CoverCandidateRepo) SelectCover(mediaID, candidateID string) error {
	r.db.Model(&model.CoverCandidate{}).Where("media_id = ?", mediaID).Update("is_selected", false)
	return r.db.Model(&model.CoverCandidate{}).Where("id = ?", candidateID).Update("is_selected", true).Error
}