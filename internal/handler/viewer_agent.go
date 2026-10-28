package handler

import (
	"errors"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"gorm.io/gorm"
)

// RegisterViewerAgent is shared by full and Lite, including AI-disabled installs.
func RegisterViewerAgent(api *gin.RouterGroup, db *gorm.DB, ai *service.AIService, executors ...service.AgentScanExecutor) {
	agent := service.NewViewerAgent(db, ai)
	var executor service.AgentScanExecutor
	if len(executors) > 0 {
		executor = executors[0]
	}
	runs := service.NewAgentScanService(db, executor)
	registerAgentScanRoutes(api, runs)
	var busy sync.Map
	api.POST("/agent/chat", func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		uid := c.GetString("user_id")
		if uid == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}
		if _, loaded := busy.LoadOrStore(uid, true); loaded {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "上一条消息还在处理中，请稍等"})
			return
		}
		defer busy.Delete(uid)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
		var req service.ViewerAgentRequest
		if c.ShouldBindJSON(&req) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "消息格式不正确"})
			return
		}
		out, err := agent.Chat(c.Request.Context(), uid, req)
		if errors.Is(err, service.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "只有管理员可以检查整个影视库和管理任务"})
			return
		}
		if errors.Is(err, service.ErrAgentInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "暂时无法读取影视库，请稍后重试，也可以使用找片页面"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": out})
	})
}
