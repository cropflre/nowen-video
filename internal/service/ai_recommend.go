package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
)

// AIRecommendReason AI 生成的推荐理由
type AIRecommendReason struct {
	Reason string `json:"reason"` // 推荐理由（一句话）
}

const recommendReasonSystemPrompt = `你是一个影视推荐助手。根据用户的观影偏好和推荐的影片信息，生成一句简短、有吸引力的推荐理由。

请严格返回以下 JSON 格式（不要包含任何其他文字）：
{
  "reason": "推荐理由（15-30字，中文，要有吸引力和个性化感觉）"
}

规则：
1. 推荐理由要简短有力，像朋友推荐一样自然
2. 可以提及影片的亮点（类型、评分、导演、特色等）
3. 如果知道用户喜欢的类型，可以关联用户偏好
4. 避免使用"推荐"这个词，用更自然的表达
5. 示例：
   - "高分科幻佳作，烧脑剧情让人欲罢不能"
   - "豆瓣 9.0 的治愈系动画，适合周末放松"
   - "你喜欢悬疑片？这部绝对不会让你失望"
   - "今年最佳动作片，视觉效果炸裂"`

// GenerateRecommendReason 使用 AI 为推荐结果生成个性化理由
func (s *AIService) GenerateRecommendReason(media *model.Media, userGenres []string, originalReason string) string {
	if !s.IsRecommendReasonEnabled() {
		return originalReason
	}

	// 检查缓存（基于媒体ID + 用户偏好类型）
	genreKey := strings.Join(userGenres, ",")
	cacheKey := fmt.Sprintf("reason:%s:%s", media.ID, genreKey)
	if cached, ok := s.GetCache(cacheKey); ok {
		return cached
	}

	// 构建用户提示
	var parts []string
	parts = append(parts, fmt.Sprintf("影片: %s", media.Title))
	if media.Year > 0 {
		parts = append(parts, fmt.Sprintf("年份: %d", media.Year))
	}
	if media.Genres != "" {
		parts = append(parts, fmt.Sprintf("类型: %s", media.Genres))
	}
	if media.Rating > 0 {
		parts = append(parts, fmt.Sprintf("评分: %.1f", media.Rating))
	}
	if media.Overview != "" {
		// 截取简介前100字
		overview := media.Overview
		runes := []rune(overview)
		if len(runes) > 100 {
			overview = string(runes[:100]) + "..."
		}
		parts = append(parts, fmt.Sprintf("简介: %s", overview))
	}
	if len(userGenres) > 0 {
		parts = append(parts, fmt.Sprintf("用户喜欢的类型: %s", strings.Join(userGenres, "、")))
	}

	userPrompt := strings.Join(parts, "\n")

	// 调用 LLM
	result, err := s.ChatCompletion(
		recommendReasonSystemPrompt,
		userPrompt,
		0.7, // 较高温度，增加创造性
		100,
	)
	if err != nil {
		s.logger.Debugf("AI 推荐理由生成失败: %v", err)
		return originalReason
	}

	// 解析 JSON
	result = cleanJSONResponse(result)

	var reason AIRecommendReason
	if err := json.Unmarshal([]byte(result), &reason); err != nil {
		// 如果不是 JSON，直接使用原始文本
		reason.Reason = strings.TrimSpace(result)
		// 去除可能的引号
		reason.Reason = strings.Trim(reason.Reason, "\"'")
	}

	if reason.Reason == "" {
		return originalReason
	}

	// 写入缓存
	s.SetCache(cacheKey, reason.Reason)

	return reason.Reason
}

// BatchGenerateRecommendReasons 批量为推荐结果生成 AI 理由
// 为了控制 API 调用次数，只为前 N 个结果生成
func (s *AIService) BatchGenerateRecommendReasons(recommendations []RecommendedMedia, userGenres []string, maxCount int) []RecommendedMedia {
	if !s.IsRecommendReasonEnabled() || len(recommendations) == 0 {
		return recommendations
	}

	if maxCount <= 0 {
		maxCount = 5 // 默认只为前5个生成AI理由
	}
	if maxCount > len(recommendations) {
		maxCount = len(recommendations)
	}

	for i := 0; i < maxCount; i++ {
		aiReason := s.GenerateRecommendReason(
			&recommendations[i].Media,
			userGenres,
			recommendations[i].Reason,
		)
		recommendations[i].Reason = aiReason
	}

	return recommendations
}
