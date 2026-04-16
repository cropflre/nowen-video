package handler

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AdminHandler 管理处理器
type AdminHandler struct {
	userService       *service.UserService
	authService       *service.AuthService
	transcodeService  *service.TranscodeService
	monitorService    *service.MonitorService
	schedulerService  *service.SchedulerService
	permissionService *service.PermissionService
	libraryService    *service.LibraryService
	metadataService   *service.MetadataService
	seriesService     *service.SeriesService
	settingRepo       *repository.SystemSettingRepo
	libraryRepo       *repository.LibraryRepo
	cfg               *config.Config
	logger            *zap.SugaredLogger
	db                *gorm.DB
}

// ==================== 用户管理 ====================

// ListUsers 获取所有用户
func (h *AdminHandler) ListUsers(c *gin.Context) {
	users, err := h.userService.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users})
}

// DeleteUser 删除用户
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	currentUserID, _ := c.Get("user_id")
	if id == currentUserID.(string) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除自己"})
		return
	}

	if err := h.userService.DeleteUser(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除用户失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ResetUserPassword 管理员重置用户密码
func (h *AdminHandler) ResetUserPassword(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=6,max=64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效，新密码至少6位"})
		return
	}

	if err := h.authService.ResetPassword(userID, req.NewPassword); err != nil {
		if err == service.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		h.logger.Errorf("重置用户密码失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置密码失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码已重置"})
}

// ==================== 系统信息 ====================

// SystemInfo 系统信息
func (h *AdminHandler) SystemInfo(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"version":    "0.1.0",
			"go_version": runtime.Version(),
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"cpus":       runtime.NumCPU(),
			"goroutines": runtime.NumGoroutine(),
			"memory": gin.H{
				"alloc_mb":       memStats.Alloc / 1024 / 1024,
				"total_alloc_mb": memStats.TotalAlloc / 1024 / 1024,
				"sys_mb":         memStats.Sys / 1024 / 1024,
			},
			"hw_accel": h.transcodeService.GetHWAccelInfo(),
		},
	})
}

// ==================== 转码管理 ====================

// TranscodeStatus 转码任务状态
func (h *AdminHandler) TranscodeStatus(c *gin.Context) {
	jobs := h.transcodeService.GetRunningJobs()

	var result []gin.H
	for _, job := range jobs {
		result = append(result, gin.H{
			"id":       job.Task.ID,
			"media_id": job.Task.MediaID,
			"quality":  job.Quality,
			"status":   job.Task.Status,
			"progress": job.Task.Progress,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// CancelTranscode 取消正在运行的转码任务
func (h *AdminHandler) CancelTranscode(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务ID不能为空"})
		return
	}

	if err := h.transcodeService.CancelTranscode(taskID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "转码任务已取消"})
}
