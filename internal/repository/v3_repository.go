package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== V3: VideoChapterRepo ====================

type VideoChapterRepo struct {
	db *gorm.DB
}

func (r *VideoChapterRepo) Create(chapter *model.VideoChapter) error {
	return r.db.Create(chapter).Error
}

func (r *VideoChapterRepo) ListByMediaID(mediaID string) ([]model.VideoChapter, error) {
	var chapters []model.VideoChapter
	err := r.db.Where("media_id = ?", mediaID).Order("start_time ASC").Find(&chapters).Error
	return chapters, err
}

func (r *VideoChapterRepo) DeleteByMediaID(mediaID string) error {
	return r.db.Where("media_id = ?", mediaID).Delete(&model.VideoChapter{}).Error
}

func (r *VideoChapterRepo) FindByID(id string) (*model.VideoChapter, error) {
	var chapter model.VideoChapter
	err := r.db.First(&chapter, "id = ?", id).Error
	return &chapter, err
}

func (r *VideoChapterRepo) Update(chapter *model.VideoChapter) error {
	return r.db.Save(chapter).Error
}

func (r *VideoChapterRepo) Delete(id string) error {
	return r.db.Delete(&model.VideoChapter{}, "id = ?", id).Error
}

// ==================== V3: VideoHighlightRepo ====================

type VideoHighlightRepo struct {
	db *gorm.DB
}

func (r *VideoHighlightRepo) Create(highlight *model.VideoHighlight) error {
	return r.db.Create(highlight).Error
}

func (r *VideoHighlightRepo) ListByMediaID(mediaID string) ([]model.VideoHighlight, error) {
	var highlights []model.VideoHighlight
	err := r.db.Where("media_id = ?", mediaID).Order("score DESC").Find(&highlights).Error
	return highlights, err
}

func (r *VideoHighlightRepo) DeleteByMediaID(mediaID string) error {
	return r.db.Where("media_id = ?", mediaID).Delete(&model.VideoHighlight{}).Error
}

func (r *VideoHighlightRepo) Delete(id string) error {
	return r.db.Delete(&model.VideoHighlight{}, "id = ?", id).Error
}

// ==================== V3: AIAnalysisTaskRepo ====================

type AIAnalysisTaskRepo struct {
	db *gorm.DB
}

func (r *AIAnalysisTaskRepo) Create(task *model.AIAnalysisTask) error {
	return r.db.Create(task).Error
}

func (r *AIAnalysisTaskRepo) FindByID(id string) (*model.AIAnalysisTask, error) {
	var task model.AIAnalysisTask
	err := r.db.First(&task, "id = ?", id).Error
	return &task, err
}

func (r *AIAnalysisTaskRepo) Update(task *model.AIAnalysisTask) error {
	return r.db.Save(task).Error
}

func (r *AIAnalysisTaskRepo) ListByMediaID(mediaID string) ([]model.AIAnalysisTask, error) {
	var tasks []model.AIAnalysisTask
	err := r.db.Where("media_id = ?", mediaID).Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

func (r *AIAnalysisTaskRepo) ListByStatus(status string, limit int) ([]model.AIAnalysisTask, error) {
	var tasks []model.AIAnalysisTask
	err := r.db.Where("status = ?", status).Order("created_at ASC").Limit(limit).Find(&tasks).Error
	return tasks, err
}

// ==================== V3: CoverCandidateRepo ====================

type CoverCandidateRepo struct {
	db *gorm.DB
}

func (r *CoverCandidateRepo) Create(candidate *model.CoverCandidate) error {
	return r.db.Create(candidate).Error
}

func (r *CoverCandidateRepo) ListByMediaID(mediaID string) ([]model.CoverCandidate, error) {
	var candidates []model.CoverCandidate
	err := r.db.Where("media_id = ?", mediaID).Order("score DESC").Find(&candidates).Error
	return candidates, err
}

func (r *CoverCandidateRepo) DeleteByMediaID(mediaID string) error {
	return r.db.Where("media_id = ?", mediaID).Delete(&model.CoverCandidate{}).Error
}

func (r *CoverCandidateRepo) SelectCover(mediaID, candidateID string) error {
	// 先取消所有选中
	r.db.Model(&model.CoverCandidate{}).Where("media_id = ?", mediaID).Update("is_selected", false)
	// 选中指定候选
	return r.db.Model(&model.CoverCandidate{}).Where("id = ?", candidateID).Update("is_selected", true).Error
}
