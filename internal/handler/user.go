package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// UserHandler 用户处理器
type UserHandler struct {
	userService  *service.UserService
	authService  *service.AuthService
	mediaService *service.MediaService
	loginLogRepo *repository.LoginLogRepo
	logger       *zap.SugaredLogger
}

// Profile 获取当前用户信息
func (h *UserHandler) Profile(c *gin.Context) {
	userID, _ := c.Get("user_id")
	user, err := h.userService.GetProfile(userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": user})
}

// UpdateProfile 更新当前用户的资料（用户名/昵称/邮箱/头像）
// 若用户名发生变更，会一并下发新 Token，避免当前会话因 token_version 自增而掉线。
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req service.UpdateSelfProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}
	user, usernameChanged, err := h.userService.UpdateSelfProfile(userID.(string), &req)
	if err != nil {
		switch err {
		case service.ErrUserExists:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case service.ErrUserNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			h.logger.Errorf("更新资料失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		}
		return
	}

	resp := gin.H{"data": user}
	if usernameChanged && h.authService != nil {
		if token, tokenErr := h.authService.RefreshToken(userID.(string)); tokenErr == nil {
			resp["token"] = token.Token
			resp["expires_at"] = token.ExpiresAt
		}
	}
	c.JSON(http.StatusOK, resp)
}

// LoginLogs 当前用户查看自己的登录历史
func (h *UserHandler) LoginLogs(c *gin.Context) {
	userID, _ := c.Get("user_id")
	if h.loginLogRepo == nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	limit := 20
	logs, err := h.loginLogRepo.ListByUser(userID.(string), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

// UpdateProgressRequest 更新观看进度请求
type UpdateProgressRequest struct {
	Position float64 `json:"position" binding:"required"`
	Duration float64 `json:"duration" binding:"required"`
}

// UpdateProgress 更新观看进度
func (h *UserHandler) UpdateProgress(c *gin.Context) {
	userID, _ := c.Get("user_id")
	mediaID := c.Param("mediaId")

	var req UpdateProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if err := h.mediaService.UpdateProgress(userID.(string), mediaID, req.Position, req.Duration); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新进度失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

// Favorites 获取收藏列表
func (h *UserHandler) Favorites(c *gin.Context) {
	userID, _ := c.Get("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	favs, total, err := h.mediaService.ListFavorites(userID.(string), page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取收藏列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  favs,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// AddFavorite 添加收藏
func (h *UserHandler) AddFavorite(c *gin.Context) {
	userID, _ := c.Get("user_id")
	mediaID := c.Param("mediaId")

	if err := h.mediaService.AddFavorite(userID.(string), mediaID); err != nil {
		if err == service.ErrAlreadyFavorited {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "收藏失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "已收藏"})
}

// RemoveFavorite 移除收藏
func (h *UserHandler) RemoveFavorite(c *gin.Context) {
	userID, _ := c.Get("user_id")
	mediaID := c.Param("mediaId")

	if err := h.mediaService.RemoveFavorite(userID.(string), mediaID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "取消收藏失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已取消收藏"})
}

// CheckFavorite 检查是否已收藏
func (h *UserHandler) CheckFavorite(c *gin.Context) {
	userID, _ := c.Get("user_id")
	mediaID := c.Param("mediaId")

	favorited := h.mediaService.IsFavorited(userID.(string), mediaID)
	c.JSON(http.StatusOK, gin.H{"data": favorited})
}

// GetProgress 获取用户对指定媒体的观看进度
func (h *UserHandler) GetProgress(c *gin.Context) {
	userID, _ := c.Get("user_id")
	mediaID := c.Param("mediaId")

	progress, err := h.mediaService.GetProgress(userID.(string), mediaID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": progress})
}

// History 获取观看历史列表
func (h *UserHandler) History(c *gin.Context) {
	userID, _ := c.Get("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 20
	}

	histories, total, err := h.mediaService.ListHistory(userID.(string), page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取观看历史失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  histories,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// DeleteHistory 删除单条观看记录
func (h *UserHandler) DeleteHistory(c *gin.Context) {
	userID, _ := c.Get("user_id")
	mediaID := c.Param("mediaId")

	if err := h.mediaService.DeleteHistory(userID.(string), mediaID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除观看记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ClearHistory 清空观看历史
func (h *UserHandler) ClearHistory(c *gin.Context) {
	userID, _ := c.Get("user_id")

	if err := h.mediaService.ClearHistory(userID.(string)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清空观看历史失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已清空"})
}
