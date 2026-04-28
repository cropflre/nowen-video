// Package service MGStage 数据源刮削器
// 借鉴自 mdcx-master 的 mgstage.py
// MGStage 是 MGS 系列番号的唯一可靠来源（200GANA、MGS 素人系列等）
// 访问限制：需要 age_check=1 cookie
package service

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// MGStage 正则
var (
	mgstageTitleRe    = regexp.MustCompile(`(?is)<h1[^>]*class="tag"[^>]*>([^<]+)</h1>`)
	mgstageCoverRe    = regexp.MustCompile(`(?is)<a[^>]+href="([^"]+\.jpg)"[^>]+id="EnlargeImage"`)
	mgstagePlotRe     = regexp.MustCompile(`(?is)<p[^>]+class="introduction"[^>]*>([\s\S]*?)</p>`)
	mgstageInfoRowRe  = regexp.MustCompile(`(?is)<tr>\s*<th[^>]*>([^<]+)</th>\s*<td[^>]*>([\s\S]*?)</td>\s*</tr>`)
	mgstageActressRe  = regexp.MustCompile(`(?is)<th[^>]*>出演[：:]?\s*</th>\s*<td[^>]*>([\s\S]*?)</td>`)
	mgstageGenreRe    = regexp.MustCompile(`(?is)<a[^>]+href="/search/cSearch/genre/[^"]+"[^>]*>([^<]+)</a>`)
	mgstageFanartRe   = regexp.MustCompile(`(?is)<a[^>]+href="([^"]+\.jpg)"[^>]+class="sample-image"`)
	mgstageTrailerRe  = regexp.MustCompile(`(?is)"url"\s*:\s*"([^"]+\.mp4)"`)
)

// scrapeMGStage 从 MGStage 刮削元数据
func (s *AdultScraperService) scrapeMGStage(code string) (*AdultMetadata, error) {
	baseURL := s.cfg.AdultScraper.MGStageURL
	if baseURL == "" {
		baseURL = "https://www.mgstage.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// MGStage 番号保留短横线，例如 200GANA-2801
	detailURL := fmt.Sprintf("%s/product/product_detail/%s/", baseURL, strings.ToUpper(code))
	html, err := s.fetchMGStageHTML(detailURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("mgstage 抓取失败: %w", err)
	}

	return s.parseMGStageHTML(html, code)
}

// fetchMGStageHTML 抓取 MGStage HTML（带 age_check=1 cookie）
func (s *AdultScraperService) fetchMGStageHTML(targetURL, referer string) (string, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", err
	}
	setAdultScraperHeaders(req)
	req.AddCookie(&http.Cookie{Name: "adc", Value: "1"})
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return doRequestReadString(s.client, req)
}

// parseMGStageHTML 解析 MGStage 详情页
func (s *AdultScraperService) parseMGStageHTML(html, code string) (*AdultMetadata, error) {
	meta := &AdultMetadata{
		Code:        code,
		Source:      "mgstage",
		Genres:      []string{},
		Actresses:   []string{},
		ActorPhotos: make(map[string]string),
		ExtraFanart: []string{},
	}

	// 标题
	if m := mgstageTitleRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Title = strings.TrimSpace(m[1])
	}

	// 封面
	if m := mgstageCoverRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Cover = cleanURL(m[1])
		meta.Thumb = strings.Replace(meta.Cover, "pb_e_", "pb_t_", 1)
	}

	// 简介
	if m := mgstagePlotRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Plot = strings.TrimSpace(stripHTMLTags(m[1]))
	}

	// 演员
	if m := mgstageActressRe.FindStringSubmatch(html); len(m) > 1 {
		inner := m[1]
		// 提取 <a> 标签中的名字
		aRe := regexp.MustCompile(`<a[^>]*>([^<]+)</a>`)
		for _, a := range aRe.FindAllStringSubmatch(inner, -1) {
			if len(a) > 1 {
				name := strings.TrimSpace(a[1])
				if name != "" {
					meta.Actresses = append(meta.Actresses, name)
				}
			}
		}
		// 若没 <a> 标签则直接用文本
		if len(meta.Actresses) == 0 {
			text := strings.TrimSpace(stripHTMLTags(inner))
			if text != "" && text != "----" {
				meta.Actresses = append(meta.Actresses, text)
			}
		}
	}

	// 类型标签
	seenGenre := make(map[string]struct{})
	for _, m := range mgstageGenreRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			g := strings.TrimSpace(m[1])
			if g == "" {
				continue
			}
			if _, ok := seenGenre[g]; !ok {
				seenGenre[g] = struct{}{}
				meta.Genres = append(meta.Genres, g)
			}
		}
	}

	// 信息行表格
	for _, m := range mgstageInfoRowRe.FindAllStringSubmatch(html, -1) {
		if len(m) < 3 {
			continue
		}
		key := strings.TrimSpace(stripHTMLTags(m[1]))
		val := strings.TrimSpace(stripHTMLTags(m[2]))
		if val == "----" || val == "" {
			continue
		}
		switch {
		case strings.Contains(key, "配信開始日") || strings.Contains(key, "商品発売日"):
			meta.ReleaseDate = normalizeFanzaDate(val)
		case strings.Contains(key, "収録時間"):
			dur := regexp.MustCompile(`\d+`).FindString(val)
			if dur != "" {
				fmt.Sscanf(dur, "%d", &meta.Duration)
			}
		case strings.Contains(key, "メーカー"):
			meta.Studio = val
		case strings.Contains(key, "レーベル"):
			meta.Label = val
		case strings.Contains(key, "シリーズ"):
			meta.Series = val
		case strings.Contains(key, "評価"):
			// MGStage 不常有评分字段，预留
		}
	}

	// 剧照
	for _, m := range mgstageFanartRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			meta.ExtraFanart = append(meta.ExtraFanart, cleanURL(m[1]))
		}
	}

	// Trailer（嵌入 JSON 中的 mp4 URL）
	if m := mgstageTrailerRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Trailer = cleanURL(m[1])
	}

	if meta.Title == "" {
		return nil, fmt.Errorf("mgstage 解析失败：未提取到标题（番号可能不存在）")
	}
	return meta, nil
}
