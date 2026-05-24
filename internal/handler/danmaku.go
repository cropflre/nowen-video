package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// DanmakuHandler exposes local bullet comments.
type DanmakuHandler struct {
	danmakuService *service.DanmakuService
	logger         *zap.SugaredLogger
}

func (h *DanmakuHandler) ListByMedia(c *gin.Context) {
	mediaID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	items, err := h.danmakuService.ListByMedia(mediaID, limit)
	if err != nil {
		h.logger.Debugf("获取弹幕失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取弹幕失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  items,
		"total": len(items),
	})
}
