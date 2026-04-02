package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// ==================== P1: 批量移动媒体 Handler（挂在 LibraryHandler 上） ====================

// BatchMoveMedia 批量移动媒体到目标媒体库
func (h *LibraryHandler) BatchMoveMedia(c *gin.Context) {
	var req service.BatchMoveMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if len(req.MediaIDs) == 0 || req.TargetLibrary == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "media_ids 和 target_library_id 不能为空"})
		return
	}
	result, err := h.libService.BatchMoveMedia(req.MediaIDs, req.TargetLibrary)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ==================== P2: 标签管理 Handler ====================

type TagHandler struct {
	tagService *service.TagService
	logger     *zap.SugaredLogger
}

// CreateTag 创建标签
func (h *TagHandler) CreateTag(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		Color    string `json:"color"`
		Icon     string `json:"icon"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	userID, _ := c.Get("user_id")
	tag, err := h.tagService.CreateTag(req.Name, req.Color, req.Icon, req.Category, userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tag})
}

// ListTags 获取标签列表
func (h *TagHandler) ListTags(c *gin.Context) {
	category := c.Query("category")
	tags, err := h.tagService.ListTags(category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tags})
}

// UpdateTag 更新标签
func (h *TagHandler) UpdateTag(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name     string `json:"name"`
		Color    string `json:"color"`
		Icon     string `json:"icon"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	tag, err := h.tagService.UpdateTag(id, req.Name, req.Color, req.Icon, req.Category)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tag})
}

// DeleteTag 删除标签
func (h *TagHandler) DeleteTag(c *gin.Context) {
	id := c.Param("id")
	if err := h.tagService.DeleteTag(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "标签已删除"})
}

// ListCategories 获取标签分类列表
func (h *TagHandler) ListCategories(c *gin.Context) {
	categories, err := h.tagService.ListCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": categories})
}

// AddTagToMedia 给媒体添加标签
func (h *TagHandler) AddTagToMedia(c *gin.Context) {
	var req struct {
		MediaID string `json:"media_id"`
		TagID   string `json:"tag_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	userID, _ := c.Get("user_id")
	if err := h.tagService.AddTagToMedia(req.MediaID, req.TagID, userID.(string)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "标签已添加"})
}

// RemoveTagFromMedia 移除媒体标签
func (h *TagHandler) RemoveTagFromMedia(c *gin.Context) {
	mediaID := c.Param("media_id")
	tagID := c.Param("tag_id")
	if err := h.tagService.RemoveTagFromMedia(mediaID, tagID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "标签已移除"})
}

// GetMediaTags 获取媒体的所有标签
func (h *TagHandler) GetMediaTags(c *gin.Context) {
	mediaID := c.Param("media_id")
	tags, err := h.tagService.GetMediaTags(mediaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tags})
}

// BatchAddTags 批量给媒体添加标签
func (h *TagHandler) BatchAddTags(c *gin.Context) {
	var req struct {
		MediaIDs []string `json:"media_ids"`
		TagIDs   []string `json:"tag_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	userID, _ := c.Get("user_id")
	count, err := h.tagService.BatchAddTags(req.MediaIDs, req.TagIDs, userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"added": count}})
}

// ==================== P2: 分享链接 Handler ====================

type ShareLinkHandler struct {
	shareService  *service.ShareLinkService
	mediaService  *service.MediaService
	seriesService *service.SeriesService
	logger        *zap.SugaredLogger
}

// CreateShareLink 创建分享链接
func (h *ShareLinkHandler) CreateShareLink(c *gin.Context) {
	var req struct {
		MediaID       string `json:"media_id"`
		SeriesID      string `json:"series_id"`
		Title         string `json:"title"`
		Description   string `json:"description"`
		Password      string `json:"password"`
		MaxViews      int    `json:"max_views"`
		AllowDownload bool   `json:"allow_download"`
		ExpiresIn     int    `json:"expires_in"` // 过期时间（小时）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	userID, _ := c.Get("user_id")
	link, err := h.shareService.CreateShareLink(
		userID.(string), req.MediaID, req.SeriesID,
		req.Title, req.Description, req.Password,
		req.MaxViews, req.AllowDownload, req.ExpiresIn,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": link})
}

// GetShareByCode 通过短链接码获取分享内容（公开接口）
func (h *ShareLinkHandler) GetShareByCode(c *gin.Context) {
	code := c.Param("code")
	password := c.Query("password")
	link, err := h.shareService.GetShareByCode(code, password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取关联的媒体/剧集信息
	result := gin.H{"share": link}
	if link.MediaID != "" {
		media, err := h.mediaService.GetDetail(link.MediaID, "")
		if err == nil {
			result["media"] = media
		}
	}
	if link.SeriesID != "" {
		series, err := h.seriesService.GetDetail(link.SeriesID)
		if err == nil {
			result["series"] = series
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ListUserShares 获取用户的分享列表
func (h *ShareLinkHandler) ListUserShares(c *gin.Context) {
	userID, _ := c.Get("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	links, total, err := h.shareService.ListUserShares(userID.(string), page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": links, "total": total, "page": page, "size": size})
}

// DeleteShare 删除分享链接
func (h *ShareLinkHandler) DeleteShare(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")
	if err := h.shareService.DeleteShare(id, userID.(string)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "分享链接已删除"})
}

// ToggleShare 启用/禁用分享链接
func (h *ShareLinkHandler) ToggleShare(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")
	link, err := h.shareService.ToggleShare(id, userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": link})
}

// ==================== P3: 自定义匹配规则 Handler ====================

type MatchRuleHandler struct {
	ruleService *service.MatchRuleService
	logger      *zap.SugaredLogger
}

// CreateRule 创建匹配规则
func (h *MatchRuleHandler) CreateRule(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		RuleType    string `json:"rule_type"`
		Pattern     string `json:"pattern"`
		Action      string `json:"action"`
		ActionValue string `json:"action_value"`
		LibraryID   string `json:"library_id"`
		Priority    int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	userID, _ := c.Get("user_id")
	rule, err := h.ruleService.CreateRule(
		req.Name, req.Description, req.RuleType, req.Pattern,
		req.Action, req.ActionValue, req.LibraryID, userID.(string), req.Priority,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rule})
}

// ListRules 获取匹配规则列表
func (h *MatchRuleHandler) ListRules(c *gin.Context) {
	libraryID := c.Query("library_id")
	rules, err := h.ruleService.ListRules(libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rules})
}

// UpdateRule 更新匹配规则
func (h *MatchRuleHandler) UpdateRule(c *gin.Context) {
	id := c.Param("id")
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	rule, err := h.ruleService.UpdateRule(id, updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rule})
}

// DeleteRule 删除匹配规则
func (h *MatchRuleHandler) DeleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.ruleService.DeleteRule(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "规则已删除"})
}

// TestRule 测试匹配规则
func (h *MatchRuleHandler) TestRule(c *gin.Context) {
	var req struct {
		RuleType  string `json:"rule_type"`
		Pattern   string `json:"pattern"`
		TestInput string `json:"test_input"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	matched, err := h.ruleService.TestRule(req.RuleType, req.Pattern, req.TestInput)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"matched": matched}})
}
