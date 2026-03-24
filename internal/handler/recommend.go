package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// RecommendHandler 推荐处理器
type RecommendHandler struct {
	recommendService *service.RecommendService
	logger           *zap.SugaredLogger
}

// GetRecommendations 获取个性化推荐列表
func (h *RecommendHandler) GetRecommendations(c *gin.Context) {
	userID, _ := c.Get("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if limit < 1 || limit > 50 {
		limit = 20
	}

	recommendations, err := h.recommendService.GetRecommendations(userID.(string), limit)
	if err != nil {
		h.logger.Errorf("获取推荐列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取推荐失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": recommendations})
}
