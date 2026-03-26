package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// AIAssistantHandler AI助手HTTP处理器
type AIAssistantHandler struct {
	assistantService *service.AIAssistantService
	logger           *zap.SugaredLogger
}

// Chat 处理AI助手对话
func (h *AIAssistantHandler) Chat(c *gin.Context) {
	var req service.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "消息内容不能为空"})
		return
	}

	userID := c.GetString("user_id")
	resp, err := h.assistantService.Chat(req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// ExecuteAction 执行AI建议的操作
func (h *AIAssistantHandler) ExecuteAction(c *gin.Context) {
	var req service.ExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID := c.GetString("user_id")
	resp, err := h.assistantService.ExecuteAction(req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// UndoOperation 撤销操作
func (h *AIAssistantHandler) UndoOperation(c *gin.Context) {
	opID := c.Param("opId")
	if opID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "操作ID不能为空"})
		return
	}

	userID := c.GetString("user_id")
	resp, err := h.assistantService.UndoOperation(opID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// GetSession 获取会话信息
func (h *AIAssistantHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	session, err := h.assistantService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": session})
}

// DeleteSession 删除会话
func (h *AIAssistantHandler) DeleteSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	h.assistantService.DeleteSession(sessionID)
	c.JSON(http.StatusOK, gin.H{"message": "会话已删除"})
}

// GetOperationHistory 获取操作历史
func (h *AIAssistantHandler) GetOperationHistory(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	userID := c.GetString("user_id")
	history := h.assistantService.GetOperationHistory(userID, limit)
	c.JSON(http.StatusOK, gin.H{"data": history})
}

// AnalyzeMisclassification 分析文件库中的误分类情况
func (h *AIAssistantHandler) AnalyzeMisclassification(c *gin.Context) {
	report, err := h.assistantService.AnalyzeMisclassification()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": report})
}

// ReclassifyFiles 批量重分类文件
func (h *AIAssistantHandler) ReclassifyFiles(c *gin.Context) {
	var req service.ReclassifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID := c.GetString("user_id")
	result, err := h.assistantService.ReclassifyFiles(req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}
