package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

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
	err := r.db.Preload("Media").Preload("Media.Series").
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
	err := query.Preload("Media").Preload("Media.Series").
		Order("updated_at DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&histories).Error
	return histories, total, err
}

func (r *WatchHistoryRepo) GetByUserAndMedia(userID, mediaID string) (*model.WatchHistory, error) {
	var history model.WatchHistory
	err := r.db.Where("user_id = ? AND media_id = ?", userID, mediaID).First(&history).Error
	if err != nil {
		return nil, err
	}
	return &history, nil
}

func (r *WatchHistoryRepo) DeleteHistory(userID, mediaID string) error {
	return r.db.Where("user_id = ? AND media_id = ?", userID, mediaID).Delete(&model.WatchHistory{}).Error
}

func (r *WatchHistoryRepo) ClearHistory(userID string) error {
	return r.db.Where("user_id = ?", userID).Delete(&model.WatchHistory{}).Error
}

func (r *WatchHistoryRepo) DeleteByMediaID(mediaID string) error {
	return r.db.Where("media_id = ?", mediaID).Delete(&model.WatchHistory{}).Error
}

func (r *WatchHistoryRepo) DeleteByLibraryMediaIDs(libraryID string) error {
	return r.db.Where("media_id IN (SELECT id FROM media WHERE library_id = ?)", libraryID).Delete(&model.WatchHistory{}).Error
}

func (r *WatchHistoryRepo) CleanOrphaned() (int64, error) {
	result := r.db.Where("media_id NOT IN (SELECT id FROM media WHERE deleted_at IS NULL)").Delete(&model.WatchHistory{})
	return result.RowsAffected, result.Error
}

func (r *WatchHistoryRepo) GetAllByUserID(userID string) ([]model.WatchHistory, error) {
	var histories []model.WatchHistory
	err := r.db.Where("user_id = ?", userID).Find(&histories).Error
	return histories, err
}

func (r *WatchHistoryRepo) GetAllHistory(maxRecords int) ([]model.WatchHistory, error) {
	if maxRecords <= 0 {
		maxRecords = 10000
	}
	var histories []model.WatchHistory
	err := r.db.Order("updated_at DESC").Limit(maxRecords).Find(&histories).Error
	return histories, err
}

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

func (r *WatchHistoryRepo) GetHistoryByUserIDs(userIDs []string) ([]model.WatchHistory, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var histories []model.WatchHistory
	err := r.db.Where("user_id IN ?", userIDs).Find(&histories).Error
	return histories, err
}

// GetLatestByMediaID 获取指定媒体的最新观看记录（不限用户）
func (r *WatchHistoryRepo) GetLatestByMediaID(mediaID string) (*model.WatchHistory, error) {
	var history model.WatchHistory
	err := r.db.Where("media_id = ?", mediaID).Order("updated_at DESC").First(&history).Error
	if err != nil {
		return nil, err
	}
	return &history, nil
}

// PopularMedia 热门媒体统计结果
type PopularMedia struct {
	MediaID    string
	WatchCount int
}

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

	query := r.db.Model(&model.Favorite{}).
		Joins("JOIN media ON media.id = favorites.media_id AND media.deleted_at IS NULL").
		Where("favorites.user_id = ?", userID)
	query.Count(&total)
	err := query.Preload("Media").Order("favorites.created_at DESC").Offset((page - 1) * size).Limit(size).Find(&favs).Error
	return favs, total, err
}

func (r *FavoriteRepo) Exists(userID, mediaID string) bool {
	var count int64
	r.db.Model(&model.Favorite{}).Where("user_id = ? AND media_id = ?", userID, mediaID).Count(&count)
	return count > 0
}

func (r *FavoriteRepo) DeleteByMediaID(mediaID string) error {
	return r.db.Where("media_id = ?", mediaID).Delete(&model.Favorite{}).Error
}

func (r *FavoriteRepo) DeleteByLibraryMediaIDs(libraryID string) error {
	return r.db.Where("media_id IN (SELECT id FROM media WHERE library_id = ?)", libraryID).Delete(&model.Favorite{}).Error
}

func (r *FavoriteRepo) CleanOrphaned() (int64, error) {
	result := r.db.Where("media_id NOT IN (SELECT id FROM media WHERE deleted_at IS NULL)").Delete(&model.Favorite{})
	return result.RowsAffected, result.Error
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
