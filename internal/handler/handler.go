package handler

import (
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// Handlers 聚合所有HTTP处理器
type Handlers struct {
	Auth           *AuthHandler
	Library        *LibraryHandler
	Media          *MediaHandler
	Series         *SeriesHandler
	Stream         *StreamHandler
	User           *UserHandler
	Admin          *AdminHandler
	Subtitle       *SubtitleHandler
	Metadata       *MetadataHandler
	Playlist       *PlaylistHandler
	Recommend      *RecommendHandler
	Cast           *CastHandler
	WS             *WSHandler
	Bookmark       *BookmarkHandler
	Comment        *CommentHandler
	Stats          *StatsHandler
	AI             *AIHandler
	ScrapeManager  *ScrapeManagerHandler
	FileManager    *FileManagerHandler
	AIAssistant    *AIAssistantHandler
	Notification   *NotificationHandler
	SubtitleSearch *SubtitleSearchHandler
	BatchMetadata  *BatchMetadataHandler
	EmbyCompat     *EmbyCompatHandler
	// V2: 中期发展规划处理器
	UserProfile     *UserProfileHandler
	OfflineDownload *OfflineDownloadHandler
	ABR             *ABRHandler
	Plugin          *PluginHandler
	Music           *MusicHandler
	Photo           *PhotoHandler
	Federation      *FederationHandler
	// V3: 新增处理器
	AIScene *AISceneHandler
	// V5: Pulse 数据中心
	Pulse *PulseHandler
	// V6: P1~P3 新增处理器
	// 视频预处理
	Preprocess *PreprocessHandler
	// 字幕预处理
	SubtitlePreprocess *SubtitlePreprocessHandler
	// 电影系列合集
	Collection *CollectionHandler
	// V2.1: WebDAV 存储管理
	Storage *StorageHandler
	// 系统日志
	SystemLog *SystemLogHandler
	// 番号刮削管理
	AdultScraper *AdultScraperHandler
}

func NewHandlers(services *service.Services, repos *repository.Repositories, cfg *config.Config, logger *zap.SugaredLogger) *Handlers {
	h := &Handlers{
		Auth:    &AuthHandler{authService: services.Auth, serverName: cfg.Emby.ServerName, logger: logger},
		Library: &LibraryHandler{libService: services.Library, permSvc: services.Permission, logger: logger},
		Media:   &MediaHandler{mediaService: services.Media, personRepo: repos.Person, mediaPersonRepo: repos.MediaPerson, logger: logger},
		Series:  &SeriesHandler{seriesService: services.Series, mediaPersonRepo: repos.MediaPerson, logger: logger},
		Stream:  &StreamHandler{streamService: services.Stream, transcodeService: services.Transcode, logger: logger},
		User:    &UserHandler{userService: services.User, mediaService: services.Media, loginLogRepo: repos.LoginLog, logger: logger},
		Admin: &AdminHandler{
			userService:       services.User,
			authService:       services.Auth,
			transcodeService:  services.Transcode,
			schedulerService:  services.Scheduler,
			permissionService: services.Permission,
			libraryService:    services.Library,
			metadataService:   services.Metadata,
			seriesService:     services.Series,
			settingRepo:       repos.SystemSetting,
			libraryRepo:       repos.Library,
			loginLogRepo:      repos.LoginLog,
			auditLogRepo:      repos.AuditLog,
			inviteRepo:        repos.InviteCode,
			idleScheduler:     services.IdleScheduler,
			cfg:               cfg,
			logger:            logger,
			db:                repos.DB(),
		},
		Subtitle:       &SubtitleHandler{scanner: services.Scanner, streamService: services.Stream, asrService: services.ASR, logger: logger},
		Metadata:       &MetadataHandler{metadataService: services.Metadata, logger: logger},
		Playlist:       &PlaylistHandler{playlistService: services.Playlist, logger: logger},
		Recommend:      &RecommendHandler{recommendService: services.Recommend, logger: logger},
		Cast:           &CastHandler{castService: services.Cast, logger: logger},
		WS:             &WSHandler{hub: services.WSHub, logger: logger},
		Bookmark:       &BookmarkHandler{bookmarkService: services.Bookmark, logger: logger},
		Comment:        &CommentHandler{commentService: services.Comment, logger: logger},
		Stats:          &StatsHandler{statsService: services.Stats, logger: logger},
		AI:             &AIHandler{aiService: services.AI, logger: logger},
		ScrapeManager:  &ScrapeManagerHandler{scrapeService: services.ScrapeManager, logger: logger},
		FileManager:    &FileManagerHandler{fileService: services.FileManager, logger: logger},
		AIAssistant:    &AIAssistantHandler{assistantService: services.AIAssistant, logger: logger},
		Notification:   &NotificationHandler{notifyService: services.Notification, logger: logger},
		SubtitleSearch: &SubtitleSearchHandler{subtitleSearch: services.SubtitleSearch, streamService: services.Stream, logger: logger},
		BatchMetadata:  &BatchMetadataHandler{batchService: services.BatchMetadata, importExportSvc: services.ImportExport, logger: logger},
		EmbyCompat:     &EmbyCompatHandler{embyService: services.EmbyCompat, logger: logger},
		// V2
		UserProfile:     &UserProfileHandler{profileService: services.UserProfile, logger: logger},
		OfflineDownload: &OfflineDownloadHandler{downloadService: services.OfflineDownload, logger: logger},
		ABR:             &ABRHandler{abrService: services.ABR, logger: logger},
		Plugin:          &PluginHandler{pluginService: services.Plugin, logger: logger},
		Music:           &MusicHandler{musicService: services.Music, logger: logger},
		Photo:           &PhotoHandler{photoService: services.Photo, logger: logger},
		Federation:      &FederationHandler{federationService: services.Federation, logger: logger},
		// V3
		AIScene: &AISceneHandler{sceneService: services.AIScene, logger: logger},
		// V5: Pulse 数据中心
		Pulse: &PulseHandler{pulseService: services.Pulse, logger: logger},
		// V6: P1~P3 新增处理器
		// 视频预处理
		Preprocess: NewPreprocessHandler(services.Preprocess),
		// 字幕预处理
		SubtitlePreprocess: NewSubtitlePreprocessHandler(services.SubtitlePreprocess),
		// 电影系列合集
		Collection: &CollectionHandler{collectionService: services.Collection, streamService: services.Stream, logger: logger},
		// V2.1: WebDAV 存储管理
		Storage: NewStorageHandler(services.WebDAV, services.RemoteStorage, cfg, logger),
		// 系统日志
		SystemLog: &SystemLogHandler{logRepo: repos.SystemLog, logger: logger},
		// 番号刮削管理
		AdultScraper: &AdultScraperHandler{scraperService: services.AdultScraper, cfg: cfg, logger: logger},
	}

	// P3~P5：注入番号刮削扩展服务
	h.AdultScraper.SetP3Services(
		services.AdultBatch,
		services.AdultTaskStore,
		services.AdultProxy,
		services.AdultCache,
		services.AdultScheduler,
	)

	return h
}
