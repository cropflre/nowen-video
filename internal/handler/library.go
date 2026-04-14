package handler

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// LibraryHandler 媒体库处理器
type LibraryHandler struct {
	libService *service.LibraryService
	logger     *zap.SugaredLogger
}

// CreateLibraryRequest 创建媒体库请求
type CreateLibraryRequest struct {
	Name string `json:"name" binding:"required"`
	Path string `json:"path" binding:"required"`
	Type string `json:"type" binding:"required,oneof=movie tvshow mixed other"`
	// 高级设置
	PreferLocalNFO    *bool   `json:"prefer_local_nfo"`
	EnableFileFilter  *bool   `json:"enable_file_filter"`
	MinFileSize       *int    `json:"min_file_size"`
	MetadataLang      *string `json:"metadata_lang"`
	AllowAdultContent *bool   `json:"allow_adult_content"`
	AutoDownloadSub   *bool   `json:"auto_download_sub"`
	// 媒体库级别设置
	EnableFileWatch *bool `json:"enable_file_watch"`
}

// UpdateLibraryRequest 更新媒体库请求（所有字段可选）
type UpdateLibraryRequest struct {
	Name *string `json:"name"`
	Path *string `json:"path"`
	Type *string `json:"type"`
	// 高级设置
	PreferLocalNFO    *bool   `json:"prefer_local_nfo"`
	EnableFileFilter  *bool   `json:"enable_file_filter"`
	MinFileSize       *int    `json:"min_file_size"`
	MetadataLang      *string `json:"metadata_lang"`
	AllowAdultContent *bool   `json:"allow_adult_content"`
	AutoDownloadSub   *bool   `json:"auto_download_sub"`
	// 媒体库级别设置
	EnableFileWatch *bool `json:"enable_file_watch"`
}

// List 获取所有媒体库
func (h *LibraryHandler) List(c *gin.Context) {
	libs, err := h.libService.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取媒体库列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": libs})
}

// Create 创建媒体库
func (h *LibraryHandler) Create(c *gin.Context) {
	var req CreateLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	// 验证媒体库路径是否存在且可访问
	info, err := os.Stat(req.Path)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("路径不存在: %s，请检查路径是否正确以及Docker卷映射是否配置", req.Path)})
			return
		}
		if os.IsPermission(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无权限访问路径: %s，请检查文件权限", req.Path)})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("访问路径失败: %v", err)})
		return
	}
	if !info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("路径不是目录: %s", req.Path)})
		return
	}

	lib, err := h.libService.Create(req.Name, req.Path, req.Type)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建媒体库失败"})
		return
	}

	// 应用高级设置
	if req.PreferLocalNFO != nil {
		lib.PreferLocalNFO = *req.PreferLocalNFO
	}
	if req.EnableFileFilter != nil {
		lib.EnableFileFilter = *req.EnableFileFilter
	}
	if req.MinFileSize != nil {
		lib.MinFileSize = *req.MinFileSize
	}
	if req.MetadataLang != nil {
		lib.MetadataLang = *req.MetadataLang
	}
	if req.AllowAdultContent != nil {
		lib.AllowAdultContent = *req.AllowAdultContent
	}
	if req.AutoDownloadSub != nil {
		lib.AutoDownloadSub = *req.AutoDownloadSub
	}
	if req.EnableFileWatch != nil {
		lib.EnableFileWatch = *req.EnableFileWatch
	}

	// 如果有高级设置需要更新，则再次保存
	needUpdate := req.PreferLocalNFO != nil || req.EnableFileFilter != nil || req.MinFileSize != nil ||
		req.MetadataLang != nil || req.AllowAdultContent != nil || req.AutoDownloadSub != nil ||
		req.EnableFileWatch != nil
	if needUpdate {
		h.libService.Update(lib)
	}

	c.JSON(http.StatusCreated, gin.H{"data": lib})
}

// Scan 触发扫描
func (h *LibraryHandler) Scan(c *gin.Context) {
	id := c.Param("id")
	if err := h.libService.Scan(id); err != nil {
		if err == service.ErrScanInProgress {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "扫描已启动"})
}

// Delete 删除媒体库
func (h *LibraryHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.libService.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除媒体库失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// Reindex 重建媒体库索引
func (h *LibraryHandler) Reindex(c *gin.Context) {
	id := c.Param("id")
	if err := h.libService.Reindex(id); err != nil {
		if err == service.ErrScanInProgress {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "重建索引已启动"})
}

// Update 更新媒体库设置
func (h *LibraryHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req UpdateLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	lib, err := h.libService.FindByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "媒体库不存在"})
		return
	}

	// 更新基础信息
	if req.Name != nil && *req.Name != "" {
		lib.Name = *req.Name
	}
	if req.Path != nil && *req.Path != "" {
		// 路径变更时验证新路径是否存在且可访问
		if *req.Path != lib.Path {
			pathInfo, pathErr := os.Stat(*req.Path)
			if pathErr != nil {
				if os.IsNotExist(pathErr) {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("路径不存在: %s", *req.Path)})
					return
				}
				if os.IsPermission(pathErr) {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无权限访问路径: %s", *req.Path)})
					return
				}
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("访问路径失败: %v", pathErr)})
				return
			}
			if !pathInfo.IsDir() {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("路径不是目录: %s", *req.Path)})
				return
			}
		}
		lib.Path = *req.Path
	}
	if req.Type != nil && *req.Type != "" {
		lib.Type = *req.Type
	}

	// 更新高级设置
	if req.PreferLocalNFO != nil {
		lib.PreferLocalNFO = *req.PreferLocalNFO
	}
	if req.EnableFileFilter != nil {
		lib.EnableFileFilter = *req.EnableFileFilter
	}
	if req.MinFileSize != nil {
		lib.MinFileSize = *req.MinFileSize
	}
	if req.MetadataLang != nil {
		lib.MetadataLang = *req.MetadataLang
	}
	if req.AllowAdultContent != nil {
		lib.AllowAdultContent = *req.AllowAdultContent
	}
	if req.AutoDownloadSub != nil {
		lib.AutoDownloadSub = *req.AutoDownloadSub
	}
	if req.EnableFileWatch != nil {
		lib.EnableFileWatch = *req.EnableFileWatch
	}

	if err := h.libService.Update(lib); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新媒体库失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": lib})
}

// DetectDuplicates 检测重复媒体
// GET /api/admin/libraries/:id/duplicates
func (h *LibraryHandler) DetectDuplicates(c *gin.Context) {
	libraryID := c.Param("id")
	groups, err := h.libService.DetectDuplicates(libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检测重复媒体失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  groups,
		"total": len(groups),
	})
}

// DetectAllDuplicates 检测所有媒体库的重复媒体
// GET /api/admin/duplicates
func (h *LibraryHandler) DetectAllDuplicates(c *gin.Context) {
	groups, err := h.libService.DetectDuplicates("")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检测重复媒体失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  groups,
		"total": len(groups),
	})
}

// MarkDuplicates 标记重复媒体
// POST /api/admin/libraries/:id/mark-duplicates
func (h *LibraryHandler) MarkDuplicates(c *gin.Context) {
	libraryID := c.Param("id")
	count, err := h.libService.MarkDuplicates(libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "标记重复媒体失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("已标记 %d 个重复文件", count),
		"marked":  count,
	})
}
