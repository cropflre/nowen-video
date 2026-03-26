package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
)

// AIMetadataResult AI 生成的元数据
type AIMetadataResult struct {
	Overview string `json:"overview"` // 简介
	Genres   string `json:"genres"`   // 类型标签（逗号分隔）
	Year     int    `json:"year"`     // 年份（如果能推断）
	Country  string `json:"country"`  // 制片国家
	Tagline  string `json:"tagline"`  // 宣传语
}

const metadataEnhanceSystemPrompt = `你是一个电影/电视剧元数据专家。用户会给你一个影视作品的标题（可能包含年份和其他信息），
你需要根据你的知识为这部作品生成元数据信息。

请严格返回以下 JSON 格式（不要包含任何其他文字）：
{
  "overview": "作品简介（100-200字，中文）",
  "genres": "类型标签（逗号分隔，如：动作,科幻,冒险）",
  "year": 0,
  "country": "制片国家（如：美国、中国、日本）",
  "tagline": "一句话宣传语（中文，20字以内）"
}

规则：
1. overview 必须填写，即使你不确定具体内容，也要根据标题推测合理的简介
2. genres 使用中文类型标签，常见类型：动作、冒险、动画、喜剧、犯罪、纪录、剧情、家庭、奇幻、历史、恐怖、音乐、悬疑、爱情、科幻、惊悚、战争、西部
3. year 如果标题中包含年份信息则提取，否则根据你的知识填写，不确定则填 0
4. country 不确定则留空
5. tagline 简短有力的宣传语，不确定则留空
6. 如果你完全不认识这部作品，仍然要根据标题中的线索（如关键词、语言等）做出合理推测`

// EnrichMetadata 使用 AI 为媒体生成/补充元数据
// 作为 TMDb → 豆瓣 → Bangumi 之后的第四层 Fallback
func (s *AIService) EnrichMetadata(media *model.Media, searchTitle string) error {
	if !s.IsMetadataEnhanceEnabled() {
		return fmt.Errorf("AI 元数据增强未启用")
	}

	// 检查缓存
	cacheKey := "metadata:" + strings.ToLower(strings.TrimSpace(searchTitle))
	if cached, ok := s.GetCache(cacheKey); ok {
		var result AIMetadataResult
		if err := json.Unmarshal([]byte(cached), &result); err == nil {
			s.applyAIMetadata(media, &result)
			return nil
		}
	}

	// 构建用户提示
	userPrompt := fmt.Sprintf("请为以下影视作品生成元数据：\n标题: %s", searchTitle)
	if media.Year > 0 {
		userPrompt += fmt.Sprintf("\n年份: %d", media.Year)
	}
	if media.MediaType == "episode" {
		userPrompt += "\n类型: 电视剧/剧集"
	} else {
		userPrompt += "\n类型: 电影"
	}
	if media.Resolution != "" {
		userPrompt += fmt.Sprintf("\n分辨率: %s", media.Resolution)
	}

	// 调用 LLM
	result, err := s.ChatCompletion(
		metadataEnhanceSystemPrompt,
		userPrompt,
		0.3, // 适中温度，允许一定创造性
		500,
	)
	if err != nil {
		return fmt.Errorf("AI 元数据生成失败: %w", err)
	}

	// 解析 JSON
	result = cleanJSONResponse(result)

	var metadata AIMetadataResult
	if err := json.Unmarshal([]byte(result), &metadata); err != nil {
		return fmt.Errorf("AI 元数据 JSON 解析失败: %w", err)
	}

	// 应用到媒体
	s.applyAIMetadata(media, &metadata)

	// 写入缓存
	if cacheBytes, err := json.Marshal(metadata); err == nil {
		s.SetCache(cacheKey, string(cacheBytes))
	}

	s.logger.Infof("AI 元数据增强成功: %s → genres=%s, year=%d", searchTitle, metadata.Genres, metadata.Year)
	return nil
}

// applyAIMetadata 将 AI 生成的元数据应用到媒体（只补充缺失字段，不覆盖已有数据）
func (s *AIService) applyAIMetadata(media *model.Media, result *AIMetadataResult) {
	if media.Overview == "" && result.Overview != "" {
		media.Overview = result.Overview
	}
	if media.Genres == "" && result.Genres != "" {
		media.Genres = result.Genres
	}
	if media.Year == 0 && result.Year > 0 {
		media.Year = result.Year
	}
	if media.Country == "" && result.Country != "" {
		media.Country = result.Country
	}
	if media.Tagline == "" && result.Tagline != "" {
		media.Tagline = result.Tagline
	}
}

// EnrichSeriesMetadata 使用 AI 为剧集合集生成/补充元数据
func (s *AIService) EnrichSeriesMetadata(series *model.Series, searchTitle string) error {
	if !s.IsMetadataEnhanceEnabled() {
		return fmt.Errorf("AI 元数据增强未启用")
	}

	// 检查缓存
	cacheKey := "metadata:series:" + strings.ToLower(strings.TrimSpace(searchTitle))
	if cached, ok := s.GetCache(cacheKey); ok {
		var result AIMetadataResult
		if err := json.Unmarshal([]byte(cached), &result); err == nil {
			s.applyAISeriesMetadata(series, &result)
			return nil
		}
	}

	userPrompt := fmt.Sprintf("请为以下电视剧/剧集生成元数据：\n标题: %s", searchTitle)
	if series.Year > 0 {
		userPrompt += fmt.Sprintf("\n年份: %d", series.Year)
	}
	if series.EpisodeCount > 0 {
		userPrompt += fmt.Sprintf("\n集数: %d", series.EpisodeCount)
	}
	if series.SeasonCount > 0 {
		userPrompt += fmt.Sprintf("\n季数: %d", series.SeasonCount)
	}

	result, err := s.ChatCompletion(
		metadataEnhanceSystemPrompt,
		userPrompt,
		0.3,
		500,
	)
	if err != nil {
		return fmt.Errorf("AI 剧集元数据生成失败: %w", err)
	}

	result = cleanJSONResponse(result)

	var metadata AIMetadataResult
	if err := json.Unmarshal([]byte(result), &metadata); err != nil {
		return fmt.Errorf("AI 剧集元数据 JSON 解析失败: %w", err)
	}

	s.applyAISeriesMetadata(series, &metadata)

	if cacheBytes, err := json.Marshal(metadata); err == nil {
		s.SetCache(cacheKey, string(cacheBytes))
	}

	s.logger.Infof("AI 剧集元数据增强成功: %s → genres=%s", searchTitle, metadata.Genres)
	return nil
}

// applyAISeriesMetadata 将 AI 生成的元数据应用到剧集合集
func (s *AIService) applyAISeriesMetadata(series *model.Series, result *AIMetadataResult) {
	if series.Overview == "" && result.Overview != "" {
		series.Overview = result.Overview
	}
	if series.Genres == "" && result.Genres != "" {
		series.Genres = result.Genres
	}
	if series.Year == 0 && result.Year > 0 {
		series.Year = result.Year
	}
	if series.Country == "" && result.Country != "" {
		series.Country = result.Country
	}
}
