package service

import (
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// Services 聚合所有服务
type Services struct {
	User          *UserService
	Auth          *AuthService
	Library       *LibraryService
	Media         *MediaService
	Series        *SeriesService
	Stream        *StreamService
	Transcode     *TranscodeService
	Metadata      *MetadataService
	Scanner       *ScannerService
	Playlist      *PlaylistService
	Recommend     *RecommendService
	Cast          *CastService
	Bookmark      *BookmarkService
	Comment       *CommentService
	Permission    *PermissionService
	Scheduler     *SchedulerService
	Monitor       *MonitorService
	FileWatcher   *FileWatcherService
	NFO           *NFOService
	Stats         *StatsService
	Backup        *BackupService
	Webhook       *WebhookService
	VFS           *VFSManager
	WSHub         *WSHub
	AI            *AIService
	ScrapeManager *ScrapeManagerService
	FileManager   *FileManagerService
	AIAssistant   *AIAssistantService
	TheTVDB       *TheTVDBService
	Fanart        *FanartService
	ProviderChain *ProviderChain
}

func NewServices(repos *repository.Repositories, cfg *config.Config, logger *zap.SugaredLogger) *Services {
	transcoder := NewTranscodeService(repos.Transcode, cfg, logger)
	scanner := NewScannerService(repos.Media, repos.Series, cfg, logger)
	metadata := NewMetadataService(repos.Media, repos.Series, cfg, logger)

	// 创建WebSocket Hub
	wsHub := NewWSHub(logger)
	go wsHub.Run()

	// 注入WSHub到各服务
	scanner.SetWSHub(wsHub)
	transcoder.SetWSHub(wsHub)
	metadata.SetWSHub(wsHub)

	// 创建Library服务
	libService := NewLibraryService(repos.Library, repos.Media, repos.Series, repos.Favorite, repos.WatchHistory, scanner, metadata, logger)
	libService.SetWSHub(wsHub)

	// 创建调度器服务
	scheduler := NewSchedulerService(repos.ScheduledTask, repos.Library, logger)
	scheduler.SetLibraryService(libService)
	scheduler.SetWSHub(wsHub)
	scheduler.Start()

	// 创建监控服务
	monitor := NewMonitorService(cfg, transcoder, logger)
	monitor.SetWSHub(wsHub)
	monitor.Start()

	// 创建文件监听服务
	fileWatcher := NewFileWatcherService(cfg, logger, repos.Library, repos.Media, repos.Series, scanner, metadata)
	fileWatcher.SetWSHub(wsHub)
	if err := fileWatcher.Start(); err != nil {
		logger.Errorf("文件监听服务启动失败: %v", err)
	}

	// 注入文件监听到媒体库服务
	libService.SetFileWatcher(fileWatcher)

	// 创建新服务
	nfoService := NewNFOService(logger)
	statsService := NewStatsService(repos.PlaybackStats, repos.Media, logger)
	backupService := NewBackupService(repos.Media, repos.Series, repos.Library, repos.Person, repos.MediaPerson, cfg, logger)
	webhookService := NewWebhookService(logger)
	vfsManager := NewVFSManager(logger)

	// 创建 AI 服务
	aiService := NewAIService(cfg.AI, cfg, repos.Media, logger)

	// 注入 AI 服务到元数据服务
	metadata.SetAIService(aiService)

	// 创建 TheTVDB 服务（剧集增强源）
	thetvdbService := NewTheTVDBService(repos.Media, repos.Series, cfg, logger)

	// 创建 Fanart.tv 服务（图片增强源）
	fanartService := NewFanartService(repos.Media, repos.Series, cfg, logger)

	// 创建多数据源调度链（第三阶段：统一 Provider 接口）
	providerChain := NewProviderChain(logger)
	providerChain.Register(NewTMDbProvider(metadata))            // 优先级 10：主数据源
	providerChain.Register(NewDoubanProvider(metadata.douban))   // 优先级 20：豆瓣补充
	providerChain.Register(NewTheTVDBProvider(thetvdbService))   // 优先级 25：剧集增强
	providerChain.Register(NewBangumiProvider(metadata.bangumi)) // 优先级 30：动画专项
	providerChain.Register(NewFanartProvider(fanartService))     // 优先级 50：图片增强
	providerChain.Register(NewAIProvider(aiService))             // 优先级 100：AI 兜底

	// 注入 ProviderChain 到元数据服务
	metadata.SetProviderChain(providerChain)

	// 创建推荐服务并注入 AI
	recommendService := NewRecommendService(repos.Media, repos.Series, repos.WatchHistory, repos.Favorite, logger)
	recommendService.SetAIService(aiService)

	// 创建刮削管理服务
	scrapeManager := NewScrapeManagerService(
		repos.ScrapeTask, repos.ScrapeHistory,
		repos.Media, repos.Series,
		metadata, aiService, logger,
	)
	scrapeManager.SetWSHub(wsHub)

	// 创建文件管理服务
	fileManager := NewFileManagerService(
		repos.Media, repos.Series,
		metadata, aiService, logger,
	)
	fileManager.SetWSHub(wsHub)

	// 创建AI助手服务
	aiAssistant := NewAIAssistantService(
		aiService, fileManager,
		repos.Media, repos.Series, logger,
	)
	aiAssistant.SetWSHub(wsHub)

	return &Services{
		User:          NewUserService(repos.User, cfg, logger),
		Auth:          NewAuthService(repos.User, cfg, logger),
		Library:       libService,
		Media:         NewMediaService(repos.Media, repos.Series, repos.WatchHistory, repos.Favorite, logger),
		Series:        NewSeriesService(repos.Series, repos.Media, logger),
		Stream:        NewStreamService(repos.Media, repos.Series, transcoder, cfg, logger),
		Transcode:     transcoder,
		Metadata:      metadata,
		Scanner:       scanner,
		Playlist:      NewPlaylistService(repos.Playlist, logger),
		Recommend:     recommendService,
		Cast:          NewCastService(repos.Media, cfg, logger),
		Bookmark:      NewBookmarkService(repos.Bookmark, repos.Media, logger),
		Comment:       NewCommentService(repos.Comment, repos.Media, logger),
		Permission:    NewPermissionService(repos.UserPermission, repos.ContentRating, repos.WatchHistory, repos.AccessLog, logger),
		Scheduler:     scheduler,
		Monitor:       monitor,
		FileWatcher:   fileWatcher,
		NFO:           nfoService,
		Stats:         statsService,
		Backup:        backupService,
		Webhook:       webhookService,
		VFS:           vfsManager,
		WSHub:         wsHub,
		AI:            aiService,
		ScrapeManager: scrapeManager,
		FileManager:   fileManager,
		AIAssistant:   aiAssistant,
		TheTVDB:       thetvdbService,
		Fanart:        fanartService,
		ProviderChain: providerChain,
	}
}
