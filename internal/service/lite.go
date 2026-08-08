package service

import (
	"fmt"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// NewLiteServices creates the NAS-oriented server profile.
//
// The lite profile keeps the complete movie/series experience while avoiding
// startup of unrelated subsystems such as music, photos, federation, plugins,
// preprocessing workers, Emby compatibility, adult scraping and AI scene
// analysis. The full profile remains available through cmd/server.
func NewLiteServices(repos *repository.Repositories, cfg *config.Config, logger *zap.SugaredLogger) *Services {
	mediaExecution, err := NewMediaExecutionService(repos.DB(), cfg, logger)
	if err != nil {
		panic(fmt.Sprintf("initialize media execution service: %v", err))
	}
	artifactMaintenance := NewArtifactMaintenanceService(repos.Transcode, cfg, logger)
	scanner := NewScannerService(repos.Media, repos.Series, cfg, logger)
	metadata := NewMetadataService(repos.Media, repos.Series, repos.Person, repos.MediaPerson, cfg, logger)

	wsHub := NewWSHub(logger)
	go wsHub.Run()
	scanner.SetWSHub(wsHub)
	artifactMaintenance.SetWSHub(wsHub)
	metadata.SetWSHub(wsHub)

	libraryService := NewLibraryService(
		repos.Library,
		repos.Media,
		repos.Series,
		repos.Favorite,
		repos.WatchHistory,
		repos.MediaPerson,
		nil, // Lite 不迁移或访问 AI 扫描归类表
		cfg,
		scanner,
		metadata,
		logger,
	)
	libraryService.SetWSHub(wsHub)

	fileWatcher := NewFileWatcherService(cfg, logger, repos.Library, repos.Media, repos.Series, scanner, metadata)
	fileWatcher.SetWSHub(wsHub)
	if err := fileWatcher.Start(); err != nil {
		logger.Errorf("文件监听服务启动失败: %v", err)
	}
	libraryService.SetFileWatcher(fileWatcher)

	nfoService := NewNFOService(logger)
	metadata.SetNFOService(nfoService)

	vfsManager := NewVFSManager(logger)
	scanner.SetVFSManager(vfsManager)
	nfoService.SetVFSManager(vfsManager)

	// Remote storage remains available but is initialized only when explicitly
	// configured, so local-library users do not pay for remote connection probes.
	webDAVService := NewWebDAVService(cfg, logger, vfsManager)
	if cfg.Storage.WebDAV.Enabled {
		if err := webDAVService.Initialize(); err != nil {
			logger.Warnf("WebDAV 服务初始化失败: %v", err)
		}
	}
	remoteStorageService := NewRemoteStorageService(cfg, logger, vfsManager)
	if cfg.Storage.Alist.Enabled || cfg.Storage.S3.Enabled {
		if err := remoteStorageService.Initialize(); err != nil {
			logger.Warnf("远程存储服务初始化失败: %v", err)
		}
	}
	SetGlobalRemoteStorageService(remoteStorageService)

	aiService := NewAIService(cfg.AI, cfg, repos.Media, repos.AICache, logger)
	var aiEnhancer *AIService
	if cfg.AI.Enabled {
		aiEnhancer = aiService
		metadata.SetAIService(aiService)
	}

	theTVDBService := NewTheTVDBService(repos.Media, repos.Series, cfg, logger)
	fanartService := NewFanartService(repos.Media, repos.Series, cfg, logger)
	providerChain := NewProviderChain(logger)
	providerChain.Register(NewTMDbProvider(metadata))
	providerChain.Register(NewDoubanProvider(metadata.douban))
	providerChain.Register(NewTheTVDBProvider(theTVDBService))
	providerChain.Register(NewBangumiProvider(metadata.bangumi))
	providerChain.Register(NewFanartProvider(fanartService))
	if cfg.AI.Enabled {
		providerChain.Register(NewAIProvider(aiService))
	}
	metadata.SetProviderChain(providerChain)
	metadata.SetTheTVDBService(theTVDBService)

	recommendService := NewRecommendService(
		repos.Media,
		repos.Series,
		repos.WatchHistory,
		repos.Favorite,
		repos.RecommendCache,
		logger,
	)
	if cfg.AI.Enabled {
		recommendService.SetAIService(aiService)
	}

	scrapeManager := NewScrapeManagerService(
		repos.ScrapeTask,
		repos.ScrapeHistory,
		repos.Media,
		repos.Series,
		metadata,
		aiEnhancer,
		logger,
	)
	scrapeManager.SetWSHub(wsHub)

	fileManager := NewFileManagerService(
		repos.Media,
		repos.Series,
		repos.FileOpLog,
		metadata,
		aiEnhancer,
		logger,
	)
	fileManager.SetWSHub(wsHub)

	subtitleSearchService := NewSubtitleSearchService("", cfg.Cache.CacheDir, logger)
	batchMetadataService := NewBatchMetadataService(repos.DB(), logger)
	importExportService := NewMediaImportExportService(repos.DB(), logger)
	statsService := NewStatsService(repos.PlaybackStats, repos.Media, logger)
	collectionService := NewCollectionService(repos.MovieCollection, repos.Media, logger)

	aicostService := NewAICostService(aiService)
	var aiRouter *AIRouter
	if cfg.AI.Enabled {
		aiRouter = NewAIRouter(aiService, aicostService, repos.AIUsage, repos.AIFailover, cfg, logger)
		aiRouter.LoadMonthUsage()
	}

	streamService := NewStreamService(repos.Media, repos.Series, mediaExecution, cfg, logger)
	streamService.SetSettingRepo(repos.SystemSetting)
	streamService.SetVFSManager(vfsManager)

	svcs := &Services{
		User:                NewUserService(repos.User, repos.AuditLog, cfg, logger),
		Auth:                NewAuthService(repos.User, repos.InviteCode, repos.LoginLog, repos.AuditLog, cfg, logger),
		Library:             libraryService,
		Media:               NewMediaService(repos.Media, repos.Series, repos.WatchHistory, repos.Favorite, repos.Library, repos.PlaybackStats, cfg, logger),
		Series:              NewSeriesService(repos.Series, repos.Media, logger),
		Stream:              streamService,
		MediaExecution:      mediaExecution,
		ArtifactMaintenance: artifactMaintenance,
		Metadata:            metadata,
		Scanner:             scanner,
		Playlist:            NewPlaylistService(repos.Playlist, logger),
		Recommend:           recommendService,
		Bookmark:            NewBookmarkService(repos.Bookmark, repos.Media, logger),
		Permission:          NewPermissionService(repos.UserPermission, repos.ContentRating, repos.WatchHistory, logger),
		FileWatcher:         fileWatcher,
		NFO:                 nfoService,
		Stats:               statsService,
		VFS:                 vfsManager,
		WebDAV:              webDAVService,
		RemoteStorage:       remoteStorageService,
		WSHub:               wsHub,
		AI:                  aiService,
		ScrapeManager:       scrapeManager,
		FileManager:         fileManager,
		TheTVDB:             theTVDBService,
		Fanart:              fanartService,
		ProviderChain:       providerChain,
		SubtitleSearch:      subtitleSearchService,
		BatchMetadata:       batchMetadataService,
		ImportExport:        importExportService,
		Collection:          collectionService,
		AICost:              aicostService,
		AIRouter:            aiRouter,
	}

	svcs.Series.SetMediaPersonRepo(repos.MediaPerson)
	svcs.Library.SetSeriesService(svcs.Series)
	svcs.Library.SetCollectionService(collectionService)

	// Scanning in the lite profile performs indexing and metadata matching only.
	// Expensive video/subtitle preprocessing and AI organization workers are not
	// started automatically.
	scanner.SetOnScanComplete(func(libraryID string) {
		logger.Infof("媒体库扫描完成 library_id=%s（lite 模式未启动预处理 worker）", libraryID)
	})

	return svcs
}
