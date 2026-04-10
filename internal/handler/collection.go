package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// CollectionHandler 电影系列合集处理器
type CollectionHandler struct {
	collectionService *service.CollectionService
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
