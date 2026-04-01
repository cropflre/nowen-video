package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// PulseRepo Pulse 数据中心仓储
type PulseRepo struct {
	db *gorm.DB
}

// ==================== 仪表盘概览 ====================

// DashboardOverview 仪表盘概览数据
type DashboardOverview struct {
	TotalMovies   int64   `json:"total_movies"`
	TotalSeries   int64   `json:"total_series"`
	TotalEpisodes int64   `json:"total_episodes"`
	TotalUsers    int64   `json:"total_users"`
	TotalStorageB int64   `json:"total_storage_bytes"`
	TotalPlayMin  float64 `json:"total_play_minutes"`
	TodayPlayMin  float64 `json:"today_play_minutes"`
	ActiveUsers7d int64   `json:"active_users_7d"`
}

// GetDashboardOverview 获取仪表盘概览数据
func (r *PulseRepo) GetDashboardOverview() (*DashboardOverview, error) {
	overview := &DashboardOverview{}

	// 电影总数
	r.db.Model(&model.Media{}).Where("media_type = ?", "movie").Count(&overview.TotalMovies)

	// 剧集合集总数
	r.db.Model(&model.Series{}).Count(&overview.TotalSeries)

	// 剧集单集总数
	r.db.Model(&model.Media{}).Where("media_type = ?", "episode").Count(&overview.TotalEpisodes)

	// 用户总数
	r.db.Model(&model.User{}).Count(&overview.TotalUsers)

	// 总存储空间（字节）
	r.db.Model(&model.Media{}).Select("COALESCE(SUM(file_size), 0)").Scan(&overview.TotalStorageB)

	// 总播放分钟数
	r.db.Model(&model.PlaybackStats{}).Select("COALESCE(SUM(watch_minutes), 0)").Scan(&overview.TotalPlayMin)

	// 今日播放分钟数
	today := time.Now().Format("2006-01-02")
	r.db.Model(&model.PlaybackStats{}).Where("date = ?", today).
		Select("COALESCE(SUM(watch_minutes), 0)").Scan(&overview.TodayPlayMin)

	// 最近7天活跃用户数
	weekAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	r.db.Model(&model.PlaybackStats{}).Where("date >= ?", weekAgo).
		Select("COUNT(DISTINCT user_id)").Scan(&overview.ActiveUsers7d)

	return overview, nil
}

// ==================== 播放趋势 ====================

// TrendPoint 趋势数据点
type TrendPoint struct {
	Date         string  `json:"date"`
	TotalMinutes float64 `json:"total_minutes"`
	PlayCount    int64   `json:"play_count"`
	UniqueUsers  int64   `json:"unique_users"`
}

// GetPlayTrends 获取播放趋势（最近N天）
func (r *PulseRepo) GetPlayTrends(days int) ([]TrendPoint, error) {
	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	var results []TrendPoint
	err := r.db.Model(&model.PlaybackStats{}).
		Select("date, COALESCE(SUM(watch_minutes), 0) as total_minutes, COUNT(*) as play_count, COUNT(DISTINCT user_id) as unique_users").
		Where("date >= ?", startDate).
		Group("date").Order("date ASC").
		Scan(&results).Error
	return results, err
}

// ==================== 热门内容排行 ====================

// TopContentItem 热门内容项
type TopContentItem struct {
	MediaID      string  `json:"media_id"`
	Title        string  `json:"title"`
	PosterPath   string  `json:"poster_path"`
	MediaType    string  `json:"media_type"`
	TotalMinutes float64 `json:"total_minutes"`
	PlayCount    int64   `json:"play_count"`
	UniqueUsers  int64   `json:"unique_users"`
}

// GetTopContent 获取热门内容排行
func (r *PulseRepo) GetTopContent(days, limit int) ([]TopContentItem, error) {
	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	var results []TopContentItem
	err := r.db.Raw(`
		SELECT ps.media_id, m.title, m.poster_path, m.media_type,
			COALESCE(SUM(ps.watch_minutes), 0) as total_minutes,
			COUNT(*) as play_count,
			COUNT(DISTINCT ps.user_id) as unique_users
		FROM playback_stats ps
		JOIN media m ON ps.media_id = m.id AND m.deleted_at IS NULL
		WHERE ps.date >= ?
		GROUP BY ps.media_id
		ORDER BY total_minutes DESC
		LIMIT ?
	`, startDate, limit).Scan(&results).Error
	return results, err
}

// ==================== 用户活跃排行 ====================

// TopUserItem 用户活跃排行项
type TopUserItem struct {
	UserID       string  `json:"user_id"`
	Username     string  `json:"username"`
	Avatar       string  `json:"avatar"`
	TotalMinutes float64 `json:"total_minutes"`
	PlayCount    int64   `json:"play_count"`
	MediaCount   int64   `json:"media_count"`
}

// GetTopUsers 获取用户活跃排行
func (r *PulseRepo) GetTopUsers(days, limit int) ([]TopUserItem, error) {
	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	var results []TopUserItem
	err := r.db.Raw(`
		SELECT ps.user_id, u.username, u.avatar,
			COALESCE(SUM(ps.watch_minutes), 0) as total_minutes,
			COUNT(*) as play_count,
			COUNT(DISTINCT ps.media_id) as media_count
		FROM playback_stats ps
		JOIN users u ON ps.user_id = u.id AND u.deleted_at IS NULL
		WHERE ps.date >= ?
		GROUP BY ps.user_id
		ORDER BY total_minutes DESC
		LIMIT ?
	`, startDate, limit).Scan(&results).Error
	return results, err
}

// ==================== 最近播放记录 ====================

// RecentPlayItem 最近播放记录项
type RecentPlayItem struct {
	UserID       string  `json:"user_id"`
	Username     string  `json:"username"`
	MediaID      string  `json:"media_id"`
	Title        string  `json:"title"`
	PosterPath   string  `json:"poster_path"`
	MediaType    string  `json:"media_type"`
	WatchMinutes float64 `json:"watch_minutes"`
	Date         string  `json:"date"`
	CreatedAt    string  `json:"created_at"`
}

// GetRecentPlays 获取最近播放记录
func (r *PulseRepo) GetRecentPlays(limit int) ([]RecentPlayItem, error) {
	var results []RecentPlayItem
	err := r.db.Raw(`
		SELECT ps.user_id, u.username, ps.media_id, m.title, m.poster_path, m.media_type,
			ps.watch_minutes, ps.date, ps.created_at
		FROM playback_stats ps
		JOIN users u ON ps.user_id = u.id AND u.deleted_at IS NULL
		JOIN media m ON ps.media_id = m.id AND m.deleted_at IS NULL
		ORDER BY ps.created_at DESC
		LIMIT ?
	`, limit).Scan(&results).Error
	return results, err
}

// ==================== 类型分布 ====================

// GenreDistItem 类型分布项
type GenreDistItem struct {
	Genre        string  `json:"genre"`
	Count        int64   `json:"count"`
	TotalMinutes float64 `json:"total_minutes"`
}

// GetGenreDistribution 获取媒体类型分布
func (r *PulseRepo) GetGenreDistribution() ([]GenreDistItem, error) {
	var results []GenreDistItem
	err := r.db.Raw(`
		SELECT m.genres as genre, COUNT(*) as count,
			COALESCE(SUM(ps.watch_minutes), 0) as total_minutes
		FROM media m
		LEFT JOIN playback_stats ps ON m.id = ps.media_id
		WHERE m.genres != '' AND m.deleted_at IS NULL
		GROUP BY m.genres
		ORDER BY count DESC
		LIMIT 20
	`).Scan(&results).Error
	return results, err
}

// ==================== 画质分布 ====================

// ResolutionDistItem 画质分布项
type ResolutionDistItem struct {
	Resolution string `json:"resolution"`
	Count      int64  `json:"count"`
}

// GetResolutionDistribution 获取画质分布
func (r *PulseRepo) GetResolutionDistribution() ([]ResolutionDistItem, error) {
	var results []ResolutionDistItem
	err := r.db.Raw(`
		SELECT CASE
			WHEN resolution = '' OR resolution IS NULL THEN '未知'
			ELSE resolution
		END as resolution, COUNT(*) as count
		FROM media
		WHERE deleted_at IS NULL
		GROUP BY resolution
		ORDER BY count DESC
	`).Scan(&results).Error
	return results, err
}

// ==================== 编码格式分布 ====================

// CodecDistItem 编码格式分布项
type CodecDistItem struct {
	Codec string `json:"codec"`
	Count int64  `json:"count"`
}

// GetCodecDistribution 获取编码格式分布
func (r *PulseRepo) GetCodecDistribution() ([]CodecDistItem, error) {
	var results []CodecDistItem
	err := r.db.Raw(`
		SELECT CASE
			WHEN video_codec = '' OR video_codec IS NULL THEN '未知'
			ELSE video_codec
		END as codec, COUNT(*) as count
		FROM media
		WHERE deleted_at IS NULL
		GROUP BY video_codec
		ORDER BY count DESC
	`).Scan(&results).Error
	return results, err
}

// ==================== 媒体库增长趋势 ====================

// GrowthPoint 增长趋势数据点
type GrowthPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// GetMediaGrowth 获取媒体库增长趋势（按月统计新增数量）
func (r *PulseRepo) GetMediaGrowth(months int) ([]GrowthPoint, error) {
	startDate := time.Now().AddDate(0, -months, 0).Format("2006-01-02")
	var results []GrowthPoint
	err := r.db.Raw(`
		SELECT strftime('%Y-%m', created_at) as date, COUNT(*) as count
		FROM media
		WHERE created_at >= ? AND deleted_at IS NULL
		GROUP BY strftime('%Y-%m', created_at)
		ORDER BY date ASC
	`, startDate).Scan(&results).Error
	return results, err
}

// ==================== 时段分布（全局） ====================

// HourlyDistItem 时段分布项
type HourlyDistItem struct {
	Hour      int     `json:"hour"`
	PlayCount int64   `json:"play_count"`
	TotalMin  float64 `json:"total_minutes"`
}

// GetHourlyDistribution 获取全局播放时段分布
func (r *PulseRepo) GetHourlyDistribution(days int) ([]HourlyDistItem, error) {
	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	var results []HourlyDistItem
	err := r.db.Raw(`
		SELECT CAST(strftime('%H', created_at) AS INTEGER) as hour,
			COUNT(*) as play_count,
			COALESCE(SUM(watch_minutes), 0) as total_minutes
		FROM playback_stats
		WHERE date >= ?
		GROUP BY hour
		ORDER BY hour ASC
	`, startDate).Scan(&results).Error
	return results, err
}

// ==================== 媒体库统计 ====================

// LibraryStatItem 媒体库统计项
type LibraryStatItem struct {
	LibraryID   string `json:"library_id"`
	LibraryName string `json:"library_name"`
	LibraryType string `json:"library_type"`
	MediaCount  int64  `json:"media_count"`
	SeriesCount int64  `json:"series_count"`
	TotalSizeB  int64  `json:"total_size_bytes"`
}

// GetLibraryStats 获取各媒体库统计
func (r *PulseRepo) GetLibraryStats() ([]LibraryStatItem, error) {
	var results []LibraryStatItem
	err := r.db.Raw(`
		SELECT l.id as library_id, l.name as library_name, l.type as library_type,
			COUNT(DISTINCT m.id) as media_count,
			COUNT(DISTINCT s.id) as series_count,
			COALESCE(SUM(m.file_size), 0) as total_size_b
		FROM libraries l
		LEFT JOIN media m ON l.id = m.library_id AND m.deleted_at IS NULL
		LEFT JOIN series s ON l.id = s.library_id AND s.deleted_at IS NULL
		WHERE l.deleted_at IS NULL
		GROUP BY l.id
		ORDER BY media_count DESC
	`).Scan(&results).Error
	return results, err
}
