package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// CollectionHandler 电影系列合集处理器
type CollectionHandler struct {
	collectionService *service.CollectionService
	streamService     *service.StreamService
	logger            *zap.SugaredLogger
}

// GetMediaCollection 获取电影所属的合集
// GET /api/media/:id/collection
func (h *CollectionHandler) GetMediaCollection(c *gin.Context) {
	mediaID := c.Param("id")
	if mediaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少媒体ID"})
		return
	}

	result, err := h.collectionService.GetCollectionByMediaID(mediaID)
	if err != nil {
		// 没有合集不算错误，返回空
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// GetCollectionDetail 获取合集详情
// GET /api/collections/:id
func (h *CollectionHandler) GetCollectionDetail(c *gin.Context) {
	collectionID := c.Param("id")
	if collectionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少合集ID"})
		return
	}

	result, err := h.collectionService.GetCollectionDetail(collectionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "合集不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ListCollections 获取合集列表
// GET /api/collections
func (h *CollectionHandler) ListCollections(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	colls, total, err := h.collectionService.ListCollections(page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取合集列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  colls,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// SearchCollections 搜索合集
// GET /api/collections/search?keyword=xxx
func (h *CollectionHandler) SearchCollections(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少搜索关键词"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	colls, err := h.collectionService.SearchCollections(keyword, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": colls})
}

// AutoMatch 自动匹配电影系列合集
// POST /api/admin/collections/auto-match
func (h *CollectionHandler) AutoMatch(c *gin.Context) {
	count, err := h.collectionService.AutoMatchCollections()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "自动匹配失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "自动匹配完成",
		"created": count,
	})
}

// CreateCollection 手动创建合集
// POST /api/admin/collections
func (h *CollectionHandler) CreateCollection(c *gin.Context) {
	var req struct {
		Name     string   `json:"name" binding:"required"`
		MediaIDs []string `json:"media_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	coll, err := h.collectionService.CreateCollection(req.Name, req.MediaIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建合集失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": coll})
}

// UpdateCollection 更新合集信息
// PUT /api/admin/collections/:id
func (h *CollectionHandler) UpdateCollection(c *gin.Context) {
	collectionID := c.Param("id")
	var req struct {
		Name     string `json:"name" binding:"required"`
		Overview string `json:"overview"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if err := h.collectionService.UpdateCollection(collectionID, req.Name, req.Overview); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteCollection 删除合集
// DELETE /api/admin/collections/:id
func (h *CollectionHandler) DeleteCollection(c *gin.Context) {
	collectionID := c.Param("id")
	if err := h.collectionService.DeleteCollection(collectionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// AddMedia 向合集添加电影
// POST /api/admin/collections/:id/media
func (h *CollectionHandler) AddMedia(c *gin.Context) {
	collectionID := c.Param("id")
	var req struct {
		MediaID string `json:"media_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if err := h.collectionService.AddMediaToCollection(collectionID, req.MediaID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "添加成功"})
}

// Poster 获取合集封面海报图片
// GET /api/collections/:id/poster
// 策略：优先使用合集自身的 PosterPath，如果为空则使用第一部电影的海报
func (h *CollectionHandler) Poster(c *gin.Context) {
	collectionID := c.Param("id")
	if collectionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少合集ID"})
		return
	}

	// 1. 尝试通过 CollectionService 获取合集自身的海报路径
	posterPath, err := h.collectionService.GetCollectionPosterPath(collectionID)
	if err == nil && posterPath != "" {
		if _, statErr := os.Stat(posterPath); statErr == nil {
			h.servePosterFile(c, posterPath)
			return
		}
	}

	// 2. 回退：通过 StreamService 获取第一部电影的海报
	if h.streamService != nil {
		firstMediaID, err := h.collectionService.GetFirstMediaID(collectionID)
		if err == nil && firstMediaID != "" {
			mediaPosterPath, err := h.streamService.GetPosterPath(firstMediaID)
			if err == nil && mediaPosterPath != "" {
				h.servePosterFile(c, mediaPosterPath)
				return
			}
		}
	}

	// 3. 返回占位图
	c.Header("Content-Type", "image/svg+xml")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("X-Poster-Placeholder", "true")
	c.String(http.StatusOK, collectionPosterPlaceholderSVG)
}

// servePosterFile 提供海报文件服务（带缓存和 ETag 支持）
func (h *CollectionHandler) servePosterFile(c *gin.Context, posterPath string) {
	fileInfo, statErr := os.Stat(posterPath)
	if statErr != nil {
		c.Header("Content-Type", "image/svg+xml")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.String(http.StatusOK, collectionPosterPlaceholderSVG)
		return
	}

	etag := fmt.Sprintf(`"%x-%x"`, fileInfo.ModTime().UnixNano(), fileInfo.Size())
	c.Header("ETag", etag)

	if match := c.GetHeader("If-None-Match"); match == etag {
		c.Status(http.StatusNotModified)
		return
	}

	ext := strings.ToLower(filepath.Ext(posterPath))
	switch ext {
	case ".jpg", ".jpeg":
		c.Header("Content-Type", "image/jpeg")
	case ".png":
		c.Header("Content-Type", "image/png")
	case ".webp":
		c.Header("Content-Type", "image/webp")
	default:
		c.Header("Content-Type", "application/octet-stream")
	}

	c.Header("Cache-Control", "public, max-age=86400, must-revalidate")
	c.File(posterPath)
}

// 合集海报占位图 SVG
const collectionPosterPlaceholderSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="450" viewBox="0 0 300 450">
  <defs>
    <linearGradient id="bg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" style="stop-color:#1a1a2e"/>
      <stop offset="100%" style="stop-color:#16213e"/>
    </linearGradient>
  </defs>
  <rect width="300" height="450" fill="url(#bg)"/>
  <text x="150" y="210" text-anchor="middle" fill="#4a5568" font-size="48" font-family="sans-serif">🎬</text>
  <text x="150" y="260" text-anchor="middle" fill="#4a5568" font-size="14" font-family="sans-serif">系列合集</text>
</svg>`

// MergeDuplicates 合并所有同名重复合集
// POST /api/admin/collections/merge-duplicates
func (h *CollectionHandler) MergeDuplicates(c *gin.Context) {
	merged, err := h.collectionService.MergeDuplicateCollections()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "合并失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("合并完成，共处理 %d 组同名合集", merged),
		"merged":  merged,
	})
}

// CleanupEmpty 清理所有空壳合集（无关联电影的合集）
// POST /api/admin/collections/cleanup-empty
func (h *CollectionHandler) CleanupEmpty(c *gin.Context) {
	cleaned, err := h.collectionService.CleanupEmptyCollections()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清理失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("清理完成，共删除 %d 个空壳合集", cleaned),
		"cleaned": cleaned,
	})
}

// Rematch 重新匹配所有电影系列合集
// POST /api/admin/collections/rematch
// 清除所有自动匹配的合集关联和记录，然后重新执行自动匹配
// 手动创建的合集（auto_matched = false）及其关联会被保留
func (h *CollectionHandler) Rematch(c *gin.Context) {
	created, err := h.collectionService.ReMatchCollections()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重新匹配失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("重新匹配完成，新建 %d 个合集", created),
		"created": created,
	})
}

// DuplicateStats 获取重复合集统计信息
// GET /api/admin/collections/duplicate-stats
func (h *CollectionHandler) DuplicateStats(c *gin.Context) {
	stats, err := h.collectionService.GetDuplicateCollectionStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取统计失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  stats,
		"count": len(stats),
	})
}

// RemoveMedia 从合集移除电影
// DELETE /api/admin/collections/:id/media/:mediaId
func (h *CollectionHandler) RemoveMedia(c *gin.Context) {
	collectionID := c.Param("id")
	mediaID := c.Param("mediaId")

	if err := h.collectionService.RemoveMediaFromCollection(collectionID, mediaID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "移除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "移除成功"})
}
