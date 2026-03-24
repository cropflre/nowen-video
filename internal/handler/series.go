package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// SeriesHandler 剧集合集处理器
type SeriesHandler struct {
	seriesService *service.SeriesService
	logger        *zap.SugaredLogger
}

// List 获取剧集合集列表
func (h *SeriesHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	libraryID := c.Query("library_id")

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	series, total, err := h.seriesService.ListSeries(page, size, libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取剧集列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  series,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// Detail 获取剧集合集详情（含所有剧集）
func (h *SeriesHandler) Detail(c *gin.Context) {
	id := c.Param("id")
	series, err := h.seriesService.GetSeriesDetail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "剧集不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": series})
}

// Seasons 获取季列表（季视图）
func (h *SeriesHandler) Seasons(c *gin.Context) {
	id := c.Param("id")
	seasons, err := h.seriesService.GetSeasons(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取季列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": seasons})
}

// SeasonEpisodes 获取指定季的剧集列表（集视图）
func (h *SeriesHandler) SeasonEpisodes(c *gin.Context) {
	id := c.Param("id")
	seasonNum, _ := strconv.Atoi(c.Param("season"))

	episodes, err := h.seriesService.GetSeasonEpisodes(id, seasonNum)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取剧集列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": episodes})
}

// NextEpisode 获取下一集
func (h *SeriesHandler) NextEpisode(c *gin.Context) {
	id := c.Param("id")
	season, _ := strconv.Atoi(c.Query("season"))
	episode, _ := strconv.Atoi(c.Query("episode"))

	next, err := h.seriesService.GetNextEpisode(id, season, episode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取下一集失败"})
		return
	}
	if next == nil {
		c.JSON(http.StatusOK, gin.H{"data": nil, "message": "已是最后一集"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": next})
}
