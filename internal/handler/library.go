package handler

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// normalizeLibraryPaths 将 Paths 和 Path 合并为单一的完整路径列表，去除空白与重复项
// paths 优先；若 paths 为空则退化为 [path]。结果可能为空切片。
func normalizeLibraryPaths(paths []string, path string) []string {
	out := make([]string, 0, len(paths)+1)
	seen := make(map[string]bool)
	append1 := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	if len(paths) > 0 {
		for _, p := range paths {
			append1(p)
		}
	} else {
		append1(path)
	}
	return out
}

// LibraryHandler 媒体库处理器
type LibraryHandler struct {
	libService *service.LibraryService
	permSvc    *service.PermissionService
	logger     *zap.SugaredLogger
}

// CreateLibraryRequest 创建媒体库请求
type CreateLibraryRequest struct {
	Name string `json:"name" binding:"required"`
	// Path 主路径（保留以兼容旧客户端）。如传入 Paths 则以 Paths[0] 为主路径
	Path string `json:"path"`
	// Paths 完整的媒体文件夹列表（支持多个）。与 Path 二选一，优先使用 Paths
	Paths []string `json:"paths"`
	Type  string   `json:"type" binding:"required,oneof=movie tvshow mixed other"`
	// 高级设置
	PreferLocalNFO    *bool   `json:"prefer_local_nfo"`
	EnableFileFilter  *bool   `json:"enable_file_filter"`
	MinFileSize       *int    `json:"min_file_size"`
	MetadataLang      *string `json:"metadata_lang"`
	AllowAdultContent *bool   `json:"allow_adult_content"`
	AutoDownloadSub   *bool   `json:"auto_download_sub"`
	// 扫描行为设置
	AutoScrapeMetadata *bool `json:"auto_scrape_metadata"`
	// 媒体库级别设置
	EnableFileWatch *bool `json:"enable_file_watch"`
}

// UpdateLibraryRequest 更新媒体库请求（所有字段可选）
type UpdateLibraryRequest struct {
	Name *string `json:"name"`
	// Path 主路径（保留）。传入 Paths 时优先使用 Paths
	Path  *string  `json:"path"`
	Paths []string `json:"paths"`
	Type  *string  `json:"type"`
	// 高级设置
	PreferLocalNFO    *bool   `json:"prefer_local_nfo"`
	EnableFileFilter  *bool   `json:"enable_file_filter"`
	MinFileSize       *int    `json:"min_file_size"`
	MetadataLang      *string `json:"metadata_lang"`
	AllowAdultContent *bool   `json:"allow_adult_content"`
	AutoDownloadSub   *bool   `json:"auto_download_sub"`
	// 扫描行为设置
	AutoScrapeMetadata *bool `json:"auto_scrape_metadata"`
	// 媒体库级别设置
	EnableFileWatch *bool `json:"enable_file_watch"`
}

// List 获取所有媒体库（按用户权限过滤）
func (h *LibraryHandler) List(c *gin.Context) {
	libs, err := h.libService.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取媒体库列表失败"})
		return
	}
	// 管理员不限制；普通用户按 UserPermission 过滤
	if role, _ := c.Get("role"); role != "admin" && h.permSvc != nil {
		if uid, ok := c.Get("user_id"); ok {
			libs = h.permSvc.FilterLibraries(uid.(string), libs)
		}
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

	// 归一化路径列表：优先使用 Paths，其次 Path
	paths := normalizeLibraryPaths(req.Paths, req.Path)
	if len(paths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请至少指定一个媒体文件夹路径"})
		return
	}

	// 逐个校验每个路径是否存在且为目录
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("路径不存在: %s，请检查路径是否正确以及Docker卷映射是否配置", p)})
				return
			}
			if os.IsPermission(err) {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无权限访问路径: %s，请检查文件权限", p)})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("访问路径失败 %s: %v", p, err)})
			return
		}
		if !info.IsDir() {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("路径不是目录: %s", p)})
			return
		}
	}

	lib, err := h.libService.CreateWithPaths(req.Name, paths, req.Type)
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
	if req.AutoScrapeMetadata != nil {
		lib.AutoScrapeMetadata = *req.AutoScrapeMetadata
	}
	if req.EnableFileWatch != nil {
		lib.EnableFileWatch = *req.EnableFileWatch
	}

	// 如果有高级设置需要更新，则再次保存
	needUpdate := req.PreferLocalNFO != nil || req.EnableFileFilter != nil || req.MinFileSize != nil ||
		req.MetadataLang != nil || req.AllowAdultContent != nil || req.AutoDownloadSub != nil ||
		req.AutoScrapeMetadata != nil || req.EnableFileWatch != nil
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
	// 路径更新：优先使用 Paths（多路径）；如仅传 Path 则视为单路径媒体库
	var newPaths []string
	if len(req.Paths) > 0 {
		newPaths = normalizeLibraryPaths(req.Paths, "")
	} else if req.Path != nil && *req.Path != "" {
		// 单路径时保留已有的 ExtraPaths（避免旧客户端误清除）
		newPaths = append([]string{strings.TrimSpace(*req.Path)}, lib.AllPaths()[1:]...)
		newPaths = normalizeLibraryPaths(newPaths, "")
	}
	if len(newPaths) > 0 {
		// 对比是否有路径真正变更，变更就全量验证存在性
		oldPaths := lib.AllPaths()
		changed := len(oldPaths) != len(newPaths)
		if !changed {
			for i := range newPaths {
				if oldPaths[i] != newPaths[i] {
					changed = true
					break
				}
			}
		}
		if changed {
			for _, p := range newPaths {
				pathInfo, pathErr := os.Stat(p)
				if pathErr != nil {
					if os.IsNotExist(pathErr) {
						c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("路径不存在: %s", p)})
						return
					}
					if os.IsPermission(pathErr) {
						c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无权限访问路径: %s", p)})
						return
					}
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("访问路径失败 %s: %v", p, pathErr)})
					return
				}
				if !pathInfo.IsDir() {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("路径不是目录: %s", p)})
					return
				}
			}
		}
		lib.SetAllPaths(newPaths)
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
	if req.AutoScrapeMetadata != nil {
		lib.AutoScrapeMetadata = *req.AutoScrapeMetadata
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
