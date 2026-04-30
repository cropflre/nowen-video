package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

// PreprocessHandler 视频预处理 API 处理器
type PreprocessHandler struct {
	preprocessService *service.PreprocessService
}

func NewPreprocessHandler(preprocessService *service.PreprocessService) *PreprocessHandler {
	return &PreprocessHandler{preprocessService: preprocessService}
}

// SubmitMedia 提交单个媒体预处理
//
// 请求体支持 force 字段：
//   - true：绕过"可直接播放则跳过"的判定，用于用户在前端显式点击"预处理/强制转码"按钮的场景；
//   - false 或不传：自动路径默认行为，如果媒体可在浏览器零转码直接播放则拒绝入队。
func (h *PreprocessHandler) SubmitMedia(c *gin.Context) {
	var req struct {
		MediaID  string `json:"media_id" binding:"required"`
		Priority int    `json:"priority"`
		Force    bool   `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 media_id"})
		return
	}

	task, err := h.preprocessService.SubmitMedia(req.MediaID, req.Priority, req.Force)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "预处理任务已提交",
		"data":    task,
	})
}

// BatchSubmit 批量提交预处理
//
// 请求体 force 字段语义同 SubmitMedia。
func (h *PreprocessHandler) BatchSubmit(c *gin.Context) {
	var req struct {
		MediaIDs []string `json:"media_ids" binding:"required"`
		Priority int      `json:"priority"`
		Force    bool     `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 media_ids"})
		return
	}

	tasks, err := h.preprocessService.BatchSubmit(req.MediaIDs, req.Priority, req.Force)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "批量预处理任务已提交",
		"data": gin.H{
			"submitted": len(tasks),
			"tasks":     tasks,
		},
	})
}

// SubmitLibrary 提交整个媒体库预处理
func (h *PreprocessHandler) SubmitLibrary(c *gin.Context) {
	libraryID := c.Param("id")
	var req struct {
		Priority int `json:"priority"`
	}
	c.ShouldBindJSON(&req)

	count, err := h.preprocessService.SubmitLibrary(libraryID, req.Priority)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "媒体库预处理任务已提交",
		"data": gin.H{
			"submitted": count,
		},
	})
}

// GetTask 获取任务详情
func (h *PreprocessHandler) GetTask(c *gin.Context) {
	taskID := c.Param("id")

	task, err := h.preprocessService.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": task})
}

// GetMediaTask 获取媒体的预处理状态
func (h *PreprocessHandler) GetMediaTask(c *gin.Context) {
	mediaID := c.Param("id")

	task, err := h.preprocessService.GetMediaTask(mediaID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"media_id": mediaID,
				"status":   "none",
				"message":  "未预处理",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": task})
}

// ListTasks 分页获取任务列表
func (h *PreprocessHandler) ListTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	tasks, total, err := h.preprocessService.ListTasks(page, pageSize, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
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

// PauseTask 暂停任务
func (h *PreprocessHandler) PauseTask(c *gin.Context) {
	taskID := c.Param("id")
	if err := h.preprocessService.PauseTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已暂停"})
}

// ResumeTask 恢复任务
func (h *PreprocessHandler) ResumeTask(c *gin.Context) {
	taskID := c.Param("id")
	if err := h.preprocessService.ResumeTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已恢复"})
}

// CancelTask 取消任务
func (h *PreprocessHandler) CancelTask(c *gin.Context) {
	taskID := c.Param("id")
	if err := h.preprocessService.CancelTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已取消"})
}

// RetryTask 重试任务
func (h *PreprocessHandler) RetryTask(c *gin.Context) {
	taskID := c.Param("id")
	if err := h.preprocessService.RetryTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已重新提交"})
}

// DeleteTask 删除任务
func (h *PreprocessHandler) DeleteTask(c *gin.Context) {
	taskID := c.Param("id")
	if err := h.preprocessService.DeleteTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已删除"})
}

// BatchDeleteTasks 批量删除预处理任务
func (h *PreprocessHandler) BatchDeleteTasks(c *gin.Context) {
	var req struct {
		TaskIDs []string `json:"task_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 task_ids"})
		return
	}

	deleted, err := h.preprocessService.BatchDeleteTasks(req.TaskIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "批量删除完成",
		"data":    gin.H{"deleted": deleted},
	})
}

// BatchCancelTasks 批量取消预处理任务
func (h *PreprocessHandler) BatchCancelTasks(c *gin.Context) {
	var req struct {
		TaskIDs []string `json:"task_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 task_ids"})
		return
	}

	cancelled, err := h.preprocessService.BatchCancelTasks(req.TaskIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "批量取消完成",
		"data":    gin.H{"cancelled": cancelled},
	})
}

// BatchRetryTasks 批量重试预处理任务
func (h *PreprocessHandler) BatchRetryTasks(c *gin.Context) {
	var req struct {
		TaskIDs []string `json:"task_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 task_ids"})
		return
	}

	retried, err := h.preprocessService.BatchRetryTasks(req.TaskIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "批量重试完成",
		"data":    gin.H{"retried": retried},
	})
}

// GetStatistics 获取预处理统计
func (h *PreprocessHandler) GetStatistics(c *gin.Context) {
	stats := h.preprocessService.GetStatistics()
	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// GetSystemLoad 获取系统负载
func (h *PreprocessHandler) GetSystemLoad(c *gin.Context) {
	load := h.preprocessService.GetSystemLoad()
	c.JSON(http.StatusOK, gin.H{"data": load})
}

// CleanCache 清理预处理缓存
func (h *PreprocessHandler) CleanCache(c *gin.Context) {
	mediaID := c.Param("id")
	if err := h.preprocessService.CleanPreprocessCache(mediaID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "预处理缓存已清理"})
}

// ServePreprocessedMaster 提供预处理后的 HLS 主播放列表
func (h *PreprocessHandler) ServePreprocessedMaster(c *gin.Context) {
	mediaID := c.Param("id")

	masterPath, err := h.preprocessService.GetPreprocessedMasterPath(mediaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "public, max-age=3600")
	c.File(masterPath)
}

// ServePreprocessedSegment 提供预处理后的 HLS 分片
func (h *PreprocessHandler) ServePreprocessedSegment(c *gin.Context) {
	mediaID := c.Param("id")
	quality := c.Param("quality")
	segment := c.Param("segment")

	task, err := h.preprocessService.GetMediaTask(mediaID)
	if err != nil || task.Status != "completed" {
		c.JSON(http.StatusNotFound, gin.H{"error": "预处理未完成"})
		return
	}

	// 构建文件路径
	var filePath string
	if segment == "stream.m3u8" {
		filePath = task.OutputDir + "/hls/" + quality + "/stream.m3u8"
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
	} else {
		filePath = task.OutputDir + "/hls/" + quality + "/" + segment
		c.Header("Content-Type", "video/mp2t")
	}

	c.Header("Cache-Control", "public, max-age=604800")
	c.File(filePath)
}

// ServeThumbnail 提供预处理的封面缩略图
func (h *PreprocessHandler) ServeThumbnail(c *gin.Context) {
	mediaID := c.Param("id")

	task, err := h.preprocessService.GetMediaTask(mediaID)
	if err != nil || task.ThumbnailPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "封面不存在"})
		return
	}

	c.Header("Content-Type", "image/jpeg")
	c.Header("Cache-Control", "public, max-age=604800")
	c.File(task.ThumbnailPath)
}

// ServeKeyframe 提供关键帧预览
func (h *PreprocessHandler) ServeKeyframe(c *gin.Context) {
	mediaID := c.Param("id")
	index := c.Param("index")

	task, err := h.preprocessService.GetMediaTask(mediaID)
	if err != nil || task.KeyframesDir == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "关键帧不存在"})
		return
	}

	filePath := task.KeyframesDir + "/kf_" + index + ".jpg"
	c.Header("Content-Type", "image/jpeg")
	c.Header("Cache-Control", "public, max-age=604800")
	c.File(filePath)
}

// ServeSprite 提供进度条预览雪碧图
func (h *PreprocessHandler) ServeSprite(c *gin.Context) {
	mediaID := c.Param("id")

	task, err := h.preprocessService.GetMediaTask(mediaID)
	if err != nil || task.SpritePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "雪碧图不存在"})
		return
	}

	c.Header("Content-Type", "image/jpeg")
	c.Header("Cache-Control", "public, max-age=604800")
	c.File(task.SpritePath)
}

// ServeSpriteVTT 提供进度条预览 WebVTT 索引文件
func (h *PreprocessHandler) ServeSpriteVTT(c *gin.Context) {
	mediaID := c.Param("id")

	task, err := h.preprocessService.GetMediaTask(mediaID)
	if err != nil || task.SpriteVTTPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "WebVTT 索引不存在"})
		return
	}

	c.Header("Content-Type", "text/vtt; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=604800")
	c.File(task.SpriteVTTPath)
}
