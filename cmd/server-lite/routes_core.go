package main

import (
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
	repos *repository.Repositories,
	jwtMiddleware gin.HandlerFunc,
) {
	api := r.Group("/api")
	api.Use(jwtMiddleware)

	api.GET("/libraries", handlers.Library.List)
	api.GET("/libraries/scan-status", middleware.AdminOnly(), handlers.Library.ScanStatus)
	api.POST("/libraries", middleware.AdminOnly(), handlers.Library.Create)
	api.PUT("/libraries/:id", middleware.AdminOnly(), handlers.Library.Update)
	api.POST("/libraries/:id/scan", middleware.AdminOnly(), handlers.Library.Scan)
	api.POST("/libraries/:id/reindex", middleware.AdminOnly(), handlers.Library.Reindex)
	api.DELETE("/libraries/:id", middleware.AdminOnly(), handlers.Library.Delete)

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

	api.GET("/series", handlers.Series.List)
	api.GET("/series/:id", handlers.Series.Detail)
	api.GET("/series/:id/seasons", handlers.Series.Seasons)
	api.GET("/series/:id/seasons/:season", handlers.Series.SeasonEpisodes)
	api.GET("/series/:id/next", handlers.Series.NextEpisode)
	api.GET("/series/:id/poster", handlers.Series.Poster)
	api.GET("/series/:id/backdrop", handlers.Series.Backdrop)
	api.GET("/series/:id/persons", handlers.Series.GetPersons)

	// Lite 的常规播放信息接口直接携带 playback_plan，客户端一次请求即可
	// 得到旧字段和服务端决策。独立 /plan 仍作为诊断与显式重规划接口保留。
	api.GET("/stream/:id/info", guardByMediaID, playbackPlan.GetInfo)
	api.GET("/stream/:id/plan", guardByMediaID, playbackPlan.Get)
	api.GET("/stream/:id/direct", guardByMediaID, handlers.Stream.Direct)
	api.GET("/stream/:id/remux", guardByMediaID, handlers.Stream.Remux)
	api.GET("/stream/:id/master.m3u8", guardByMediaID, handlers.Stream.Master)
	api.GET("/stream/:id/strm-seg", guardByMediaID, handlers.Stream.STRMSegment)
	api.GET("/stream/:id/strm-check", guardByMediaID, handlers.Stream.STRMCheck)
	api.POST("/stream/:id/playback", guardByMediaID, handlers.Stream.RetiredRuntimeHLS)
	api.POST("/stream/:id/bandwidth", guardByMediaID, handlers.Stream.RetiredRuntimeHLS)
	api.GET("/stream/:id/throttle", guardByMediaID, handlers.Stream.RetiredRuntimeHLS)
	api.GET("/stream/:id/:quality/:segment", guardByMediaID, handlers.Stream.Segment)
	api.GET("/audio-track/:id/:trackIdx", guardByMediaID, handlers.Stream.AudioPlaylist)
	api.GET("/audio-track/:id/:trackIdx/:seg", guardByMediaID, handlers.Stream.AudioSegment)
	api.GET("/media/:id/poster", handlers.Stream.Poster)

	// Runtime transcode is session-scoped. These routes never resolve a
	// persistent Artifact and every playlist/segment read holds a Reader Lease.
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

	api.POST("/stats/playback", handlers.Stats.RecordPlayback)
	api.GET("/stats/me", handlers.Stats.GetUserStats)
	api.POST("/logs/playback-error", handlers.SystemLog.ReportPlaybackError)

	api.GET("/media/:id/collection", handlers.Collection.GetMediaCollection)
	api.GET("/collections", handlers.Collection.ListCollections)
	api.GET("/collections/search", handlers.Collection.SearchCollections)
	api.GET("/collections/:id", handlers.Collection.GetCollectionDetail)
	api.GET("/collections/:id/poster", handlers.Collection.Poster)
}
