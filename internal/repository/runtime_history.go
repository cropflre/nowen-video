package repository

import (
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// RuntimeHistoryFilter is a read-only query contract over retired persistent
// Runtime execution metadata. It never participates in scheduling or playback.
type RuntimeHistoryFilter struct {
	Page     int
	PageSize int
	Status   string
	Intent   string
	MediaID  string
	Search   string
	From     *time.Time
	To       *time.Time
}

type RuntimeHistoryCounts struct {
	Jobs              int64
	Attempts          int64
	Artifacts         int64
	LegacyTasks       int64
	OrphanLegacyTasks int64
	ArtifactBytes     int64
	ByStatus          map[string]int64
	OldestAt          *time.Time
	NewestAt          *time.Time
}

type RuntimeHistoryRepo struct {
	db *gorm.DB
}

func NewRuntimeHistoryRepo(db *gorm.DB) *RuntimeHistoryRepo {
	return &RuntimeHistoryRepo{db: db}
}

func (r *RuntimeHistoryRepo) ListJobs(filter RuntimeHistoryFilter) ([]model.TranscodeJobRecord, int64, error) {
	query := r.db.Model(&model.TranscodeJobRecord{})
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(filter.Intent); value != "" {
		query = query.Where("intent = ?", value)
	}
	if value := strings.TrimSpace(filter.MediaID); value != "" {
		query = query.Where("media_id = ?", value)
	}
	if filter.From != nil {
		query = query.Where("created_at >= ?", *filter.From)
	}
	if filter.To != nil {
		query = query.Where("created_at <= ?", *filter.To)
	}
	if value := strings.TrimSpace(filter.Search); value != "" {
		like := "%" + value + "%"
		query = query.Where(
			"id LIKE ? OR media_id LIKE ? OR intent LIKE ? OR profile_id LIKE ? OR worker_id LIKE ? OR session_id LIKE ?",
			like, like, like, like, like, like,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 25
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var jobs []model.TranscodeJobRecord
	err := query.
		Order("COALESCE(completed_at, updated_at) DESC, created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&jobs).Error
	return jobs, total, err
}

func (r *RuntimeHistoryRepo) FindJob(id string) (*model.TranscodeJobRecord, error) {
	var job model.TranscodeJobRecord
	if err := r.db.First(&job, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *RuntimeHistoryRepo) ListAttempts(jobIDs []string) ([]model.TranscodeAttemptRecord, error) {
	if len(jobIDs) == 0 {
		return []model.TranscodeAttemptRecord{}, nil
	}
	var rows []model.TranscodeAttemptRecord
	err := r.db.Where("job_id IN ?", jobIDs).Order("job_id ASC, number ASC, created_at ASC").Find(&rows).Error
	return rows, err
}

func (r *RuntimeHistoryRepo) ListArtifacts(jobIDs []string) ([]model.TranscodeArtifactRecord, error) {
	if len(jobIDs) == 0 {
		return []model.TranscodeArtifactRecord{}, nil
	}
	var rows []model.TranscodeArtifactRecord
	err := r.db.Where("job_id IN ?", jobIDs).Order("job_id ASC, created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *RuntimeHistoryRepo) MediaTitles(mediaIDs []string) (map[string]string, error) {
	result := make(map[string]string)
	if len(mediaIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		ID    string
		Title string
	}
	if err := r.db.Model(&model.Media{}).Select("id", "title").Where("id IN ?", mediaIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row.Title
	}
	return result, nil
}

func (r *RuntimeHistoryRepo) Counts() (*RuntimeHistoryCounts, error) {
	counts := &RuntimeHistoryCounts{ByStatus: make(map[string]int64)}
	if err := r.db.Model(&model.TranscodeJobRecord{}).Count(&counts.Jobs).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&model.TranscodeAttemptRecord{}).Count(&counts.Attempts).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&model.TranscodeArtifactRecord{}).Count(&counts.Artifacts).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&model.TranscodeArtifactRecord{}).Select("COALESCE(SUM(size_bytes), 0)").Scan(&counts.ArtifactBytes).Error; err != nil {
		return nil, err
	}
	if r.db.Migrator().HasTable(&model.TranscodeTask{}) {
		if err := r.db.Model(&model.TranscodeTask{}).Count(&counts.LegacyTasks).Error; err != nil {
			return nil, err
		}
		if err := r.db.Table("transcode_tasks AS legacy").
			Joins("LEFT JOIN transcode_jobs AS jobs ON jobs.legacy_task_id = legacy.id").
			Where("jobs.id IS NULL").
			Count(&counts.OrphanLegacyTasks).Error; err != nil {
			return nil, err
		}
	}

	var statusRows []struct {
		Status string
		Count  int64
	}
	if err := r.db.Model(&model.TranscodeJobRecord{}).
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&statusRows).Error; err != nil {
		return nil, err
	}
	for _, row := range statusRows {
		counts.ByStatus[row.Status] = row.Count
	}

	var oldest model.TranscodeJobRecord
	if err := r.db.Model(&model.TranscodeJobRecord{}).
		Select("created_at").
		Order("created_at ASC").
		Take(&oldest).Error; err == nil {
		value := oldest.CreatedAt
		counts.OldestAt = &value
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	var newest model.TranscodeJobRecord
	if err := r.db.Model(&model.TranscodeJobRecord{}).
		Select("updated_at", "completed_at").
		Order("COALESCE(completed_at, updated_at) DESC").
		Take(&newest).Error; err == nil {
		if newest.CompletedAt != nil {
			value := *newest.CompletedAt
			counts.NewestAt = &value
		} else {
			value := newest.UpdatedAt
			counts.NewestAt = &value
		}
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return counts, nil
}
