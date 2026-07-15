package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

// Versions 获取同片多版本列表（供前端版本切换 UI 使用）
// GET /api/media/:id/versions
func (h *MediaHandler) Versions(c *gin.Context) {
	id := c.Param("id")
	versions, err := h.mediaService.GetVersions(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "媒体不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  versions,
		"total": len(versions),
	})
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

// ListMixed 获取混合列表（电影+剧集合集按稳定条件筛选、排序和分页）
func (h *MediaHandler) ListMixed(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	yearFrom, _ := strconv.Atoi(c.Query("year_from"))
	yearTo, _ := strconv.Atoi(c.Query("year_to"))

	if page < 1 {
		page = 1
	}
	// size 上限保留 2000 以兼容旧 Web 的全量模式；移动端 Paging 使用更小分页。
	if size < 1 || size > 2000 {
		size = 20
	}

	result, err := h.mediaService.ListMixedFiltered(page, size, service.MixedListFilter{
		LibraryID:   c.Query("library_id"),
		ContentType: c.Query("type"),
		Genre:       c.Query("genre"),
		Query:       c.Query("q"),
		YearFrom:    yearFrom,
		YearTo:      yearTo,
		Sort:        c.Query("sort"),
		Order:       c.Query("order"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取混合列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":         result.Items,
		"total":        result.Total,
		"movie_count":  result.MovieCount,
		"series_count": result.SeriesCount,
		"page":         page,
		"size":         size,
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

var (
	_ = fmt.Sprintf
	_ = os.Stat
	_ = filepath.Ext
	_ = strings.ToLower
)
