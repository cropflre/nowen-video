// Package handler 番号刮削 P2 扩展 API
// 包含：聚合刮削（多源并发）、测试所有源、映射表管理
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

// ==================== P2：聚合刮削 API ====================

// ScrapeAggregated 多数据源并发聚合刮削
// POST /api/admin/adult-scraper/aggregate
// Body: {"code": "SSIS-001"}
// Response: {"merged": meta, "sources": {"fanza": {...}, "javbus": {...}, ...}}
func (h *AdultScraperHandler) ScrapeAggregated(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供番号（code 字段）"})
		return
	}

	merged, sources, err := h.scraperService.ScrapeByCodeAggregated(req.Code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"merged":  merged,
			"sources": sources,
			"count":   len(sources),
		},
	})
}

// TestAllSources 测试所有已启用数据源对同一番号的响应情况
// GET /api/admin/adult-scraper/test-sources?code=SSIS-001
// 返回每个数据源的成功/失败状态和耗时
func (h *AdultScraperHandler) TestAllSources(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 code 参数"})
		return
	}

	_, sources, err := h.scraperService.ScrapeByCodeAggregated(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"code":    code,
				"success": false,
				"sources": gin.H{},
				"error":   err.Error(),
			},
		})
		return
	}

	// 构造每个数据源的结果摘要
	srcSummary := gin.H{}
	for name, meta := range sources {
		srcSummary[name] = gin.H{
			"success":   true,
			"title":     meta.Title,
			"cover":     meta.Cover != "",
			"plot":      meta.Plot != "",
			"actresses": len(meta.Actresses),
			"genres":    len(meta.Genres),
			"fanart":    len(meta.ExtraFanart),
			"trailer":   meta.Trailer != "",
			"rating":    meta.Rating,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"code":    code,
			"success": true,
			"sources": srcSummary,
			"count":   len(sources),
		},
	})
}

// ==================== P2：映射表管理 API ====================

// GetMappings 查看当前所有别名映射
// GET /api/admin/adult-scraper/mappings
func (h *AdultScraperHandler) GetMappings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"note": "别名映射：左侧为别名，右侧为规范名。系统自动将左侧匹配到的数据替换为右侧。",
			"description": gin.H{
				"actress": "演员别名映射（日文/繁体/英文 -> 简体中文规范名）",
				"studio":  "片商别名映射",
				"series":  "系列别名映射",
				"genre":   "标签别名映射",
			},
		},
	})
}

// AddMappings 批量新增映射表
// POST /api/admin/adult-scraper/mappings
// Body: {"type": "actress", "mappings": {"三上悠亜": "三上悠亚", ...}}
func (h *AdultScraperHandler) AddMappings(c *gin.Context) {
	var req struct {
		Type     string            `json:"type" binding:"required"` // actress/studio/series/genre
		Mappings map[string]string `json:"mappings" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}
	if len(req.Mappings) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mappings 不能为空"})
		return
	}

	switch req.Type {
	case "actress":
		service.SetActressAliases(req.Mappings)
	case "studio":
		service.SetStudioAliases(req.Mappings)
	case "series":
		service.SetSeriesAliases(req.Mappings)
	case "genre":
		service.SetGenreAliases(req.Mappings)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "type 必须是 actress/studio/series/genre 之一"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "映射已添加",
		"count":   len(req.Mappings),
		"type":    req.Type,
	})
}

// TestNormalize 测试规范化效果
// POST /api/admin/adult-scraper/normalize-test
// Body: {"actresses": ["三上悠亜","Mikami Yua"], "studio": "S1"}
func (h *AdultScraperHandler) TestNormalize(c *gin.Context) {
	var req struct {
		Actresses []string `json:"actresses"`
		Studio    string   `json:"studio"`
		Series    string   `json:"series"`
		Genres    []string `json:"genres"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"actresses": gin.H{
				"input":      req.Actresses,
				"normalized": service.NormalizeActresses(req.Actresses),
			},
			"studio": gin.H{
				"input":      req.Studio,
				"normalized": service.NormalizeStudio(req.Studio),
			},
			"series": gin.H{
				"input":      req.Series,
				"normalized": service.NormalizeSeries(req.Series),
			},
			"genres": gin.H{
				"input":      req.Genres,
				"normalized": service.NormalizeGenres(req.Genres),
			},
		},
	})
}

// ==================== P2：更新扩展后的配置 ====================

// UpdateConfigExtended 支持 P1/P2 所有新配置项的完整更新接口
// PUT /api/admin/adult-scraper/config-ext
func (h *AdultScraperHandler) UpdateConfigExtended(c *gin.Context) {
	var req struct {
		// 基础
		Enabled             *bool  `json:"enabled"`
		EnableJavBus        *bool  `json:"enable_javbus"`
		JavBusURL           string `json:"javbus_url"`
		EnableJavDB         *bool  `json:"enable_javdb"`
		JavDBURL            string `json:"javdb_url"`
		PythonServiceURL    string `json:"python_service_url"`
		PythonServiceAPIKey string `json:"python_service_api_key"`
		MinRequestInterval  *int   `json:"min_request_interval"`
		MaxRequestInterval  *int   `json:"max_request_interval"`

		// P1 数据源
		EnableFreejavbt *bool  `json:"enable_freejavbt"`
		FreejavbtURL    string `json:"freejavbt_url"`
		EnableJav321    *bool  `json:"enable_jav321"`
		Jav321URL       string `json:"jav321_url"`

		// P1 多媒体资源
		DownloadExtraFanart *bool `json:"download_extra_fanart"`
		MaxExtraFanart      *int  `json:"max_extra_fanart"`
		DownloadActorPhoto  *bool `json:"download_actor_photo"`
		FetchTrailer        *bool `json:"fetch_trailer"`

		// P1 翻译
		EnableTranslate     *bool  `json:"enable_translate"`
		TranslateProvider   string `json:"translate_provider"`
		TranslateEndpoint   string `json:"translate_endpoint"`
		TranslateAPIKey     string `json:"translate_api_key"`
		TranslateAPISecret  string `json:"translate_api_secret"`
		TranslateTargetLang string `json:"translate_target_lang"`

		// P2 数据源
		EnableFanza   *bool  `json:"enable_fanza"`
		FanzaURL      string `json:"fanza_url"`
		EnableMGStage *bool  `json:"enable_mgstage"`
		MGStageURL    string `json:"mgstage_url"`
		EnableFC2Hub  *bool  `json:"enable_fc2hub"`
		FC2HubURL     string `json:"fc2hub_url"`

		// P2 其他
		EnableAggregatedMode *bool `json:"enable_aggregated_mode"`
		EnablePosterCrop     *bool `json:"enable_poster_crop"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	cfg := &h.cfg.AdultScraper

	// 基础
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.EnableJavBus != nil {
		cfg.EnableJavBus = *req.EnableJavBus
	}
	if req.JavBusURL != "" {
		cfg.JavBusURL = req.JavBusURL
	}
	if req.EnableJavDB != nil {
		cfg.EnableJavDB = *req.EnableJavDB
	}
	if req.JavDBURL != "" {
		cfg.JavDBURL = req.JavDBURL
	}
	cfg.PythonServiceURL = req.PythonServiceURL
	cfg.PythonServiceAPIKey = req.PythonServiceAPIKey
	if req.MinRequestInterval != nil && *req.MinRequestInterval > 0 {
		cfg.MinRequestInterval = *req.MinRequestInterval
	}
	if req.MaxRequestInterval != nil && *req.MaxRequestInterval > 0 {
		cfg.MaxRequestInterval = *req.MaxRequestInterval
	}

	// P1 数据源
	if req.EnableFreejavbt != nil {
		cfg.EnableFreejavbt = *req.EnableFreejavbt
	}
	if req.FreejavbtURL != "" {
		cfg.FreejavbtURL = req.FreejavbtURL
	}
	if req.EnableJav321 != nil {
		cfg.EnableJav321 = *req.EnableJav321
	}
	if req.Jav321URL != "" {
		cfg.Jav321URL = req.Jav321URL
	}

	// P1 多媒体
	if req.DownloadExtraFanart != nil {
		cfg.DownloadExtraFanart = *req.DownloadExtraFanart
	}
	if req.MaxExtraFanart != nil {
		cfg.MaxExtraFanart = *req.MaxExtraFanart
	}
	if req.DownloadActorPhoto != nil {
		cfg.DownloadActorPhoto = *req.DownloadActorPhoto
	}
	if req.FetchTrailer != nil {
		cfg.FetchTrailer = *req.FetchTrailer
	}

	// P1 翻译
	if req.EnableTranslate != nil {
		cfg.EnableTranslate = *req.EnableTranslate
	}
	if req.TranslateProvider != "" {
		cfg.TranslateProvider = req.TranslateProvider
	}
	if req.TranslateEndpoint != "" {
		cfg.TranslateEndpoint = req.TranslateEndpoint
	}
	if req.TranslateAPIKey != "" {
		cfg.TranslateAPIKey = req.TranslateAPIKey
	}
	if req.TranslateAPISecret != "" {
		cfg.TranslateAPISecret = req.TranslateAPISecret
	}
	if req.TranslateTargetLang != "" {
		cfg.TranslateTargetLang = req.TranslateTargetLang
	}

	// P2 数据源
	if req.EnableFanza != nil {
		cfg.EnableFanza = *req.EnableFanza
	}
	if req.FanzaURL != "" {
		cfg.FanzaURL = req.FanzaURL
	}
	if req.EnableMGStage != nil {
		cfg.EnableMGStage = *req.EnableMGStage
	}
	if req.MGStageURL != "" {
		cfg.MGStageURL = req.MGStageURL
	}
	if req.EnableFC2Hub != nil {
		cfg.EnableFC2Hub = *req.EnableFC2Hub
	}
	if req.FC2HubURL != "" {
		cfg.FC2HubURL = req.FC2HubURL
	}

	// P2 其他
	if req.EnableAggregatedMode != nil {
		cfg.EnableAggregatedMode = *req.EnableAggregatedMode
	}
	if req.EnablePosterCrop != nil {
		cfg.EnablePosterCrop = *req.EnablePosterCrop
	}

	c.JSON(http.StatusOK, gin.H{"message": "扩展配置已更新"})
}
