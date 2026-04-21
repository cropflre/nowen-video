package emby

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// Handler 聚合 Emby 兼容层所需的全部依赖。
type Handler struct {
	cfg     *config.Config
	logger  *zap.SugaredLogger
	idMap   *IDMapper

	// 业务服务（只复用，不新建）
	auth      *service.AuthService
	stream    *service.StreamService
	transcode *service.TranscodeService // 用于节流（SetPlaybackPosition）

	// 仓储（只读为主）
	userRepo      *repository.UserRepo
	libraryRepo   *repository.LibraryRepo
	mediaRepo     *repository.MediaRepo
	seriesRepo    *repository.SeriesRepo
	watchRepo     *repository.WatchHistoryRepo
	favoriteRepo  *repository.FavoriteRepo
	mediaPersonRepo *repository.MediaPersonRepo

	// 固定的 ServerId（基于 JWTSecret 派生，重启稳定）
	serverID string
}

// NewHandler 构造 Emby 兼容层。
// 复用现有 service / repository 层，不做侵入性改动。
func NewHandler(
	cfg *config.Config,
	logger *zap.SugaredLogger,
	auth *service.AuthService,
	stream *service.StreamService,
	transcode *service.TranscodeService,
	repos *repository.Repositories,
) *Handler {
	return &Handler{
		cfg:             cfg,
		logger:          logger,
		idMap:           NewIDMapper(),
		auth:            auth,
		stream:          stream,
		transcode:       transcode,
		userRepo:        repos.User,
		libraryRepo:     repos.Library,
		mediaRepo:       repos.Media,
		seriesRepo:      repos.Series,
		watchRepo:       repos.WatchHistory,
		favoriteRepo:    repos.Favorite,
		mediaPersonRepo: repos.MediaPerson,
		serverID:        deriveServerID(cfg),
	}
}

// ServerID 派生稳定的 ServerId（Emby 客户端会把 ServerId 存入本地缓存，
// 重启后仍需保持一致以免用户反复重新配对）。
func (h *Handler) ServerID() string { return h.serverID }

// IDMapper 提供给其他包使用的映射器（例如路由层需要反向解析）。
func (h *Handler) IDMapper() *IDMapper { return h.idMap }

// deriveServerID 基于 JWTSecret 派生一个 40 位 hex 字符串作为 ServerId。
// 不直接暴露 secret。
func deriveServerID(cfg *config.Config) string {
	seed := cfg.Secrets.JWTSecret
	if seed == "" {
		seed = "nowen-video-server"
	}
	sum := sha1.Sum([]byte("nowen-video-server-id|" + seed))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// ==================== Library → View 映射 ====================

// mapLibraryToView 把 nowen Library 转成 Emby 左侧导航里的 "View"（CollectionFolder）。
func (h *Handler) mapLibraryToView(lib *model.Library) BaseItemDto {
	collType := "movies"
	switch strings.ToLower(lib.Type) {
	case "tvshow", "tvshows", "series":
		collType = "tvshows"
	case "music":
		collType = "music"
	case "photo", "photos":
		collType = "photos"
	case "mixed":
		collType = "mixed"
	}
	return BaseItemDto{
		Name:           lib.Name,
		ServerId:       h.serverID,
		Id:             h.idMap.ToEmbyID(lib.ID),
		IsFolder:       true,
		Type:           "CollectionFolder",
		CollectionType: collType,
		LocationType:   "FileSystem",
		Path:           lib.Path,
		DateCreated:    formatEmbyTime(lib.CreatedAt),
	}
}

// mapMediaToItem 将 Media（电影或剧集单集）转成 Emby BaseItemDto。
// kind = "Movie" / "Episode" / "Video"。
func (h *Handler) mapMediaToItem(m *model.Media, userData *UserItemData) BaseItemDto {
	itemType := "Movie"
	mediaType := "Video"
	if m.MediaType == "episode" {
		itemType = "Episode"
	}

	item := BaseItemDto{
		Name:            m.Title,
		OriginalTitle:   m.OrigTitle,
		ServerId:        h.serverID,
		Id:              h.idMap.ToEmbyID(m.ID),
		SortName:        m.Title,
		Overview:        m.Overview,
		ProductionYear:  m.Year,
		PremiereDate:    m.Premiered,
		CommunityRating: m.Rating,
		RunTimeTicks:    secondsToTicks(m.Duration),
		IsFolder:        false,
		Type:            itemType,
		MediaType:       mediaType,
		LocationType:    "FileSystem",
		Path:            m.FilePath,
		DateCreated:     formatEmbyTime(m.CreatedAt),
		ParentId:        h.idMap.ToEmbyID(m.LibraryID),
		ProviderIds:     buildProviderIds(m),
		Genres:          splitGenres(m.Genres),
		GenreItems:      genresToNameIdPairs(splitGenres(m.Genres)),
		ImageTags:       buildImageTags(m),
		UserData:        userData,
		Width:           parseResolutionWidth(m.Resolution),
		Height:          parseResolutionHeight(m.Resolution),
		Container:       containerFromPath(m.FilePath),
	}

	// 时长 fallback：如果没有秒级时长但有分钟级 Runtime，使用它
	if item.RunTimeTicks == 0 && m.Runtime > 0 {
		item.RunTimeTicks = minutesToTicks(m.Runtime)
	}

	if m.Tagline != "" {
		item.Taglines = []string{m.Tagline}
	}
	if m.Studio != "" {
		item.Studios = []NameIdPair{{Name: m.Studio, Id: h.idMap.ToEmbyID("studio:" + m.Studio)}}
	}

	// 剧集字段
	if itemType == "Episode" {
		item.IndexNumber = m.EpisodeNum
		item.ParentIndexNumber = m.SeasonNum
		if m.EpisodeTitle != "" {
			item.Name = m.EpisodeTitle
		}
		if m.SeriesID != "" {
			item.SeriesId = h.idMap.ToEmbyID(m.SeriesID)
		}
	}
	return item
}

// mapSeriesToItem 将 Series 转成 Emby "Series" 条目。
func (h *Handler) mapSeriesToItem(s *model.Series, userData *UserItemData) BaseItemDto {
	item := BaseItemDto{
		Name:            s.Title,
		OriginalTitle:   s.OrigTitle,
		ServerId:        h.serverID,
		Id:              h.idMap.ToEmbyID(s.ID),
		SortName:        s.Title,
		Overview:        s.Overview,
		ProductionYear:  s.Year,
		CommunityRating: s.Rating,
		IsFolder:        true,
		Type:            "Series",
		MediaType:       "",
		LocationType:    "FileSystem",
		Path:            s.FolderPath,
		DateCreated:     formatEmbyTime(s.CreatedAt),
		ParentId:        h.idMap.ToEmbyID(s.LibraryID),
		ProviderIds:     buildSeriesProviderIds(s),
		Genres:          splitGenres(s.Genres),
		GenreItems:      genresToNameIdPairs(splitGenres(s.Genres)),
		ImageTags:       buildSeriesImageTags(s),
		ChildCount:      s.EpisodeCount,
		UserData:        userData,
	}
	if s.Studio != "" {
		item.Studios = []NameIdPair{{Name: s.Studio, Id: h.idMap.ToEmbyID("studio:" + s.Studio)}}
	}
	return item
}
