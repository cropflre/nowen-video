package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== TranscodeRepo ====================

// TranscodeRepo is now only the read-only migration gateway for an existing
// transcode_tasks table. New databases never create that table and no runtime
// component may write, update or delete its rows.
type TranscodeRepo struct {
	db *gorm.DB
}

func (r *TranscodeRepo) LegacyTableExists() bool {
	return r != nil && r.db != nil && r.db.Migrator().HasTable(&model.TranscodeTask{})
}

type LegacyProjectionCursor struct {
	UpdatedAt time.Time
	ID        string
}

func LegacyProjectionCursorAfter(left, right LegacyProjectionCursor) bool {
	if left.UpdatedAt.Equal(right.UpdatedAt) {
		return left.ID > right.ID
	}
	return left.UpdatedAt.After(right.UpdatedAt)
}

func (r *TranscodeRepo) legacyTerminalWithOutputQuery() *gorm.DB {
	return r.db.Model(&model.TranscodeTask{}).Where(
		"status IN ? AND TRIM(COALESCE(output_dir, '')) <> ''",
		[]string{"done", "completed", "failed", "cancelled"},
	)
}

func (r *TranscodeRepo) LegacyProjectionHighWater() (*LegacyProjectionCursor, error) {
	if !r.LegacyTableExists() {
		return nil, nil
	}
	var row struct {
		UpdatedAt time.Time
		ID        string
	}
	result := r.legacyTerminalWithOutputQuery().
		Select("updated_at", "id").
		Order("updated_at DESC, id DESC").
		Limit(1).
		Find(&row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &LegacyProjectionCursor{UpdatedAt: row.UpdatedAt, ID: row.ID}, nil
}

func (r *TranscodeRepo) CountLegacyTerminalWithOutputThrough(highWater LegacyProjectionCursor) (int64, error) {
	if !r.LegacyTableExists() {
		return 0, nil
	}
	var count int64
	err := r.legacyTerminalWithOutputQuery().
		Where("updated_at < ? OR (updated_at = ? AND id <= ?)", highWater.UpdatedAt, highWater.UpdatedAt, highWater.ID).
		Count(&count).Error
	return count, err
}

func (r *TranscodeRepo) ListLegacyTerminalWithOutputAfter(after *LegacyProjectionCursor, highWater LegacyProjectionCursor, limit int) ([]model.TranscodeTask, error) {
	if !r.LegacyTableExists() {
		return []model.TranscodeTask{}, nil
	}
	if limit <= 0 {
		limit = 250
	}
	if limit > 2000 {
		limit = 2000
	}
	query := r.legacyTerminalWithOutputQuery().
		Where("updated_at < ? OR (updated_at = ? AND id <= ?)", highWater.UpdatedAt, highWater.UpdatedAt, highWater.ID)
	if after != nil {
		query = query.Where("updated_at > ? OR (updated_at = ? AND id > ?)", after.UpdatedAt, after.UpdatedAt, after.ID)
	}
	var tasks []model.TranscodeTask
	err := query.Order("updated_at ASC, id ASC").Limit(limit).Find(&tasks).Error
	return tasks, err
}

// ListLegacyTerminalWithOutput remains as a bounded compatibility reader for
// diagnostics. Migration code uses the explicit cursor/high-water API above.
func (r *TranscodeRepo) ListLegacyTerminalWithOutput(limit int) ([]model.TranscodeTask, error) {
	highWater, err := r.LegacyProjectionHighWater()
	if err != nil || highWater == nil {
		return []model.TranscodeTask{}, err
	}
	return r.ListLegacyTerminalWithOutputAfter(nil, *highWater, limit)
}

// ==================== PlaybackStatsRepo ====================

type PlaybackStatsRepo struct {
	db *gorm.DB
}

func (r *PlaybackStatsRepo) Record(stat *model.PlaybackStats) error {
	return r.db.Create(stat).Error
}

func (r *PlaybackStatsRepo) DeleteByMediaIDs(mediaIDs []string) (int64, error) {
	if len(mediaIDs) == 0 {
		return 0, nil
	}
	result := r.db.Where("media_id IN ?", mediaIDs).Delete(&model.PlaybackStats{})
	return result.RowsAffected, result.Error
}

// CleanOrphaned 删除媒体主体已不存在的播放统计。
func (r *PlaybackStatsRepo) CleanOrphaned() (int64, error) {
	result := r.db.Where("media_id NOT IN (SELECT id FROM media WHERE deleted_at IS NULL)").
		Delete(&model.PlaybackStats{})
	return result.RowsAffected, result.Error
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
