package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// AdultScraperHandler 番号刮削管理处理器
type AdultScraperHandler struct {
	scraperService *service.AdultScraperService
	cfg            *config.Config
	logger         *zap.SugaredLogger

	// P3~P5 扩展依赖（可选注入）
	batchService *service.AdultBatchService
	taskStore    *service.AdultTaskStore
	proxyManager *service.AdultProxyManager
	cache        *service.AdultMetadataCache
	scheduler    *service.AdultScheduler

	// Python 子进程启动器（可选注入，用于运行时按需拉起 Python 微服务）
	pythonLauncher *service.AdultPythonLauncher

	// 文件夹扫描 + 自定义文件夹刮削服务（参考 mdcx 项目）
	folderBatch *service.AdultFolderBatchService
}

// SetPythonLauncher 注入 Python 微服务启动器
// 注入后，UpdateConfig 若检测到配置变更，会按需拉起 / 回收 Python 子进程
func (h *AdultScraperHandler) SetPythonLauncher(l *service.AdultPythonLauncher) {
	h.pythonLauncher = l
}

// SetFolderBatchService 注入自定义文件夹刮削服务
func (h *AdultScraperHandler) SetFolderBatchService(fs *service.AdultFolderBatchService) {
	h.folderBatch = fs
}

// SetP3Services 注入 P3~P5 扩展服务（由 NewHandlers 调用）
func (h *AdultScraperHandler) SetP3Services(
	batch *service.AdultBatchService,
	store *service.AdultTaskStore,
	proxy *service.AdultProxyManager,
	cache *service.AdultMetadataCache,
	scheduler *service.AdultScheduler,
) {
	h.batchService = batch
	h.taskStore = store
	h.proxyManager = proxy
	h.cache = cache
	h.scheduler = scheduler
}

// ==================== 配置查询 ====================

// GetConfig 获取番号刮削配置状态
func (h *AdultScraperHandler) GetConfig(c *gin.Context) {
	cfg := h.cfg.AdultScraper

	// 构建数据源状态列表
	sources := []gin.H{}

	// JavBus
	javbusURL := cfg.JavBusURL
	if javbusURL == "" {
		javbusURL = "https://www.javbus.com"
	}
	sources = append(sources, gin.H{
		"id":       "javbus",
		"name":     "JavBus",
		"type":     "go_native",
		"enabled":  cfg.EnableJavBus,
		"url":      javbusURL,
		"priority": 1,
		"desc":     "Go 原生爬虫，纯正则解析 HTML，零外部依赖",
	})

	// JavDB
	javdbURL := cfg.JavDBURL
	if javdbURL == "" {
		javdbURL = "https://javdb.com"
	}
	sources = append(sources, gin.H{
		"id":       "javdb",
		"name":     "JavDB",
		"type":     "go_native",
		"enabled":  cfg.EnableJavDB,
		"url":      javdbURL,
		"priority": 2,
		"desc":     "Go 原生爬虫，搜索 + 详情页两步刮削，支持评分",
	})

	// Python 微服务
	pythonEnabled := cfg.PythonServiceURL != ""
	sources = append(sources, gin.H{
		"id":       "python",
		"name":     "Python 微服务",
		"type":     "python_service",
		"enabled":  pythonEnabled,
		"url":      cfg.PythonServiceURL,
		"priority": 3,
		"desc":     "BeautifulSoup 解析，处理 Cloudflare 等强反爬场景（Fallback）",
	})

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"enabled":              cfg.Enabled,
			"sources":              sources,
			"min_request_interval": cfg.MinRequestInterval,
			"max_request_interval": cfg.MaxRequestInterval,
			"supported_formats": []gin.H{
				{"type": "standard", "pattern": "ABC-123", "example": "SSIS-001, MIDV-123"},
				{"type": "fc2", "pattern": "FC2-PPV-1234567", "example": "FC2-PPV-1234567"},
				{"type": "uncensored", "pattern": "123456-789", "example": "1pondo/caribbeancom 格式"},
				{"type": "heyzo", "pattern": "HEYZO-1234", "example": "HEYZO-1234"},
			},
			// Cookie 登录配置（参考 mdcx：每个站点一个完整 Cookie 字符串）
			// 出于安全考虑，返回原始 cookie 完整内容仅在 admin 场景下进行（本接口仅 admin 可访问）
			"cookies": gin.H{
				"javbus":    cfg.CookieJavBus,
				"javdb":     cfg.CookieJavDB,
				"freejavbt": cfg.CookieFreejavbt,
				"jav321":    cfg.CookieJav321,
				"fanza":     cfg.CookieFanza,
				"mgstage":   cfg.CookieMGStage,
				"fc2hub":    cfg.CookieFC2Hub,
			},
		},
	})
}

// UpdateConfig 更新番号刮削配置
func (h *AdultScraperHandler) UpdateConfig(c *gin.Context) {
	var req struct {
		Enabled             *bool   `json:"enabled"`
		EnableJavBus        *bool   `json:"enable_javbus"`
		JavBusURL           *string `json:"javbus_url"`
		EnableJavDB         *bool   `json:"enable_javdb"`
		JavDBURL            *string `json:"javdb_url"`
		PythonServiceURL    *string `json:"python_service_url"`
		PythonServiceAPIKey *string `json:"python_service_api_key"`
		AutoStartPython     *bool   `json:"auto_start_python"`
		MinRequestInterval  *int    `json:"min_request_interval"`
		MaxRequestInterval  *int    `json:"max_request_interval"`
		// Cookie 登录（参考 mdcx，每个站点一个完整 Cookie 字符串）
		CookieJavBus    *string `json:"cookie_javbus"`
		CookieJavDB     *string `json:"cookie_javdb"`
		CookieFreejavbt *string `json:"cookie_freejavbt"`
		CookieJav321    *string `json:"cookie_jav321"`
		CookieFanza     *string `json:"cookie_fanza"`
		CookieMGStage   *string `json:"cookie_mgstage"`
		CookieFC2Hub    *string `json:"cookie_fc2hub"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	// 记录变更前状态，用于判断是否需要拉起 / 回收 Python 子进程
	oldEnabled := h.cfg.AdultScraper.Enabled
	oldAutoStart := h.cfg.AdultScraper.AutoStartPython

	// 更新配置（使用指针语义：提供字段即更新，未提供保持原值）
	if req.Enabled != nil {
		h.cfg.AdultScraper.Enabled = *req.Enabled
	}
	if req.EnableJavBus != nil {
		h.cfg.AdultScraper.EnableJavBus = *req.EnableJavBus
	}
	if req.JavBusURL != nil {
		h.cfg.AdultScraper.JavBusURL = *req.JavBusURL
	}
	if req.EnableJavDB != nil {
		h.cfg.AdultScraper.EnableJavDB = *req.EnableJavDB
	}
	if req.JavDBURL != nil {
		h.cfg.AdultScraper.JavDBURL = *req.JavDBURL
	}
	if req.PythonServiceURL != nil {
		h.cfg.AdultScraper.PythonServiceURL = *req.PythonServiceURL
	}
	if req.PythonServiceAPIKey != nil {
		h.cfg.AdultScraper.PythonServiceAPIKey = *req.PythonServiceAPIKey
	}
	if req.AutoStartPython != nil {
		h.cfg.AdultScraper.AutoStartPython = *req.AutoStartPython
	}
	if req.MinRequestInterval != nil && *req.MinRequestInterval > 0 {
		h.cfg.AdultScraper.MinRequestInterval = *req.MinRequestInterval
	}
	if req.MaxRequestInterval != nil && *req.MaxRequestInterval > 0 {
		h.cfg.AdultScraper.MaxRequestInterval = *req.MaxRequestInterval
	}
	// Cookie 登录配置
	if req.CookieJavBus != nil {
		h.cfg.AdultScraper.CookieJavBus = *req.CookieJavBus
	}
	if req.CookieJavDB != nil {
		h.cfg.AdultScraper.CookieJavDB = *req.CookieJavDB
	}
	if req.CookieFreejavbt != nil {
		h.cfg.AdultScraper.CookieFreejavbt = *req.CookieFreejavbt
	}
	if req.CookieJav321 != nil {
		h.cfg.AdultScraper.CookieJav321 = *req.CookieJav321
	}
	if req.CookieFanza != nil {
		h.cfg.AdultScraper.CookieFanza = *req.CookieFanza
	}
	if req.CookieMGStage != nil {
		h.cfg.AdultScraper.CookieMGStage = *req.CookieMGStage
	}
	if req.CookieFC2Hub != nil {
		h.cfg.AdultScraper.CookieFC2Hub = *req.CookieFC2Hub
	}

	// 持久化配置到磁盘，确保重启后不丢失
	if err := h.cfg.SaveAdultScraperConfig(); err != nil {
		h.logger.Warnf("持久化番号刮削配置失败: %v", err)
	}

	// 按需拉起 / 回收 Python 子进程
	// - 启用变为 true 且 AutoStartPython=true 时，尝试拉起
	// - 启用变为 false 时，回收已运行的进程
	if h.pythonLauncher != nil {
		nowEnabled := h.cfg.AdultScraper.Enabled
		nowAutoStart := h.cfg.AdultScraper.AutoStartPython
		switch {
		case !oldEnabled && nowEnabled && nowAutoStart:
			h.logger.Infof("番号刮削已启用，尝试拉起 Python 微服务子进程...")
			if err := h.pythonLauncher.Start(); err != nil {
				h.logger.Warnf("拉起 Python 微服务失败: %v", err)
			}
		case oldEnabled && !nowEnabled:
			h.logger.Infof("番号刮削已禁用，回收 Python 微服务子进程")
			h.pythonLauncher.Stop()
		case nowEnabled && !oldAutoStart && nowAutoStart:
			// 用户在运行时打开了自动启动开关
			if err := h.pythonLauncher.Start(); err != nil {
				h.logger.Warnf("拉起 Python 微服务失败: %v", err)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置已更新"})
}

// ==================== 手动刮削 ====================

// ScrapeByCode 手动刮削指定番号
func (h *AdultScraperHandler) ScrapeByCode(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供番号（code 字段）"})
		return
	}

	meta, err := h.scraperService.ScrapeByCode(req.Code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": meta})
}

// ParseCode 测试番号识别（使用增强版引擎，支持 30+ 种番号格式）
func (h *AdultScraperHandler) ParseCode(c *gin.Context) {
	input := c.Query("input")
	if input == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 input 参数"})
		return
	}

	// 使用增强版引擎（借鉴 mdcx-master 项目，支持 30+ 种番号格式）
	info := service.ParseCodeEnhanced(input)

	// 同时保留旧接口字段（向后兼容）
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"input":     input,
			"code":      info.Number,
			"code_type": info.CodeType,
			"is_adult":  info.IsAdult,
			// 增强字段
			"letters":      info.Letters,
			"short_number": info.ShortNumber,
			"mosaic":       info.Mosaic,      // 有码/无码/国产/欧美
			"cd_part":      info.CDPart,      // CD1/PART2 等
			"has_chn_sub":  info.HasChnSub,   // 中文字幕版
		},
	})
}

// ==================== Python 微服务健康检查 ====================

// PythonServiceHealth 检查 Python 微服务健康状态
func (h *AdultScraperHandler) PythonServiceHealth(c *gin.Context) {
	url := h.cfg.AdultScraper.PythonServiceURL
	if url == "" {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"configured": false,
				"status":     "not_configured",
				"message":    "Python 微服务未配置",
			},
		})
		return
	}

	// 尝试访问健康检查端点
	resp, err := http.Get(url + "/health")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"configured": true,
				"status":     "offline",
				"message":    "无法连接到 Python 微服务: " + err.Error(),
				"url":        url,
			},
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"configured": true,
				"status":     "online",
				"message":    "Python 微服务运行正常",
				"url":        url,
			},
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"configured": true,
				"status":     "error",
				"message":    "Python 微服务返回异常状态码",
				"url":        url,
				"http_code":  resp.StatusCode,
			},
		})
	}
}
