package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authService *service.AuthService
	serverName  string // 服务器名称（用于局域网发现识别）
	logger      *zap.SugaredLogger
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	token, err := h.authService.Login(&req, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		switch err {
		case service.ErrUserDisabled:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, token)
}

// Status 获取系统初始化状态（公开接口，无需认证）
// 同时返回服务器身份信息，供安卓端局域网发现时识别 NowenVideo 服务器
func (h *AuthHandler) Status(c *gin.Context) {
	status, err := h.authService.GetInitStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取状态失败"})
		return
	}

	// 构建响应：在原有 InitStatus 基础上追加服务器身份字段
	serverName := h.serverName
	if serverName == "" {
		serverName = "NowenVideo"
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"initialized":       status.Initialized,
			"registration_open": status.RegistrationOpen,
			"invite_required":   status.InviteRequired,
			// 服务器身份信息（供局域网发现识别）
			"server_name":  serverName,
			"server_type":  "nowen-video",
			"version":      "0.1.0",
		},
	})
}

// Register 用户注册
func (h *AuthHandler) Register(c *gin.Context) {
	var req service.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效，用户名至少3位，密码至少6位"})
		return
	}

	token, err := h.authService.Register(&req, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		switch err {
		case service.ErrUserExists:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case service.ErrRegistrationDisabled:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case service.ErrInvalidInviteCode:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
		}
		return
	}

	c.JSON(http.StatusCreated, token)
}

// RefreshToken 刷新令牌（此接口需要认证中间件保护）
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists || userIDVal == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供有效的认证信息"})
		return
	}

	userID, ok := userIDVal.(string)
	if !ok || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证信息无效"})
		return
	}

	token, err := h.authService.RefreshToken(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "令牌刷新失败"})
		return
	}
	c.JSON(http.StatusOK, token)
}

// ChangePassword 修改密码（需要认证）
// 成功后旧 Token 失效，前端需用返回的新 Token 继续访问。
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req service.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效，新密码至少6位"})
		return
	}

	if err := h.authService.ChangePassword(userID.(string), &req); err != nil {
		switch err {
		case service.ErrInvalidCredentials:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "当前密码错误"})
		case service.ErrUserNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		default:
			h.logger.Errorf("修改密码失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "修改密码失败"})
		}
		return
	}

	// 修改密码后自动签发新 Token，避免用户被强制退出登录
	token, err := h.authService.RefreshToken(userID.(string))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "密码修改成功，请重新登录"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功", "data": token})
}
