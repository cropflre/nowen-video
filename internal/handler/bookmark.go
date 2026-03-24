package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// BookmarkHandler 视频书签处理器
type BookmarkHandler struct {
	bookmarkService *service.BookmarkService
	logger          *zap.SugaredLogger
}

// CreateBookmarkRequest 创建书签请求
type CreateBookmarkRequest struct {
	MediaID  string  `json:"media_id" binding:"required"`
	Position float64 `json:"position" binding:"required"`
	Title    string  `json:"title" binding:"required"`
	Note     string  `json:"note"`
}

// Create 添加书签
func (h *BookmarkHandler) Create(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req CreateBookmarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	bookmark, err := h.bookmarkService.Create(userID.(string), req.MediaID, req.Title, req.Note, req.Position)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加书签失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": bookmark})
}

// ListByMedia 获取某个媒体的书签
func (h *BookmarkHandler) ListByMedia(c *gin.Context) {
	userID, _ := c.Get("user_id")
	mediaID := c.Param("mediaId")

	bookmarks, err := h.bookmarkService.ListByMedia(userID.(string), mediaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取书签失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": bookmarks})
}

// ListByUser 获取用户所有书签
func (h *BookmarkHandler) ListByUser(c *gin.Context) {
	userID, _ := c.Get("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	bookmarks, total, err := h.bookmarkService.ListByUser(userID.(string), page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取书签失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": bookmarks, "total": total, "page": page, "size": size})
}

// Delete 删除书签
func (h *BookmarkHandler) Delete(c *gin.Context) {
	userID, _ := c.Get("user_id")
	bookmarkID := c.Param("id")

	if err := h.bookmarkService.Delete(userID.(string), bookmarkID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除书签失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// UpdateBookmarkRequest 更新书签请求
type UpdateBookmarkRequest struct {
	Title string `json:"title" binding:"required"`
	Note  string `json:"note"`
}

// Update 更新书签
func (h *BookmarkHandler) Update(c *gin.Context) {
	userID, _ := c.Get("user_id")
	bookmarkID := c.Param("id")

	var req UpdateBookmarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if err := h.bookmarkService.Update(userID.(string), bookmarkID, req.Title, req.Note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新书签失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}
