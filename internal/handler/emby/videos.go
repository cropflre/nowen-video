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
//   GET /emby/Videos/{itemId}/stream
//   GET /emby/Videos/{itemId}/stream.{container}
//   GET /emby/Videos/{itemId}/master.m3u8
//   GET /emby/Videos/{itemId}/hls1/main/{segment}.ts

func (h *Handler) StreamVideoHandler(c *gin.Context) {
	embyID := c.Param("id")
	uuid := h.idMap.Resolve(embyID)
	if uuid == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Item not found"})
		return
	}

	media, err := h.mediaRepo.FindByID(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Media not found"})
		return
	}

	if strings.TrimSpace(media.StreamURL) != "" {
		if err := h.stream.ProxyRemoteStream(media.StreamURL, c.Writer, c.Request); err != nil {
			h.logger.Warnf("[emby] proxy remote stream failed media=%s err=%v", uuid, err)
			if !c.Writer.Written() {
				c.JSON(http.StatusBadGateway, gin.H{"Error": "Upstream failed"})
			}
		}
		return
	}

	filePath := media.FilePath
	if filePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "File path not configured"})
		return
	}
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "File not found"})
		return
	}
	if fileInfo.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Path is a directory"})
		return
	}

	container := strings.ToLower(containerFromPath(filePath))
	userAgent := c.Request.Header.Get("User-Agent")
	wantRemux := c.Query("Static") == "false" || c.Query("TranscodingProtocol") == "hls"
	legacyRemux := h.stream.ShouldRemux(media, userAgent)
	_, managedRemux, _ := h.stream.CanManagedRemuxByID(uuid)

	if wantRemux || legacyRemux || managedRemux {
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
	file, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"Error": "Failed to open file"})
		return
	}
	defer file.Close()
	http.ServeContent(c.Writer, c.Request, fileInfo.Name(), fileInfo.ModTime(), file)
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

	// Full runtime HLS is ephemeral and strictly bound to the external Emby
	// PlaySessionId. The returned master contains one current Generation; seek
	// or profile changes create a new Generation instead of sharing media-level
	// output directories between clients.
	if runtime := h.playbackSessionRuntime(); runtime != nil {
		externalID, startPositionMS, hasStart, maxBitrate := parseEmbyPlaybackRequest(c)
		mapping, err := runtime.ensure(
			c.Request.Context(),
			c.GetString("user_id"),
			uuid,
			externalID,
			startPositionMS,
			hasStart,
			maxBitrate,
		)
		if err != nil {
			h.logger.Warnf("[emby] playback session start failed media=%s err=%v", uuid, err)
			c.JSON(http.StatusBadGateway, gin.H{"Error": "Runtime transcode unavailable"})
			return
		}
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
		c.Header("Cache-Control", "private, no-store")
		c.Header("Pragma", "no-cache")
		c.String(http.StatusOK, buildEmbySessionMaster(c, embyID, mapping))
		return
	}

	// Compatibility fallback for deployments that have not mounted the new
	// PlaybackSessionService yet.
	maxBitrate := 0
	if value := c.Query("maxBitrate"); value != "" {
		maxBitrate = atoiSafe(value)
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
	for index, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/api/stream/") && strings.HasSuffix(line, "/stream.m3u8") {
			parts := strings.Split(line, "/")
			if len(parts) >= 6 {
				quality := parts[4]
				lines[index] = fmt.Sprintf("/emby/Videos/%s/hls1/%s/main.m3u8", embyID, quality)
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
