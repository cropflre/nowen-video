// Package service Fanza (DMM) 官方数据源刮削器
// 借鉴自 mdcx-master 的 fanza.py
// Fanza 是 DMM 旗下成人内容官方站点，是番号元数据的权威来源：
//   - 封面：最大最清晰（official_jacket）
//   - 简介：日文原版简介完整
//   - 演员/片商：官方规范名
//   - 日期：准确的发售日期
// 访问限制：Fanza 有 cookie 验证（age_check_done=1），已自动处理
package service

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// Fanza 正则（预编译）
var (
	fanzaSearchLinkRe  = regexp.MustCompile(`(?is)<a[^>]+href="(https?://[^"]*/digital/videoa/-/detail/=/cid=[^"]+)"`)
	fanzaSearchLinkRe2 = regexp.MustCompile(`(?is)<a[^>]+href="(https?://[^"]*/mono/dvd/-/detail/=/cid=[^"]+)"`)
	fanzaTitleRe       = regexp.MustCompile(`(?is)<h1[^>]*id="title"[^>]*>([^<]+)</h1>`)
	fanzaCoverRe       = regexp.MustCompile(`(?is)<img[^>]+id="package-src-[^"]*"[^>]+src="([^"]+)"`)
	fanzaCoverAltRe    = regexp.MustCompile(`(?is)<a[^>]+href="([^"]+pl\.jpg)"[^>]*>`)
	fanzaPlotRe        = regexp.MustCompile(`(?is)<div[^>]+class="mg-b20 lh4"[^>]*>([\s\S]*?)</div>`)
	fanzaTableRowRe    = regexp.MustCompile(`(?is)<tr>\s*<td[^>]*>([^<]+)</td>\s*<td[^>]*>([\s\S]*?)</td>\s*</tr>`)
	fanzaFanartRe      = regexp.MustCompile(`(?is)<a[^>]+href="([^"]+\.jpg)"[^>]+name="sample-image"`)
	fanzaFanartAltRe   = regexp.MustCompile(`(?is)<img[^>]+src="([^"]+)"[^>]+id="sample-image\d+"`)
	fanzaTrailerAPIRe  = regexp.MustCompile(`(?is)cid=([^"&]+)`)
	fanzaActressLinkRe = regexp.MustCompile(`(?is)<a[^>]+href="/digital/videoa/-/list/=/article=actress/id=[^"]+"[^>]*>([^<]+)</a>`)
	fanzaGenreLinkRe   = regexp.MustCompile(`(?is)<a[^>]+href="/digital/videoa/-/list/=/article=keyword/id=[^"]+"[^>]*>([^<]+)</a>`)
	fanzaRatingRe      = regexp.MustCompile(`(?is)class="d-review__average"[^>]*>\s*([\d.]+)`)
	fanzaContentIDRe   = regexp.MustCompile(`(?is)<td[^>]*>品番[:：]?</td>\s*<td[^>]*>([^<]+)</td>`)
)

// scrapeFanza 从 Fanza 刮削番号元数据
func (s *AdultScraperService) scrapeFanza(code string) (*AdultMetadata, error) {
	baseURL := s.cfg.AdultScraper.FanzaURL
	if baseURL == "" {
		baseURL = "https://www.dmm.co.jp"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// Fanza 搜索需要移除番号中的横线（如 SSIS-001 -> ssis001 / ssis00001）
	fanzaCode := buildFanzaCID(code)

	// Step 1: 搜索番号
	searchURL := fmt.Sprintf("%s/search/=/searchstr=%s", baseURL, fanzaCode)
	html, err := s.fetchFanzaHTML(searchURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("fanza 搜索失败: %w", err)
	}

	// 尝试两种搜索结果链接（数字版 / 单体 DVD）
	detailURL := ""
	if m := fanzaSearchLinkRe.FindStringSubmatch(html); len(m) > 1 {
		detailURL = m[1]
	} else if m := fanzaSearchLinkRe2.FindStringSubmatch(html); len(m) > 1 {
		detailURL = m[1]
	}

	// 若搜索没结果，尝试直接拼 cid URL（ssis00001 这种格式有时搜索无结果但直链可用）
	if detailURL == "" {
		for _, padded := range buildFanzaCIDCandidates(code) {
			directURL := fmt.Sprintf("%s/digital/videoa/-/detail/=/cid=%s/", baseURL, padded)
			if h, e := s.fetchFanzaHTML(directURL, baseURL); e == nil && strings.Contains(h, "id=\"title\"") {
				html = h
				detailURL = directURL
				break
			}
			randomDelay(1000, 2000)
		}
	}

	if detailURL == "" {
		return nil, fmt.Errorf("fanza 未找到番号 %s", code)
	}

	// 若没有提前加载详情页 HTML，则去抓详情页
	if !strings.Contains(html, "id=\"title\"") {
		randomDelay(
			s.cfg.AdultScraper.MinRequestInterval,
			s.cfg.AdultScraper.MaxRequestInterval,
		)
		h, err := s.fetchFanzaHTML(detailURL, baseURL)
		if err != nil {
			return nil, fmt.Errorf("fanza 详情页抓取失败: %w", err)
		}
		html = h
	}

	return s.parseFanzaHTML(html, code, detailURL)
}

// fetchFanzaHTML 抓取 Fanza HTML（带 age_check cookie）
func (s *AdultScraperService) fetchFanzaHTML(targetURL, referer string) (string, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", err
	}
	s.applyBrowserHeaders(req)
	// Fanza 的年龄认证 cookie（必需）
	req.AddCookie(&http.Cookie{Name: "age_check_done", Value: "1"})
	req.AddCookie(&http.Cookie{Name: "ckcy", Value: "1"})
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return doRequestReadString(s.client, req)
}

// parseFanzaHTML 解析 Fanza 详情页
func (s *AdultScraperService) parseFanzaHTML(html, code, detailURL string) (*AdultMetadata, error) {
	meta := &AdultMetadata{
		Code:        code,
		Source:      "fanza",
		Genres:      []string{},
		Actresses:   []string{},
		ActorPhotos: make(map[string]string),
		ExtraFanart: []string{},
	}

	// 标题（去除番号前缀）
	if m := fanzaTitleRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Title = strings.TrimSpace(m[1])
	}

	// 封面（优先 id=package-src 中的大图，否则找 pl.jpg 直链）
	if m := fanzaCoverRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Cover = cleanURL(m[1])
	} else if m := fanzaCoverAltRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Cover = cleanURL(m[1])
	}
	// 把 pl.jpg（大图）替换为 ps.jpg（缩略图）做 thumb
	if meta.Cover != "" {
		meta.Thumb = strings.Replace(meta.Cover, "pl.jpg", "ps.jpg", 1)
	}

	// 简介
	if m := fanzaPlotRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Plot = strings.TrimSpace(stripHTMLTags(m[1]))
	}

	// 演员（日文原官方名）
	seenActress := make(map[string]struct{})
	for _, m := range fanzaActressLinkRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			name := strings.TrimSpace(stripHTMLTags(m[1]))
			if name == "" || name == "----" {
				continue
			}
			if _, ok := seenActress[name]; !ok {
				seenActress[name] = struct{}{}
				meta.Actresses = append(meta.Actresses, name)
			}
		}
	}

	// 类型标签
	seenGenre := make(map[string]struct{})
	for _, m := range fanzaGenreLinkRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			g := strings.TrimSpace(stripHTMLTags(m[1]))
			if g == "" {
				continue
			}
			if _, ok := seenGenre[g]; !ok {
				seenGenre[g] = struct{}{}
				meta.Genres = append(meta.Genres, g)
			}
		}
	}

	// 表格信息：发售日/片商/系列/时长/导演等
	for _, m := range fanzaTableRowRe.FindAllStringSubmatch(html, -1) {
		if len(m) < 3 {
			continue
		}
		key := strings.TrimSpace(stripHTMLTags(m[1]))
		key = strings.TrimRight(key, ":：")
		valHTML := m[2]
		val := strings.TrimSpace(stripHTMLTags(valHTML))
		if val == "----" || val == "" {
			continue
		}
		switch {
		case strings.Contains(key, "発売日") || strings.Contains(key, "配信開始日"):
			meta.ReleaseDate = normalizeFanzaDate(val)
		case strings.Contains(key, "収録時間"):
			// "120分" -> 120
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
		case strings.Contains(key, "監督"):
			meta.Director = val
		}
	}

	// 评分
	if m := fanzaRatingRe.FindStringSubmatch(html); len(m) > 1 {
		fmt.Sscanf(m[1], "%f", &meta.Rating)
	}

	// 剧照（样张图）
	for _, m := range fanzaFanartRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			meta.ExtraFanart = append(meta.ExtraFanart, cleanURL(m[1]))
		}
	}
	if len(meta.ExtraFanart) == 0 {
		for _, m := range fanzaFanartAltRe.FindAllStringSubmatch(html, -1) {
			if len(m) > 1 {
				// 把 -1.jpg 替换为 jp-1.jpg 取大图
				img := cleanURL(m[1])
				img = strings.Replace(img, "-", "jp-", 1)
				meta.ExtraFanart = append(meta.ExtraFanart, img)
			}
		}
	}

	// Trailer（Fanza 提供 movie-player/view/ API）
	if m := fanzaTrailerAPIRe.FindStringSubmatch(detailURL); len(m) > 1 {
		cid := m[1]
		cid = strings.TrimRight(cid, "/")
		// Fanza 的 movie player API（注：此处只存 URL，不做二次抓取）
		meta.Trailer = fmt.Sprintf("https://www.dmm.co.jp/service/digitalapi/-/html5_player/=/cid=%s/", cid)
	}

	if meta.Title == "" {
		return nil, fmt.Errorf("fanza 解析失败：未提取到标题")
	}
	return meta, nil
}

// ==================== 辅助函数 ====================

// buildFanzaCID 把标准番号转为 Fanza cid 格式
// SSIS-001 -> ssis00001（5 位数字补零）
func buildFanzaCID(code string) string {
	code = strings.ToLower(code)
	re := regexp.MustCompile(`^([a-z]+)-?(\d+)$`)
	m := re.FindStringSubmatch(code)
	if len(m) < 3 {
		return strings.ReplaceAll(code, "-", "")
	}
	prefix := m[1]
	num := m[2]
	// 补零到 5 位
	for len(num) < 5 {
		num = "0" + num
	}
	return prefix + num
}

// buildFanzaCIDCandidates 生成可能的 Fanza cid 格式候选（做直链尝试）
// 不同番号有不同补零规则，生成 3/5/兼容三种
func buildFanzaCIDCandidates(code string) []string {
	code = strings.ToLower(code)
	re := regexp.MustCompile(`^([a-z]+)-?(\d+)$`)
	m := re.FindStringSubmatch(code)
	if len(m) < 3 {
		return []string{strings.ReplaceAll(code, "-", "")}
	}
	prefix := m[1]
	num := m[2]
	candidates := []string{}
	for _, pad := range []int{5, 4, 3} {
		n := num
		for len(n) < pad {
			n = "0" + n
		}
		candidates = append(candidates, prefix+n)
	}
	return candidates
}

// normalizeFanzaDate Fanza 日期格式化：2024/01/01 -> 2024-01-01
func normalizeFanzaDate(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "-")
	// 取前 10 个字符（剔除 "2024-01-01 10:00" 这种时间后缀）
	if len(s) > 10 {
		s = s[:10]
	}
	return s
}
