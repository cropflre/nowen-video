package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

type PlaybackPlanHandler struct {
	stream *service.StreamService
	logger *zap.SugaredLogger
}

func NewPlaybackPlanHandler(stream *service.StreamService, logger *zap.SugaredLogger) *PlaybackPlanHandler {
	return &PlaybackPlanHandler{stream: stream, logger: logger}
}

func (h *PlaybackPlanHandler) Get(c *gin.Context) {
	mediaID := c.Param("id")
	if mediaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	caps := h.stream.DefaultPlaybackClientCapabilities(c.GetHeader("User-Agent"))
	caps.SupportsDirectPlay = queryBool(c, "supports_direct", caps.SupportsDirectPlay)
	caps.SupportsRemux = queryBool(c, "supports_remux", caps.SupportsRemux)
	caps.SupportsHEVC = queryBool(c, "supports_hevc", caps.SupportsHEVC)
	caps.ForceTranscode = queryBool(c, "force_transcode", false)
	caps.MaxBitrate = queryPositiveInt(c, "max_bitrate")

	plan, err := h.stream.PlanPlayback(mediaID, caps)
	if err != nil {
		h.logger.Warnf("生成播放规划失败 media_id=%s: %v", mediaID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": plan})
}

func queryBool(c *gin.Context, key string, defaultValue bool) bool {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func queryPositiveInt(c *gin.Context, key string) int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}
