package handler

import "github.com/gin-gonic/gin"

// Startup bridge routes are retained temporarily for cached client URLs, but
// they can no longer read startup or continuation Artifacts. Runtime playback
// must create a Playback Session, whose Generation owns both playlist and
// segment files.
func (h *ArtifactStreamHandler) StartupBridgePlaylist(c *gin.Context) {
	writePlaybackSessionRequired(c, "startup_bridge_playlist_retired")
}

func (h *ArtifactStreamHandler) StartupBridgeSegment(c *gin.Context) {
	writePlaybackSessionRequired(c, "startup_bridge_segment_retired")
}

func (h *ArtifactStreamHandler) StartupContinuationSegment(c *gin.Context) {
	writePlaybackSessionRequired(c, "startup_continuation_segment_retired")
}
