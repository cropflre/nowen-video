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

	for _, prefix := range []string{"", "/emby"} {
		mount := func(group *gin.RouterGroup) {
			registerEmbyPublic(group, h)

			// 需要认证的路由
			secured := group.Group("", EmbyAuth(jwtSecret))
			registerEmbyAuthed(secured, h)
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
// 所有 Emby 端点都以大写开头（/System/ /Users/ /Items/ /Videos/ /Shows/ /Sessions/ 等），
// 与项目的 /api/* 不会冲突，但为了安全——若以后新增其他无前缀路由，该守卫不会误伤。
func notUnderEmbyAPIGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 所有 Emby 路径都在下面 registerEmbyPublic/Authed 中显式注册，
		// Gin 会先匹配精确路径，匹配不到才会走到 NoRoute，因此无需额外逻辑。
		c.Next()
	}
}

// registerEmbyPublic 注册无需认证的 Emby 端点。
func registerEmbyPublic(g *gin.RouterGroup, h *Handler) {
	// System / Ping
	g.GET("/System/Ping", h.PingHandler)
	g.HEAD("/System/Ping", h.PingHandler)
	g.POST("/System/Ping", h.PingHandler)
	g.GET("/System/Info/Public", h.SystemInfoPublicHandler)

	// 用户登录
	g.POST("/Users/AuthenticateByName", h.AuthenticateByNameHandler)
	g.GET("/Users/Public", h.PublicUsersHandler)

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
	g.GET("/System/Endpoint", h.SystemEndpointHandler)

	// 用户
	g.POST("/Sessions/Logout", h.LogoutHandler)
	g.GET("/Users/Me", h.GetCurrentUserHandler)
	g.GET("/Users", h.ListUsersHandler)
	g.GET("/Users/:userId", h.GetUserByIDHandler)
	g.GET("/Users/:userId/Views", h.UserViewsHandler)
	g.GET("/Library/MediaFolders", h.MediaFoldersHandler)

	// Items
	g.GET("/Items", h.ItemsHandler)
	g.GET("/Users/:userId/Items", h.ItemsHandler)
	g.GET("/Users/:userId/Items/:id", wrapItemIDParam(h.ItemHandler))
	g.GET("/Items/:id", h.ItemHandler)
	g.GET("/Items/:id/Similar", h.SimilarItemsHandler)
	g.GET("/Users/:userId/Items/Latest", h.LatestItemsHandler)
	g.GET("/Users/:userId/Items/Resume", h.ResumeHandler)
	g.GET("/Genres", h.GenresHandler)

	// Shows / Seasons / Episodes
	g.GET("/Shows/:seriesId/Seasons", h.SeasonsHandler)
	g.GET("/Shows/:seriesId/Episodes", h.EpisodesHandler)
	g.GET("/Shows/NextUp", h.NextUpHandler)

	// Playback
	g.GET("/Items/:id/PlaybackInfo", h.PlaybackInfoHandler)
	g.POST("/Items/:id/PlaybackInfo", h.PlaybackInfoHandler)

	// Video 流（:id 是 emby 数字 id）
	g.GET("/Videos/:id/stream", h.StreamVideoHandler)
	g.HEAD("/Videos/:id/stream", h.StreamVideoHandler)
	g.GET("/Videos/:id/stream.:container", h.StreamVideoHandler)
	g.HEAD("/Videos/:id/stream.:container", h.StreamVideoHandler)
	g.GET("/Videos/:id/original", h.OriginalVideoHandler)
	g.GET("/Videos/:id/original.:container", h.OriginalVideoHandler)
	g.GET("/Videos/:id/master.m3u8", h.HLSMasterHandler)
	g.GET("/Videos/:id/main.m3u8", h.HLSMasterHandler)
	g.GET("/Videos/:id/hls1/:quality/main.m3u8", h.HLSPlaylistHandler)
	g.GET("/Videos/:id/hls1/:quality/:segment", h.HLSSegmentHandler)

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

	// Sessions 列表（最小实现：返回空数组，Infuse 会忽略）
	g.GET("/Sessions", func(c *gin.Context) {
		c.JSON(http.StatusOK, []SessionInfo{})
	})

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
