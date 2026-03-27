package service

import (
	"time"

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
	media, err := s.mediaRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	// 如果是剧集类型，加载关联的合集信息
	if media.MediaType == "episode" && media.SeriesID != "" {
		series, err := s.seriesRepo.FindByIDOnly(media.SeriesID)
		if err == nil {
			media.Series = series
		}
	}
	return media, nil
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

// MixedItem 混合项 — 统一表示电影或合集
type MixedItem struct {
	Type   string        `json:"type"` // "movie" 或 "series"
	Media  *model.Media  `json:"media,omitempty"`
	Series *model.Series `json:"series,omitempty"`
}

// ListMixed 获取电影与合集的混合列表（Emby风格：电影+合集混合展示，按时间排序）
func (s *MediaService) ListMixed(page, size int, libraryID string) ([]MixedItem, int64, error) {
	// 1. 获取所有独立电影（非剧集）
	movies, err := s.mediaRepo.RecentNonEpisodeAll(libraryID)
	if err != nil {
		return nil, 0, err
	}

	// 2. 获取所有合集
	seriesList, err := s.seriesRepo.ListAll(libraryID)
	if err != nil {
		return nil, 0, err
	}

	// 3. 合并为混合列表，按 created_at 降序排列
	var allItems []MixedItem
	for i := range movies {
		allItems = append(allItems, MixedItem{
			Type:  "movie",
			Media: &movies[i],
		})
	}
	for i := range seriesList {
		allItems = append(allItems, MixedItem{
			Type:   "series",
			Series: &seriesList[i],
		})
	}

	// 按 created_at 降序排序
	sortMixedItems(allItems)

	// 4. 分页
	total := int64(len(allItems))
	start := (page - 1) * size
	if start >= int(total) {
		return []MixedItem{}, total, nil
	}
	end := start + size
	if end > int(total) {
		end = int(total)
	}

	return allItems[start:end], total, nil
}

// sortMixedItems 按 created_at 降序排序混合列表
func sortMixedItems(items []MixedItem) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			ti := getMixedItemTime(items[i])
			tj := getMixedItemTime(items[j])
			if tj.After(ti) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func getMixedItemTime(item MixedItem) time.Time {
	if item.Media != nil {
		return item.Media.CreatedAt
	}
	if item.Series != nil {
		return item.Series.CreatedAt
	}
	return time.Time{}
}

// RecentMixed 最近添加混合列表（电影+合集按时间混合排列）
func (s *MediaService) RecentMixed(limit int) ([]MixedItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	movies, err := s.mediaRepo.RecentNonEpisode(limit)
	if err != nil {
		return nil, err
	}

	seriesList, err := s.seriesRepo.RecentUpdated(limit)
	if err != nil {
		return nil, err
	}

	var items []MixedItem
	for i := range movies {
		items = append(items, MixedItem{
			Type:  "movie",
			Media: &movies[i],
		})
	}
	for i := range seriesList {
		items = append(items, MixedItem{
			Type:   "series",
			Series: &seriesList[i],
		})
	}

	sortMixedItems(items)

	if len(items) > limit {
		items = items[:limit]
	}

	return items, nil
}

// CountNonEpisodeByLibrary 统计指定媒体库中非剧集媒体的数量
func (s *MediaService) CountNonEpisodeByLibrary(libraryID string) (int64, error) {
	return s.mediaRepo.CountNonEpisodeByLibrary(libraryID)
}

// Search 搜索媒体
func (s *MediaService) Search(keyword string, page, size int) ([]model.Media, int64, error) {
	return s.mediaRepo.Search(keyword, page, size)
}

// SearchAdvanced 高级搜索（支持多条件筛选和排序）
// 对 episode 类型的结果按 SeriesID 聚合，同一剧集只展示一个合集级别的条目
func (s *MediaService) SearchAdvanced(params repository.SearchAdvancedParams) ([]model.Media, int64, error) {
	media, total, err := s.mediaRepo.SearchAdvanced(params)
	if err != nil {
		return nil, 0, err
	}

	// 对结果进行剧集聚合去重
	media, deduped := s.deduplicateEpisodes(media)
	// 修正总数：减去被去重的数量
	total -= int64(deduped)

	return media, total, nil
}

// deduplicateEpisodes 对搜索结果中的 episode 按 SeriesID 聚合
// 同一个 SeriesID 的剧集只保留一个，并用 Series 合集信息替换展示字段
// 返回去重后的列表和被移除的数量
func (s *MediaService) deduplicateEpisodes(media []model.Media) ([]model.Media, int) {
	if len(media) == 0 {
		return media, 0
	}

	var result []model.Media
	seenSeriesIDs := make(map[string]bool)
	seriesCache := make(map[string]*model.Series)
	removed := 0

	for i := range media {
		item := media[i]

		// 非剧集类型直接保留
		if item.MediaType != "episode" || item.SeriesID == "" {
			result = append(result, item)
			continue
		}

		// 同一剧集已出现过，跳过
		if seenSeriesIDs[item.SeriesID] {
			removed++
			continue
		}
		seenSeriesIDs[item.SeriesID] = true

		// 用 Series 合集信息替换 episode 的展示字段
		if series, ok := seriesCache[item.SeriesID]; ok {
			enrichMediaWithSeriesInfo(&item, series)
		} else if series, err := s.seriesRepo.FindByIDOnly(item.SeriesID); err == nil {
			seriesCache[item.SeriesID] = series
			enrichMediaWithSeriesInfo(&item, series)
		}

		result = append(result, item)
	}

	return result, removed
}

// enrichMediaWithSeriesInfo 用 Series 合集信息替换 Media 的展示字段
func enrichMediaWithSeriesInfo(media *model.Media, series *model.Series) {
	if series == nil {
		return
	}
	if series.Title != "" {
		media.Title = series.Title
	}
	if series.PosterPath != "" {
		media.PosterPath = series.PosterPath
	}
	if series.BackdropPath != "" {
		media.BackdropPath = series.BackdropPath
	}
	if series.Rating > 0 {
		media.Rating = series.Rating
	}
	if series.Overview != "" {
		media.Overview = series.Overview
	}
	if series.Genres != "" {
		media.Genres = series.Genres
	}
	if series.Year > 0 {
		media.Year = series.Year
	}
	// 附加 Series 对象，前端可据此判断媒体类型并展示剧集信息（季数/集数）
	media.Series = series
	// 清除单集的文件大小和时长，避免前端误显示单集数据
	media.FileSize = 0
	media.Duration = 0
}

// SearchMixedResult 混合搜索结果
type SearchMixedResult struct {
	Media       []model.Media  `json:"media"`
	Series      []model.Series `json:"series"`
	MediaTotal  int64          `json:"media_total"`
	SeriesTotal int64          `json:"series_total"`
}

// SearchMixed 混合搜索（同时搜索媒体和合集）
func (s *MediaService) SearchMixed(keyword string, page, size int) (*SearchMixedResult, error) {
	media, mediaTotal, err := s.mediaRepo.Search(keyword, page, size)
	if err != nil {
		return nil, err
	}

	series, seriesTotal, err := s.seriesRepo.SearchSeries(keyword, page, size)
	if err != nil {
		return nil, err
	}

	return &SearchMixedResult{
		Media:       media,
		Series:      series,
		MediaTotal:  mediaTotal,
		SeriesTotal: seriesTotal,
	}, nil
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

// IsFavorited 检查是否已收藏
func (s *MediaService) IsFavorited(userID, mediaID string) bool {
	return s.favRepo.Exists(userID, mediaID)
}

// ListFavorites 获取收藏列表
func (s *MediaService) ListFavorites(userID string, page, size int) ([]model.Favorite, int64, error) {
	return s.favRepo.List(userID, page, size)
}

// ListHistory 获取观看历史列表
func (s *MediaService) ListHistory(userID string, page, size int) ([]model.WatchHistory, int64, error) {
	return s.historyRepo.ListHistory(userID, page, size)
}

// GetProgress 获取用户对指定媒体的观看进度
func (s *MediaService) GetProgress(userID, mediaID string) (*model.WatchHistory, error) {
	return s.historyRepo.GetByUserAndMedia(userID, mediaID)
}

// DeleteHistory 删除单条观看记录
func (s *MediaService) DeleteHistory(userID, mediaID string) error {
	return s.historyRepo.DeleteHistory(userID, mediaID)
}

// DeleteMedia 删除单个媒体记录
func (s *MediaService) DeleteMedia(id string) error {
	return s.mediaRepo.DeleteByID(id)
}

// UpdateMedia 更新媒体元数据
func (s *MediaService) UpdateMedia(media *model.Media) error {
	return s.mediaRepo.Update(media)
}

// GetMediaByID 获取媒体（不加载关联，用于管理操作）
func (s *MediaService) GetMediaByID(id string) (*model.Media, error) {
	return s.mediaRepo.FindByID(id)
}

// ClearHistory 清空观看历史
func (s *MediaService) ClearHistory(userID string) error {
	return s.historyRepo.ClearHistory(userID)
}
