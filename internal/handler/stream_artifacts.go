package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const runtimeTranscodeRetiredCode = "playback_session_required"

// ArtifactStreamHandler is retained as the aggregate handler type while the
// durable Artifact implementation continues to serve explicit administrator
// preprocessing. Runtime playback methods are overridden here and can no
// longer enter persistent Artifact, shared media/profile, on-demand, or audio
// cache paths.
type ArtifactStreamHandler struct {
	*StreamHandler
}

func NewArtifactStreamHandler(base *StreamHandler) *ArtifactStreamHandler {
	return &ArtifactStreamHandler{StreamHandler: base}
}

func (h *ArtifactStreamHandler) Master(c *gin.Context) {
	writePlaybackSessionRequired(c, "legacy_master_playlist_retired")
}

func (h *ArtifactStreamHandler) Segment(c *gin.Context) {
	writePlaybackSessionRequired(c, "legacy_runtime_segment_retired")
}

func (h *ArtifactStreamHandler) AudioPlaylist(c *gin.Context) {
	writePlaybackSessionRequired(c, "legacy_audio_playlist_retired")
}

func (h *ArtifactStreamHandler) AudioSegment(c *gin.Context) {
	writePlaybackSessionRequired(c, "legacy_audio_segment_retired")
}

func writePlaybackSessionRequired(c *gin.Context, reason string) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(http.StatusGone, gin.H{
		"error":              "runtime transcoding requires an ephemeral playback session",
		"error_code":         runtimeTranscodeRetiredCode,
		"reason":             reason,
		"session_create_url": "/api/playback/sessions",
	})
}
