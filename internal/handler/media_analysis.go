package handler

import (
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MediaAnalysisHandler exposes local/distributed media analysis.
// It intentionally does not accept or inspect AI configuration.
type MediaAnalysisHandler struct {
	analysis *service.MediaAnalysisService
	logger   *zap.SugaredLogger
}

func NewMediaAnalysisHandler(analysis *service.MediaAnalysisService, logger *zap.SugaredLogger) *MediaAnalysisHandler {
	return &MediaAnalysisHandler{analysis: analysis, logger: logger}
}

type mediaHighlightView struct {
	ID             string  `json:"id"`
	MediaID        string  `json:"media_id"`
	Title          string  `json:"title"`
	StartTime      float64 `json:"start_time"`
	EndTime        float64 `json:"end_time"`
	Score          float64 `json:"score"`
	Tags           string  `json:"tags"`
	Source         string  `json:"source"`
	AnalysisMethod string  `json:"analysis_method"`
	ThumbnailURL   string  `json:"thumbnail_url,omitempty"`
	PreviewURL     string  `json:"preview_url,omitempty"`
	Version        int     `json:"version"`
}

func highlightView(mediaID string, h model.VideoHighlight) mediaHighlightView {
	view := mediaHighlightView{
		ID: h.ID, MediaID: h.MediaID, Title: h.Title,
		StartTime: h.StartTime, EndTime: h.EndTime, Score: h.Score,
		Tags: h.Tags, Source: h.Source, AnalysisMethod: h.AnalysisMethod, Version: h.Version,
	}
	if h.Thumbnail != "" {
		view.ThumbnailURL = "/api/media/" + mediaID + "/highlights/" + h.ID + "/thumbnail"
	}
	// preview_thumbnail_v1 已把客户端生成的精彩片段也接入同一 lazy preview URL。
	// 首次请求时由 Media Compute Node V2 尝试 Desktop -> Android，auto 再回退 Server。
	view.PreviewURL = "/api/media/" + mediaID + "/highlights/" + h.ID + "/preview"
	return view
}

func (h *MediaAnalysisHandler) ListHighlights(c *gin.Context) {
	mediaID := c.Param("id")
	result, err := h.analysis.ListHighlights(mediaID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrMediaNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	items := make([]mediaHighlightView, 0, len(result.Highlights))
	for _, item := range result.Highlights {
		items = append(items, highlightView(mediaID, item))
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"highlights": items,
		"stale": result.Stale,
	}})
}

func (h *MediaAnalysisHandler) AnalyzeHighlights(c *gin.Context) {
	mediaID := c.Param("id")
	task, err := h.analysis.AnalyzeHighlights(mediaID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMediaAnalysisInProgress):
			c.JSON(http.StatusAccepted, gin.H{"data": task, "message": "精彩片段分析已在进行中"})
			return
		case errors.Is(err, service.ErrMediaAnalysisUnsupported):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "当前媒体源暂不支持本地精彩片段分析"})
			return
		case errors.Is(err, service.ErrMediaNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "媒体不存在"})
			return
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "启动精彩片段分析失败: " + err.Error()})
			return
		}
	}
	c.JSON(http.StatusAccepted, gin.H{"data": task, "message": "精彩片段分析任务已启动"})
}

func (h *MediaAnalysisHandler) Status(c *gin.Context) {
	mediaID := c.Param("id")
	task, err := h.analysis.LatestTask(mediaID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取分析状态失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": task})
}

func (h *MediaAnalysisHandler) DeleteHighlights(c *gin.Context) {
	mediaID := c.Param("id")
	if err := h.analysis.DeleteHighlights(mediaID); err != nil && !errors.Is(err, os.ErrNotExist) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除精彩片段失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "精彩片段已删除"})
}

func (h *MediaAnalysisHandler) Thumbnail(c *gin.Context) {
	path, err := h.analysis.HighlightAsset(c.Param("id"), c.Param("highlightId"), "thumbnail")
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	h.serveFile(c, path)
}

func (h *MediaAnalysisHandler) Preview(c *gin.Context) {
	path, err := h.analysis.EnsureHighlightPreviewDistributed(c.Param("id"), c.Param("highlightId"))
	if err != nil {
		h.logger.Debugf("distributed lazy highlight preview failed media=%s highlight=%s: %v", c.Param("id"), c.Param("highlightId"), err)
		c.Status(http.StatusNotFound)
		return
	}
	h.serveFile(c, path)
}

func (h *MediaAnalysisHandler) serveFile(c *gin.Context, path string) {
	c.Header("Cache-Control", "private, max-age=86400")
	c.File(path)
}
