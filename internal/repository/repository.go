package repository

import (
	"fmt"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// Repositories 聚合所有数据仓储
type Repositories struct {
	User           *UserRepo
	Library        *LibraryRepo
	Media          *MediaRepo
	Series         *SeriesRepo
	WatchHistory   *WatchHistoryRepo
	Favorite       *FavoriteRepo
	Transcode      *TranscodeRepo
	Playlist       *PlaylistRepo
	Bookmark       *BookmarkRepo
	Comment        *CommentRepo
	AccessLog      *AccessLogRepo
	ScheduledTask  *ScheduledTaskRepo
	ContentRating  *ContentRatingRepo
	UserPermission *UserPermissionRepo
	SystemSetting  *SystemSettingRepo
}

func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		User:           &UserRepo{db: db},
		Library:        &LibraryRepo{db: db},
		Media:          &MediaRepo{db: db},
		Series:         &SeriesRepo{db: db},
		WatchHistory:   &WatchHistoryRepo{db: db},
		Favorite:       &FavoriteRepo{db: db},
		Transcode:      &TranscodeRepo{db: db},
		Playlist:       &PlaylistRepo{db: db},
		Bookmark:       &BookmarkRepo{db: db},
		Comment:        &CommentRepo{db: db},
		AccessLog:      &AccessLogRepo{db: db},
		ScheduledTask:  &ScheduledTaskRepo{db: db},
		ContentRating:  &ContentRatingRepo{db: db},
		UserPermission: &UserPermissionRepo{db: db},
		SystemSetting:  &SystemSettingRepo{db: db},
	}
}

// ==================== UserRepo ====================

type UserRepo struct {
	db *gorm.DB
}

func (r *UserRepo) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepo) FindByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ?", username).First(&user).Error
	return &user, err
}

func (r *UserRepo) FindByID(id string) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, "id = ?", id).Error
	return &user, err
}

func (r *UserRepo) List() ([]model.User, error) {
	var users []model.User
	err := r.db.Find(&users).Error
	return users, err
}

func (r *UserRepo) Count() (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).Count(&count).Error
	return count, err
}

func (r *UserRepo) Delete(id string) error {
	return r.db.Delete(&model.User{}, "id = ?", id).Error
}

// ==================== LibraryRepo ====================

type LibraryRepo struct {
	db *gorm.DB
}

func (r *LibraryRepo) Create(lib *model.Library) error {
	return r.db.Create(lib).Error
}

func (r *LibraryRepo) FindByID(id string) (*model.Library, error) {
	var lib model.Library
	err := r.db.First(&lib, "id = ?", id).Error
	return &lib, err
}

func (r *LibraryRepo) List() ([]model.Library, error) {
	var libs []model.Library
	err := r.db.Find(&libs).Error
	return libs, err
}

func (r *LibraryRepo) Update(lib *model.Library) error {
	return r.db.Save(lib).Error
}

func (r *LibraryRepo) Delete(id string) error {
	return r.db.Delete(&model.Library{}, "id = ?", id).Error
}

// ==================== MediaRepo ====================

type MediaRepo struct {
	db *gorm.DB
}

func (r *MediaRepo) Create(media *model.Media) error {
	return r.db.Create(media).Error
}

func (r *MediaRepo) FindByID(id string) (*model.Media, error) {
	var media model.Media
	err := r.db.First(&media, "id = ?", id).Error
	return &media, err
}

func (r *MediaRepo) FindByFilePath(filePath string) (*model.Media, error) {
	var media model.Media
	err := r.db.Where("file_path = ?", filePath).First(&media).Error
	return &media, err
}

func (r *MediaRepo) List(page, size int, libraryID string) ([]model.Media, int64, error) {
	var media []model.Media
	var total int64

	query := r.db.Model(&model.Media{})
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}

	query.Count(&total)
	err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&media).Error
	return media, total, err
}

func (r *MediaRepo) Recent(limit int) ([]model.Media, error) {
	var media []model.Media
	err := r.db.Order("created_at DESC").Limit(limit).Find(&media).Error
	return media, err
}

func (r *MediaRepo) Search(keyword string, page, size int) ([]model.Media, int64, error) {
	var media []model.Media
	var total int64

	query := r.db.Model(&model.Media{}).Where("title LIKE ?", "%"+keyword+"%")
	query.Count(&total)
	err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&media).Error
	return media, total, err
}

func (r *MediaRepo) DeleteByLibraryID(libraryID string) error {
	// 使用 Unscoped 硬删除，彻底清除关联数据，避免软删除导致的孤立数据
	return r.db.Unscoped().Where("library_id = ?", libraryID).Delete(&model.Media{}).Error
}

// CleanOrphanedByLibraryIDs 清理孤立的媒体数据（library_id 不在有效列表中的记录）
func (r *MediaRepo) CleanOrphanedByLibraryIDs(validLibraryIDs []string) (int64, error) {
	var result *gorm.DB
	if len(validLibraryIDs) == 0 {
		// 没有任何有效媒体库，删除所有媒体
		result = r.db.Unscoped().Where("1 = 1").Delete(&model.Media{})
	} else {
		result = r.db.Unscoped().Where("library_id NOT IN ?", validLibraryIDs).Delete(&model.Media{})
	}
	return result.RowsAffected, result.Error
}

func (r *MediaRepo) Update(media *model.Media) error {
	return r.db.Save(media).Error
}

// FindByIDs 批量查询媒体（避免 N+1 查询）
func (r *MediaRepo) FindByIDs(ids []string) ([]model.Media, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var media []model.Media
	err := r.db.Where("id IN ?", ids).Find(&media).Error
	return media, err
}

// ListByGenres 根据类型模糊查询媒体（用于基于内容的推荐，避免全量加载）
func (r *MediaRepo) ListByGenres(genres []string, excludeIDs []string, limit int) ([]model.Media, error) {
	if len(genres) == 0 {
		return nil, nil
	}
	query := r.db.Model(&model.Media{})
	// 使用 OR 条件匹配任意一个类型
	for i, genre := range genres {
		if i == 0 {
			query = query.Where("genres LIKE ?", "%"+genre+"%")
		} else {
			query = query.Or("genres LIKE ?", "%"+genre+"%")
		}
	}
	if len(excludeIDs) > 0 {
		query = query.Where("id NOT IN ?", excludeIDs)
	}
	var media []model.Media
	err := query.Order("rating DESC").Limit(limit).Find(&media).Error
	return media, err
}

func (r *MediaRepo) ListByLibraryID(libraryID string) ([]model.Media, error) {
	var media []model.Media
	err := r.db.Where("library_id = ?", libraryID).Find(&media).Error
	return media, err
}

// ListBySeriesID 获取指定剧集合集的所有剧集
func (r *MediaRepo) ListBySeriesID(seriesID string) ([]model.Media, error) {
	var media []model.Media
	err := r.db.Where("series_id = ?", seriesID).
		Order("season_num ASC, episode_num ASC").Find(&media).Error
	return media, err
}

// ListBySeriesAndSeason 获取指定合集指定季的剧集
func (r *MediaRepo) ListBySeriesAndSeason(seriesID string, seasonNum int) ([]model.Media, error) {
	var media []model.Media
	err := r.db.Where("series_id = ? AND season_num = ?", seriesID, seasonNum).
		Order("episode_num ASC").Find(&media).Error
	return media, err
}

// RecentNonEpisode 获取最近添加的非剧集媒体（不包含已归入合集的剧集）
func (r *MediaRepo) RecentNonEpisode(limit int) ([]model.Media, error) {
	var media []model.Media
	err := r.db.Where("(series_id = '' OR series_id IS NULL) AND library_id != ''").
		Order("created_at DESC").Limit(limit).Find(&media).Error
	return media, err
}

// RecentNonEpisodeAll 获取所有非剧集媒体（可选按媒体库过滤，用于混合列表）
func (r *MediaRepo) RecentNonEpisodeAll(libraryID string) ([]model.Media, error) {
	var media []model.Media
	query := r.db.Where("(series_id = '' OR series_id IS NULL) AND library_id != ''")
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	err := query.Order("created_at DESC").Find(&media).Error
	return media, err
}

// ListNonEpisode 获取非剧集媒体列表（分页，不包含已归入合集的剧集）
func (r *MediaRepo) ListNonEpisode(page, size int, libraryID string) ([]model.Media, int64, error) {
	var media []model.Media
	var total int64

	query := r.db.Model(&model.Media{}).Where("(series_id = '' OR series_id IS NULL) AND library_id != ''")
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}

	query.Count(&total)
	err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&media).Error
	return media, total, err
}

// CleanGhostMedia 清理幽灵 Media 记录（library_id 为空的无主记录，通常由豆瓣刮削 Series 时误创建）
func (r *MediaRepo) CleanGhostMedia() (int64, error) {
	result := r.db.Unscoped().Where("library_id = '' OR library_id IS NULL").Delete(&model.Media{})
	return result.RowsAffected, result.Error
}

// CountByLibrary 统计指定媒体库中非剧集媒体的数量
func (r *MediaRepo) CountNonEpisodeByLibrary(libraryID string) (int64, error) {
	var count int64
	query := r.db.Model(&model.Media{}).Where("(series_id = '' OR series_id IS NULL) AND library_id != ''")
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	err := query.Count(&count).Error
	return count, err
}

// CountNonEpisode 统计非剧集媒体的总数（可选按媒体库过滤）
func (r *MediaRepo) CountNonEpisode(libraryID string) (int64, error) {
	var count int64
	query := r.db.Model(&model.Media{}).Where("(series_id = '' OR series_id IS NULL) AND library_id != ''")
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	err := query.Count(&count).Error
	return count, err
}

// ==================== SeriesRepo ====================

type SeriesRepo struct {
	db *gorm.DB
}

func (r *SeriesRepo) Create(series *model.Series) error {
	return r.db.Create(series).Error
}

func (r *SeriesRepo) FindByID(id string) (*model.Series, error) {
	var series model.Series
	err := r.db.Preload("Episodes", func(db *gorm.DB) *gorm.DB {
		return db.Order("season_num ASC, episode_num ASC")
	}).First(&series, "id = ?", id).Error
	return &series, err
}

func (r *SeriesRepo) FindByFolderPath(folderPath string) (*model.Series, error) {
	var series model.Series
	err := r.db.Where("folder_path = ?", folderPath).First(&series).Error
	return &series, err
}

func (r *SeriesRepo) ListByLibraryID(libraryID string) ([]model.Series, error) {
	var series []model.Series
	err := r.db.Where("library_id = ?", libraryID).Order("title ASC").Find(&series).Error
	return series, err
}

func (r *SeriesRepo) List(page, size int, libraryID string) ([]model.Series, int64, error) {
	var series []model.Series
	var total int64

	query := r.db.Model(&model.Series{})
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}

	query.Count(&total)
	err := query.Order("title ASC").Offset((page - 1) * size).Limit(size).Find(&series).Error
	return series, total, err
}

// CountByLibrary 统计指定媒体库中合集的数量（可选按媒体库过滤）
func (r *SeriesRepo) CountByLibrary(libraryID string) (int64, error) {
	var count int64
	query := r.db.Model(&model.Series{})
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	err := query.Count(&count).Error
	return count, err
}

// ListAll 获取指定媒体库中的所有合集（不分页）
func (r *SeriesRepo) ListAll(libraryID string) ([]model.Series, error) {
	var series []model.Series
	query := r.db
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	err := query.Order("created_at DESC").Find(&series).Error
	return series, err
}

func (r *SeriesRepo) Update(series *model.Series) error {
	return r.db.Save(series).Error
}

func (r *SeriesRepo) Delete(id string) error {
	return r.db.Delete(&model.Series{}, "id = ?", id).Error
}

func (r *SeriesRepo) DeleteByLibraryID(libraryID string) error {
	// 使用 Unscoped 硬删除，彻底清除关联数据
	return r.db.Unscoped().Where("library_id = ?", libraryID).Delete(&model.Series{}).Error
}

// CleanOrphanedByLibraryIDs 清理孤立的剧集合集数据
func (r *SeriesRepo) CleanOrphanedByLibraryIDs(validLibraryIDs []string) (int64, error) {
	var result *gorm.DB
	if len(validLibraryIDs) == 0 {
		result = r.db.Unscoped().Where("1 = 1").Delete(&model.Series{})
	} else {
		result = r.db.Unscoped().Where("library_id NOT IN ?", validLibraryIDs).Delete(&model.Series{})
	}
	return result.RowsAffected, result.Error
}

// GetSeasonNumbers 获取指定合集的所有季号
func (r *SeriesRepo) GetSeasonNumbers(seriesID string) ([]int, error) {
	var seasons []int
	err := r.db.Model(&model.Media{}).
		Where("series_id = ?", seriesID).
		Distinct("season_num").
		Order("season_num ASC").
		Pluck("season_num", &seasons).Error
	return seasons, err
}

// RecentUpdated 获取最近有新剧集添加的合集列表
func (r *SeriesRepo) RecentUpdated(limit int) ([]model.Series, error) {
	var series []model.Series
	err := r.db.Order("updated_at DESC").Limit(limit).Find(&series).Error
	return series, err
}

// RecentUpdatedByLibrary 获取指定媒体库中最近更新的合集
func (r *SeriesRepo) RecentUpdatedByLibrary(libraryID string, limit int) ([]model.Series, error) {
	var series []model.Series
	err := r.db.Where("library_id = ?", libraryID).
		Order("updated_at DESC").Limit(limit).Find(&series).Error
	return series, err
}

// ==================== WatchHistoryRepo ====================

type WatchHistoryRepo struct {
	db *gorm.DB
}

func (r *WatchHistoryRepo) Upsert(history *model.WatchHistory) error {
	var existing model.WatchHistory
	err := r.db.Where("user_id = ? AND media_id = ?", history.UserID, history.MediaID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(history).Error
	}
	existing.Position = history.Position
	existing.Duration = history.Duration
	existing.Completed = history.Completed
	return r.db.Save(&existing).Error
}

func (r *WatchHistoryRepo) ContinueWatching(userID string, limit int) ([]model.WatchHistory, error) {
	var histories []model.WatchHistory
	err := r.db.Preload("Media").
		Where("user_id = ? AND completed = ?", userID, false).
		Order("updated_at DESC").
		Limit(limit).
		Find(&histories).Error
	return histories, err
}

func (r *WatchHistoryRepo) ListHistory(userID string, page, size int) ([]model.WatchHistory, int64, error) {
	var histories []model.WatchHistory
	var total int64

	query := r.db.Model(&model.WatchHistory{}).Where("user_id = ?", userID)
	query.Count(&total)
	err := query.Preload("Media").
		Order("updated_at DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&histories).Error
	return histories, total, err
}

func (r *WatchHistoryRepo) DeleteHistory(userID, mediaID string) error {
	return r.db.Where("user_id = ? AND media_id = ?", userID, mediaID).Delete(&model.WatchHistory{}).Error
}

func (r *WatchHistoryRepo) ClearHistory(userID string) error {
	return r.db.Where("user_id = ?", userID).Delete(&model.WatchHistory{}).Error
}

// GetAllByUserID 获取指定用户的所有观看记录（用于推荐系统）
func (r *WatchHistoryRepo) GetAllByUserID(userID string) ([]model.WatchHistory, error) {
	var histories []model.WatchHistory
	err := r.db.Where("user_id = ?", userID).Find(&histories).Error
	return histories, err
}

// GetAllHistory 获取所有用户的观看记录（用于协同过滤，限制最大数量避免内存溢出）
func (r *WatchHistoryRepo) GetAllHistory(maxRecords int) ([]model.WatchHistory, error) {
	if maxRecords <= 0 {
		maxRecords = 10000 // 默认上限 1万条
	}
	var histories []model.WatchHistory
	err := r.db.Order("updated_at DESC").Limit(maxRecords).Find(&histories).Error
	return histories, err
}

// GetActiveUserIDs 获取最近有观看行为的用户ID列表（用于协同过滤采样）
func (r *WatchHistoryRepo) GetActiveUserIDs(limit int) ([]string, error) {
	var userIDs []string
	err := r.db.Model(&model.WatchHistory{}).
		Select("DISTINCT user_id").
		Order("MAX(updated_at) DESC").
		Group("user_id").
		Limit(limit).
		Pluck("user_id", &userIDs).Error
	return userIDs, err
}

// GetHistoryByUserIDs 获取指定用户列表的观看记录（协同过滤用）
func (r *WatchHistoryRepo) GetHistoryByUserIDs(userIDs []string) ([]model.WatchHistory, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var histories []model.WatchHistory
	err := r.db.Where("user_id IN ?", userIDs).Find(&histories).Error
	return histories, err
}

// PopularMedia 热门媒体统计结果
type PopularMedia struct {
	MediaID    string
	WatchCount int
}

// GetMostWatched 获取最多人观看的媒体（用于热门推荐）
func (r *WatchHistoryRepo) GetMostWatched(limit int) ([]PopularMedia, error) {
	var results []PopularMedia
	err := r.db.Model(&model.WatchHistory{}).
		Select("media_id, COUNT(DISTINCT user_id) as watch_count").
		Group("media_id").
		Order("watch_count DESC").
		Limit(limit).
		Scan(&results).Error
	return results, err
}

// ==================== FavoriteRepo ====================

type FavoriteRepo struct {
	db *gorm.DB
}

func (r *FavoriteRepo) Add(fav *model.Favorite) error {
	return r.db.Create(fav).Error
}

func (r *FavoriteRepo) Remove(userID, mediaID string) error {
	return r.db.Where("user_id = ? AND media_id = ?", userID, mediaID).Delete(&model.Favorite{}).Error
}

func (r *FavoriteRepo) List(userID string, page, size int) ([]model.Favorite, int64, error) {
	var favs []model.Favorite
	var total int64

	query := r.db.Model(&model.Favorite{}).Where("user_id = ?", userID)
	query.Count(&total)
	err := query.Preload("Media").Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&favs).Error
	return favs, total, err
}

func (r *FavoriteRepo) Exists(userID, mediaID string) bool {
	var count int64
	r.db.Model(&model.Favorite{}).Where("user_id = ? AND media_id = ?", userID, mediaID).Count(&count)
	return count > 0
}

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

// ==================== PlaylistRepo ====================

type PlaylistRepo struct {
	db *gorm.DB
}

func (r *PlaylistRepo) Create(playlist *model.Playlist) error {
	return r.db.Create(playlist).Error
}

func (r *PlaylistRepo) FindByID(id string) (*model.Playlist, error) {
	var playlist model.Playlist
	err := r.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC").Preload("Media")
	}).First(&playlist, "id = ?", id).Error
	return &playlist, err
}

func (r *PlaylistRepo) ListByUserID(userID string) ([]model.Playlist, error) {
	var playlists []model.Playlist
	err := r.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC").Preload("Media")
	}).Where("user_id = ?", userID).Order("created_at DESC").Find(&playlists).Error
	return playlists, err
}

func (r *PlaylistRepo) Update(playlist *model.Playlist) error {
	return r.db.Save(playlist).Error
}

func (r *PlaylistRepo) Delete(id string) error {
	// 先删除播放列表项
	r.db.Where("playlist_id = ?", id).Delete(&model.PlaylistItem{})
	return r.db.Delete(&model.Playlist{}, "id = ?", id).Error
}

func (r *PlaylistRepo) AddItem(item *model.PlaylistItem) error {
	return r.db.Create(item).Error
}

func (r *PlaylistRepo) RemoveItem(playlistID, mediaID string) error {
	return r.db.Where("playlist_id = ? AND media_id = ?", playlistID, mediaID).Delete(&model.PlaylistItem{}).Error
}

func (r *PlaylistRepo) GetMaxSortOrder(playlistID string) int {
	var maxOrder int
	r.db.Model(&model.PlaylistItem{}).Where("playlist_id = ?", playlistID).Select("COALESCE(MAX(sort_order), 0)").Scan(&maxOrder)
	return maxOrder
}

// ==================== BookmarkRepo ====================

type BookmarkRepo struct {
	db *gorm.DB
}

func (r *BookmarkRepo) Create(bookmark *model.Bookmark) error {
	return r.db.Create(bookmark).Error
}

func (r *BookmarkRepo) FindByID(id string) (*model.Bookmark, error) {
	var bookmark model.Bookmark
	err := r.db.First(&bookmark, "id = ?", id).Error
	return &bookmark, err
}

func (r *BookmarkRepo) ListByUserAndMedia(userID, mediaID string) ([]model.Bookmark, error) {
	var bookmarks []model.Bookmark
	err := r.db.Where("user_id = ? AND media_id = ?", userID, mediaID).
		Order("position ASC").Find(&bookmarks).Error
	return bookmarks, err
}

func (r *BookmarkRepo) ListByUser(userID string, page, size int) ([]model.Bookmark, int64, error) {
	var bookmarks []model.Bookmark
	var total int64
	query := r.db.Model(&model.Bookmark{}).Where("user_id = ?", userID)
	query.Count(&total)
	err := query.Preload("Media").Order("created_at DESC").
		Offset((page - 1) * size).Limit(size).Find(&bookmarks).Error
	return bookmarks, total, err
}

func (r *BookmarkRepo) Delete(id string) error {
	return r.db.Delete(&model.Bookmark{}, "id = ?", id).Error
}

func (r *BookmarkRepo) Update(bookmark *model.Bookmark) error {
	return r.db.Save(bookmark).Error
}

// ==================== CommentRepo ====================

type CommentRepo struct {
	db *gorm.DB
}

func (r *CommentRepo) Create(comment *model.Comment) error {
	return r.db.Create(comment).Error
}

func (r *CommentRepo) FindByID(id string) (*model.Comment, error) {
	var comment model.Comment
	err := r.db.Preload("User").First(&comment, "id = ?", id).Error
	return &comment, err
}

func (r *CommentRepo) ListByMedia(mediaID string, page, size int) ([]model.Comment, int64, error) {
	var comments []model.Comment
	var total int64
	query := r.db.Model(&model.Comment{}).Where("media_id = ?", mediaID)
	query.Count(&total)
	err := query.Preload("User").Order("created_at DESC").
		Offset((page - 1) * size).Limit(size).Find(&comments).Error
	return comments, total, err
}

func (r *CommentRepo) Delete(id string) error {
	return r.db.Delete(&model.Comment{}, "id = ?", id).Error
}

func (r *CommentRepo) Update(comment *model.Comment) error {
	return r.db.Save(comment).Error
}

func (r *CommentRepo) GetAverageRating(mediaID string) (float64, int64, error) {
	var result struct {
		Avg   float64
		Count int64
	}
	err := r.db.Model(&model.Comment{}).Where("media_id = ? AND rating > 0", mediaID).
		Select("COALESCE(AVG(rating), 0) as avg, COUNT(*) as count").Scan(&result).Error
	return result.Avg, result.Count, err
}

// ==================== AccessLogRepo ====================

type AccessLogRepo struct {
	db *gorm.DB
}

func (r *AccessLogRepo) Create(log *model.AccessLog) error {
	return r.db.Create(log).Error
}

func (r *AccessLogRepo) List(page, size int, userID, action string) ([]model.AccessLog, int64, error) {
	var logs []model.AccessLog
	var total int64
	query := r.db.Model(&model.AccessLog{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}
	query.Count(&total)
	err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&logs).Error
	return logs, total, err
}

func (r *AccessLogRepo) CleanOlderThan(days int) error {
	return r.db.Where("created_at < datetime('now', ?)", fmt.Sprintf("-%d days", days)).
		Delete(&model.AccessLog{}).Error
}

// ==================== ScheduledTaskRepo ====================

type ScheduledTaskRepo struct {
	db *gorm.DB
}

func (r *ScheduledTaskRepo) Create(task *model.ScheduledTask) error {
	return r.db.Create(task).Error
}

func (r *ScheduledTaskRepo) FindByID(id string) (*model.ScheduledTask, error) {
	var task model.ScheduledTask
	err := r.db.First(&task, "id = ?", id).Error
	return &task, err
}

func (r *ScheduledTaskRepo) List() ([]model.ScheduledTask, error) {
	var tasks []model.ScheduledTask
	err := r.db.Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

func (r *ScheduledTaskRepo) Update(task *model.ScheduledTask) error {
	return r.db.Save(task).Error
}

func (r *ScheduledTaskRepo) Delete(id string) error {
	return r.db.Delete(&model.ScheduledTask{}, "id = ?", id).Error
}

func (r *ScheduledTaskRepo) ListEnabled() ([]model.ScheduledTask, error) {
	var tasks []model.ScheduledTask
	err := r.db.Where("enabled = ?", true).Find(&tasks).Error
	return tasks, err
}

// ==================== ContentRatingRepo ====================

type ContentRatingRepo struct {
	db *gorm.DB
}

func (r *ContentRatingRepo) Upsert(rating *model.ContentRating) error {
	var existing model.ContentRating
	err := r.db.Where("media_id = ?", rating.MediaID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(rating).Error
	}
	existing.Level = rating.Level
	return r.db.Save(&existing).Error
}

func (r *ContentRatingRepo) FindByMediaID(mediaID string) (*model.ContentRating, error) {
	var rating model.ContentRating
	err := r.db.Where("media_id = ?", mediaID).First(&rating).Error
	return &rating, err
}

func (r *ContentRatingRepo) Delete(mediaID string) error {
	return r.db.Where("media_id = ?", mediaID).Delete(&model.ContentRating{}).Error
}

// ==================== UserPermissionRepo ====================

type UserPermissionRepo struct {
	db *gorm.DB
}

func (r *UserPermissionRepo) Upsert(perm *model.UserPermission) error {
	var existing model.UserPermission
	err := r.db.Where("user_id = ?", perm.UserID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(perm).Error
	}
	existing.AllowedLibraries = perm.AllowedLibraries
	existing.MaxRatingLevel = perm.MaxRatingLevel
	existing.DailyTimeLimit = perm.DailyTimeLimit
	return r.db.Save(&existing).Error
}

func (r *UserPermissionRepo) FindByUserID(userID string) (*model.UserPermission, error) {
	var perm model.UserPermission
	err := r.db.Where("user_id = ?", userID).First(&perm).Error
	return &perm, err
}

func (r *UserPermissionRepo) Delete(userID string) error {
	return r.db.Where("user_id = ?", userID).Delete(&model.UserPermission{}).Error
}

// ==================== SystemSettingRepo ====================

type SystemSettingRepo struct {
	db *gorm.DB
}

// Get 获取单个设置值
func (r *SystemSettingRepo) Get(key string) (string, error) {
	var setting model.SystemSetting
	err := r.db.Where("`key` = ?", key).First(&setting).Error
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

// Set 设置（Upsert）单个键值对
func (r *SystemSettingRepo) Set(key, value string) error {
	var existing model.SystemSetting
	err := r.db.Where("`key` = ?", key).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(&model.SystemSetting{Key: key, Value: value}).Error
	}
	if err != nil {
		return err
	}
	existing.Value = value
	return r.db.Save(&existing).Error
}

// GetAll 获取所有系统设置
func (r *SystemSettingRepo) GetAll() (map[string]string, error) {
	var settings []model.SystemSetting
	err := r.db.Find(&settings).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

// SetMulti 批量设置多个键值对
func (r *SystemSettingRepo) SetMulti(kvs map[string]string) error {
	for key, value := range kvs {
		if err := r.Set(key, value); err != nil {
			return err
		}
	}
	return nil
}
