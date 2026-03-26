package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// FileManagerHandler 影视文件管理 API 处理器
type FileManagerHandler struct {
	fileService *service.FileManagerService
	logger      *zap.SugaredLogger
}

// ==================== 文件列表与查询 ====================

// ListFiles 获取影视文件列表
func (h *FileManagerHandler) ListFiles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	libraryID := c.Query("library_id")
	mediaType := c.Query("media_type")
	keyword := c.Query("keyword")
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	// 解析刮削状态筛选
	var scrapedOnly *bool
	if v := c.Query("scraped"); v != "" {
		b := v == "true" || v == "1"
		scrapedOnly = &b
	}

	files, total, err := h.fileService.ListFiles(page, size, libraryID, mediaType, keyword, sortBy, sortOrder, scrapedOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文件列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  files,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GetFileDetail 获取文件详情
func (h *FileManagerHandler) GetFileDetail(c *gin.Context) {
	id := c.Param("id")
	media, err := h.fileService.GetFileDetail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": media})
}

// ==================== 文件导入 ====================

// ImportFile 导入单个影视文件
func (h *FileManagerHandler) ImportFile(c *gin.Context) {
	var req service.FileImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID := c.GetString("user_id")
	media, err := h.fileService.ImportFile(req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": media, "message": "文件导入成功"})
}

// BatchImportFiles 批量导入影视文件
func (h *FileManagerHandler) BatchImportFiles(c *gin.Context) {
	var req struct {
		Files []service.FileImportRequest `json:"files" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID := c.GetString("user_id")
	result := h.fileService.BatchImportFiles(req.Files, userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "批量导入完成",
		"data":    result,
	})
}

// ScanDirectory 扫描目录获取可导入的影视文件
func (h *FileManagerHandler) ScanDirectory(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "目录路径不能为空"})
		return
	}

	files, err := h.fileService.ScanDirectoryFiles(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": files, "total": len(files)})
}

// ==================== 文件编辑 ====================

// UpdateFile 更新文件信息
func (h *FileManagerHandler) UpdateFile(c *gin.Context) {
	id := c.Param("id")
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID := c.GetString("user_id")
	media, err := h.fileService.UpdateFileInfo(id, updates, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": media, "message": "更新成功"})
}

// ==================== 安全删除 ====================

// DeleteFile 安全删除文件记录
func (h *FileManagerHandler) DeleteFile(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	if err := h.fileService.DeleteFile(id, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文件记录已删除（原始文件未受影响）"})
}

// BatchDeleteFiles 批量安全删除
func (h *FileManagerHandler) BatchDeleteFiles(c *gin.Context) {
	var req struct {
		MediaIDs []string `json:"media_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID := c.GetString("user_id")
	deleted, errors := h.fileService.BatchDeleteFiles(req.MediaIDs, userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "批量删除完成",
		"deleted": deleted,
		"errors":  errors,
	})
}

// ==================== AI智能刮削 ====================

// ScrapeFile 对单个文件执行刮削
func (h *FileManagerHandler) ScrapeFile(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Source string `json:"source"` // tmdb / bangumi / ai
	}
	c.ShouldBindJSON(&req)

	userID := c.GetString("user_id")
	if err := h.fileService.ScrapeFileMetadata(id, req.Source, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "刮削已启动"})
}

// BatchScrapeFiles 批量刮削
func (h *FileManagerHandler) BatchScrapeFiles(c *gin.Context) {
	var req struct {
		MediaIDs []string `json:"media_ids" binding:"required"`
		Source   string   `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	userID := c.GetString("user_id")
	started, errors := h.fileService.BatchScrapeFiles(req.MediaIDs, req.Source, userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "批量刮削已启动",
		"started": started,
		"errors":  errors,
	})
}

// ==================== AI批量重命名 ====================

// PreviewRename 预览重命名结果
func (h *FileManagerHandler) PreviewRename(c *gin.Context) {
	var req struct {
		MediaIDs []string `json:"media_ids" binding:"required"`
		Template string   `json:"template"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if req.Template == "" {
		req.Template = "{title} ({year}) [{resolution}]"
	}

	previews, err := h.fileService.PreviewRename(req.MediaIDs, req.Template)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成预览失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": previews})
}

// ExecuteRename 执行重命名
func (h *FileManagerHandler) ExecuteRename(c *gin.Context) {
	var req struct {
		MediaIDs []string `json:"media_ids" binding:"required"`
		Template string   `json:"template"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if req.Template == "" {
		req.Template = "{title} ({year}) [{resolution}]"
	}

	userID := c.GetString("user_id")
	renamed, errors := h.fileService.ExecuteRename(req.MediaIDs, req.Template, userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "重命名完成",
		"renamed": renamed,
		"errors":  errors,
	})
}

// AIGenerateRenames AI智能生成重命名建议
func (h *FileManagerHandler) AIGenerateRenames(c *gin.Context) {
	var req struct {
		MediaIDs   []string `json:"media_ids" binding:"required"`
		TargetLang string   `json:"target_lang"` // 目标翻译语言: zh/en/ja/ko/fr/de 等
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	previews, err := h.fileService.AIGenerateRenames(req.MediaIDs, req.TargetLang)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": previews})
}

// GetRenameTemplates 获取重命名模板列表
func (h *FileManagerHandler) GetRenameTemplates(c *gin.Context) {
	templates := h.fileService.GetRenameTemplates()
	c.JSON(http.StatusOK, gin.H{"data": templates})
}

// ==================== 统计与日志 ====================

// GetStats 获取文件管理统计
func (h *FileManagerHandler) GetStats(c *gin.Context) {
	stats, err := h.fileService.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取统计失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// GetOperationLogs 获取操作日志
func (h *FileManagerHandler) GetOperationLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 200 {
		limit = 50
	}

	logs := h.fileService.GetOperationLogs(limit)
	c.JSON(http.StatusOK, gin.H{"data": logs})
}
