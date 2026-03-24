package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// PlaylistHandler 播放列表处理器
type PlaylistHandler struct {
	playlistService *service.PlaylistService
	logger          *zap.SugaredLogger
}

// CreatePlaylistRequest 创建播放列表请求
type CreatePlaylistRequest struct {
	Name string `json:"name" binding:"required"`
}

// List 获取用户的播放列表
func (h *PlaylistHandler) List(c *gin.Context) {
	userID, _ := c.Get("user_id")
	playlists, err := h.playlistService.List(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取播放列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": playlists})
}

// Create 创建播放列表
func (h *PlaylistHandler) Create(c *gin.Context) {
	var req CreatePlaylistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID, _ := c.Get("user_id")
	playlist, err := h.playlistService.Create(userID.(string), req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建播放列表失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": playlist})
}

// Detail 获取播放列表详情
func (h *PlaylistHandler) Detail(c *gin.Context) {
	id := c.Param("id")
	playlist, err := h.playlistService.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "播放列表不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": playlist})
}

// Delete 删除播放列表
func (h *PlaylistHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	if err := h.playlistService.Delete(id, userID.(string)); err != nil {
		if err == service.ErrForbidden {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权操作"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除播放列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// AddItem 添加媒体到播放列表
func (h *PlaylistHandler) AddItem(c *gin.Context) {
	playlistID := c.Param("id")
	mediaID := c.Param("mediaId")
	userID, _ := c.Get("user_id")

	if err := h.playlistService.AddItem(playlistID, mediaID, userID.(string)); err != nil {
		if err == service.ErrForbidden {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权操作"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加失败"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "已添加"})
}

// RemoveItem 从播放列表移除媒体
func (h *PlaylistHandler) RemoveItem(c *gin.Context) {
	playlistID := c.Param("id")
	mediaID := c.Param("mediaId")
	userID, _ := c.Get("user_id")

	if err := h.playlistService.RemoveItem(playlistID, mediaID, userID.(string)); err != nil {
		if err == service.ErrForbidden {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权操作"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "移除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已移除"})
}
