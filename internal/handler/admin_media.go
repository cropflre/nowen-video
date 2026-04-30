package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ==================== 媒体管理 ====================

// DeleteMedia 删除单个媒体记录（仅从数据库移除，不删除文件）
func (h *AdminHandler) DeleteMedia(c *gin.Context) {
	mediaID := c.Param("mediaId")

	if err := h.libraryService.DeleteMedia(mediaID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除影片失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "影片已删除"})
}

// UpdateMediaMetadataRequest 编辑元数据请求
type UpdateMediaMetadataRequest struct {
	Title     *string  `json:"title"`
	OrigTitle *string  `json:"orig_title"`
	Year      *int     `json:"year"`
	Overview  *string  `json:"overview"`
	Rating    *float64 `json:"rating"`
	Genres    *string  `json:"genres"`
	Country   *string  `json:"country"`
	Language  *string  `json:"language"`
	Tagline   *string  `json:"tagline"`
	Studio    *string  `json:"studio"`
}

// ==================== STRM 单条覆写 ====================

// UpdateMediaSTRMRequest 单条 media 的 STRM 请求头覆写请求
// 所有字段均为指针：为 nil 表示不改；为 "" 表示清空
type UpdateMediaSTRMRequest struct {
	StreamURL    *string            `json:"stream_url"`    // 可选：替换远程 URL（token 过期后手工刷新等）
	UserAgent    *string            `json:"user_agent"`    // 单条覆盖 UA
	Referer      *string            `json:"referer"`       // 单条覆盖 Referer
	Cookie       *string            `json:"cookie"`        // 单条覆盖 Cookie
	Headers      *map[string]string `json:"headers"`       // 额外 Header（整体替换）
	RefreshURL   *string            `json:"refresh_url"`   // 刷新直链的上游 API（预留）
	ClearHeaders bool               `json:"clear_headers"` // 兼容字段：显式清空 headers
}

// UpdateMediaSTRM 更新单条 media 的 STRM 请求头覆写
// PUT /api/admin/media/:mediaId/strm
//
// 典型场景：某个视频拉不动时，直接在管理页面粘贴浏览器 F12 复制的 UA/Referer/Cookie，
// 立即生效（不需要重新扫描媒体库）。
func (h *AdminHandler) UpdateMediaSTRM(c *gin.Context) {
	mediaID := c.Param("mediaId")
	var req UpdateMediaSTRMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	media, err := h.libraryService.GetMediaByID(mediaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "影片不存在"})
		return
	}
	if media.StreamURL == "" && (req.StreamURL == nil || *req.StreamURL == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该影片不是 STRM 远程流，无法覆写"})
		return
	}

	if req.StreamURL != nil {
		media.StreamURL = strings.TrimSpace(*req.StreamURL)
	}
	if req.UserAgent != nil {
		media.StreamUA = *req.UserAgent
	}
	if req.Referer != nil {
		media.StreamReferer = *req.Referer
	}
	if req.Cookie != nil {
		media.StreamCookie = *req.Cookie
	}
	if req.RefreshURL != nil {
		media.StreamRefreshURL = *req.RefreshURL
	}
	if req.ClearHeaders {
		media.StreamHeaders = ""
	} else if req.Headers != nil {
		if len(*req.Headers) == 0 {
			media.StreamHeaders = ""
		} else {
			if b, err := json.Marshal(*req.Headers); err == nil {
				media.StreamHeaders = string(b)
			}
		}
	}

	if err := h.libraryService.UpdateMedia(media); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败: " + err.Error()})
		return
	}
	h.auditFromContext(c, "media.update_strm", "media", mediaID, "")
	c.JSON(http.StatusOK, gin.H{
		"message": "STRM 请求头已更新",
		"data": gin.H{
			"id":             media.ID,
			"stream_url":     media.StreamURL,
			"stream_ua":      media.StreamUA,
			"stream_referer": media.StreamReferer,
			"stream_cookie":  media.StreamCookie,
			"stream_headers": media.StreamHeaders,
		},
	})
}

// GetMediaSTRM 获取单条 media 的当前 STRM 覆写状态（用于管理页面回显）
// GET /api/admin/media/:mediaId/strm
func (h *AdminHandler) GetMediaSTRM(c *gin.Context) {
	mediaID := c.Param("mediaId")
	media, err := h.libraryService.GetMediaByID(mediaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "影片不存在"})
		return
	}
	var hdrMap map[string]string
	if media.StreamHeaders != "" {
		_ = json.Unmarshal([]byte(media.StreamHeaders), &hdrMap)
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id":                 media.ID,
		"title":              media.Title,
		"is_strm":            media.StreamURL != "",
		"stream_url":         media.StreamURL,
		"stream_ua":          media.StreamUA,
		"stream_referer":     media.StreamReferer,
		"stream_cookie":      media.StreamCookie,
		"stream_headers":     hdrMap,
		"stream_refresh_url": media.StreamRefreshURL,
	}})
}

// UpdateMediaMetadata 编辑媒体元数据
func (h *AdminHandler) UpdateMediaMetadata(c *gin.Context) {
	mediaID := c.Param("mediaId")

	var req UpdateMediaMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	media, err := h.libraryService.GetMediaByID(mediaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "影片不存在"})
		return
	}

	// 仅更新提供的字段
	if req.Title != nil {
		media.Title = *req.Title
	}
	if req.OrigTitle != nil {
		media.OrigTitle = *req.OrigTitle
	}
	if req.Year != nil {
		media.Year = *req.Year
	}
	if req.Overview != nil {
		media.Overview = *req.Overview
	}
	if req.Rating != nil {
		media.Rating = *req.Rating
	}
	if req.Genres != nil {
		media.Genres = *req.Genres
	}
	if req.Country != nil {
		media.Country = *req.Country
	}
	if req.Language != nil {
		media.Language = *req.Language
	}
	if req.Tagline != nil {
		media.Tagline = *req.Tagline
	}
	if req.Studio != nil {
		media.Studio = *req.Studio
	}

	if err := h.libraryService.UpdateMedia(media); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新元数据失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "元数据已更新", "data": media})
}

// ==================== 剧集合集管理 ====================

// UpdateSeriesMetadataRequest 编辑剧集合集元数据请求
type UpdateSeriesMetadataRequest struct {
	Title     *string  `json:"title"`
	OrigTitle *string  `json:"orig_title"`
	Year      *int     `json:"year"`
	Overview  *string  `json:"overview"`
	Rating    *float64 `json:"rating"`
	Genres    *string  `json:"genres"`
	Country   *string  `json:"country"`
	Language  *string  `json:"language"`
	Studio    *string  `json:"studio"`
}

// UpdateSeriesMetadata 编辑剧集合集元数据
func (h *AdminHandler) UpdateSeriesMetadata(c *gin.Context) {
	seriesID := c.Param("seriesId")

	var req UpdateSeriesMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	series, err := h.libraryService.GetSeriesByID(seriesID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "剧集合集不存在"})
		return
	}

	if req.Title != nil {
		series.Title = *req.Title
	}
	if req.OrigTitle != nil {
		series.OrigTitle = *req.OrigTitle
	}
	if req.Year != nil {
		series.Year = *req.Year
	}
	if req.Overview != nil {
		series.Overview = *req.Overview
	}
	if req.Rating != nil {
		series.Rating = *req.Rating
	}
	if req.Genres != nil {
		series.Genres = *req.Genres
	}
	if req.Country != nil {
		series.Country = *req.Country
	}
	if req.Language != nil {
		series.Language = *req.Language
	}
	if req.Studio != nil {
		series.Studio = *req.Studio
	}

	if err := h.libraryService.UpdateSeries(series); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新元数据失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "元数据已更新", "data": series})
}

// DeleteSeries 删除剧集合集记录（仅从数据库移除，不删除文件）
func (h *AdminHandler) DeleteSeries(c *gin.Context) {
	seriesID := c.Param("seriesId")

	if err := h.libraryService.DeleteSeries(seriesID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除剧集失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "剧集已删除"})
}

// ==================== 图片管理 ====================

// SearchTMDbImages 获取TMDb条目的所有可用图片
func (h *AdminHandler) SearchTMDbImages(c *gin.Context) {
	mediaType := c.DefaultQuery("type", "movie") // movie 或 tv
	tmdbID, err := strconv.Atoi(c.Query("tmdb_id"))
	if err != nil || tmdbID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供有效的 tmdb_id"})
		return
	}

	result, err := h.metadataService.SearchTMDbImages(mediaType, tmdbID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取图片列表失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// UploadMediaImage 上传图片到指定Media
func (h *AdminHandler) UploadMediaImage(c *gin.Context) {
	mediaID := c.Param("mediaId")
	imageType := c.DefaultQuery("type", "poster") // poster 或 backdrop
	if imageType != "poster" && imageType != "backdrop" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type 必须为 poster 或 backdrop"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传图片文件"})
		return
	}

	// 检查文件大小（10MB）
	if file.Size > 10*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片文件过大，最大支持10MB"})
		return
	}

	// 检查文件格式
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 JPG、PNG、WebP 格式"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取上传文件失败"})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取上传文件失败"})
		return
	}

	localPath, err := h.metadataService.SaveUploadedImageForMedia(mediaID, data, ext, imageType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存图片失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "图片已更新", "path": localPath})
}

// UploadSeriesImage 上传图片到指定Series
func (h *AdminHandler) UploadSeriesImage(c *gin.Context) {
	seriesID := c.Param("seriesId")
	imageType := c.DefaultQuery("type", "poster")
	if imageType != "poster" && imageType != "backdrop" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type 必须为 poster 或 backdrop"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传图片文件"})
		return
	}

	if file.Size > 10*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片文件过大，最大支持10MB"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 JPG、PNG、WebP 格式"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取上传文件失败"})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取上传文件失败"})
		return
	}

	localPath, err := h.metadataService.SaveUploadedImageForSeries(seriesID, data, ext, imageType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存图片失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "图片已更新", "path": localPath})
}

// SetMediaImageByURL 通过URL设置Media图片
func (h *AdminHandler) SetMediaImageByURL(c *gin.Context) {
	mediaID := c.Param("mediaId")

	var req struct {
		URL       string `json:"url" binding:"required"`
		ImageType string `json:"image_type"` // poster 或 backdrop
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供图片URL"})
		return
	}

	if req.ImageType == "" {
		req.ImageType = "poster"
	}
	if req.ImageType != "poster" && req.ImageType != "backdrop" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image_type 必须为 poster 或 backdrop"})
		return
	}

	localPath, err := h.metadataService.DownloadURLImageForMedia(mediaID, req.URL, req.ImageType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "下载图片失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "图片已更新", "path": localPath})
}

// SetSeriesImageByURL 通过URL设置Series图片
func (h *AdminHandler) SetSeriesImageByURL(c *gin.Context) {
	seriesID := c.Param("seriesId")

	var req struct {
		URL       string `json:"url" binding:"required"`
		ImageType string `json:"image_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供图片URL"})
		return
	}

	if req.ImageType == "" {
		req.ImageType = "poster"
	}
	if req.ImageType != "poster" && req.ImageType != "backdrop" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image_type 必须为 poster 或 backdrop"})
		return
	}

	localPath, err := h.metadataService.DownloadURLImageForSeries(seriesID, req.URL, req.ImageType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "下载图片失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "图片已更新", "path": localPath})
}

// SetMediaImageFromTMDb 从TMDb选择图片设置到Media
func (h *AdminHandler) SetMediaImageFromTMDb(c *gin.Context) {
	mediaID := c.Param("mediaId")

	var req struct {
		TMDbPath  string `json:"tmdb_path" binding:"required"` // TMDb图片路径，如 /abc123.jpg
		ImageType string `json:"image_type"`                   // poster 或 backdrop
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 tmdb_path"})
		return
	}

	if req.ImageType == "" {
		req.ImageType = "poster"
	}

	localPath, err := h.metadataService.DownloadTMDbImageForMedia(mediaID, req.TMDbPath, req.ImageType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "下载图片失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "图片已更新", "path": localPath})
}

// SetSeriesImageFromTMDb 从TMDb选择图片设置到Series
func (h *AdminHandler) SetSeriesImageFromTMDb(c *gin.Context) {
	seriesID := c.Param("seriesId")

	var req struct {
		TMDbPath  string `json:"tmdb_path" binding:"required"`
		ImageType string `json:"image_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 tmdb_path"})
		return
	}

	if req.ImageType == "" {
		req.ImageType = "poster"
	}

	localPath, err := h.metadataService.DownloadTMDbImageForSeries(seriesID, req.TMDbPath, req.ImageType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "下载图片失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "图片已更新", "path": localPath})
}
