package service

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// MediaService 媒体服务
type MediaService struct {
	mediaRepo   *repository.MediaRepo
	seriesRepo  *repository.SeriesRepo
	historyRepo *repository.WatchHistoryRepo
	favRepo     *repository.FavoriteRepo
	logger      *zap.SugaredLogger
}

func NewMediaService(
	mediaRepo *repository.MediaRepo,
	seriesRepo *repository.SeriesRepo,
	historyRepo *repository.WatchHistoryRepo,
	favRepo *repository.FavoriteRepo,
	logger *zap.SugaredLogger,
) *MediaService {
	return &MediaService{
		mediaRepo:   mediaRepo,
		seriesRepo:  seriesRepo,
		historyRepo: historyRepo,
		favRepo:     favRepo,
		logger:      logger,
	}
}

// ListMedia 获取媒体列表
func (s *MediaService) ListMedia(page, size int, libraryID string) ([]model.Media, int64, error) {
	return s.mediaRepo.List(page, size, libraryID)
}

// GetDetail 获取媒体详情
func (s *MediaService) GetDetail(id string) (*model.Media, error) {
	return s.mediaRepo.FindByID(id)
}

// Recent 最近添加
func (s *MediaService) Recent(limit int) ([]model.Media, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.mediaRepo.Recent(limit)
}

// RecentAggregated 最近添加（聚合模式：合集内剧集聚合为合集，独立媒体直接展示）
func (s *MediaService) RecentAggregated(limit int) ([]model.Media, []model.Series, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	// 获取最近添加的独立媒体（不属于任何合集的）
	independentMedia, err := s.mediaRepo.RecentNonEpisode(limit)
	if err != nil {
		return nil, nil, err
	}
	// 获取最近有更新的合集
	recentSeries, err := s.seriesRepo.RecentUpdated(limit)
	if err != nil {
		return nil, nil, err
	}
	return independentMedia, recentSeries, nil
}

// ListMediaAggregated 获取媒体列表（聚合模式：仅返回不属于合集的媒体）
func (s *MediaService) ListMediaAggregated(page, size int, libraryID string) ([]model.Media, int64, error) {
	return s.mediaRepo.ListNonEpisode(page, size, libraryID)
}

// CountNonEpisodeByLibrary 统计指定媒体库中非剧集媒体的数量
func (s *MediaService) CountNonEpisodeByLibrary(libraryID string) (int64, error) {
	return s.mediaRepo.CountNonEpisodeByLibrary(libraryID)
}

// Search 搜索媒体
func (s *MediaService) Search(keyword string, page, size int) ([]model.Media, int64, error) {
	return s.mediaRepo.Search(keyword, page, size)
}

// ContinueWatching 获取续播列表
func (s *MediaService) ContinueWatching(userID string, limit int) ([]model.WatchHistory, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	return s.historyRepo.ContinueWatching(userID, limit)
}

// UpdateProgress 更新观看进度
func (s *MediaService) UpdateProgress(userID, mediaID string, position, duration float64) error {
	completed := false
	if duration > 0 && position/duration > 0.9 {
		completed = true
	}

	history := &model.WatchHistory{
		UserID:    userID,
		MediaID:   mediaID,
		Position:  position,
		Duration:  duration,
		Completed: completed,
	}
	return s.historyRepo.Upsert(history)
}

// AddFavorite 添加收藏
func (s *MediaService) AddFavorite(userID, mediaID string) error {
	if s.favRepo.Exists(userID, mediaID) {
		return ErrAlreadyFavorited
	}
	fav := &model.Favorite{
		UserID:  userID,
		MediaID: mediaID,
	}
	return s.favRepo.Add(fav)
}

// RemoveFavorite 移除收藏
func (s *MediaService) RemoveFavorite(userID, mediaID string) error {
	return s.favRepo.Remove(userID, mediaID)
}

// ListFavorites 获取收藏列表
func (s *MediaService) ListFavorites(userID string, page, size int) ([]model.Favorite, int64, error) {
	return s.favRepo.List(userID, page, size)
}

// ListHistory 获取观看历史列表
func (s *MediaService) ListHistory(userID string, page, size int) ([]model.WatchHistory, int64, error) {
	return s.historyRepo.ListHistory(userID, page, size)
}

// DeleteHistory 删除单条观看记录
func (s *MediaService) DeleteHistory(userID, mediaID string) error {
	return s.historyRepo.DeleteHistory(userID, mediaID)
}

// ClearHistory 清空观看历史
func (s *MediaService) ClearHistory(userID string) error {
	return s.historyRepo.ClearHistory(userID)
}
