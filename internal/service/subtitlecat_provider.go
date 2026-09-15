package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/html"
)

const (
	subtitleCatProviderName = "subtitlecat"
	subtitleCatBaseURL      = "https://www.subtitlecat.com"
	subtitleCatSearchTTL    = 20 * time.Minute
	subtitleCatDetailTTL    = 45 * time.Minute
	subtitleCatMaxHTMLBytes = int64(4 << 20)
	subtitleCatMaxSRTBytes  = int64(4 << 20)
)

var (
	subtitleCatDetailPathPattern   = regexp.MustCompile(`(?i)^/subs/\d+/[^?#]+\.html$`)
	subtitleCatDownloadPathPattern = regexp.MustCompile(`(?i)^/subs/\d+/[^?#]+\.srt$`)
	subtitleCatSizePattern         = regexp.MustCompile(`(?i)\bSIZE\s+([0-9.]+\s*(?:KB|MB|GB))\b`)
	subtitleCatDownloadsPattern    = regexp.MustCompile(`(?i)\b(\d+)\s+downloads?\b`)
	subtitleCatLanguagesPattern    = regexp.MustCompile(`(?i)\b(\d+)\s+languages?\b`)
	subtitleCatLangSuffixPattern   = regexp.MustCompile(`(?i)-([a-z]{2,3}(?:-[a-z0-9]{2,8})?)\.srt$`)
)

type subtitleCatSearchItem struct {
	Title         string
	DetailPath    string
	FileSize      string
	DownloadCount int
	LanguageCount int
	MatchScore    int
}

type subtitleCatSearchCacheEntry struct {
	ExpiresAt time.Time
	Results   []SubtitleSearchResult
}

type subtitleCatDetailCacheEntry struct {
	ExpiresAt time.Time
	Details   *SubtitleProviderDetails
}

// SubtitleCatProvider 通过服务端 HTML 解析接入 SubtitleCat。
// 所有 URL 规则、HTML 解析和语言映射都封装在本 Provider 内，避免泄漏到 Handler/UI。
type SubtitleCatProvider struct {
	logger       *zap.SugaredLogger
	baseURL      *url.URL
	client       *http.Client
	allowedHosts map[string]struct{}
	allowHTTP    bool

	mu          sync.RWMutex
	searchCache map[string]subtitleCatSearchCacheEntry
	detailCache map[string]subtitleCatDetailCacheEntry
}

func NewSubtitleCatProvider(logger *zap.SugaredLogger) *SubtitleCatProvider {
	base, _ := url.Parse(subtitleCatBaseURL)
	p := &SubtitleCatProvider{
		logger:       logger,
		baseURL:      base,
		allowedHosts: map[string]struct{}{"subtitlecat.com": {}, "www.subtitlecat.com": {}},
		searchCache:  make(map[string]subtitleCatSearchCacheEntry),
		detailCache:  make(map[string]subtitleCatDetailCacheEntry),
	}
	p.client = p.newHTTPClient()
	return p
}

// newSubtitleCatProviderForTest 仅供 fixture/httptest 使用，生产构造器始终固定官方 HTTPS host allowlist。
func newSubtitleCatProviderForTest(baseURL string, logger *zap.SugaredLogger) (*SubtitleCatProvider, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	p := &SubtitleCatProvider{
		logger:       logger,
		baseURL:      base,
		allowedHosts: map[string]struct{}{strings.ToLower(base.Hostname()): {}},
		allowHTTP:    true,
		searchCache:  make(map[string]subtitleCatSearchCacheEntry),
		detailCache:  make(map[string]subtitleCatDetailCacheEntry),
	}
	p.client = p.newHTTPClient()
	return p, nil
}

func (p *SubtitleCatProvider) Name() string { return subtitleCatProviderName }

func (p *SubtitleCatProvider) newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("subtitlecat redirect limit exceeded")
			}
			return p.validateURL(req.URL)
		},
	}
}

func (p *SubtitleCatProvider) validateURL(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("empty URL")
	}
	if u.Scheme != "https" && !(p.allowHTTP && u.Scheme == "http") {
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if _, ok := p.allowedHosts[host]; !ok {
		return fmt.Errorf("host is not allowed: %s", host)
	}
	if u.User != nil {
		return fmt.Errorf("userinfo is not allowed")
	}
	return nil
}

func (p *SubtitleCatProvider) resolveAllowedURL(ref string, download bool) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return nil, err
	}
	if !u.IsAbs() {
		u = p.baseURL.ResolveReference(u)
	}
	if err := p.validateURL(u); err != nil {
		return nil, err
	}
	cleanPath := path.Clean(u.EscapedPath())
	if decoded, err := url.PathUnescape(cleanPath); err == nil {
		cleanPath = decoded
	}
	if download {
		if !subtitleCatDownloadPathPattern.MatchString(cleanPath) {
			return nil, fmt.Errorf("invalid SubtitleCat download path")
		}
	} else if !subtitleCatDetailPathPattern.MatchString(cleanPath) && cleanPath != "/index.php" && cleanPath != "/" {
		return nil, fmt.Errorf("invalid SubtitleCat page path")
	}
	return u, nil
}

func (p *SubtitleCatProvider) Search(ctx context.Context, req SubtitleProviderSearchRequest) ([]SubtitleSearchResult, error) {
	started := time.Now()
	queries := dedupeSubtitleCatQueries(req.Queries)
	if len(queries) == 0 && strings.TrimSpace(req.Title) != "" {
		queries = []string{strings.TrimSpace(req.Title)}
	}
	if len(queries) == 0 {
		return nil, nil
	}
	languages := req.Languages
	if len(languages) == 0 {
		languages = []string{"zh-CN", "zh-TW", "en"}
	}
	for i := range languages {
		languages[i] = normalizeSubtitleLanguageCode(languages[i])
	}

	cacheKey := strings.ToLower(strings.Join(queries, "\x1f") + "|" + strings.Join(languages, ","))
	p.mu.RLock()
	if cached, ok := p.searchCache[cacheKey]; ok && time.Now().Before(cached.ExpiresAt) {
		out := cloneSubtitleSearchResults(cached.Results)
		p.mu.RUnlock()
		return out, nil
	}
	p.mu.RUnlock()

	itemsByPath := make(map[string]subtitleCatSearchItem)
	var lastErr error
	// 最多使用前三个逐步放宽的关键词，避免一次打开字幕窗口造成过多外部请求。
	for qi, query := range queries {
		if qi >= 3 {
			break
		}
		items, err := p.searchPage(ctx, query)
		if err != nil {
			lastErr = err
			continue
		}
		for _, item := range items {
			if _, exists := itemsByPath[item.DetailPath]; exists {
				continue
			}
			item.MatchScore = scoreSubtitleCatCandidate(item.Title, req)
			if item.MatchScore < 20 {
				continue
			}
			itemsByPath[item.DetailPath] = item
		}
		// 精确关键词已经拿到足够候选时不继续放宽，减少抓取。
		if len(itemsByPath) >= 12 {
			break
		}
	}

	candidates := make([]subtitleCatSearchItem, 0, len(itemsByPath))
	for _, item := range itemsByPath {
		candidates = append(candidates, item)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].MatchScore != candidates[j].MatchScore {
			return candidates[i].MatchScore > candidates[j].MatchScore
		}
		return candidates[i].DownloadCount > candidates[j].DownloadCount
	})
	if len(candidates) > 8 {
		candidates = candidates[:8]
	}

	wanted := make(map[string]bool, len(languages))
	for _, lang := range languages {
		wanted[strings.ToLower(lang)] = true
	}

	results := make([]SubtitleSearchResult, 0, len(candidates)*len(languages))
	seenDownloads := make(map[string]bool)
	for _, candidate := range candidates {
		detailID := encodeSubtitleCatRef("detail", candidate.DetailPath)
		details, err := p.GetDetails(ctx, detailID)
		if err != nil {
			lastErr = err
			continue
		}
		available := make([]string, 0, len(details.Languages))
		for _, lang := range details.Languages {
			available = append(available, lang.Code)
		}
		for _, lang := range details.Languages {
			if !wanted[strings.ToLower(normalizeSubtitleLanguageCode(lang.Code))] {
				continue
			}
			if seenDownloads[lang.DownloadID] {
				continue
			}
			seenDownloads[lang.DownloadID] = true
			matchType := "title"
			if subtitleEpisodeTag.MatchString(req.FileName) {
				matchType = "episode"
			}
			results = append(results, SubtitleSearchResult{
				ID:                 lang.DownloadID,
				Title:              candidate.Title,
				FileName:           strings.TrimSuffix(candidate.Title, ".srt") + "-" + lang.Code + ".srt",
				Language:           lang.Code,
				LanguageName:       lang.Name,
				Format:             "srt",
				Rating:             float64(candidate.MatchScore) / 10.0,
				DownloadCount:      candidate.DownloadCount,
				Source:             subtitleCatProviderName,
				DownloadURL:        "",
				MatchType:          matchType,
				FileSize:           candidate.FileSize,
				MatchScore:         candidate.MatchScore,
				SourceURL:          details.SourceURL,
				AvailableLanguages: available,
			})
		}
	}

	sortSubtitleSearchResults(results)
	p.mu.Lock()
	p.searchCache[cacheKey] = subtitleCatSearchCacheEntry{
		ExpiresAt: time.Now().Add(subtitleCatSearchTTL),
		Results:   cloneSubtitleSearchResults(results),
	}
	p.pruneExpiredCachesLocked()
	p.mu.Unlock()

	if p.logger != nil {
		p.logger.Infow("subtitle provider search",
			"provider", subtitleCatProviderName,
			"operation", "search",
			"query", queries[0],
			"results", len(results),
			"duration_ms", time.Since(started).Milliseconds(),
		)
	}
	if len(results) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return results, nil
}

func (p *SubtitleCatProvider) searchPage(ctx context.Context, query string) ([]subtitleCatSearchItem, error) {
	u := *p.baseURL
	u.Path = "/index.php"
	q := u.Query()
	q.Set("search", query)
	u.RawQuery = q.Encode()

	body, err := p.fetch(ctx, &u, false, subtitleCatMaxHTMLBytes)
	if err != nil {
		return nil, err
	}
	items, err := parseSubtitleCatSearchHTML(body)
	if err != nil {
		p.logParseFailure("search", "selector_not_found", err)
		return nil, err
	}
	return items, nil
}

func (p *SubtitleCatProvider) GetDetails(ctx context.Context, id string) (*SubtitleProviderDetails, error) {
	kind, ref, err := decodeSubtitleCatRef(id)
	if err != nil || kind != "detail" {
		return nil, fmt.Errorf("invalid SubtitleCat detail id")
	}
	p.mu.RLock()
	if cached, ok := p.detailCache[ref]; ok && time.Now().Before(cached.ExpiresAt) {
		copyValue := *cached.Details
		copyValue.Languages = append([]SubtitleProviderLanguage(nil), cached.Details.Languages...)
		p.mu.RUnlock()
		return &copyValue, nil
	}
	p.mu.RUnlock()

	u, err := p.resolveAllowedURL(ref, false)
	if err != nil {
		return nil, err
	}
	body, err := p.fetch(ctx, u, false, subtitleCatMaxHTMLBytes)
	if err != nil {
		return nil, err
	}
	details, err := parseSubtitleCatDetailHTML(body, u, p)
	if err != nil {
		p.logParseFailure("detail", "selector_not_found", err)
		return nil, err
	}
	details.ID = id
	details.SourceURL = u.String()
	p.mu.Lock()
	p.detailCache[ref] = subtitleCatDetailCacheEntry{ExpiresAt: time.Now().Add(subtitleCatDetailTTL), Details: details}
	p.pruneExpiredCachesLocked()
	p.mu.Unlock()
	copyValue := *details
	copyValue.Languages = append([]SubtitleProviderLanguage(nil), details.Languages...)
	return &copyValue, nil
}

func (p *SubtitleCatProvider) Download(ctx context.Context, id string) (*SubtitleProviderDownload, error) {
	kind, ref, err := decodeSubtitleCatRef(id)
	if err != nil || kind != "download" {
		return nil, fmt.Errorf("invalid SubtitleCat download id")
	}
	u, err := p.resolveAllowedURL(ref, true)
	if err != nil {
		return nil, err
	}
	body, err := p.fetch(ctx, u, true, subtitleCatMaxSRTBytes)
	if err != nil {
		return nil, err
	}
	lang := subtitleCatLanguageFromDownloadPath(u.Path)
	return &SubtitleProviderDownload{
		Content:  body,
		FileName: path.Base(u.Path),
		Language: lang,
	}, nil
}

func (p *SubtitleCatProvider) fetch(ctx context.Context, u *url.URL, subtitle bool, maxBytes int64) ([]byte, error) {
	if err := p.validateURL(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	// 使用普通浏览器兼容请求头，避免站点基于 User-Agent 返回精简/兼容页面。
	// 403/429/Cloudflare challenge 仍然按 Provider unavailable 处理，不做任何绕过。
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", subtitleCatBaseURL+"/")
	if subtitle {
		req.Header.Set("Accept", "application/x-subrip,text/plain;q=0.9,*/*;q=0.1")
	} else {
		req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, &SubtitleProviderUnavailableError{Provider: p.Name(), Reason: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, &SubtitleProviderUnavailableError{Provider: p.Name(), Reason: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SubtitleCat HTTP %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("SubtitleCat response exceeds %d bytes", maxBytes)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if subtitle && strings.Contains(contentType, "text/html") {
		return nil, fmt.Errorf("SubtitleCat returned HTML instead of subtitle")
	}
	if looksLikeSubtitleCatChallenge(body) {
		return nil, &SubtitleProviderUnavailableError{Provider: p.Name(), Reason: "anti-bot challenge"}
	}
	return body, nil
}

// normalizeSubtitleCatDetailPath 将 SubtitleCat 当前页面可能出现的
// /subs/..., subs/...、./subs/... 或绝对 URL 统一成安全的 /subs/... 路径。
func normalizeSubtitleCatDetailPath(href string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return "", false
	}
	candidate := strings.TrimSpace(u.Path)
	if candidate == "" {
		return "", false
	}
	candidate = strings.TrimPrefix(candidate, "./")
	if !strings.HasPrefix(candidate, "/") {
		candidate = "/" + candidate
	}
	candidate = path.Clean(candidate)
	if decoded, err := url.PathUnescape(candidate); err == nil {
		candidate = decoded
	}
	if !subtitleCatDetailPathPattern.MatchString(candidate) {
		return "", false
	}
	return candidate, true
}

func parseSubtitleCatSearchHTML(body []byte) ([]subtitleCatSearchItem, error) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	items := make([]subtitleCatSearchItem, 0, 16)
	seen := make(map[string]bool)
	walkHTML(doc, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "a" {
			return
		}
		detailPath, ok := normalizeSubtitleCatDetailPath(htmlAttr(n, "href"))
		if !ok || seen[detailPath] {
			return
		}
		title := strings.TrimSpace(htmlNodeText(n))
		if title == "" {
			return
		}
		seen[detailPath] = true
		rowText := htmlNodeText(nearestHTMLAncestor(n, "tr"))
		item := subtitleCatSearchItem{Title: title, DetailPath: detailPath}
		if m := subtitleCatSizePattern.FindStringSubmatch(rowText); len(m) > 1 {
			item.FileSize = strings.TrimSpace(m[1])
		}
		if m := subtitleCatDownloadsPattern.FindStringSubmatch(rowText); len(m) > 1 {
			item.DownloadCount, _ = strconv.Atoi(m[1])
		}
		if m := subtitleCatLanguagesPattern.FindStringSubmatch(rowText); len(m) > 1 {
			item.LanguageCount, _ = strconv.Atoi(m[1])
		}
		items = append(items, item)
	})
	if len(items) == 0 {
		// “0 subtitles found” 是正常空结果；页面结构完全未知才视为解析失败。
		text := strings.ToLower(htmlNodeText(doc))
		if strings.Contains(text, "0 subtitles found") || strings.Contains(text, "no subtitles") {
			return nil, nil
		}
		return nil, fmt.Errorf("no SubtitleCat detail links found")
	}
	return items, nil
}

func parseSubtitleCatDetailHTML(body []byte, pageURL *url.URL, provider *SubtitleCatProvider) (*SubtitleProviderDetails, error) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	title := ""
	walkHTML(doc, func(n *html.Node) {
		if title != "" || n.Type != html.ElementNode || n.Data != "h2" {
			return
		}
		text := strings.TrimSpace(htmlNodeText(n))
		const prefix = "All language subtitles for "
		if strings.HasPrefix(text, prefix) {
			title = strings.TrimSpace(strings.TrimPrefix(text, prefix))
		}
	})
	languages := make([]SubtitleProviderLanguage, 0, 16)
	seen := make(map[string]bool)
	walkHTML(doc, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "a" {
			return
		}
		href := htmlAttr(n, "href")
		u, err := provider.resolveAllowedURL(href, true)
		if err != nil || !subtitleCatDownloadPathPattern.MatchString(u.Path) {
			return
		}
		code := subtitleCatLanguageFromDownloadPath(u.Path)
		if code == "" || seen[strings.ToLower(code)] {
			return
		}
		seen[strings.ToLower(code)] = true
		languages = append(languages, SubtitleProviderLanguage{
			Code:       code,
			Name:       subtitleCatLanguageName(code),
			DownloadID: encodeSubtitleCatRef("download", u.Path),
		})
	})
	if len(languages) == 0 {
		return nil, fmt.Errorf("no downloadable SRT links found")
	}
	sort.SliceStable(languages, func(i, j int) bool {
		pi, pj := subtitleLanguagePriority(languages[i].Code), subtitleLanguagePriority(languages[j].Code)
		if pi != pj {
			return pi < pj
		}
		return languages[i].Name < languages[j].Name
	})
	if title == "" && pageURL != nil {
		title = strings.TrimSuffix(path.Base(pageURL.Path), ".html")
	}
	return &SubtitleProviderDetails{Title: title, Languages: languages}, nil
}

func scoreSubtitleCatCandidate(candidate string, req SubtitleProviderSearchRequest) int {
	candidateLower := strings.ToLower(candidate)
	targetFile := strings.ToLower(req.FileName)
	if targetFile == "" && len(req.Queries) > 0 {
		targetFile = strings.ToLower(req.Queries[0])
	}

	targetEpisode := subtitleEpisodeTag.FindString(targetFile)
	candidateEpisode := subtitleEpisodeTag.FindString(candidateLower)
	if targetEpisode != "" && candidateEpisode != "" && !strings.EqualFold(targetEpisode, candidateEpisode) {
		return -1 // TV 场景禁止串集
	}

	score := 0
	if targetEpisode != "" && candidateEpisode != "" {
		score += 45
	}
	targetYear := req.Year
	if targetYear <= 0 {
		if m := subtitleYearTag.FindStringSubmatch(targetFile); len(m) > 1 {
			targetYear, _ = strconv.Atoi(m[1])
		}
	}
	candidateYear := 0
	if m := subtitleYearTag.FindStringSubmatch(candidateLower); len(m) > 1 {
		candidateYear, _ = strconv.Atoi(m[1])
	}
	if targetYear > 0 && candidateYear > 0 {
		if targetYear == candidateYear {
			score += 18
		} else {
			score -= 30
		}
	}

	targetTitle := req.Title
	if targetTitle == "" {
		if req.MediaType == "episode" {
			targetTitle = ParseEpisodeFilename(req.FileName).SeriesTitle
		} else {
			targetTitle = ParseMovieFilename(req.FileName).Title
		}
	}
	targetTokens := subtitleMatchTokens(targetTitle)
	candidateTokens := subtitleMatchTokens(candidate)
	if len(targetTokens) > 0 {
		matches := 0
		for token := range targetTokens {
			if candidateTokens[token] {
				matches++
			}
		}
		score += int(float64(matches) / float64(len(targetTokens)) * 35)
		normTitle := strings.Join(sortedTokenKeys(targetTokens), " ")
		normCandidate := strings.Join(sortedTokenKeys(candidateTokens), " ")
		if normTitle != "" && (strings.Contains(normCandidate, normTitle) || strings.Contains(normTitle, normCandidate)) {
			score += 10
		}
	}
	if strings.EqualFold(strings.TrimSuffix(candidate, ".srt"), strings.TrimSuffix(req.FileName, path.Ext(req.FileName))) {
		score += 15
	}
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func subtitleMatchTokens(value string) map[string]bool {
	value = strings.ToLower(value)
	value = regexp.MustCompile(`[^\pL\pN]+`).ReplaceAllString(value, " ")
	stop := map[string]bool{
		"1080p": true, "2160p": true, "720p": true, "web": true, "dl": true,
		"bluray": true, "remux": true, "x264": true, "x265": true, "h264": true, "h265": true,
		"hevc": true, "aac": true, "dts": true, "hdr": true, "srt": true,
	}
	out := make(map[string]bool)
	for _, token := range strings.Fields(value) {
		if len([]rune(token)) < 2 || stop[token] {
			continue
		}
		out[token] = true
	}
	return out
}

func sortedTokenKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func subtitleCatLanguageFromDownloadPath(downloadPath string) string {
	m := subtitleCatLangSuffixPattern.FindStringSubmatch(downloadPath)
	if len(m) < 2 {
		return ""
	}
	return normalizeSubtitleLanguageCode(m[1])
}

func subtitleCatLanguageName(code string) string {
	switch normalizeSubtitleLanguageCode(code) {
	case "zh-CN":
		return "简体中文"
	case "zh-TW":
		return "繁体中文"
	case "en":
		return "English"
	case "ja":
		return "日本語"
	case "ko":
		return "한국어"
	default:
		return getLanguageName(code)
	}
}

func encodeSubtitleCatRef(kind, ref string) string {
	return subtitleCatProviderName + ":" + kind + ":" + base64.RawURLEncoding.EncodeToString([]byte(ref))
}

func decodeSubtitleCatRef(id string) (string, string, error) {
	parts := strings.SplitN(id, ":", 3)
	if len(parts) != 3 || parts[0] != subtitleCatProviderName {
		return "", "", fmt.Errorf("invalid SubtitleCat ref")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", "", err
	}
	ref := string(raw)
	if !strings.HasPrefix(ref, "/subs/") {
		return "", "", fmt.Errorf("invalid SubtitleCat ref path")
	}
	return parts[1], ref, nil
}

func looksLikeSubtitleCatChallenge(body []byte) bool {
	text := strings.ToLower(string(body))
	return strings.Contains(text, "cf-chl-") ||
		strings.Contains(text, "cloudflare ray id") ||
		strings.Contains(text, "just a moment...") ||
		strings.Contains(text, "attention required! | cloudflare")
}

func walkHTML(n *html.Node, fn func(*html.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, fn)
	}
}

func htmlAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}

func nearestHTMLAncestor(n *html.Node, tag string) *html.Node {
	for current := n; current != nil; current = current.Parent {
		if current.Type == html.ElementNode && strings.EqualFold(current.Data, tag) {
			return current
		}
	}
	return n
}

func htmlNodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			text := strings.TrimSpace(node.Data)
			if text != "" {
				if b.Len() > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(text)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func dedupeSubtitleCatQueries(in []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(in))
	for _, query := range in {
		query = strings.TrimSpace(query)
		key := strings.ToLower(query)
		if query == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, query)
	}
	return out
}

func cloneSubtitleSearchResults(in []SubtitleSearchResult) []SubtitleSearchResult {
	out := append([]SubtitleSearchResult(nil), in...)
	for i := range out {
		out[i].AvailableLanguages = append([]string(nil), in[i].AvailableLanguages...)
	}
	return out
}

func (p *SubtitleCatProvider) pruneExpiredCachesLocked() {
	now := time.Now()
	for key, entry := range p.searchCache {
		if now.After(entry.ExpiresAt) {
			delete(p.searchCache, key)
		}
	}
	for key, entry := range p.detailCache {
		if now.After(entry.ExpiresAt) {
			delete(p.detailCache, key)
		}
	}
}

func (p *SubtitleCatProvider) logParseFailure(operation, reason string, err error) {
	if p.logger == nil {
		return
	}
	p.logger.Warnw("subtitle provider parse failed",
		"provider", subtitleCatProviderName,
		"operation", operation,
		"reason", reason,
		"error", err,
	)
}
