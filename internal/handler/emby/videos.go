package emby

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// ==================== Video 流接口 ====================
//
// Infuse / Emby 客户端真正发起播放时会对下列 URL 发 GET（或 HEAD）请求：
//
//   GET /emby/Videos/{itemId}/stream                     -- 直通/Remux
//   GET /emby/Videos/{itemId}/stream.{container}         -- 同上（部分客户端加扩展名）
//   GET /emby/Videos/{itemId}/master.m3u8                -- HLS master
//   GET /emby/Videos/{itemId}/hls1/main/{segment}.ts     -- HLS 分片
//
// 为了兼容多种客户端，以下三种变体均映射到同一套 handler。
// 认证方式：
//   - X-Emby-Token 头（iOS/Android Emby App）
//   - api_key query（Infuse 会把 token 直接拼到 URL 查询参数上）
// 这两种都由 EmbyAuth 中间件统一处理。

// StreamVideoHandler 对应 GET/HEAD /Videos/{id}/stream(.{container})?...
func (h *Handler) StreamVideoHandler(c *gin.Context) {
	embyID := c.Param("id")
	uuid := h.idMap.Resolve(embyID)
	if uuid == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Item not found"})
		return
	}

	m, err := h.mediaRepo.FindByID(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Media not found"})
		return
	}

	if strings.TrimSpace(m.StreamURL) != "" {
		if err := h.stream.ProxyRemoteStream(m.StreamURL, c.Writer, c.Request); err != nil {
			h.logger.Warnf("[emby] proxy remote stream failed media=%s err=%v", uuid, err)
			if !c.Writer.Written() {
				c.JSON(http.StatusBadGateway, gin.H{"Error": "Upstream failed"})
			}
		}
		return
	}

	filePath := m.FilePath
	if filePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "File path not configured"})
		return
	}
	fi, err := os.Stat(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "File not found"})
		return
	}
	if fi.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Path is a directory"})
		return
	}

	container := strings.ToLower(containerFromPath(filePath))
	ua := c.Request.Header.Get("User-Agent")
	wantRemux := c.Query("Static") == "false" || c.Query("TranscodingProtocol") == "hls"
	canRemux := h.stream.ShouldRemux(m, ua)

	if wantRemux || canRemux {
		if err := h.stream.ManagedRemuxStream(uuid, c.Writer, c.Request); err != nil {
			h.logger.Warnf("[emby] managed remux failed media=%s err=%v, fallback to direct serve", uuid, err)
			if c.Writer.Written() {
				return
			}
		} else {
			return
		}
	}

	mime := mimeFromContainer(container)
	c.Header("Content-Type", mime)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "private, max-age=3600")
	f, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"Error": "Failed to open file"})
		return
	}
	defer f.Close()
	http.ServeContent(c.Writer, c.Request, fi.Name(), fi.ModTime(), f)
}

func (h *Handler) OriginalVideoHandler(c *gin.Context) {
	h.StreamVideoHandler(c)
}

func (h *Handler) HLSMasterHandler(c *gin.Context) {
	embyID := c.Param("id")
	uuid := h.idMap.Resolve(embyID)
	if uuid == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Item not found"})
		return
	}
	maxBitrate := 0
	if v := c.Query("maxBitrate"); v != "" {
		maxBitrate = atoiSafe(v)
	}
	playlist, err := h.stream.GetMasterPlaylistFiltered(uuid, maxBitrate)
	if err != nil {
		h.logger.Warnf("[emby] master playlist failed media=%s err=%v", uuid, err)
		c.JSON(http.StatusNotFound, gin.H{"Error": "HLS not available"})
		return
	}
	playlist = rewriteMasterForEmby(playlist, embyID)
	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, playlist)
}

func rewriteMasterForEmby(playlist, embyID string) string {
	lines := strings.Split(playlist, "\n")
	for i, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "/api/stream/") && strings.HasSuffix(ln, "/stream.m3u8") {
			parts := strings.Split(ln, "/")
			if len(parts) >= 6 {
				quality := parts[4]
				lines[i] = fmt.Sprintf("/emby/Videos/%s/hls1/%s/main.m3u8", embyID, quality)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func (h *Handler) HLSPlaylistHandler(c *gin.Context) {
	embyID := c.Param("id")
	uuid := h.idMap.Resolve(embyID)
	if uuid == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Item not found"})
		return
	}
	quality := c.Param("quality")
	playlist, err := h.stream.GetSegmentPlaylist(uuid, quality)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Quality not available"})
		return
	}
	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, playlist)
}

func (h *Handler) HLSSegmentHandler(c *gin.Context) {
	embyID := c.Param("id")
	uuid := h.idMap.Resolve(embyID)
	if uuid == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Item not found"})
		return
	}
	quality := c.Param("quality")
	segment := c.Param("segment")
	if err := h.stream.ServeSegment(uuid, quality, segment, c.Writer, c.Request); err != nil {
		h.logger.Warnf("[emby] segment failed media=%s seg=%s err=%v", uuid, segment, err)
		if !c.Writer.Written() {
			c.JSON(http.StatusNotFound, gin.H{"Error": fmt.Sprintf("Segment failed: %v", err)})
		}
	}
}
