package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

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

// ValidateTMDbConfig 测试当前已保存的 TMDb API Key 是否可用
// GET /api/admin/settings/tmdb/validate
//
// 始终返回 200，结果放在 data.valid / data.message 中，方便前端统一处理。
func (h *AdminHandler) ValidateTMDbConfig(c *gin.Context) {
	if h.cfg.GetTMDbAPIKey() == "" {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"valid":   false,
				"message": "尚未配置 TMDb API Key，请先填写并保存",
			},
		})
		return
	}

	ok, msg := h.metadataService.PingTMDb("")
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"valid":   ok,
			"message": msg,
		},
	})
}

// TestTMDbAPIKeyRequest 临时测试 TMDb API Key 的请求体
type TestTMDbAPIKeyRequest struct {
	APIKey string `json:"api_key" binding:"required"`
}

// TestTMDbAPIKey 测试"用户输入但尚未保存"的 TMDb API Key 是否可用
// POST /api/admin/settings/tmdb/test
// Body: { "api_key": "xxx" }
//
// 不会修改任何配置，纯粹用于保存前的联通性/有效性预检。
func (h *AdminHandler) TestTMDbAPIKey(c *gin.Context) {
	var req TestTMDbAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供要测试的 api_key"})
		return
	}

	key := strings.TrimSpace(req.APIKey)
	if len(key) < 16 || len(key) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API Key 格式不正确，请检查后重试"})
		return
	}

	ok, msg := h.metadataService.PingTMDb(key)
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"valid":   ok,
			"message": msg,
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

	// 硬性校验：必须包含豆瓣登录凭证 dbcl2（bid 只是匿名访客 ID，不代表登录态）
	if !strings.Contains(cookie, "dbcl2=") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cookie 中缺少豆瓣登录凭证 dbcl2（通常被 HttpOnly 保护，JS 无法读取）。请使用浏览器扩展 Cookie-Editor / EditThisCookie 导出完整 Cookie 后再粘贴"})
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
		// 区分"缺少 dbcl2"和"dbcl2 已过期"两种情况，给用户更精确的提示
		saved := h.cfg.GetDoubanCookie()
		msg := "Cookie 已过期或被风控，请退出豆瓣重新登录后再导出"
		if !strings.Contains(saved, "dbcl2=") {
			msg = "当前 Cookie 中不含登录凭证 dbcl2，豆瓣以匿名身份访问被重定向。请使用浏览器扩展（Cookie-Editor / EditThisCookie）导出完整 Cookie（包含 HttpOnly 字段）后重新保存"
		}
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"valid":   false,
				"message": msg,
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

// ==================== 豆瓣 Cookie 懒人版一键导入 ====================
//
// 流程：
//  1. 管理员点击"懒人版登录" → 前端请求 CreateDoubanImportToken，获得一次性 token 与脚本
//  2. 管理员把 Bookmarklet 拖到书签栏 / 或浏览器插件触发 / 或手动执行脚本片段
//  3. 已登录的 douban.com 页面执行该脚本，读取 document.cookie 并 POST 到
//     /api/admin/settings/douban/import?token=xxx 完成导入
//  4. 前端轮询 GetDoubanImportTokenStatus 获知导入状态，自动刷新 UI

// doubanImportToken 一次性导入令牌
type doubanImportToken struct {
	Token     string
	ExpiresAt time.Time
	Consumed  bool      // 是否已被使用
	Success   bool      // 导入是否成功
	Message   string    // 导入结果消息
	Username  string    // 豆瓣用户名（若成功）
	UpdatedAt time.Time // 状态最后更新时间
}

// doubanImportTokens 全局 token 存储（进程内存，重启失效即可）
var (
	doubanImportTokens   = make(map[string]*doubanImportToken)
	doubanImportTokensMu sync.Mutex
)

const doubanImportTokenTTL = 5 * time.Minute

// cleanupExpiredDoubanImportTokens 清理过期 token（调用处已持有锁）
func cleanupExpiredDoubanImportTokens() {
	now := time.Now()
	for k, v := range doubanImportTokens {
		if now.After(v.ExpiresAt) {
			delete(doubanImportTokens, k)
		}
	}
}

// CreateDoubanImportToken 创建一次性导入 token，并返回 Bookmarklet / 脚本片段
func (h *AdminHandler) CreateDoubanImportToken(c *gin.Context) {
	token, err := generateSecureToken(24)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 token 失败: " + err.Error()})
		return
	}

	doubanImportTokensMu.Lock()
	cleanupExpiredDoubanImportTokens()
	doubanImportTokens[token] = &doubanImportToken{
		Token:     token,
		ExpiresAt: time.Now().Add(doubanImportTokenTTL),
		UpdatedAt: time.Now(),
	}
	doubanImportTokensMu.Unlock()

	// 构造目标 URL（优先使用请求来源站点的绝对地址，保证脚本在 douban.com 跨域时能找到后端）
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := c.Request.Host
	if fwdHost := c.GetHeader("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}
	importURL := fmt.Sprintf("%s://%s/api/admin/settings/douban/import?token=%s", scheme, host, token)

	// Bookmarklet: 用户拖到书签栏 → 在已登录的 douban.com 页面点击即可
	bookmarklet := buildDoubanBookmarklet(importURL)

	// 纯净的一段 JS（控制台 / 浏览器插件使用）
	script := buildDoubanImportScript(importURL)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"token":       token,
			"expires_in":  int(doubanImportTokenTTL.Seconds()),
			"expires_at":  doubanImportTokens[token].ExpiresAt.Unix(),
			"import_url":  importURL,
			"bookmarklet": bookmarklet,
			"script":      script,
			"douban_url":  "https://www.douban.com/",
		},
	})
}

// GetDoubanImportTokenStatus 查询 token 当前状态（前端轮询）
func (h *AdminHandler) GetDoubanImportTokenStatus(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 token 参数"})
		return
	}

	doubanImportTokensMu.Lock()
	defer doubanImportTokensMu.Unlock()
	cleanupExpiredDoubanImportTokens()

	item, ok := doubanImportTokens[token]
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"status":  "expired",
				"message": "token 已过期或不存在，请重新生成",
			},
		})
		return
	}

	status := "pending"
	if item.Consumed {
		if item.Success {
			status = "success"
		} else {
			status = "failed"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"status":         status,
			"message":        item.Message,
			"username":       item.Username,
			"expires_at":     item.ExpiresAt.Unix(),
			"remaining_secs": int(time.Until(item.ExpiresAt).Seconds()),
		},
	})
}

// ImportDoubanCookieRequest 懒人版导入请求体
type ImportDoubanCookieRequest struct {
	Cookie string `json:"cookie" binding:"required"`
}

// ImportDoubanCookie 懒人版一键导入豆瓣 Cookie（由 Bookmarklet / 扩展 调用）
//
// 请求：POST /api/admin/settings/douban/import?token=xxx
// Body：{"cookie": "bid=xxx; dbcl2=xxx; ..."}
//
// 注意：此接口使用独立的 CORS 放行中间件以允许 douban.com 跨域调用（见 main.go 中的路由注册）。
func (h *AdminHandler) ImportDoubanCookie(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 token 参数"})
		return
	}

	// 先在锁内取出并标记，避免长时间持锁
	doubanImportTokensMu.Lock()
	cleanupExpiredDoubanImportTokens()
	item, ok := doubanImportTokens[token]
	if !ok {
		doubanImportTokensMu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "token 无效或已过期，请回到后台重新生成"})
		return
	}
	if item.Consumed {
		doubanImportTokensMu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "此导入链接已被使用，请重新生成"})
		return
	}
	doubanImportTokensMu.Unlock()

	var req ImportDoubanCookieRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.markDoubanImportResult(token, false, "", "请求体解析失败: "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 Cookie 字段"})
		return
	}

	cookie := strings.TrimSpace(req.Cookie)
	if len(cookie) < 20 || len(cookie) > 8192 {
		h.markDoubanImportResult(token, false, "", "Cookie 长度异常")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cookie 长度异常，请确认已在豆瓣登录"})
		return
	}
	if !strings.Contains(cookie, "dbcl2=") {
		h.markDoubanImportResult(token, false, "", "Cookie 中缺少登录凭证 dbcl2（可能被 HttpOnly 保护，JS 读不到）")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cookie 中缺少豆瓣登录凭证 dbcl2（浏览器已将其标记为 HttpOnly，JS / Bookmarklet 无法读取）。请改用方式 3：安装 Cookie-Editor / EditThisCookie 扩展导出完整 Cookie"})
		return
	}

	// 保存配置
	if err := h.cfg.SetDoubanCookie(cookie); err != nil {
		h.logger.Errorf("保存豆瓣 Cookie 失败: %v", err)
		h.markDoubanImportResult(token, false, "", "保存配置失败: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	// 异步校验 Cookie 是否真的有效 + 获取用户名（不阻塞返回）
	valid, username, verr := h.metadataService.ValidateDoubanCookie()
	if verr != nil {
		h.logger.Warnf("懒人版导入后校验豆瓣 Cookie 异常: %v", verr)
	}

	msg := "Cookie 已导入"
	if valid && username != "" {
		msg = "已导入并校验通过，豆瓣账号：" + username
	} else if valid {
		msg = "Cookie 已导入并校验通过"
	} else {
		msg = "Cookie 已导入，但校验未通过，可能 Cookie 不完整"
	}

	h.markDoubanImportResult(token, true, username, msg)
	h.logger.Infof("豆瓣 Cookie 懒人版导入成功: user=%s ip=%s", username, c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"success":  true,
			"username": username,
			"message":  msg,
		},
	})
}

// markDoubanImportResult 标记 token 导入结果
func (h *AdminHandler) markDoubanImportResult(token string, success bool, username, message string) {
	doubanImportTokensMu.Lock()
	defer doubanImportTokensMu.Unlock()
	if item, ok := doubanImportTokens[token]; ok {
		item.Consumed = true
		item.Success = success
		item.Username = username
		item.Message = message
		item.UpdatedAt = time.Now()
	}
}

// buildDoubanBookmarklet 生成 javascript:... 形式的书签链接
func buildDoubanBookmarklet(importURL string) string {
	// 注意：Bookmarklet 里不能带换行，且需要 URL 编码特殊字符
	inner := buildDoubanImportScript(importURL)
	// 最小化：压缩空白
	inner = strings.Join(strings.Fields(inner), " ")
	return "javascript:(function(){" + inner + "})();void(0);"
}

// buildDoubanImportScript 生成用于在 douban.com 页面执行的 JS 脚本
// 【剪贴板中转方案】：不直接跨域请求后端（避免 http 后台 + https 豆瓣的混合内容问题），
// 而是把 document.cookie 写入剪贴板，由用户回到后台粘贴导入。
// 参数 importURL 保留以兼容签名，但新版脚本中并不使用。
func buildDoubanImportScript(importURL string) string {
	_ = importURL
	return `
if (!/douban\.com$/i.test(location.hostname) && !/\.douban\.com$/i.test(location.hostname)) {
	alert('请在 douban.com 已登录的页面执行此操作（当前不是豆瓣域名）');
	return;
}
var c = document.cookie || '';
if (c.length < 20) { alert('未检测到豆瓣 Cookie，请先登录豆瓣'); return; }
if (c.indexOf('dbcl2=') < 0) {
	alert('⚠️ 当前浏览器无法读取豆瓣登录凭证 dbcl2（通常被设为 HttpOnly，出于安全原因 JS 读不到）。\n\n此方式在你的浏览器中不可用，请改用【方式 3：Cookie 浏览器插件】：\n1. 安装扩展 Cookie-Editor 或 EditThisCookie\n2. 在豆瓣已登录页打开扩展\n3. 点 Export → Header String\n4. 回到管理后台使用【手动配置 Cookie】粘贴保存');
	return;
}
function done(){ alert('✅ 豆瓣 Cookie 已复制到剪贴板！\n\n请回到管理后台弹窗，点击【从剪贴板粘贴并导入】按钮即可完成登录。'); }
function fallback(){
	try {
		var ta = document.createElement('textarea');
		ta.value = c; ta.style.position='fixed'; ta.style.opacity='0';
		document.body.appendChild(ta); ta.select();
		var ok = document.execCommand('copy');
		document.body.removeChild(ta);
		if (ok) { done(); }
		else { prompt('自动复制失败，请手动复制以下 Cookie 并回到后台粘贴：', c); }
	} catch(e) {
		prompt('自动复制失败，请手动复制以下 Cookie 并回到后台粘贴：', c);
	}
}
if (navigator.clipboard && navigator.clipboard.writeText) {
	navigator.clipboard.writeText(c).then(done).catch(fallback);
} else {
	fallback();
}
`
}

// DoubanImportCORS 为豆瓣 Cookie 一键导入接口专属的 CORS 中间件
// 只允许来自 *.douban.com 的跨域调用（Bookmarklet 场景）
func DoubanImportCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			// 允许豆瓣主域及其子域跨域调用
			low := strings.ToLower(origin)
			if strings.HasPrefix(low, "https://www.douban.com") ||
				strings.HasPrefix(low, "https://douban.com") ||
				strings.HasPrefix(low, "http://www.douban.com") ||
				strings.HasPrefix(low, "http://douban.com") ||
				strings.HasSuffix(strings.TrimPrefix(strings.TrimPrefix(low, "https://"), "http://"), ".douban.com") {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
				c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				c.Writer.Header().Set("Access-Control-Max-Age", "300")
				c.Writer.Header().Set("Vary", "Origin")
			}
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
