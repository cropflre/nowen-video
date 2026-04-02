package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// SubtitleHandler 字幕处理器
type SubtitleHandler struct {
	scanner       *service.ScannerService
	streamService *service.StreamService
	logger        *zap.SugaredLogger
}

// ListTracks 获取媒体的字幕轨道列表（内嵌 + 外挂）
func (h *SubtitleHandler) ListTracks(c *gin.Context) {
	id := c.Param("id")

	// 获取媒体文件路径
	filePath, _, err := h.streamService.GetDirectStreamInfo(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "媒体不存在"})
		return
	}

	// STRM 远程流不支持字幕提取
	if filePath == "__strm__" {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"embedded": []interface{}{},
				"external": []interface{}{},
			},
		})
		return
	}

	// 获取内嵌字幕
	embedded, err := h.scanner.GetSubtitleTracks(filePath)
	if err != nil {
		h.logger.Warnf("获取内嵌字幕失败: %v", err)
		embedded = nil
	}

	// 获取外挂字幕
	external := h.scanner.GetExternalSubtitles(filePath)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"embedded": embedded,
			"external": external,
		},
	})
}

// ExtractTrack 提取内嵌字幕为WebVTT格式
func (h *SubtitleHandler) ExtractTrack(c *gin.Context) {
	id := c.Param("id")
	indexStr := c.Param("index")

	streamIndex, err := strconv.Atoi(indexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的字幕轨道索引"})
		return
	}

	// 获取媒体文件路径
	filePath, _, err := h.streamService.GetDirectStreamInfo(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "媒体不存在"})
		return
	}

	// STRM 远程流不支持字幕提取
	if filePath == "__strm__" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "STRM 远程流不支持字幕提取"})
		return
	}
	tracks, err := h.scanner.GetSubtitleTracks(filePath)
	if err == nil {
		for _, track := range tracks {
			if track.Index == streamIndex && track.Bitmap {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "该字幕轨道为图形字幕（" + track.Codec + "），无法提取为文本格式",
				})
				return
			}
		}
	}

	// 提取字幕为WebVTT格式（浏览器原生支持）
	vttPath, err := h.scanner.ExtractSubtitle(filePath, streamIndex, "vtt")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "字幕提取失败: " + err.Error()})
		return
	}

	c.Header("Content-Type", "text/vtt; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=604800")
	c.File(vttPath)
}

// ServeExternal 提供外挂字幕文件（自动转换为WebVTT格式）
func (h *SubtitleHandler) ServeExternal(c *gin.Context) {
	// 外挂字幕路径通过query参数传入
	subPath := c.Query("path")
	if subPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少字幕路径"})
		return
	}

	// 安全检查：确保是字幕文件
	ext := service.GetFileExt(subPath)
	switch ext {
	case ".vtt":
		// VTT 格式浏览器原生支持，直接返回
		c.Header("Content-Type", "text/vtt; charset=utf-8")
		c.Header("Cache-Control", "public, max-age=604800")
		c.File(subPath)
		return
	case ".srt", ".ass", ".ssa":
		// 非VTT格式需要通过FFmpeg转换为WebVTT
		vttPath, err := h.scanner.ConvertSubtitleToVTT(subPath)
		if err != nil {
			h.logger.Warnf("字幕转换失败，尝试直接返回原始文件: %v", err)
			// 转换失败时回退到直接返回原始文件（部分播放器可能支持）
			c.Header("Content-Type", "text/plain; charset=utf-8")
			c.Header("Cache-Control", "public, max-age=604800")
			c.File(subPath)
			return
		}
		c.Header("Content-Type", "text/vtt; charset=utf-8")
		c.Header("Cache-Control", "public, max-age=604800")
		c.File(vttPath)
		return
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的字幕格式"})
		return
	}
}
