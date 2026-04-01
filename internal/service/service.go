package service

import (
	"fmt"
	"path/filepath"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// Services 聚合所有服务
type Services struct {
	User           *UserService
	Auth           *AuthService
	Library        *LibraryService
	Media          *MediaService
	Series         *SeriesService
	Stream         *StreamService
	Transcode      *TranscodeService
	Metadata       *MetadataService
	Scanner        *ScannerService
	Playlist       *PlaylistService
	Recommend      *RecommendService
	Cast           *CastService
	Bookmark       *BookmarkService
	Comment        *CommentService
	Permission     *PermissionService
	Scheduler      *SchedulerService
	Monitor        *MonitorService
	FileWatcher    *FileWatcherService
	NFO            *NFOService
	Stats          *StatsService
	Backup         *BackupService
	Webhook        *WebhookService
	VFS            *VFSManager
	WSHub          *WSHub
	AI             *AIService
	ScrapeManager  *ScrapeManagerService
	FileManager    *FileManagerService
	AIAssistant    *AIAssistantService
	TheTVDB        *TheTVDBService
	Fanart         *FanartService
	ProviderChain  *ProviderChain
	Notification   *NotificationService
	SubtitleSearch *SubtitleSearchService
	BatchMetadata  *BatchMetadataService
	ImportExport   *MediaImportExportService
	EmbyCompat     *EmbyCompatService
	// V2: 中期发展规划服务
	UserProfile     *UserProfileService
	OfflineDownload *OfflineDownloadService
	ABR             *ABRService
	Plugin          *PluginService
	Music           *MusicService
	Photo           *PhotoService
	Federation      *FederationService
	// V3: 新增服务
	AIScene      *AISceneService
	FamilySocial *FamilySocialService
	Live         *LiveService
	CloudSync    *CloudSyncService
	// V5: Pulse 数据中心
	Pulse *PulseService
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
	aiService := NewAIService(cfg.AI, cfg, repos.Media, repos.AICache, logger)

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
	recommendService := NewRecommendService(repos.Media, repos.Series, repos.WatchHistory, repos.Favorite, repos.RecommendCache, logger)
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

	// 创建智能通知服务
	notificationService := NewNotificationService(logger)

	// 创建字幕在线搜索服务
	subtitleSearchService := NewSubtitleSearchService("", cfg.Cache.CacheDir, logger)

	// 创建批量元数据编辑服务
	batchMetadataService := NewBatchMetadataService(repos.DB(), logger)

	// 创建媒体库导入/导出服务
	importExportService := NewMediaImportExportService(repos.DB(), logger)

	// 创建 EMBY 兼容服务
	embyCompatService := NewEmbyCompatService(repos.Media, repos.Series, nfoService, scanner, logger)
	embyCompatService.SetWSHub(wsHub)

	// V2: 创建多用户配置文件服务
	userProfileService := NewUserProfileService(repos.DB(), logger)

	// V2: 创建离线下载服务
	offlineDownloadService := NewOfflineDownloadService(repos.DB(), filepath.Join(cfg.Cache.CacheDir, "downloads"), logger)
	offlineDownloadService.SetWSHub(wsHub)

	// V2: 创建ABR自适应码率服务
	abrService := NewABRService(cfg, cfg.App.HWAccel, logger)
	abrService.SetWSHub(wsHub)

	// V2: 创建可扩展插件系统
	pluginService := NewPluginService(repos.DB(), filepath.Join(cfg.Cache.CacheDir, "plugins"), logger)

	// V2: 创建音乐库服务
	musicService := NewMusicService(repos.DB(), logger)

	// V2: 创建图片库服务
	photoService := NewPhotoService(repos.DB(), filepath.Join(cfg.Cache.CacheDir, "thumbnails", "photos"), logger)

	// V2: 创建多服务器联邦架构服务
	federationService := NewFederationService(repos.DB(), fmt.Sprintf("node_local_%d", cfg.App.Port), logger)
	federationService.SetWSHub(wsHub)

	// V3: 创建AI场景识别服务
	aiSceneService := NewAISceneService(
		cfg, aiService,
		repos.Media, repos.VideoChapter, repos.VideoHighlight,
		repos.AIAnalysisTask, repos.CoverCandidate, logger,
	)
	aiSceneService.SetWSHub(wsHub)

	// V3: 创建家庭社交服务
	familySocialService := NewFamilySocialService(
		repos.FamilyGroup, repos.FamilyMember,
		repos.MediaShare, repos.MediaLike, repos.MediaRecommendation,
		repos.Media, repos.Series, logger,
	)
	familySocialService.SetWSHub(wsHub)

	// V3: 创建直播服务
	liveService := NewLiveService(
		cfg, repos.LiveSource, repos.LivePlaylist, repos.LiveRecording, logger,
	)
	liveService.SetWSHub(wsHub)

	// V3: 创建云端同步服务
	cloudSyncService := NewCloudSyncService(
		repos.SyncDevice, repos.SyncRecord, repos.UserSyncConfig,
		repos.WatchHistory, repos.Favorite, repos.Playlist, logger,
	)
	cloudSyncService.SetWSHub(wsHub)

	// V5: 创建 Pulse 数据中心服务
	pulseService := NewPulseService(repos.Pulse, logger)
	pulseService.SetWSHub(wsHub)

	return &Services{
		User:           NewUserService(repos.User, cfg, logger),
		Auth:           NewAuthService(repos.User, cfg, logger),
		Library:        libService,
		Media:          NewMediaService(repos.Media, repos.Series, repos.WatchHistory, repos.Favorite, repos.Library, repos.PlaybackStats, cfg, logger),
		Series:         NewSeriesService(repos.Series, repos.Media, logger),
		Stream:         NewStreamService(repos.Media, repos.Series, transcoder, cfg, logger),
		Transcode:      transcoder,
		Metadata:       metadata,
		Scanner:        scanner,
		Playlist:       NewPlaylistService(repos.Playlist, logger),
		Recommend:      recommendService,
		Cast:           NewCastService(repos.Media, cfg, logger),
		Bookmark:       NewBookmarkService(repos.Bookmark, repos.Media, logger),
		Comment:        NewCommentService(repos.Comment, repos.Media, logger),
		Permission:     NewPermissionService(repos.UserPermission, repos.ContentRating, repos.WatchHistory, repos.AccessLog, logger),
		Scheduler:      scheduler,
		Monitor:        monitor,
		FileWatcher:    fileWatcher,
		NFO:            nfoService,
		Stats:          statsService,
		Backup:         backupService,
		Webhook:        webhookService,
		VFS:            vfsManager,
		WSHub:          wsHub,
		AI:             aiService,
		ScrapeManager:  scrapeManager,
		FileManager:    fileManager,
		AIAssistant:    aiAssistant,
		TheTVDB:        thetvdbService,
		Fanart:         fanartService,
		ProviderChain:  providerChain,
		Notification:   notificationService,
		SubtitleSearch: subtitleSearchService,
		BatchMetadata:  batchMetadataService,
		ImportExport:   importExportService,
		EmbyCompat:     embyCompatService,
		// V2
		UserProfile:     userProfileService,
		OfflineDownload: offlineDownloadService,
		ABR:             abrService,
		Plugin:          pluginService,
		Music:           musicService,
		Photo:           photoService,
		Federation:      federationService,
		// V3
		AIScene:      aiSceneService,
		FamilySocial: familySocialService,
		Live:         liveService,
		CloudSync:    cloudSyncService,
		// V5: Pulse 数据中心
		Pulse: pulseService,
	}
}
