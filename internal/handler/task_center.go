package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// TaskCenterHandler exposes the read-only unified task view used by Lite.
type TaskCenterHandler struct {
	service *service.TaskCenterService
	logger  *zap.SugaredLogger
}

func NewTaskCenterHandler(taskService *service.TaskCenterService, logger *zap.SugaredLogger) *TaskCenterHandler {
	return &TaskCenterHandler{service: taskService, logger: logger}
}

func (h *TaskCenterHandler) List(c *gin.Context) {
	activeOnly, _ := strconv.ParseBool(c.DefaultQuery("active", "false"))
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit 必须是整数"})
		return
	}

	snapshot, err := h.service.Snapshot(activeOnly, limit)
	if err != nil {
		h.logger.Errorf("获取统一任务列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取任务列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": snapshot})
}
