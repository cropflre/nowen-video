package service

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
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
	libRepo     *repository.LibraryRepo
	statsRepo   *repository.PlaybackStatsRepo
	cfg         *config.Config
	logger      *zap.SugaredLogger
}

func NewMediaService(
	mediaRepo *repository.MediaRepo,
	seriesRepo *repository.SeriesRepo,
	historyRepo *repository.WatchHistoryRepo,
	favRepo *repository.FavoriteRepo,
	libRepo *repository.LibraryRepo,
	statsRepo *repository.PlaybackStatsRepo,
	cfg *config.Config,
	logger *zap.SugaredLogger,
) *MediaService {
	return &MediaService{
		mediaRepo:   mediaRepo,
		seriesRepo:  seriesRepo,
		historyRepo: historyRepo,
		favRepo:     favRepo,
		libRepo:     libRepo,
		statsRepo:   statsRepo,
		cfg:         cfg,
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

	// 2.5 对同名 Series 去重：标准化标题相同的多个 Series 只展示元数据最丰富的那个
	seriesList = deduplicateSeriesByTitle(seriesList)

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

// deduplicateSeriesByTitle 对同名 Series 去重
// 标准化标题相同（去掉季号后相同）的多个 Series，合并它们的统计信息，只保留元数据最丰富的那个
// 保留的 Series 会更新 SeasonCount 和 EpisodeCount 为所有同名系列的总和
func deduplicateSeriesByTitle(seriesList []model.Series) []model.Series {
	type groupInfo struct {
		bestIdx      int
		bestScore    int
		totalSeasons int
		totalEps     int
		latestUpdate time.Time
	}

	// 按 libraryID + 标准化标题分组
	type gk struct {
		lib  string
		name string
	}
	groups := make(map[gk]*groupInfo)
	groupOrder := make([]gk, 0) // 保持插入顺序

	for i, ser := range seriesList {
		normalized := normalizeSeriesTitleForMerge(ser.Title)
		if normalized == "" {
			normalized = ser.Title
		}
		key := gk{lib: ser.LibraryID, name: normalized}

		if g, ok := groups[key]; ok {
			g.totalSeasons += ser.SeasonCount
			g.totalEps += ser.EpisodeCount
			if ser.UpdatedAt.After(g.latestUpdate) {
				g.latestUpdate = ser.UpdatedAt
			}
			// 比较元数据丰富度
			score := seriesMetadataScore(&ser)
			if score > g.bestScore {
				g.bestScore = score
				g.bestIdx = i
			}
		} else {
			groups[key] = &groupInfo{
				bestIdx:      i,
				bestScore:    seriesMetadataScore(&ser),
				totalSeasons: ser.SeasonCount,
				totalEps:     ser.EpisodeCount,
				latestUpdate: ser.UpdatedAt,
			}
			groupOrder = append(groupOrder, key)
		}
	}

	// 构建去重后的结果
	result := make([]model.Series, 0, len(groups))
	for _, key := range groupOrder {
		g := groups[key]
		ser := seriesList[g.bestIdx]
		// 更新展示数据（使用所有同名系列的总和）
		ser.SeasonCount = g.totalSeasons
		ser.EpisodeCount = g.totalEps
		if g.latestUpdate.After(ser.UpdatedAt) {
			ser.UpdatedAt = g.latestUpdate
		}
		result = append(result, ser)
	}
	return result
}

// seriesMetadataScore 评估 Series 元数据丰富度
func seriesMetadataScore(ser *model.Series) int {
	score := 0
	if ser.Overview != "" {
		score += 3
	}
	if ser.PosterPath != "" {
		score += 3
	}
	if ser.BackdropPath != "" {
		score += 2
	}
	if ser.Rating > 0 {
		score += 2
	}
	if ser.TMDbID > 0 {
		score += 2
	}
	score += ser.EpisodeCount
	return score
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
// 自动对同名 Series 去重：标准化标题相同的多个 Series 只展示最新更新的那个
func (s *MediaService) RecentMixed(limit int) ([]MixedItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	movies, err := s.mediaRepo.RecentNonEpisode(limit)
	if err != nil {
		return nil, err
	}

	seriesList, err := s.seriesRepo.RecentUpdated(limit * 2) // 多取一些，去重后再截断
	if err != nil {
		return nil, err
	}

	// 对同名 Series 去重：按标准化标题分组，每组只保留最新更新的
	seriesList = deduplicateSeriesByTitle(seriesList)

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

// UpdateProgress 更新观看进度，并同步记录播放统计
func (s *MediaService) UpdateProgress(userID, mediaID string, position, duration float64) error {
	completed := false
	if duration > 0 && position/duration > 0.9 {
		completed = true
	}

	// 计算增量观看时长，写入 PlaybackStats 表
	if s.statsRepo != nil && position > 0 {
		s.recordPlaybackDelta(userID, mediaID, position)
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

// recordPlaybackDelta 计算增量观看时长并写入 PlaybackStats
// 通过对比上次保存的进度位置，计算本次实际观看的分钟数
func (s *MediaService) recordPlaybackDelta(userID, mediaID string, currentPosition float64) {
	var deltaMinutes float64

	// 获取上次保存的进度
	oldHistory, err := s.historyRepo.GetByUserAndMedia(userID, mediaID)
	if err == nil && oldHistory != nil && currentPosition > oldHistory.Position {
		// 正常连续播放：增量 = 当前位置 - 上次位置
		deltaMinutes = (currentPosition - oldHistory.Position) / 60.0
	} else if err != nil || oldHistory == nil {
		// 首次观看该媒体：无法计算增量，使用保守估计（上报间隔约15秒 = 0.25分钟）
		deltaMinutes = 0.25
	}
	// currentPosition <= oldHistory.Position 的情况（拖动回退）不记录

	// 防止异常数据：增量不超过5分钟（正常上报间隔约15秒，留足余量）
	if deltaMinutes <= 0 || deltaMinutes > 5 {
		return
	}

	stat := &model.PlaybackStats{
		UserID:       userID,
		MediaID:      mediaID,
		WatchMinutes: deltaMinutes,
		Date:         time.Now().Format("2006-01-02"),
	}
	if err := s.statsRepo.Record(stat); err != nil {
		s.logger.Debugf("记录播放统计失败: %v", err)
	}
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

// ==================== 增强详情 ====================

// StreamDetail 流详细信息
type StreamDetail struct {
	Index          int               `json:"index"`
	CodecType      string            `json:"codec_type"`      // video, audio, subtitle
	CodecName      string            `json:"codec_name"`      // h264, hevc, aac 等
	CodecLongName  string            `json:"codec_long_name"` // 编码器完整名称
	Profile        string            `json:"profile,omitempty"`
	Level          int               `json:"level,omitempty"` // 编码等级
	Width          int               `json:"width,omitempty"`
	Height         int               `json:"height,omitempty"`
	CodedWidth     int               `json:"coded_width,omitempty"`    // 编码宽度
	CodedHeight    int               `json:"coded_height,omitempty"`   // 编码高度
	AspectRatio    string            `json:"aspect_ratio,omitempty"`   // 显示宽高比（如 "16:9"）
	FrameRate      string            `json:"frame_rate,omitempty"`     // 帧率（如 "23.976"）
	BitRate        string            `json:"bit_rate,omitempty"`       // 码率
	BitDepth       int               `json:"bit_depth,omitempty"`      // 位深度
	RefFrames      int               `json:"ref_frames,omitempty"`     // 参考帧数
	IsInterlaced   bool              `json:"is_interlaced"`            // 是否隔行扫描
	SampleRate     string            `json:"sample_rate,omitempty"`    // 音频采样率
	Channels       int               `json:"channels,omitempty"`       // 音频声道数
	ChannelLayout  string            `json:"channel_layout,omitempty"` // 声道布局（如 "stereo", "5.1"）
	Language       string            `json:"language,omitempty"`       // 语言
	Title          string            `json:"title,omitempty"`          // 轨道标题
	IsDefault      bool              `json:"is_default"`
	IsForced       bool              `json:"is_forced"`
	PixFmt         string            `json:"pix_fmt,omitempty"`         // 像素格式
	ColorSpace     string            `json:"color_space,omitempty"`     // 色彩空间
	ColorTransfer  string            `json:"color_transfer,omitempty"`  // 色彩传输特性
	ColorPrimaries string            `json:"color_primaries,omitempty"` // 色彩原色
	ColorRange     string            `json:"color_range,omitempty"`     // 色彩范围（tv/pc）
	BitsPerSample  int               `json:"bits_per_sample,omitempty"`
	Duration       string            `json:"duration,omitempty"`
	StartTime      string            `json:"start_time,omitempty"` // 起始时间
	NbFrames       string            `json:"nb_frames,omitempty"`  // 总帧数
	Tags           map[string]string `json:"tags,omitempty"`
}

// FormatDetail 容器格式详细信息
type FormatDetail struct {
	FormatName     string            `json:"format_name"`      // 容器格式（如 "matroska,webm"）
	FormatLongName string            `json:"format_long_name"` // 容器格式完整名称
	Duration       string            `json:"duration"`         // 总时长（秒）
	Size           string            `json:"size"`             // 文件大小（字节）
	BitRate        string            `json:"bit_rate"`         // 总码率
	StreamCount    int               `json:"stream_count"`     // 流数量
	StartTime      string            `json:"start_time"`       // 起始时间
	Tags           map[string]string `json:"tags,omitempty"`   // 容器元数据标签
}

// FileDetail 文件系统详细信息
type FileDetail struct {
	FileName    string `json:"file_name"`   // 文件名
	FileDir     string `json:"file_dir"`    // 所在目录
	FileExt     string `json:"file_ext"`    // 扩展名
	FileSize    int64  `json:"file_size"`   // 文件大小（字节）
	MimeType    string `json:"mime_type"`   // MIME 类型
	Permissions string `json:"permissions"` // 文件权限（如 "-rwxr-xr-x"）
	Owner       string `json:"owner"`       // 文件所有者
	CreatedAt   string `json:"created_at"`  // 文件创建时间
	ModifiedAt  string `json:"modified_at"` // 文件修改时间
	MD5         string `json:"md5"`         // MD5 哈希值（按需计算）
}

// LibraryInfo 媒体库简要信息
type LibraryInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
}

// PlaybackStatsInfo 播放统计信息
type PlaybackStatsInfo struct {
	TotalPlayCount    int64   `json:"total_play_count"`    // 总播放次数（所有用户）
	TotalWatchMinutes float64 `json:"total_watch_minutes"` // 总观看分钟数
	UniqueViewers     int64   `json:"unique_viewers"`      // 独立观看人数
	LastPlayedAt      string  `json:"last_played_at"`      // 最后播放时间
}

// MediaDetailEnhanced 增强的媒体详情
type MediaDetailEnhanced struct {
	Media         *model.Media       `json:"media"`
	TechSpecs     *TechSpecs         `json:"tech_specs"`     // 技术规格
	Library       *LibraryInfo       `json:"library"`        // 所属媒体库
	PlaybackStats *PlaybackStatsInfo `json:"playback_stats"` // 播放统计
	FileInfo      *FileDetail        `json:"file_info"`      // 文件信息
}

// TechSpecs 技术规格
type TechSpecs struct {
	Streams []StreamDetail `json:"streams"` // 所有流信息
	Format  *FormatDetail  `json:"format"`  // 容器格式信息
}

// GetDetailEnhanced 获取增强的媒体详情（包含技术规格、媒体库信息、播放统计）
func (s *MediaService) GetDetailEnhanced(id string) (*MediaDetailEnhanced, error) {
	media, err := s.mediaRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	// 加载关联的合集信息
	if media.MediaType == "episode" && media.SeriesID != "" {
		series, err := s.seriesRepo.FindByIDOnly(media.SeriesID)
		if err == nil {
			media.Series = series
		}
	}

	result := &MediaDetailEnhanced{
		Media: media,
	}

	// 1. 获取技术规格（FFprobe 详细信息）
	result.TechSpecs = s.probeTechSpecs(media.FilePath)

	// 2. 获取媒体库信息
	if media.LibraryID != "" && s.libRepo != nil {
		if lib, err := s.libRepo.FindByID(media.LibraryID); err == nil {
			result.Library = &LibraryInfo{
				ID:   lib.ID,
				Name: lib.Name,
				Type: lib.Type,
				Path: lib.Path,
			}
		}
	}

	// 3. 获取播放统计
	result.PlaybackStats = s.getMediaPlaybackStats(id)

	// 4. 获取文件信息
	result.FileInfo = s.getFileDetail(media.FilePath)

	return result, nil
}

// probeTechSpecs 使用 FFprobe 获取详细技术规格
func (s *MediaService) probeTechSpecs(filePath string) *TechSpecs {
	ffprobePath := "ffprobe"
	if s.cfg != nil && s.cfg.App.FFprobePath != "" {
		ffprobePath = s.cfg.App.FFprobePath
	}

	cmd := exec.Command(ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		s.logger.Warnf("FFprobe 获取技术规格失败: %s, 错误: %v", filePath, err)
		return nil
	}

	var probeResult struct {
		Streams []struct {
			Index            int               `json:"index"`
			CodecType        string            `json:"codec_type"`
			CodecName        string            `json:"codec_name"`
			CodecLongName    string            `json:"codec_long_name"`
			Profile          string            `json:"profile"`
			Level            int               `json:"level"`
			Width            int               `json:"width"`
			Height           int               `json:"height"`
			CodedWidth       int               `json:"coded_width"`
			CodedHeight      int               `json:"coded_height"`
			DisplayAspect    string            `json:"display_aspect_ratio"`
			RFrameRate       string            `json:"r_frame_rate"`
			AvgFrameRate     string            `json:"avg_frame_rate"`
			BitRate          string            `json:"bit_rate"`
			BitsPerRawSample string            `json:"bits_per_raw_sample"`
			NbFrames         string            `json:"nb_frames"`
			Refs             int               `json:"refs"`
			FieldOrder       string            `json:"field_order"`
			StartTime        string            `json:"start_time"`
			SampleRate       string            `json:"sample_rate"`
			Channels         int               `json:"channels"`
			ChannelLayout    string            `json:"channel_layout"`
			BitsPerSample    int               `json:"bits_per_sample"`
			PixFmt           string            `json:"pix_fmt"`
			ColorSpace       string            `json:"color_space"`
			ColorTransfer    string            `json:"color_transfer"`
			ColorPrimaries   string            `json:"color_primaries"`
			ColorRange       string            `json:"color_range"`
			Duration         string            `json:"duration"`
			Tags             map[string]string `json:"tags"`
			Disposition      struct {
				Default int `json:"default"`
				Forced  int `json:"forced"`
			} `json:"disposition"`
		} `json:"streams"`
		Format struct {
			Filename       string            `json:"filename"`
			NbStreams      int               `json:"nb_streams"`
			FormatName     string            `json:"format_name"`
			FormatLongName string            `json:"format_long_name"`
			StartTime      string            `json:"start_time"`
			Duration       string            `json:"duration"`
			Size           string            `json:"size"`
			BitRate        string            `json:"bit_rate"`
			Tags           map[string]string `json:"tags"`
		} `json:"format"`
	}

	if err := json.Unmarshal(output, &probeResult); err != nil {
		s.logger.Warnf("解析 FFprobe 输出失败: %s, 错误: %v", filePath, err)
		return nil
	}

	specs := &TechSpecs{
		Format: &FormatDetail{
			FormatName:     probeResult.Format.FormatName,
			FormatLongName: probeResult.Format.FormatLongName,
			Duration:       probeResult.Format.Duration,
			Size:           probeResult.Format.Size,
			BitRate:        probeResult.Format.BitRate,
			StreamCount:    probeResult.Format.NbStreams,
			StartTime:      probeResult.Format.StartTime,
			Tags:           probeResult.Format.Tags,
		},
	}

	for _, stream := range probeResult.Streams {
		// 解析位深度
		bitDepth := stream.BitsPerSample
		if bitDepth == 0 && stream.BitsPerRawSample != "" {
			if bd, err := strconv.Atoi(stream.BitsPerRawSample); err == nil {
				bitDepth = bd
			}
		}

		detail := StreamDetail{
			Index:          stream.Index,
			CodecType:      stream.CodecType,
			CodecName:      stream.CodecName,
			CodecLongName:  stream.CodecLongName,
			Profile:        stream.Profile,
			Level:          stream.Level,
			Width:          stream.Width,
			Height:         stream.Height,
			CodedWidth:     stream.CodedWidth,
			CodedHeight:    stream.CodedHeight,
			AspectRatio:    stream.DisplayAspect,
			BitRate:        stream.BitRate,
			BitDepth:       bitDepth,
			RefFrames:      stream.Refs,
			IsInterlaced:   stream.FieldOrder != "" && stream.FieldOrder != "progressive" && stream.FieldOrder != "unknown",
			SampleRate:     stream.SampleRate,
			Channels:       stream.Channels,
			ChannelLayout:  stream.ChannelLayout,
			PixFmt:         stream.PixFmt,
			ColorSpace:     stream.ColorSpace,
			ColorTransfer:  stream.ColorTransfer,
			ColorPrimaries: stream.ColorPrimaries,
			ColorRange:     stream.ColorRange,
			BitsPerSample:  bitDepth,
			Duration:       stream.Duration,
			StartTime:      stream.StartTime,
			NbFrames:       stream.NbFrames,
			IsDefault:      stream.Disposition.Default == 1,
			IsForced:       stream.Disposition.Forced == 1,
			Tags:           stream.Tags,
		}

		// 解析帧率
		if stream.CodecType == "video" {
			detail.FrameRate = parseFrameRate(stream.RFrameRate)
			if detail.FrameRate == "" || detail.FrameRate == "0" {
				detail.FrameRate = parseFrameRate(stream.AvgFrameRate)
			}
		}

		// 提取语言和标题
		if stream.Tags != nil {
			detail.Language = stream.Tags["language"]
			detail.Title = stream.Tags["title"]
		}

		specs.Streams = append(specs.Streams, detail)
	}

	return specs
}

// parseFrameRate 解析帧率字符串（如 "24000/1001" -> "23.976"）
func parseFrameRate(fps string) string {
	if fps == "" || fps == "0/0" {
		return ""
	}
	parts := strings.Split(fps, "/")
	if len(parts) == 2 {
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && den > 0 {
			rate := num / den
			if rate > 0 && rate < 1000 {
				return fmt.Sprintf("%.3f", rate)
			}
		}
	}
	return fps
}

// getMediaPlaybackStats 获取媒体的播放统计信息
func (s *MediaService) getMediaPlaybackStats(mediaID string) *PlaybackStatsInfo {
	stats := &PlaybackStatsInfo{}

	if s.statsRepo == nil {
		return stats
	}

	// 获取总播放次数和总观看分钟数
	totalMinutes, totalCount, uniqueViewers, err := s.statsRepo.GetMediaStats(mediaID)
	if err != nil {
		s.logger.Debugf("获取媒体播放统计失败: %v", err)
		return stats
	}

	stats.TotalPlayCount = totalCount
	stats.TotalWatchMinutes = totalMinutes
	stats.UniqueViewers = uniqueViewers

	// 获取最后播放时间
	if s.historyRepo != nil {
		if lastHistory, err := s.historyRepo.GetLatestByMediaID(mediaID); err == nil && lastHistory != nil {
			stats.LastPlayedAt = lastHistory.UpdatedAt.Format(time.RFC3339)
		}
	}

	return stats
}

// getFileDetail 获取文件系统详细信息
// getMimeType 根据扩展名推断 MIME 类型
func getMimeType(ext string) string {
	mimeMap := map[string]string{
		".mp4": "video/mp4", ".mkv": "video/x-matroska", ".avi": "video/x-msvideo",
		".mov": "video/quicktime", ".wmv": "video/x-ms-wmv", ".flv": "video/x-flv",
		".webm": "video/webm", ".ts": "video/mp2t", ".m4v": "video/x-m4v",
		".mpg": "video/mpeg", ".mpeg": "video/mpeg", ".3gp": "video/3gpp",
		".ogv": "video/ogg", ".rmvb": "application/vnd.rn-realmedia-vbr",
		".rm": "application/vnd.rn-realmedia",
	}
	if mime, ok := mimeMap[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

func (s *MediaService) getFileDetail(filePath string) *FileDetail {
	ext := strings.ToLower(filepath.Ext(filePath))
	detail := &FileDetail{
		FileName: filepath.Base(filePath),
		FileDir:  filepath.Dir(filePath),
		FileExt:  ext,
		MimeType: getMimeType(ext),
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return detail
	}

	detail.FileSize = info.Size()
	detail.ModifiedAt = info.ModTime().Format(time.RFC3339)
	// 注意：Go 标准库不直接支持获取文件创建时间，使用修改时间作为近似值
	detail.CreatedAt = info.ModTime().Format(time.RFC3339)

	// 获取文件权限
	detail.Permissions = info.Mode().String()

	// 获取文件所有者（跨平台兼容）
	detail.Owner = getFileOwner(info)

	return detail
}
