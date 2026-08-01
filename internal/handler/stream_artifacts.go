package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ArtifactStreamHandler is the formal migration Adapter for runtime HLS. It
// embeds the existing non-HLS endpoints while overriding Segment with the
// Artifact Store contract. Once the old shared-directory methods are removed,
// this Adapter can be renamed back to StreamHandler without client changes.
type ArtifactStreamHandler struct {
	*StreamHandler
}

func NewArtifactStreamHandler(base *StreamHandler) *ArtifactStreamHandler {
	return &ArtifactStreamHandler{StreamHandler: base}
}

func (h *ArtifactStreamHandler) Segment(c *gin.Context) {
	id := c.Param("id")
	quality := c.Param("quality")
	segment := c.Param("segment")

	if segment == "stream.m3u8" {
		playlist, err := h.streamService.GetArtifactSegmentPlaylist(id, quality)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
		c.Header("Cache-Control", "no-cache")
		c.String(http.StatusOK, playlist)
		return
	}

	if err := h.streamService.ServeArtifactSegment(id, quality, segment, c.Writer, c.Request); err == nil {
		return
	}
	if err := h.streamService.ServeOnDemandSegment(id, quality, segment, c.Writer, c.Request); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
}
