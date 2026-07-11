package emby

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// WakeOnLanInfoHandler 对应 /System/WakeOnLanInfo。
// 官方 Emby Android/TV 客户端会把该接口按数组解析；没有 WOL 设备时返回空数组即可。
// 未显式注册时，请求会落到 nowen-video 的 SPA fallback，客户端拿到 HTML 后会判定连接失败。
func (h *Handler) WakeOnLanInfoHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{})
}

// BitrateTestHandler 对应 /Playback/BitrateTest?Size=...。
// 官方客户端使用该接口做带宽探测，响应必须是指定长度的二进制数据，而不是前端 HTML。
func (h *Handler) BitrateTestHandler(c *gin.Context) {
	size := atoiSafe(c.Query("Size"))
	if size <= 0 {
		size = atoiSafe(c.Query("size"))
	}
	if size <= 0 {
		size = 1024
	}

	// 官方客户端常用约 5MB；限制到 10MB，避免恶意参数造成过量内存分配。
	if size > 10*1024*1024 {
		size = 10 * 1024 * 1024
	}

	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/octet-stream", make([]byte, size))
}

// WebManifestHandler 对应 /web/manifest.json。
// 返回最小可解析的 Emby manifest，防止客户端内置 WebView 请求落到 nowen-video 的 index.html。
func (h *Handler) WebManifestHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name":             embyProductName,
		"short_name":       "Emby",
		"start_url":        "/emby/web/index.html",
		"display":          "standalone",
		"background_color": "#101010",
		"theme_color":      "#52b54b",
		"icons":            []gin.H{},
	})
}
