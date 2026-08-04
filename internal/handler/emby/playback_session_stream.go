package emby

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// SessionHLSPlaylistHandler serves the media playlist for one immutable
// playback Generation. A stale playlist request is redirected to the current
// Generation, while already-issued stale segment URLs remain readable until
// the session manager drains their Reader Leases.
func (h *Handler) SessionHLSPlaylistHandler(c *gin.Context) {
	runtime := h.playbackSessionRuntime()
	if runtime == nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Playback session runtime unavailable"})
		return
	}

	embyID := c.Param("id")
	mediaID := h.idMap.Resolve(embyID)
	if mediaID == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Item not found"})
		return
	}
	userID := c.GetString("user_id")
	externalID := c.Param("playSessionID")
	mapping, ok := runtime.find(userID, externalID, mediaID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Playback session not found"})
		return
	}

	generationID, err := strconv.ParseUint(c.Param("generationID"), 10, 64)
	if err != nil || generationID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Invalid generation id"})
		return
	}
	if generationID != mapping.GenerationID {
		c.Redirect(
			http.StatusTemporaryRedirect,
			embySessionPlaylistURL(c, embyID, mapping.ExternalID, mapping.GenerationID),
		)
		return
	}

	playlist, err := runtime.openPlaylist(mapping, generationID)
	if err != nil {
		h.logger.Warnf(
			"[emby] session playlist failed media=%s play_session=%s generation=%d err=%v",
			mediaID,
			externalID,
			generationID,
			err,
		)
		c.JSON(http.StatusNotFound, gin.H{"Error": "Playback generation not available"})
		return
	}

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "private, no-store")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	c.String(
		http.StatusOK,
		rewriteEmbySessionPlaylist(c, embyID, mapping, generationID, playlist),
	)
}

// SessionHLSSegmentHandler serves a specific Generation segment with the
// PlaybackSessionService Reader Lease held for the full HTTP response.
func (h *Handler) SessionHLSSegmentHandler(c *gin.Context) {
	runtime := h.playbackSessionRuntime()
	if runtime == nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Playback session runtime unavailable"})
		return
	}

	embyID := c.Param("id")
	mediaID := h.idMap.Resolve(embyID)
	if mediaID == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Item not found"})
		return
	}
	userID := c.GetString("user_id")
	externalID := c.Param("playSessionID")
	mapping, ok := runtime.find(userID, externalID, mediaID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Playback session not found"})
		return
	}
	generationID, err := strconv.ParseUint(c.Param("generationID"), 10, 64)
	if err != nil || generationID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Invalid generation id"})
		return
	}

	file, err := runtime.openSegment(mapping, generationID, c.Param("segment"))
	if err != nil {
		h.logger.Debugf(
			"[emby] session segment unavailable media=%s play_session=%s generation=%d segment=%s err=%v",
			mediaID,
			externalID,
			generationID,
			c.Param("segment"),
			err,
		)
		c.JSON(http.StatusNotFound, gin.H{
			"Error": fmt.Sprintf("Playback segment unavailable: %v", err),
		})
		return
	}
	defer file.Release()

	c.Header("Content-Type", file.ContentType)
	c.Header("Cache-Control", "private, no-store")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	http.ServeFile(c.Writer, c.Request, file.Path)
}
