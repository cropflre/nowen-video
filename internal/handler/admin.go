package handler

import (
	"net/http"
	"runtime"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// AdminHandler 管理处理器
type AdminHandler struct {
	userService       *service.UserService
	transcodeService  *service.TranscodeService
	monitorService    *service.MonitorService
	schedulerService  *service.SchedulerService
	permissionService *service.PermissionService
	libraryService    *service.LibraryService
	metadataService   *service.MetadataService
	cfg               *config.Config
	logger            *zap.SugaredLogger
}

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

// ==================== TMDb 配置管理 ====================

// GetTMDbConfig 获取 TMDb API Key 配置状态
func (h *AdminHandler) GetTMDbConfig(c *gin.Context) {
	maskedKey := h.cfg.GetTMDbAPIKeyMasked()
	configured := h.cfg.GetTMDbAPIKey() != ""

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"configured": configured,
			"masked_key": maskedKey,
		},
	})
}

// UpdateTMDbConfigRequest 更新 TMDb API Key 请求
type UpdateTMDbConfigRequest struct {
	APIKey string `json:"api_key" binding:"required"`
}

// UpdateTMDbConfig 更新 TMDb API Key
func (h *AdminHandler) UpdateTMDbConfig(c *gin.Context) {
	var req UpdateTMDbConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供有效的 API Key"})
		return
	}

	key := req.APIKey
	if len(key) < 16 || len(key) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API Key 格式不正确，请检查后重试"})
		return
	}

	if err := h.cfg.SetTMDbAPIKey(key); err != nil {
		h.logger.Errorf("保存 TMDb API Key 失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	h.logger.Info("TMDb API Key 已更新")
	c.JSON(http.StatusOK, gin.H{
		"message": "TMDb API Key 已保存",
		"data": gin.H{
			"configured": true,
			"masked_key": h.cfg.GetTMDbAPIKeyMasked(),
		},
	})
}

// ClearTMDbConfig 清除 TMDb API Key
func (h *AdminHandler) ClearTMDbConfig(c *gin.Context) {
	if err := h.cfg.ClearTMDbAPIKey(); err != nil {
		h.logger.Errorf("清除 TMDb API Key 失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清除配置失败: " + err.Error()})
		return
	}

	h.logger.Info("TMDb API Key 已清除")
	c.JSON(http.StatusOK, gin.H{
		"message": "TMDb API Key 已清除",
		"data": gin.H{
			"configured": false,
			"masked_key": "",
		},
	})
}

// ==================== 系统监控 ====================

// GetMetrics 获取实时系统指标
func (h *AdminHandler) GetMetrics(c *gin.Context) {
	metrics := h.monitorService.GetMetrics()
	c.JSON(http.StatusOK, gin.H{"data": metrics})
}

// ==================== 定时任务管理 ====================

// ListScheduledTasks 获取定时任务列表
func (h *AdminHandler) ListScheduledTasks(c *gin.Context) {
	tasks, err := h.schedulerService.ListTasks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取任务列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tasks})
}

// CreateScheduledTaskRequest 创建定时任务请求
type CreateScheduledTaskRequest struct {
	Name     string `json:"name" binding:"required"`
	Type     string `json:"type" binding:"required"`     // scan, scrape, cleanup
	Schedule string `json:"schedule" binding:"required"` // @daily, @every 6h等
	TargetID string `json:"target_id"`
}

// CreateScheduledTask 创建定时任务
func (h *AdminHandler) CreateScheduledTask(c *gin.Context) {
	var req CreateScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	task, err := h.schedulerService.CreateTask(req.Name, req.Type, req.Schedule, req.TargetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": task})
}

// UpdateScheduledTaskRequest 更新定时任务请求
type UpdateScheduledTaskRequest struct {
	Name     string `json:"name" binding:"required"`
	Schedule string `json:"schedule" binding:"required"`
	Enabled  bool   `json:"enabled"`
}

// UpdateScheduledTask 更新定时任务
func (h *AdminHandler) UpdateScheduledTask(c *gin.Context) {
	id := c.Param("id")

	var req UpdateScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if err := h.schedulerService.UpdateTask(id, req.Name, req.Schedule, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

// DeleteScheduledTask 删除定时任务
func (h *AdminHandler) DeleteScheduledTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.schedulerService.DeleteTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// RunScheduledTaskNow 立即执行定时任务
func (h *AdminHandler) RunScheduledTaskNow(c *gin.Context) {
	id := c.Param("id")

	if err := h.schedulerService.RunTaskNow(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务已开始执行"})
}

// ==================== 批量操作 ====================

// BatchScanRequest 批量扫描请求
type BatchScanRequest struct {
	LibraryIDs []string `json:"library_ids" binding:"required"`
}

// BatchScan 批量扫描多个媒体库
func (h *AdminHandler) BatchScan(c *gin.Context) {
	var req BatchScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	var started []string
	var errors []gin.H
	for _, id := range req.LibraryIDs {
		if err := h.libraryService.Scan(id); err != nil {
			errors = append(errors, gin.H{"library_id": id, "error": err.Error()})
		} else {
			started = append(started, id)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "批量扫描已启动",
		"started": started,
		"errors":  errors,
	})
}

// BatchScrapeRequest 批量刮削请求
type BatchScrapeRequest struct {
	MediaIDs []string `json:"media_ids" binding:"required"`
}

// BatchScrape 批量刮削元数据
func (h *AdminHandler) BatchScrape(c *gin.Context) {
	var req BatchScrapeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	// 异步执行批量刮削
	go func() {
		success := 0
		failed := 0
		for _, id := range req.MediaIDs {
			if err := h.metadataService.ScrapeMedia(id); err != nil {
				failed++
			} else {
				success++
			}
		}
		h.logger.Infof("批量刮削完成: 成功 %d, 失败 %d", success, failed)
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "批量刮削已启动",
		"total":   len(req.MediaIDs),
	})
}

// ==================== 权限管理 ====================

// GetUserPermission 获取用户权限设置
func (h *AdminHandler) GetUserPermission(c *gin.Context) {
	userID := c.Param("userId")
	perm, err := h.permissionService.GetUserPermission(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取权限失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": perm})
}

// UpdateUserPermissionRequest 更新用户权限请求
type UpdateUserPermissionRequest struct {
	AllowedLibraries string `json:"allowed_libraries"` // 逗号分隔的媒体库ID
	MaxRatingLevel   string `json:"max_rating_level"`
	DailyTimeLimit   int    `json:"daily_time_limit"` // 分钟
}

// UpdateUserPermission 更新用户权限
func (h *AdminHandler) UpdateUserPermission(c *gin.Context) {
	userID := c.Param("userId")

	var req UpdateUserPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if err := h.permissionService.UpdateUserPermission(userID, req.AllowedLibraries, req.MaxRatingLevel, req.DailyTimeLimit); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新权限失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "权限已更新"})
}

// SetContentRatingRequest 设置内容分级请求
type SetContentRatingRequest struct {
	Level string `json:"level" binding:"required"` // G, PG, PG-13, R, NC-17
}

// SetContentRating 设置媒体内容分级
func (h *AdminHandler) SetContentRating(c *gin.Context) {
	mediaID := c.Param("mediaId")

	var req SetContentRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if err := h.permissionService.SetContentRating(mediaID, req.Level); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "分级已设置"})
}

// GetContentRating 获取媒体内容分级
func (h *AdminHandler) GetContentRating(c *gin.Context) {
	mediaID := c.Param("mediaId")
	level, err := h.permissionService.GetContentRating(mediaID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"media_id": mediaID, "level": ""}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"media_id": mediaID, "level": level}})
}

// ==================== 访问日志 ====================

// ListAccessLogs 获取访问日志
func (h *AdminHandler) ListAccessLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	userID := c.Query("user_id")
	action := c.Query("action")

	logs, total, err := h.permissionService.ListAccessLogs(page, size, userID, action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取日志失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": logs, "total": total, "page": page, "size": size})
}

// ==================== 手动元数据匹配 ====================

// SearchMetadataRequest 搜索元数据请求
type SearchMetadataRequest struct {
	Query     string `json:"query" binding:"required"`
	Year      int    `json:"year"`
	MediaType string `json:"media_type"` // movie, tv
}

// SearchMetadata 手动搜索TMDb元数据
func (h *AdminHandler) SearchMetadata(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供搜索关键词"})
		return
	}
	mediaType := c.DefaultQuery("type", "movie")
	year, _ := strconv.Atoi(c.Query("year"))

	results, err := h.metadataService.SearchTMDb(mediaType, query, year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": results})
}

// MatchMetadataRequest 手动关联元数据请求
type MatchMetadataRequest struct {
	TMDbID int `json:"tmdb_id" binding:"required"`
}

// MatchMetadata 手动关联TMDb元数据到指定媒体
func (h *AdminHandler) MatchMetadata(c *gin.Context) {
	mediaID := c.Param("mediaId")

	var req MatchMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if err := h.metadataService.MatchMediaWithTMDb(mediaID, req.TMDbID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "关联元数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "元数据已关联"})
}
