package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// TaskCenterHandler exposes the unified Lite task view and delegates supported
// lifecycle operations to the existing module-specific services.
type TaskCenterHandler struct {
	service *service.TaskCenterService
	actions *service.TaskActionDispatcher
	logger  *zap.SugaredLogger
}

func NewTaskCenterHandler(taskService *service.TaskCenterService, actions *service.TaskActionDispatcher, logger *zap.SugaredLogger) *TaskCenterHandler {
	return &TaskCenterHandler{service: taskService, actions: actions, logger: logger}
}

type taskCenterItem struct {
	service.UnifiedTask
	Actions []string `json:"actions"`
}

type taskCenterListResponse struct {
	Tasks   []taskCenterItem          `json:"tasks"`
	Summary service.TaskCenterSummary `json:"summary"`
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
	items := make([]taskCenterItem, 0, len(snapshot.Tasks))
	for _, task := range snapshot.Tasks {
		items = append(items, taskCenterItem{
			UnifiedTask: task,
			Actions:     service.AvailableTaskActions(task.Kind, task.Status),
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": taskCenterListResponse{Tasks: items, Summary: snapshot.Summary}})
}

func (h *TaskCenterHandler) Action(c *gin.Context) {
	if h.actions == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "任务操作服务不可用", "code": "task_actions_unavailable"})
		return
	}
	userID, _ := c.Get("user_id")
	actor, _ := userID.(string)
	result, err := h.actions.Execute(c.Param("kind"), c.Param("id"), c.Param("action"), actor)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在", "code": "task_not_found"})
		case errors.Is(err, service.ErrTaskActionConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "当前任务状态不允许执行该操作", "code": "task_action_conflict"})
		case errors.Is(err, service.ErrTaskActionUnsupported):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "该任务不支持此操作", "code": "task_action_unsupported"})
		default:
			h.logger.Errorf("统一任务操作失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "任务操作失败", "code": "task_action_failed"})
		}
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": result})
}
