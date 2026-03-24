package handler

import (
	"net/http"

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
	// 新增6项功能
	EnableGPUTranscode *bool   `json:"enable_gpu_transcode"`
	GPUFallbackCPU     *bool   `json:"gpu_fallback_cpu"`
	MetadataStorePath  *string `json:"metadata_store_path"`
	PlayCachePath      *string `json:"play_cache_path"`
	EnableFileWatch    *bool   `json:"enable_file_watch"`
	EnableDirectLink   *bool   `json:"enable_direct_link"`
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
	// 新增6项功能的高级设置
	if req.EnableGPUTranscode != nil {
		lib.EnableGPUTranscode = *req.EnableGPUTranscode
	}
	if req.GPUFallbackCPU != nil {
		lib.GPUFallbackCPU = *req.GPUFallbackCPU
	}
	if req.MetadataStorePath != nil {
		lib.MetadataStorePath = *req.MetadataStorePath
	}
	if req.PlayCachePath != nil {
		lib.PlayCachePath = *req.PlayCachePath
	}
	if req.EnableFileWatch != nil {
		lib.EnableFileWatch = *req.EnableFileWatch
	}
	if req.EnableDirectLink != nil {
		lib.EnableDirectLink = *req.EnableDirectLink
	}

	// 如果有高级设置需要更新，则再次保存
	needUpdate := req.PreferLocalNFO != nil || req.EnableFileFilter != nil || req.MinFileSize != nil ||
		req.MetadataLang != nil || req.AllowAdultContent != nil || req.AutoDownloadSub != nil ||
		req.EnableGPUTranscode != nil || req.GPUFallbackCPU != nil || req.MetadataStorePath != nil ||
		req.PlayCachePath != nil || req.EnableFileWatch != nil || req.EnableDirectLink != nil
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
	var req CreateLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	lib, err := h.libService.FindByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "媒体库不存在"})
		return
	}

	// 更新所有提供的字段
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
	if req.EnableGPUTranscode != nil {
		lib.EnableGPUTranscode = *req.EnableGPUTranscode
	}
	if req.GPUFallbackCPU != nil {
		lib.GPUFallbackCPU = *req.GPUFallbackCPU
	}
	if req.MetadataStorePath != nil {
		lib.MetadataStorePath = *req.MetadataStorePath
	}
	if req.PlayCachePath != nil {
		lib.PlayCachePath = *req.PlayCachePath
	}
	if req.EnableFileWatch != nil {
		lib.EnableFileWatch = *req.EnableFileWatch
	}
	if req.EnableDirectLink != nil {
		lib.EnableDirectLink = *req.EnableDirectLink
	}

	if err := h.libService.Update(lib); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新媒体库失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": lib})
}
