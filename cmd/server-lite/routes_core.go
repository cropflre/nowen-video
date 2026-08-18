package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/handler"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
)

func registerCoreAPI(
	r *gin.Engine,
	cfg *config.Config,
	services *service.Services,
	handlers *handler.Handlers,
	playbackPlan *handler.PlaybackPlanHandler,
	playbackSessions *handler.PlaybackSessionHandler,
	mediaAnalysis *handler.MediaAnalysisHandler,
	mediaAnalysisService *service.MediaAnalysisService,
	repos *repository.Repositories,
	jwtMiddleware gin.HandlerFunc,
) {
	api := r.Group("/api")
	api.Use(jwtMiddleware)
	service.AttachMediaAnalysisWorkerSettings(mediaAnalysisService, repos.SystemSetting)
	service.AttachMediaChapterRepository(mediaAnalysisService, repos.VideoChapter)

	guardLibraryDeleteWhileScanning := func(c *gin.Context) {
		libraryID := c.Param("id")
		for _, phase := range services.Library.ActiveScanPhases() {
			if phase.LibraryID == libraryID {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "媒体库正在扫描，请等待扫描结束后再删除"})
				return
			}
		}
		c.Next()
	}
	cleanupLibraryAnalysis := func(c *gin.Context) {
		mediaRows, _ := repos.Media.ListByLibraryID(c.Param("id"))
		c.Next()
		if c.Writer.Status() >= http.StatusBadRequest {
			return
		}
		for _, item := range mediaRows {
			mediaAnalysisService.CleanupMedia(item.ID)
			mediaAnalysisService.CleanupChapters(item.ID)
		}
	}

	api.GET("/libraries", handlers.Library.List)
	api.GET("/libraries/scan-status", middleware.AdminOnly(), handlers.Library.ScanStatus)
	api.POST("/libraries", middleware.AdminOnly(), handlers.Library.Create)
	api.PUT("/libraries/:id", middleware.AdminOnly(), handlers.Library.Update)
	api.POST("/libraries/:id/scan", middleware.AdminOnly(), handlers.Library.Scan)
	api.POST("/libraries/:id/reindex", middleware.AdminOnly(), handlers.Library.Reindex)
	api.DELETE("/libraries/:id", middleware.AdminOnly(), guardLibraryDeleteWhileScanning, cleanupLibraryAnalysis, handlers.Library.Delete)

	guardByMediaID := handler.MediaPermissionGuard(services.Permission, repos.Media, "id")
	guardByLibraryQuery := handler.LibraryPermissionGuard(services.Permission, "")

	api.GET("/media", guardByLibraryQuery, handlers.Media.List)
	api.GET("/media/recent", handlers.Media.Recent)
	api.GET("/media/recent/aggregated", handlers.Media.RecentAggregated)
	api.GET("/media/recent/mixed", handlers.Media.RecentMixed)
	api.GET("/media/aggregated", guardByLibraryQuery, handlers.Media.ListAggregated)
	api.GET("/media/mixed", guardByLibraryQuery, handlers.Media.ListMixed)
	api.GET("/media/continue", handlers.Media.Continue)
	api.GET("/media/:id", guardByMediaID, handlers.Media.Detail)
	api.GET("/media/:id/enhanced", guardByMediaID, handlers.Media.DetailEnhanced)
	api.GET("/media/:id/versions", guardByMediaID, handlers.Media.Versions)
	api.POST("/media/:id/scrape", guardByMediaID, middleware.AdminOnly(), handlers.Metadata.ScrapeMedia)

	// 精彩片段由服务端统一调度与持久化。默认 auto 模式优先交给合格客户端，
	// 没有客户端时自动回退现有 Sparse V2；读/播继续沿用媒体权限，重计算与节点接口仅管理员可用。
	api.GET("/media/:id/highlights", guardByMediaID, mediaAnalysis.ListHighlights)
	api.GET("/media/:id/highlights/status", guardByMediaID, mediaAnalysis.Status)
	api.GET("/media/:id/highlights/:highlightId/thumbnail", guardByMediaID, mediaAnalysis.Thumbnail)
	api.GET("/media/:id/highlights/:highlightId/preview", guardByMediaID, mediaAnalysis.Preview)
	api.POST("/media/:id/highlights/analyze", guardByMediaID, middleware.AdminOnly(), mediaAnalysis.AnalyzeHighlightsDistributed)
	api.DELETE("/media/:id/highlights", guardByMediaID, middleware.AdminOnly(), mediaAnalysis.DeleteHighlights)
	api.POST("/media/:id/ai/highlights", guardByMediaID, middleware.AdminOnly(), mediaAnalysis.AnalyzeHighlightsDistributed)

	// 章节继续复用 Web V3 的历史 URL，但执行链已经接管为 Media Compute Node V2 chapter_detect_v1。
	api.GET("/media/:id/chapters", guardByMediaID, mediaAnalysis.ListChapters)
	api.POST("/media/:id/ai/chapters", guardByMediaID, middleware.AdminOnly(), mediaAnalysis.GenerateChaptersDistributed)
	api.GET("/media/:id/ai/tasks", guardByMediaID, middleware.AdminOnly(), mediaAnalysis.AnalysisTasks)
	api.GET("/ai/tasks/:taskId", middleware.AdminOnly(), mediaAnalysis.AnalysisTask)

	api.GET("/admin/media-analysis/config", middleware.AdminOnly(), mediaAnalysis.WorkerConfig)
	api.PUT("/admin/media-analysis/config", middleware.AdminOnly(), mediaAnalysis.UpdateWorkerConfig)
	api.GET("/admin/media-analysis/workers", middleware.AdminOnly(), mediaAnalysis.Workers)
	api.POST("/media-analysis/workers/heartbeat", middleware.AdminOnly(), mediaAnalysis.WorkerHeartbeat)
	api.POST("/media-analysis/workers/claim", middleware.AdminOnly(), mediaAnalysis.WorkerClaim)
	api.POST("/media-analysis/workers/tasks/:taskId/progress", middleware.AdminOnly(), mediaAnalysis.WorkerProgress)
	api.POST("/media-analysis/workers/tasks/:taskId/complete", middleware.AdminOnly(), handler.ValidateMediaAnalysisWorkerComplete, mediaAnalysis.WorkerComplete)
	api.POST("/media-analysis/workers/tasks/:taskId/fail", middleware.AdminOnly(), mediaAnalysis.WorkerFail)

	api.GET("/series", handlers.Series.List)
	api.GET("/series/:id", handlers.Series.Detail)
	api.GET("/series/:id/seasons", handlers.Series.Seasons)
	api.GET("/series/:id/seasons/:season", handlers.Series.SeasonEpisodes)
	api.GET("/series/:id/next", handlers.Series.NextEpisode)
	api.GET("/series/:id/poster", handlers.Series.Poster)
	api.GET("/series/:id/backdrop", handlers.Series.Backdrop)
	api.GET("/series/:id/persons", handlers.Series.GetPersons)

	api.GET("/stream/:id/info", guardByMediaID, playbackPlan.GetInfo)
	api.GET("/stream/:id/plan", guardByMediaID, playbackPlan.Get)
	api.GET("/stream/:id/direct", guardByMediaID, handlers.Stream.Direct)
	api.GET("/stream/:id/remux", guardByMediaID, handlers.Stream.Remux)
	api.GET("/stream/:id/strm-seg", guardByMediaID, handlers.Stream.STRMSegment)
	api.GET("/stream/:id/strm-check", guardByMediaID, handlers.Stream.STRMCheck)
	api.GET("/media/:id/poster", handlers.Stream.Poster)
	api.GET("/media/:id/backdrop", handlers.Stream.Backdrop)

	api.POST("/playback/sessions", playbackSessions.Create)
	api.GET("/playback/sessions/:sessionID/status", playbackSessions.Status)
	api.POST("/playback/sessions/:sessionID/heartbeat", playbackSessions.Heartbeat)
	api.POST("/playback/sessions/:sessionID/restart", playbackSessions.Restart)
	api.DELETE("/playback/sessions/:sessionID", playbackSessions.Close)
	api.GET("/playback/sessions/:sessionID/generations/:generationID/stream.m3u8", playbackSessions.Playlist)
	api.GET("/playback/sessions/:sessionID/generations/:generationID/:file", playbackSessions.Segment)

	api.GET("/media/:id/persons", handlers.Media.GetPersons)
	api.GET("/persons/:id", handlers.Media.GetPersonDetail)
	api.GET("/persons/:id/media", handlers.Media.GetPersonMedia)
	api.GET("/persons/:id/profile", handlers.Media.PersonProfile)

	api.GET("/subtitle/:id/tracks", handlers.Subtitle.ListTracks)
	api.GET("/subtitle/:id/extract/:index", handlers.Subtitle.ExtractTrack)
	api.GET("/subtitle/external", handlers.Subtitle.ServeExternal)
	api.POST("/subtitle/:id/extract-all", handlers.Subtitle.ExtractAll)
	api.POST("/subtitle/:id/extract-all/async", handlers.Subtitle.ExtractAllAsync)
	api.GET("/subtitle/download", handlers.Subtitle.DownloadExtracted)
	api.GET("/subtitle/:id/search", handlers.SubtitleSearch.SearchSubtitles)
	api.POST("/subtitle/:id/download", handlers.SubtitleSearch.DownloadSubtitle)

	api.GET("/users/me", handlers.User.Profile)
	api.PUT("/users/me", handlers.User.UpdateProfile)
	api.GET("/users/me/login-logs", handlers.User.LoginLogs)
	api.PUT("/users/me/progress/:mediaId", handlers.User.UpdateProgress)
	api.GET("/users/me/progress/:mediaId", handlers.User.GetProgress)
	api.GET("/users/me/favorites", handlers.User.Favorites)
	api.POST("/users/me/favorites/:mediaId", handlers.User.AddFavorite)
	api.DELETE("/users/me/favorites/:mediaId", handlers.User.RemoveFavorite)
	api.GET("/users/me/favorites/:mediaId/check", handlers.User.CheckFavorite)
	api.GET("/users/me/history", handlers.User.History)
	api.DELETE("/users/me/history/:mediaId", handlers.User.DeleteHistory)
	api.DELETE("/users/me/history", handlers.User.ClearHistory)

	api.GET("/playlists", handlers.Playlist.List)
	api.POST("/playlists", handlers.Playlist.Create)
	api.GET("/playlists/:id", handlers.Playlist.Detail)
	api.DELETE("/playlists/:id", handlers.Playlist.Delete)
	api.POST("/playlists/:id/items/:mediaId", handlers.Playlist.AddItem)
	api.DELETE("/playlists/:id/items/:mediaId", handlers.Playlist.RemoveItem)

	api.GET("/search", handlers.Media.Search)
	api.GET("/search/advanced", handlers.Media.SearchAdvanced)
	api.GET("/search/mixed", handlers.Media.SearchMixed)
	api.GET("/recommend", handlers.Recommend.GetRecommendations)
	api.GET("/recommend/similar/:mediaId", handlers.Recommend.GetSimilarMedia)
	if cfg.AI.Enabled {
		api.GET("/ai/search", handlers.AI.SmartSearch)
	}

	api.POST("/bookmarks", handlers.Bookmark.Create)
	api.GET("/bookmarks", handlers.Bookmark.ListByUser)
	api.GET("/bookmarks/media/:mediaId", handlers.Bookmark.ListByMedia)
	api.PUT("/bookmarks/:id", handlers.Bookmark.Update)
	api.DELETE("/bookmarks/:id", handlers.Bookmark.Delete)

	// Comments are part of the normal media-detail experience in Lite as well.
	// Listing/creating are guarded by the same media permission contract as the
	// detail endpoint; deletion is still restricted by CommentService to owner/admin.
	api.GET("/media/:id/comments", guardByMediaID, handlers.Comment.ListByMedia)
	api.POST("/media/:id/comments", guardByMediaID, handlers.Comment.Create)
	api.DELETE("/comments/:id", handlers.Comment.Delete)

	api.POST("/stats/playback", handlers.Stats.RecordPlayback)
	api.GET("/stats/me", handlers.Stats.GetUserStats)

	api.GET("/media/:id/collection", handlers.Collection.GetMediaCollection)
	api.GET("/collections", handlers.Collection.ListCollections)
	api.GET("/collections/search", handlers.Collection.SearchCollections)
	api.GET("/collections/:id", handlers.Collection.GetCollectionDetail)
	api.GET("/collections/:id/poster", handlers.Collection.Poster)
}
