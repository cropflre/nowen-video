package emby

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ==================== 字幕 ====================
//
// Emby 客户端按以下 URL 拉取外挂字幕：
//
//   GET /Videos/{itemId}/{sourceId}/Subtitles/{streamIndex}/Stream.{format}
//   GET /Videos/{itemId}/Subtitles/{streamIndex}/Stream.{format}
//
// streamIndex 来自 MediaStream.Index（视频/音频流之后的编号）。
// 我们的 `helpers.buildMediaStreams` 从 `media.SubtitlePaths`（| 分隔）
// 构造 subtitle stream，index 从 2 开始。
//
// 简化策略：直接按文件原样返回，让客户端自己解析 srt/ass/vtt。
// 如需 Emby 那样的 on-the-fly 转码（ass→vtt），可在此扩展。

// SubtitleStreamHandler 对应 /Videos/{id}/.../Subtitles/{index}/Stream.{ext}
func (h *Handler) SubtitleStreamHandler(c *gin.Context) {
	embyID := c.Param("id")
	uuid := h.idMap.Resolve(embyID)
	if uuid == "" {
		c.Status(http.StatusNotFound)
		return
	}
	m, err := h.mediaRepo.FindByID(uuid)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if m.SubtitlePaths == "" {
		c.Status(http.StatusNotFound)
		return
	}

	// 解析 streamIndex；MediaStreams 前两条是视频+音频，字幕从 index=2 起
	idx, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	subIdx := idx - 2
	if subIdx < 0 {
		c.Status(http.StatusNotFound)
		return
	}
	paths := splitPaths(m.SubtitlePaths)
	if subIdx >= len(paths) {
		c.Status(http.StatusNotFound)
		return
	}
	subPath := paths[subIdx]

	fi, err := os.Stat(subPath)
	if err != nil || fi.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(subPath), "."))
	switch ext {
	case "vtt":
		c.Header("Content-Type", "text/vtt; charset=utf-8")
	case "srt":
		c.Header("Content-Type", "application/x-subrip; charset=utf-8")
	case "ass", "ssa":
		c.Header("Content-Type", "text/x-ssa; charset=utf-8")
	default:
		c.Header("Content-Type", "application/octet-stream")
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.File(subPath)
}
