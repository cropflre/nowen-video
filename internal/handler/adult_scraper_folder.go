// Package handler - 文件夹扫描 & 自定义文件夹刮削 API
// 配套前端：文件夹导航 + 自定义路径批量刮削 Tab
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

// ==================== 文件夹扫描 ====================

// ScanFolder 扫描指定路径下的视频文件并识别番号
// GET /api/admin/adult-scraper/folder/scan?path=/mnt/movies&recursive=true&max_depth=3
func (h *AdultScraperHandler) ScanFolder(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 path 参数"})
		return
	}
	recursive, _ := strconv.ParseBool(c.DefaultQuery("recursive", "true"))
	maxDepth, _ := strconv.Atoi(c.DefaultQuery("max_depth", "0"))

	if h.scraperService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "番号刮削服务未初始化"})
		return
	}
	result, err := h.scraperService.ScanFolder(path, recursive, maxDepth)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ==================== 自定义文件夹批量刮削 ====================

// StartFolderBatch 对一批视频文件路径启动批量刮削
// POST /api/admin/adult-scraper/folder/batch/start
// body: { paths: []string, aggregated: bool, concurrency: int, skip_if_has_nfo: bool }
func (h *AdultScraperHandler) StartFolderBatch(c *gin.Context) {
	var req struct {
		Paths         []string `json:"paths" binding:"required"`
		Aggregated    bool     `json:"aggregated"`
		Concurrency   int      `json:"concurrency"`
		SkipIfHasNFO  bool     `json:"skip_if_has_nfo"`
		OverrideCode  string   `json:"override_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Paths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 paths 列表"})
		return
	}
	if h.folderBatch == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "文件夹刮削服务未初始化"})
		return
	}
	taskID, err := h.folderBatch.Start(service.FolderBatchOptions{
		Paths:        req.Paths,
		Aggregated:   req.Aggregated,
		Concurrency:  req.Concurrency,
		SkipIfHasNFO: req.SkipIfHasNFO,
		OverrideCode: req.OverrideCode,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

// GetFolderBatch 查询单个文件夹刮削任务
// GET /api/admin/adult-scraper/folder/batch/:id
func (h *AdultScraperHandler) GetFolderBatch(c *gin.Context) {
	id := c.Param("id")
	if h.folderBatch == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "文件夹刮削服务未初始化"})
		return
	}
	task := h.folderBatch.Get(id)
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": task})
}

// ListFolderBatch 列出所有文件夹刮削任务（活跃 + 历史）
// GET /api/admin/adult-scraper/folder/batch
func (h *AdultScraperHandler) ListFolderBatch(c *gin.Context) {
	if h.folderBatch == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "文件夹刮削服务未初始化"})
		return
	}
	active, history := h.folderBatch.List()
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"active": active, "history": history}})
}

// CancelFolderBatch 取消文件夹刮削任务
// POST /api/admin/adult-scraper/folder/batch/:id/cancel
func (h *AdultScraperHandler) CancelFolderBatch(c *gin.Context) {
	id := c.Param("id")
	if h.folderBatch == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "文件夹刮削服务未初始化"})
		return
	}
	if err := h.folderBatch.Cancel(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已取消"})
}

// ==================== Cookie 连通性测试 ====================

// TestCookie 测试指定站点 Cookie 是否有效
// GET /api/admin/adult-scraper/cookie/test?site=javdb
func (h *AdultScraperHandler) TestCookie(c *gin.Context) {
	site := c.Query("site")
	if site == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 site 参数（javbus/javdb/freejavbt/jav321/fanza/mgstage/fc2hub）"})
		return
	}
	if h.scraperService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "番号刮削服务未初始化"})
		return
	}
	ok, message, code := h.scraperService.TestCookieLogin(site)
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"ok":          ok,
			"message":     message,
			"status_code": code,
			"site":        site,
		},
	})
}
