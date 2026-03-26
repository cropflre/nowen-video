package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== OpenAI 兼容 API 数据结构 ====================

// ChatMessage OpenAI 聊天消息
type ChatMessage struct {
	Role    string `json:"role"` // system / user / assistant
	Content string `json:"content"`
}

// ChatCompletionRequest OpenAI 聊天补全请求
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatCompletionResponse OpenAI 聊天补全响应
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ==================== AI 服务 ====================

// AIErrorLog AI 调用错误日志条目
type AIErrorLog struct {
	Time    string `json:"time"`
	Action  string `json:"action"`
	Error   string `json:"error"`
	Latency int64  `json:"latency_ms"`
}

// AIService AI 功能统一服务
type AIService struct {
	cfg       config.AIConfig
	appCfg    *config.Config
	mediaRepo *repository.MediaRepo
	logger    *zap.SugaredLogger
	client    *http.Client

	// 缓存（简单内存缓存）
	cache   map[string]*aiCacheEntry
	cacheMu sync.RWMutex

	// 并发控制
	semaphore chan struct{}

	// 限流
	lastRequest time.Time
	rateMu      sync.Mutex

	// 月度调用计数
	monthlyCount int
	countMonth   int // 当前月份
	countMu      sync.Mutex

	// Token 消耗统计
	totalPromptTokens     int
	totalCompletionTokens int
	tokenMu               sync.Mutex

	// 错误日志（保留最近 50 条）
	errorLogs []AIErrorLog
	errorMu   sync.Mutex
}

// aiCacheEntry 缓存条目
type aiCacheEntry struct {
	Value     string
	ExpiresAt time.Time
}

// NewAIService 创建 AI 服务
func NewAIService(cfg config.AIConfig, appCfg *config.Config, mediaRepo *repository.MediaRepo, logger *zap.SugaredLogger) *AIService {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30
	}

	maxConcurrent := cfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}

	s := &AIService{
		cfg:       cfg,
		appCfg:    appCfg,
		mediaRepo: mediaRepo,
		logger:    logger,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		cache:     make(map[string]*aiCacheEntry),
		semaphore: make(chan struct{}, maxConcurrent),
	}

	if cfg.Enabled {
		logger.Infof("AI 服务已启用 (提供商: %s, 模型: %s, API: %s)",
			cfg.Provider, cfg.Model, maskAPIBase(cfg.APIBase))
		logger.Infof("AI 功能开关 — 智能搜索: %v, 推荐理由: %v, 元数据增强: %v",
			cfg.EnableSmartSearch, cfg.EnableRecommendReason, cfg.EnableMetadataEnhance)
	} else {
		logger.Info("AI 服务未启用（如需启用，请配置 config/ai.yaml）")
	}

	return s
}

// IsEnabled 检查 AI 服务是否启用
func (s *AIService) IsEnabled() bool {
	return s.cfg.Enabled && s.cfg.APIKey != "" && s.cfg.APIBase != ""
}

// IsSmartSearchEnabled 检查智能搜索是否启用
func (s *AIService) IsSmartSearchEnabled() bool {
	return s.IsEnabled() && s.cfg.EnableSmartSearch
}

// IsRecommendReasonEnabled 检查推荐理由生成是否启用
func (s *AIService) IsRecommendReasonEnabled() bool {
	return s.IsEnabled() && s.cfg.EnableRecommendReason
}

// IsMetadataEnhanceEnabled 检查元数据增强是否启用
func (s *AIService) IsMetadataEnhanceEnabled() bool {
	return s.IsEnabled() && s.cfg.EnableMetadataEnhance
}

// ==================== LLM 调用核心 ====================

// ChatCompletion 调用 LLM 聊天补全 API
func (s *AIService) ChatCompletion(systemPrompt, userPrompt string, temperature float64, maxTokens int) (string, error) {
	if !s.IsEnabled() {
		return "", fmt.Errorf("AI 服务未启用")
	}

	// 预算检查
	if !s.checkBudget() {
		return "", fmt.Errorf("AI 月度调用预算已用尽")
	}

	// 并发控制
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	// 限流
	s.rateLimit()

	// 构建请求
	reqBody := ChatCompletionRequest{
		Model: s.cfg.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	apiURL := strings.TrimRight(s.cfg.APIBase, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("AI API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		s.logger.Warnf("AI API 返回错误 (HTTP %d): %s", resp.StatusCode, string(respBody))
		errMsg := fmt.Errorf("AI API 返回 HTTP %d", resp.StatusCode)
		s.addErrorLog("chat_completion", errMsg, time.Since(time.Now()).Milliseconds())
		return "", errMsg
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("解析 AI 响应失败: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("AI 未返回任何结果")
	}

	// 更新调用计数和 Token 统计
	s.incrementCount()
	s.tokenMu.Lock()
	s.totalPromptTokens += chatResp.Usage.PromptTokens
	s.totalCompletionTokens += chatResp.Usage.CompletionTokens
	s.tokenMu.Unlock()

	result := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	s.logger.Debugf("AI 调用成功 (tokens: %d+%d=%d)",
		chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens, chatResp.Usage.TotalTokens)

	return result, nil
}

// ==================== 缓存 ====================

// GetCache 从缓存获取
func (s *AIService) GetCache(key string) (string, bool) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	entry, ok := s.cache[key]
	if !ok {
		return "", false
	}
	if time.Now().After(entry.ExpiresAt) {
		return "", false
	}
	return entry.Value, true
}

// SetCache 写入缓存
func (s *AIService) SetCache(key, value string) {
	ttl := s.cfg.CacheTTLHours
	if ttl <= 0 {
		return // 不缓存
	}

	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	s.cache[key] = &aiCacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(time.Duration(ttl) * time.Hour),
	}

	// 简单的缓存淘汰：超过 1000 条时清理过期的
	if len(s.cache) > 1000 {
		now := time.Now()
		for k, v := range s.cache {
			if now.After(v.ExpiresAt) {
				delete(s.cache, k)
			}
		}
	}
}

// ==================== 限流与预算 ====================

// rateLimit 请求间隔限流
func (s *AIService) rateLimit() {
	interval := time.Duration(s.cfg.RequestIntervalMs) * time.Millisecond
	if interval <= 0 {
		return
	}

	s.rateMu.Lock()
	defer s.rateMu.Unlock()

	elapsed := time.Since(s.lastRequest)
	if elapsed < interval {
		time.Sleep(interval - elapsed)
	}
	s.lastRequest = time.Now()
}

// checkBudget 检查月度预算
func (s *AIService) checkBudget() bool {
	if s.cfg.MonthlyBudget <= 0 {
		return true // 不限制
	}

	s.countMu.Lock()
	defer s.countMu.Unlock()

	currentMonth := time.Now().Month()
	if int(currentMonth) != s.countMonth {
		s.monthlyCount = 0
		s.countMonth = int(currentMonth)
	}

	return s.monthlyCount < s.cfg.MonthlyBudget
}

// incrementCount 增加调用计数
func (s *AIService) incrementCount() {
	s.countMu.Lock()
	defer s.countMu.Unlock()
	s.monthlyCount++
}

// ==================== 辅助 ====================

// maskAPIBase 掩码 API 地址（日志用）
func maskAPIBase(base string) string {
	if len(base) <= 20 {
		return base
	}
	return base[:20] + "..."
}

// GetStatus 获取 AI 服务状态（用于前端展示）
func (s *AIService) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"enabled":                 s.cfg.Enabled,
		"provider":                s.cfg.Provider,
		"model":                   s.cfg.Model,
		"api_base":                s.cfg.APIBase,
		"api_configured":          s.cfg.APIKey != "",
		"timeout":                 s.cfg.Timeout,
		"enable_smart_search":     s.cfg.EnableSmartSearch,
		"enable_recommend_reason": s.cfg.EnableRecommendReason,
		"enable_metadata_enhance": s.cfg.EnableMetadataEnhance,
		"cache_ttl_hours":         s.cfg.CacheTTLHours,
		"max_concurrent":          s.cfg.MaxConcurrent,
		"request_interval_ms":     s.cfg.RequestIntervalMs,
	}

	s.countMu.Lock()
	status["monthly_calls"] = s.monthlyCount
	status["monthly_budget"] = s.cfg.MonthlyBudget
	s.countMu.Unlock()

	s.tokenMu.Lock()
	status["total_prompt_tokens"] = s.totalPromptTokens
	status["total_completion_tokens"] = s.totalCompletionTokens
	status["total_tokens"] = s.totalPromptTokens + s.totalCompletionTokens
	s.tokenMu.Unlock()

	s.cacheMu.RLock()
	status["cache_entries"] = len(s.cache)
	s.cacheMu.RUnlock()

	return status
}

// ==================== 配置更新 ====================

// UpdateConfig 动态更新 AI 配置
func (s *AIService) UpdateConfig(updates map[string]interface{}) error {
	for key, val := range updates {
		switch key {
		case "enabled":
			if v, ok := val.(bool); ok {
				s.cfg.Enabled = v
			}
		case "provider":
			if v, ok := val.(string); ok {
				s.cfg.Provider = v
			}
		case "api_base":
			if v, ok := val.(string); ok {
				s.cfg.APIBase = v
			}
		case "api_key":
			if v, ok := val.(string); ok {
				s.cfg.APIKey = v
			}
		case "model":
			if v, ok := val.(string); ok {
				s.cfg.Model = v
			}
		case "timeout":
			if v, ok := val.(float64); ok {
				s.cfg.Timeout = int(v)
				s.client.Timeout = time.Duration(int(v)) * time.Second
			}
		case "enable_smart_search":
			if v, ok := val.(bool); ok {
				s.cfg.EnableSmartSearch = v
			}
		case "enable_recommend_reason":
			if v, ok := val.(bool); ok {
				s.cfg.EnableRecommendReason = v
			}
		case "enable_metadata_enhance":
			if v, ok := val.(bool); ok {
				s.cfg.EnableMetadataEnhance = v
			}
		case "monthly_budget":
			if v, ok := val.(float64); ok {
				s.cfg.MonthlyBudget = int(v)
			}
		case "cache_ttl_hours":
			if v, ok := val.(float64); ok {
				s.cfg.CacheTTLHours = int(v)
			}
		case "max_concurrent":
			if v, ok := val.(float64); ok {
				s.cfg.MaxConcurrent = int(v)
			}
		case "request_interval_ms":
			if v, ok := val.(float64); ok {
				s.cfg.RequestIntervalMs = int(v)
			}
		}
	}

	// 同步到全局配置
	s.appCfg.AI = s.cfg

	s.logger.Infof("AI 配置已更新")
	return nil
}

// ==================== 连接测试 ====================

// TestConnection 测试 AI API 连接
// 注意：此方法绕过 IsEnabled() 检查，直接验证 API 密钥和网络连通性
func (s *AIService) TestConnection() (map[string]interface{}, error) {
	if s.cfg.APIKey == "" || s.cfg.APIBase == "" {
		return nil, fmt.Errorf("API Key 或 API Base 未配置")
	}

	start := time.Now()

	// 直接构建请求，绕过 ChatCompletion 中的 IsEnabled/预算检查
	reqBody := ChatCompletionRequest{
		Model: s.cfg.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: "你是一个测试助手。"},
			{Role: "user", Content: "请回复 OK"},
		},
		Temperature: 0.0,
		MaxTokens:   10,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	apiURL := strings.TrimRight(s.cfg.APIBase, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)

	resp, err := s.client.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		s.addErrorLog("connection_test", err, latency)
		return map[string]interface{}{
			"success":    false,
			"error":      fmt.Sprintf("网络请求失败: %v", err),
			"latency_ms": latency,
		}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		errMsg := fmt.Errorf("读取响应失败: %w", err)
		s.addErrorLog("connection_test", errMsg, latency)
		return map[string]interface{}{
			"success":    false,
			"error":      errMsg.Error(),
			"latency_ms": latency,
		}, errMsg
	}

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Errorf("API 返回 HTTP %d: %s", resp.StatusCode, string(respBody))
		s.addErrorLog("connection_test", errMsg, latency)
		return map[string]interface{}{
			"success":    false,
			"error":      errMsg.Error(),
			"latency_ms": latency,
		}, errMsg
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		errMsg := fmt.Errorf("解析响应失败: %w", err)
		s.addErrorLog("connection_test", errMsg, latency)
		return map[string]interface{}{
			"success":    false,
			"error":      errMsg.Error(),
			"latency_ms": latency,
		}, errMsg
	}

	result := ""
	if len(chatResp.Choices) > 0 {
		result = strings.TrimSpace(chatResp.Choices[0].Message.Content)
	}

	return map[string]interface{}{
		"success":    true,
		"response":   result,
		"latency_ms": latency,
		"provider":   s.cfg.Provider,
		"model":      s.cfg.Model,
	}, nil
}

// ==================== 缓存管理 ====================

// ClearCache 清空所有 AI 缓存
func (s *AIService) ClearCache() int {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	count := len(s.cache)
	s.cache = make(map[string]*aiCacheEntry)
	s.logger.Infof("AI 缓存已清空，共清理 %d 条", count)
	return count
}

// GetCacheStats 获取缓存统计
func (s *AIService) GetCacheStats() map[string]interface{} {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	total := len(s.cache)
	expired := 0
	now := time.Now()
	for _, entry := range s.cache {
		if now.After(entry.ExpiresAt) {
			expired++
		}
	}

	return map[string]interface{}{
		"total_entries":   total,
		"active_entries":  total - expired,
		"expired_entries": expired,
		"ttl_hours":       s.cfg.CacheTTLHours,
	}
}

// ==================== 错误日志 ====================

// addErrorLog 添加错误日志
func (s *AIService) addErrorLog(action string, err error, latencyMs int64) {
	s.errorMu.Lock()
	defer s.errorMu.Unlock()

	entry := AIErrorLog{
		Time:    time.Now().Format("2006-01-02 15:04:05"),
		Action:  action,
		Error:   err.Error(),
		Latency: latencyMs,
	}

	s.errorLogs = append([]AIErrorLog{entry}, s.errorLogs...)
	if len(s.errorLogs) > 50 {
		s.errorLogs = s.errorLogs[:50]
	}
}

// GetErrorLogs 获取最近的错误日志
func (s *AIService) GetErrorLogs() []AIErrorLog {
	s.errorMu.Lock()
	defer s.errorMu.Unlock()

	result := make([]AIErrorLog, len(s.errorLogs))
	copy(result, s.errorLogs)
	return result
}

// ==================== 功能测试 ====================

// TestSmartSearch 测试智能搜索功能
func (s *AIService) TestSmartSearch(query string) (map[string]interface{}, error) {
	start := time.Now()
	intent, err := s.ParseSearchIntent(query)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		s.addErrorLog("test_smart_search", err, latency)
		return nil, err
	}

	return map[string]interface{}{
		"success":    true,
		"intent":     intent,
		"latency_ms": latency,
	}, nil
}

// TestRecommendReason 测试推荐理由生成
func (s *AIService) TestRecommendReason(title string, genres string) (map[string]interface{}, error) {
	start := time.Now()

	// 构建一个模拟的 media 对象
	mockMedia := &model.Media{
		Title:  title,
		Genres: genres,
		Rating: 8.5,
	}

	reason := s.GenerateRecommendReason(mockMedia, strings.Split(genres, ","), "基于你的观影偏好推荐")
	latency := time.Since(start).Milliseconds()

	return map[string]interface{}{
		"success":    true,
		"reason":     reason,
		"latency_ms": latency,
	}, nil
}
