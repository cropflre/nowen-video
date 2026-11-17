package service

import (
	"errors"
	"fmt"
	"strings"

	llmprices "github.com/Mattbusel/llm-prices/go"
)

// ==================== AI 模型 Catalog ====================
//
// 该文件提供两件事：
//   1. AIModelCatalog：内置主流 LLM 模型的元数据 + 定价（USD per 1K tokens）；
//   2. AICostService：基于 catalog 给出"单次调用预估"和"累计花费"。
//
// 设计原则：
//   - 静态 catalog 优先（云厂商定价相对稳定）；价格用美元（USD），前端按汇率展示 CNY；
//   - 价格全部存"每 1K tokens 美元"，方便心算（gpt-4o-mini ≈ $0.00015 / 1K input）；
//   - 未在 catalog 中的 model 仍可估价（fallback 到 provider 默认价 / 0），不报错。
//
// 注意：定价以 2024-11 各厂商公开价为准，可能滞后；用户可在前端覆盖（后续扩展）。

// AIModelInfo 单个模型的展示 + 计费信息
type AIModelInfo struct {
	Provider      string  `json:"provider"`        // openai / deepseek / qwen / zhipu / anthropic / local
	ID            string  `json:"id"`              // 调用时填到 cfg.Model 的字符串
	Label         string  `json:"label"`           // UI 显示的友好名
	ContextLength int     `json:"context_length"`  // 上下文窗口（tokens）
	InputPrice    float64 `json:"input_price"`     // USD per 1K input tokens
	OutputPrice   float64 `json:"output_price"`    // USD per 1K output tokens
	Recommended   bool    `json:"recommended"`     // 是否在该 provider 下推荐（前端高亮 / 默认选）
	Notes         string  `json:"notes,omitempty"` // 说明（中文）
}

// builtinAIModelCatalog 内置主流模型清单（USD per 1K tokens）
//
// 数据来源：各厂商官网公开定价（2024-11）。
// 更新策略：每季度人工核对；前端如发现价差可由用户在配置中覆盖（后续）。
var builtinAIModelCatalog = []AIModelInfo{
	// ===== OpenAI =====
	{Provider: "openai", ID: "gpt-4o-mini", Label: "GPT-4o mini", ContextLength: 128000, InputPrice: 0.00015, OutputPrice: 0.0006, Recommended: true, Notes: "性价比最高，懒人入库默认推荐"},
	{Provider: "openai", ID: "gpt-4o", Label: "GPT-4o", ContextLength: 128000, InputPrice: 0.0025, OutputPrice: 0.01, Notes: "通用旗舰，识别效果好"},
	{Provider: "openai", ID: "gpt-4-turbo", Label: "GPT-4 Turbo", ContextLength: 128000, InputPrice: 0.01, OutputPrice: 0.03, Notes: "老牌旗舰，价格偏高"},
	{Provider: "openai", ID: "gpt-3.5-turbo", Label: "GPT-3.5 Turbo", ContextLength: 16385, InputPrice: 0.0005, OutputPrice: 0.0015, Notes: "便宜但识别效果一般"},

	// ===== DeepSeek =====
	{Provider: "deepseek", ID: "deepseek-chat", Label: "DeepSeek-V3 Chat", ContextLength: 64000, InputPrice: 0.00014, OutputPrice: 0.00028, Recommended: true, Notes: "国产性价比之王"},
	{Provider: "deepseek", ID: "deepseek-reasoner", Label: "DeepSeek-R1 Reasoner", ContextLength: 64000, InputPrice: 0.00055, OutputPrice: 0.00219, Notes: "深度推理，慢但准"},

	// ===== Qwen / 阿里通义 =====
	{Provider: "qwen", ID: "qwen-plus", Label: "通义千问 Plus", ContextLength: 131072, InputPrice: 0.00056, OutputPrice: 0.00168, Recommended: true, Notes: "中文识别效果好"},
	{Provider: "qwen", ID: "qwen-turbo", Label: "通义千问 Turbo", ContextLength: 8192, InputPrice: 0.00042, OutputPrice: 0.00084, Notes: "快、便宜"},
	{Provider: "qwen", ID: "qwen-max", Label: "通义千问 Max", ContextLength: 32768, InputPrice: 0.0028, OutputPrice: 0.0084, Notes: "旗舰，价格偏高"},

	// ===== 智谱 GLM =====
	{Provider: "zhipu", ID: "glm-4-flash", Label: "GLM-4 Flash", ContextLength: 128000, InputPrice: 0.0, OutputPrice: 0.0, Recommended: true, Notes: "免费版（限速），适合个人玩家"},
	{Provider: "zhipu", ID: "glm-4-air", Label: "GLM-4 Air", ContextLength: 128000, InputPrice: 0.00014, OutputPrice: 0.00014, Notes: "便宜稳定"},
	{Provider: "zhipu", ID: "glm-4-plus", Label: "GLM-4 Plus", ContextLength: 128000, InputPrice: 0.0070, OutputPrice: 0.0070, Notes: "旗舰"},

	// ===== Anthropic Claude =====
	{Provider: "anthropic", ID: "claude-3-5-haiku-20241022", Label: "Claude 3.5 Haiku", ContextLength: 200000, InputPrice: 0.0008, OutputPrice: 0.004, Recommended: true, Notes: "快、便宜、上下文长"},
	{Provider: "anthropic", ID: "claude-3-5-sonnet-20241022", Label: "Claude 3.5 Sonnet", ContextLength: 200000, InputPrice: 0.003, OutputPrice: 0.015, Notes: "通用旗舰"},

	// ===== 本地（占位，价格 0） =====
	{Provider: "local", ID: "ollama", Label: "本地 Ollama", ContextLength: 32768, InputPrice: 0, OutputPrice: 0, Notes: "本地推理，无 API 费用"},
}

// providerFallbackPrice 当 model 不在 catalog 时给一个保守估价（按 provider 平均）
var providerFallbackPrice = map[string]struct{ In, Out float64 }{
	"openai":    {0.0025, 0.01},
	"deepseek":  {0.0003, 0.001},
	"qwen":      {0.001, 0.003},
	"zhipu":     {0.0005, 0.0015},
	"anthropic": {0.003, 0.015},
	"local":     {0, 0},
}

// ==================== Service ====================

// AICostService AI 成本估算服务
//
// 依赖 AIService 来取实时 token 累计；不持有锁，所有数据读操作走 AIService.GetStatus()。
type AICostService struct {
	ai *AIService
}

// NewAICostService 构造
func NewAICostService(ai *AIService) *AICostService {
	return &AICostService{ai: ai}
}

// ListModels 列出所有内置模型；可按 provider 过滤
//
// provider 大小写不敏感；空字符串表示返回全部。
func (s *AICostService) ListModels(provider string) []AIModelInfo {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		out := make([]AIModelInfo, len(builtinAIModelCatalog))
		copy(out, builtinAIModelCatalog)
		return out
	}
	out := make([]AIModelInfo, 0, 8)
	for _, m := range builtinAIModelCatalog {
		if m.Provider == provider {
			out = append(out, m)
		}
	}
	return out
}

// FindModel 根据 (provider, modelID) 精确查找；找不到返回 nil。
//
// provider 可空，空时仅按 modelID 查找（取第一个匹配）。
func (s *AICostService) FindModel(provider, modelID string) *AIModelInfo {
	provider = strings.ToLower(strings.TrimSpace(provider))
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return nil
	}
	for i := range builtinAIModelCatalog {
		m := &builtinAIModelCatalog[i]
		if m.ID != modelID {
			continue
		}
		if provider != "" && m.Provider != provider {
			continue
		}
		// 返回副本（避免外部修改 catalog）
		cp := *m
		return &cp
	}
	return nil
}

// CostEstimate 单次调用估价结果
type CostEstimate struct {
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	InputPrice       float64 `json:"input_price"`  // USD/1K
	OutputPrice      float64 `json:"output_price"` // USD/1K
	CostUSD          float64 `json:"cost_usd"`
	CostCNY          float64 `json:"cost_cny"` // 按 USD→CNY 汇率换算
	UsdToCnyRate     float64 `json:"usd_to_cny_rate"`
	Fallback         bool    `json:"fallback"` // 是否走了 provider fallback 估价
	Note             string  `json:"note,omitempty"`
}

// usdToCnyRate 美元到人民币汇率（保守估计 1 USD ≈ 7.2 CNY）。
//
// 实际可改为读配置；这里硬编码避免引入外部 API。
const usdToCnyRate = 7.2

// Estimate 给定 provider+model+tokens 用量，返回估价（USD + CNY）
func (s *AICostService) Estimate(provider, modelID string, promptTokens, completionTokens int) (*CostEstimate, error) {
	if promptTokens < 0 || completionTokens < 0 {
		return nil, errors.New("tokens 不能为负数")
	}

	est := &CostEstimate{
		Provider:         strings.ToLower(strings.TrimSpace(provider)),
		Model:            strings.TrimSpace(modelID),
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		UsdToCnyRate:     usdToCnyRate,
	}

	if m := s.FindModel(est.Provider, est.Model); m != nil {
		est.InputPrice = m.InputPrice
		est.OutputPrice = m.OutputPrice
		if est.Provider == "" {
			est.Provider = m.Provider
		}
	} else if in, out, checked, ok := listedPrice(est.Provider, est.Model); ok {
		// catalog 外的 OpenAI / Anthropic 型号：按官方标价（llm-prices，离线数据）估算
		est.InputPrice = in
		est.OutputPrice = out
		est.Note = fmt.Sprintf("模型 %q 不在内置 catalog，已按官方标价估算（%s 核对）", est.Model, checked)
	} else {
		// fallback：按 provider 平均价
		if p, ok := providerFallbackPrice[est.Provider]; ok {
			est.InputPrice = p.In
			est.OutputPrice = p.Out
			est.Fallback = true
			est.Note = fmt.Sprintf("模型 %q 不在内置 catalog，已按 %s 平均价估算", est.Model, est.Provider)
		} else {
			// 完全未知 provider：返回 0 + 提示
			est.Fallback = true
			est.Note = fmt.Sprintf("未知 provider=%q model=%q，无法估价", est.Provider, est.Model)
		}
	}

	est.CostUSD = float64(promptTokens)/1000.0*est.InputPrice +
		float64(completionTokens)/1000.0*est.OutputPrice
	est.CostCNY = est.CostUSD * usdToCnyRate
	return est, nil
}

// listedPrice 返回 OpenAI / Anthropic 型号的官方标价（USD per 1K tokens）及核对日期。
// 数据来自 llm-prices：编译进二进制、不联网，每行注明官方价格页与核对日期。
func listedPrice(provider, modelID string) (in, out float64, checked string, ok bool) {
	if provider != "openai" && provider != "anthropic" {
		return 0, 0, "", false
	}
	p, found := llmprices.LookupProvider(provider, modelID)
	if !found || p.Input == nil || p.Output == nil {
		return 0, 0, "", false
	}
	return *p.Input / 1000, *p.Output / 1000, p.Checked, true
}

// CostSummary 累计花费汇总（取自 AIService 的运行期统计）
type CostSummary struct {
	Provider          string       `json:"provider"`
	Model             string       `json:"model"`
	MonthlyCalls      int64        `json:"monthly_calls"` // 当月调用次数（来自 AIService.GetStatus）
	TotalPromptTokens int64        `json:"total_prompt_tokens"`
	TotalCompletion   int64        `json:"total_completion_tokens"`
	Estimate          CostEstimate `json:"estimate"`
	CurrentModelInfo  *AIModelInfo `json:"current_model_info,omitempty"`
}

// Summary 返回当前 provider/model 下，AIService 累计 token 估算出的总花费。
//
// 数据来自 AIService.GetStatus()（包含 monthly_calls / total_prompt_tokens / total_completion_tokens
// / provider / model）。如 AIService 未注入，则返回零值。
func (s *AICostService) Summary() (*CostSummary, error) {
	if s.ai == nil {
		return nil, errors.New("AIService 未注入")
	}
	st := s.ai.GetStatus()

	sum := &CostSummary{
		Provider:          asString(st["provider"]),
		Model:             asString(st["model"]),
		MonthlyCalls:      asInt64(st["monthly_calls"]),
		TotalPromptTokens: asInt64(st["total_prompt_tokens"]),
		TotalCompletion:   asInt64(st["total_completion_tokens"]),
	}
	est, err := s.Estimate(sum.Provider, sum.Model, int(sum.TotalPromptTokens), int(sum.TotalCompletion))
	if err == nil && est != nil {
		sum.Estimate = *est
	}
	if m := s.FindModel(sum.Provider, sum.Model); m != nil {
		sum.CurrentModelInfo = m
	}
	return sum, nil
}

// ==================== 小工具 ====================

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func asInt64(v interface{}) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case uint64:
		return int64(x)
	case float64:
		return int64(x)
	default:
		return 0
	}
}
