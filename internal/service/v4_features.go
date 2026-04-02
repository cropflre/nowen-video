package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== P1: 批量移动媒体服务 ====================

// BatchMoveMediaRequest 批量移动媒体请求
type BatchMoveMediaRequest struct {
	MediaIDs      []string `json:"media_ids"`
	TargetLibrary string   `json:"target_library_id"`
}

// BatchMoveResult 批量移动结果
type BatchMoveResult struct {
	Total   int      `json:"total"`
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors"`
}

// BatchMoveMedia 批量移动媒体到目标媒体库
func (s *LibraryService) BatchMoveMedia(mediaIDs []string, targetLibraryID string) (*BatchMoveResult, error) {
	// 验证目标媒体库存在
	targetLib, err := s.repo.FindByID(targetLibraryID)
	if err != nil {
		return nil, fmt.Errorf("目标媒体库不存在")
	}

	result := &BatchMoveResult{Total: len(mediaIDs)}

	for _, mediaID := range mediaIDs {
		media, err := s.mediaRepo.FindByID(mediaID)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("媒体 %s 不存在", mediaID))
			continue
		}

		// 跳过已在目标媒体库的
		if media.LibraryID == targetLibraryID {
			result.Success++
			continue
		}

		// 更新媒体库ID
		media.LibraryID = targetLibraryID
		if err := s.mediaRepo.Update(media); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("移动 %s 失败: %v", media.Title, err))
			continue
		}

		result.Success++
	}

	s.logger.Infof("批量移动媒体到 %s: 成功 %d, 失败 %d", targetLib.Name, result.Success, result.Failed)

	// 广播事件
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent("media_batch_moved", map[string]interface{}{
			"target_library": targetLib.Name,
			"success":        result.Success,
			"failed":         result.Failed,
		})
	}

	return result, nil
}

// ==================== P2: 标签管理服务 ====================

// TagService 标签管理服务
type TagService struct {
	tagRepo      *repository.TagRepo
	mediaTagRepo *repository.MediaTagRepo
	logger       *zap.SugaredLogger
}

// NewTagService 创建标签服务
func NewTagService(tagRepo *repository.TagRepo, mediaTagRepo *repository.MediaTagRepo, logger *zap.SugaredLogger) *TagService {
	return &TagService{
		tagRepo:      tagRepo,
		mediaTagRepo: mediaTagRepo,
		logger:       logger,
	}
}

// CreateTag 创建标签
func (s *TagService) CreateTag(name, color, icon, category, createdBy string) (*model.Tag, error) {
	if name == "" {
		return nil, fmt.Errorf("标签名称不能为空")
	}
	// 检查重名
	if existing, _ := s.tagRepo.FindByName(name); existing != nil {
		return nil, fmt.Errorf("标签 '%s' 已存在", name)
	}
	if color == "" {
		color = "#3b82f6"
	}
	tag := &model.Tag{
		Name:      name,
		Color:     color,
		Icon:      icon,
		Category:  category,
		CreatedBy: createdBy,
	}
	if err := s.tagRepo.Create(tag); err != nil {
		return nil, err
	}
	s.logger.Infof("创建标签: %s (分类: %s)", name, category)
	return tag, nil
}

// ListTags 获取标签列表
func (s *TagService) ListTags(category string) ([]model.Tag, error) {
	return s.tagRepo.List(category)
}

// UpdateTag 更新标签
func (s *TagService) UpdateTag(id string, name, color, icon, category string) (*model.Tag, error) {
	tag, err := s.tagRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("标签不存在")
	}
	if name != "" {
		// 检查重名（排除自身）
		if existing, _ := s.tagRepo.FindByName(name); existing != nil && existing.ID != id {
			return nil, fmt.Errorf("标签 '%s' 已存在", name)
		}
		tag.Name = name
	}
	if color != "" {
		tag.Color = color
	}
	if icon != "" {
		tag.Icon = icon
	}
	tag.Category = category
	if err := s.tagRepo.Update(tag); err != nil {
		return nil, err
	}
	return tag, nil
}

// DeleteTag 删除标签
func (s *TagService) DeleteTag(id string) error {
	return s.tagRepo.Delete(id)
}

// AddTagToMedia 给媒体添加标签
func (s *TagService) AddTagToMedia(mediaID, tagID, userID string) error {
	if s.mediaTagRepo.Exists(mediaID, tagID) {
		return nil // 已存在，幂等
	}
	mt := &model.MediaTag{
		MediaID:   mediaID,
		TagID:     tagID,
		CreatedBy: userID,
	}
	if err := s.mediaTagRepo.Create(mt); err != nil {
		return err
	}
	s.tagRepo.IncrementUsage(tagID)
	return nil
}

// RemoveTagFromMedia 移除媒体标签
func (s *TagService) RemoveTagFromMedia(mediaID, tagID string) error {
	if err := s.mediaTagRepo.Delete(mediaID, tagID); err != nil {
		return err
	}
	s.tagRepo.DecrementUsage(tagID)
	return nil
}

// GetMediaTags 获取媒体的所有标签
func (s *TagService) GetMediaTags(mediaID string) ([]model.Tag, error) {
	mediaTags, err := s.mediaTagRepo.ListByMediaID(mediaID)
	if err != nil {
		return nil, err
	}
	var tags []model.Tag
	for _, mt := range mediaTags {
		tags = append(tags, mt.Tag)
	}
	return tags, nil
}

// BatchAddTags 批量给媒体添加标签
func (s *TagService) BatchAddTags(mediaIDs []string, tagIDs []string, userID string) (int, error) {
	count := 0
	for _, mediaID := range mediaIDs {
		for _, tagID := range tagIDs {
			if !s.mediaTagRepo.Exists(mediaID, tagID) {
				mt := &model.MediaTag{
					MediaID:   mediaID,
					TagID:     tagID,
					CreatedBy: userID,
				}
				if err := s.mediaTagRepo.Create(mt); err == nil {
					s.tagRepo.IncrementUsage(tagID)
					count++
				}
			}
		}
	}
	return count, nil
}

// ListCategories 获取所有标签分类
func (s *TagService) ListCategories() ([]string, error) {
	return s.tagRepo.ListCategories()
}

// ==================== P2: 分享链接服务 ====================

// ShareLinkService 分享链接服务
type ShareLinkService struct {
	repo       *repository.ShareLinkRepo
	mediaRepo  *repository.MediaRepo
	seriesRepo *repository.SeriesRepo
	logger     *zap.SugaredLogger
}

// NewShareLinkService 创建分享链接服务
func NewShareLinkService(repo *repository.ShareLinkRepo, mediaRepo *repository.MediaRepo, seriesRepo *repository.SeriesRepo, logger *zap.SugaredLogger) *ShareLinkService {
	return &ShareLinkService{
		repo:       repo,
		mediaRepo:  mediaRepo,
		seriesRepo: seriesRepo,
		logger:     logger,
	}
}

// generateCode 生成随机短链接码
func generateCode(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

// CreateShareLink 创建分享链接
func (s *ShareLinkService) CreateShareLink(userID, mediaID, seriesID, title, description, password string, maxViews int, allowDownload bool, expiresIn int) (*model.ShareLink, error) {
	if mediaID == "" && seriesID == "" {
		return nil, fmt.Errorf("必须指定媒体或剧集")
	}

	// 验证媒体/剧集存在
	if mediaID != "" {
		if _, err := s.mediaRepo.FindByID(mediaID); err != nil {
			return nil, fmt.Errorf("媒体不存在")
		}
	}
	if seriesID != "" {
		if _, err := s.seriesRepo.FindByID(seriesID); err != nil {
			return nil, fmt.Errorf("剧集不存在")
		}
	}

	code := generateCode(8)

	var expiresAt *time.Time
	if expiresIn > 0 {
		t := time.Now().Add(time.Duration(expiresIn) * time.Hour)
		expiresAt = &t
	}

	link := &model.ShareLink{
		Code:          code,
		MediaID:       mediaID,
		SeriesID:      seriesID,
		CreatedBy:     userID,
		Title:         title,
		Description:   description,
		Password:      password,
		MaxViews:      maxViews,
		AllowDownload: allowDownload,
		ExpiresAt:     expiresAt,
		IsActive:      true,
	}

	if err := s.repo.Create(link); err != nil {
		return nil, err
	}

	s.logger.Infof("创建分享链接: %s (媒体=%s, 剧集=%s)", code, mediaID, seriesID)
	return link, nil
}

// GetShareByCode 通过短链接码获取分享内容
func (s *ShareLinkService) GetShareByCode(code, password string) (*model.ShareLink, error) {
	link, err := s.repo.FindByCode(code)
	if err != nil {
		return nil, fmt.Errorf("分享链接不存在")
	}

	if !link.IsActive {
		return nil, fmt.Errorf("分享链接已禁用")
	}

	// 检查过期
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return nil, fmt.Errorf("分享链接已过期")
	}

	// 检查访问次数
	if link.MaxViews > 0 && link.ViewCount >= link.MaxViews {
		return nil, fmt.Errorf("分享链接已达到最大访问次数")
	}

	// 检查密码
	if link.Password != "" && link.Password != password {
		return nil, fmt.Errorf("密码错误")
	}

	// 增加访问计数
	s.repo.IncrementViewCount(link.ID)

	return link, nil
}

// ListUserShares 获取用户的分享列表
func (s *ShareLinkService) ListUserShares(userID string, page, size int) ([]model.ShareLink, int64, error) {
	return s.repo.ListByUser(userID, page, size)
}

// DeleteShare 删除分享链接
func (s *ShareLinkService) DeleteShare(id, userID string) error {
	link, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("分享链接不存在")
	}
	if link.CreatedBy != userID {
		return fmt.Errorf("无权删除此分享链接")
	}
	return s.repo.Delete(id)
}

// ToggleShare 启用/禁用分享链接
func (s *ShareLinkService) ToggleShare(id, userID string) (*model.ShareLink, error) {
	link, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("分享链接不存在")
	}
	if link.CreatedBy != userID {
		return nil, fmt.Errorf("无权操作此分享链接")
	}
	link.IsActive = !link.IsActive
	if err := s.repo.Update(link); err != nil {
		return nil, err
	}
	return link, nil
}

// ==================== P3: 自定义匹配规则服务 ====================

// MatchRuleService 自定义匹配规则服务
type MatchRuleService struct {
	repo   *repository.MatchRuleRepo
	logger *zap.SugaredLogger
}

// NewMatchRuleService 创建匹配规则服务
func NewMatchRuleService(repo *repository.MatchRuleRepo, logger *zap.SugaredLogger) *MatchRuleService {
	return &MatchRuleService{
		repo:   repo,
		logger: logger,
	}
}

// CreateRule 创建匹配规则
func (s *MatchRuleService) CreateRule(name, description, ruleType, pattern, action, actionValue, libraryID, createdBy string, priority int) (*model.MatchRule, error) {
	if name == "" || pattern == "" || action == "" {
		return nil, fmt.Errorf("名称、匹配模式和动作不能为空")
	}

	// 验证规则类型
	validTypes := map[string]bool{"filename": true, "path": true, "regex": true, "keyword": true}
	if !validTypes[ruleType] {
		return nil, fmt.Errorf("无效的规则类型: %s", ruleType)
	}

	// 验证动作类型
	validActions := map[string]bool{"set_type": true, "set_genre": true, "set_tag": true, "skip": true, "set_library": true}
	if !validActions[action] {
		return nil, fmt.Errorf("无效的动作类型: %s", action)
	}

	// 如果是正则类型，验证正则表达式合法性
	if ruleType == "regex" {
		if _, err := regexp.Compile(pattern); err != nil {
			return nil, fmt.Errorf("无效的正则表达式: %v", err)
		}
	}

	rule := &model.MatchRule{
		Name:        name,
		Description: description,
		RuleType:    ruleType,
		Pattern:     pattern,
		Action:      action,
		ActionValue: actionValue,
		Priority:    priority,
		Enabled:     true,
		LibraryID:   libraryID,
		CreatedBy:   createdBy,
	}

	if err := s.repo.Create(rule); err != nil {
		return nil, err
	}

	s.logger.Infof("创建匹配规则: %s (类型=%s, 模式=%s)", name, ruleType, pattern)
	return rule, nil
}

// ListRules 获取匹配规则列表
func (s *MatchRuleService) ListRules(libraryID string) ([]model.MatchRule, error) {
	return s.repo.List(libraryID)
}

// UpdateRule 更新匹配规则
func (s *MatchRuleService) UpdateRule(id string, updates map[string]interface{}) (*model.MatchRule, error) {
	rule, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("规则不存在")
	}

	if name, ok := updates["name"].(string); ok && name != "" {
		rule.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		rule.Description = desc
	}
	if ruleType, ok := updates["rule_type"].(string); ok && ruleType != "" {
		rule.RuleType = ruleType
	}
	if pattern, ok := updates["pattern"].(string); ok && pattern != "" {
		if rule.RuleType == "regex" {
			if _, err := regexp.Compile(pattern); err != nil {
				return nil, fmt.Errorf("无效的正则表达式: %v", err)
			}
		}
		rule.Pattern = pattern
	}
	if action, ok := updates["action"].(string); ok && action != "" {
		rule.Action = action
	}
	if actionValue, ok := updates["action_value"].(string); ok {
		rule.ActionValue = actionValue
	}
	if priority, ok := updates["priority"].(float64); ok {
		rule.Priority = int(priority)
	}
	if enabled, ok := updates["enabled"].(bool); ok {
		rule.Enabled = enabled
	}
	if libraryID, ok := updates["library_id"].(string); ok {
		rule.LibraryID = libraryID
	}

	if err := s.repo.Update(rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// DeleteRule 删除匹配规则
func (s *MatchRuleService) DeleteRule(id string) error {
	return s.repo.Delete(id)
}

// TestRule 测试匹配规则（给定文件名，返回是否匹配）
func (s *MatchRuleService) TestRule(ruleType, pattern, testInput string) (bool, error) {
	switch ruleType {
	case "filename":
		return strings.Contains(strings.ToLower(testInput), strings.ToLower(pattern)), nil
	case "path":
		return strings.Contains(strings.ToLower(testInput), strings.ToLower(pattern)), nil
	case "regex":
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false, fmt.Errorf("无效的正则表达式: %v", err)
		}
		return re.MatchString(testInput), nil
	case "keyword":
		keywords := strings.Split(pattern, ",")
		for _, kw := range keywords {
			kw = strings.TrimSpace(kw)
			if kw != "" && strings.Contains(strings.ToLower(testInput), strings.ToLower(kw)) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("未知规则类型: %s", ruleType)
	}
}
