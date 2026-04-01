package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// PulseHandler Pulse 数据中心 API 处理器
type PulseHandler struct {
	pulseService *service.PulseService
	logger       *zap.SugaredLogger
}

// ==================== 全景仪表盘 ====================

// GetDashboard 获取仪表盘完整数据（一次性加载）
func (h *PulseHandler) GetDashboard(c *gin.Context) {
	data, err := h.pulseService.GetDashboard()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取仪表盘数据失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// GetPlayTrends 获取播放趋势
func (h *PulseHandler) GetPlayTrends(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	trends, err := h.pulseService.GetPlayTrends(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取播放趋势失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": trends})
}

// GetTopContent 获取热门内容排行
func (h *PulseHandler) GetTopContent(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	items, err := h.pulseService.GetTopContent(days, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取热门内容失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// GetTopUsers 获取用户活跃排行
func (h *PulseHandler) GetTopUsers(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	items, err := h.pulseService.GetTopUsers(days, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户排行失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// GetRecentPlays 获取最近播放记录
func (h *PulseHandler) GetRecentPlays(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.pulseService.GetRecentPlays(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取最近播放失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// ==================== 媒体分析 ====================

// GetAnalytics 获取媒体分析完整数据
func (h *PulseHandler) GetAnalytics(c *gin.Context) {
	data, err := h.pulseService.GetAnalytics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取媒体分析失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// GetHourlyDistribution 获取播放时段分布
func (h *PulseHandler) GetHourlyDistribution(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	items, err := h.pulseService.GetHourlyDistribution(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取时段分布失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// GetLibraryStats 获取媒体库统计
func (h *PulseHandler) GetLibraryStats(c *gin.Context) {
	items, err := h.pulseService.GetLibraryStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取媒体库统计失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// GetMediaGrowth 获取媒体库增长趋势
func (h *PulseHandler) GetMediaGrowth(c *gin.Context) {
	months, _ := strconv.Atoi(c.DefaultQuery("months", "12"))
	items, err := h.pulseService.GetMediaGrowth(months)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取增长趋势失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}
