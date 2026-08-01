package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"gorm.io/gorm"
)

func (h *ArtifactStreamHandler) StartupBridgePlaylist(c *gin.Context) {
	if h == nil || h.StreamHandler == nil || h.streamService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "stream service unavailable"})
		return
	}
	playlist, err := h.streamService.GetStartupBridgePlaylist(c.Param("id"), c.Param("quality"))
	if err != nil {
		startupBridgeError(c, err)
		return
	}
	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-store")
	c.String(http.StatusOK, playlist)
}

func (h *ArtifactStreamHandler) StartupBridgeSegment(c *gin.Context) {
	if h == nil || h.StreamHandler == nil || h.streamService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "stream service unavailable"})
		return
	}
	file, err := h.streamService.ResolveStartupBridgeSegment(
		c.Param("id"),
		c.Param("quality"),
		c.Param("segment"),
	)
	if err != nil {
		startupBridgeError(c, err)
		return
	}
	sendStartupBridgeFile(c, file)
}

func (h *ArtifactStreamHandler) StartupContinuationSegment(c *gin.Context) {
	if h == nil || h.StreamHandler == nil || h.streamService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "stream service unavailable"})
		return
	}
	file, err := h.streamService.ResolveStartupContinuationSegment(
		c.Param("id"),
		c.Param("quality"),
		c.Param("segment"),
	)
	if err != nil {
		startupBridgeError(c, err)
		return
	}
	sendStartupBridgeFile(c, file)
}

func sendStartupBridgeFile(c *gin.Context, file *service.StartupBridgeFile) {
	if file == nil || file.Path == "" {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Content-Type", "video/mp2t")
	c.Header("Accept-Ranges", "bytes")
	if file.Immutable {
		c.Header("Cache-Control", "private, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "no-store")
	}
	http.ServeFile(c.Writer, c.Request, file.Path)
}

func startupBridgeError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, service.ErrMediaNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
