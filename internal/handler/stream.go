package handler

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// StreamHandler 流媒体处理器
type StreamHandler struct {
	streamService *service.StreamService
	logger        *zap.SugaredLogger
}

// Direct 直接提供原始文件流（支持Range请求，用于MP4等浏览器兼容格式）
func (h *StreamHandler) Direct(c *gin.Context) {
	id := c.Param("id")
	filePath, contentType, err := h.streamService.GetDirectStreamInfo(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 使用http.ServeFile自动处理Range请求（断点续播、拖动进度条）
	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "public, max-age=86400")
	http.ServeFile(c.Writer, c.Request, filePath)
}

// Master 获取HLS主播放列表
func (h *StreamHandler) Master(c *gin.Context) {
	id := c.Param("id")
	playlist, err := h.streamService.GetMasterPlaylist(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, playlist)
}

// Segment 提供HLS分片或子播放列表
func (h *StreamHandler) Segment(c *gin.Context) {
	id := c.Param("id")
	quality := c.Param("quality")
	segment := c.Param("segment")

	// 如果请求的是子m3u8播放列表
	if segment == "stream.m3u8" {
		playlist, err := h.streamService.GetSegmentPlaylist(id, quality)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
		c.Header("Cache-Control", "no-cache")
		c.String(http.StatusOK, playlist)
		return
	}

	// 提供.ts分片文件
	if err := h.streamService.ServeSegment(id, quality, segment, c.Writer, c.Request); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
}

// MediaInfo 获取媒体的播放信息（前端用于决定播放模式）
func (h *StreamHandler) MediaInfo(c *gin.Context) {
	id := c.Param("id")
	info, err := h.streamService.GetMediaPlayInfo(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": info})
}

// Poster 提供媒体海报/缩略图
func (h *StreamHandler) Poster(c *gin.Context) {
	id := c.Param("id")
	posterPath, err := h.streamService.GetPosterPath(id)
	if err != nil || posterPath == "" {
		// 返回默认占位图
		c.Header("Content-Type", "image/svg+xml")
		c.String(http.StatusOK, `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="450" viewBox="0 0 300 450"><rect fill="#1e1e2e" width="300" height="450"/><text fill="#666" font-family="sans-serif" font-size="14" text-anchor="middle" x="150" y="225">No Poster</text></svg>`)
		return
	}

	ext := strings.ToLower(filepath.Ext(posterPath))
	switch ext {
	case ".jpg", ".jpeg":
		c.Header("Content-Type", "image/jpeg")
	case ".png":
		c.Header("Content-Type", "image/png")
	case ".webp":
		c.Header("Content-Type", "image/webp")
	default:
		c.Header("Content-Type", "application/octet-stream")
	}

	c.Header("Cache-Control", "public, max-age=604800") // 缓存7天
	c.File(posterPath)
}
