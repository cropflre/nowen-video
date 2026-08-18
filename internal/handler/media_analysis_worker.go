package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"gorm.io/gorm"
)

type mediaAnalysisModeRequest struct {
	ExecutionMode string `json:"execution_mode" binding:"required"`
}

// AnalyzeHighlightsDistributed 是新的统一入口。任务优先交给可用客户端，
// auto 模式在没有合格客户端时回退现有服务端 Sparse V2。
func (h *MediaAnalysisHandler) AnalyzeHighlightsDistributed(c *gin.Context) {
	mediaID := c.Param("id")
	task, err := h.analysis.AnalyzeHighlightsDistributed(mediaID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMediaAnalysisInProgress):
			c.JSON(http.StatusAccepted, gin.H{"data": task, "message": "精彩片段分析已在进行中"})
			return
		case errors.Is(err, service.ErrMediaAnalysisDisabled):
			c.JSON(http.StatusConflict, gin.H{"error": "精彩片段计算已关闭"})
			return
		case errors.Is(err, service.ErrMediaAnalysisUnsupported):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "当前媒体源暂不支持精彩片段计算"})
			return
		case errors.Is(err, service.ErrMediaNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "媒体不存在"})
			return
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "启动精彩片段分析失败: " + err.Error()})
			return
		}
	}
	c.JSON(http.StatusAccepted, gin.H{
		"data": task,
		"message": "精彩片段分析任务已启动",
		"execution_mode": h.analysis.ExecutionMode(),
	})
}

func (h *MediaAnalysisHandler) WorkerConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"execution_mode": h.analysis.ExecutionMode(),
		"modes": []string{
			service.MediaAnalysisModeAuto,
			service.MediaAnalysisModeClientPreferred,
			service.MediaAnalysisModeServerOnly,
			service.MediaAnalysisModeOff,
		},
	}})
}

func (h *MediaAnalysisHandler) UpdateWorkerConfig(c *gin.Context) {
	var request mediaAnalysisModeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "execution_mode 不能为空"})
		return
	}
	if err := h.analysis.SetExecutionMode(request.ExecutionMode); err != nil {
		if errors.Is(err, service.ErrMediaAnalysisInvalidMode) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的精彩片段计算模式"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存精彩片段计算模式失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"execution_mode": h.analysis.ExecutionMode()}})
}

func (h *MediaAnalysisHandler) Workers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.analysis.Workers()})
}

func (h *MediaAnalysisHandler) WorkerHeartbeat(c *gin.Context) {
	var request service.MediaAnalysisWorkerHeartbeat
	if err := c.ShouldBindJSON(&request); err != nil || request.WorkerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "worker_id 不能为空"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": h.analysis.HeartbeatWorker(request)})
}

func (h *MediaAnalysisHandler) WorkerClaim(c *gin.Context) {
	var request service.MediaAnalysisWorkerClaimRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.WorkerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "计算节点参数无效"})
		return
	}
	claim, err := h.analysis.ClaimWorkerTask(request)
	if errors.Is(err, service.ErrMediaAnalysisWorkerNoTask) {
		c.Status(http.StatusNoContent)
		return
	}
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": claim})
}

func (h *MediaAnalysisHandler) WorkerProgress(c *gin.Context) {
	var request service.MediaAnalysisWorkerProgress
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "进度参数无效"})
		return
	}
	if err := h.analysis.UpdateWorkerProgress(c.Param("taskId"), request); err != nil {
		writeMediaAnalysisWorkerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MediaAnalysisHandler) WorkerComplete(c *gin.Context) {
	var request service.MediaAnalysisWorkerComplete
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "计算结果格式无效"})
		return
	}
	if err := h.analysis.CompleteWorkerTask(c.Param("taskId"), request); err != nil {
		writeMediaAnalysisWorkerError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "精彩片段客户端计算结果已接收"})
}

func (h *MediaAnalysisHandler) WorkerFail(c *gin.Context) {
	var request service.MediaAnalysisWorkerFailure
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "失败回报格式无效"})
		return
	}
	if err := h.analysis.FailWorkerTask(c.Param("taskId"), request); err != nil {
		writeMediaAnalysisWorkerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func writeMediaAnalysisWorkerError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrMediaAnalysisWorkerClaim):
		c.JSON(http.StatusConflict, gin.H{"error": "计算任务租约已失效，请重新领取"})
	case errors.Is(err, service.ErrMediaAnalysisFingerprintChanged):
		c.JSON(http.StatusConflict, gin.H{"error": "媒体文件在计算期间已发生变化，请重新分析"})
	case errors.Is(err, service.ErrMediaAnalysisWorkerResult):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "客户端提交的精彩片段结果无效"})
	case errors.Is(err, service.ErrMediaNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "媒体或任务不存在"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
