package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/service"
	"gorm.io/gorm"
)

func registerAgentScanRoutes(api *gin.RouterGroup, runs *service.AgentScanService) {
	group := api.Group("/admin/agent/runs", middleware.AdminOnly())
	respond := func(c *gin.Context, value interface{}, err error) {
		c.Header("Cache-Control", "private, no-store")
		if err == nil {
			c.JSON(200, gin.H{"data": value})
			return
		}
		status, message := http.StatusServiceUnavailable, "暂时无法处理任务，请稍后重试"
		switch {
		case errors.Is(err, service.ErrForbidden):
			status, message = 403, "只有管理员本人可以管理自己的任务"
		case errors.Is(err, gorm.ErrRecordNotFound):
			status, message = 404, "任务或影视库不存在"
		case errors.Is(err, service.ErrAgentPreviewChanged):
			status, message = 409, err.Error()
		case errors.Is(err, service.ErrAgentRunBusy):
			status, message = 409, err.Error()
		case errors.Is(err, service.ErrAgentRunUnavailable):
			status, message = 503, err.Error()
		case errors.Is(err, service.ErrAgentScanUnsupported):
			status, message = 400, err.Error()
		case errors.Is(err, service.ErrAgentMetadataUnsupported), errors.Is(err, service.ErrAgentMetadataNoChanges):
			status, message = 400, err.Error()
		case errors.Is(err, service.ErrAgentMetadataSource):
			status, message = 503, err.Error()
		}
		c.JSON(status, gin.H{"error": message})
	}
	group.GET("", func(c *gin.Context) {
		value, err := runs.List(c.Request.Context(), c.GetString("user_id"))
		respond(c, value, err)
	})
	group.POST("/metadata", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2048)
		var input struct {
			MediaID string `json:"media_id" binding:"required,max=128"`
		}
		if c.ShouldBindJSON(&input) != nil {
			c.JSON(400, gin.H{"error": "请选择要补全资料的电影"})
			return
		}
		value, err := runs.PreviewMetadata(c.Request.Context(), c.GetString("user_id"), input.MediaID)
		respond(c, value, err)
	})
	group.POST("/:runID/undo", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2048)
		var input struct {
			Digest string `json:"approval_digest" binding:"required,len=64"`
		}
		if c.ShouldBindJSON(&input) != nil {
			c.JSON(400, gin.H{"error": "请重新打开任务记录并撤销"})
			return
		}
		value, err := runs.UndoMetadata(c.Request.Context(), c.GetString("user_id"), c.Param("runID"), input.Digest)
		respond(c, value, err)
	})
	group.POST("", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2048)
		var input struct {
			LibraryID string `json:"library_id" binding:"required,max=128"`
		}
		if c.ShouldBindJSON(&input) != nil {
			c.JSON(400, gin.H{"error": "请选择要预览的影视库"})
			return
		}
		value, err := runs.Preview(c.Request.Context(), c.GetString("user_id"), input.LibraryID)
		respond(c, value, err)
	})
	group.POST("/:runID/confirm", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2048)
		var input struct {
			Digest string `json:"approval_digest" binding:"required,len=64"`
		}
		if c.ShouldBindJSON(&input) != nil {
			c.JSON(400, gin.H{"error": "请重新打开任务预览并确认"})
			return
		}
		value, err := runs.Confirm(c.Request.Context(), c.GetString("user_id"), c.Param("runID"), input.Digest)
		respond(c, value, err)
	})
	group.POST("/:runID/cancel", func(c *gin.Context) {
		value, err := runs.Cancel(c.Request.Context(), c.GetString("user_id"), c.Param("runID"))
		respond(c, value, err)
	})
}
