package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// PulseHandler remains only to keep the legacy Full composition source-compatible.
// The public product surface is gone; old endpoint registrations behave exactly
// like missing routes and reveal no retired module metadata.
type PulseHandler struct {
	pulseService *service.PulseService
	logger       *zap.SugaredLogger
}

func (h *PulseHandler) removed(c *gin.Context) {
	c.AbortWithStatus(http.StatusNotFound)
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
