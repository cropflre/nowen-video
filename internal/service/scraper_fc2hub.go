// Package service FC2Hub 数据源刮削器（FC2 无码作品）
// 借鉴自 mdcx-master 的 fc2hub.py
// FC2Hub 专门收录 FC2-PPV 系列番号，补充 JavBus/JavDB 缺失的 FC2 内容
package service

import (
	"fmt"
	"regexp"
	"strings"
)

// FC2Hub 正则
var (
	fc2hubSearchRe   = regexp.MustCompile(`(?is)<a[^>]+href="(/archive\?kw=\d+[^"]*)"[^>]*>[\s\S]*?<img`)
	fc2hubDetailLinkRe = regexp.MustCompile(`(?is)<a[^>]+href="(/article/\d+[^"]*)"[^>]*class="overlay"`)
	fc2hubTitleRe    = regexp.MustCompile(`(?is)<h3[^>]*class="p-b-5"[^>]*>([^<]+)</h3>`)
	fc2hubCoverRe    = regexp.MustCompile(`(?is)<a[^>]+href="([^"]+\.(?:jpg|jpeg|png))"[^>]*class="comic-cover"`)
	fc2hubCoverAltRe = regexp.MustCompile(`(?is)<img[^>]+src="([^"]+)"[^>]+class="responsive"`)
	fc2hubPlotRe     = regexp.MustCompile(`(?is)<div[^>]+class="comic-description"[^>]*>([\s\S]*?)</div>`)
	fc2hubFanartRe   = regexp.MustCompile(`(?is)<a[^>]+href="([^"]+\.(?:jpg|jpeg|png))"[^>]+data-fancybox`)
	fc2hubInfoRe     = regexp.MustCompile(`(?is)<li[^>]*>\s*<strong>([^<]+)</strong>\s*[:：]?\s*([\s\S]*?)</li>`)
	fc2hubTagRe      = regexp.MustCompile(`(?is)<a[^>]+href="/tag/[^"]+"[^>]*>([^<]+)</a>`)
)

// scrapeFC2Hub 从 FC2Hub 刮削 FC2 番号
func (s *AdultScraperService) scrapeFC2Hub(code string) (*AdultMetadata, error) {
	// FC2Hub 只处理 FC2 系列番号
	// 输入可能是 "FC2-PPV-1234567" 或 "FC2-1234567"，提取纯数字
	fc2Num := extractFC2Number(code)
	if fc2Num == "" {
		return nil, fmt.Errorf("非 FC2 番号，跳过 FC2Hub")
	}

	baseURL := s.cfg.AdultScraper.FC2HubURL
	if baseURL == "" {
		baseURL = "https://fc2hub.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// Step 1: 搜索 FC2 番号
	searchURL := fmt.Sprintf("%s/archive?kw=%s", baseURL, fc2Num)
	html, err := s.fetchHTMLForAdult(searchURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("fc2hub 搜索失败: %w", err)
	}

	// Step 2: 从搜索结果提取详情页链接
	detailPath := ""
	if m := fc2hubDetailLinkRe.FindStringSubmatch(html); len(m) > 1 {
		detailPath = m[1]
	}
	if detailPath == "" {
		return nil, fmt.Errorf("fc2hub 未找到 FC2-%s", fc2Num)
	}

	detailURL := detailPath
	if strings.HasPrefix(detailPath, "/") {
		detailURL = baseURL + detailPath
	}

	randomDelay(
		s.cfg.AdultScraper.MinRequestInterval,
		s.cfg.AdultScraper.MaxRequestInterval,
	)

	// Step 3: 抓取详情页
	detailHTML, err := s.fetchHTMLForAdult(detailURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("fc2hub 详情页抓取失败: %w", err)
	}

	return s.parseFC2HubHTML(detailHTML, code, fc2Num)
}

// parseFC2HubHTML 解析 FC2Hub 详情页
func (s *AdultScraperService) parseFC2HubHTML(html, code, fc2Num string) (*AdultMetadata, error) {
	meta := &AdultMetadata{
		Code:        code,
		Source:      "fc2hub",
		Genres:      []string{},
		Actresses:   []string{},
		ActorPhotos: make(map[string]string),
		ExtraFanart: []string{},
	}

	// 标题
	if m := fc2hubTitleRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Title = strings.TrimSpace(m[1])
	}

	// 封面
	if m := fc2hubCoverRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Cover = cleanURL(m[1])
	} else if m := fc2hubCoverAltRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Cover = cleanURL(m[1])
	}

	// 简介
	if m := fc2hubPlotRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Plot = strings.TrimSpace(stripHTMLTags(m[1]))
	}

	// 信息行（演员/发布日期/时长等）
	for _, m := range fc2hubInfoRe.FindAllStringSubmatch(html, -1) {
		if len(m) < 3 {
			continue
		}
		key := strings.TrimSpace(m[1])
		valRaw := m[2]
		val := strings.TrimSpace(stripHTMLTags(valRaw))
		switch {
		case strings.Contains(key, "Seller") || strings.Contains(key, "卖家") || strings.Contains(key, "賣家"):
			// FC2 的 Seller 相当于制作者
			meta.Studio = val
		case strings.Contains(key, "Actress") || strings.Contains(key, "演员") || strings.Contains(key, "女優"):
			// 多个演员可能用逗号/分号分隔
			for _, name := range splitMulti(val, ",;、") {
				name = strings.TrimSpace(name)
				if name != "" {
					meta.Actresses = append(meta.Actresses, name)
				}
			}
		case strings.Contains(key, "Date") || strings.Contains(key, "发布") || strings.Contains(key, "発売"):
			meta.ReleaseDate = normalizeFanzaDate(val)
		}
	}

	// 标签
	seenTag := make(map[string]struct{})
	for _, m := range fc2hubTagRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			tag := strings.TrimSpace(m[1])
			if tag == "" {
				continue
			}
			if _, ok := seenTag[tag]; !ok {
				seenTag[tag] = struct{}{}
				meta.Genres = append(meta.Genres, tag)
			}
		}
	}
	// FC2 统一标记为无码
	meta.Genres = append(meta.Genres, "无码", "FC2")

	// 剧照
	for _, m := range fc2hubFanartRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			meta.ExtraFanart = append(meta.ExtraFanart, cleanURL(m[1]))
		}
	}

	if meta.Title == "" {
		return nil, fmt.Errorf("fc2hub 解析失败：未提取到标题")
	}
	return meta, nil
}

// ==================== 辅助函数 ====================

// extractFC2Number 从番号中提取 FC2 数字部分
// FC2-PPV-1234567 -> 1234567
// FC2-1234567 -> 1234567
// 非 FC2 番号返回空
func extractFC2Number(code string) string {
	code = strings.ToUpper(code)
	re := regexp.MustCompile(`(?:FC2[-\s]*(?:PPV)?[-\s]*)(\d{4,})`)
	if m := re.FindStringSubmatch(code); len(m) > 1 {
		return m[1]
	}
	return ""
}

// splitMulti 按多个分隔符拆分字符串
func splitMulti(s, sep string) []string {
	if s == "" {
		return nil
	}
	// 把所有分隔符换成统一分隔
	for _, c := range sep {
		s = strings.ReplaceAll(s, string(c), "|")
	}
	parts := strings.Split(s, "|")
	result := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
