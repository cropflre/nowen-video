package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/handler"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化日志
	logger, _ := zap.NewProduction()
	if cfg.App.Debug {
		logger, _ = zap.NewDevelopment()
	}
	defer logger.Sync()
	sugar := logger.Sugar()

	// 初始化数据库
	db, err := gorm.Open(sqlite.Open(cfg.GetDBDSN()), &gorm.Config{})
	if err != nil {
		sugar.Fatalf("连接数据库失败: %v", err)
	}

	// 自动迁移
	if err := model.AutoMigrate(db); err != nil {
		sugar.Fatalf("数据库迁移失败: %v", err)
	}
	sugar.Info("数据库迁移完成")

	// 初始化各层
	repos := repository.NewRepositories(db)
	services := service.NewServices(repos, cfg, sugar)
	handlers := handler.NewHandlers(services, repos, cfg, sugar)

	// 确保首次运行时创建管理员账号
	if err := services.User.EnsureAdminExists(); err != nil {
		sugar.Warnf("创建默认管理员失败: %v", err)
	}

	// 启动时清理孤立数据（处理历史遗留的数据不一致问题）
	services.Library.CleanOrphanedData()

	// 设置路由
	if !cfg.App.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

	// 全局中间件
	r.Use(middleware.CORS(cfg.App.CORSOrigins...))
	r.Use(middleware.Security())
	r.Use(middleware.RateLimitWithConfig(middleware.RateLimitConfig{
		MaxRequests:  600, // 每分钟600次请求
		Window:       time.Minute,
		ExcludePaths: []string{"/api/ws"}, // WebSocket 不受速率限制
	}))

	// JWT Secret 安全检查
	if cfg.Secrets.JWTSecret == "" {
		sugar.Fatal("JWT Secret 未配置或自动生成失败，无法启动")
	}
	if cfg.IsDefaultJWTSecret() {
		sugar.Warn("⚠️  正在使用自动生成的 JWT Secret，建议在配置文件中设置固定值以避免重启后令牌失效")
	}

	// 公开路由（无需认证）
	auth := r.Group("/api/auth")
	{
		auth.POST("/login", handlers.Auth.Login)
		auth.GET("/status", handlers.Auth.Status)                                // 系统初始化状态（公开）
		auth.POST("/register", middleware.RateLimit(10), handlers.Auth.Register) // 注册接口额外限制：每分钟10次
	}

	// 需要认证的auth路由
	authProtected := r.Group("/api/auth")
	authProtected.Use(middleware.JWTAuth(cfg.Secrets.JWTSecret))
	{
		authProtected.POST("/refresh", handlers.Auth.RefreshToken)
		authProtected.PUT("/password", handlers.Auth.ChangePassword)
	}

	// PWA 资源文件（公开访问）
	r.GET("/manifest.json", func(c *gin.Context) {
		c.File(cfg.App.WebDir + "/manifest.json")
	})
	r.GET("/sw.js", func(c *gin.Context) {
		c.Header("Service-Worker-Allowed", "/")
		c.File(cfg.App.WebDir + "/sw.js")
	})

	// WebSocket路由（需要认证）
	r.GET("/api/ws", middleware.JWTAuth(cfg.Secrets.JWTSecret), handlers.WS.HandleWebSocket)

	// 需要认证的路由
	api := r.Group("/api")
	api.Use(middleware.JWTAuth(cfg.Secrets.JWTSecret))
	{
		// 媒体库
		api.GET("/libraries", handlers.Library.List)
		api.POST("/libraries", middleware.AdminOnly(), handlers.Library.Create)
		api.PUT("/libraries/:id", middleware.AdminOnly(), handlers.Library.Update)
		api.POST("/libraries/:id/scan", middleware.AdminOnly(), handlers.Library.Scan)
		api.POST("/libraries/:id/reindex", middleware.AdminOnly(), handlers.Library.Reindex)
		api.DELETE("/libraries/:id", middleware.AdminOnly(), handlers.Library.Delete)

		// 媒体内容
		api.GET("/media", handlers.Media.List)
		api.GET("/media/:id", handlers.Media.Detail)
		api.GET("/media/:id/enhanced", handlers.Media.DetailEnhanced)
		api.GET("/media/recent", handlers.Media.Recent)
		api.GET("/media/recent/aggregated", handlers.Media.RecentAggregated)
		api.GET("/media/recent/mixed", handlers.Media.RecentMixed)
		api.GET("/media/aggregated", handlers.Media.ListAggregated)
		api.GET("/media/mixed", handlers.Media.ListMixed)
		api.GET("/media/continue", handlers.Media.Continue)

		// 剧集合集
		api.GET("/series", handlers.Series.List)
		api.GET("/series/:id", handlers.Series.Detail)
		api.GET("/series/:id/seasons", handlers.Series.Seasons)
		api.GET("/series/:id/seasons/:season", handlers.Series.SeasonEpisodes)
		api.GET("/series/:id/next", handlers.Series.NextEpisode)

		// 流媒体
		api.GET("/stream/:id/info", handlers.Stream.MediaInfo)
		api.GET("/stream/:id/direct", handlers.Stream.Direct)
		api.GET("/stream/:id/remux", handlers.Stream.Remux)
		api.GET("/stream/:id/master.m3u8", handlers.Stream.Master)
		api.GET("/stream/:id/:quality/:segment", handlers.Stream.Segment)

		// 海报/缩略图
		api.GET("/media/:id/poster", handlers.Stream.Poster)
		api.GET("/series/:id/poster", handlers.Series.Poster)
		api.GET("/series/:id/backdrop", handlers.Series.Backdrop)
		api.GET("/series/:id/persons", handlers.Series.GetPersons)
		api.GET("/media/:id/persons", handlers.Media.GetPersons)

		// 演员作品
		api.GET("/persons/:id", handlers.Media.GetPersonDetail)
		api.GET("/persons/:id/media", handlers.Media.GetPersonMedia)

		// 字幕
		api.GET("/subtitle/:id/tracks", handlers.Subtitle.ListTracks)
		api.GET("/subtitle/:id/extract/:index", handlers.Subtitle.ExtractTrack)
		api.GET("/subtitle/external", handlers.Subtitle.ServeExternal)
		// P0: 批量字幕提取导出
		api.POST("/subtitle/:id/extract-all", handlers.Subtitle.ExtractAll)
		// P2: 异步字幕提取（大文件）
		api.POST("/subtitle/:id/extract-all/async", handlers.Subtitle.ExtractAllAsync)
		// 下载已提取的字幕文件
		api.GET("/subtitle/download", handlers.Subtitle.DownloadExtracted)

		// 字幕在线搜索与下载
		api.GET("/subtitle/:id/search", handlers.SubtitleSearch.SearchSubtitles)
		api.POST("/subtitle/:id/download", handlers.SubtitleSearch.DownloadSubtitle)

		// AI 字幕生成（语音识别）
		api.POST("/subtitle/:id/ai/generate", handlers.Subtitle.GenerateAISubtitle)
		api.GET("/subtitle/:id/ai/status", handlers.Subtitle.GetAISubtitleStatus)
		api.GET("/subtitle/:id/ai/serve", handlers.Subtitle.ServeAISubtitle)
		api.DELETE("/subtitle/:id/ai", handlers.Subtitle.DeleteAISubtitle)

		// 字幕翻译（AI 翻译）
		api.POST("/subtitle/:id/translate", handlers.Subtitle.TranslateSubtitle)
		api.GET("/subtitle/:id/translate/status", handlers.Subtitle.GetTranslateStatus)
		api.GET("/subtitle/:id/translate/:lang/serve", handlers.Subtitle.ServeTranslatedSubtitle)

		// ASR 服务状态
		api.GET("/asr/status", handlers.Subtitle.GetASRStatus)

		// 字幕预处理状态（用户可查询）
		api.GET("/subtitle-preprocess/media/:id/status", handlers.SubtitlePreprocess.GetMediaStatus)

		// 视频预处理（用户可查询状态和播放预处理内容）
		api.GET("/preprocess/media/:id/status", handlers.Preprocess.GetMediaTask)
		api.GET("/preprocess/media/:id/master.m3u8", handlers.Preprocess.ServePreprocessedMaster)
		api.GET("/preprocess/media/:id/:quality/:segment", handlers.Preprocess.ServePreprocessedSegment)
		api.GET("/preprocess/media/:id/thumbnail", handlers.Preprocess.ServeThumbnail)
		api.GET("/preprocess/media/:id/keyframe/:index", handlers.Preprocess.ServeKeyframe)

		// 元数据刮削（管理员）
		api.POST("/media/:id/scrape", middleware.AdminOnly(), handlers.Metadata.ScrapeMedia)

		// 用户
		api.GET("/users/me", handlers.User.Profile)
		api.PUT("/users/me/progress/:mediaId", handlers.User.UpdateProgress)
		api.GET("/users/me/favorites", handlers.User.Favorites)
		api.POST("/users/me/favorites/:mediaId", handlers.User.AddFavorite)
		api.DELETE("/users/me/favorites/:mediaId", handlers.User.RemoveFavorite)
		api.GET("/users/me/favorites/:mediaId/check", handlers.User.CheckFavorite)
		api.GET("/users/me/progress/:mediaId", handlers.User.GetProgress)

		// 观看历史
		api.GET("/users/me/history", handlers.User.History)
		api.DELETE("/users/me/history/:mediaId", handlers.User.DeleteHistory)
		api.DELETE("/users/me/history", handlers.User.ClearHistory)

		// 播放列表
		api.GET("/playlists", handlers.Playlist.List)
		api.POST("/playlists", handlers.Playlist.Create)
		api.GET("/playlists/:id", handlers.Playlist.Detail)
		api.DELETE("/playlists/:id", handlers.Playlist.Delete)
		api.POST("/playlists/:id/items/:mediaId", handlers.Playlist.AddItem)
		api.DELETE("/playlists/:id/items/:mediaId", handlers.Playlist.RemoveItem)

		// 搜索
		api.GET("/search", handlers.Media.Search)
		api.GET("/search/advanced", handlers.Media.SearchAdvanced)
		api.GET("/search/mixed", handlers.Media.SearchMixed)

		// 智能推荐
		api.GET("/recommend", handlers.Recommend.GetRecommendations)
		api.GET("/recommend/similar/:mediaId", handlers.Recommend.GetSimilarMedia)

		// AI 智能搜索
		api.GET("/ai/search", handlers.AI.SmartSearch)

		// 投屏
		api.GET("/cast/devices", handlers.Cast.ListDevices)
		api.POST("/cast/devices/refresh", handlers.Cast.RefreshDevices)
		api.POST("/cast/start", handlers.Cast.CastMedia)
		api.GET("/cast/sessions", handlers.Cast.ListSessions)
		api.GET("/cast/sessions/:sessionId", handlers.Cast.GetSession)
		api.POST("/cast/sessions/:sessionId/control", handlers.Cast.ControlCast)
		api.DELETE("/cast/sessions/:sessionId", handlers.Cast.StopSession)

		// 视频书签
		api.POST("/bookmarks", handlers.Bookmark.Create)
		api.GET("/bookmarks", handlers.Bookmark.ListByUser)
		api.GET("/bookmarks/media/:mediaId", handlers.Bookmark.ListByMedia)
		api.PUT("/bookmarks/:id", handlers.Bookmark.Update)
		api.DELETE("/bookmarks/:id", handlers.Bookmark.Delete)

		// 评论与评分
		api.GET("/media/:id/comments", handlers.Comment.ListByMedia)
		api.POST("/media/:id/comments", handlers.Comment.Create)
		api.DELETE("/comments/:id", handlers.Comment.Delete)

		// 播放统计
		api.POST("/stats/playback", handlers.Stats.RecordPlayback)
		api.GET("/stats/me", handlers.Stats.GetUserStats)

		// ==================== V2: 音乐库 ====================
		api.GET("/music/tracks", handlers.Music.ListTracks)
		api.GET("/music/albums", handlers.Music.ListAlbums)
		api.GET("/music/albums/:id", handlers.Music.GetAlbum)
		api.GET("/music/search", handlers.Music.SearchMusic)
		api.GET("/music/tracks/:id/lyrics", handlers.Music.GetLyrics)
		api.POST("/music/tracks/:id/love", handlers.Music.ToggleLove)
		api.GET("/music/playlists", handlers.Music.ListPlaylists)
		api.POST("/music/playlists", handlers.Music.CreatePlaylist)
		api.GET("/music/playlists/:id", handlers.Music.GetPlaylist)
		api.POST("/music/playlists/:id/tracks", handlers.Music.AddToPlaylist)

		// ==================== V2: 图片库 ====================
		api.GET("/photos", handlers.Photo.ListPhotos)
		api.GET("/photos/:id", handlers.Photo.GetPhoto)
		api.GET("/photos/albums", handlers.Photo.ListAlbums)
		api.POST("/photos/albums", handlers.Photo.CreateAlbum)
		api.POST("/photos/albums/:id/photos", handlers.Photo.AddPhotosToAlbum)
		api.POST("/photos/:id/favorite", handlers.Photo.ToggleFavorite)
		api.POST("/photos/:id/rating", handlers.Photo.SetRating)
		api.GET("/photos/search", handlers.Photo.SearchPhotos)
		api.GET("/photos/stats", handlers.Photo.GetStats)

		// ==================== V2: 联邦架构（共享媒体搜索） ====================
		api.GET("/federation/search", handlers.Federation.SearchSharedMedia)
		api.GET("/federation/stream/:id", handlers.Federation.GetSharedMediaStream)

		// ==================== V3: AI 场景识别与内容理解 ====================
		api.POST("/media/:id/ai/chapters", handlers.AIScene.GenerateChapters)

		// ==================== 电影系列合集 ====================
		api.GET("/media/:id/collection", handlers.Collection.GetMediaCollection)
		api.GET("/collections", handlers.Collection.ListCollections)
		api.GET("/collections/search", handlers.Collection.SearchCollections) // search 必须在 :id 之前注册
		api.GET("/collections/:id", handlers.Collection.GetCollectionDetail)
		api.GET("/collections/:id/poster", handlers.Collection.Poster)
		api.GET("/media/:id/chapters", handlers.AIScene.GetChapters)
		api.POST("/media/:id/ai/highlights", handlers.AIScene.ExtractHighlights)
		api.GET("/media/:id/highlights", handlers.AIScene.GetHighlights)
		api.POST("/media/:id/ai/covers", handlers.AIScene.GenerateCoverCandidates)
		api.GET("/media/:id/covers", handlers.AIScene.GetCoverCandidates)
		api.POST("/media/:id/covers/:candidateId/select", handlers.AIScene.SelectCover)
		api.POST("/media/:id/covers/apply", handlers.AIScene.ApplyCover)
		api.GET("/media/:id/ai/tasks", handlers.AIScene.GetAnalysisTasks)
		api.GET("/ai/tasks/:taskId", handlers.AIScene.GetAnalysisTask)

	}

	// 管理路由
	admin := r.Group("/api/admin")
	admin.Use(middleware.JWTAuth(cfg.Secrets.JWTSecret), middleware.AdminOnly())
	{
		admin.GET("/users", handlers.Admin.ListUsers)
		admin.DELETE("/users/:id", handlers.Admin.DeleteUser)
		admin.PUT("/users/:id/password", handlers.Admin.ResetUserPassword)
		admin.GET("/system", handlers.Admin.SystemInfo)
		admin.GET("/transcode/status", handlers.Admin.TranscodeStatus)
		admin.POST("/transcode/:taskId/cancel", handlers.Admin.CancelTranscode)

		// TMDb 配置管理
		admin.GET("/settings/tmdb", handlers.Admin.GetTMDbConfig)
		admin.PUT("/settings/tmdb", handlers.Admin.UpdateTMDbConfig)
		admin.DELETE("/settings/tmdb", handlers.Admin.ClearTMDbConfig)

		// 系统全局设置
		admin.GET("/settings/system", handlers.Admin.GetSystemSettings)
		admin.PUT("/settings/system", handlers.Admin.UpdateSystemSettings)

		// 系统监控
		admin.GET("/metrics", handlers.Admin.GetMetrics)

		// 定时任务管理
		admin.GET("/tasks", handlers.Admin.ListScheduledTasks)
		admin.POST("/tasks", handlers.Admin.CreateScheduledTask)
		admin.PUT("/tasks/:id", handlers.Admin.UpdateScheduledTask)
		admin.DELETE("/tasks/:id", handlers.Admin.DeleteScheduledTask)
		admin.POST("/tasks/:id/run", handlers.Admin.RunScheduledTaskNow)

		// 批量操作
		admin.POST("/batch/scan", handlers.Admin.BatchScan)
		admin.POST("/batch/scrape", handlers.Admin.BatchScrape)

		// 权限管理
		admin.GET("/permissions/:userId", handlers.Admin.GetUserPermission)
		admin.PUT("/permissions/:userId", handlers.Admin.UpdateUserPermission)

		// 内容分级
		admin.GET("/rating/:mediaId", handlers.Admin.GetContentRating)
		admin.PUT("/rating/:mediaId", handlers.Admin.SetContentRating)

		// 手动元数据匹配
		admin.GET("/metadata/search", handlers.Admin.SearchMetadata)
		admin.POST("/media/:mediaId/match", handlers.Admin.MatchMetadata)
		admin.POST("/media/:mediaId/unmatch", handlers.Admin.UnmatchMetadata)
		admin.PUT("/media/:mediaId/metadata", handlers.Admin.UpdateMediaMetadata)
		admin.DELETE("/media/:mediaId", handlers.Admin.DeleteMedia)

		// 剧集合集管理
		admin.POST("/series/:seriesId/match", handlers.Admin.MatchSeriesMetadata)
		admin.POST("/series/:seriesId/unmatch", handlers.Admin.UnmatchSeriesMetadata)
		admin.POST("/series/:seriesId/scrape", handlers.Admin.ScrapeSeriesMetadata)
		admin.PUT("/series/:seriesId/metadata", handlers.Admin.UpdateSeriesMetadata)
		admin.DELETE("/series/:seriesId", handlers.Admin.DeleteSeries)

		// 剧集合并（多季自动合并为一个整体）
		admin.POST("/series/merge", handlers.Admin.MergeSeries)
		admin.POST("/series/auto-merge", handlers.Admin.AutoMergeSeries)
		admin.GET("/series/merge-candidates", handlers.Admin.MergeCandidates)

		// 图片管理
		admin.GET("/images/tmdb", handlers.Admin.SearchTMDbImages)
		admin.POST("/media/:mediaId/image/upload", handlers.Admin.UploadMediaImage)
		admin.POST("/media/:mediaId/image/url", handlers.Admin.SetMediaImageByURL)
		admin.POST("/media/:mediaId/image/tmdb", handlers.Admin.SetMediaImageFromTMDb)
		admin.POST("/series/:seriesId/image/upload", handlers.Admin.UploadSeriesImage)
		admin.POST("/series/:seriesId/image/url", handlers.Admin.SetSeriesImageByURL)
		admin.POST("/series/:seriesId/image/tmdb", handlers.Admin.SetSeriesImageFromTMDb)

		// 文件系统浏览
		admin.GET("/fs/browse", handlers.Admin.BrowseFS)

		// 一键清空数据（保留影视文件）
		admin.POST("/system/clear-data", handlers.Admin.ClearAllData)

		// Bangumi 数据源
		admin.GET("/metadata/bangumi/search", handlers.Admin.SearchBangumi)
		admin.GET("/metadata/bangumi/subject/:subjectId", handlers.Admin.GetBangumiSubject)
		admin.POST("/media/:mediaId/match/bangumi", handlers.Admin.MatchMediaBangumi)
		admin.POST("/series/:seriesId/match/bangumi", handlers.Admin.MatchSeriesBangumi)
		admin.GET("/settings/bangumi", handlers.Admin.GetBangumiConfig)
		admin.PUT("/settings/bangumi", handlers.Admin.UpdateBangumiConfig)
		admin.DELETE("/settings/bangumi", handlers.Admin.ClearBangumiConfig)

		// 豆瓣数据源
		admin.GET("/metadata/douban/search", handlers.Admin.SearchDouban)
		admin.POST("/media/:mediaId/match/douban", handlers.Admin.MatchMediaDouban)
		admin.POST("/series/:seriesId/match/douban", handlers.Admin.MatchSeriesDouban)

		// TheTVDB 数据源
		admin.GET("/metadata/thetvdb/search", handlers.Admin.SearchTheTVDB)
		admin.POST("/series/:seriesId/match/thetvdb", handlers.Admin.MatchSeriesTheTVDB)

		// AI 管理
		admin.GET("/ai/status", handlers.AI.GetAIStatus)
		admin.PUT("/ai/config", handlers.AI.UpdateAIConfig)
		admin.POST("/ai/test", handlers.AI.TestAIConnection)
		admin.DELETE("/ai/cache", handlers.AI.ClearAICache)
		admin.GET("/ai/cache", handlers.AI.GetAICacheStats)
		admin.GET("/ai/errors", handlers.AI.GetAIErrorLogs)
		admin.POST("/ai/test/search", handlers.AI.TestSmartSearch)
		admin.POST("/ai/test/recommend", handlers.AI.TestRecommendReason)

		// 用户观影统计（管理员）
		admin.GET("/stats/:userId", handlers.Stats.GetUserStatsAdmin)

		// 刮削数据管理
		admin.POST("/scrape/tasks", handlers.ScrapeManager.CreateTask)
		admin.POST("/scrape/tasks/batch", handlers.ScrapeManager.BatchCreateTasks)
		admin.GET("/scrape/tasks", handlers.ScrapeManager.ListTasks)
		admin.GET("/scrape/tasks/:id", handlers.ScrapeManager.GetTask)
		admin.PUT("/scrape/tasks/:id", handlers.ScrapeManager.UpdateTask)
		admin.DELETE("/scrape/tasks/:id", handlers.ScrapeManager.DeleteTask)
		admin.POST("/scrape/tasks/:id/scrape", handlers.ScrapeManager.StartScrape)
		admin.POST("/scrape/tasks/:id/translate", handlers.ScrapeManager.TranslateTask)
		admin.POST("/scrape/batch/scrape", handlers.ScrapeManager.BatchStartScrape)
		admin.POST("/scrape/batch/translate", handlers.ScrapeManager.BatchTranslate)
		admin.POST("/scrape/batch/delete", handlers.ScrapeManager.BatchDeleteTasks)
		admin.POST("/scrape/export", handlers.ScrapeManager.ExportTasks)
		admin.GET("/scrape/statistics", handlers.ScrapeManager.GetStatistics)
		admin.GET("/scrape/history", handlers.ScrapeManager.GetHistory)

		// 影视文件管理
		admin.GET("/files", handlers.FileManager.ListFiles)
		admin.GET("/files/folders", handlers.FileManager.GetFolderTree)
		admin.GET("/files/by-folder", handlers.FileManager.ListFilesByFolder)
		admin.POST("/files/folders/create", handlers.FileManager.CreateFolder)
		admin.POST("/files/folders/rename", handlers.FileManager.RenameFolder)
		admin.POST("/files/folders/delete", handlers.FileManager.DeleteFolder)
		admin.GET("/files/:id", handlers.FileManager.GetFileDetail)
		admin.POST("/files/import", handlers.FileManager.ImportFile)
		admin.POST("/files/import/batch", handlers.FileManager.BatchImportFiles)
		admin.GET("/files/scan", handlers.FileManager.ScanDirectory)
		admin.PUT("/files/:id", handlers.FileManager.UpdateFile)
		admin.DELETE("/files/:id", handlers.FileManager.DeleteFile)
		admin.POST("/files/batch/delete", handlers.FileManager.BatchDeleteFiles)
		admin.POST("/files/:id/scrape", handlers.FileManager.ScrapeFile)
		admin.POST("/files/batch/scrape", handlers.FileManager.BatchScrapeFiles)
		admin.POST("/files/rename/preview", handlers.FileManager.PreviewRename)
		admin.POST("/files/rename/execute", handlers.FileManager.ExecuteRename)
		admin.POST("/files/rename/ai", handlers.FileManager.AIGenerateRenames)
		admin.GET("/files/rename/templates", handlers.FileManager.GetRenameTemplates)
		admin.GET("/files/stats", handlers.FileManager.GetStats)
		admin.GET("/files/logs", handlers.FileManager.GetOperationLogs)

		// AI助手
		admin.POST("/assistant/chat", handlers.AIAssistant.Chat)
		admin.POST("/assistant/execute", handlers.AIAssistant.ExecuteAction)
		admin.POST("/assistant/undo/:opId", handlers.AIAssistant.UndoOperation)
		admin.GET("/assistant/session/:sessionId", handlers.AIAssistant.GetSession)
		admin.DELETE("/assistant/session/:sessionId", handlers.AIAssistant.DeleteSession)
		admin.GET("/assistant/history", handlers.AIAssistant.GetOperationHistory)
		admin.GET("/assistant/misclassification", handlers.AIAssistant.AnalyzeMisclassification)
		admin.POST("/assistant/reclassify", handlers.AIAssistant.ReclassifyFiles)

		// 智能通知系统
		admin.GET("/notification/config", handlers.Notification.GetConfig)
		admin.PUT("/notification/config", handlers.Notification.UpdateConfig)
		admin.POST("/notification/test", handlers.Notification.TestNotification)

		// 批量元数据编辑
		admin.POST("/batch/metadata/media", handlers.BatchMetadata.BatchUpdateMedia)
		admin.POST("/batch/metadata/series", handlers.BatchMetadata.BatchUpdateSeries)

		// 媒体库导入/导出
		admin.POST("/import/test", handlers.BatchMetadata.TestImportConnection)
		admin.POST("/import/libraries", handlers.BatchMetadata.FetchImportLibraries)
		admin.POST("/import/external", handlers.BatchMetadata.ImportFromExternal)
		admin.GET("/export/library", handlers.BatchMetadata.ExportLibrary)
		admin.POST("/import/data", handlers.BatchMetadata.ImportFromExportData)

		// ==================== EMBY 格式兼容导入 ====================
		admin.GET("/emby/detect", handlers.EmbyCompat.DetectEmbyFormat)
		admin.POST("/emby/import", handlers.EmbyCompat.ImportEmbyLibrary)
		admin.GET("/emby/info", handlers.EmbyCompat.GetEmbyCompatInfo)
		admin.GET("/emby/nfo/:mediaId", handlers.EmbyCompat.GenerateEmbyNFO)

		// ==================== V2: 多用户配置文件 ====================
		admin.GET("/profiles", handlers.UserProfile.ListProfiles)
		admin.POST("/profiles", handlers.UserProfile.CreateProfile)
		admin.GET("/profiles/:id", handlers.UserProfile.GetProfile)
		admin.PUT("/profiles/:id", handlers.UserProfile.UpdateProfile)
		admin.DELETE("/profiles/:id", handlers.UserProfile.DeleteProfile)
		admin.POST("/profiles/:id/switch", handlers.UserProfile.SwitchProfile)
		admin.GET("/profiles/:id/watch-logs", handlers.UserProfile.GetWatchLogs)
		admin.GET("/profiles/:id/usage", handlers.UserProfile.GetDailyUsage)
		admin.GET("/profiles/:id/stats", handlers.UserProfile.GetProfileStats)

		// ==================== V2: 离线下载 ====================
		admin.POST("/downloads", handlers.OfflineDownload.CreateDownload)
		admin.POST("/downloads/batch", handlers.OfflineDownload.BatchDownload)
		admin.GET("/downloads", handlers.OfflineDownload.ListDownloads)
		admin.GET("/downloads/queue", handlers.OfflineDownload.GetQueueInfo)
		admin.POST("/downloads/:id/cancel", handlers.OfflineDownload.CancelDownload)
		admin.POST("/downloads/:id/pause", handlers.OfflineDownload.PauseDownload)
		admin.POST("/downloads/:id/resume", handlers.OfflineDownload.ResumeDownload)
		admin.DELETE("/downloads/:id", handlers.OfflineDownload.DeleteDownload)

		// ==================== V2: ABR 自适应码率 ====================
		admin.GET("/abr/status", handlers.ABR.GetStatus)
		admin.GET("/abr/gpu", handlers.ABR.GetGPUInfo)
		admin.DELETE("/abr/cache", handlers.ABR.CleanCache)

		// ==================== V2: 插件系统 ====================
		admin.GET("/plugins", handlers.Plugin.ListPlugins)
		admin.GET("/plugins/:id", handlers.Plugin.GetPlugin)
		admin.POST("/plugins/:id/enable", handlers.Plugin.EnablePlugin)
		admin.POST("/plugins/:id/disable", handlers.Plugin.DisablePlugin)
		admin.DELETE("/plugins/:id", handlers.Plugin.UninstallPlugin)
		admin.PUT("/plugins/:id/config", handlers.Plugin.UpdatePluginConfig)
		admin.POST("/plugins/scan", handlers.Plugin.ScanPlugins)

		// ==================== V2: 音乐库管理 ====================
		admin.POST("/music/scan", handlers.Music.ScanLibrary)

		// ==================== V2: 图片库管理 ====================
		admin.POST("/photos/scan", handlers.Photo.ScanLibrary)

		// ==================== V2: 多服务器联邦架构 ====================
		admin.GET("/federation/nodes", handlers.Federation.ListNodes)
		admin.POST("/federation/nodes", handlers.Federation.RegisterNode)
		admin.DELETE("/federation/nodes/:id", handlers.Federation.RemoveNode)
		admin.POST("/federation/nodes/:id/sync", handlers.Federation.SyncNode)
		admin.GET("/federation/stats", handlers.Federation.GetStats)
		admin.GET("/federation/sync-tasks", handlers.Federation.GetSyncTasks)

		// ==================== P1: 批量移动媒体 ====================
		admin.POST("/media/batch-move", handlers.Library.BatchMoveMedia)

		// ==================== 重复媒体检测 ====================
		admin.GET("/duplicates", handlers.Library.DetectAllDuplicates)
		admin.GET("/libraries/:id/duplicates", handlers.Library.DetectDuplicates)
		admin.POST("/libraries/:id/mark-duplicates", handlers.Library.MarkDuplicates)

		// ==================== V5: Pulse 数据中心（管理员） ====================
		admin.GET("/pulse/dashboard", handlers.Pulse.GetDashboard)
		admin.GET("/pulse/dashboard/trends", handlers.Pulse.GetPlayTrends)
		admin.GET("/pulse/dashboard/top-content", handlers.Pulse.GetTopContent)
		admin.GET("/pulse/dashboard/top-users", handlers.Pulse.GetTopUsers)
		admin.GET("/pulse/dashboard/recent", handlers.Pulse.GetRecentPlays)
		admin.GET("/pulse/analytics", handlers.Pulse.GetAnalytics)
		admin.GET("/pulse/analytics/hourly", handlers.Pulse.GetHourlyDistribution)
		admin.GET("/pulse/analytics/libraries", handlers.Pulse.GetLibraryStats)
		admin.GET("/pulse/analytics/growth", handlers.Pulse.GetMediaGrowth)

		// ==================== 视频预处理管理 ====================
		admin.POST("/preprocess/submit", handlers.Preprocess.SubmitMedia)
		admin.POST("/preprocess/batch", handlers.Preprocess.BatchSubmit)
		admin.POST("/preprocess/library/:id", handlers.Preprocess.SubmitLibrary)
		admin.GET("/preprocess/tasks", handlers.Preprocess.ListTasks)
		admin.GET("/preprocess/tasks/:id", handlers.Preprocess.GetTask)
		admin.POST("/preprocess/tasks/:id/pause", handlers.Preprocess.PauseTask)
		admin.POST("/preprocess/tasks/:id/resume", handlers.Preprocess.ResumeTask)
		admin.POST("/preprocess/tasks/:id/cancel", handlers.Preprocess.CancelTask)
		admin.POST("/preprocess/tasks/:id/retry", handlers.Preprocess.RetryTask)
		admin.DELETE("/preprocess/tasks/:id", handlers.Preprocess.DeleteTask)
		admin.GET("/preprocess/statistics", handlers.Preprocess.GetStatistics)
		admin.GET("/preprocess/system-load", handlers.Preprocess.GetSystemLoad)
		admin.GET("/preprocess/performance-config", handlers.Preprocess.GetPerformanceConfig)
		admin.PUT("/preprocess/performance-config", handlers.Preprocess.UpdatePerformanceConfig)
		admin.POST("/preprocess/tasks/batch-delete", handlers.Preprocess.BatchDeleteTasks)
		admin.POST("/preprocess/tasks/batch-cancel", handlers.Preprocess.BatchCancelTasks)
		admin.POST("/preprocess/tasks/batch-retry", handlers.Preprocess.BatchRetryTasks)
		admin.DELETE("/preprocess/cache/:id", handlers.Preprocess.CleanCache)

		// ==================== 字幕预处理管理 ====================
		admin.POST("/subtitle-preprocess/submit", handlers.SubtitlePreprocess.SubmitMedia)
		admin.POST("/subtitle-preprocess/batch", handlers.SubtitlePreprocess.BatchSubmit)
		admin.POST("/subtitle-preprocess/library/:id", handlers.SubtitlePreprocess.SubmitLibrary)
		admin.GET("/subtitle-preprocess/tasks", handlers.SubtitlePreprocess.ListTasks)
		admin.GET("/subtitle-preprocess/tasks/:id", handlers.SubtitlePreprocess.GetTask)
		admin.POST("/subtitle-preprocess/tasks/:id/cancel", handlers.SubtitlePreprocess.CancelTask)
		admin.POST("/subtitle-preprocess/tasks/:id/retry", handlers.SubtitlePreprocess.RetryTask)
		admin.DELETE("/subtitle-preprocess/tasks/:id", handlers.SubtitlePreprocess.DeleteTask)
		admin.GET("/subtitle-preprocess/statistics", handlers.SubtitlePreprocess.GetStatistics)
		admin.POST("/subtitle-preprocess/tasks/batch-delete", handlers.SubtitlePreprocess.BatchDeleteTasks)
		admin.POST("/subtitle-preprocess/tasks/batch-cancel", handlers.SubtitlePreprocess.BatchCancelTasks)
		admin.POST("/subtitle-preprocess/tasks/batch-retry", handlers.SubtitlePreprocess.BatchRetryTasks)
		admin.POST("/subtitle-preprocess/retry-all-failed", handlers.SubtitlePreprocess.RetryAllFailed)
		admin.DELETE("/subtitle-preprocess/tasks/by-status/:status", handlers.SubtitlePreprocess.DeleteByStatus)
		admin.GET("/subtitle-preprocess/asr-health", handlers.SubtitlePreprocess.CheckASRHealth)

		// ==================== 电影系列合集管理 ====================
		admin.POST("/collections", handlers.Collection.CreateCollection)
		admin.POST("/collections/auto-match", handlers.Collection.AutoMatch)             // auto-match 必须在 :id 之前注册
		admin.POST("/collections/merge-duplicates", handlers.Collection.MergeDuplicates) // 合并同名重复合集
		admin.POST("/collections/cleanup-empty", handlers.Collection.CleanupEmpty)       // 清理空壳合集
		admin.GET("/collections/duplicate-stats", handlers.Collection.DuplicateStats)    // 重复合集统计
		admin.PUT("/collections/:id", handlers.Collection.UpdateCollection)
		admin.DELETE("/collections/:id", handlers.Collection.DeleteCollection)
		admin.POST("/collections/:id/media", handlers.Collection.AddMedia)
		admin.DELETE("/collections/:id/media/:mediaId", handlers.Collection.RemoveMedia)

	}

	// ==================== V2: 联邦 API（供其他节点调用） ====================
	federation := r.Group("/api/federation")
	{
		federation.GET("/health", handlers.Federation.Health)
		federation.GET("/media", handlers.Federation.MediaList)
	}

	// 静态文件（前端构建产物）
	r.Static("/assets", cfg.App.WebDir+"/assets")
	r.NoRoute(func(c *gin.Context) {
		c.File(cfg.App.WebDir + "/index.html")
	})

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	sugar.Infof("nowen-video 启动于 %s", addr)

	// 使用 http.Server 实现优雅关闭
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// 在 goroutine 中启动服务器
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sugar.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 等待中断信号以优雅关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	sugar.Info("正在关闭服务器...")

	// 设置 30 秒超时用于优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		sugar.Fatalf("服务器强制关闭: %v", err)
	}

	sugar.Info("服务器已优雅关闭")
}
