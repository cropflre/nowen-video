package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// AIHandler AI 功能处理器
type AIHandler struct {
	aiService *service.AIService
	logger    *zap.SugaredLogger
}

// SmartSearch AI 智能搜索（解析自然语言查询）
func (h *AIHandler) SmartSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "搜索关键词不能为空"})
		return
	}

	intent, err := h.aiService.ParseSearchIntent(query)
	if err != nil {
		h.logger.Warnf("AI 智能搜索失败: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"data": service.SearchIntent{
				Query:  query,
				Parsed: false,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": intent})
}

// GetAIStatus 获取 AI 服务状态（管理员）
func (h *AIHandler) GetAIStatus(c *gin.Context) {
	status := h.aiService.GetStatus()
	c.JSON(http.StatusOK, gin.H{"data": status})
}

// UpdateAIConfig 更新 AI 配置（管理员）
func (h *AIHandler) UpdateAIConfig(c *gin.Context) {
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	if err := h.aiService.UpdateConfig(updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	status := h.aiService.GetStatus()
	c.JSON(http.StatusOK, gin.H{"message": "AI 配置已更新", "data": status})
}

// TestAIConnection 测试 AI API 连接（管理员）
func (h *AIHandler) TestAIConnection(c *gin.Context) {
	result, err := h.aiService.TestConnection()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": result})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ClearAICache 清空 AI 缓存（管理员）
func (h *AIHandler) ClearAICache(c *gin.Context) {
	count := h.aiService.ClearCache()
	c.JSON(http.StatusOK, gin.H{
		"message": "AI 缓存已清空",
		"data":    gin.H{"cleared": count},
	})
}

// GetAICacheStats 获取 AI 缓存统计（管理员）
func (h *AIHandler) GetAICacheStats(c *gin.Context) {
	stats := h.aiService.GetCacheStats()
	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// GetAIErrorLogs 获取 AI 错误日志（管理员）
func (h *AIHandler) GetAIErrorLogs(c *gin.Context) {
	logs := h.aiService.GetErrorLogs()
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

// TestSmartSearch 测试智能搜索功能（管理员）
func (h *AIHandler) TestSmartSearch(c *gin.Context) {
	var req struct {
		Query string `json:"query" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 query 参数"})
		return
	}

	result, err := h.aiService.TestSmartSearch(req.Query)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"success": false, "error": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// TestRecommendReason 测试推荐理由生成（管理员）
func (h *AIHandler) TestRecommendReason(c *gin.Context) {
	var req struct {
		Title  string `json:"title" binding:"required"`
		Genres string `json:"genres"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 title 参数"})
		return
	}

	result, err := h.aiService.TestRecommendReason(req.Title, req.Genres)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"success": false, "error": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}
