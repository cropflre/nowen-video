package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SearchIntent AI 解析后的搜索意图
type SearchIntent struct {
	// 提取的搜索关键词（电影/剧集名称）
	Query string `json:"query"`
	// 媒体类型筛选: movie / episode / ""(不限)
	MediaType string `json:"media_type,omitempty"`
	// 类型标签筛选（如 "科幻", "动作"）
	Genre string `json:"genre,omitempty"`
	// 年份范围
	YearMin int `json:"year_min,omitempty"`
	YearMax int `json:"year_max,omitempty"`
	// 最低评分
	MinRating float64 `json:"min_rating,omitempty"`
	// 排序方式: relevance / rating_desc / year_desc / year_asc
	SortBy string `json:"sort_by,omitempty"`
	// 是否为自然语言查询（true 表示 AI 已解析，false 表示直接透传）
	Parsed bool `json:"parsed"`
}

const smartSearchSystemPrompt = `你是一个视频媒体库的搜索助手。用户会用自然语言描述他们想找的电影或电视剧。
你的任务是将用户的自然语言查询解析为结构化的搜索参数。

请严格返回以下 JSON 格式（不要包含任何其他文字）：
{
  "query": "提取的搜索关键词（电影/剧集名称，尽量精确）",
  "media_type": "movie 或 episode 或空字符串（不确定时留空）",
  "genre": "类型标签（如 科幻、动作、喜剧、恐怖、爱情、动画等，不确定时留空）",
  "year_min": 0,
  "year_max": 0,
  "min_rating": 0,
  "sort_by": "relevance"
}

规则：
1. query 字段必须填写，提取用户想搜索的核心关键词
2. 如果用户提到了具体的电影/剧集名称，直接使用该名称
3. 如果用户描述了模糊的特征（如"那部太空电影"），尝试推断可能的名称
4. year_min/year_max 为 0 表示不限制
5. min_rating 为 0 表示不限制，范围 0-10
6. sort_by 可选值: relevance(相关度) / rating_desc(评分最高) / year_desc(最新) / year_asc(最早)
7. 如果用户的查询已经是明确的电影名称，只需填 query 字段，其他留空/默认`

// ParseSearchIntent 使用 AI 解析自然语言搜索查询
func (s *AIService) ParseSearchIntent(userQuery string) (*SearchIntent, error) {
	if !s.IsSmartSearchEnabled() {
		// AI 未启用，返回原始查询
		return &SearchIntent{
			Query:  userQuery,
			Parsed: false,
		}, nil
	}

	// 短查询（<=4个字符）大概率是精确搜索，不需要 AI 解析
	if len([]rune(userQuery)) <= 4 {
		return &SearchIntent{
			Query:  userQuery,
			Parsed: false,
		}, nil
	}

	// 检查缓存
	cacheKey := "search:" + strings.ToLower(strings.TrimSpace(userQuery))
	if cached, ok := s.GetCache(cacheKey); ok {
		var intent SearchIntent
		if err := json.Unmarshal([]byte(cached), &intent); err == nil {
			intent.Parsed = true
			return &intent, nil
		}
	}

	// 调用 LLM
	result, err := s.ChatCompletion(
		smartSearchSystemPrompt,
		userQuery,
		0.1, // 低温度，确保输出稳定
		200,
	)
	if err != nil {
		s.logger.Warnf("AI 搜索意图解析失败，降级为原始查询: %v", err)
		return &SearchIntent{
			Query:  userQuery,
			Parsed: false,
		}, nil
	}

	// 解析 JSON 响应
	// 清理可能的 markdown 代码块包裹
	result = cleanJSONResponse(result)

	var intent SearchIntent
	if err := json.Unmarshal([]byte(result), &intent); err != nil {
		s.logger.Warnf("AI 搜索意图解析 JSON 失败: %v, 原始响应: %s", err, result)
		return &SearchIntent{
			Query:  userQuery,
			Parsed: false,
		}, nil
	}

	// 验证必填字段
	if intent.Query == "" {
		intent.Query = userQuery
	}
	intent.Parsed = true

	// 写入缓存
	if cacheBytes, err := json.Marshal(intent); err == nil {
		s.SetCache(cacheKey, string(cacheBytes))
	}

	s.logger.Infof("AI 搜索意图解析: '%s' → query='%s', type='%s', genre='%s', year=[%d,%d], rating>=%.1f",
		userQuery, intent.Query, intent.MediaType, intent.Genre, intent.YearMin, intent.YearMax, intent.MinRating)

	return &intent, nil
}

// cleanJSONResponse 清理 LLM 返回的 JSON（去除 markdown 代码块等）
func cleanJSONResponse(s string) string {
	s = strings.TrimSpace(s)
	// 去除 ```json ... ``` 包裹
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) >= 3 {
			// 去掉第一行和最后一行
			s = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	s = strings.TrimSpace(s)
	// 确保是 JSON 对象
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		s = s[start : end+1]
	}
	return s
}

// FormatSearchIntentDebug 格式化搜索意图为调试字符串
func (intent *SearchIntent) FormatSearchIntentDebug() string {
	parts := []string{fmt.Sprintf("query=%q", intent.Query)}
	if intent.MediaType != "" {
		parts = append(parts, fmt.Sprintf("type=%s", intent.MediaType))
	}
	if intent.Genre != "" {
		parts = append(parts, fmt.Sprintf("genre=%s", intent.Genre))
	}
	if intent.YearMin > 0 || intent.YearMax > 0 {
		parts = append(parts, fmt.Sprintf("year=[%d,%d]", intent.YearMin, intent.YearMax))
	}
	if intent.MinRating > 0 {
		parts = append(parts, fmt.Sprintf("rating>=%.1f", intent.MinRating))
	}
	if intent.SortBy != "" && intent.SortBy != "relevance" {
		parts = append(parts, fmt.Sprintf("sort=%s", intent.SortBy))
	}
	return strings.Join(parts, ", ")
}
