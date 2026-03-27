package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// CommentHandler 评论处理器
type CommentHandler struct {
	commentService *service.CommentService
	logger         *zap.SugaredLogger
}

// CreateCommentRequest 创建评论请求
type CreateCommentRequest struct {
	Content string  `json:"content" binding:"required"`
	Rating  float64 `json:"rating"` // 可选评分
}

// Create 发表评论
func (h *CommentHandler) Create(c *gin.Context) {
	userID, _ := c.Get("user_id")
	mediaID := c.Param("id")

	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	comment, err := h.commentService.Create(userID.(string), mediaID, req.Content, req.Rating)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发表评论失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": comment})
}

// ListByMedia 获取媒体的评论
func (h *CommentHandler) ListByMedia(c *gin.Context) {
	mediaID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	comments, total, err := h.commentService.ListByMedia(mediaID, page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取评论失败"})
		return
	}

	// 获取平均评分
	avgRating, ratingCount, _ := h.commentService.GetAverageRating(mediaID)

	c.JSON(http.StatusOK, gin.H{
		"data":         comments,
		"total":        total,
		"page":         page,
		"size":         size,
		"avg_rating":   avgRating,
		"rating_count": ratingCount,
	})
}

// Delete 删除评论
func (h *CommentHandler) Delete(c *gin.Context) {
	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")
	commentID := c.Param("id")

	if err := h.commentService.Delete(userID.(string), commentID, role.(string)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除评论失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}
