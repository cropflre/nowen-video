package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// LazyIngestHandler 「一键入库」HTTP 入口
//
// 设计：用户只给 source_path，系统自动完成扫描 → 分类 → 命名 → 落盘 → 建库 → 扫描。
// 所有耗时操作都异步化，前端通过 GET /jobs/:id 轮询或 WS（事件 ingest_progress）订阅进度。
type LazyIngestHandler struct {
	svc    *service.LazyIngestService
	logger *zap.SugaredLogger
}

// NewLazyIngestHandler 构造
func NewLazyIngestHandler(svc *service.LazyIngestService, logger *zap.SugaredLogger) *LazyIngestHandler {
	return &LazyIngestHandler{svc: svc, logger: logger}
}

// ===== 请求体 =====

type lazyIngestSubmitRequest struct {
	SourcePath  string `json:"source_path" binding:"required"`
	TargetRoot  string `json:"target_root"`
	NamingStyle string `json:"naming_style"`
	// hardlink（默认）/ move（专家模式：原地移动 + journal 回滚）
	Mode string `json:"mode"`
	// 强制重新入库：即使同源已被处理过也继续
	Force bool `json:"force"`
}

// ===== 路由实现 =====

// Submit POST /api/admin/ingest/submit
//
// 创建并立刻启动一个懒人入库任务。返回 job 对象（含 id），前端拿 id 轮询/订阅进度。
func (h *LazyIngestHandler) Submit(c *gin.Context) {
	var req lazyIngestSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid, _ := c.Get("userID")
	createdBy, _ := uid.(string)

	job, err := h.svc.Submit(service.LazyIngestInput{
		SourcePath:  req.SourcePath,
		TargetRoot:  req.TargetRoot,
		NamingStyle: req.NamingStyle,
		Mode:        req.Mode,
		Force:       req.Force,
		CreatedBy:   createdBy,
	})
	if err != nil {
		// 重复入库检测命中 -> 409，前端据此弹「已入库过，是否强制重新入库」
		var dup *service.ErrAlreadyIngested
		if errors.As(err, &dup) {
			c.JSON(http.StatusConflict, gin.H{
				"error":         dup.Error(),
				"code":          "already_ingested",
				"existing_jobs": dup.ExistingJobs,
			})
			return
		}
		h.logger.Warnf("[LazyIngest] 创建任务失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": job})
}

// GetJob GET /api/admin/ingest/jobs/:id
//
// 用于前端轮询任务进度（status / phase / progress / stats）。
func (h *LazyIngestHandler) GetJob(c *gin.Context) {
	id := c.Param("id")
	job, err := h.svc.GetJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": job})
}

// ListJobs GET /api/admin/ingest/jobs?limit=50
//
// 返回最近 N 条任务（默认 50，最多 200）。
func (h *LazyIngestHandler) ListJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	jobs, err := h.svc.ListJobs(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": jobs})
}

// CancelJob POST /api/admin/ingest/jobs/:id/cancel
//
// 仅 pending/scanning 状态可取消。
func (h *LazyIngestHandler) CancelJob(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.CancelJob(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// GetJobItems GET /api/admin/ingest/jobs/:id/items
//
// 返回该 job 的文件级明细（来自 RenamePlanItem），用于"失败明细页"。
// 前端可按 status 分组渲染：executed / skipped / unsafe / failed / pending。
func (h *LazyIngestHandler) GetJobItems(c *gin.Context) {
	id := c.Param("id")
	items, err := h.svc.GetJobItems(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": len(items)})
}

// RollbackJob POST /api/admin/ingest/jobs/:id/rollback
//
// 仅适用于 mode=move 的 Job。按 journal 倒序还原文件位置。
func (h *LazyIngestHandler) RollbackJob(c *gin.Context) {
	id := c.Param("id")
	res, err := h.svc.RollbackJob(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}
