package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== TranscodeRepo ====================

type TranscodeRepo struct {
	db *gorm.DB
}

func (r *TranscodeRepo) Create(task *model.TranscodeTask) error {
	return r.db.Create(task).Error
}

func (r *TranscodeRepo) Update(task *model.TranscodeTask) error {
	return r.db.Save(task).Error
}

// UpdateProgress 仅更新进度字段（避免全量 Save 导致 SQLite 写锁竞争）
func (r *TranscodeRepo) UpdateProgress(taskID string, progress float64) error {
	return r.db.Model(&model.TranscodeTask{}).Where("id = ?", taskID).
		Updates(map[string]interface{}{"progress": progress, "updated_at": time.Now()}).Error
}

func (r *TranscodeRepo) FindByMediaAndQuality(mediaID, quality string) (*model.TranscodeTask, error) {
	var task model.TranscodeTask
	err := r.db.Where("media_id = ? AND quality = ? AND status = ?", mediaID, quality, "done").First(&task).Error
	return &task, err
}

func (r *TranscodeRepo) ListRunning() ([]model.TranscodeTask, error) {
	var tasks []model.TranscodeTask
	err := r.db.Where("status IN ?", []string{"pending", "running"}).Find(&tasks).Error
	return tasks, err
}

// ListStaleDone 查询 done 状态且 updated_at 早于 before 的任务（用于缓存清理）
func (r *TranscodeRepo) ListStaleDone(before time.Time) ([]model.TranscodeTask, error) {
	var tasks []model.TranscodeTask
	err := r.db.Where("status = ? AND updated_at < ?", "done", before).Find(&tasks).Error
	return tasks, err
}

// ListStaleFailed 查询 failed/cancelled 状态且 updated_at 早于 before 的任务
func (r *TranscodeRepo) ListStaleFailed(before time.Time) ([]model.TranscodeTask, error) {
	var tasks []model.TranscodeTask
	err := r.db.Where("status IN ? AND updated_at < ?", []string{"failed", "cancelled"}, before).Find(&tasks).Error
	return tasks, err
}

// DeleteByID 根据 ID 删除任务记录
func (r *TranscodeRepo) DeleteByID(id string) error {
	return r.db.Delete(&model.TranscodeTask{}, "id = ?", id).Error
}

// ==================== PlaybackStatsRepo ====================

type PlaybackStatsRepo struct {
	db *gorm.DB
}

func (r *PlaybackStatsRepo) Record(stat *model.PlaybackStats) error {
	return r.db.Create(stat).Error
}

func (r *PlaybackStatsRepo) GetUserDailyStats(userID string, startDate, endDate string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	err := r.db.Model(&model.PlaybackStats{}).
		Select("date, SUM(watch_minutes) as total_minutes, COUNT(DISTINCT media_id) as media_count").
		Where("user_id = ? AND date >= ? AND date <= ?", userID, startDate, endDate).
		Group("date").Order("date ASC").
		Scan(&results).Error
	return results, err
}

func (r *PlaybackStatsRepo) GetUserTotalMinutes(userID string) (float64, error) {
	var total float64
	err := r.db.Model(&model.PlaybackStats{}).Where("user_id = ?", userID).
		Select("COALESCE(SUM(watch_minutes), 0)").Scan(&total).Error
	return total, err
}

func (r *PlaybackStatsRepo) GetUserTopGenres(userID string, limit int) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	err := r.db.Raw(`
		SELECT m.genres, SUM(ps.watch_minutes) as total_minutes
		FROM playback_stats ps
		JOIN media m ON ps.media_id = m.id
		WHERE ps.user_id = ? AND m.genres != ''
		GROUP BY m.genres
		ORDER BY total_minutes DESC
		LIMIT ?
	`, userID, limit).Scan(&results).Error
	return results, err
}

// GetMediaStats 获取指定媒体的播放统计（总播放次数、总观看分钟数、独立观看人数）
func (r *PlaybackStatsRepo) GetMediaStats(mediaID string) (totalMinutes float64, totalCount int64, uniqueViewers int64, err error) {
	err = r.db.Model(&model.PlaybackStats{}).
		Where("media_id = ?", mediaID).
		Select("COALESCE(SUM(watch_minutes), 0)").Scan(&totalMinutes).Error
	if err != nil {
		return
	}
	err = r.db.Model(&model.PlaybackStats{}).
		Where("media_id = ?", mediaID).
		Count(&totalCount).Error
	if err != nil {
		return
	}
	err = r.db.Model(&model.PlaybackStats{}).
		Where("media_id = ?", mediaID).
		Select("COUNT(DISTINCT user_id)").Scan(&uniqueViewers).Error
	return
}

// GetMostWatchedMedia 获取用户观看最多的影视（电影按 media 维度聚合，电视剧按 series 维度聚合）
// 对于剧集类型（media_type='episode'），使用所属剧集合集（series）的标题与海报进行展示，
// 避免显示为单集的文件名；同一部电视剧的所有集的观看时长会累加到一起。
// 返回字段中的 media_type 为 'series'（电视剧）或 'movie'（电影），便于前端选择正确的海报接口。
func (r *PlaybackStatsRepo) GetMostWatchedMedia(userID string, limit int) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	// 使用子查询先计算出聚合维度（media_id/title/poster_path/media_type），
	// 外层再按这些别名进行 GROUP BY，避免 SQLite 在 GROUP BY 时因 m.title 与 s.title
	// 同名而报 "ambiguous column name: title" 的错误。
	err := r.db.Raw(`
		SELECT media_id, title, poster_path, media_type, SUM(watch_minutes) as total_minutes
		FROM (
			SELECT
				CASE WHEN m.media_type = 'episode' AND m.series_id != '' THEN m.series_id ELSE ps.media_id END AS media_id,
				CASE WHEN m.media_type = 'episode' AND s.title != '' THEN s.title ELSE m.title END AS title,
				CASE WHEN m.media_type = 'episode' AND s.poster_path != '' THEN s.poster_path ELSE m.poster_path END AS poster_path,
				CASE WHEN m.media_type = 'episode' AND m.series_id != '' THEN 'series' ELSE 'movie' END AS media_type,
				ps.watch_minutes AS watch_minutes
			FROM playback_stats ps
			JOIN media m ON ps.media_id = m.id AND m.deleted_at IS NULL
			LEFT JOIN series s ON m.series_id = s.id AND s.deleted_at IS NULL
			WHERE ps.user_id = ?
		) t
		GROUP BY media_id, title, poster_path, media_type
		ORDER BY total_minutes DESC
		LIMIT ?
	`, userID, limit).Scan(&results).Error
	return results, err
}
