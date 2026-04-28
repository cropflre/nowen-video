package handler

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// SystemLogHandler 系统日志处理器
type SystemLogHandler struct {
	logRepo *repository.SystemLogRepo
	logger  *zap.SugaredLogger
}

// ListSystemLogs 查询系统日志（分页 + 过滤）
// GET /api/admin/system-logs?page=1&size=50&type=api&level=error&keyword=xxx&method=GET&start=2024-01-01&end=2024-12-31
func (h *SystemLogHandler) ListSystemLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))

	filter := &repository.SystemLogFilter{
		Type:    c.Query("type"),
		Level:   c.Query("level"),
		Keyword: c.Query("keyword"),
		Method:  c.Query("method"),
		Path:    c.Query("path"),
		UserID:  c.Query("user_id"),
		MediaID: c.Query("media_id"),
	}

	if s := c.Query("start"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			filter.StartTime = &t
		}
	}
	if s := c.Query("end"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			end := t.Add(24*time.Hour - time.Second)
			filter.EndTime = &end
		}
	}
	if s := c.Query("min_status"); s != "" {
		filter.MinStatus, _ = strconv.Atoi(s)
	}
	if s := c.Query("max_status"); s != "" {
		filter.MaxStatus, _ = strconv.Atoi(s)
	}

	logs, total, err := h.logRepo.List(page, size, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询日志失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  logs,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GetSystemLogStats 获取日志统计信息
// GET /api/admin/system-logs/stats
func (h *SystemLogHandler) GetSystemLogStats(c *gin.Context) {
	stats, err := h.logRepo.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取统计失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// ExportSystemLogs 导出日志为 CSV
// GET /api/admin/system-logs/export?type=api&level=error&start=2024-01-01&end=2024-12-31&max_rows=5000
func (h *SystemLogHandler) ExportSystemLogs(c *gin.Context) {
	maxRows, _ := strconv.Atoi(c.DefaultQuery("max_rows", "5000"))

	filter := &repository.SystemLogFilter{
		Type:    c.Query("type"),
		Level:   c.Query("level"),
		Keyword: c.Query("keyword"),
		Method:  c.Query("method"),
	}
	if s := c.Query("start"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			filter.StartTime = &t
		}
	}
	if s := c.Query("end"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			end := t.Add(24*time.Hour - time.Second)
			filter.EndTime = &end
		}
	}

	logs, err := h.logRepo.ListForExport(maxRows, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出失败"})
		return
	}

	// 设置 CSV 下载头
	filename := fmt.Sprintf("system-logs-%s.csv", time.Now().Format("20060102-150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// 写入 BOM（Excel 兼容 UTF-8）
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(c.Writer)
	// 表头
	w.Write([]string{"时间", "类型", "级别", "消息", "方法", "路径", "状态码", "耗时(ms)", "客户端IP", "用户", "详情"})

	for _, log := range logs {
		w.Write([]string{
			log.CreatedAt.Format("2006-01-02 15:04:05"),
			log.Type,
			log.Level,
			log.Message,
			log.Method,
			log.Path,
			strconv.Itoa(log.StatusCode),
			strconv.FormatInt(log.LatencyMs, 10),
			log.ClientIP,
			log.Username,
			log.Detail,
		})
	}
	w.Flush()
}

// CleanSystemLogs 清理旧日志
// POST /api/admin/system-logs/clean  { "days": 30 }
func (h *SystemLogHandler) CleanSystemLogs(c *gin.Context) {
	var req struct {
		Days int `json:"days" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定保留天数（最少1天）"})
		return
	}

	deleted, err := h.logRepo.CleanOlderThan(req.Days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清理失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("已清理 %d 条日志（保留最近 %d 天）", deleted, req.Days),
		"deleted": deleted,
	})
}

// ReportPlaybackError 前端上报播放错误
// POST /api/logs/playback-error  { "media_id": "xxx", "media_title": "xxx", "message": "xxx", "detail": "xxx" }
func (h *SystemLogHandler) ReportPlaybackError(c *gin.Context) {
	var req struct {
		MediaID    string `json:"media_id"`
		MediaTitle string `json:"media_title"`
		Message    string `json:"message" binding:"required"`
		Detail     string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}

	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	userIDStr, _ := userID.(string)
	usernameStr, _ := username.(string)

	log := &model.SystemLog{
		Type:       model.LogTypePlayback,
		Level:      model.LogLevelError,
		Message:    req.Message,
		Detail:     req.Detail,
		MediaID:    req.MediaID,
		MediaTitle: req.MediaTitle,
		UserID:     userIDStr,
		Username:   usernameStr,
		ClientIP:   c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		CreatedAt:  time.Now(),
	}

	go func() {
		_ = h.logRepo.Create(log)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "已记录"})
}
