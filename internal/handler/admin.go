package handler

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
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
	schedulerService  *service.SchedulerService
	permissionService *service.PermissionService
	libraryService    *service.LibraryService
	metadataService   *service.MetadataService
	seriesService     *service.SeriesService
	settingRepo       *repository.SystemSettingRepo
	libraryRepo       *repository.LibraryRepo
	loginLogRepo      *repository.LoginLogRepo
	auditLogRepo      *repository.AuditLogRepo
	inviteRepo        *repository.InviteCodeRepo
	idleScheduler     *service.IdleScheduler
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

// CreateUser 管理员创建用户
func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req service.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}
	user, err := h.userService.CreateUser(&req)
	if err != nil {
		switch err {
		case service.ErrUserExists:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			h.logger.Errorf("创建用户失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		}
		return
	}
	h.auditFromContext(c, "user.create", "user", user.ID, "username="+user.Username+",role="+user.Role)
	c.JSON(http.StatusCreated, gin.H{"data": user})
}

// UpdateUser 管理员更新用户
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var req service.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}
	user, err := h.userService.UpdateUserByAdmin(id, &req)
	if err != nil {
		switch err {
		case service.ErrUserNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case service.ErrLastAdmin:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			h.logger.Errorf("更新用户失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户失败"})
		}
		return
	}
	h.auditFromContext(c, "user.update", "user", id, "")
	c.JSON(http.StatusOK, gin.H{"data": user})
}

// SetUserDisabled 启用/禁用用户
func (h *AdminHandler) SetUserDisabled(c *gin.Context) {
	id := c.Param("id")
	currentUserID, _ := c.Get("user_id")
	if id == currentUserID.(string) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能禁用自己"})
		return
	}

	var req struct {
		Disabled bool `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}
	if err := h.userService.SetUserDisabled(id, req.Disabled); err != nil {
		switch err {
		case service.ErrUserNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case service.ErrLastAdmin:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			h.logger.Errorf("更新用户禁用状态失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		}
		return
	}
	action := "user.enable"
	if req.Disabled {
		action = "user.disable"
	}
	h.auditFromContext(c, action, "user", id, "")
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
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
		switch err {
		case service.ErrUserNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case service.ErrLastAdmin:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			h.logger.Errorf("删除用户失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除用户失败"})
		}
		return
	}
	h.auditFromContext(c, "user.delete", "user", id, "")
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ResetUserPassword 管理员重置用户密码
func (h *AdminHandler) ResetUserPassword(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		NewPassword            string `json:"new_password" binding:"required,min=6,max=64"`
		ForceChangeOnNextLogin *bool  `json:"force_change_on_next_login"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效，新密码至少6位"})
		return
	}

	forceChange := true // 默认要求强制改密
	if req.ForceChangeOnNextLogin != nil {
		forceChange = *req.ForceChangeOnNextLogin
	}

	if err := h.authService.ResetPassword(userID, req.NewPassword, forceChange); err != nil {
		if err == service.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		h.logger.Errorf("重置用户密码失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置密码失败"})
		return
	}
	h.auditFromContext(c, "user.reset_password", "user", userID, "")
	c.JSON(http.StatusOK, gin.H{"message": "密码已重置"})
}

// ==================== 登录日志 & 审计日志 ====================

// ListLoginLogs 查询登录日志
func (h *AdminHandler) ListLoginLogs(c *gin.Context) {
	if h.loginLogRepo == nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}, "total": 0})
		return
	}
	page, size := parsePagination(c, 50, 200)
	onlyFailed := c.Query("only_failed") == "true"
	logs, total, err := h.loginLogRepo.ListRecent(page, size, onlyFailed)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs, "total": total, "page": page, "size": size})
}

// ListAuditLogs 查询审计日志
func (h *AdminHandler) ListAuditLogs(c *gin.Context) {
	if h.auditLogRepo == nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}, "total": 0})
		return
	}
	page, size := parsePagination(c, 50, 200)
	action := c.Query("action")
	logs, total, err := h.auditLogRepo.List(page, size, action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs, "total": total, "page": page, "size": size})
}

// ==================== 邀请码管理 ====================

// ListInviteCodes 列出所有邀请码
func (h *AdminHandler) ListInviteCodes(c *gin.Context) {
	if h.inviteRepo == nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	codes, err := h.inviteRepo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": codes})
}

// CreateInviteCode 创建邀请码
func (h *AdminHandler) CreateInviteCode(c *gin.Context) {
	var req struct {
		Code      string `json:"code"`
		MaxUses   int    `json:"max_uses"`
		ExpiresIn int    `json:"expires_in_hours"` // 0 = 永不过期
		Note      string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}
	if req.Code == "" {
		// 自动生成 12 位十六进制
		s, err := generateSecureToken(12)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成邀请码失败"})
			return
		}
		req.Code = s
	}
	if req.MaxUses <= 0 {
		req.MaxUses = 1
	}
	ic := &model.InviteCode{
		Code:    req.Code,
		MaxUses: req.MaxUses,
		Note:    req.Note,
	}
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		ic.ExpiresAt = &t
	}
	if creatorID, ok := c.Get("user_id"); ok {
		ic.CreatorID, _ = creatorID.(string)
	}
	if err := h.inviteRepo.Create(ic); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败（可能邀请码已存在）"})
		return
	}
	h.auditFromContext(c, "invite.create", "invite", ic.ID, ic.Code)
	c.JSON(http.StatusCreated, gin.H{"data": ic})
}

// DeleteInviteCode 删除邀请码
func (h *AdminHandler) DeleteInviteCode(c *gin.Context) {
	id := c.Param("id")
	if err := h.inviteRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	h.auditFromContext(c, "invite.delete", "invite", id, "")
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// auditFromContext 从 Gin 上下文中提取操作者信息并记录审计日志
func (h *AdminHandler) auditFromContext(c *gin.Context, action, targetType, targetID, detail string) {
	operatorID, _ := c.Get("user_id")
	operator, _ := c.Get("username")
	opIDStr, _ := operatorID.(string)
	opNameStr, _ := operator.(string)
	h.userService.Audit(opIDStr, opNameStr, action, targetType, targetID, detail, c.ClientIP())
}

// parsePagination 通用分页参数
func parsePagination(c *gin.Context, defaultSize, maxSize int) (page, size int) {
	page = 1
	size = defaultSize
	if p := c.Query("page"); p != "" {
		if v, err := parseIntDefault(p, 1); err == nil {
			page = v
		}
	}
	if s := c.Query("size"); s != "" {
		if v, err := parseIntDefault(s, defaultSize); err == nil {
			size = v
		}
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > maxSize {
		size = defaultSize
	}
	return
}

// ==================== 系统信息 ====================

// SystemInfo 系统信息
func (h *AdminHandler) SystemInfo(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 本进程内存占用：
	//  - alloc_mb：Go 堆上仍在使用的对象大小
	//  - sys_mb：进程已从操作系统申请的内存总量（近似 RSS/进程常驻内存）
	//  - process_used_mb：用于前端展示的"本进程占用"，取 Sys 作为进程内存占用
	processUsedMB := float64(memStats.Sys) / 1024 / 1024
	sysMem := gin.H{
		"alloc_mb":        float64(memStats.Alloc) / 1024 / 1024,
		"total_alloc_mb":  float64(memStats.TotalAlloc) / 1024 / 1024,
		"sys_mb":          processUsedMB,
		"process_used_mb": processUsedMB,
	}

	// 主机总内存仅用于展示"占主机百分比"参考值，不再作为主数值

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"version":    "0.1.0",
			"go_version": runtime.Version(),
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"cpus":       runtime.NumCPU(),
			"goroutines": runtime.NumGoroutine(),
			"memory":     sysMem,
			"hw_accel":   h.transcodeService.GetHWAccelInfo(),
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
