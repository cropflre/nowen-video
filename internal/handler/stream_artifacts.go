package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
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

	if startupProfile, ok := service.ParseStartupBridgeProfile(quality); ok {
		h.startupVirtualSegment(c, id, startupProfile, segment)
		return
	}

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

	artifactID := c.Query(service.HLSArtifactVersionQuery)
	artifactErr := h.streamService.ServeArtifactSegmentVersion(id, quality, artifactID, segment, c.Writer, c.Request)
	if artifactErr == nil {
		return
	}

	// A versioned playlist is an immutable read contract. When that exact
	// retained Artifact is unavailable, never fall through to on-demand or the
	// current version: doing so could mix segments from different encodes.
	if artifactID != "" {
		c.Header("Cache-Control", "no-store")
		if errors.Is(artifactErr, service.ErrArtifactNotReady) {
			c.JSON(http.StatusGone, gin.H{
				"error":       "artifact version is no longer available",
				"error_code":  "artifact_version_unavailable",
				"artifact_id": artifactID,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": artifactErr.Error()})
		return
	}

	if !errors.Is(artifactErr, service.ErrArtifactNotReady) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": artifactErr.Error()})
		return
	}

	// On-demand is a formal first-segment availability fallback for legacy
	// unversioned requests only, not a blanket error mask.
	if err := h.streamService.ServeOnDemandSegment(id, quality, segment, c.Writer, c.Request); err != nil {
		c.Header("Retry-After", "1")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
}

func (h *ArtifactStreamHandler) startupVirtualSegment(c *gin.Context, mediaID, profileID, segment string) {
	if segment == "stream.m3u8" {
		playlist, err := h.streamService.GetStartupBridgePlaylist(mediaID, profileID)
		if err != nil {
			startupBridgeError(c, err)
			return
		}
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
		c.Header("Cache-Control", "no-store")
		c.String(http.StatusOK, playlist)
		return
	}

	source, actual, ok := service.ParseStartupBridgeSegment(segment)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	var file *service.StartupBridgeFile
	var err error
	if source == "startup" {
		file, err = h.streamService.ResolveStartupBridgeSegment(mediaID, profileID, actual)
	} else {
		file, err = h.streamService.ResolveStartupContinuationSegment(mediaID, profileID, actual)
	}
	if err != nil {
		startupBridgeError(c, err)
		return
	}
	sendStartupBridgeFile(c, file)
}
