package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== 番号识别与解析 ====================

// 番号正则模式（覆盖主流格式）
var (
	// 标准番号：ABC-123, SSIS-001, MIDV-123
	stdCodeRegex = regexp.MustCompile(`(?i)\b([A-Z]{2,10})-?(\d{3,8})\b`)
	// FC2 番号：FC2-PPV-1234567 或 FC2PPV-1234567
	fc2CodeRegex = regexp.MustCompile(`(?i)\bFC2-?PPV-?(\d{5,8})\b`)
	// 无码番号：n1234, k1234, 1pondo 格式 123456_789
	uncensoredRegex = regexp.MustCompile(`(?i)\b(\d{6})[-_](\d{3,4})\b`)
	// HEYZO 格式：HEYZO-1234
	heyzoRegex = regexp.MustCompile(`(?i)\bHEYZO-?(\d{4})\b`)
)

// AdultMetadata 番号刮削结果
type AdultMetadata struct {
	Code        string   `json:"code"`         // 番号（如 SSIS-001）
	Title       string   `json:"title"`        // 标题
	OriginalTitle string `json:"original_title"` // 原始标题（日文）— P1 新增
	Plot        string   `json:"plot"`         // 剧情简介 — P1 新增
	OriginalPlot string  `json:"original_plot"` // 原始剧情简介 — P1 新增
	Cover       string   `json:"cover"`        // 封面图 URL（大图/poster）
	Thumb       string   `json:"thumb"`        // 缩略图（小图/fanart 背景） — P1 新增
	Actresses   []string `json:"actresses"`    // 演员列表
	ActorPhotos map[string]string `json:"actor_photos"` // 演员头像 URL（演员名 -> URL）— P1 新增
	Studio      string   `json:"studio"`       // 片商/制作商
	Label       string   `json:"label"`        // 发行商
	Series      string   `json:"series"`       // 系列名
	Genres      []string `json:"genres"`       // 类型标签
	ReleaseDate string   `json:"release_date"` // 发行日期
	Duration    int      `json:"duration"`     // 时长（分钟）
	Rating      float64  `json:"rating"`       // 评分
	Trailer     string   `json:"trailer"`      // 预告片 URL — P1 新增
	ExtraFanart []string `json:"extra_fanart"` // 额外剧照 URL 列表 — P1 新增
	Director    string   `json:"director"`     // 导演 — P1 新增
	Source      string   `json:"source"`       // 数据来源（javbus/javdb/fc2/freejavbt/jav321/python）
}

// ==================== 成人内容刮削服务 ====================

// AdultScraperService 番号刮削服务（混合架构：Go 原生 + Python 微服务）
type AdultScraperService struct {
	cfg       *config.Config
	mediaRepo *repository.MediaRepo
	logger    *zap.SugaredLogger
	client    *http.Client

	// NFO 写入服务（可选）：刮削成功后自动生成 .nfo 文件
	// 使 Emby/Jellyfin/Infuse 等客户端能正确识别番号元数据
	nfoService *NFOService
}

// NewAdultScraperService 创建番号刮削服务
func NewAdultScraperService(cfg *config.Config, mediaRepo *repository.MediaRepo, logger *zap.SugaredLogger) *AdultScraperService {
	return &AdultScraperService{
		cfg:       cfg,
		mediaRepo: mediaRepo,
		logger:    logger,
		client: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     60 * time.Second,
			},
		},
	}
}

// SetNFOService 注入 NFO 写入服务（用于刮削成功后生成 .nfo 文件）
func (s *AdultScraperService) SetNFOService(nfo *NFOService) {
	s.nfoService = nfo
}

// IsEnabled 检查服务是否可用
func (s *AdultScraperService) IsEnabled() bool {
	return s.cfg.AdultScraper.Enabled
}

// ==================== 番号识别 ====================

// ParseCode 从文件名/标题中识别番号
// 返回: (番号, 番号类型)
// 类型: "standard" / "fc2" / "uncensored" / "heyzo" / ""
func ParseCode(input string) (string, string) {
	input = strings.ToUpper(input)

	// 1. FC2 番号（优先匹配，避免被标准模式误匹配）
	if matches := fc2CodeRegex.FindStringSubmatch(input); len(matches) > 1 {
		return "FC2-PPV-" + matches[1], "fc2"
	}

	// 2. HEYZO 番号
	if matches := heyzoRegex.FindStringSubmatch(input); len(matches) > 1 {
		return "HEYZO-" + matches[1], "heyzo"
	}

	// 3. 无码番号（1pondo/caribbeancom 格式）
	if matches := uncensoredRegex.FindStringSubmatch(input); len(matches) > 2 {
		return matches[1] + "-" + matches[2], "uncensored"
	}

	// 4. 标准番号（最后匹配，范围最广）
	if matches := stdCodeRegex.FindStringSubmatch(input); len(matches) > 2 {
		prefix := matches[1]
		num := matches[2]
		// 排除常见误匹配（分辨率、编码等）
		excludePrefixes := map[string]bool{
			"MP": true, "AVC": true, "AAC": true, "DTS": true,
			"AC": true, "HD": true, "SD": true, "BD": true,
			"CD": true, "DVD": true, "WEB": true, "MKV": true,
			"AVI": true, "HEVC": true, "FHD": true, "UHD": true,
		}
		if excludePrefixes[prefix] {
			return "", ""
		}
		return prefix + "-" + num, "standard"
	}

	return "", ""
}

// ==================== Go 原生爬虫：JavBus ====================

// scrapeJavBus 从 JavBus 刮削番号元数据
func (s *AdultScraperService) scrapeJavBus(code string) (*AdultMetadata, error) {
	baseURL := s.cfg.AdultScraper.JavBusURL
	if baseURL == "" {
		baseURL = "https://www.javbus.com"
	}

	targetURL := fmt.Sprintf("%s/%s", strings.TrimRight(baseURL, "/"), code)
	s.logger.Debugf("JavBus 刮削: %s", targetURL)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	setAdultScraperHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("JavBus 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("JavBus 未找到番号: %s", code)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JavBus 返回异常状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	return s.parseJavBusHTML(string(body), code)
}

// parseJavBusHTML 解析 JavBus HTML 页面（纯正则，不依赖 goquery）
func (s *AdultScraperService) parseJavBusHTML(html, code string) (*AdultMetadata, error) {
	meta := &AdultMetadata{
		Code:   code,
		Source: "javbus",
	}

	// 提取标题：<h3>SSIS-001 标题内容</h3>
	titleRe := regexp.MustCompile(`<h3>([^<]+)</h3>`)
	if matches := titleRe.FindStringSubmatch(html); len(matches) > 1 {
		meta.Title = strings.TrimSpace(matches[1])
		// 去掉番号前缀
		meta.Title = strings.TrimPrefix(meta.Title, code+" ")
		meta.Title = strings.TrimPrefix(meta.Title, code)
		meta.Title = strings.TrimSpace(meta.Title)
	}

	// 提取封面图：<a class="bigImage" href="..."><img src="..." ...>
	coverRe := regexp.MustCompile(`class="bigImage"[^>]*href="([^"]+)"`)
	if matches := coverRe.FindStringSubmatch(html); len(matches) > 1 {
		meta.Cover = matches[1]
	}

	// 提取制作商
	studioRe := regexp.MustCompile(`製作商:</span>\s*<a[^>]*>([^<]+)</a>`)
	if matches := studioRe.FindStringSubmatch(html); len(matches) > 1 {
		meta.Studio = strings.TrimSpace(matches[1])
	}

	// 提取發行商
	labelRe := regexp.MustCompile(`發行商:</span>\s*<a[^>]*>([^<]+)</a>`)
	if matches := labelRe.FindStringSubmatch(html); len(matches) > 1 {
		meta.Label = strings.TrimSpace(matches[1])
	}

	// 提取系列
	seriesRe := regexp.MustCompile(`系列:</span>\s*<a[^>]*>([^<]+)</a>`)
	if matches := seriesRe.FindStringSubmatch(html); len(matches) > 1 {
		meta.Series = strings.TrimSpace(matches[1])
	}

	// 提取发行日期
	dateRe := regexp.MustCompile(`發行日期:</span>\s*(\d{4}-\d{2}-\d{2})`)
	if matches := dateRe.FindStringSubmatch(html); len(matches) > 1 {
		meta.ReleaseDate = matches[1]
	}

	// 提取时长
	durationRe := regexp.MustCompile(`長度:</span>\s*(\d+)分鐘`)
	if matches := durationRe.FindStringSubmatch(html); len(matches) > 1 {
		fmt.Sscanf(matches[1], "%d", &meta.Duration)
	}

	// 提取演员列表（并同时抓取演员头像 URL —— P1 新增）
	if meta.ActorPhotos == nil {
		meta.ActorPhotos = make(map[string]string)
	}
	actressBlockRe := regexp.MustCompile(`(?is)<a[^>]*class="avatar-box"[^>]*>(.*?)</a>`)
	for _, blk := range actressBlockRe.FindAllStringSubmatch(html, -1) {
		if len(blk) < 2 {
			continue
		}
		inner := blk[1]
		nameRe := regexp.MustCompile(`<span>([^<]+)</span>`)
		imgRe := regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
		nameMatch := nameRe.FindStringSubmatch(inner)
		imgMatch := imgRe.FindStringSubmatch(inner)
		if len(nameMatch) > 1 {
			name := strings.TrimSpace(nameMatch[1])
			meta.Actresses = append(meta.Actresses, name)
			if len(imgMatch) > 1 {
				photo := cleanURL(imgMatch[1])
				// 跳过 JavBus 默认头像（空白图）
				if !strings.Contains(photo, "nowprinting") && !strings.Contains(photo, "default") {
					meta.ActorPhotos[name] = photo
				}
			}
		}
	}

	// 提取类型标签
	genreRe := regexp.MustCompile(`<span class="genre"><label><a[^>]*>([^<]+)</a>`)
	genreMatches := genreRe.FindAllStringSubmatch(html, -1)
	for _, m := range genreMatches {
		if len(m) > 1 {
			meta.Genres = append(meta.Genres, strings.TrimSpace(m[1]))
		}
	}

	// P1：提取剧照 ExtraFanart —— JavBus 样张区块
	// <a class="sample-box" href="sample.jpg">
	fanartRe := regexp.MustCompile(`(?is)<a[^>]+class="sample-box"[^>]+href="([^"]+)"`)
	for _, m := range fanartRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			meta.ExtraFanart = append(meta.ExtraFanart, cleanURL(m[1]))
		}
	}

	// P1：提取 Trailer 预告片（部分详情页含有 preview-video 或 video 标签）
	trailerRe := regexp.MustCompile(`(?is)<video[^>]+src="([^"]+\.(?:mp4|webm|m3u8))"`)
	if m := trailerRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Trailer = cleanURL(m[1])
	}

	if meta.Title == "" {
		return nil, fmt.Errorf("JavBus 解析失败：未提取到标题")
	}

	return meta, nil
}

// ==================== Go 原生爬虫：JavDB ====================

// scrapeJavDB 从 JavDB 刮削番号元数据
func (s *AdultScraperService) scrapeJavDB(code string) (*AdultMetadata, error) {
	baseURL := s.cfg.AdultScraper.JavDBURL
	if baseURL == "" {
		baseURL = "https://javdb.com"
	}

	// 第一步：搜索番号
	searchURL := fmt.Sprintf("%s/search?q=%s&f=all", strings.TrimRight(baseURL, "/"), url.QueryEscape(code))
	s.logger.Debugf("JavDB 搜索: %s", searchURL)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	setAdultScraperHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("JavDB 搜索请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JavDB 搜索返回异常状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取搜索结果失败: %w", err)
	}

	// 从搜索结果中提取详情页链接
	detailPath := s.extractJavDBDetailPath(string(body), code)
	if detailPath == "" {
		return nil, fmt.Errorf("JavDB 未找到番号: %s", code)
	}

	// 第二步：访问详情页
	randomDelay(1000, 2000) // 请求间隔，避免被封
	detailURL := fmt.Sprintf("%s%s", strings.TrimRight(baseURL, "/"), detailPath)
	s.logger.Debugf("JavDB 详情: %s", detailURL)

	req2, err := http.NewRequest("GET", detailURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造详情请求失败: %w", err)
	}
	setAdultScraperHeaders(req2)

	resp2, err := s.client.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("JavDB 详情请求失败: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JavDB 详情返回异常状态码: %d", resp2.StatusCode)
	}

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		return nil, fmt.Errorf("读取详情页失败: %w", err)
	}

	return s.parseJavDBHTML(string(body2), code)
}

// extractJavDBDetailPath 从 JavDB 搜索结果中提取详情页路径
func (s *AdultScraperService) extractJavDBDetailPath(html, code string) string {
	// 搜索结果中的链接格式：<a href="/v/xxxxx" class="box" ...>
	// 匹配包含目标番号的搜索结果项
	codeUpper := strings.ToUpper(code)
	// 先找所有搜索结果项
	itemRe := regexp.MustCompile(`<a\s+href="(/v/[^"]+)"[^>]*class="box"[^>]*>[\s\S]*?</a>`)
	items := itemRe.FindAllStringSubmatch(html, 20)

	for _, item := range items {
		if len(item) > 1 {
			// 检查该项是否包含目标番号
			if strings.Contains(strings.ToUpper(item[0]), codeUpper) {
				return item[1]
			}
		}
	}

	// 备选：直接匹配第一个搜索结果
	firstRe := regexp.MustCompile(`<a\s+href="(/v/[^"]+)"`)
	if matches := firstRe.FindStringSubmatch(html); len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// parseJavDBHTML 解析 JavDB 详情页 HTML
func (s *AdultScraperService) parseJavDBHTML(html, code string) (*AdultMetadata, error) {
	meta := &AdultMetadata{
		Code:   code,
		Source: "javdb",
	}

	// 提取标题：<h2 class="title is-4"><strong>标题</strong></h2>
	titleRe := regexp.MustCompile(`<h2[^>]*class="title[^"]*"[^>]*>\s*<strong[^>]*>([^<]+)</strong>`)
	if matches := titleRe.FindStringSubmatch(html); len(matches) > 1 {
		meta.Title = strings.TrimSpace(matches[1])
		// 去掉番号前缀
		meta.Title = strings.TrimPrefix(meta.Title, code+" ")
		meta.Title = strings.TrimPrefix(meta.Title, code)
		meta.Title = strings.TrimSpace(meta.Title)
	}

	// 提取封面图：<img src="..." class="video-cover" ...>
	coverRe := regexp.MustCompile(`<img[^>]*src="([^"]+)"[^>]*class="video-cover"`)
	if matches := coverRe.FindStringSubmatch(html); len(matches) > 1 {
		meta.Cover = matches[1]
	}
	// 备选封面匹配
	if meta.Cover == "" {
		coverRe2 := regexp.MustCompile(`class="video-cover"[^>]*src="([^"]+)"`)
		if matches := coverRe2.FindStringSubmatch(html); len(matches) > 1 {
			meta.Cover = matches[1]
		}
	}

	// 提取评分：<span class="score"><span class="value">4.5</span></span>
	ratingRe := regexp.MustCompile(`class="score"[^>]*>\s*<span[^>]*class="value"[^>]*>([0-9.]+)`)
	if matches := ratingRe.FindStringSubmatch(html); len(matches) > 1 {
		fmt.Sscanf(matches[1], "%f", &meta.Rating)
	}

	// 提取日期：日期: 2024-01-01
	dateRe := regexp.MustCompile(`日期[：:]\s*</strong>\s*(\d{4}-\d{2}-\d{2})`)
	if matches := dateRe.FindStringSubmatch(html); len(matches) > 1 {
		meta.ReleaseDate = matches[1]
	}

	// 提取时長
	durationRe := regexp.MustCompile(`時長[：:]\s*</strong>\s*(\d+)\s*分鐘`)
	if matches := durationRe.FindStringSubmatch(html); len(matches) > 1 {
		fmt.Sscanf(matches[1], "%d", &meta.Duration)
	}

	// 提取片商
	studioRe := regexp.MustCompile(`片商[：:]\s*</strong>\s*<a[^>]*>([^<]+)</a>`)
	if matches := studioRe.FindStringSubmatch(html); len(matches) > 1 {
		meta.Studio = strings.TrimSpace(matches[1])
	}

	// 提取演員
	actressRe := regexp.MustCompile(`演員[：:][\s\S]*?<div[^>]*class="panel-block"[^>]*>([\s\S]*?)</div>`)
	if matches := actressRe.FindStringSubmatch(html); len(matches) > 1 {
		actorLinkRe := regexp.MustCompile(`<a[^>]*>([^<]+)</a>`)
		actorMatches := actorLinkRe.FindAllStringSubmatch(matches[1], -1)
		for _, m := range actorMatches {
			if len(m) > 1 {
				name := strings.TrimSpace(m[1])
				if name != "" {
					meta.Actresses = append(meta.Actresses, name)
				}
			}
		}
	}

	// 提取類別標籤
	genreRe := regexp.MustCompile(`類別[：:][\s\S]*?<div[^>]*class="panel-block"[^>]*>([\s\S]*?)</div>`)
	if matches := genreRe.FindStringSubmatch(html); len(matches) > 1 {
		tagRe := regexp.MustCompile(`<a[^>]*>([^<]+)</a>`)
		tagMatches := tagRe.FindAllStringSubmatch(matches[1], -1)
		for _, m := range tagMatches {
			if len(m) > 1 {
				tag := strings.TrimSpace(m[1])
				if tag != "" {
					meta.Genres = append(meta.Genres, tag)
				}
			}
		}
	}

	// ==================== P1 扩展字段提取 ====================

	// P1：导演
	directorRe := regexp.MustCompile(`導演[：:]\s*</strong>\s*<a[^>]*>([^<]+)</a>`)
	if m := directorRe.FindStringSubmatch(html); len(m) > 1 {
		meta.Director = strings.TrimSpace(m[1])
	}

	// P1：系列
	if meta.Series == "" {
		seriesReJdb := regexp.MustCompile(`系列[：:]\s*</strong>\s*<a[^>]*>([^<]+)</a>`)
		if m := seriesReJdb.FindStringSubmatch(html); len(m) > 1 {
			meta.Series = strings.TrimSpace(m[1])
		}
	}
	// P1：发行商
	if meta.Label == "" {
		labelReJdb := regexp.MustCompile(`發行[：:]\s*</strong>\s*<a[^>]*>([^<]+)</a>`)
		if m := labelReJdb.FindStringSubmatch(html); len(m) > 1 {
			meta.Label = strings.TrimSpace(m[1])
		}
	}

	// P1：剧照 ExtraFanart（JavDB 详情页的 tile-images）
	fanartReJdb := regexp.MustCompile(`(?is)<a[^>]+class="tile-item"[^>]+href="([^"]+\.(?:jpg|jpeg|png|webp))"`)
	for _, m := range fanartReJdb.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			meta.ExtraFanart = append(meta.ExtraFanart, cleanURL(m[1]))
		}
	}
	// 备选剧照匹配：preview-images data-src
	if len(meta.ExtraFanart) == 0 {
		fanartReJdb2 := regexp.MustCompile(`(?is)<div[^>]+class="preview-images"[^>]*>([\s\S]*?)</div>`)
		if m := fanartReJdb2.FindStringSubmatch(html); len(m) > 1 {
			imgRe := regexp.MustCompile(`href="([^"]+\.(?:jpg|jpeg|png|webp))"`)
			for _, im := range imgRe.FindAllStringSubmatch(m[1], -1) {
				if len(im) > 1 {
					meta.ExtraFanart = append(meta.ExtraFanart, cleanURL(im[1]))
				}
			}
		}
	}

	// P1：Trailer 预告片
	trailerReJdb := regexp.MustCompile(`(?is)<video[^>]+src="([^"]+\.(?:mp4|webm|m3u8))"`)
	if m := trailerReJdb.FindStringSubmatch(html); len(m) > 1 {
		meta.Trailer = cleanURL(m[1])
	} else {
		// 备选：video-meta-panel 中的 preview-video-tag
		trailerReJdb2 := regexp.MustCompile(`(?is)data-video[^=]*="([^"]+\.(?:mp4|m3u8))"`)
		if m := trailerReJdb2.FindStringSubmatch(html); len(m) > 1 {
			meta.Trailer = cleanURL(m[1])
		}
	}

	// P1：演员头像（JavDB 演员区块）
	if meta.ActorPhotos == nil {
		meta.ActorPhotos = make(map[string]string)
	}
	// <a class="actor"><img src="..." ...><span>演员名</span></a>
	actorBlockRe := regexp.MustCompile(`(?is)<img[^>]+src="([^"]+)"[^>]*>\s*</a>\s*<strong[^>]*>\s*<a[^>]*>([^<]+)</a>`)
	for _, m := range actorBlockRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 2 {
			name := strings.TrimSpace(m[2])
			photo := cleanURL(m[1])
			if name != "" && photo != "" {
				meta.ActorPhotos[name] = photo
			}
		}
	}

	if meta.Title == "" {
		return nil, fmt.Errorf("JavDB 解析失败：未提取到标题")
	}

	return meta, nil
}

// ==================== Python 微服务 Fallback ====================

// scrapePythonService 调用 Python 微服务刮削（Cloudflare 等强反爬场景的 fallback）
// Python 微服务需要独立部署，提供 REST API：
//   POST /api/scrape
//   Body: {"code": "SSIS-001", "sources": ["javdb", "javbus", "fanza"]}
//   Response: AdultMetadata JSON
func (s *AdultScraperService) scrapePythonService(code string) (*AdultMetadata, error) {
	serviceURL := s.cfg.AdultScraper.PythonServiceURL
	if serviceURL == "" {
		return nil, fmt.Errorf("Python 刮削微服务未配置")
	}

	apiURL := fmt.Sprintf("%s/api/scrape", strings.TrimRight(serviceURL, "/"))
	s.logger.Debugf("Python 微服务刮削: %s -> %s", code, apiURL)

	// 构造请求
	reqBody := fmt.Sprintf(`{"code":"%s","sources":["javdb","javbus","fanza","fc2"]}`, code)
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("构造 Python 微服务请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Python 微服务可能需要 API Key 认证
	if apiKey := s.cfg.AdultScraper.PythonServiceAPIKey; apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	// Python 微服务超时较长（可能需要渲染 JS）
	pythonClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := pythonClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Python 微服务请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Python 微服务返回异常: HTTP %d, body=%s", resp.StatusCode, string(body))
	}

	var meta AdultMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("解析 Python 微服务响应失败: %w", err)
	}

	meta.Source = "python"
	return &meta, nil
}

// ==================== 混合调度：Go 优先，Python Fallback ====================

// ScrapeByCode 根据番号刮削元数据（混合调度）
// 策略：
//  0. 若启用聚合模式，直接走 ScrapeByCodeAggregated（并发+合并，最完整）
//  1. Fanza 优先（官方数据最权威）
//  2. JavBus
//  3. JavDB
//  4. Freejavbt（中文元数据优秀）
//  5. JAV321（中文简介丰富）
//  6. MGStage（MGS 系列番号专用）
//  7. FC2Hub（FC2-PPV 无码作品）
//  8. Python 微服务 fallback
//
// 成功结果会自动执行：演员/片商/系列/标签规范化 + 日文翻译
func (s *AdultScraperService) ScrapeByCode(code string) (*AdultMetadata, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("番号刮削功能未启用")
	}

	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, fmt.Errorf("番号不能为空")
	}

	// P2：聚合模式（并发+合并）
	if s.cfg.AdultScraper.EnableAggregatedMode {
		merged, _, err := s.ScrapeByCodeAggregated(code)
		return merged, err
	}

	s.logger.Infof("开始番号刮削: %s", code)

	// onSuccess 统一处理成功结果（规范化 + 翻译）
	onSuccess := func(meta *AdultMetadata) *AdultMetadata {
		NormalizeMetadata(meta)
		s.TranslateAdultMetadata(meta)
		return meta
	}

	// P2：Fanza 优先（官方数据）
	if s.cfg.AdultScraper.EnableFanza {
		meta, err := s.scrapeFanza(code)
		if err == nil {
			s.logger.Infof("Fanza 刮削成功: %s -> %s", code, meta.Title)
			return onSuccess(meta), nil
		}
		s.logger.Debugf("Fanza 刮削失败: %s - %v", code, err)
		randomDelay(1500, 3000)
	}

	// 第一层：Go 原生爬虫 - JavBus
	if s.cfg.AdultScraper.EnableJavBus {
		meta, err := s.scrapeJavBus(code)
		if err == nil {
			s.logger.Infof("JavBus 刮削成功: %s -> %s", code, meta.Title)
			return onSuccess(meta), nil
		}
		s.logger.Debugf("JavBus 刮削失败: %s - %v", code, err)
		randomDelay(1500, 3000)
	}

	// 第二层：Go 原生爬虫 - JavDB
	if s.cfg.AdultScraper.EnableJavDB {
		meta, err := s.scrapeJavDB(code)
		if err == nil {
			s.logger.Infof("JavDB 刮削成功: %s -> %s", code, meta.Title)
			return onSuccess(meta), nil
		}
		s.logger.Debugf("JavDB 刮削失败: %s - %v", code, err)
		randomDelay(1500, 3000)
	}

	// 第三层：Go 原生爬虫 - Freejavbt（P1 新增）
	if s.cfg.AdultScraper.EnableFreejavbt {
		meta, err := s.scrapeFreejavbt(code)
		if err == nil {
			s.logger.Infof("Freejavbt 刮削成功: %s -> %s", code, meta.Title)
			return onSuccess(meta), nil
		}
		s.logger.Debugf("Freejavbt 刮削失败: %s - %v", code, err)
		randomDelay(1500, 3000)
	}

	// 第四层：Go 原生爬虫 - JAV321（P1 新增）
	if s.cfg.AdultScraper.EnableJav321 {
		meta, err := s.scrapeJav321(code)
		if err == nil {
			s.logger.Infof("JAV321 刮削成功: %s -> %s", code, meta.Title)
			return onSuccess(meta), nil
		}
		s.logger.Debugf("JAV321 刮削失败: %s - %v", code, err)
		randomDelay(1500, 3000)
	}

	// P2：MGStage（MGS/200GANA 系列番号）
	if s.cfg.AdultScraper.EnableMGStage {
		meta, err := s.scrapeMGStage(code)
		if err == nil {
			s.logger.Infof("MGStage 刮削成功: %s -> %s", code, meta.Title)
			return onSuccess(meta), nil
		}
		s.logger.Debugf("MGStage 刮削失败: %s - %v", code, err)
		randomDelay(1500, 3000)
	}

	// P2：FC2Hub（FC2-PPV 无码）
	if s.cfg.AdultScraper.EnableFC2Hub {
		meta, err := s.scrapeFC2Hub(code)
		if err == nil {
			s.logger.Infof("FC2Hub 刮削成功: %s -> %s", code, meta.Title)
			return onSuccess(meta), nil
		}
		s.logger.Debugf("FC2Hub 刮削失败: %s - %v", code, err)
		randomDelay(1500, 3000)
	}

	// Fallback：Python 微服务（处理 Cloudflare 等强反爬）
	if s.cfg.AdultScraper.PythonServiceURL != "" {
		meta, err := s.scrapePythonService(code)
		if err == nil {
			s.logger.Infof("Python 微服务刮削成功: %s -> %s", code, meta.Title)
			return onSuccess(meta), nil
		}
		s.logger.Debugf("Python 微服务刮削失败: %s - %v", code, err)
	}

	return nil, fmt.Errorf("所有数据源均未找到番号 %s 的元数据", code)
}

// ==================== 应用元数据到 Media ====================

// ApplyToMedia 将番号刮削结果应用到 Media 模型
func (s *AdultScraperService) ApplyToMedia(media *model.Media, meta *AdultMetadata, mode string) error {
	if meta == nil {
		return fmt.Errorf("元数据为空")
	}

	if mode == "primary" {
		// 主数据源模式：覆盖写入
		if meta.Title != "" {
			media.Title = meta.Title
		}
		if meta.Cover != "" {
			// 下载封面图到本地
			localPath, err := s.downloadCover(media, meta.Cover)
			if err == nil {
				media.PosterPath = localPath
			} else {
				s.logger.Debugf("下载封面失败: %v", err)
			}
		}
		if len(meta.Genres) > 0 {
			media.Genres = strings.Join(meta.Genres, ",")
		}
		if meta.Studio != "" {
			media.Studio = meta.Studio
		}
		if meta.ReleaseDate != "" {
			media.Premiered = meta.ReleaseDate
			if len(meta.ReleaseDate) >= 4 {
				fmt.Sscanf(meta.ReleaseDate[:4], "%d", &media.Year)
			}
		}
		if meta.Duration > 0 {
			media.Runtime = meta.Duration
		}
		if meta.Rating > 0 {
			// JavDB 评分是 5 分制，转换为 10 分制
			if meta.Source == "javdb" && meta.Rating <= 5 {
				media.Rating = meta.Rating * 2
			} else {
				media.Rating = meta.Rating
			}
		}
		// 演员信息存入 Tagline 字段（临时方案，后续可扩展到 Person 表）
		if len(meta.Actresses) > 0 {
			media.Tagline = "演员: " + strings.Join(meta.Actresses, ", ")
		}
		// 原始标题保存番号
		media.OrigTitle = meta.Code
	} else {
		// 补充模式：仅填充缺失字段
		if media.Title == "" && meta.Title != "" {
			media.Title = meta.Title
		}
		if media.PosterPath == "" && meta.Cover != "" {
			localPath, err := s.downloadCover(media, meta.Cover)
			if err == nil {
				media.PosterPath = localPath
			}
		}
		if media.Genres == "" && len(meta.Genres) > 0 {
			media.Genres = strings.Join(meta.Genres, ",")
		}
		if media.Studio == "" && meta.Studio != "" {
			media.Studio = meta.Studio
		}
		if media.Premiered == "" && meta.ReleaseDate != "" {
			media.Premiered = meta.ReleaseDate
		}
		if media.Year == 0 && meta.ReleaseDate != "" && len(meta.ReleaseDate) >= 4 {
			fmt.Sscanf(meta.ReleaseDate[:4], "%d", &media.Year)
		}
		if media.Runtime == 0 && meta.Duration > 0 {
			media.Runtime = meta.Duration
		}
		if media.Rating == 0 && meta.Rating > 0 {
			if meta.Source == "javdb" && meta.Rating <= 5 {
				media.Rating = meta.Rating * 2
			} else {
				media.Rating = meta.Rating
			}
		}
		if media.OrigTitle == "" {
			media.OrigTitle = meta.Code
		}
	}

	// ==================== P1：额外字段填充（剧情/原始标题/导演）====================
	if meta.Plot != "" && (mode == "primary" || media.Overview == "") {
		media.Overview = meta.Plot
	}
	if meta.OriginalTitle != "" && (mode == "primary" || media.OrigTitle == "" || media.OrigTitle == meta.Code) {
		// 保留番号到 OrigTitle 需求较常见；这里把原标题追加到 Overview 开头以便查阅
		if !strings.Contains(media.Overview, meta.OriginalTitle) && meta.OriginalTitle != meta.Title {
			media.Overview = meta.OriginalTitle + "\n\n" + media.Overview
		}
	}

	// ==================== P1：剧照 / 演员头像下载 ====================
	// 仅在本地文件系统时生效（webdav:// 不支持写入）
	if media.FilePath != "" && !IsWebDAVPath(media.FilePath) {
		s.DownloadExtraFanart(media, meta)
		s.DownloadActorPhotos(media, meta)
	}

	// ==================== P2：封面自动裁剪成竖版 poster（Emby 媒体墙更美观）====================
	if s.cfg.AdultScraper.EnablePosterCrop && media.PosterPath != "" &&
		media.FilePath != "" && !IsWebDAVPath(media.FilePath) {
		if posterPath, err := s.GeneratePosterForMedia(media.FilePath, media.PosterPath); err != nil {
			s.logger.Debugf("封面裁剪失败（不影响主流程）: %v", err)
		} else if posterPath != "" {
			s.logger.Debugf("封面裁剪成功: %s", posterPath)
		}
	}

	// ==================== NFO 自动生成（让 Emby/Jellyfin/Infuse 能识别番号元数据）====================
	// 只在本地文件系统生成（webdav:// 不支持写入）
	if s.nfoService != nil && media.FilePath != "" && !IsWebDAVPath(media.FilePath) {
		if nfoPath, err := s.nfoService.WriteAdultNFO(media.FilePath, media, meta); err != nil {
			s.logger.Debugf("NFO 生成失败（不影响主流程）: %v", err)
		} else {
			s.logger.Debugf("NFO 生成成功: %s", nfoPath)
		}
	}

	return nil
}

// downloadCover 下载封面图到本地缓存
func (s *AdultScraperService) downloadCover(media *model.Media, coverURL string) (string, error) {
	if coverURL == "" {
		return "", fmt.Errorf("封面 URL 为空")
	}

	// 确保 URL 完整
	if !strings.HasPrefix(coverURL, "http") {
		coverURL = "https:" + coverURL
	}

	req, err := http.NewRequest("GET", coverURL, nil)
	if err != nil {
		return "", err
	}
	setAdultScraperHeaders(req)
	// 设置 Referer（部分站点需要）
	if strings.Contains(coverURL, "javbus") {
		req.Header.Set("Referer", "https://www.javbus.com/")
	} else if strings.Contains(coverURL, "javdb") {
		req.Header.Set("Referer", "https://javdb.com/")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载封面失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载封面返回异常状态码: %d", resp.StatusCode)
	}

	// 确定保存路径
	ext := filepath.Ext(coverURL)
	if ext == "" || len(ext) > 5 {
		ext = ".jpg"
	}

	cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", media.ID)
	os.MkdirAll(cacheDir, 0755)
	localPath := filepath.Join(cacheDir, "poster"+ext)

	file, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("保存封面失败: %w", err)
	}

	return localPath, nil
}

// ==================== 辅助方法 ====================

// setAdultScraperHeaders 设置爬虫请求头（模拟浏览器）
func setAdultScraperHeaders(req *http.Request) {
	req.Header.Set("User-Agent", getRandomUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,ja;q=0.8,en;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
}

// IsAdultContent 判断媒体是否为成人内容（通过文件名/路径中的番号特征）
func IsAdultContent(media *model.Media) bool {
	// 优先使用增强版番号识别（支持 30+ 种格式）
	info := ParseCodeEnhanced(filepath.Base(media.FilePath))
	if info.IsAdult {
		return true
	}

	// 兜底：旧版番号识别
	code, _ := ParseCode(filepath.Base(media.FilePath))
	if code != "" {
		return true
	}

	// 通过路径关键词判断
	pathLower := strings.ToLower(media.FilePath)
	adultPathKeywords := []string{
		"adult", "av", "jav", "censored", "uncensored",
		"成人", "番号", "有码", "无码", "骑兵", "步兵",
	}
	for _, kw := range adultPathKeywords {
		if strings.Contains(pathLower, kw) {
			return true
		}
	}

	return false
}
