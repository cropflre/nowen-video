// Package service Freejavbt 数据源刮削器
// 借鉴自 mdcx-master 项目的 freejavbt.py 实现
// Freejavbt 特点：中文元数据优秀，站点反爬较弱，搜索+详情页两步刮削
package service

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Freejavbt 正则表达式集合（预编译，提升性能）
var (
	freejavbtSearchRe   = regexp.MustCompile(`(?is)<a\s+class="box"\s+href="([^"]+)"`)
	freejavbtCoverRe    = regexp.MustCompile(`(?is)<img[^>]+class="video-cover"[^>]+src="([^"]+)"`)
	freejavbtTitleRe    = regexp.MustCompile(`(?is)<h1[^>]*class="title"[^>]*>([^<]+)</h1>`)
	freejavbtPlotRe     = regexp.MustCompile(`(?is)<div[^>]*class="description"[^>]*>(.*?)</div>`)
	freejavbtDateRe     = regexp.MustCompile(`(?is)<strong>发行日期:</strong>\s*<span[^>]*>([^<]+)</span>`)
	freejavbtDurationRe = regexp.MustCompile(`(?is)<strong>时长:</strong>\s*<span[^>]*>(\d+)`)
	freejavbtStudioRe   = regexp.MustCompile(`(?is)<strong>片商:</strong>.*?<a[^>]*>([^<]+)</a>`)
	freejavbtLabelRe    = regexp.MustCompile(`(?is)<strong>发行商:</strong>.*?<a[^>]*>([^<]+)</a>`)
	freejavbtSeriesRe   = regexp.MustCompile(`(?is)<strong>系列:</strong>.*?<a[^>]*>([^<]+)</a>`)
	freejavbtDirectorRe = regexp.MustCompile(`(?is)<strong>导演:</strong>.*?<a[^>]*>([^<]+)</a>`)
	freejavbtGenreRe    = regexp.MustCompile(`(?is)<a[^>]+class="genre"[^>]*>([^<]+)</a>`)
	freejavbtActressRe  = regexp.MustCompile(`(?is)<a[^>]+class="star"[^>]*>\s*<img[^>]+alt="([^"]+)"[^>]+src="([^"]+)"`)
	freejavbtRatingRe   = regexp.MustCompile(`(?is)<span[^>]*class="score"[^>]*>([\d.]+)</span>`)
	freejavbtFanartRe   = regexp.MustCompile(`(?is)<a[^>]+data-fancybox="gallery"[^>]+href="([^"]+)"`)
	freejavbtTrailerRe  = regexp.MustCompile(`(?is)<video[^>]+src="([^"]+)"`)
)

// scrapeFreejavbt 从 Freejavbt 刮削番号元数据
func (s *AdultScraperService) scrapeFreejavbt(code string) (*AdultMetadata, error) {
	baseURL := s.cfg.AdultScraper.FreejavbtURL
	if baseURL == "" {
		baseURL = "https://freejavbt.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// Step 1: 搜索番号，获取详情页链接
	searchURL := fmt.Sprintf("%s/search/%s", baseURL, code)
	searchHTML, err := s.fetchHTMLForAdult(searchURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("freejavbt 搜索失败: %w", err)
	}

	detailPath := ""
	if m := freejavbtSearchRe.FindStringSubmatch(searchHTML); len(m) > 1 {
		detailPath = m[1]
	}
	if detailPath == "" {
		return nil, fmt.Errorf("freejavbt 未找到番号 %s 的搜索结果", code)
	}

	// 拼接完整详情 URL
	detailURL := detailPath
	if strings.HasPrefix(detailPath, "/") {
		detailURL = baseURL + detailPath
	} else if !strings.HasPrefix(detailPath, "http") {
		detailURL = baseURL + "/" + detailPath
	}

	// 站点内间隔（防反爬）
	randomDelay(
		s.cfg.AdultScraper.MinRequestInterval,
		s.cfg.AdultScraper.MaxRequestInterval,
	)

	// Step 2: 抓取详情页
	detailHTML, err := s.fetchHTMLForAdult(detailURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("freejavbt 详情页抓取失败: %w", err)
	}

	return s.parseFreejavbtHTML(detailHTML, code)
}

// parseFreejavbtHTML 解析 Freejavbt 详情页 HTML
func (s *AdultScraperService) parseFreejavbtHTML(html, code string) (*AdultMetadata, error) {
	meta := &AdultMetadata{
		Code:        code,
		Source:      "freejavbt",
		Genres:      []string{},
		Actresses:   []string{},
		ActorPhotos: make(map[string]string),
		ExtraFanart: []string{},
	}

	// 标题
	if m := freejavbtTitleRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Title = strings.TrimSpace(stripHTMLTags(m[1]))
	}
	// 简介
	if m := freejavbtPlotRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Plot = strings.TrimSpace(stripHTMLTags(m[1]))
	}
	// 封面
	if m := freejavbtCoverRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Cover = cleanURL(m[1])
	}
	// 发行日期
	if m := freejavbtDateRe.FindStringSubmatch(html); len(m) > 1 {
		meta.ReleaseDate = strings.TrimSpace(m[1])
	}
	// 时长
	if m := freejavbtDurationRe.FindStringSubmatch(html); len(m) > 1 {
		fmt.Sscanf(m[1], "%d", &meta.Duration)
	}
	// 片商/发行商/系列/导演
	if m := freejavbtStudioRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Studio = strings.TrimSpace(m[1])
	}
	if m := freejavbtLabelRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Label = strings.TrimSpace(m[1])
	}
	if m := freejavbtSeriesRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Series = strings.TrimSpace(m[1])
	}
	if m := freejavbtDirectorRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Director = strings.TrimSpace(m[1])
	}
	// 评分
	if m := freejavbtRatingRe.FindStringSubmatch(html); len(m) > 1 {
		fmt.Sscanf(m[1], "%f", &meta.Rating)
	}
	// 类型
	for _, m := range freejavbtGenreRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			g := strings.TrimSpace(stripHTMLTags(m[1]))
			if g != "" {
				meta.Genres = append(meta.Genres, g)
			}
		}
	}
	// 演员（顺便抓取演员头像）
	for _, m := range freejavbtActressRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 2 {
			name := strings.TrimSpace(m[1])
			photo := cleanURL(m[2])
			if name != "" {
				meta.Actresses = append(meta.Actresses, name)
				if photo != "" {
					meta.ActorPhotos[name] = photo
				}
			}
		}
	}
	// Extra Fanart
	for _, m := range freejavbtFanartRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			meta.ExtraFanart = append(meta.ExtraFanart, cleanURL(m[1]))
		}
	}
	// Trailer
	if m := freejavbtTrailerRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Trailer = cleanURL(m[1])
	}

	if meta.Title == "" {
		return nil, fmt.Errorf("freejavbt 解析失败：未找到标题")
	}
	return meta, nil
}

// fetchHTMLForAdult 抓取 HTML（复用 applyBrowserHeaders 并自动带 Referer，自动注入站点 Cookie）
func (s *AdultScraperService) fetchHTMLForAdult(targetURL, referer string) (string, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", err
	}
	s.applyBrowserHeaders(req)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	client := s.client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// stripHTMLTags 去除 HTML 标签
func stripHTMLTags(s string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(s, "")
}

// cleanURL 清理 URL（补全 https: 前缀、去除参数等）
func cleanURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	}
	return u
}
