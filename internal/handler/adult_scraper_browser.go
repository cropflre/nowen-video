package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

// BrowserMediaStatus reports whether a local video path already has NFO metadata.
func (h *AdultScraperHandler) BrowserMediaStatus(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = c.Query("video_path")
	}
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 path 参数"})
		return
	}

	status, err := h.scraperService.CheckBrowserMediaStatus(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": status})
}

// ImportBrowserMetadata writes metadata captured by the Chrome extension to a local media file.
func (h *AdultScraperHandler) ImportBrowserMetadata(c *gin.Context) {
	var req struct {
		VideoPath    string                     `json:"video_path" binding:"required"`
		Metadata     service.AdultMetadata      `json:"metadata" binding:"required"`
		ShortReviews []service.JavDBShortReview `json:"short_reviews"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效，请提供 video_path 和 metadata"})
		return
	}

	result, err := h.scraperService.ImportBrowserMetadataToPath(req.VideoPath, &req.Metadata)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := gin.H{"data": result}
	if h.danmakuService != nil && len(req.ShortReviews) > 0 {
		mediaID, count, err := h.danmakuService.ImportJavDBReviewsForPath(req.VideoPath, req.ShortReviews)
		if err != nil {
			resp["danmaku_error"] = err.Error()
		} else {
			resp["danmaku"] = gin.H{
				"media_id": mediaID,
				"imported": count,
				"source":   "javdb",
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}
