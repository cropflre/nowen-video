package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// MetadataHandler 元数据处理器
type MetadataHandler struct {
	metadataService *service.MetadataService
	logger          *zap.SugaredLogger
}

// ScrapeMedia 手动刮削单个媒体的元数据
// Body 可选：{"mode": "fill-missing" | "replace"}，缺省为只补缺失。
func (h *MetadataHandler) ScrapeMedia(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Mode string `json:"mode"`
	}
	_ = c.ShouldBindJSON(&req) // 空 body 时保持零值，走默认档位

	if err := h.metadataService.ScrapeMediaWithMode(id, service.ParseScrapeMode(req.Mode)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "元数据刮削失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "元数据刮削成功"})
}
