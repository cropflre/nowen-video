package emby

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 把 Emby 兼容层的所有路由挂到给定的 gin.Engine 上。
//
// 为什么不用一个单纯的 RouterGroup？因为 Emby/Infuse 客户端存在两种调用前缀：
//   - `/emby/System/Info`（大多数客户端默认）
//   - `/System/Info`（旧版 / 某些 UI 配置）
//
// 这里在根路径下挂一对前缀并应用统一中间件；公开接口单独列出不走 JWTAuth。
//
// 中间件顺序：EmbyCORS → 公开路径白名单 → EmbyAuth。
func RegisterRoutes(r *gin.Engine, h *Handler, jwtSecret string) {
	// CORS 在整个子路由上统一开启（与全局 CORS 并行不冲突）
	corsMW := EmbyCORS()

	// WebSocket 集线器（在所有前缀下共享）
	var wsHub *WSHub
	if h.cfg.Emby.EnableWebSocket {
		wsHub = NewWSHub(h.logger)
	}

	for _, prefix := range []string{"", "/emby"} {
		mount := func(group *gin.RouterGroup) {
			registerEmbyPublic(group, h)

			// 需要认证的路由
			secured := group.Group("", EmbyAuth(jwtSecret))
			registerEmbyAuthed(secured, h)

			// WebSocket（路径固定 /embywebsocket 与 /socket，自带 query token 鉴权）
			if wsHub != nil {
				group.GET("/embywebsocket", wsHub.Handler(jwtSecret))
				group.GET("/socket", wsHub.Handler(jwtSecret))
			}
		}

		if prefix == "" {
			grp := r.Group("", corsMW, notUnderEmbyAPIGuard())
			mount(grp)
		} else {
			grp := r.Group(prefix, corsMW)
			mount(grp)
		}
	}
}

// notUnderEmbyAPIGuard 阻止根路径的 Emby 路由抢占现有 /api/* 的路由。
// 所有 Emby 端点都在 registerEmbyPublic/registerEmbyAuthed 中显式注册，
// 与项目现有的 /api/* 路由不会冲突。
func notUnderEmbyAPIGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// registerEmbyPublic 注册无需认证的 Emby 端点。
func registerEmbyPublic(g *gin.RouterGroup, h *Handler) {
	// System / Ping：官方客户端既可能使用 PascalCase，也可能使用 lowercase。
	for _, path := range []string{"/System/Ping", "/system/ping"} {
		g.GET(path, h.PingHandler)
		g.HEAD(path, h.PingHandler)
		g.POST(path, h.PingHandler)
	}
	g.GET("/System/Info/Public", h.SystemInfoPublicHandler)
	g.GET("/system/info/public", h.SystemInfoPublicHandler)

	// 用户登录
	g.POST("/Users/AuthenticateByName", h.AuthenticateByNameHandler)
	g.POST("/users/authenticatebyname", h.AuthenticateByNameHandler)
	g.GET("/Users/Public", h.PublicUsersHandler)
	g.GET("/users/public", h.PublicUsersHandler)

	// 官方客户端辅助探测端点。必须显式返回 Emby 期望的 JSON/二进制，
	// 避免未匹配请求落到 nowen-video 前端 index.html。
	g.GET("/System/WakeOnLanInfo", h.WakeOnLanInfoHandler)
	g.GET("/system/wakeonlaninfo", h.WakeOnLanInfoHandler)
	g.GET("/Playback/BitrateTest", h.BitrateTestHandler)
	g.GET("/playback/bitratetest", h.BitrateTestHandler)
	g.GET("/web/manifest.json", h.WebManifestHandler)
	g.GET("/Web/manifest.json", h.WebManifestHandler)

	// Branding / Localization / QuickConnect
	// 这些端点不涉及隐私且被 Jellyfin/Emby Web 客户端在登录页无条件拉取，因此对外公开。
	g.GET("/Branding/Configuration", h.BrandingConfigHandler)
	g.GET("/Branding/Css", h.BrandingCssHandler)
	g.GET("/Branding/Css.css", h.BrandingCssHandler)
	g.GET("/Localization/Cultures", h.LocalizationCulturesHandler)
	g.GET("/Localization/Countries", h.LocalizationCountriesHandler)
	g.GET("/Localization/Options", h.LocalizationOptionsHandler)
	g.GET("/Localization/ParentalRatings", h.LocalizationParentalRatingsHandler)
	g.GET("/QuickConnect/Enabled", h.QuickConnectEnabledHandler)

	// 图片端点允许无 token（Infuse 有时从缓存的 URL 拉图时会丢失 token）
	g.GET("/Items/:id/Images/:type", h.ImageHandler)
	g.GET("/Items/:id/Images/:type/:index", h.ImageHandler)
	g.HEAD("/Items/:id/Images/:type", h.ImageHandler)
	g.HEAD("/Items/:id/Images/:type/:index", h.ImageHandler)

	// 预检（OPTIONS）由 EmbyCORS 中间件统一 204 短路，不再在此注册 catch-all。
	// 注册 `/*path` 会与 gin 引擎已有的静态/参数路由发生冲突并 panic。
}

// registerEmbyAuthed 注册需要 Emby token 的端点。
func registerEmbyAuthed(g *gin.RouterGroup, h *Handler) {
	// System（已登录视角）
	g.GET("/System/Info", h.SystemInfoHandler)
	g.GET("/system/info", h.SystemInfoHandler)
	g.GET("/System/Endpoint", h.SystemEndpointHandler)
	g.GET("/system/endpoint", h.SystemEndpointHandler)
	g.GET("/System/Configuration", h.SystemConfigurationHandler)
	g.GET("/system/configuration", h.SystemConfigurationHandler)

	// 用户
	g.POST("/Sessions/Logout", h.LogoutHandler)
	g.POST("/sessions/logout", h.LogoutHandler)
	g.GET("/Users/Me", h.GetCurrentUserHandler)
	g.GET("/users/me", h.GetCurrentUserHandler)
	g.GET("/Users", h.ListUsersHandler)
	g.GET("/users", h.ListUsersHandler)
	g.GET("/Users/:userId", h.GetUserByIDHandler)
	g.GET("/users/:userId", h.GetUserByIDHandler)
	g.GET("/Users/:userId/Views", h.UserViewsHandler)
	g.GET("/users/:userId/views", h.UserViewsHandler)
	g.GET("/Library/MediaFolders", h.MediaFoldersHandler)
	g.GET("/library/mediafolders", h.MediaFoldersHandler)

	// Items
	g.GET("/Items", h.ItemsHandler)
	g.GET("/items", h.ItemsHandler)
	g.GET("/Users/:userId/Items", h.ItemsHandler)
	g.GET("/users/:userId/items", h.ItemsHandler)
	// 字面量路由必须放在 /Items/:id 前，避免官方客户端的 /Items/resume 被 :id 截获。
	g.GET("/Items/Prefixes", h.ItemPrefixesHandler)
	g.GET("/items/prefixes", h.ItemPrefixesHandler)
	g.GET("/Users/:userId/Items/Latest", h.LatestItemsHandler)
	g.GET("/users/:userId/items/latest", h.LatestItemsHandler)
	g.GET("/Users/:userId/Items/latest", h.LatestItemsHandler)
	g.GET("/Users/:userId/Items/Resume", h.ResumeHandler)
	g.GET("/Users/:userId/Items/resume", h.ResumeHandler)
	g.GET("/users/:userId/items/resume", h.ResumeHandler)
	g.GET("/Users/:userId/Items/:id", wrapItemIDParam(h.ItemHandler))
	g.GET("/users/:userId/items/:id", wrapItemIDParam(h.ItemHandler))
	g.GET("/Items/:id/Similar", h.SimilarItemsHandler)
	g.GET("/items/:id/similar", h.SimilarItemsHandler)
	g.GET("/Items/:id", h.ItemHandler)
	g.GET("/items/:id", h.ItemHandler)
	g.GET("/Genres", h.GenresHandler)

	// Shows / Seasons / Episodes
	g.GET("/Shows/:seriesId/Seasons", h.SeasonsHandler)
	g.GET("/Shows/:seriesId/Episodes", h.EpisodesHandler)
	g.GET("/Shows/NextUp", h.NextUpHandler)

	// Playback
	g.GET("/Items/:id/PlaybackInfo", h.PlaybackInfoHandler)
	g.GET("/items/:id/playbackinfo", h.PlaybackInfoHandler)
	g.POST("/Items/:id/PlaybackInfo", h.PlaybackInfoHandler)
	g.POST("/items/:id/playbackinfo", h.PlaybackInfoHandler)

	// Video 流（:id 是 emby 数字 id）
	g.GET("/Videos/:id/stream", h.StreamVideoHandler)
	g.HEAD("/Videos/:id/stream", h.StreamVideoHandler)
	g.GET("/videos/:id/stream", h.StreamVideoHandler)
	g.HEAD("/videos/:id/stream", h.StreamVideoHandler)
	g.GET("/Videos/:id/stream.:container", h.StreamVideoHandler)
	g.HEAD("/Videos/:id/stream.:container", h.StreamVideoHandler)
	g.GET("/videos/:id/stream.:container", h.StreamVideoHandler)
	g.HEAD("/videos/:id/stream.:container", h.StreamVideoHandler)
	g.GET("/Videos/:id/original", h.OriginalVideoHandler)
	g.GET("/Videos/:id/original.:container", h.OriginalVideoHandler)
	g.GET("/Videos/:id/master.m3u8", h.HLSMasterHandler)
	g.HEAD("/Videos/:id/master.m3u8", h.HLSMasterHandler)
	g.GET("/videos/:id/master.m3u8", h.HLSMasterHandler)
	g.HEAD("/videos/:id/master.m3u8", h.HLSMasterHandler)
	g.GET("/Videos/:id/main.m3u8", h.HLSMasterHandler)
	g.HEAD("/Videos/:id/main.m3u8", h.HLSMasterHandler)
	g.GET("/videos/:id/main.m3u8", h.HLSMasterHandler)
	g.HEAD("/videos/:id/main.m3u8", h.HLSMasterHandler)
	g.GET("/Videos/:id/hls1/:quality/main.m3u8", h.HLSPlaylistHandler)
	g.HEAD("/Videos/:id/hls1/:quality/main.m3u8", h.HLSPlaylistHandler)
	g.GET("/videos/:id/hls1/:quality/main.m3u8", h.HLSPlaylistHandler)
	g.HEAD("/videos/:id/hls1/:quality/main.m3u8", h.HLSPlaylistHandler)
	g.GET("/Videos/:id/hls1/:quality/:segment", h.HLSSegmentHandler)
	g.HEAD("/Videos/:id/hls1/:quality/:segment", h.HLSSegmentHandler)
	g.GET("/videos/:id/hls1/:quality/:segment", h.HLSSegmentHandler)
	g.HEAD("/videos/:id/hls1/:quality/:segment", h.HLSSegmentHandler)

	// 字幕
	g.GET("/Videos/:id/:sourceId/Subtitles/:index/Stream.:ext", h.SubtitleStreamHandler)
	g.GET("/Videos/:id/Subtitles/:index/Stream.:ext", h.SubtitleStreamHandlerNoSource)

	// 播放会话上报
	g.POST("/Sessions/Playing", h.PlayingStartHandler)
	g.POST("/Sessions/Playing/Progress", h.PlayingProgressHandler)
	g.POST("/Sessions/Playing/Stopped", h.PlayingStoppedHandler)
	g.GET("/Users/:userId/PlayingItems/:itemId", h.PlayingGetStartHandler)
	g.POST("/Users/:userId/PlayingItems/:itemId", h.PlayingGetStartHandler)
	g.POST("/Users/:userId/PlayingItems/:itemId/Progress", h.PlayingGetProgressHandler)
	g.DELETE("/Users/:userId/PlayingItems/:itemId", h.PlayingGetStoppedHandler)

	// 收藏 / 已播放
	g.POST("/Users/:userId/FavoriteItems/:itemId", h.AddFavoriteHandler)
	g.DELETE("/Users/:userId/FavoriteItems/:itemId", h.RemoveFavoriteHandler)
	g.POST("/Users/:userId/PlayedItems/:itemId", h.MarkPlayedHandler)
	g.DELETE("/Users/:userId/PlayedItems/:itemId", h.MarkUnplayedHandler)

	// Sessions 列表。官方 Emby 客户端登录后会用 DeviceId 查询当前会话；返回空数组会导致初始化失败。
	g.GET("/Sessions", h.SessionsHandler)
	g.GET("/sessions", h.SessionsHandler)

	// Displayable preferences（Emby 客户端保存 UI 设置用；简化：echo 回去）
	g.GET("/DisplayPreferences/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"Id": c.Param("id"), "CustomPrefs": gin.H{}})
	})
	g.POST("/DisplayPreferences/:id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
}

// SubtitleStreamHandlerNoSource 兼容无 sourceId 变体。
func (h *Handler) SubtitleStreamHandlerNoSource(c *gin.Context) {
	h.SubtitleStreamHandler(c)
}

// wrapItemIDParam 允许 "/Users/:userId/Items/:id" 的 :id 参数映射到 handler 中的 c.Param("id")。
// gin 的参数绑定默认使用路径中声明的名字，这里已经一致（两处都用 :id），无需额外处理。
// 保留这个 wrapper 以便将来需要特殊处理（例如把 :id = "Latest"/"Resume" 字面量路由区分）。
func wrapItemIDParam(h gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		switch strings.ToLower(id) {
		case "latest", "resume":
			// 这些字面量路径由专门的 handler 处理，直接跳过
			c.Status(http.StatusNotFound)
			return
		}
		h(c)
	}
}
