package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// CastHandler 投屏处理器
type CastHandler struct {
	castService *service.CastService
	logger      *zap.SugaredLogger
}

// ListDevices 列出已发现的投屏设备
func (h *CastHandler) ListDevices(c *gin.Context) {
	devices := h.castService.ListDevices()
	c.JSON(http.StatusOK, gin.H{"data": devices})
}

// RefreshDevices 手动刷新设备发现
func (h *CastHandler) RefreshDevices(c *gin.Context) {
	h.castService.RefreshDevices()
	c.JSON(http.StatusOK, gin.H{"message": "已触发设备发现，请稍后刷新列表"})
}

// CastRequest 投屏请求
type CastRequest struct {
	DeviceID string `json:"device_id" binding:"required"`
	MediaID  string `json:"media_id" binding:"required"`
}

// CastMedia 投屏媒体到设备
func (h *CastHandler) CastMedia(c *gin.Context) {
	var req CastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	// 获取服务器地址（用于设备回调）
	serverAddr := c.Request.Host
	if serverAddr == "" {
		serverAddr = "localhost:8080"
	}

	session, err := h.castService.CastMedia(req.DeviceID, req.MediaID, serverAddr)
	if err != nil {
		h.logger.Errorf("投屏失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    session,
		"message": "投屏成功",
	})
}

// ControlRequest 投屏控制请求
type ControlRequest struct {
	Action string  `json:"action" binding:"required"` // play / pause / stop / seek / volume
	Value  float64 `json:"value"`                     // seek=秒数, volume=0-100
}

// ControlCast 控制投屏播放
func (h *CastHandler) ControlCast(c *gin.Context) {
	sessionID := c.Param("sessionId")

	var req ControlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if err := h.castService.ControlCast(sessionID, req.Action, req.Value); err != nil {
		h.logger.Errorf("投屏控制失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "操作成功"})
}

// GetSession 获取投屏会话状态
func (h *CastHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	session, err := h.castService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": session})
}

// ListSessions 列出活跃的投屏会话
func (h *CastHandler) ListSessions(c *gin.Context) {
	sessions := h.castService.ListSessions()
	c.JSON(http.StatusOK, gin.H{"data": sessions})
}

// StopSession 停止投屏
func (h *CastHandler) StopSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	if err := h.castService.ControlCast(sessionID, "stop", 0); err != nil {
		h.logger.Errorf("停止投屏失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已停止投屏"})
}
