package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// MediaHandler 媒体处理器
type MediaHandler struct {
	mediaService    *service.MediaService
	personRepo      *repository.PersonRepo
	mediaPersonRepo *repository.MediaPersonRepo
	logger          *zap.SugaredLogger
}

// List 获取媒体列表
func (h *MediaHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	libraryID := c.Query("library_id")

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	media, total, err := h.mediaService.ListMedia(page, size, libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取媒体列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  media,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// Detail 获取媒体详情
func (h *MediaHandler) Detail(c *gin.Context) {
	id := c.Param("id")
	media, err := h.mediaService.GetDetail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "媒体不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": media})
}

// DetailEnhanced 获取增强的媒体详情（包含技术规格、媒体库信息、播放统计）
func (h *MediaHandler) DetailEnhanced(c *gin.Context) {
	id := c.Param("id")
	enhanced, err := h.mediaService.GetDetailEnhanced(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "媒体不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": enhanced})
}

// Recent 最近添加
func (h *MediaHandler) Recent(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	media, err := h.mediaService.Recent(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取最近媒体失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": media})
}

// RecentAggregated 最近添加（聚合模式：剧集按合集聚合）
func (h *MediaHandler) RecentAggregated(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	media, series, err := h.mediaService.RecentAggregated(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取最近媒体失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"media":  media,
		"series": series,
	})
}

// ListAggregated 获取媒体列表（聚合模式：仅返回独立媒体，不包含已归入合集的剧集）
func (h *MediaHandler) ListAggregated(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	libraryID := c.Query("library_id")

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	media, total, err := h.mediaService.ListMediaAggregated(page, size, libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取媒体列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  media,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// ListMixed 获取混合列表（电影+剧集合集按时间混合展示，Emby风格）
func (h *MediaHandler) ListMixed(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	libraryID := c.Query("library_id")

	if page < 1 {
		page = 1
	}
	// size 上限设为 2000，满足前端全量筛选的常见家庭影视库场景，
	// 同时避免 size=50000 这类请求引发的数据库/内存压力与潜在 DoS 风险。
	if size < 1 || size > 2000 {
		size = 20
	}

	items, total, err := h.mediaService.ListMixed(page, size, libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取混合列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// RecentMixed 最近添加混合列表（电影+合集按时间混合排列）
func (h *MediaHandler) RecentMixed(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.mediaService.RecentMixed(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取最近媒体失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Continue 继续观看
func (h *MediaHandler) Continue(c *gin.Context) {
	userID, _ := c.Get("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	histories, err := h.mediaService.ContinueWatching(userID.(string), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取续播列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": histories})
}

// Search 搜索媒体
func (h *MediaHandler) Search(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "搜索关键词不能为空"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	media, total, err := h.mediaService.Search(keyword, page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  media,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// SearchAdvanced 高级搜索（支持多条件筛选和排序）
// GET /api/search/advanced?q=xxx&type=movie&genre=动作&year_min=2020&year_max=2025&min_rating=7&sort_by=rating&sort_order=desc&page=1&size=20
func (h *MediaHandler) SearchAdvanced(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	yearMin, _ := strconv.Atoi(c.DefaultQuery("year_min", "0"))
	yearMax, _ := strconv.Atoi(c.DefaultQuery("year_max", "0"))
	minRating, _ := strconv.ParseFloat(c.DefaultQuery("min_rating", "0"), 64)

	params := repository.SearchAdvancedParams{
		Keyword:   c.Query("q"),
		MediaType: c.Query("type"),
		Genre:     c.Query("genre"),
		YearMin:   yearMin,
		YearMax:   yearMax,
		MinRating: minRating,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
		Page:      page,
		Size:      size,
	}

	media, total, err := h.mediaService.SearchAdvanced(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  media,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// SearchMixed 混合搜索（同时搜索媒体和合集）
// GET /api/search/mixed?q=xxx&page=1&size=20
func (h *MediaHandler) SearchMixed(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "搜索关键词不能为空"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	result, err := h.mediaService.SearchMixed(keyword, page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"media":        result.Media,
		"series":       result.Series,
		"media_total":  result.MediaTotal,
		"series_total": result.SeriesTotal,
		"page":         page,
		"size":         size,
	})
}

// GetPersons 获取媒体的演职人员列表
func (h *MediaHandler) GetPersons(c *gin.Context) {
	mediaID := c.Param("id")
	persons, err := h.mediaPersonRepo.ListByMediaID(mediaID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": persons})
}

// GetPersonDetail 获取演员详情
// GET /api/persons/:id
func (h *MediaHandler) GetPersonDetail(c *gin.Context) {
	personID := c.Param("id")
	person, err := h.personRepo.FindByID(personID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "person not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": person})
}

// GetPersonMedia 获取某个演员参演的所有影视作品
// GET /api/persons/:id/media
func (h *MediaHandler) GetPersonMedia(c *gin.Context) {
	personID := c.Param("id")

	// 查询该演员参演的电影
	media, err := h.mediaPersonRepo.ListMediaByPersonID(personID)
	if err != nil {
		media = nil
	}

	// 查询该演员参演的剧集
	series, err := h.mediaPersonRepo.ListSeriesByPersonID(personID)
	if err != nil {
		series = nil
	}

	c.JSON(http.StatusOK, gin.H{
		"media":  media,
		"series": series,
	})
}
