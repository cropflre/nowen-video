package emby

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// ==================== Video 流接口 ====================
//
// Direct/Remux remain request-scoped. Full video transcoding is available only
// through the PlaySessionId -> PlaybackSession mapping created by master.m3u8.

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

	runtime := h.playbackSessionRuntime()
	if runtime == nil {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"Error":     "Playback session runtime unavailable",
			"ErrorCode": "playback_session_runtime_unavailable",
		})
		return
	}

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
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusBadGateway, gin.H{
			"Error":     "Runtime transcode unavailable",
			"ErrorCode": "playback_session_start_failed",
		})
		return
	}
	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "private, no-store")
	c.Header("Pragma", "no-cache")
	c.String(http.StatusOK, buildEmbySessionMaster(c, embyID, mapping))
}

// The old quality-scoped routes are retained only so cached client URLs receive
// a deterministic retirement response. They never resolve shared directories,
// persistent runtime Artifacts, or on-demand segments.
func (h *Handler) HLSPlaylistHandler(c *gin.Context) {
	writeEmbyPlaybackSessionRequired(c, "legacy_quality_playlist_retired")
}

func (h *Handler) HLSSegmentHandler(c *gin.Context) {
	writeEmbyPlaybackSessionRequired(c, "legacy_quality_segment_retired")
}

func writeEmbyPlaybackSessionRequired(c *gin.Context, reason string) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(http.StatusGone, gin.H{
		"Error":     "Runtime transcoding requires PlaySessionId-bound HLS",
		"ErrorCode": "playback_session_required",
		"Reason":    reason,
	})
}
