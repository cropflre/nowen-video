package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== AI 助手服务 ====================

// AIAssistantService AI助手服务
type AIAssistantService struct {
	ai          *AIService
	fileManager *FileManagerService
	mediaRepo   *repository.MediaRepo
	seriesRepo  *repository.SeriesRepo
	wsHub       *WSHub
	logger      *zap.SugaredLogger

	// 会话管理
	sessions   map[string]*ChatSession
	sessionsMu sync.RWMutex

	// 操作历史（用于撤销）
	opHistory   []AssistantOperation
	opHistoryMu sync.Mutex
}

// ChatSession 对话会话
type ChatSession struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Messages  []ChatMsg      `json:"messages"`
	Context   SessionContext `json:"context"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// ChatMsg 对话消息
type ChatMsg struct {
	Role      string    `json:"role"` // user / assistant / system
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	// 助手消息附带的操作建议
	Actions []SuggestedAction `json:"actions,omitempty"`
	// 助手消息附带的预览数据
	Previews []OperationPreview `json:"previews,omitempty"`
}

// SessionContext 会话上下文
type SessionContext struct {
	SelectedMediaIDs []string `json:"selected_media_ids,omitempty"` // 当前选中的文件
	LibraryID        string   `json:"library_id,omitempty"`         // 当前媒体库
	LastIntent       *Intent  `json:"last_intent,omitempty"`        // 上一次解析的意图
}

// Intent AI解析的用户意图
type Intent struct {
	Action     string            `json:"action"`     // rename / scrape / delete / classify / tag / analyze / fix / move
	SubAction  string            `json:"sub_action"` // 子操作类型
	Targets    string            `json:"targets"`    // all / selected / filtered
	Params     map[string]string `json:"params"`     // 操作参数
	Confidence float64           `json:"confidence"` // 置信度 0-1
	Reasoning  string            `json:"reasoning"`  // AI推理过程
}

// SuggestedAction 建议的操作
type SuggestedAction struct {
	ID          string `json:"id"`
	Label       string `json:"label"`       // 按钮文字
	Description string `json:"description"` // 操作描述
	Action      string `json:"action"`      // 操作类型
	Params      string `json:"params"`      // JSON参数
	Dangerous   bool   `json:"dangerous"`   // 是否危险操作
}

// OperationPreview 操作预览
type OperationPreview struct {
	MediaID    string `json:"media_id"`
	Title      string `json:"title"`
	OldValue   string `json:"old_value"`
	NewValue   string `json:"new_value"`
	ChangeType string `json:"change_type"` // rename / tag / metadata / move
}

// AssistantOperation 助手执行的操作（用于撤销）
type AssistantOperation struct {
	ID         string             `json:"id"`
	SessionID  string             `json:"session_id"`
	Action     string             `json:"action"`
	Previews   []OperationPreview `json:"previews"`
	ExecutedAt time.Time          `json:"executed_at"`
	UserID     string             `json:"user_id"`
	Undone     bool               `json:"undone"`
}

// ChatRequest 对话请求
type ChatRequest struct {
	SessionID string   `json:"session_id"` // 空则创建新会话
	Message   string   `json:"message"`
	MediaIDs  []string `json:"media_ids,omitempty"`  // 当前选中的文件
	LibraryID string   `json:"library_id,omitempty"` // 当前媒体库
}

// ChatResponse 对话响应
type ChatResponse struct {
	SessionID string  `json:"session_id"`
	Message   ChatMsg `json:"message"`
	Intent    *Intent `json:"intent,omitempty"`
}

// ExecuteRequest 执行操作请求
type ExecuteRequest struct {
	SessionID string `json:"session_id"`
	ActionID  string `json:"action_id"`
}

// ExecuteResponse 执行操作响应
type ExecuteResponse struct {
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Results []OperationPreview `json:"results,omitempty"`
	Errors  []string           `json:"errors,omitempty"`
	OpID    string             `json:"op_id"` // 操作ID，用于撤销
}

// NewAIAssistantService 创建AI助手服务
func NewAIAssistantService(
	ai *AIService,
	fileManager *FileManagerService,
	mediaRepo *repository.MediaRepo,
	seriesRepo *repository.SeriesRepo,
	logger *zap.SugaredLogger,
) *AIAssistantService {
	return &AIAssistantService{
		ai:          ai,
		fileManager: fileManager,
		mediaRepo:   mediaRepo,
		seriesRepo:  seriesRepo,
		logger:      logger,
		sessions:    make(map[string]*ChatSession),
	}
}

// SetWSHub 设置WebSocket Hub
func (s *AIAssistantService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// ==================== 对话管理 ====================

// Chat 处理用户对话
func (s *AIAssistantService) Chat(req ChatRequest, userID string) (*ChatResponse, error) {
	if s.ai == nil || !s.ai.IsEnabled() {
		return nil, fmt.Errorf("AI服务未启用，请先在系统管理中配置AI服务")
	}

	// 获取或创建会话
	session := s.getOrCreateSession(req.SessionID, userID)

	// 更新会话上下文
	if len(req.MediaIDs) > 0 {
		session.Context.SelectedMediaIDs = req.MediaIDs
	}
	if req.LibraryID != "" {
		session.Context.LibraryID = req.LibraryID
	}

	// 添加用户消息
	userMsg := ChatMsg{
		Role:      "user",
		Content:   req.Message,
		Timestamp: time.Now(),
	}
	session.Messages = append(session.Messages, userMsg)

	// 构建上下文信息
	contextInfo := s.buildContextInfo(session)

	// 调用AI解析意图并生成回复
	systemPrompt := s.buildSystemPrompt()
	userPrompt := s.buildUserPrompt(req.Message, contextInfo, session)

	response, err := s.ai.ChatCompletion(systemPrompt, userPrompt, 0.3, 2000)
	if err != nil {
		return nil, fmt.Errorf("AI处理失败: %w", err)
	}

	// 解析AI响应
	assistantMsg, intent := s.parseAIResponse(response, session)

	// 保存意图到上下文
	if intent != nil {
		session.Context.LastIntent = intent
	}

	// 添加助手消息
	session.Messages = append(session.Messages, assistantMsg)
	session.UpdatedAt = time.Now()

	return &ChatResponse{
		SessionID: session.ID,
		Message:   assistantMsg,
		Intent:    intent,
	}, nil
}

// getOrCreateSession 获取或创建会话
func (s *AIAssistantService) getOrCreateSession(sessionID, userID string) *ChatSession {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	if sessionID != "" {
		if session, ok := s.sessions[sessionID]; ok {
			return session
		}
	}

	// 创建新会话
	session := &ChatSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		Messages:  []ChatMsg{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.sessions[session.ID] = session

	// 清理过期会话（保留最近50个）
	if len(s.sessions) > 50 {
		s.cleanOldSessions()
	}

	return session
}

// cleanOldSessions 清理过期会话
func (s *AIAssistantService) cleanOldSessions() {
	cutoff := time.Now().Add(-24 * time.Hour)
	for id, session := range s.sessions {
		if session.UpdatedAt.Before(cutoff) {
			delete(s.sessions, id)
		}
	}
}

// buildSystemPrompt 构建系统提示词
func (s *AIAssistantService) buildSystemPrompt() string {
	return `你是一个影视文件管理AI助手，专门帮助用户管理影视文件库。你的能力包括：

## 核心能力
1. **批量重命名**：根据用户指令智能重命名文件（支持电视剧、电影、纪录片等模式）
2. **批量刮削**：为文件获取元数据（从TMDb、Bangumi等数据源）
3. **文件分类**：按类型、年份、评分等属性自动分组整理
4. **元数据编辑**：批量修改标签、描述、评分等信息
5. **重复检测**：识别重复文件并提供处理建议
6. **智能纠错**：检测文件名不一致问题并修复
7. **文件分析**：分析文件库状态，提供优化建议
8. **误分类检测**：检测被错误标记为电影的剧集文件，基于文件名模式（S01E01、第X集等）、目录结构（季目录）、同目录文件数量等特征进行智能识别
9. **批量重分类**：将误分类的文件批量修正为正确的类型，并自动关联到剧集合集

## 响应格式
你必须严格按照以下JSON格式响应（不要输出其他内容）：
{
  "message": "你的回复文本（使用Markdown格式，友好自然的语气）",
  "intent": {
    "action": "操作类型（rename/scrape/delete/classify/tag/analyze/fix/suggest/analyze_misclassification/reclassify）",
    "sub_action": "子操作类型",
    "targets": "操作目标（all/selected/filtered）",
    "params": {"key": "value"},
    "confidence": 0.95,
    "reasoning": "推理过程"
  },
  "actions": [
    {
      "id": "唯一ID",
      "label": "按钮文字",
      "description": "操作描述",
      "action": "操作类型",
      "params": "JSON参数字符串",
      "dangerous": false
    }
  ],
  "previews": [
    {
      "media_id": "文件ID",
      "title": "文件标题",
      "old_value": "旧值",
      "new_value": "新值",
      "change_type": "变更类型"
    }
  ]
}

## 重要规则
- 对于危险操作（删除、批量修改），必须先展示预览并要求确认
- 如果用户的指令不够明确，主动询问细节
- 根据上下文推断用户意图，提供智能建议
- 如果没有选中文件但用户要求操作，提醒用户先选择文件
- previews数组最多展示10条预览，如果超过则在message中说明总数
- 如果无法确定操作意图，将action设为"suggest"并提供建议`
}

// buildUserPrompt 构建用户提示词
func (s *AIAssistantService) buildUserPrompt(message, contextInfo string, session *ChatSession) string {
	// 构建对话历史（最近5轮）
	historyStr := ""
	startIdx := 0
	if len(session.Messages) > 10 {
		startIdx = len(session.Messages) - 10
	}
	for i := startIdx; i < len(session.Messages); i++ {
		msg := session.Messages[i]
		historyStr += fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content)
	}

	return fmt.Sprintf(`## 当前上下文
%s

## 对话历史
%s

## 用户当前指令
%s

请分析用户意图并按照指定JSON格式响应。`, contextInfo, historyStr, message)
}

// buildContextInfo 构建上下文信息
func (s *AIAssistantService) buildContextInfo(session *ChatSession) string {
	var parts []string

	// 选中的文件信息
	if len(session.Context.SelectedMediaIDs) > 0 {
		parts = append(parts, fmt.Sprintf("已选中 %d 个文件", len(session.Context.SelectedMediaIDs)))

		// 获取前5个文件的详细信息
		limit := 5
		if len(session.Context.SelectedMediaIDs) < limit {
			limit = len(session.Context.SelectedMediaIDs)
		}
		var fileInfos []string
		for i := 0; i < limit; i++ {
			media, err := s.mediaRepo.FindByID(session.Context.SelectedMediaIDs[i])
			if err == nil {
				info := fmt.Sprintf("- ID:%s 标题:'%s' 原标题:'%s' 年份:%d 类型:%s 分辨率:%s 评分:%.1f",
					media.ID, media.Title, media.OrigTitle, media.Year, media.MediaType, media.Resolution, media.Rating)
				if media.SeriesID != "" {
					info += fmt.Sprintf(" 季:%d 集:%d", media.SeasonNum, media.EpisodeNum)
				}
				if media.Genres != "" {
					info += fmt.Sprintf(" 类型:%s", media.Genres)
				}
				fileInfos = append(fileInfos, info)
			}
		}
		if len(fileInfos) > 0 {
			parts = append(parts, "选中文件详情:\n"+strings.Join(fileInfos, "\n"))
		}
		if len(session.Context.SelectedMediaIDs) > limit {
			parts = append(parts, fmt.Sprintf("（还有 %d 个文件未展示）", len(session.Context.SelectedMediaIDs)-limit))
		}
	} else {
		parts = append(parts, "当前未选中任何文件")
	}

	// 媒体库信息
	if session.Context.LibraryID != "" {
		parts = append(parts, fmt.Sprintf("当前媒体库ID: %s", session.Context.LibraryID))
	}

	// 上一次意图
	if session.Context.LastIntent != nil {
		parts = append(parts, fmt.Sprintf("上一次操作意图: %s (%s)", session.Context.LastIntent.Action, session.Context.LastIntent.Reasoning))
	}

	return strings.Join(parts, "\n")
}

// parseAIResponse 解析AI响应
func (s *AIAssistantService) parseAIResponse(response string, session *ChatSession) (ChatMsg, *Intent) {
	// 尝试解析JSON
	var parsed struct {
		Message  string             `json:"message"`
		Intent   *Intent            `json:"intent"`
		Actions  []SuggestedAction  `json:"actions"`
		Previews []OperationPreview `json:"previews"`
	}

	// 清理可能的markdown代码块包裹
	cleanResponse := response
	cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
	cleanResponse = strings.TrimPrefix(cleanResponse, "```")
	cleanResponse = strings.TrimSuffix(cleanResponse, "```")
	cleanResponse = strings.TrimSpace(cleanResponse)

	msg := ChatMsg{
		Role:      "assistant",
		Timestamp: time.Now(),
	}

	if err := json.Unmarshal([]byte(cleanResponse), &parsed); err != nil {
		// JSON解析失败，直接使用原始文本
		s.logger.Debugf("AI响应JSON解析失败，使用原始文本: %v", err)
		msg.Content = response
		return msg, nil
	}

	msg.Content = parsed.Message
	msg.Actions = parsed.Actions
	msg.Previews = parsed.Previews

	// 如果AI没有生成预览但意图是rename且有选中文件，自动生成预览
	if parsed.Intent != nil && parsed.Intent.Action == "rename" && len(msg.Previews) == 0 && len(session.Context.SelectedMediaIDs) > 0 {
		previews := s.generateRenamePreviews(session, parsed.Intent)
		msg.Previews = previews
	}

	// 如果意图是误分类检测，自动添加操作按钮
	if parsed.Intent != nil && parsed.Intent.Action == "analyze_misclassification" {
		if len(msg.Actions) == 0 {
			msg.Actions = []SuggestedAction{
				{
					ID:          uuid.New().String(),
					Label:       "🔍 开始分析",
					Description: "扫描文件库，检测被误标记为电影的剧集文件",
					Action:      "analyze_misclassification",
					Dangerous:   false,
				},
			}
		}
	}

	// 如果意图是重分类，自动添加操作按钮
	if parsed.Intent != nil && parsed.Intent.Action == "reclassify" {
		if len(msg.Actions) == 0 {
			msg.Actions = []SuggestedAction{
				{
					ID:          uuid.New().String(),
					Label:       "🔴 重分类高置信度文件",
					Description: "将置信度>80%的误分类文件重新分类为剧集",
					Action:      "reclassify_high_confidence",
					Dangerous:   false,
				},
				{
					ID:          uuid.New().String(),
					Label:       "⚠️ 重分类所有疑似文件",
					Description: "将所有疑似误分类的文件重新分类为剧集（包含低置信度）",
					Action:      "reclassify_all",
					Dangerous:   true,
				},
			}
		}
	}

	return msg, parsed.Intent
}

// generateRenamePreviews 生成重命名预览
func (s *AIAssistantService) generateRenamePreviews(session *ChatSession, intent *Intent) []OperationPreview {
	var previews []OperationPreview
	limit := 10
	if len(session.Context.SelectedMediaIDs) < limit {
		limit = len(session.Context.SelectedMediaIDs)
	}

	template := "{title} ({year}) [{resolution}]"
	if t, ok := intent.Params["template"]; ok && t != "" {
		template = t
	}

	for i := 0; i < limit; i++ {
		media, err := s.mediaRepo.FindByID(session.Context.SelectedMediaIDs[i])
		if err != nil {
			continue
		}
		newTitle := s.fileManager.generateRenameTitle(media, template)
		if newTitle != media.Title {
			previews = append(previews, OperationPreview{
				MediaID:    media.ID,
				Title:      media.Title,
				OldValue:   media.Title,
				NewValue:   newTitle,
				ChangeType: "rename",
			})
		}
	}
	return previews
}

// ==================== 操作执行 ====================

// ExecuteAction 执行AI建议的操作
func (s *AIAssistantService) ExecuteAction(req ExecuteRequest, userID string) (*ExecuteResponse, error) {
	s.sessionsMu.RLock()
	session, ok := s.sessions[req.SessionID]
	s.sessionsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("会话不存在或已过期")
	}

	// 查找对应的操作
	var targetAction *SuggestedAction
	for _, msg := range session.Messages {
		for i := range msg.Actions {
			if msg.Actions[i].ID == req.ActionID {
				targetAction = &msg.Actions[i]
				break
			}
		}
	}
	if targetAction == nil {
		return nil, fmt.Errorf("操作不存在")
	}

	// 根据操作类型执行
	switch targetAction.Action {
	case "rename_template":
		return s.executeRenameTemplate(session, targetAction, userID)
	case "rename_ai":
		return s.executeRenameAI(session, targetAction, userID)
	case "scrape":
		return s.executeScrape(session, targetAction, userID)
	case "batch_tag":
		return s.executeBatchTag(session, targetAction, userID)
	case "analyze":
		return s.executeAnalyze(session, userID)
	case "analyze_misclassification":
		return s.executeAnalyzeMisclassification(session, userID)
	case "reclassify":
		return s.executeReclassify(session, targetAction, userID)
	case "reclassify_high_confidence":
		return s.executeReclassifyByConfidence(session, 0.8, userID)
	case "reclassify_all":
		return s.executeReclassifyByConfidence(session, 0.3, userID)
	default:
		return nil, fmt.Errorf("不支持的操作类型: %s", targetAction.Action)
	}
}

// executeRenameTemplate 执行模板重命名
func (s *AIAssistantService) executeRenameTemplate(session *ChatSession, action *SuggestedAction, userID string) (*ExecuteResponse, error) {
	if len(session.Context.SelectedMediaIDs) == 0 {
		return &ExecuteResponse{Success: false, Message: "没有选中的文件"}, nil
	}

	// 解析参数
	var params struct {
		Template string `json:"template"`
	}
	if action.Params != "" {
		json.Unmarshal([]byte(action.Params), &params)
	}
	if params.Template == "" {
		params.Template = "{title} ({year}) [{resolution}]"
	}

	// 执行重命名
	renamed, errors := s.fileManager.ExecuteRename(session.Context.SelectedMediaIDs, params.Template, userID)

	// 记录操作历史
	op := AssistantOperation{
		ID:         uuid.New().String(),
		SessionID:  session.ID,
		Action:     "rename_template",
		ExecutedAt: time.Now(),
		UserID:     userID,
	}
	s.addOperation(op)

	resp := &ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("成功重命名 %d 个文件", renamed),
		OpID:    op.ID,
	}
	if len(errors) > 0 {
		resp.Errors = errors
		resp.Message += fmt.Sprintf("，%d 个失败", len(errors))
	}

	return resp, nil
}

// executeRenameAI 执行AI智能重命名
func (s *AIAssistantService) executeRenameAI(session *ChatSession, action *SuggestedAction, userID string) (*ExecuteResponse, error) {
	if len(session.Context.SelectedMediaIDs) == 0 {
		return &ExecuteResponse{Success: false, Message: "没有选中的文件"}, nil
	}

	// 解析参数
	var params struct {
		TargetLang string `json:"target_lang"`
	}
	if action.Params != "" {
		json.Unmarshal([]byte(action.Params), &params)
	}

	// 先生成预览
	previews, err := s.fileManager.AIGenerateRenames(session.Context.SelectedMediaIDs, params.TargetLang)
	if err != nil {
		return &ExecuteResponse{Success: false, Message: err.Error()}, nil
	}

	// 执行重命名（使用AI生成的标题）
	successCount := 0
	var errors []string
	var opPreviews []OperationPreview

	for _, p := range previews {
		// 通过更新文件信息来重命名
		updates := map[string]interface{}{"title": p.NewTitle}
		_, err := s.fileManager.UpdateFileInfo(p.MediaID, updates, userID)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", p.OldTitle, err))
		} else {
			successCount++
			opPreviews = append(opPreviews, OperationPreview{
				MediaID:    p.MediaID,
				Title:      p.OldTitle,
				OldValue:   p.OldTitle,
				NewValue:   p.NewTitle,
				ChangeType: "rename",
			})
		}
	}

	// 记录操作
	op := AssistantOperation{
		ID:         uuid.New().String(),
		SessionID:  session.ID,
		Action:     "rename_ai",
		Previews:   opPreviews,
		ExecutedAt: time.Now(),
		UserID:     userID,
	}
	s.addOperation(op)

	resp := &ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("AI智能重命名完成，成功 %d 个", successCount),
		Results: opPreviews,
		OpID:    op.ID,
	}
	if len(errors) > 0 {
		resp.Errors = errors
	}

	return resp, nil
}

// executeScrape 执行批量刮削
func (s *AIAssistantService) executeScrape(session *ChatSession, action *SuggestedAction, userID string) (*ExecuteResponse, error) {
	if len(session.Context.SelectedMediaIDs) == 0 {
		return &ExecuteResponse{Success: false, Message: "没有选中的文件"}, nil
	}

	var params struct {
		Source string `json:"source"`
	}
	if action.Params != "" {
		json.Unmarshal([]byte(action.Params), &params)
	}

	started, errors := s.fileManager.BatchScrapeFiles(session.Context.SelectedMediaIDs, params.Source, userID)

	op := AssistantOperation{
		ID:         uuid.New().String(),
		SessionID:  session.ID,
		Action:     "scrape",
		ExecutedAt: time.Now(),
		UserID:     userID,
	}
	s.addOperation(op)

	resp := &ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("已启动 %d 个文件的刮削任务", started),
		OpID:    op.ID,
	}
	if len(errors) > 0 {
		resp.Errors = errors
	}

	return resp, nil
}

// executeBatchTag 执行批量标签
func (s *AIAssistantService) executeBatchTag(session *ChatSession, action *SuggestedAction, userID string) (*ExecuteResponse, error) {
	if len(session.Context.SelectedMediaIDs) == 0 {
		return &ExecuteResponse{Success: false, Message: "没有选中的文件"}, nil
	}

	var params struct {
		Genres string `json:"genres"`
	}
	if action.Params != "" {
		json.Unmarshal([]byte(action.Params), &params)
	}

	successCount := 0
	var errors []string
	var opPreviews []OperationPreview

	for _, id := range session.Context.SelectedMediaIDs {
		media, err := s.mediaRepo.FindByID(id)
		if err != nil {
			continue
		}
		updates := map[string]interface{}{"genres": params.Genres}
		_, err = s.fileManager.UpdateFileInfo(id, updates, userID)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", media.Title, err))
		} else {
			successCount++
			opPreviews = append(opPreviews, OperationPreview{
				MediaID:    id,
				Title:      media.Title,
				OldValue:   media.Genres,
				NewValue:   params.Genres,
				ChangeType: "tag",
			})
		}
	}

	op := AssistantOperation{
		ID:         uuid.New().String(),
		SessionID:  session.ID,
		Action:     "batch_tag",
		Previews:   opPreviews,
		ExecutedAt: time.Now(),
		UserID:     userID,
	}
	s.addOperation(op)

	return &ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("批量标签更新完成，成功 %d 个", successCount),
		Results: opPreviews,
		Errors:  errors,
		OpID:    op.ID,
	}, nil
}

// executeAnalyze 执行文件分析
func (s *AIAssistantService) executeAnalyze(session *ChatSession, userID string) (*ExecuteResponse, error) {
	stats, err := s.fileManager.GetStats("", "")
	if err != nil {
		return &ExecuteResponse{Success: false, Message: "获取统计信息失败"}, nil
	}

	// 构建分析报告
	report := fmt.Sprintf("## 📊 文件库分析报告\n\n"+
		"| 指标 | 数值 |\n|------|------|\n"+
		"| 总文件数 | %d |\n"+
		"| 电影数量 | %d |\n"+
		"| 剧集数量 | %d |\n"+
		"| 已刮削 | %d |\n"+
		"| 未刮削 | %d |\n"+
		"| 总大小 | %.2f GB |\n"+
		"| 近7天导入 | %d |\n",
		stats.TotalFiles, stats.MovieCount, stats.EpisodeCount,
		stats.ScrapedCount, stats.UnscrapedCount,
		float64(stats.TotalSizeBytes)/(1024*1024*1024),
		stats.RecentImports,
	)

	// 添加建议
	var suggestions []string
	if stats.UnscrapedCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("- 🔍 有 %d 个文件尚未刮削元数据，建议执行批量刮削", stats.UnscrapedCount))
	}
	if stats.TotalFiles > 0 && float64(stats.ScrapedCount)/float64(stats.TotalFiles) < 0.5 {
		suggestions = append(suggestions, "- ⚠️ 刮削覆盖率不足50%，建议优先处理未刮削文件")
	}
	if len(suggestions) > 0 {
		report += "\n### 💡 优化建议\n" + strings.Join(suggestions, "\n")
	}

	return &ExecuteResponse{
		Success: true,
		Message: report,
	}, nil
}

// ==================== 误分类检测与重分类 ====================

// executeAnalyzeMisclassification 执行误分类分析
func (s *AIAssistantService) executeAnalyzeMisclassification(session *ChatSession, userID string) (*ExecuteResponse, error) {
	report, err := s.fileManager.AnalyzeMisclassification()
	if err != nil {
		return &ExecuteResponse{Success: false, Message: fmt.Sprintf("分析失败: %v", err)}, nil
	}

	// 构建分析报告
	var sb strings.Builder
	sb.WriteString("## 🔍 剧集误分类分析报告\n\n")
	sb.WriteString(fmt.Sprintf("| 指标 | 数值 |\n|------|------|\n"))
	sb.WriteString(fmt.Sprintf("| 当前电影总数 | %d |\n", report.TotalMovies))
	sb.WriteString(fmt.Sprintf("| 疑似剧集数量 | %d |\n", report.SuspectedEpisodes))
	sb.WriteString(fmt.Sprintf("| 🔴 高置信度 (>80%%) | %d |\n", report.HighConfidence))
	sb.WriteString(fmt.Sprintf("| 🟡 中置信度 (50%%-80%%) | %d |\n", report.MediumConfidence))
	sb.WriteString(fmt.Sprintf("| 🟢 低置信度 (<50%%) | %d |\n", report.LowConfidence))

	// 常见模式
	if len(report.CommonPatterns) > 0 {
		sb.WriteString("\n### 📋 常见误分类模式\n")
		for _, p := range report.CommonPatterns {
			sb.WriteString(fmt.Sprintf("- %s\n", p))
		}
	}

	// 详细列表（最多展示15条）
	if len(report.Items) > 0 {
		sb.WriteString("\n### 📄 疑似误分类文件详情\n")
		limit := 15
		if len(report.Items) < limit {
			limit = len(report.Items)
		}
		for i := 0; i < limit; i++ {
			item := report.Items[i]
			sb.WriteString(fmt.Sprintf("\n**%d. %s** (置信度: %.0f%%)\n", i+1, item.Title, item.Confidence*100))
			sb.WriteString(fmt.Sprintf("- 文件: `%s`\n", filepath.Base(item.FilePath)))
			sb.WriteString(fmt.Sprintf("- 目录: `%s`\n", filepath.Base(item.DirPath)))
			sb.WriteString(fmt.Sprintf("- 同目录文件数: %d\n", item.SiblingCount))
			for _, r := range item.Reasons {
				sb.WriteString(fmt.Sprintf("  - ✓ %s\n", r))
			}
		}
		if len(report.Items) > limit {
			sb.WriteString(fmt.Sprintf("\n*（还有 %d 个文件未展示）*\n", len(report.Items)-limit))
		}
	}

	// 建议
	if len(report.Suggestions) > 0 {
		sb.WriteString("\n### 💡 优化建议\n")
		for _, s := range report.Suggestions {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
	}

	// 将报告数据缓存到会话上下文中，供后续重分类使用
	session.Context.LastIntent = &Intent{
		Action:     "analyze_misclassification",
		Targets:    "all",
		Confidence: 1.0,
		Reasoning:  fmt.Sprintf("分析完成，发现 %d 个疑似误分类文件", report.SuspectedEpisodes),
	}

	// 构建预览数据
	var previews []OperationPreview
	previewLimit := 10
	if len(report.Items) < previewLimit {
		previewLimit = len(report.Items)
	}
	for i := 0; i < previewLimit; i++ {
		item := report.Items[i]
		previews = append(previews, OperationPreview{
			MediaID:    item.MediaID,
			Title:      item.Title,
			OldValue:   item.CurrentType,
			NewValue:   item.SuggestedType,
			ChangeType: "reclassify",
		})
	}

	resp := &ExecuteResponse{
		Success: true,
		Message: sb.String(),
		Results: previews,
	}

	return resp, nil
}

// executeReclassify 执行指定文件的重分类
func (s *AIAssistantService) executeReclassify(session *ChatSession, action *SuggestedAction, userID string) (*ExecuteResponse, error) {
	var params struct {
		MediaIDs       []string `json:"media_ids"`
		AutoLinkSeries bool     `json:"auto_link_series"`
	}
	if action.Params != "" {
		json.Unmarshal([]byte(action.Params), &params)
	}

	if len(params.MediaIDs) == 0 {
		return &ExecuteResponse{Success: false, Message: "未指定要重分类的文件"}, nil
	}

	req := ReclassifyRequest{
		MediaIDs:       params.MediaIDs,
		NewType:        "episode",
		AutoLinkSeries: params.AutoLinkSeries,
	}

	result, err := s.fileManager.ReclassifyFiles(req, userID)
	if err != nil {
		return &ExecuteResponse{Success: false, Message: err.Error()}, nil
	}

	// 记录操作
	var opPreviews []OperationPreview
	for _, id := range params.MediaIDs {
		opPreviews = append(opPreviews, OperationPreview{
			MediaID:    id,
			OldValue:   "movie",
			NewValue:   "episode",
			ChangeType: "reclassify",
		})
	}

	op := AssistantOperation{
		ID:         uuid.New().String(),
		SessionID:  session.ID,
		Action:     "reclassify",
		Previews:   opPreviews,
		ExecutedAt: time.Now(),
		UserID:     userID,
	}
	s.addOperation(op)

	msg := fmt.Sprintf("重分类完成！成功 %d 个", result.Success)
	if result.LinkedSeries > 0 {
		msg += fmt.Sprintf("，其中 %d 个已自动关联到剧集合集", result.LinkedSeries)
	}
	if result.Failed > 0 {
		msg += fmt.Sprintf("，失败 %d 个", result.Failed)
	}

	resp := &ExecuteResponse{
		Success: true,
		Message: msg,
		Errors:  result.Errors,
		OpID:    op.ID,
	}

	return resp, nil
}

// executeReclassifyByConfidence 按置信度阈值批量重分类
func (s *AIAssistantService) executeReclassifyByConfidence(session *ChatSession, minConfidence float64, userID string) (*ExecuteResponse, error) {
	// 先执行分析
	report, err := s.fileManager.AnalyzeMisclassification()
	if err != nil {
		return &ExecuteResponse{Success: false, Message: fmt.Sprintf("分析失败: %v", err)}, nil
	}

	// 筛选符合置信度的文件
	var mediaIDs []string
	for _, item := range report.Items {
		if item.Confidence >= minConfidence {
			mediaIDs = append(mediaIDs, item.MediaID)
		}
	}

	if len(mediaIDs) == 0 {
		confLabel := "高"
		if minConfidence < 0.5 {
			confLabel = "任意"
		}
		return &ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("没有找到%s置信度（≥%.0f%%）的误分类文件", confLabel, minConfidence*100),
		}, nil
	}

	req := ReclassifyRequest{
		MediaIDs:       mediaIDs,
		NewType:        "episode",
		AutoLinkSeries: true,
	}

	result, err := s.fileManager.ReclassifyFiles(req, userID)
	if err != nil {
		return &ExecuteResponse{Success: false, Message: err.Error()}, nil
	}

	// 记录操作
	var opPreviews []OperationPreview
	for _, id := range mediaIDs {
		opPreviews = append(opPreviews, OperationPreview{
			MediaID:    id,
			OldValue:   "movie",
			NewValue:   "episode",
			ChangeType: "reclassify",
		})
	}

	op := AssistantOperation{
		ID:         uuid.New().String(),
		SessionID:  session.ID,
		Action:     "reclassify",
		Previews:   opPreviews,
		ExecutedAt: time.Now(),
		UserID:     userID,
	}
	s.addOperation(op)

	msg := fmt.Sprintf("批量重分类完成！\n- 处理文件: %d 个（置信度≥%.0f%%）\n- 成功: %d 个",
		result.Total, minConfidence*100, result.Success)
	if result.LinkedSeries > 0 {
		msg += fmt.Sprintf("\n- 自动关联剧集合集: %d 个", result.LinkedSeries)
	}
	if result.Failed > 0 {
		msg += fmt.Sprintf("\n- 失败: %d 个", result.Failed)
	}
	msg += "\n\n💡 建议对重分类的文件重新执行刮削，以获取正确的剧集元数据。"

	resp := &ExecuteResponse{
		Success: true,
		Message: msg,
		Results: opPreviews,
		Errors:  result.Errors,
		OpID:    op.ID,
	}

	return resp, nil
}

// ==================== 撤销操作 ====================

// UndoOperation 撤销操作
func (s *AIAssistantService) UndoOperation(opID, userID string) (*ExecuteResponse, error) {
	s.opHistoryMu.Lock()
	defer s.opHistoryMu.Unlock()

	var targetOp *AssistantOperation
	for i := range s.opHistory {
		if s.opHistory[i].ID == opID && !s.opHistory[i].Undone {
			targetOp = &s.opHistory[i]
			break
		}
	}

	if targetOp == nil {
		return nil, fmt.Errorf("操作不存在或已撤销")
	}

	// 根据操作类型执行撤销
	successCount := 0
	var errors []string

	for _, preview := range targetOp.Previews {
		switch preview.ChangeType {
		case "rename":
			updates := map[string]interface{}{"title": preview.OldValue}
			_, err := s.fileManager.UpdateFileInfo(preview.MediaID, updates, userID)
			if err != nil {
				errors = append(errors, err.Error())
			} else {
				successCount++
			}
		case "tag":
			updates := map[string]interface{}{"genres": preview.OldValue}
			_, err := s.fileManager.UpdateFileInfo(preview.MediaID, updates, userID)
			if err != nil {
				errors = append(errors, err.Error())
			} else {
				successCount++
			}
		case "reclassify":
			updates := map[string]interface{}{"media_type": preview.OldValue}
			_, err := s.fileManager.UpdateFileInfo(preview.MediaID, updates, userID)
			if err != nil {
				errors = append(errors, err.Error())
			} else {
				successCount++
			}
		}
	}

	targetOp.Undone = true

	resp := &ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("撤销完成，恢复了 %d 个文件", successCount),
	}
	if len(errors) > 0 {
		resp.Errors = errors
	}

	return resp, nil
}

// addOperation 添加操作记录
func (s *AIAssistantService) addOperation(op AssistantOperation) {
	s.opHistoryMu.Lock()
	defer s.opHistoryMu.Unlock()

	s.opHistory = append([]AssistantOperation{op}, s.opHistory...)
	// 保留最近100条
	if len(s.opHistory) > 100 {
		s.opHistory = s.opHistory[:100]
	}
}

// GetOperationHistory 获取操作历史
func (s *AIAssistantService) GetOperationHistory(userID string, limit int) []AssistantOperation {
	s.opHistoryMu.Lock()
	defer s.opHistoryMu.Unlock()

	var result []AssistantOperation
	for _, op := range s.opHistory {
		if op.UserID == userID {
			result = append(result, op)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetSession 获取会话信息
func (s *AIAssistantService) GetSession(sessionID string) (*ChatSession, error) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话不存在")
	}
	return session, nil
}

// DeleteSession 删除会话
func (s *AIAssistantService) DeleteSession(sessionID string) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	delete(s.sessions, sessionID)
}

// AnalyzeMisclassification 分析误分类（代理到FileManagerService）
func (s *AIAssistantService) AnalyzeMisclassification() (*MisclassificationReport, error) {
	return s.fileManager.AnalyzeMisclassification()
}

// ReclassifyFiles 批量重分类（代理到FileManagerService）
func (s *AIAssistantService) ReclassifyFiles(req ReclassifyRequest, userID string) (*ReclassifyResult, error) {
	return s.fileManager.ReclassifyFiles(req, userID)
}
