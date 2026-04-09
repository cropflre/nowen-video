package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// StatsHandler 播放统计 API
type StatsHandler struct {
	statsService *service.StatsService
	logger       *zap.SugaredLogger
}

// RecordPlayback 记录播放统计
func (h *StatsHandler) RecordPlayback(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		MediaID      string  `json:"media_id" binding:"required"`
		WatchMinutes float64 `json:"watch_minutes" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if err := h.statsService.RecordPlayback(userID.(string), req.MediaID, req.WatchMinutes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// GetUserStats 获取用户统计概览
func (h *StatsHandler) GetUserStats(c *gin.Context) {
	userID, _ := c.Get("user_id")

	overview, err := h.statsService.GetUserOverview(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取统计失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": overview})
}

// GetUserStatsAdmin 管理员查看指定用户的统计
func (h *StatsHandler) GetUserStatsAdmin(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户ID不能为空"})
		return
	}

	overview, err := h.statsService.GetUserOverview(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取统计失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": overview})
}
