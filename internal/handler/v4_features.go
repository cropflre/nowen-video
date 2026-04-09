package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

// ==================== P1: 批量移动媒体 Handler（挂在 LibraryHandler 上） ====================

// BatchMoveMedia 批量移动媒体到目标媒体库
func (h *LibraryHandler) BatchMoveMedia(c *gin.Context) {
	var req service.BatchMoveMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if len(req.MediaIDs) == 0 || req.TargetLibrary == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "media_ids 和 target_library_id 不能为空"})
		return
	}
	result, err := h.libService.BatchMoveMedia(req.MediaIDs, req.TargetLibrary)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
