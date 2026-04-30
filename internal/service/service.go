package service

import (
	"fmt"
	"path/filepath"
	"strings"

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
	FileWatcher    *FileWatcherService
	NFO            *NFOService
	Stats          *StatsService
	Webhook        *WebhookService
	VFS            *VFSManager
	WebDAV         *WebDAVService
	RemoteStorage  *RemoteStorageService // V2.3: Alist / S3 统一管理
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
	AIScene *AISceneService
	// V5: Pulse 数据中心
	Pulse *PulseService
	// V6: P1~P3 新增功能
	ASR                *ASRService
	Preprocess         *PreprocessService
	SubtitlePreprocess *SubtitlePreprocessService
	GPUMonitor         *GPUMonitor
	// 电影系列合集
	Collection *CollectionService
	// 番号刮削服务（混合架构：Go 原生爬虫 + Python 微服务）
	AdultScraper *AdultScraperService
	// P3：番号批量刮削 + 进度推送
	AdultBatch *AdultBatchService
	// P3：智能镜像管理（多数据源可用性检测 + 熔断）
	AdultProxy *AdultProxyManager
	// P3：任务持久化存储
	AdultTaskStore *AdultTaskStore
	// P5：定时调度器（每日自动补刮）
	AdultScheduler *AdultScheduler
	// P5：元数据缓存（LRU+TTL，避免重复抓取）
	AdultCache *AdultMetadataCache
	// 自定义文件夹批量刮削（参考 mdcx：用户自选任意路径进行扫描+刮削）
	AdultFolderBatch *AdultFolderBatchService
}

func NewServices(repos *repository.Repositories, cfg *config.Config, logger *zap.SugaredLogger) *Services {
	transcoder := NewTranscodeService(repos.Transcode, cfg, logger)
	scanner := NewScannerService(repos.Media, repos.Series, cfg, logger)
	metadata := NewMetadataService(repos.Media, repos.Series, repos.Person, repos.MediaPerson, cfg, logger)

	// 创建WebSocket Hub
	wsHub := NewWSHub(logger)
	go wsHub.Run()

	// 注入WSHub到各服务
	scanner.SetWSHub(wsHub)
	transcoder.SetWSHub(wsHub)
	metadata.SetWSHub(wsHub)

	// 创建Library服务
	libService := NewLibraryService(repos.Library, repos.Media, repos.Series, repos.Favorite, repos.WatchHistory, repos.MediaPerson, cfg, scanner, metadata, logger)
	libService.SetWSHub(wsHub)

	// 创建调度器服务
	scheduler := NewSchedulerService(repos.ScheduledTask, repos.Library, logger)
	scheduler.SetLibraryService(libService)
	scheduler.SetTranscodeService(transcoder) // 注入转码服务，用于 cleanup 任务
	scheduler.SetWSHub(wsHub)
	scheduler.Start()

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
	webhookService := NewWebhookService(logger)
	vfsManager := NewVFSManager(logger)

	// 创建 WebDAV 存储服务
	webdavService := NewWebDAVService(cfg, logger, vfsManager)
	if err := webdavService.Initialize(); err != nil {
		logger.Warnf("WebDAV 服务初始化失败（可稍后在管理页面重新配置）: %v", err)
	}

	// V2.3: 创建远程存储统一服务（Alist + S3）
	remoteStorageService := NewRemoteStorageService(cfg, logger, vfsManager)
	if err := remoteStorageService.Initialize(); err != nil {
		logger.Warnf("远程存储服务初始化失败（可稍后在管理页面重新配置）: %v", err)
	}
	// 注入到全局 URL 解析器
	SetGlobalRemoteStorageService(remoteStorageService)

	// V2.1: 将 VFSManager 注入到 scanner 与 transcoder，支持 webdav:// 路径扫描

	// 创建 AI 服务
	aiService := NewAIService(cfg.AI, cfg, repos.Media, repos.AICache, logger)

	// 注入 AI 服务到元数据服务
	metadata.SetAIService(aiService)

	// 创建 TheTVDB 服务（剧集增强源）
	thetvdbService := NewTheTVDBService(repos.Media, repos.Series, cfg, logger)

	// 创建 Fanart.tv 服务（图片增强源）
	fanartService := NewFanartService(repos.Media, repos.Series, cfg, logger)

	// 创建番号刮削服务（混合架构：Go 原生爬虫 + Python 微服务）
	adultScraper := NewAdultScraperService(cfg, repos.Media, logger)
	// 注入 NFO 服务，使番号刮削成功后自动生成 .nfo 文件
	// 这样 Emby/Jellyfin/Infuse 等客户端能正确识别番号元数据
	adultScraper.SetNFOService(nfoService)

	// 创建多数据源调度链（第三阶段：统一 Provider 接口）
	providerChain := NewProviderChain(logger)
	providerChain.Register(NewAdultProvider(adultScraper))       // 优先级 5：番号内容（最高优先级，仅匹配成人内容）
	providerChain.Register(NewTMDbProvider(metadata))            // 优先级 10：主数据源
	providerChain.Register(NewDoubanProvider(metadata.douban))   // 优先级 20：豆瓣补充
	providerChain.Register(NewTheTVDBProvider(thetvdbService))   // 优先级 25：剧集增强
	providerChain.Register(NewBangumiProvider(metadata.bangumi)) // 优先级 30：动画专项
	providerChain.Register(NewFanartProvider(fanartService))     // 优先级 50：图片增强
	providerChain.Register(NewAIProvider(aiService))             // 优先级 100：AI 兜底

	// 注入 ProviderChain 到元数据服务
	metadata.SetProviderChain(providerChain)

	// 注入 TheTVDB 服务到元数据服务（用于手动匹配）
	metadata.SetTheTVDBService(thetvdbService)

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
		repos.Media, repos.Series, repos.FileOpLog,
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

	// 创建 ASR 语音识别字幕服务
	asrService := NewASRService(cfg, repos.Media, logger)
	asrService.SetWSHub(wsHub)
	asrService.SetAIService(aiService) // Phase 4: 字幕翻译需要 AI 服务

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
	// 使用 TranscodeService 检测后的实际硬件加速模式（而非配置中的 "auto"）
	detectedHWAccel := transcoder.GetHWAccelInfo()
	abrService := NewABRService(cfg, detectedHWAccel, logger)
	abrService.SetWSHub(wsHub)

	// 创建 GPU 安全监控服务
	// 【最佳性能策略】GPU 安全阈值直接使用内置默认值（85% / 80℃ / 60% / 70℃），
	// 不再暴露给用户手动调节，保证硬件不过载的同时无需心智负担。
	gpuThresholdCfg := DefaultGPUThresholdConfig()
	gpuMonitor := NewGPUMonitor(detectedHWAccel, gpuThresholdCfg, logger)
	gpuMonitor.SetWSHub(wsHub)
	gpuMonitor.Start()

	// 创建视频预处理服务
	preprocessService := NewPreprocessService(cfg, repos.Preprocess, repos.Media, abrService, detectedHWAccel, logger)
	preprocessService.SetWSHub(wsHub)
	preprocessService.SetGPUMonitor(gpuMonitor)

	// 创建字幕预处理服务
	subtitlePreprocessService := NewSubtitlePreprocessService(cfg, repos.SubtitlePreprocess, repos.Media, asrService, scanner, logger)
	subtitlePreprocessService.SetWSHub(wsHub)

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

	// V5: 创建 Pulse 数据中心服务
	pulseService := NewPulseService(repos.Pulse, logger)
	pulseService.SetWSHub(wsHub)

	svcs := &Services{
		User:           NewUserService(repos.User, repos.AuditLog, cfg, logger),
		Auth:           NewAuthService(repos.User, repos.InviteCode, repos.LoginLog, repos.AuditLog, cfg, logger),
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
		Permission:     NewPermissionService(repos.UserPermission, repos.ContentRating, repos.WatchHistory, logger),
		Scheduler:      scheduler,
		FileWatcher:    fileWatcher,
		NFO:            nfoService,
		Stats:          statsService,
		Webhook:        webhookService,
		VFS:            vfsManager,
		WebDAV:         webdavService,
		RemoteStorage:  remoteStorageService,
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
		AIScene: aiSceneService,
		// V5: Pulse 数据中心
		Pulse: pulseService,
		// V6: P1~P3 新增功能
		ASR:                asrService,
		Preprocess:         preprocessService,
		SubtitlePreprocess: subtitlePreprocessService,
		GPUMonitor:         gpuMonitor,
		// 电影系列合集
		Collection: NewCollectionService(repos.MovieCollection, repos.Media, logger),
		// 番号刮削
		AdultScraper: adultScraper,
	}

	// ==================== P3~P5：番号刮削扩展服务 ====================
	adultProxy := NewAdultProxyManager()
	adultCache := NewAdultMetadataCache(cfg.App.DataDir)
	adultTaskStore := NewAdultTaskStore(cfg.App.DataDir)
	adultBatch := NewAdultBatchService(adultScraper, wsHub)
	adultBatch.SetTaskStore(adultTaskStore)
	adultScheduler := NewAdultScheduler(adultBatch, adultProxy, adultScraper)
	adultFolderBatch := NewAdultFolderBatchService(adultScraper)

	svcs.AdultProxy = adultProxy
	svcs.AdultCache = adultCache
	svcs.AdultTaskStore = adultTaskStore
	svcs.AdultBatch = adultBatch
	svcs.AdultScheduler = adultScheduler
	svcs.AdultFolderBatch = adultFolderBatch

	// 启动调度器（后台循环，默认未启用，需配置开启）
	adultScheduler.Start()

	// 延迟注入：SeriesService 需要 MediaPersonRepo（用于合并时迁移演职员关联）	svcs.Series.SetMediaPersonRepo(repos.MediaPerson)

	// 延迟注入：LibraryService 需要 SeriesService（用于扫描后自动合并重复剧集）
	svcs.Library.SetSeriesService(svcs.Series)

	// 延迟注入：LibraryService 需要 CollectionService（用于扫描+刮削后自动匹配电影系列合集）
	svcs.Library.SetCollectionService(svcs.Collection)

	// 延迟注入：StreamService 需要 PreprocessService（用于自动选择预处理内容）
	svcs.Stream.SetPreprocessService(preprocessService)

	// 延迟注入：StreamService 需要 SystemSettingRepo（用于读取播放偏好设置）
	svcs.Stream.SetSettingRepo(repos.SystemSetting)

	// 延迟注入：PreprocessService 需要 SystemSettingRepo（用于读取 ABR 策略等配置）
	preprocessService.SetSettingRepo(repos.SystemSetting)

	// V2.1 延迟注入：Scanner 与 Stream 都需要 VFSManager（支持 webdav:// 路径）
	scanner.SetVFSManager(vfsManager)
	svcs.Stream.SetVFSManager(vfsManager)
	preprocessService.SetVFSManager(vfsManager)
	nfoService.SetVFSManager(vfsManager)

	// 延迟注入：SchedulerService 需要 SubtitlePreprocessService（用于定时字幕预处理）
	scheduler.SetSubtitlePreprocessService(subtitlePreprocessService, cfg.AI.SubtitleTargetLangs)

	// 延迟注入：扫描完成后自动触发预处理（受系统设置控制）
	scanner.SetOnScanComplete(func(libraryID string) {
		// 检查系统设置：是否启用扫描后自动预处理
		autoPreprocess := false
		if setting, err := repos.SystemSetting.Get("auto_preprocess_on_scan"); err == nil && (setting == "true" || setting == "1") {
			autoPreprocess = true
		}

		if autoPreprocess {
			count, err := preprocessService.SubmitLibrary(libraryID, 0)
			if err != nil {
				logger.Warnf("扫描后自动提交预处理失败: %v", err)
			} else if count > 0 {
				logger.Infof("扫描后自动提交 %d 个预处理任务", count)
			}
		} else {
			logger.Info("扫描后自动预处理已关闭（可在系统设置中开启）")
		}

		// P1: 扫描后自动触发字幕预处理（如果配置启用）
		if cfg.AI.AutoSubtitlePreprocess && autoPreprocess {
			var targetLangs []string
			if cfg.AI.SubtitleTargetLangs != "" {
				for _, lang := range strings.Split(cfg.AI.SubtitleTargetLangs, ",") {
					lang = strings.TrimSpace(lang)
					if lang != "" {
						targetLangs = append(targetLangs, lang)
					}
				}
			}
			subCount, err := subtitlePreprocessService.SubmitLibrary(libraryID, targetLangs, false)
			if err != nil {
				logger.Warnf("扫描后自动提交字幕预处理失败: %v", err)
			} else if subCount > 0 {
				logger.Infof("扫描后自动提交 %d 个字幕预处理任务", subCount)
			}
		}
		// 注意：电影系列合集匹配已移至 library.go 中刮削完成后执行，确保标题已更新
	})

	return svcs
}
