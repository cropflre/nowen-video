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

// GetStorageUsage 获取预处理产物的磁盘占用统计
//
// 查询参数：
//   - limit：返回明细条目数（默认 20，最大 200，0=不限制）
func (h *PreprocessHandler) GetStorageUsage(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 0 {
		limit = 0
	}
	if limit > 200 {
		limit = 200
	}

	usage, err := h.preprocessService.GetStorageUsage(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": usage})
}

// CleanOrphanCache 清理所有孤儿预处理目录（DB 中无对应任务的目录）
func (h *PreprocessHandler) CleanOrphanCache(c *gin.Context) {
	cleaned, freed, err := h.preprocessService.CleanOrphanCache()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "孤儿预处理目录已清理",
		"data": gin.H{
			"cleaned":     cleaned,
			"freed_bytes": freed,
		},
	})
}

// GetCacheUsage 获取整个 cache/ 目录的分类占用统计
//
// 查询参数：
//   - force=1：跳过 30s 内存缓存，强制重新扫盘
func (h *PreprocessHandler) GetCacheUsage(c *gin.Context) {
	force := c.Query("force") == "1" || c.Query("force") == "true"
	usage, err := h.preprocessService.GetCacheUsage(force)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": usage})
}

// CleanCacheCategory 清空单个缓存分类目录
//
// 查询参数：
//   - key=transcode / preprocess / abr / thumbnails / webdav_download / ...
//   - mode=all（默认，真正清空）/ orphan（仅 preprocess 分类有效，只清孤儿）
//
// 仅对 cacheCategoryMeta 中 cleanable=true 的分类生效；其它返回 400。
func (h *PreprocessHandler) CleanCacheCategory(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 key 参数"})
		return
	}
	mode := c.Query("mode")
	res, err := h.preprocessService.CleanCacheCategory(key, mode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "已清理",
		"data":    res,
	})
}

// CleanAllCache 一键清空所有可清理分类
func (h *PreprocessHandler) CleanAllCache(c *gin.Context) {
	results, totalFreed, totalCount, err := h.preprocessService.CleanAllCleanableCache()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "一键清理完成",
		"data": gin.H{
			"results":      results,
			"total_freed":  totalFreed,
			"total_count":  totalCount,
			"category_num": len(results),
		},
	})
}

// ==================== 自定义筛选预处理 ====================

// preprocessFilterRequest 共用的请求体（预览 + 提交）
//
// exclude_* 三个字段都用指针，区分"未传 = 使用默认值 true"和"显式传 false"。
type preprocessFilterRequest struct {
	service.PreprocessFilter
	ExcludeAlreadyPreprocessedPtr *bool `json:"exclude_already_preprocessed_ptr,omitempty"`
	ExcludeDirectlyPlayablePtr    *bool `json:"exclude_directly_playable_ptr,omitempty"`
	ExcludeStrmPtr                *bool `json:"exclude_strm_ptr,omitempty"`
}

// 把请求体转成 service.PreprocessFilter，并应用默认值
func (r *preprocessFilterRequest) toServiceFilter() *service.PreprocessFilter {
	f := r.PreprocessFilter
	// 默认值：三个 exclude 都为 true（最安全的行为：不重复预处理 + 不浪费 CPU + 不动远程流）
	f.ExcludeAlreadyPreprocessed = true
	f.ExcludeDirectlyPlayable = true
	f.ExcludeStrm = true
	if r.ExcludeAlreadyPreprocessedPtr != nil {
		f.ExcludeAlreadyPreprocessed = *r.ExcludeAlreadyPreprocessedPtr
	}
	if r.ExcludeDirectlyPlayablePtr != nil {
		f.ExcludeDirectlyPlayable = *r.ExcludeDirectlyPlayablePtr
	}
	if r.ExcludeStrmPtr != nil {
		f.ExcludeStrm = *r.ExcludeStrmPtr
	}
	return &f
}

// PreviewByFilter 预览：根据筛选条件返回命中数量、抽样列表、分布直方图
//
// POST /api/admin/preprocess/filter-preview
func (h *PreprocessHandler) PreviewByFilter(c *gin.Context) {
	var req preprocessFilterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	preview, err := h.preprocessService.PreviewFilter(req.toServiceFilter())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": preview})
}

// SubmitByFilter 按筛选条件批量提交预处理任务
//
// POST /api/admin/preprocess/submit-by-filter
//
// 请求体除了 PreprocessFilter 字段外，还接受：
//   - priority：任务优先级（默认 0）
//   - force：是否强制（默认 false；true 会让 SubmitMedia 跳过"可直接播放"判定）
func (h *PreprocessHandler) SubmitByFilter(c *gin.Context) {
	var req struct {
		preprocessFilterRequest
		Priority int  `json:"priority"`
		Force    bool `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	submitted, skipped, err := h.preprocessService.SubmitByFilter(
		req.toServiceFilter(), req.Priority, req.Force,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "已按筛选条件提交预处理任务",
		"data": gin.H{
			"submitted": submitted,
			"skipped":   skipped,
		},
	})
}

// ListCandidateMedia 列出可供用户手动勾选预处理的影视列表
//
// GET /api/admin/preprocess/candidates
//
// Query 参数：
//   - page, size：分页（默认 1 / 20，size 上限 200）
//   - keyword：标题/原名/番号模糊匹配
//   - library_id：媒体库 ID
//   - media_type：movie / episode
//   - video_codec：视频编码
//   - only_need_preprocess：true 时仅返回"需要预处理"的（排除已完成/可直接播放/STRM）
//   - sort_by：updated_at(默认) / file_size / duration / year
//   - sort_order：desc(默认) / asc
func (h *PreprocessHandler) ListCandidateMedia(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	onlyNeed := c.Query("only_need_preprocess")
	params := service.PreprocessCandidateParams{
		Page:               page,
		Size:               size,
		Keyword:            c.Query("keyword"),
		LibraryID:          c.Query("library_id"),
		MediaType:          c.Query("media_type"),
		VideoCodec:         c.Query("video_codec"),
		OnlyNeedPreprocess: onlyNeed == "true" || onlyNeed == "1",
		SortBy:             c.Query("sort_by"),
		SortOrder:          c.Query("sort_order"),
	}

	result, err := h.preprocessService.ListCandidateMedia(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ServePreprocessedMaster 提供预处理后的 HLS 主播放列表
func (h *PreprocessHandler) ServePreprocessedMaster(c *gin.Context) {
	mediaID := c.Param("id")

	playlist, err := h.preprocessService.GetPreprocessedMasterPlaylist(mediaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, playlist)
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
