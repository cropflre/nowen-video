package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// PulseHandler is a compatibility tombstone for legacy Full routes.
// Pulse has been removed; endpoints return 410 so clients can distinguish a
// permanently removed capability from a temporary missing route.
type PulseHandler struct {
	pulseService *service.PulseService
	logger       *zap.SugaredLogger
}

func (h *PulseHandler) removed(c *gin.Context) {
	c.Header("Deprecation", "true")
	c.Header("Sunset", "Thu, 30 Jul 2026 00:00:00 GMT")
	c.AbortWithStatusJSON(http.StatusGone, gin.H{
		"error": "Pulse 功能已永久移除",
		"code":  "pulse_removed",
	})
}

func (h *PulseHandler) GetDashboard(c *gin.Context)          { h.removed(c) }
func (h *PulseHandler) GetPlayTrends(c *gin.Context)         { h.removed(c) }
func (h *PulseHandler) GetTopContent(c *gin.Context)         { h.removed(c) }
func (h *PulseHandler) GetTopUsers(c *gin.Context)           { h.removed(c) }
func (h *PulseHandler) GetRecentPlays(c *gin.Context)        { h.removed(c) }
func (h *PulseHandler) GetAnalytics(c *gin.Context)          { h.removed(c) }
func (h *PulseHandler) GetHourlyDistribution(c *gin.Context) { h.removed(c) }
func (h *PulseHandler) GetLibraryStats(c *gin.Context)       { h.removed(c) }
func (h *PulseHandler) GetMediaGrowth(c *gin.Context)        { h.removed(c) }
