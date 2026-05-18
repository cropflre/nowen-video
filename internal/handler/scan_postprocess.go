package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// ==================== 扫描后处理 HTTP 入口 ====================
//
// 该模块对应 /api/admin/scan-classify/*，全部为 DB 层操作。
//
// 安全约束：所有接口的副作用仅限 media_classifications 表，绝不会触发任何磁盘改名/移动。

// ScanPostProcessHandler 扫描后处理 HTTP 处理器
type ScanPostProcessHandler struct {
	svc    *service.ScanPostProcessService
	repo   *repository.ScanClassificationRepo
	logger *zap.SugaredLogger
}

// NewScanPostProcessHandler 构造
func NewScanPostProcessHandler(svc *service.ScanPostProcessService, repo *repository.ScanClassificationRepo, logger *zap.SugaredLogger) *ScanPostProcessHandler {
	return &ScanPostProcessHandler{svc: svc, repo: repo, logger: logger}
}

// ===== 请求体 =====

// reprocessRequest 整库/批量重跑参数
type reprocessRequest struct {
	LibraryID string   `json:"library_id"`
	MediaIDs  []string `json:"media_ids"`
	Async     bool     `json:"async"` // 异步入队（默认 true，整库场景更稳）
}

// correctRequest 单条人工修正参数
type correctRequest struct {
	MediaID  string `json:"media_id" binding:"required"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	TMDbID   int    `json:"tmdb_id"`
	IMDbID   string `json:"imdb_id"`
	Category string `json:"category"`
	Region   string `json:"region"`
}

// ===== 路由实现 =====

// List GET /api/admin/scan-classify
//
// 支持过滤：library_id / status / category / region / decade / keyword / min_score
func (h *ScanPostProcessHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	minScore, _ := strconv.ParseFloat(c.DefaultQuery("min_score", "0"), 64)

	filter := repository.ClassificationListFilter{
		LibraryID: c.Query("library_id"),
		Status:    c.Query("status"),
		Category:  c.Query("category"),
		Region:    c.Query("region"),
		Decade:    c.Query("decade"),
		Keyword:   c.Query("keyword"),
		MinScore:  minScore,
		Page:      page,
		Size:      size,
	}
	items, total, err := h.repo.List(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	}})
}

// Get GET /api/admin/scan-classify/:mediaId
func (h *ScanPostProcessHandler) Get(c *gin.Context) {
	mediaID := c.Param("mediaId")
	item, err := h.repo.FindByMediaID(mediaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

// Reprocess POST /api/admin/scan-classify/reprocess
//
// 优先级：当同时提供 media_ids 与 library_id 时，**仅以 media_ids 为准**（同步批量重跑）。
// 三种用法：
//  1. 仅 library_id：整库重跑（默认异步入队）
//  2. 提供 media_ids：指定条目同步重跑（library_id 即使存在也会被忽略）
//  3. 都不提供：「全部媒体库」一键重跑（异步入队，傻瓜化用法）
func (h *ScanPostProcessHandler) Reprocess(c *gin.Context) {
	var req reprocessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.MediaIDs) > 0 {
		ok, err := h.svc.ProcessBatch(req.MediaIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": gin.H{
			"mode":      "batch",
			"requested": len(req.MediaIDs),
			"processed": ok,
		}})
		return
	}

	// 整库 / 全部媒体库（library_id 为空时进入「全部」模式）
	async := true
	if !req.Async {
		// 显式传 async=false 才同步执行
		async = false
	}
	count, err := h.svc.ReprocessLibrary(req.LibraryID, async)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	mode := "library"
	if req.LibraryID == "" {
		mode = "all"
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"mode":  mode,
		"async": async,
		"count": count,
	}})
}

// Correct POST /api/admin/scan-classify/correct
//
// 单条人工修正：用户在前端修改识别结果后保存。仅写库不动磁盘。
func (h *ScanPostProcessHandler) Correct(c *gin.Context) {
	var req correctRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.svc.ManualCorrect(service.ManualCorrectInput{
		MediaID:  req.MediaID,
		Title:    req.Title,
		Year:     req.Year,
		TMDbID:   req.TMDbID,
		IMDbID:   req.IMDbID,
		Category: req.Category,
		Region:   req.Region,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// Clear DELETE /api/admin/scan-classify
//
// 清空分类记录。可选 query 参数 library_id 指定只清理某个媒体库的记录；
// 不传则清空全部。
func (h *ScanPostProcessHandler) Clear(c *gin.Context) {
	libraryID := c.Query("library_id")
	var count int64
	var err error
	if libraryID != "" {
		count, err = h.repo.DeleteByLibraryID(libraryID)
	} else {
		count, err = h.repo.DeleteAll()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.logger.Infof("扫描归类记录已清空 library_id=%s count=%d", libraryID, count)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": count}})
}

// Stats GET /api/admin/scan-classify/stats
func (h *ScanPostProcessHandler) Stats(c *gin.Context) {
	libraryID := c.Query("library_id")
	stats, err := h.repo.Stats(libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": stats})
}
