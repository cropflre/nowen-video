package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// PulseHandler 仅保留旧路由的方法签名，避免旧版服务启动链断裂。
// Pulse 功能已经从产品中移除，所有旧接口统一返回 404。
type PulseHandler struct {
	pulseService *service.PulseService
	logger       *zap.SugaredLogger
}

func (h *PulseHandler) removed(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
		"error": "Pulse 功能已移除",
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
