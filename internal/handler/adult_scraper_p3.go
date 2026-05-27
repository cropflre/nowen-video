// Package handler 番号刮削 P3~P5 扩展 API
// 功能：
//   - 批量刮削任务（启动/暂停/恢复/取消/状态/历史）
//   - 镜像管理（查询/健康检查/切换）
//   - 缓存管理
//   - 定时调度
//   - 失败分析报表
package handler

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

// ==================== 懒人刮削聚合状态 ====================

func (h *AdultScraperHandler) isLazyScraperReady() bool {
	if h == nil || h.cfg == nil || !h.cfg.AdultScraper.Enabled {
		return false
	}
	ac := h.cfg.AdultScraper
	return ac.EnableJavBus || ac.EnableJavDB || ac.EnableFreejavbt || ac.EnableJav321 ||
		ac.EnableFanza || ac.EnableMGStage || ac.EnableFC2Hub || strings.TrimSpace(ac.PythonServiceURL) != ""
}

func (h *AdultScraperHandler) ensureLazyScraperDefaults() error {
	if h == nil || h.cfg == nil {
		return nil
	}
	ac := &h.cfg.AdultScraper
	changed := false
	if !ac.Enabled {
		ac.Enabled = true
		changed = true
	}
	if !ac.EnableJavBus && !ac.EnableJavDB && !ac.EnableFreejavbt && !ac.EnableJav321 &&
		!ac.EnableFanza && !ac.EnableMGStage && !ac.EnableFC2Hub && strings.TrimSpace(ac.PythonServiceURL) == "" {
		ac.EnableJavBus = true
		ac.EnableJavDB = true
		changed = true
	}
	if ac.MinRequestInterval <= 0 {
		ac.MinRequestInterval = 1500
		changed = true
	}
	if ac.MaxRequestInterval <= 0 {
		ac.MaxRequestInterval = 3000
		changed = true
	}
	if ac.MaxRequestInterval < ac.MinRequestInterval {
		ac.MaxRequestInterval = ac.MinRequestInterval + 1500
		changed = true
	}
	if changed {
		return h.cfg.SaveAdultScraperConfig()
	}
	return nil
}

func folderTaskStartedAt(task *service.FolderBatchTask) time.Time {
	if task == nil {
		return time.Time{}
	}
	return task.StartedAt
}

func mediaTaskStartedAt(task *service.AdultBatchTask) time.Time {
	if task == nil {
		return time.Time{}
	}
	return task.StartedAt
}

func lazyTaskView(kind string, task any) gin.H {
	switch t := task.(type) {
	case *service.FolderBatchTask:
		if t == nil {
			return nil
		}
		return gin.H{
			"id":          t.ID,
			"kind":        kind,
			"status":      t.Status,
			"total":       t.Total,
			"current":     t.Current,
			"success":     t.Success,
			"failed":      t.Failed,
			"skipped":     t.Skipped,
			"started_at":  t.StartedAt,
			"finished_at": t.FinishedAt,
			"aggregated":  t.Aggregated,
			"concurrency": t.Concurrency,
		}
	case *service.AdultBatchTask:
		if t == nil {
			return nil
		}
		return gin.H{
			"id":          t.ID,
			"kind":        kind,
			"status":      t.Status,
			"total":       t.Total,
			"current":     t.Current,
			"success":     t.Success,
			"failed":      t.Failed,
			"skipped":     t.Skipped,
			"started_at":  t.StartedAt,
			"finished_at": t.FinishedAt,
			"aggregated":  t.Aggregated,
			"concurrency": t.Concurrency,
		}
	default:
		return nil
	}
}

func lazyResultView(kind string, code, title, path, status, message, source string, at time.Time) gin.H {
	return gin.H{
		"kind":    kind,
		"code":    code,
		"title":   title,
		"path":    path,
		"status":  status,
		"message": message,
		"source":  source,
		"at":      at,
	}
}

// StartLazyScrape 一键懒人刮削：初始化默认配置、扫描目录、过滤已完成项并启动任务。
// POST /api/admin/adult-scraper/lazy/start
func (h *AdultScraperHandler) StartLazyScrape(c *gin.Context) {
	var req struct {
		Path        string `json:"path" binding:"required"`
		Recursive   *bool  `json:"recursive"`
		MaxDepth    int    `json:"max_depth"`
		Concurrency int    `json:"concurrency"`
		Aggregated  *bool  `json:"aggregated"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Path) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 path 参数"})
		return
	}
	if h.scraperService == nil || h.folderBatch == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "懒人刮削服务未初始化"})
		return
	}
	if err := h.ensureLazyScraperDefaults(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "初始化懒人默认配置失败: " + err.Error()})
		return
	}

	recursive := true
	if req.Recursive != nil {
		recursive = *req.Recursive
	}
	aggregated := true
	if req.Aggregated != nil {
		aggregated = *req.Aggregated
	}
	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = 2
	}

	scan, err := h.scraperService.ScanFolder(strings.TrimSpace(req.Path), recursive, req.MaxDepth)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	paths := make([]string, 0, scan.WithCode)
	for _, item := range scan.Entries {
		if item.HasCode && !item.HasNFO {
			paths = append(paths, item.Path)
		}
	}
	if len(paths) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "没有发现需要刮削的新视频，已有 NFO 的文件已自动跳过",
			"data": gin.H{
				"task_id": "",
				"queued":  0,
				"scan":    scan,
			},
		})
		return
	}

	taskID, err := h.folderBatch.Start(service.FolderBatchOptions{
		Paths:        paths,
		Aggregated:   aggregated,
		Concurrency:  concurrency,
		SkipIfHasNFO: true,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "懒人刮削已启动",
		"data": gin.H{
			"task_id": taskID,
			"queued":  len(paths),
			"scan":    scan,
		},
	})
}

// GetLazyStatus 返回懒人刮削页面所需的聚合状态。
// 前端主界面用一个轻量接口获取任务、报表、失败项、调度和缓存摘要，避免散落请求。
func (h *AdultScraperHandler) GetLazyStatus(c *gin.Context) {
	days := 7
	if v := c.Query("days"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			days = d
		}
	}

	var folderActive []*service.FolderBatchTask
	var folderHistory []*service.FolderBatchTask
	if h.folderBatch != nil {
		folderActive, folderHistory = h.folderBatch.List()
	}
	sort.Slice(folderActive, func(i, j int) bool {
		return folderTaskStartedAt(folderActive[i]).After(folderTaskStartedAt(folderActive[j]))
	})
	sort.Slice(folderHistory, func(i, j int) bool {
		return folderTaskStartedAt(folderHistory[i]).After(folderTaskStartedAt(folderHistory[j]))
	})

	var mediaActive []*service.AdultBatchTask
	var mediaHistory []*service.AdultBatchTask
	if h.batchService != nil {
		mediaActive = h.batchService.List()
	}
	if h.taskStore != nil {
		mediaHistory = h.taskStore.List()
	}
	sort.Slice(mediaActive, func(i, j int) bool {
		return mediaTaskStartedAt(mediaActive[i]).After(mediaTaskStartedAt(mediaActive[j]))
	})
	sort.Slice(mediaHistory, func(i, j int) bool {
		return mediaTaskStartedAt(mediaHistory[i]).After(mediaTaskStartedAt(mediaHistory[j]))
	})

	allTasks := make([]gin.H, 0, len(folderActive)+len(folderHistory)+len(mediaActive)+len(mediaHistory))
	for _, t := range folderActive {
		allTasks = append(allTasks, lazyTaskView("folder", t))
	}
	for _, t := range folderHistory {
		allTasks = append(allTasks, lazyTaskView("folder", t))
	}
	for _, t := range mediaActive {
		allTasks = append(allTasks, lazyTaskView("library", t))
	}
	for _, t := range mediaHistory {
		allTasks = append(allTasks, lazyTaskView("library", t))
	}
	sort.Slice(allTasks, func(i, j int) bool {
		ti, _ := allTasks[i]["started_at"].(time.Time)
		tj, _ := allTasks[j]["started_at"].(time.Time)
		return ti.After(tj)
	})
	var currentTask gin.H
	for _, task := range allTasks {
		if fmt.Sprint(task["status"]) == "running" {
			currentTask = task
			break
		}
	}
	if currentTask == nil && len(allTasks) > 0 {
		currentTask = allTasks[0]
	}

	recentResults := make([]gin.H, 0, 128)
	folderFailedCutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	folderFailedCount := 0
	for _, task := range append(folderActive, folderHistory...) {
		for _, item := range task.Results {
			if item.Status == "failed" && (days <= 0 || item.At.After(folderFailedCutoff)) {
				folderFailedCount++
			}
			title := item.Code
			if title == "" {
				title = item.Path
			}
			recentResults = append(recentResults, lazyResultView("folder", item.Code, title, item.Path, item.Status, item.Message, item.Source, item.At))
		}
	}
	for _, task := range append(mediaActive, mediaHistory...) {
		for _, item := range task.Results {
			recentResults = append(recentResults, lazyResultView("library", item.Code, item.MediaTitle, item.MediaID, item.Status, item.ErrMsg, item.Source, item.FinishedAt))
		}
	}
	sort.Slice(recentResults, func(i, j int) bool {
		ti, _ := recentResults[i]["at"].(time.Time)
		tj, _ := recentResults[j]["at"].(time.Time)
		return ti.After(tj)
	})
	if len(recentResults) > 100 {
		recentResults = recentResults[:100]
	}

	report := service.BuildReport(h.taskStore, days)
	failedItems := []service.AdultBatchItemResult{}
	if h.taskStore != nil {
		failedItems = h.taskStore.FailedItems(days)
	}

	scheduler := gin.H{"enabled": false}
	if h.scheduler != nil {
		scheduler = gin.H{
			"config":       h.scheduler.Config(),
			"last_run_at":  h.scheduler.LastRunAt(),
			"last_task_id": h.scheduler.LastTaskID(),
		}
	}
	cache := gin.H{"enabled": false}
	if h.cache != nil {
		cache = gin.H{"enabled": true, "stats": h.cache.Stats()}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"config_ready": h.isLazyScraperReady(),
			"folder_tasks": gin.H{
				"active":  folderActive,
				"history": folderHistory,
			},
			"media_tasks": gin.H{
				"active":  mediaActive,
				"history": mediaHistory,
			},
			"current_task":        currentTask,
			"recent_results":      recentResults,
			"report":              report,
			"failed_items":        failedItems,
			"failed_count":        len(failedItems) + folderFailedCount,
			"folder_failed_count": folderFailedCount,
			"scheduler":           scheduler,
			"cache":               cache,
			"days":                days,
		},
	})
}

// RetryLazyFailed 一键重试懒人失败项，覆盖目录刮削失败与媒体库批量失败。
// POST /api/admin/adult-scraper/lazy/retry-failed
func (h *AdultScraperHandler) RetryLazyFailed(c *gin.Context) {
	var req struct {
		Days        int  `json:"days"`
		Concurrency int  `json:"concurrency"`
		Aggregated  bool `json:"aggregated"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Days <= 0 {
		req.Days = 7
	}
	if req.Concurrency <= 0 {
		req.Concurrency = 2
	}
	if err := h.ensureLazyScraperDefaults(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "初始化懒人默认配置失败: " + err.Error()})
		return
	}

	cutoff := time.Now().Add(-time.Duration(req.Days) * 24 * time.Hour)
	folderPaths := []string{}
	folderSeen := map[string]struct{}{}
	if h.folderBatch != nil {
		active, history := h.folderBatch.List()
		for _, task := range append(active, history...) {
			for _, item := range task.Results {
				if item.Status != "failed" || item.Path == "" || item.Code == "" || item.At.Before(cutoff) {
					continue
				}
				if _, ok := folderSeen[item.Path]; ok {
					continue
				}
				folderSeen[item.Path] = struct{}{}
				folderPaths = append(folderPaths, item.Path)
			}
		}
	}

	mediaIDs := []string{}
	mediaSeen := map[string]struct{}{}
	if h.taskStore != nil {
		for _, item := range h.taskStore.FailedItems(req.Days) {
			if item.MediaID == "" {
				continue
			}
			if _, ok := mediaSeen[item.MediaID]; ok {
				continue
			}
			mediaSeen[item.MediaID] = struct{}{}
			mediaIDs = append(mediaIDs, item.MediaID)
		}
	}

	folderTaskID := ""
	if len(folderPaths) > 0 {
		if h.folderBatch == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "文件夹刮削服务未初始化"})
			return
		}
		id, err := h.folderBatch.Start(service.FolderBatchOptions{
			Paths:        folderPaths,
			Aggregated:   req.Aggregated,
			Concurrency:  req.Concurrency,
			SkipIfHasNFO: true,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		folderTaskID = id
	}

	mediaTaskID := ""
	if len(mediaIDs) > 0 {
		if h.batchService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "媒体库批量刮削服务未初始化"})
			return
		}
		id, err := h.batchService.Start(service.AdultBatchOptions{
			MediaIDs:    mediaIDs,
			Concurrency: req.Concurrency,
			Aggregated:  req.Aggregated,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		mediaTaskID = id
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"folder_task_id":     folderTaskID,
			"media_task_id":      mediaTaskID,
			"folder_retry_count": len(folderPaths),
			"media_retry_count":  len(mediaIDs),
			"retry_count":        len(folderPaths) + len(mediaIDs),
		},
	})
}

// ==================== 批量刮削 ====================

// StartBatch 启动批量任务
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
			"sources":        out,
			"last_health_at": h.proxyManager.LastHealthAt(),
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
			"config":       h.scheduler.Config(),
			"last_run_at":  h.scheduler.LastRunAt(),
			"last_task_id": h.scheduler.LastTaskID(),
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
		c.JSON(http.StatusOK, gin.H{"data": service.BuildReport(nil, days)})
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
		c.JSON(http.StatusOK, gin.H{"data": []any{}, "count": 0})
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
			"task_id":     taskID,
			"retry_count": len(ids),
		},
	})
}
