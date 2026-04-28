// Package handler 番号刮削 P3~P5 扩展 API
// 功能：
//   - 批量刮削任务（启动/暂停/恢复/取消/状态/历史）
//   - 镜像管理（查询/健康检查/切换）
//   - 缓存管理
//   - 定时调度
//   - 失败分析报表
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

// ==================== 批量刮削 ====================

// StartBatch 启动批量刮削任务
// POST /api/admin/adult-scraper/batch/start
func (h *AdultScraperHandler) StartBatch(c *gin.Context) {
	var req struct {
		LibraryID     string   `json:"library_id"`
		MediaIDs      []string `json:"media_ids"`
		OnlyUnscraped bool     `json:"only_unscraped"`
		DryRun        bool     `json:"dry_run"`
		Concurrency   int      `json:"concurrency"`
		Aggregated    bool     `json:"aggregated"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}
	if h.batchService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "批量刮削服务未初始化"})
		return
	}

	taskID, err := h.batchService.Start(service.AdultBatchOptions{
		LibraryID:     req.LibraryID,
		MediaIDs:      req.MediaIDs,
		OnlyUnscraped: req.OnlyUnscraped,
		DryRun:        req.DryRun,
		Concurrency:   req.Concurrency,
		Aggregated:    req.Aggregated,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

// PauseBatch 暂停批量任务
// POST /api/admin/adult-scraper/batch/:id/pause
func (h *AdultScraperHandler) PauseBatch(c *gin.Context) {
	id := c.Param("id")
	if err := h.batchService.Pause(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已暂停"})
}

// ResumeBatch 恢复批量任务
func (h *AdultScraperHandler) ResumeBatch(c *gin.Context) {
	id := c.Param("id")
	if err := h.batchService.Resume(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已恢复"})
}

// CancelBatch 取消批量任务
func (h *AdultScraperHandler) CancelBatch(c *gin.Context) {
	id := c.Param("id")
	if err := h.batchService.Cancel(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已取消"})
}

// GetBatchStatus 查询批量任务状态
func (h *AdultScraperHandler) GetBatchStatus(c *gin.Context) {
	id := c.Param("id")
	task := h.batchService.Get(id)
	if task == nil {
		// 从持久化查
		if h.taskStore != nil {
			if t := h.taskStore.Get(id); t != nil {
				c.JSON(http.StatusOK, gin.H{"data": t})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": task})
}

// ListBatchTasks 列出所有活跃+历史任务
// GET /api/admin/adult-scraper/batch
func (h *AdultScraperHandler) ListBatchTasks(c *gin.Context) {
	var active []*service.AdultBatchTask
	var history []*service.AdultBatchTask
	if h.batchService != nil {
		active = h.batchService.List()
	}
	if h.taskStore != nil {
		history = h.taskStore.List()
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"active":  active,
			"history": history,
		},
	})
}

// ==================== 镜像管理 ====================

// ListMirrors 列出所有数据源的镜像状态
// GET /api/admin/adult-scraper/mirrors
func (h *AdultScraperHandler) ListMirrors(c *gin.Context) {
	if h.proxyManager == nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{}})
		return
	}
	sources := []string{"javbus", "javdb", "freejavbt", "jav321", "fanza", "mgstage", "fc2hub"}
	out := gin.H{}
	for _, src := range sources {
		out[src] = gin.H{
			"mirrors":   h.proxyManager.AllMirrors(src),
			"preferred": h.proxyManager.PreferredURL(src),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"sources":         out,
			"last_health_at":  h.proxyManager.LastHealthAt(),
		},
	})
}

// HealthCheckMirrors 主动触发镜像健康检查
// POST /api/admin/adult-scraper/mirrors/health-check
func (h *AdultScraperHandler) HealthCheckMirrors(c *gin.Context) {
	if h.proxyManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "代理管理器未初始化"})
		return
	}
	total, healthy := h.proxyManager.HealthCheckAll()
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"total":   total,
			"healthy": healthy,
		},
	})
}

// SetMirrors 覆盖某数据源的镜像列表
// POST /api/admin/adult-scraper/mirrors/:source
// Body: {"urls": ["https://xxx.com", "https://yyy.com"]}
func (h *AdultScraperHandler) SetMirrors(c *gin.Context) {
	src := c.Param("source")
	var req struct {
		URLs []string `json:"urls" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	h.proxyManager.SetMirrors(src, req.URLs)
	c.JSON(http.StatusOK, gin.H{"message": "镜像列表已更新", "source": src, "count": len(req.URLs)})
}

// ==================== 缓存管理 ====================

// GetCacheStats 获取缓存统计
// GET /api/admin/adult-scraper/cache
func (h *AdultScraperHandler) GetCacheStats(c *gin.Context) {
	if h.cache == nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"enabled": false}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": h.cache.Stats()})
}

// ClearCache 清空缓存
// DELETE /api/admin/adult-scraper/cache
func (h *AdultScraperHandler) ClearCache(c *gin.Context) {
	if h.cache == nil {
		c.JSON(http.StatusOK, gin.H{"message": "缓存未启用"})
		return
	}
	h.cache.Clear()
	c.JSON(http.StatusOK, gin.H{"message": "缓存已清空"})
}

// InvalidateCache 失效某个番号的缓存
// DELETE /api/admin/adult-scraper/cache/:code
func (h *AdultScraperHandler) InvalidateCache(c *gin.Context) {
	code := c.Param("code")
	if h.cache != nil {
		h.cache.Invalidate(code)
	}
	c.JSON(http.StatusOK, gin.H{"message": "已失效: " + code})
}

// ==================== 定时调度 ====================

// GetSchedulerConfig 查询调度器配置
// GET /api/admin/adult-scraper/scheduler
func (h *AdultScraperHandler) GetSchedulerConfig(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"enabled": false}})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"config":        h.scheduler.Config(),
			"last_run_at":   h.scheduler.LastRunAt(),
			"last_task_id":  h.scheduler.LastTaskID(),
		},
	})
}

// UpdateSchedulerConfig 更新调度器配置
// PUT /api/admin/adult-scraper/scheduler
func (h *AdultScraperHandler) UpdateSchedulerConfig(c *gin.Context) {
	var req service.AdultSchedulerConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "调度器未初始化"})
		return
	}
	h.scheduler.SetConfig(req)
	c.JSON(http.StatusOK, gin.H{"message": "调度器配置已更新"})
}

// TriggerScheduler 立即触发一次定时任务
// POST /api/admin/adult-scraper/scheduler/run
func (h *AdultScraperHandler) TriggerScheduler(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "调度器未初始化"})
		return
	}
	taskID, err := h.scheduler.RunOnce()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

// ==================== 失败分析报表 ====================

// GetReport 生成刮削报表
// GET /api/admin/adult-scraper/report?days=7
func (h *AdultScraperHandler) GetReport(c *gin.Context) {
	days := 0
	if v := c.Query("days"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			days = d
		}
	}
	if h.taskStore == nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{}})
		return
	}
	report := service.BuildReport(h.taskStore, days)
	c.JSON(http.StatusOK, gin.H{"data": report})
}

// GetFailedItems 获取最近的失败记录（用于一键重试）
// GET /api/admin/adult-scraper/failed-items?days=7
func (h *AdultScraperHandler) GetFailedItems(c *gin.Context) {
	days := 7
	if v := c.Query("days"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			days = d
		}
	}
	if h.taskStore == nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	items := h.taskStore.FailedItems(days)
	c.JSON(http.StatusOK, gin.H{"data": items, "count": len(items)})
}

// RetryFailed 一键重试最近失败的记录
// POST /api/admin/adult-scraper/retry-failed
// Body: {"days": 7, "concurrency": 2}
func (h *AdultScraperHandler) RetryFailed(c *gin.Context) {
	var req struct {
		Days        int  `json:"days"`
		Concurrency int  `json:"concurrency"`
		Aggregated  bool `json:"aggregated"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Days <= 0 {
		req.Days = 7
	}
	if h.taskStore == nil || h.batchService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "服务未初始化"})
		return
	}
	failed := h.taskStore.FailedItems(req.Days)
	ids := make([]string, 0, len(failed))
	seen := make(map[string]struct{})
	for _, f := range failed {
		if _, ok := seen[f.MediaID]; ok {
			continue
		}
		seen[f.MediaID] = struct{}{}
		if f.MediaID != "" {
			ids = append(ids, f.MediaID)
		}
	}
	if len(ids) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "没有可重试的失败记录"})
		return
	}
	taskID, err := h.batchService.Start(service.AdultBatchOptions{
		MediaIDs:    ids,
		Concurrency: req.Concurrency,
		Aggregated:  req.Aggregated,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"task_id":      taskID,
			"retry_count":  len(ids),
		},
	})
}
