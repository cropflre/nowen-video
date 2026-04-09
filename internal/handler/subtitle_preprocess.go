package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

// SubtitlePreprocessHandler 字幕预处理 API 处理器
type SubtitlePreprocessHandler struct {
	subtitlePreprocess *service.SubtitlePreprocessService
}

func NewSubtitlePreprocessHandler(svc *service.SubtitlePreprocessService) *SubtitlePreprocessHandler {
	return &SubtitlePreprocessHandler{subtitlePreprocess: svc}
}

// SubmitMedia 提交单个媒体字幕预处理
func (h *SubtitlePreprocessHandler) SubmitMedia(c *gin.Context) {
	var req struct {
		MediaID         string   `json:"media_id" binding:"required"`
		TargetLangs     []string `json:"target_langs"`
		ForceRegenerate bool     `json:"force_regenerate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 media_id"})
		return
	}

	task, err := h.subtitlePreprocess.SubmitMedia(req.MediaID, req.TargetLangs, req.ForceRegenerate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "字幕预处理任务已提交",
		"data":    task,
	})
}

// BatchSubmit 批量提交字幕预处理
func (h *SubtitlePreprocessHandler) BatchSubmit(c *gin.Context) {
	var req struct {
		MediaIDs        []string `json:"media_ids" binding:"required"`
		TargetLangs     []string `json:"target_langs"`
		ForceRegenerate bool     `json:"force_regenerate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 media_ids"})
		return
	}

	tasks, err := h.subtitlePreprocess.BatchSubmit(req.MediaIDs, req.TargetLangs, req.ForceRegenerate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "批量字幕预处理任务已提交",
		"data": gin.H{
			"submitted": len(tasks),
			"tasks":     tasks,
		},
	})
}

// SubmitLibrary 提交整个媒体库的字幕预处理
func (h *SubtitlePreprocessHandler) SubmitLibrary(c *gin.Context) {
	libraryID := c.Param("id")
	if libraryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供媒体库 ID"})
		return
	}

	var req struct {
		TargetLangs     []string `json:"target_langs"`
		ForceRegenerate bool     `json:"force_regenerate"`
	}
	c.ShouldBindJSON(&req)

	count, err := h.subtitlePreprocess.SubmitLibrary(libraryID, req.TargetLangs, req.ForceRegenerate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "媒体库字幕预处理任务已提交",
		"data": gin.H{
			"submitted": count,
		},
	})
}

// ListTasks 获取字幕预处理任务列表
func (h *SubtitlePreprocessHandler) ListTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	tasks, total, err := h.subtitlePreprocess.ListTasks(page, pageSize, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"tasks":     tasks,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetTask 获取字幕预处理任务详情
func (h *SubtitlePreprocessHandler) GetTask(c *gin.Context) {
	taskID := c.Param("id")
	task, err := h.subtitlePreprocess.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": task})
}

// CancelTask 取消字幕预处理任务
func (h *SubtitlePreprocessHandler) CancelTask(c *gin.Context) {
	taskID := c.Param("id")
	if err := h.subtitlePreprocess.CancelTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务已取消"})
}

// RetryTask 重试字幕预处理任务
func (h *SubtitlePreprocessHandler) RetryTask(c *gin.Context) {
	taskID := c.Param("id")
	if err := h.subtitlePreprocess.RetryTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务已重新提交"})
}

// DeleteTask 删除字幕预处理任务
func (h *SubtitlePreprocessHandler) DeleteTask(c *gin.Context) {
	taskID := c.Param("id")
	if err := h.subtitlePreprocess.DeleteTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务已删除"})
}

// GetStatistics 获取字幕预处理统计
func (h *SubtitlePreprocessHandler) GetStatistics(c *gin.Context) {
	stats := h.subtitlePreprocess.GetStatistics()
	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// BatchDeleteTasks 批量删除字幕预处理任务
func (h *SubtitlePreprocessHandler) BatchDeleteTasks(c *gin.Context) {
	var req struct {
		TaskIDs []string `json:"task_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 task_ids"})
		return
	}

	deleted, err := h.subtitlePreprocess.BatchDeleteTasks(req.TaskIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "批量删除完成",
		"data":    gin.H{"deleted": deleted},
	})
}

// BatchCancelTasks 批量取消字幕预处理任务
func (h *SubtitlePreprocessHandler) BatchCancelTasks(c *gin.Context) {
	var req struct {
		TaskIDs []string `json:"task_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 task_ids"})
		return
	}

	cancelled, err := h.subtitlePreprocess.BatchCancelTasks(req.TaskIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "批量取消完成",
		"data":    gin.H{"cancelled": cancelled},
	})
}

// BatchRetryTasks 批量重试字幕预处理任务
func (h *SubtitlePreprocessHandler) BatchRetryTasks(c *gin.Context) {
	var req struct {
		TaskIDs []string `json:"task_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 task_ids"})
		return
	}

	retried, err := h.subtitlePreprocess.BatchRetryTasks(req.TaskIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "批量重试完成",
		"data":    gin.H{"retried": retried},
	})
}

// GetMediaStatus 获取媒体的字幕预处理状态（用户可查询）
func (h *SubtitlePreprocessHandler) GetMediaStatus(c *gin.Context) {
	mediaID := c.Param("id")
	task, err := h.subtitlePreprocess.GetMediaTask(mediaID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"status":  "none",
				"message": "未进行字幕预处理",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": task})
}
