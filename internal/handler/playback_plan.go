package handler

import (
	"errors"
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

type PlannedMediaPlayInfo struct {
	*service.MediaPlayInfo
	PlaybackPlan *service.PlaybackPlan `json:"playback_plan"`
}

func NewPlaybackPlanHandler(stream *service.StreamService, logger *zap.SugaredLogger) *PlaybackPlanHandler {
	return &PlaybackPlanHandler{stream: stream, logger: logger}
}

// GetInfo is the canonical Lite playback-info entry point. It keeps all legacy
// fields while embedding the server-side playback decision, so clients no
// longer need a second /plan round trip.
func (h *PlaybackPlanHandler) GetInfo(c *gin.Context) {
	mediaID := c.Param("id")
	if mediaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	info, err := h.stream.GetMediaPlayInfo(mediaID)
	if err != nil {
		c.JSON(playbackPlanErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	plan, err := h.stream.PlanPlaybackWithInfo(mediaID, info, h.clientCapabilities(c))
	if err != nil {
		if h.logger != nil {
			h.logger.Warnf("生成播放规划失败 media_id=%s: %v", mediaID, err)
		}
		c.JSON(playbackPlanErrorStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": PlannedMediaPlayInfo{
		MediaPlayInfo: info,
		PlaybackPlan:  plan,
	}})
}

// Get remains available for diagnostics and clients that only need a plan.
func (h *PlaybackPlanHandler) Get(c *gin.Context) {
	mediaID := c.Param("id")
	if mediaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	plan, err := h.stream.PlanPlayback(mediaID, h.clientCapabilities(c))
	if err != nil {
		if h.logger != nil {
			h.logger.Warnf("生成播放规划失败 media_id=%s: %v", mediaID, err)
		}
		c.JSON(playbackPlanErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": plan})
}

func playbackPlanErrorStatus(err error) int {
	if errors.Is(err, service.ErrMediaNotFound) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

func (h *PlaybackPlanHandler) clientCapabilities(c *gin.Context) service.PlaybackClientCapabilities {
	caps := h.stream.DefaultPlaybackClientCapabilities(c.GetHeader("User-Agent"))
	caps.SupportsDirectPlay = queryBool(c, "supports_direct", caps.SupportsDirectPlay)
	caps.SupportsRemux = queryBool(c, "supports_remux", caps.SupportsRemux)
	caps.SupportsHEVC = queryBool(c, "supports_hevc", caps.SupportsHEVC)
	caps.ForceTranscode = queryBool(c, "force_transcode", false)
	caps.MaxBitrate = queryPositiveInt(c, "max_bitrate")

	// 扩展精确能力参数（来自前端 media-capabilities 探测）
	caps.HEVCHardware = queryBool(c, "hevc_hardware", false)
	caps.AudioSupportsAC3 = queryBool(c, "audio_supports_ac3", false)
	caps.AudioSupportsEAC3 = queryBool(c, "audio_supports_eac3", false)
	caps.AudioSupportsFLAC = queryBool(c, "audio_supports_flac", false)
	caps.AudioSupportsOpus = queryBool(c, "audio_supports_opus", false)
	caps.ContainerSupportsMP4 = queryBool(c, "container_supports_mp4", true)
	caps.ContainerSupportsWebM = queryBool(c, "container_supports_webm", false)
	caps.MSEH264 = queryBool(c, "mse_h264", false)
	caps.MSEHEVC = queryBool(c, "mse_hevc", false)
	caps.Platform = strings.TrimSpace(c.Query("platform"))

	return caps
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
