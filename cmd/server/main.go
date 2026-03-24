package main

import (
	"fmt"
	"log"

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
	handlers := handler.NewHandlers(services, cfg, sugar)

	// 确保首次运行时创建管理员账号
	if err := services.User.EnsureAdminExists(); err != nil {
		sugar.Warnf("创建默认管理员失败: %v", err)
	}

	// 设置路由
	r := gin.Default()

	// 全局中间件
	r.Use(middleware.CORS(cfg.App.CORSOrigins...))

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
		auth.POST("/register", handlers.Auth.Register)
	}

	// 需要认证的auth路由
	authProtected := r.Group("/api/auth")
	authProtected.Use(middleware.JWTAuth(cfg.Secrets.JWTSecret))
	{
		authProtected.POST("/refresh", handlers.Auth.RefreshToken)
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
		api.GET("/media/recent", handlers.Media.Recent)
		api.GET("/media/recent/aggregated", handlers.Media.RecentAggregated)
		api.GET("/media/aggregated", handlers.Media.ListAggregated)
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
		api.GET("/stream/:id/master.m3u8", handlers.Stream.Master)
		api.GET("/stream/:id/:quality/:segment", handlers.Stream.Segment)

		// 海报/缩略图
		api.GET("/media/:id/poster", handlers.Stream.Poster)

		// 字幕
		api.GET("/subtitle/:id/tracks", handlers.Subtitle.ListTracks)
		api.GET("/subtitle/:id/extract/:index", handlers.Subtitle.ExtractTrack)
		api.GET("/subtitle/external", handlers.Subtitle.ServeExternal)

		// 元数据刮削（管理员）
		api.POST("/media/:id/scrape", middleware.AdminOnly(), handlers.Metadata.ScrapeMedia)

		// 用户
		api.GET("/users/me", handlers.User.Profile)
		api.PUT("/users/me/progress/:mediaId", handlers.User.UpdateProgress)
		api.GET("/users/me/favorites", handlers.User.Favorites)
		api.POST("/users/me/favorites/:mediaId", handlers.User.AddFavorite)
		api.DELETE("/users/me/favorites/:mediaId", handlers.User.RemoveFavorite)

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

		// 智能推荐
		api.GET("/recommend", handlers.Recommend.GetRecommendations)

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
	}

	// 管理路由
	admin := r.Group("/api/admin")
	admin.Use(middleware.JWTAuth(cfg.Secrets.JWTSecret), middleware.AdminOnly())
	{
		admin.GET("/users", handlers.Admin.ListUsers)
		admin.DELETE("/users/:id", handlers.Admin.DeleteUser)
		admin.GET("/system", handlers.Admin.SystemInfo)
		admin.GET("/transcode/status", handlers.Admin.TranscodeStatus)

		// TMDb 配置管理
		admin.GET("/settings/tmdb", handlers.Admin.GetTMDbConfig)
		admin.PUT("/settings/tmdb", handlers.Admin.UpdateTMDbConfig)
		admin.DELETE("/settings/tmdb", handlers.Admin.ClearTMDbConfig)

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

		// 访问日志
		admin.GET("/logs", handlers.Admin.ListAccessLogs)

		// 手动元数据匹配
		admin.GET("/metadata/search", handlers.Admin.SearchMetadata)
		admin.POST("/media/:mediaId/match", handlers.Admin.MatchMetadata)
	}

	// 静态文件（前端构建产物）
	r.Static("/assets", cfg.App.WebDir+"/assets")
	r.NoRoute(func(c *gin.Context) {
		c.File(cfg.App.WebDir + "/index.html")
	})

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	sugar.Infof("nowen-video 启动于 %s", addr)
	if err := r.Run(addr); err != nil {
		sugar.Fatalf("服务器启动失败: %v", err)
	}
}
