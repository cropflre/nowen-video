// Package service JAV321 数据源刮削器
// 借鉴自 mdcx-master 项目的 jav321.py 实现
// JAV321 特点：直接通过 POST 搜索，中文简介丰富
package service

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// JAV321 正则表达式
var (
	jav321TitleRe    = regexp.MustCompile(`(?is)<div[^>]*class="panel-heading"[^>]*>\s*<h3>([^<]+)</h3>`)
	jav321InfoRe     = regexp.MustCompile(`(?is)<b>([^<:：]+)</b>\s*[:：]\s*([^<]+?)<`)
	jav321CoverRe    = regexp.MustCompile(`(?is)<img[^>]+src="([^"]+\.(?:jpg|jpeg|png|webp))"[^>]+class="img-responsive"`)
	jav321PlotRe     = regexp.MustCompile(`(?is)<div[^>]+class="panel-body"[^>]*>(.*?)</div>`)
	jav321ActressRe  = regexp.MustCompile(`(?is)<a[^>]+href="/star/[^"]+"[^>]*>([^<]+)</a>`)
	jav321GenreRe    = regexp.MustCompile(`(?is)<a[^>]+href="/genre/[^"]+"[^>]*>([^<]+)</a>`)
	jav321FanartRe   = regexp.MustCompile(`(?is)<a[^>]+href="([^"]+)"[^>]+class="sample-box"`)
	jav321TrailerRe  = regexp.MustCompile(`(?is)<source[^>]+src="([^"]+)"`)
)

// scrapeJav321 从 JAV321 刮削番号元数据（POST 搜索 + 详情页一步到位）
func (s *AdultScraperService) scrapeJav321(code string) (*AdultMetadata, error) {
	baseURL := s.cfg.AdultScraper.Jav321URL
	if baseURL == "" {
		baseURL = "https://www.jav321.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// JAV321 使用 POST 表单搜索，成功会直接 302 到详情页（或返回详情 HTML）
	searchURL := baseURL + "/search"
	formData := url.Values{}
	formData.Set("sn", code)

	// 使用统一 POST 辅助方法
	html, err := s.postFormForAdult(searchURL, baseURL, formData)
	if err != nil {
		return nil, fmt.Errorf("jav321 搜索失败: %w", err)
	}

	return s.parseJav321HTML(html, code)
}

// parseJav321HTML 解析 JAV321 详情页 HTML
func (s *AdultScraperService) parseJav321HTML(html, code string) (*AdultMetadata, error) {
	meta := &AdultMetadata{
		Code:        code,
		Source:      "jav321",
		Genres:      []string{},
		Actresses:   []string{},
		ActorPhotos: make(map[string]string),
		ExtraFanart: []string{},
	}

	// 标题
	if m := jav321TitleRe.FindStringSubmatch(html); len(m) > 1 {
		title := strings.TrimSpace(stripHTMLTags(m[1]))
		// 标题一般是 "CODE Title"，去除 CODE 前缀
		title = strings.TrimPrefix(title, code)
		title = strings.TrimPrefix(title, strings.ToUpper(code))
		meta.Title = strings.TrimSpace(title)
	}

	// 封面
	if m := jav321CoverRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Cover = cleanURL(m[1])
	}

	// 解析键值对信息块（发行日期/时长/片商/发行商/系列/导演等）
	for _, m := range jav321InfoRe.FindAllStringSubmatch(html, -1) {
		if len(m) < 3 {
			continue
		}
		key := strings.TrimSpace(m[1])
		value := strings.TrimSpace(stripHTMLTags(m[2]))
		if value == "" {
			continue
		}
		switch {
		case strings.Contains(key, "发行日期") || strings.Contains(key, "配信開始日"):
			meta.ReleaseDate = value
		case strings.Contains(key, "播放时长") || strings.Contains(key, "收录时间") || strings.Contains(key, "収録時間"):
			fmt.Sscanf(value, "%d", &meta.Duration)
		case strings.Contains(key, "制作商") || strings.Contains(key, "メーカー") || strings.Contains(key, "片商"):
			meta.Studio = value
		case strings.Contains(key, "发行商") || strings.Contains(key, "レーベル"):
			meta.Label = value
		case strings.Contains(key, "系列") || strings.Contains(key, "シリーズ"):
			meta.Series = value
		case strings.Contains(key, "导演") || strings.Contains(key, "監督"):
			meta.Director = value
		case strings.Contains(key, "平均评价") || strings.Contains(key, "ユーザー評価"):
			// 提取评分数字
			re := regexp.MustCompile(`([\d.]+)`)
			if mm := re.FindStringSubmatch(value); len(mm) > 1 {
				fmt.Sscanf(mm[1], "%f", &meta.Rating)
			}
		}
	}

	// 演员
	seenActress := make(map[string]struct{})
	for _, m := range jav321ActressRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			name := strings.TrimSpace(m[1])
			if _, ok := seenActress[name]; !ok && name != "" {
				seenActress[name] = struct{}{}
				meta.Actresses = append(meta.Actresses, name)
			}
		}
	}

	// 类型
	seenGenre := make(map[string]struct{})
	for _, m := range jav321GenreRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			g := strings.TrimSpace(m[1])
			if _, ok := seenGenre[g]; !ok && g != "" {
				seenGenre[g] = struct{}{}
				meta.Genres = append(meta.Genres, g)
			}
		}
	}

	// Extra Fanart
	for _, m := range jav321FanartRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			meta.ExtraFanart = append(meta.ExtraFanart, cleanURL(m[1]))
		}
	}

	// Trailer
	if m := jav321TrailerRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Trailer = cleanURL(m[1])
	}

	// 简介：panel-body 内第一段非空文本
	if mm := jav321PlotRe.FindAllStringSubmatch(html, -1); len(mm) > 0 {
		for _, match := range mm {
			if len(match) > 1 {
				text := strings.TrimSpace(stripHTMLTags(match[1]))
				text = strings.TrimSpace(text)
				if len(text) > 20 && !strings.Contains(text, "发行日期") {
					meta.Plot = text
					break
				}
			}
		}
	}

	if meta.Title == "" {
		return nil, fmt.Errorf("jav321 解析失败：未找到标题，可能番号不存在")
	}
	return meta, nil
}
