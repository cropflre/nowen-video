package handler

import (
	"fmt"
	"net/http"
	"os"
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
// 对于 STRM 远程流，通过后端代理转发
// 对于 WebDAV 路径（webdav://），通过 VFS 打开并使用 http.ServeContent（支持 Range）
func (h *StreamHandler) Direct(c *gin.Context) {
	id := c.Param("id")
	filePath, contentType, err := h.streamService.GetDirectStreamInfo(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if filePath == "__strm__" {
		h.logger.Debugf("STRM 代理播放: %s", id)
		if err := h.streamService.ProxyRemoteStreamForMedia(id, c.Writer, c.Request); err != nil {
			h.logger.Warnf("STRM 代理播放失败: %s, 错误: %v", id, err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "远程流播放失败: " + err.Error()})
		}
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "public, max-age=86400")

	if service.IsWebDAVPath(filePath) {
		vfsFile, err := h.streamService.OpenMediaFile(filePath)
		if err != nil {
			h.logger.Warnf("WebDAV 打开文件失败: %s, 错误: %v", filePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "打开远程文件失败: " + err.Error()})
			return
		}
		defer vfsFile.Close()

		stat, err := vfsFile.Stat()
		if err != nil {
			h.logger.Warnf("WebDAV 获取文件信息失败: %s, 错误: %v", filePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取远程文件信息失败: " + err.Error()})
			return
		}

		reader := service.NewVFSReadSeeker(vfsFile, stat.Size())
		http.ServeContent(c.Writer, c.Request, filepath.Base(filePath), stat.ModTime(), reader)
		return
	}

	http.ServeFile(c.Writer, c.Request, filePath)
}

// Remux 统一进入受资源治理的 Managed Remux。兼容音频直接 copy，
// DTS/TrueHD/FLAC/Opus 等不兼容音频只转 AAC，视频保持 bit-for-bit copy。
func (h *StreamHandler) Remux(c *gin.Context) {
	id := c.Param("id")
	h.logger.Debugf("Managed Remux 播放请求: %s", id)
	if err := h.streamService.ManagedRemuxStream(id, c.Writer, c.Request); err != nil {
		h.logger.Warnf("Managed Remux 播放失败: %s, 错误: %v", id, err)
		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Remux 播放失败: " + err.Error()})
		}
	}
}

// MediaInfo 获取媒体的播放信息。Full 旧入口在这里补充 Smart Remux 能力；
// Lite /info 则由 Playback Planner 返回同一决策。
func (h *StreamHandler) MediaInfo(c *gin.Context) {
	id := c.Param("id")
	info, err := h.streamService.GetMediaPlayInfo(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if !info.CanDirectPlay && !info.CanRemux {
		if _, supported, capabilityErr := h.streamService.CanManagedRemuxByID(id); capabilityErr == nil && supported {
			info.CanRemux = true
			info.RemuxURL = fmt.Sprintf("/api/stream/%s/remux", id)
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": info})
}

const posterPlaceholderSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="450" viewBox="0 0 300 450">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stop-color="#1a1b2e"/>
      <stop offset="100%" stop-color="#0f1019"/>
    </linearGradient>
    <linearGradient id="icon" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0%" stop-color="#3b82f6" stop-opacity="0.4"/>
      <stop offset="100%" stop-color="#8b5cf6" stop-opacity="0.25"/>
    </linearGradient>
  </defs>
  <rect fill="url(#bg)" width="300" height="450" rx="0"/>
  <rect x="0" y="0" width="300" height="450" fill="url(#icon)" opacity="0.08"/>
  <g transform="translate(150,200)" fill="none" stroke="#4a5568" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.5">
    <rect x="-24" y="-18" width="48" height="36" rx="3"/>
    <path d="M-24,-10 L-16,-10 M-24,-2 L-16,-2 M-24,6 L-16,6"/>
    <path d="M24,-10 L16,-10 M24,-2 L16,-2 M24,6 L16,6"/>
    <circle cx="-4" cy="0" r="6"/>
    <circle cx="10" cy="0" r="3"/>
  </g>
  <text fill="#4a5568" font-family="-apple-system,BlinkMacSystemFont,sans-serif" font-size="12" font-weight="500" text-anchor="middle" x="150" y="248">暂无海报</text>
</svg>`

func (h *StreamHandler) Poster(c *gin.Context) {
	id := c.Param("id")
	posterPath, err := h.streamService.GetPosterPath(id)
	if err != nil || posterPath == "" {
		c.Header("Content-Type", "image/svg+xml")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("X-Poster-Placeholder", "true")
		c.String(http.StatusOK, posterPlaceholderSVG)
		return
	}

	if service.IsWebDAVPath(posterPath) {
		vfsFile, openErr := h.streamService.OpenMediaFile(posterPath)
		if openErr != nil {
			c.Header("Content-Type", "image/svg+xml")
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("X-Poster-Placeholder", "true")
			c.String(http.StatusOK, posterPlaceholderSVG)
			return
		}
		defer vfsFile.Close()

		stat, statErr := vfsFile.Stat()
		if statErr != nil {
			c.Header("Content-Type", "image/svg+xml")
			c.Header("X-Poster-Placeholder", "true")
			c.String(http.StatusOK, posterPlaceholderSVG)
			return
		}
		etag := fmt.Sprintf(`"%x-%x"`, stat.ModTime().UnixNano(), stat.Size())
		c.Header("ETag", etag)
		if match := c.GetHeader("If-None-Match"); match == etag {
			c.Status(http.StatusNotModified)
			return
		}
		setPosterContentType(c, posterPath)
		c.Header("Cache-Control", "public, max-age=86400, must-revalidate")
		reader := service.NewVFSReadSeeker(vfsFile, stat.Size())
		http.ServeContent(c.Writer, c.Request, filepath.Base(posterPath), stat.ModTime(), reader)
		return
	}

	fileInfo, statErr := os.Stat(posterPath)
	if statErr != nil {
		c.Header("Content-Type", "image/svg+xml")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("X-Poster-Placeholder", "true")
		c.String(http.StatusOK, posterPlaceholderSVG)
		return
	}

	etag := fmt.Sprintf(`"%x-%x"`, fileInfo.ModTime().UnixNano(), fileInfo.Size())
	c.Header("ETag", etag)
	if match := c.GetHeader("If-None-Match"); match == etag {
		c.Status(http.StatusNotModified)
		return
	}

	setPosterContentType(c, posterPath)
	c.Header("Cache-Control", "public, max-age=86400, must-revalidate")
	c.File(posterPath)
}

func setPosterContentType(c *gin.Context, posterPath string) {
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
}

func (h *StreamHandler) STRMSegment(c *gin.Context) {
	id := c.Param("id")
	target := c.Query("u")
	if target == "" {
		target = c.Query("target")
	}
	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing target url"})
		return
	}
	if err := h.streamService.ProxySTRMSegment(id, target, c.Writer, c.Request); err != nil {
		h.logger.Warnf("STRM 子资源代理失败: %s, 错误: %v", id, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "远程子资源拉取失败: " + err.Error()})
	}
}

func (h *StreamHandler) STRMCheck(c *gin.Context) {
	id := c.Param("id")
	result, err := h.streamService.CheckSTRMHealth(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
