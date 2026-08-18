package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"gorm.io/gorm"
)

// ListChapters 保持既有 GET /media/:id/chapters 契约不变。
func (h *MediaAnalysisHandler) ListChapters(c *gin.Context) {
	chapters, err := h.analysis.ListChapters(c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "媒体不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取章节失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": chapters})
}

// GenerateChaptersDistributed 保持既有 POST /media/:id/ai/chapters URL，
// 内部已切换为 Media Compute Node V2 的 chapter_detect_v1。
func (h *MediaAnalysisHandler) GenerateChaptersDistributed(c *gin.Context) {
	task, err := h.analysis.AnalyzeChaptersDistributed(c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMediaAnalysisInProgress):
			c.JSON(http.StatusAccepted, gin.H{"data": task, "message": "章节检测已在进行中", "execution_mode": h.analysis.ExecutionMode()})
		case errors.Is(err, service.ErrMediaAnalysisDisabled):
			c.JSON(http.StatusConflict, gin.H{"error": "媒体计算已关闭"})
		case errors.Is(err, service.ErrMediaAnalysisUnsupported):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "当前媒体源暂不支持章节检测"})
		case errors.Is(err, service.ErrMediaNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "媒体不存在"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "启动章节检测失败: " + err.Error()})
		}
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"data": task, "message": "章节检测任务已启动", "execution_mode": h.analysis.ExecutionMode(),
	})
}

// AnalysisTasks / AnalysisTask 补齐 Web V3 已经使用的任务查询 URL，
// 使正式后端接管章节后不再依赖旧 AIScene 路由。
func (h *MediaAnalysisHandler) AnalysisTasks(c *gin.Context) {
	tasks, err := h.analysis.ListAnalysisTasks(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取分析任务失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tasks})
}

func (h *MediaAnalysisHandler) AnalysisTask(c *gin.Context) {
	task, err := h.analysis.AnalysisTask(c.Param("taskId"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取任务失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": task})
}
