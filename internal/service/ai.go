package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
	Model          string                        `json:"model"`
	Messages       []ChatMessage                 `json:"messages"`
	Temperature    float64                       `json:"temperature,omitempty"`
	TopP           float64                       `json:"top_p,omitempty"`
	MaxTokens      int                           `json:"max_tokens,omitempty"`
	ResponseFormat *ChatCompletionResponseFormat `json:"response_format,omitempty"`
}

type ChatCompletionResponseFormat struct {
	Type string `json:"type"`
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
	// cfgMu 保护 cfg 的读/写，避免“热更新”与“并发请求读取 Profiles map”类场景下发生
	// concurrent map read and map write 。
	cfgMu  sync.RWMutex
	cfg    config.AIConfig
	appCfg *config.Config

	mediaRepo *repository.MediaRepo
	logger    *zap.SugaredLogger
	client    *http.Client

	// 双层缓存：内存缓存（快速读取）+ 持久化缓存（重启不丢失）
	cache     map[string]*aiCacheEntry
	cacheMu   sync.RWMutex
	cacheRepo *repository.AICacheRepo // 持久化缓存仓储

	// 并发控制
	semaphore chan struct{}

	// 限流
	lastRequest time.Time
	rateMu      sync.Mutex

	// 全局 429 冷却闸（cooldownMu 同时保护 cooldownUntil 和 cooldownReason）。
	// 任何一次请求收到 429（或 Retry-After）都会把 cooldownUntil 设到未来时刻，
	// 期间所有请求（含重试请求）在 rateLimit() 中统一阻塞等待，
	// 避免高并发下"A 在退避、B/C 还在打"的恶性循环导致整批被服务端拉黑。
	cooldownMu     sync.Mutex
	cooldownUntil  time.Time
	cooldownReason string

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

	// V7: 可选的智能路由（由 NewAIRouter 注入。为 nil 时边路以原逻辑走）
	router *AIRouter
}

// aiCacheEntry 缓存条目
type aiCacheEntry struct {
	Value     string
	ExpiresAt time.Time
}

// NewAIService 创建 AI 服务
func NewAIService(cfg config.AIConfig, appCfg *config.Config, mediaRepo *repository.MediaRepo, cacheRepo *repository.AICacheRepo, logger *zap.SugaredLogger) *AIService {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30
	}

	// 默认参数：偏保守，避开主流云端 LLM 服务（OpenAI / 阿里云百炼 / DeepSeek）的免费/低档限流
	//   - max_concurrent = 1   ：避免单 Key 多通道并行打 API 触发 RPM 限流
	//   - request_interval_ms = 1100 ：≈ 0.9 QPS，远低于 60 QPM 免费档上限
	// 用户在前端"AI 高级设置"里可显式调高（专业版套餐通常没有 RPM 限制）。
	maxConcurrent := cfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	if cfg.RequestIntervalMs <= 0 {
		cfg.RequestIntervalMs = 1100
	}
	if cfg.CacheTTLHours <= 0 {
		cfg.CacheTTLHours = 24 * 30 // 默认缓存 30 天，命名识别结果几乎不变
	}

	s := &AIService{
		cfg:       cfg,
		appCfg:    appCfg,
		mediaRepo: mediaRepo,
		cacheRepo: cacheRepo,
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

// SetRouter 注入 AI 智能路由器（V7：故障转移与用量监控）
//
// 与 NewAIRouter 配合：
//   - NewAIRouter(ai, ...) 内部会调用 ai.SetRouter(r)
//   - 之后 ChatCompletion 的成功/失败钩子都会回调 router
//
// 二次设置会覆盖（用于运行时替换 router 实现，例如热升级测试）。
func (s *AIService) SetRouter(r *AIRouter) {
	s.cfgMu.Lock()
	s.router = r
	s.cfgMu.Unlock()
}

// Router 返回当前注入的 router（可能为 nil）
func (s *AIService) Router() *AIRouter {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.router
}

// IsEnabled 检查 AI 服务是否启用
//
// AutoPilot 语义：只要配了 APIKey 与 APIBase，即使 Enabled=false 也会被视为启用，
// 从而避免「用户填了 key 却忘记打 enabled 开关」造成的 AI 不生效。
func (s *AIService) IsEnabled() bool {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	if s.cfg.APIKey == "" || s.cfg.APIBase == "" {
		return false
	}
	if s.cfg.AutoPilot {
		return true
	}
	return s.cfg.Enabled
}

// Provider 返回当前生效的 AI 服务商（来自 AI 配置中心，可在管理面板动态切换）
func (s *AIService) Provider() string {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg.Provider
}

// Model 返回当前生效的 AI 模型
func (s *AIService) Model() string {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg.Model
}

// IsSmartSearchEnabled 检查智能搜索是否启用
func (s *AIService) IsSmartSearchEnabled() bool {
	if !s.IsEnabled() {
		return false
	}
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg.AutoPilot || s.cfg.EnableSmartSearch
}

// IsRecommendReasonEnabled 检查推荐理由生成是否启用
func (s *AIService) IsRecommendReasonEnabled() bool {
	if !s.IsEnabled() {
		return false
	}
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg.AutoPilot || s.cfg.EnableRecommendReason
}

// IsMetadataEnhanceEnabled 检查元数据增强是否启用
func (s *AIService) IsMetadataEnhanceEnabled() bool {
	if !s.IsEnabled() {
		return false
	}
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg.AutoPilot || s.cfg.EnableMetadataEnhance
}

// IsAutoPilotEnabled 是否处于全自动托管模式
func (s *AIService) IsAutoPilotEnabled() bool {
	if !s.IsEnabled() {
		return false
	}
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg.AutoPilot
}

// snapshotCfg 返回当前 cfg 的一个“使用脚手架”，包含主要只读字段。
// Profiles 是 map，拷贝为新 map 以避免上层考察 map 时发生读写竞争。
func (s *AIService) snapshotCfg() config.AIConfig {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	cp := s.cfg
	if s.cfg.Profiles != nil {
		cp.Profiles = make(map[string]config.AIProviderProfile, len(s.cfg.Profiles))
		for k, v := range s.cfg.Profiles {
			cp.Profiles[k] = v
		}
	}
	return cp
}

// ==================== LLM 调用核心 ====================

// ChatCompletion 调用 LLM 聊天补全 API
//
// 重试策略（针对常见的限流/瞬时错误）：
//   - HTTP 429 / 5xx / 网络错误自动指数退避重试（最多 4 次：1s -> 2s -> 4s -> 8s）
//   - 优先读取响应头中的 `Retry-After`（秒）作为退避时长
//   - 4xx（除 429 外）直接返回，不重试，避免无效请求消耗配额
//   - 整体最长等待 ≈ 15s，超过后返回最后一次错误
func supportsJSONResponseFormat(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "deepseek", "qwen", "zhipu":
		return true
	default:
		return false
	}
}

func (s *AIService) ChatCompletion(systemPrompt, userPrompt string, temperature float64, maxTokens int) (string, error) {
	if !s.IsEnabled() {
		return "", fmt.Errorf("AI 服务未启用")
	}

	// 以“一次快照”在本次调用全程使用同一份 cfg，避免中途热更新导致 model/url 不一致，
	// 并且避免与 UpdateConfig 发生“concurrent map read and map write”。
	cfg := s.snapshotCfg()

	// V7：走热路径时顺便检查主 provider 是否可自动恢复
	if s.router != nil {
		s.router.MaybeAutoRecover()
		cfg = s.snapshotCfg()
	}

	// 云端强制：拦截本地 AI（如 ollama），避免「全自动托管」语义下走本地推理
	if cfg.BlockLocalAI && isLocalAIProvider(cfg.Provider, cfg.APIBase) {
		return "", fmt.Errorf("当前系统禁止本地 AI（provider=%s），请在 AI 配置中选择云端服务商（OpenAI/DeepSeek/通义千问）", cfg.Provider)
	}

	// 预算检查
	if !s.checkBudget() {
		return "", fmt.Errorf("AI 月度调用预算已用尽")
	}

	// 并发控制
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	// 构建请求体（一次构建，多次重试复用）
	reqBody := ChatCompletionRequest{
		Model: cfg.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}
	// 低温 JSON/短文本任务进一步收窄 top_p，降低 deepseek-v4-flash 冗长/发散输出概率。
	if temperature <= 0.1 && maxTokens > 0 && maxTokens <= 1024 {
		reqBody.TopP = 0.1
		if maxTokens >= 128 && supportsJSONResponseFormat(cfg.Provider) {
			reqBody.ResponseFormat = &ChatCompletionResponseFormat{Type: "json_object"}
		}
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}
	apiURL := strings.TrimRight(cfg.APIBase, "/") + "/chat/completions"

	const maxAttempts = 4
	var lastErr error
	var respBody []byte

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// 限流（每次重试前都过一遍间隔限流）
		s.rateLimit()

		req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("创建请求失败: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

		resp, err := s.client.Do(req)
		if err != nil {
			// 网络错误：可重试，写入全局冷却闸（让其他并发请求一起等）
			lastErr = fmt.Errorf("AI API 请求失败: %w", err)
			if attempt < maxAttempts {
				wait := backoffDuration(attempt, "")
				s.logger.Warnf("AI 调用网络错误，%v 后重试 (第 %d/%d 次): %v", wait, attempt, maxAttempts, err)
				s.triggerCooldown(wait, fmt.Sprintf("network err: %v", err))
				continue // rateLimit() 会在下次循环顶部尊重冷却闸
			}
			break
		}

		respBody, err = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("读取响应失败: %w", err)
			if attempt < maxAttempts {
				wait := backoffDuration(attempt, "")
				s.triggerCooldown(wait, "read body err")
				continue
			}
			break
		}

		// 成功
		if resp.StatusCode == http.StatusOK {
			var chatResp ChatCompletionResponse
			if err := json.Unmarshal(respBody, &chatResp); err != nil {
				return "", fmt.Errorf("解析 AI 响应失败: %w", err)
			}
			if len(chatResp.Choices) == 0 {
				return "", fmt.Errorf("AI 未返回任何结果")
			}
			s.incrementCount()
			s.tokenMu.Lock()
			s.totalPromptTokens += chatResp.Usage.PromptTokens
			s.totalCompletionTokens += chatResp.Usage.CompletionTokens
			s.tokenMu.Unlock()
			result := strings.TrimSpace(chatResp.Choices[0].Message.Content)
			s.logger.Debugf("AI 调用成功 (tokens: %d+%d=%d, attempt=%d)",
				chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens, chatResp.Usage.TotalTokens, attempt)
			// V7：通知路由器记录用量
			if s.router != nil {
				s.router.MarkSuccess(cfg.Provider, cfg.Model,
					chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens, 0, "chat_completion")
			}
			return result, nil
		}

		// 非 200：判断是否可重试
		retryAfter := resp.Header.Get("Retry-After")
		errMsg := fmt.Errorf("AI API 返回 HTTP %d", resp.StatusCode)
		lastErr = errMsg

		retryable := resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusGatewayTimeout ||
			(resp.StatusCode >= 500 && resp.StatusCode < 600)

		if retryable && attempt < maxAttempts {
			wait := backoffDuration(attempt, retryAfter)
			s.logger.Warnf("AI API HTTP %d，%v 后重试 (第 %d/%d 次, 触发全局冷却): %s",
				resp.StatusCode, wait, attempt, maxAttempts, truncateForLog(string(respBody), 200))
			// 关键：写入全局冷却闸，所有并发请求（包括其他 worker）会在 rateLimit() 一起等
			s.triggerCooldown(wait, fmt.Sprintf("HTTP %d", resp.StatusCode))
			continue
		}

		// 不可重试 / 已达上限
		s.logger.Warnf("AI API 返回错误 (HTTP %d): %s", resp.StatusCode, string(respBody))
		s.addErrorLog("chat_completion", errMsg, 0)
		// V7：通知路由器试图切换（这里不另外重试，避免双重试逻辑交叉）
		if s.router != nil {
			s.router.MarkFailure(cfg.Provider, resp.StatusCode, string(respBody))
		}
		return "", errMsg
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("AI 调用失败（未知原因）")
	}
	s.addErrorLog("chat_completion", lastErr, 0)
	// V7：重试后仍失败（网络错误路径），同样平衡给 router
	if s.router != nil {
		s.router.MarkFailure(cfg.Provider, 0, lastErr.Error())
	}
	return "", lastErr
}

// backoffDuration 计算重试退避时长。
//
// 优先使用 Retry-After 头（OpenAI / 阿里云百炼 / 多数兼容服务返回 N 秒整数）。
// 否则使用指数退避：1s -> 2s -> 4s -> 8s（最多 8s 单次）。
func backoffDuration(attempt int, retryAfter string) time.Duration {
	// 解析 Retry-After（秒数）
	if retryAfter != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && secs > 0 {
			if secs > 30 {
				secs = 30 // 上限保护
			}
			return time.Duration(secs) * time.Second
		}
	}
	// 指数退避：1s, 2s, 4s, 8s
	wait := time.Second << (attempt - 1)
	if wait > 8*time.Second {
		wait = 8 * time.Second
	}
	return wait
}

// truncateForLog 截断长字符串，仅用于日志展示
func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ==================== 缓存 ====================

// GetCache 从缓存获取（双层：先查内存，再查持久化存储）
func (s *AIService) GetCache(key string) (string, bool) {
	// 第一层：内存缓存
	s.cacheMu.RLock()
	entry, ok := s.cache[key]
	s.cacheMu.RUnlock()
	if ok && time.Now().Before(entry.ExpiresAt) {
		return entry.Value, true
	}

	// 第二层：持久化缓存
	if s.cacheRepo != nil {
		if val, found := s.cacheRepo.Get(key); found {
			s.cfgMu.RLock()
			ttl := s.cfg.CacheTTLHours
			s.cfgMu.RUnlock()
			// 回填内存缓存
			s.cacheMu.Lock()
			s.cache[key] = &aiCacheEntry{
				Value:     val,
				ExpiresAt: time.Now().Add(time.Duration(ttl) * time.Hour),
			}
			s.cacheMu.Unlock()
			return val, true
		}
	}

	return "", false
}

// SetCache 写入缓存（双层：同时写入内存和持久化存储）
func (s *AIService) SetCache(key, value string) {
	s.cfgMu.RLock()
	ttl := s.cfg.CacheTTLHours
	s.cfgMu.RUnlock()
	if ttl <= 0 {
		return // 不缓存
	}

	// 写入内存缓存
	s.cacheMu.Lock()
	s.cache[key] = &aiCacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(time.Duration(ttl) * time.Hour),
	}

	// 简单的内存缓存淘汰：超过 500 条时清理过期的
	if len(s.cache) > 500 {
		now := time.Now()
		for k, v := range s.cache {
			if now.After(v.ExpiresAt) {
				delete(s.cache, k)
			}
		}
	}
	s.cacheMu.Unlock()

	// 写入持久化缓存（异步，不阻塞主流程）
	if s.cacheRepo != nil {
		go func() {
			if err := s.cacheRepo.Set(key, value, ttl); err != nil {
				s.logger.Debugf("AI 缓存持久化失败: %v", err)
			}
		}()
	}
}

// ==================== 限流与预算 ====================

// rateLimit 请求间隔限流 + 全局 429 冷却闸。
//
// 顺序：
//  1. 若全局冷却闸生效（来自上一次 429 / Retry-After），先阻塞等待到冷却结束；
//  2. 再走最小请求间隔限流（避免同一秒内挤压新请求）。
//
// 关键点：cooldownUntil 用 cooldownMu 短暂上锁读取，sleep 时不持锁，
// 防止 N 个请求互相阻塞导致退避后再次同时打 API。
func (s *AIService) rateLimit() {
	// 1) 全局 429 冷却闸
	for {
		s.cooldownMu.Lock()
		until := s.cooldownUntil
		reason := s.cooldownReason
		s.cooldownMu.Unlock()
		now := time.Now()
		if !until.After(now) {
			break
		}
		wait := until.Sub(now)
		if wait > 30*time.Second {
			wait = 30 * time.Second // 上限保护
		}
		s.logger.Debugf("AI 全局冷却中，等待 %v（原因: %s）", wait, reason)
		time.Sleep(wait)
		// 循环再次确认（其他请求可能在我们 sleep 期间又触发了新的 cooldown 延期）
	}

	// 2) 最小请求间隔
	s.cfgMu.RLock()
	interval := time.Duration(s.cfg.RequestIntervalMs) * time.Millisecond
	s.cfgMu.RUnlock()
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

// triggerCooldown 触发全局 429 冷却闸。
//
// 入参 wait 为本次需要等待的时长（已由调用方综合 Retry-After / 指数退避算好）。
// 函数将 cooldownUntil 推到 max(原 until, now+wait)：取较远者，
// 保证多个请求"同时挨打"时不会被某个较早完成的退避把闸提前抬起。
//
// reason 仅用于日志展示。
func (s *AIService) triggerCooldown(wait time.Duration, reason string) {
	if wait <= 0 {
		return
	}
	target := time.Now().Add(wait)
	s.cooldownMu.Lock()
	if target.After(s.cooldownUntil) {
		s.cooldownUntil = target
		s.cooldownReason = reason
	}
	s.cooldownMu.Unlock()
}

// checkBudget 检查月度预算
func (s *AIService) checkBudget() bool {
	s.cfgMu.RLock()
	budget := s.cfg.MonthlyBudget
	s.cfgMu.RUnlock()
	if budget <= 0 {
		return true // 不限制
	}

	s.countMu.Lock()
	defer s.countMu.Unlock()

	currentMonth := time.Now().Month()
	if int(currentMonth) != s.countMonth {
		s.monthlyCount = 0
		s.countMonth = int(currentMonth)
	}

	return s.monthlyCount < budget
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

// isLocalAIProvider 判断是否为本地 AI 推理实例（provider 名或 api_base 是本地地址）
// AutoPilot/BlockLocalAI 语义下会拒绝调用，确保「全部调用服务商提供的 AI API」。
func isLocalAIProvider(provider, apiBase string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "ollama" || p == "local" || p == "localai" || p == "llamacpp" || p == "llama-cpp" {
		return true
	}
	b := strings.ToLower(strings.TrimSpace(apiBase))
	if b == "" {
		return false
	}
	// 常见本地地址特征
	localHints := []string{
		"localhost", "127.0.0.1", "0.0.0.0",
		"://192.168.", "://10.", "://172.16.", "://172.17.", "://172.18.", "://172.19.",
		"://172.20.", "://172.21.", "://172.22.", "://172.23.", "://172.24.",
		"://172.25.", "://172.26.", "://172.27.", "://172.28.", "://172.29.",
		"://172.30.", "://172.31.",
		":11434", // Ollama 默认端口
	}
	for _, h := range localHints {
		if strings.Contains(b, h) {
			return true
		}
	}
	return false
}

// GetStatus 获取 AI 服务状态（用于前端展示）
func (s *AIService) GetStatus() map[string]interface{} {
	cfg := s.snapshotCfg()
	status := map[string]interface{}{
		"enabled":                 cfg.Enabled,
		"auto_pilot":              cfg.AutoPilot,
		"block_local_ai":          cfg.BlockLocalAI,
		"provider":                cfg.Provider,
		"model":                   cfg.Model,
		"api_base":                cfg.APIBase,
		"api_configured":          cfg.APIKey != "",
		"timeout":                 cfg.Timeout,
		"enable_smart_search":     cfg.EnableSmartSearch,
		"enable_recommend_reason": cfg.EnableRecommendReason,
		"enable_metadata_enhance": cfg.EnableMetadataEnhance,
		"cache_ttl_hours":         cfg.CacheTTLHours,
		"max_concurrent":          cfg.MaxConcurrent,
		"request_interval_ms":     cfg.RequestIntervalMs,
	}

	// 多 provider 配置档案（key 字段脱敏，仅返回 api_key_configured 标识）
	profilesView := make(map[string]map[string]interface{}, len(cfg.Profiles))
	for id, p := range cfg.Profiles {
		view := map[string]interface{}{
			"api_base":           p.APIBase,
			"model":              p.Model,
			"api_key_configured": p.APIKey != "",
			"enabled":            p.Enabled,
		}
		// V8：模型级 failover 元信息
		if p.CurrentModel != "" {
			view["current_model"] = p.CurrentModel
		}
		// 即便 ModelChain 为空也返回空数组，方便前端识别
		if p.ModelChain == nil {
			view["model_chain"] = []string{}
		} else {
			view["model_chain"] = p.ModelChain
		}
		profilesView[id] = view
	}
	status["profiles"] = profilesView

	s.countMu.Lock()
	status["monthly_calls"] = s.monthlyCount
	status["monthly_budget"] = cfg.MonthlyBudget
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
//
// 支持的 updates 字段：
//   - 顶层基础字段（enabled / provider / api_base / api_key / model / timeout / 各功能开关 / 高级设置）
//   - profiles: map[string]{api_base, api_key, model} 多 provider 配置档案
//
// 写入规则：
//  1. 顶层字段更新到 s.cfg
//  2. profiles 字段会与现有 s.cfg.Profiles 合并（仅覆盖传入的 provider id）
//  3. 每个 profile 的 api_key 留空时，保留现有 key（避免误清空）
//  4. 同步顶层激活配置 → s.cfg.Profiles[provider]，确保两者一致
//  5. 持久化到 config/ai.yaml
func (s *AIService) UpdateConfig(updates map[string]interface{}) error {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	for key, val := range updates {
		switch key {
		case "enabled":
			if v, ok := val.(bool); ok {
				s.cfg.Enabled = v
			}
		case "auto_pilot":
			if v, ok := val.(bool); ok {
				s.cfg.AutoPilot = v
				// 开启 AutoPilot 时隐式点亮主开关与子功能开关，避免老配置遗留中的 false 干扰「一键设为」体验
				if v {
					s.cfg.Enabled = true
					s.cfg.EnableSmartSearch = true
					s.cfg.EnableRecommendReason = true
					s.cfg.EnableMetadataEnhance = true
				}
			}
		case "block_local_ai":
			if v, ok := val.(bool); ok {
				s.cfg.BlockLocalAI = v
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
		case "profiles":
			// 多 provider 配置档案合并
			s.mergeProfilesUpdate(val)
		}
	}

	// 顶层激活配置 → 同步到 profiles[provider]，确保保存后两者一致
	s.syncActiveProfile()

	// 同步到全局配置
	s.appCfg.AI = s.cfg

	// 持久化到 config/ai.yaml
	if err := s.appCfg.SaveAIConfig(); err != nil {
		s.logger.Warnf("AI 配置已更新但持久化失败: %v", err)
		// 不阻塞热更新生效，仅警告
	} else {
		s.logger.Infof("AI 配置已更新并写入 config/ai.yaml")
	}
	return nil
}

// mergeProfilesUpdate 合并前端传来的 profiles 增量到 s.cfg.Profiles
// 调用者请确保已持有 cfgMu.Lock。输入应为 map[string]interface{}，
// 每个值为 map{api_base, api_key, model}；api_key 为空字符串时保留现有 key
func (s *AIService) mergeProfilesUpdate(val interface{}) {
	raw, ok := val.(map[string]interface{})
	if !ok {
		return
	}
	if s.cfg.Profiles == nil {
		s.cfg.Profiles = make(map[string]config.AIProviderProfile)
	}
	for providerID, profileVal := range raw {
		pm, ok := profileVal.(map[string]interface{})
		if !ok {
			continue
		}
		existing := s.cfg.Profiles[providerID]
		next := existing // 拷贝
		if v, ok := pm["api_base"].(string); ok {
			next.APIBase = v
		}
		if v, ok := pm["model"].(string); ok {
			next.Model = v
		}
		// api_key 留空时保留原值（避免前端只是显示掩码、未输入新值的情况覆盖）
		if v, ok := pm["api_key"].(string); ok && v != "" {
			next.APIKey = v
		}
		// V8：enabled / model_chain / current_model 合并
		if v, ok := pm["enabled"].(bool); ok {
			next.Enabled = v
		}
		if v, ok := pm["current_model"].(string); ok {
			next.CurrentModel = v
		}
		if v, ok := pm["model_chain"]; ok {
			// 接受 []string、[]interface{}（前端 JSON 传过来通常是 []interface{}）
			switch arr := v.(type) {
			case []string:
				next.ModelChain = append([]string{}, arr...)
			case []interface{}:
				out := make([]string, 0, len(arr))
				for _, item := range arr {
					if s, ok := item.(string); ok {
						s = strings.TrimSpace(s)
						if s != "" {
							out = append(out, s)
						}
					}
				}
				next.ModelChain = out
			}
		}
		s.cfg.Profiles[providerID] = next
	}
}

// syncActiveProfile 把当前激活 provider 的顶层字段同步到 profiles 表
// 确保「顶层即激活档案」的不变式。
//
// 注意：此函数应**保留**目标 profile 中的 Enabled / ModelChain / CurrentModel
// 等"非顶层管理"字段，仅同步 api_base / api_key / model。
func (s *AIService) syncActiveProfile() {
	if s.cfg.Provider == "" {
		return
	}
	if s.cfg.Profiles == nil {
		s.cfg.Profiles = make(map[string]config.AIProviderProfile)
	}
	existing := s.cfg.Profiles[s.cfg.Provider]
	existing.APIBase = s.cfg.APIBase
	existing.APIKey = s.cfg.APIKey
	existing.Model = s.cfg.Model
	// 激活的 provider 默认是 enabled（避免老的 enabled:false 卡住后续 failover 链）
	if !existing.Enabled {
		existing.Enabled = true
	}
	s.cfg.Profiles[s.cfg.Provider] = existing
}

// ==================== 连接测试 ====================

// TestConnection 测试 AI API 连接
// 注意：此方法绕过 IsEnabled() 检查，直接验证 API 密钥和网络连通性
func (s *AIService) TestConnection() (map[string]interface{}, error) {
	cfg := s.snapshotCfg()
	if cfg.APIKey == "" || cfg.APIBase == "" {
		return nil, fmt.Errorf("API Key 或 API Base 未配置")
	}

	start := time.Now()

	// 直接构建请求，绕过 ChatCompletion 中的 IsEnabled/预算检查
	reqBody := ChatCompletionRequest{
		Model: cfg.Model,
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

	apiURL := strings.TrimRight(cfg.APIBase, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

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
		"provider":   cfg.Provider,
		"model":      cfg.Model,
	}, nil
}

// ==================== 缓存管理 ====================

// ClearCache 清空所有 AI 缓存（包括内存和持久化）
func (s *AIService) ClearCache() int {
	s.cacheMu.Lock()
	count := len(s.cache)
	s.cache = make(map[string]*aiCacheEntry)
	s.cacheMu.Unlock()

	// 清空持久化缓存
	if s.cacheRepo != nil {
		dbCount, _ := s.cacheRepo.ClearAll()
		count += int(dbCount)
	}

	s.logger.Infof("AI 缓存已清空，共清理 %d 条", count)
	return count
}

// GetCacheStats 获取缓存统计（包括持久化层）
func (s *AIService) GetCacheStats() map[string]interface{} {
	s.cacheMu.RLock()
	total := len(s.cache)
	expired := 0
	now := time.Now()
	for _, entry := range s.cache {
		if now.After(entry.ExpiresAt) {
			expired++
		}
	}
	s.cacheMu.RUnlock()

	stats := map[string]interface{}{
		"memory_total":   total,
		"memory_active":  total - expired,
		"memory_expired": expired,
		"ttl_hours":      s.cfg.CacheTTLHours,
	}

	// 持久化缓存统计
	if s.cacheRepo != nil {
		dbTotal, _ := s.cacheRepo.Count()
		dbActive, _ := s.cacheRepo.CountActive()
		stats["db_total"] = dbTotal
		stats["db_active"] = dbActive
	}

	return stats
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
