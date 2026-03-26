package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// ScrapeManagerHandler 刮削管理 API 处理器
type ScrapeManagerHandler struct {
	scrapeService *service.ScrapeManagerService
	logger        *zap.SugaredLogger
}

// ==================== 任务创建 ====================

// CreateTask 创建单个刮削任务
func (h *ScrapeManagerHandler) CreateTask(c *gin.Context) {
	var req struct {
		URL       string `json:"url" binding:"required"`
		Source    string `json:"source"`
		MediaType string `json:"media_type"`
		Title     string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID, _ := c.Get("userID")

	task, err := h.scrapeService.CreateTask(req.URL, req.Source, req.MediaType, userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果提供了标题，更新
	if req.Title != "" {
		task.Title = req.Title
	}

	c.JSON(http.StatusOK, gin.H{"data": task, "message": "刮削任务已创建"})
}

// BatchCreateTasks 批量创建刮削任务
func (h *ScrapeManagerHandler) BatchCreateTasks(c *gin.Context) {
	var req struct {
		URLs      []string `json:"urls" binding:"required"`
		Source    string   `json:"source"`
		MediaType string   `json:"media_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID, _ := c.Get("userID")
	created, skipped, errors := h.scrapeService.BatchCreateTasks(req.URLs, req.Source, req.MediaType, userID.(string))

	c.JSON(http.StatusOK, gin.H{
		"message": "批量创建完成",
		"created": created,
		"skipped": skipped,
		"errors":  errors,
	})
}

// ==================== 刮削执行 ====================

// StartScrape 开始刮削
func (h *ScrapeManagerHandler) StartScrape(c *gin.Context) {
	taskID := c.Param("id")
	userID, _ := c.Get("userID")

	if err := h.scrapeService.StartScrape(taskID, userID.(string)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "刮削已启动"})
}

// BatchStartScrape 批量开始刮削
func (h *ScrapeManagerHandler) BatchStartScrape(c *gin.Context) {
	var req struct {
		TaskIDs []string `json:"task_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID, _ := c.Get("userID")
	started, errors := h.scrapeService.BatchStartScrape(req.TaskIDs, userID.(string))

	c.JSON(http.StatusOK, gin.H{
		"message": "批量刮削已启动",
		"started": started,
		"errors":  errors,
	})
}

// ==================== 翻译功能 ====================

// TranslateTask 翻译任务
func (h *ScrapeManagerHandler) TranslateTask(c *gin.Context) {
	taskID := c.Param("id")
	var req struct {
		TargetLang string   `json:"target_lang" binding:"required"`
		Fields     []string `json:"fields"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID, _ := c.Get("userID")
	if err := h.scrapeService.TranslateTask(taskID, req.TargetLang, userID.(string), req.Fields); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "翻译已启动"})
}

// BatchTranslate 批量翻译
func (h *ScrapeManagerHandler) BatchTranslate(c *gin.Context) {
	var req struct {
		TaskIDs    []string `json:"task_ids" binding:"required"`
		TargetLang string   `json:"target_lang" binding:"required"`
		Fields     []string `json:"fields"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID, _ := c.Get("userID")
	started, errors := h.scrapeService.BatchTranslate(req.TaskIDs, req.TargetLang, userID.(string), req.Fields)

	c.JSON(http.StatusOK, gin.H{
		"message": "批量翻译已启动",
		"started": started,
		"errors":  errors,
	})
}

// ==================== 数据管理 ====================

// ListTasks 列表查询
func (h *ScrapeManagerHandler) ListTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	status := c.Query("status")
	source := c.Query("source")

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	tasks, total, err := h.scrapeService.ListTasks(page, size, status, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  tasks,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GetTask 获取任务详情
func (h *ScrapeManagerHandler) GetTask(c *gin.Context) {
	taskID := c.Param("id")
	task, err := h.scrapeService.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": task})
}

// UpdateTask 更新任务
func (h *ScrapeManagerHandler) UpdateTask(c *gin.Context) {
	taskID := c.Param("id")
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID, _ := c.Get("userID")
	task, err := h.scrapeService.UpdateTask(taskID, updates, userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": task, "message": "更新成功"})
}

// DeleteTask 删除任务
func (h *ScrapeManagerHandler) DeleteTask(c *gin.Context) {
	taskID := c.Param("id")
	userID, _ := c.Get("userID")

	if err := h.scrapeService.DeleteTask(taskID, userID.(string)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// BatchDeleteTasks 批量删除
func (h *ScrapeManagerHandler) BatchDeleteTasks(c *gin.Context) {
	var req struct {
		TaskIDs []string `json:"task_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID, _ := c.Get("userID")
	deleted, err := h.scrapeService.BatchDeleteTasks(req.TaskIDs, userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "批量删除完成", "deleted": deleted})
}

// ExportTasks 导出任务数据
func (h *ScrapeManagerHandler) ExportTasks(c *gin.Context) {
	var req struct {
		TaskIDs []string `json:"task_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	data, err := h.scrapeService.ExportTasks(req.TaskIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

// ==================== 统计与历史 ====================

// GetStatistics 获取统计信息
func (h *ScrapeManagerHandler) GetStatistics(c *gin.Context) {
	stats, err := h.scrapeService.GetStatistics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取统计失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// GetHistory 获取操作历史
func (h *ScrapeManagerHandler) GetHistory(c *gin.Context) {
	taskID := c.Query("task_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 200 {
		limit = 50
	}

	histories, err := h.scrapeService.GetHistory(taskID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取历史失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": histories})
}
