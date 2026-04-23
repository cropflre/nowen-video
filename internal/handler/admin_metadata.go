package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ==================== TMDb 配置管理 ====================

// GetTMDbConfig 获取 TMDb API Key 配置状态
func (h *AdminHandler) GetTMDbConfig(c *gin.Context) {
	maskedKey := h.cfg.GetTMDbAPIKeyMasked()
	configured := h.cfg.GetTMDbAPIKey() != ""

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"configured": configured,
			"masked_key": maskedKey,
		},
	})
}

// UpdateTMDbConfigRequest 更新 TMDb API Key 请求
type UpdateTMDbConfigRequest struct {
	APIKey string `json:"api_key" binding:"required"`
}

// UpdateTMDbConfig 更新 TMDb API Key
func (h *AdminHandler) UpdateTMDbConfig(c *gin.Context) {
	var req UpdateTMDbConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供有效的 API Key"})
		return
	}

	key := req.APIKey
	if len(key) < 16 || len(key) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API Key 格式不正确，请检查后重试"})
		return
	}

	if err := h.cfg.SetTMDbAPIKey(key); err != nil {
		h.logger.Errorf("保存 TMDb API Key 失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	h.logger.Info("TMDb API Key 已更新")
	c.JSON(http.StatusOK, gin.H{
		"message": "TMDb API Key 已保存",
		"data": gin.H{
			"configured": true,
			"masked_key": h.cfg.GetTMDbAPIKeyMasked(),
		},
	})
}

// ClearTMDbConfig 清除 TMDb API Key
func (h *AdminHandler) ClearTMDbConfig(c *gin.Context) {
	if err := h.cfg.ClearTMDbAPIKey(); err != nil {
		h.logger.Errorf("清除 TMDb API Key 失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清除配置失败: " + err.Error()})
		return
	}

	h.logger.Info("TMDb API Key 已清除")
	c.JSON(http.StatusOK, gin.H{
		"message": "TMDb API Key 已清除",
		"data": gin.H{
			"configured": false,
			"masked_key": "",
		},
	})
}

// ==================== 手动元数据匹配 ====================

// SearchMetadataRequest 搜索元数据请求
type SearchMetadataRequest struct {
	Query     string `json:"query" binding:"required"`
	Year      int    `json:"year"`
	MediaType string `json:"media_type"` // movie, tv
}

// SearchMetadata 手动搜索TMDb元数据
func (h *AdminHandler) SearchMetadata(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供搜索关键词"})
		return
	}
	mediaType := c.DefaultQuery("type", "movie")
	year, _ := strconv.Atoi(c.Query("year"))

	results, err := h.metadataService.SearchTMDb(mediaType, query, year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": results})
}

// MatchMetadataRequest 手动关联元数据请求
type MatchMetadataRequest struct {
	TMDbID int `json:"tmdb_id" binding:"required"`
}

// MatchMetadata 手动关联TMDb元数据到指定媒体
func (h *AdminHandler) MatchMetadata(c *gin.Context) {
	mediaID := c.Param("mediaId")

	var req MatchMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if err := h.metadataService.MatchMediaWithTMDb(mediaID, req.TMDbID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "关联元数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "元数据已关联"})
}

// UnmatchMetadata 解除媒体的元数据匹配
func (h *AdminHandler) UnmatchMetadata(c *gin.Context) {
	mediaID := c.Param("mediaId")

	if err := h.metadataService.UnmatchMedia(mediaID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解除匹配失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已解除元数据匹配"})
}

// ==================== 剧集合集元数据管理 ====================

// MatchSeriesMetadata 手动匹配剧集合集元数据
func (h *AdminHandler) MatchSeriesMetadata(c *gin.Context) {
	seriesID := c.Param("seriesId")

	var req struct {
		TMDbID int `json:"tmdb_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 tmdb_id"})
		return
	}

	if err := h.metadataService.MatchSeriesWithTMDb(seriesID, req.TMDbID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "匹配失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "元数据已关联"})
}

// UnmatchSeriesMetadata 解除剧集合集的元数据匹配
func (h *AdminHandler) UnmatchSeriesMetadata(c *gin.Context) {
	seriesID := c.Param("seriesId")

	if err := h.metadataService.UnmatchSeries(seriesID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解除匹配失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已解除元数据匹配"})
}

// ScrapeSeriesMetadata 刷新剧集合集元数据
func (h *AdminHandler) ScrapeSeriesMetadata(c *gin.Context) {
	seriesID := c.Param("seriesId")

	if err := h.metadataService.ScrapeSeries(seriesID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "刷新元数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "元数据已刷新"})
}

// ==================== 豆瓣数据源管理 ====================

// SearchDouban 搜索豆瓣条目
func (h *AdminHandler) SearchDouban(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供搜索关键词"})
		return
	}
	year, _ := strconv.Atoi(c.Query("year"))

	results, err := h.metadataService.SearchDouban(query, year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "豆瓣搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": results})
}

// MatchMediaDouban 手动关联豆瓣条目到媒体
func (h *AdminHandler) MatchMediaDouban(c *gin.Context) {
	mediaID := c.Param("mediaId")

	var req struct {
		DoubanID string `json:"douban_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 douban_id"})
		return
	}

	if err := h.metadataService.MatchMediaWithDouban(mediaID, req.DoubanID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "关联豆瓣元数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已关联豆瓣元数据"})
}

// MatchSeriesDouban 手动关联豆瓣条目到剧集合集
func (h *AdminHandler) MatchSeriesDouban(c *gin.Context) {
	seriesID := c.Param("seriesId")

	var req struct {
		DoubanID string `json:"douban_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 douban_id"})
		return
	}

	if err := h.metadataService.MatchSeriesWithDouban(seriesID, req.DoubanID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "关联豆瓣元数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已关联豆瓣元数据"})
}

// ==================== TheTVDB 数据源管理 ====================

// SearchTheTVDB 搜索 TheTVDB 剧集
func (h *AdminHandler) SearchTheTVDB(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供搜索关键词"})
		return
	}
	year, _ := strconv.Atoi(c.Query("year"))

	results, err := h.metadataService.SearchTheTVDB(query, year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "TheTVDB 搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": results})
}

// MatchSeriesTheTVDB 手动关联 TheTVDB 条目到剧集合集
func (h *AdminHandler) MatchSeriesTheTVDB(c *gin.Context) {
	seriesID := c.Param("seriesId")

	var req struct {
		TVDBID int `json:"tvdb_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 tvdb_id"})
		return
	}

	if err := h.metadataService.MatchSeriesWithTheTVDB(seriesID, req.TVDBID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "关联 TheTVDB 元数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已关联 TheTVDB 元数据"})
}

// ==================== Bangumi 数据源管理 ====================

// SearchBangumi 搜索 Bangumi 条目
func (h *AdminHandler) SearchBangumi(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供搜索关键词"})
		return
	}

	// type: 2=动画, 6=三次元(电视剧/电影)
	subjectType, _ := strconv.Atoi(c.DefaultQuery("type", "2"))
	if subjectType != 1 && subjectType != 2 && subjectType != 3 && subjectType != 4 && subjectType != 6 {
		subjectType = 2 // 默认动画
	}
	year, _ := strconv.Atoi(c.Query("year"))

	results, err := h.metadataService.SearchBangumi(query, subjectType, year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": results})
}

// GetBangumiSubject 获取 Bangumi 条目详情
func (h *AdminHandler) GetBangumiSubject(c *gin.Context) {
	subjectID, err := strconv.Atoi(c.Param("subjectId"))
	if err != nil || subjectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供有效的 Bangumi 条目 ID"})
		return
	}

	subject, err := h.metadataService.GetBangumiSubjectDetail(subjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取条目详情失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": subject})
}

// MatchMediaBangumi 手动关联 Bangumi 条目到媒体
func (h *AdminHandler) MatchMediaBangumi(c *gin.Context) {
	mediaID := c.Param("mediaId")

	var req struct {
		BangumiID int `json:"bangumi_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 bangumi_id"})
		return
	}

	if err := h.metadataService.MatchMediaWithBangumi(mediaID, req.BangumiID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "关联 Bangumi 元数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已关联 Bangumi 元数据"})
}

// MatchSeriesBangumi 手动关联 Bangumi 条目到剧集合集
func (h *AdminHandler) MatchSeriesBangumi(c *gin.Context) {
	seriesID := c.Param("seriesId")

	var req struct {
		BangumiID int `json:"bangumi_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 bangumi_id"})
		return
	}

	if err := h.metadataService.MatchSeriesWithBangumi(seriesID, req.BangumiID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "关联 Bangumi 元数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已关联 Bangumi 元数据"})
}

// GetBangumiConfig 获取 Bangumi 配置状态
func (h *AdminHandler) GetBangumiConfig(c *gin.Context) {
	token := h.cfg.Secrets.BangumiAccessToken
	configured := token != ""
	maskedToken := ""
	if configured {
		if len(token) <= 8 {
			maskedToken = strings.Repeat("*", len(token))
		} else {
			maskedToken = token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"configured":   configured,
			"masked_token": maskedToken,
		},
	})
}

// UpdateBangumiConfig 更新 Bangumi Access Token
func (h *AdminHandler) UpdateBangumiConfig(c *gin.Context) {
	var req struct {
		AccessToken string `json:"access_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 Access Token"})
		return
	}

	h.cfg.Secrets.BangumiAccessToken = req.AccessToken
	h.logger.Info("Bangumi Access Token 已更新")

	c.JSON(http.StatusOK, gin.H{
		"message": "Bangumi Access Token 已保存",
		"data": gin.H{
			"configured": true,
		},
	})
}

// ClearBangumiConfig 清除 Bangumi Access Token
func (h *AdminHandler) ClearBangumiConfig(c *gin.Context) {
	h.cfg.Secrets.BangumiAccessToken = ""
	h.logger.Info("Bangumi Access Token 已清除")

	c.JSON(http.StatusOK, gin.H{
		"message": "Bangumi Access Token 已清除",
		"data": gin.H{
			"configured": false,
		},
	})
}

// ==================== 豆瓣 Cookie 配置管理 ====================

// GetDoubanConfig 获取豆瓣 Cookie 配置状态
func (h *AdminHandler) GetDoubanConfig(c *gin.Context) {
	maskedCookie := h.cfg.GetDoubanCookieMasked()
	configured := h.cfg.GetDoubanCookie() != ""

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"configured":    configured,
			"masked_cookie": maskedCookie,
		},
	})
}

// UpdateDoubanConfigRequest 更新豆瓣 Cookie 请求
type UpdateDoubanConfigRequest struct {
	Cookie string `json:"cookie" binding:"required"`
}

// UpdateDoubanConfig 更新豆瓣登录 Cookie
func (h *AdminHandler) UpdateDoubanConfig(c *gin.Context) {
	var req UpdateDoubanConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供有效的 Cookie 字符串"})
		return
	}

	cookie := strings.TrimSpace(req.Cookie)
	// 长度校验：豆瓣 Cookie 通常不少于 50 字符、不超过 4096
	if len(cookie) < 20 || len(cookie) > 4096 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cookie 格式不正确，请复制浏览器中完整的 Cookie 字符串"})
		return
	}

	// 简单合法性检查：应包含豆瓣登录态核心字段之一
	if !strings.Contains(cookie, "bid=") && !strings.Contains(cookie, "dbcl2=") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cookie 不包含豆瓣登录态字段（bid / dbcl2），请检查后重试"})
		return
	}

	if err := h.cfg.SetDoubanCookie(cookie); err != nil {
		h.logger.Errorf("保存豆瓣 Cookie 失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	h.logger.Info("豆瓣 Cookie 已更新")
	c.JSON(http.StatusOK, gin.H{
		"message": "豆瓣 Cookie 已保存",
		"data": gin.H{
			"configured":    true,
			"masked_cookie": h.cfg.GetDoubanCookieMasked(),
		},
	})
}

// ClearDoubanConfig 清除豆瓣 Cookie
func (h *AdminHandler) ClearDoubanConfig(c *gin.Context) {
	if err := h.cfg.ClearDoubanCookie(); err != nil {
		h.logger.Errorf("清除豆瓣 Cookie 失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清除配置失败: " + err.Error()})
		return
	}

	h.logger.Info("豆瓣 Cookie 已清除")
	c.JSON(http.StatusOK, gin.H{
		"message": "豆瓣 Cookie 已清除",
		"data": gin.H{
			"configured":    false,
			"masked_cookie": "",
		},
	})
}

// ValidateDoubanConfig 校验当前豆瓣 Cookie 是否有效（登录态探测）
func (h *AdminHandler) ValidateDoubanConfig(c *gin.Context) {
	if h.cfg.GetDoubanCookie() == "" {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"valid":   false,
				"message": "未配置 Cookie，当前为匿名模式",
			},
		})
		return
	}

	valid, username, err := h.metadataService.ValidateDoubanCookie()
	if err != nil {
		h.logger.Warnf("校验豆瓣 Cookie 失败: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"valid":   false,
				"message": "校验失败: " + err.Error(),
			},
		})
		return
	}

	if !valid {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"valid":   false,
				"message": "Cookie 已失效，请重新从浏览器复制最新 Cookie",
			},
		})
		return
	}

	msg := "Cookie 有效，豆瓣登录态正常"
	if username != "" {
		msg = "Cookie 有效，已识别豆瓣账号：" + username
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"valid":    true,
			"username": username,
			"message":  msg,
		},
	})
}
