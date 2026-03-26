package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// MetadataService 元数据刮削服务
type MetadataService struct {
	mediaRepo     *repository.MediaRepo
	seriesRepo    *repository.SeriesRepo
	cfg           *config.Config
	logger        *zap.SugaredLogger
	client        *http.Client
	wsHub         *WSHub          // WebSocket事件广播
	douban        *DoubanService  // 豆瓣刮削服务（补充源）
	bangumi       *BangumiService // Bangumi刮削服务（补充源）
	ai            *AIService      // AI 元数据增强（第四层 Fallback）
	providerChain *ProviderChain  // 多数据源调度链（第三阶段）
}

func NewMetadataService(mediaRepo *repository.MediaRepo, seriesRepo *repository.SeriesRepo, cfg *config.Config, logger *zap.SugaredLogger) *MetadataService {
	s := &MetadataService{
		mediaRepo:  mediaRepo,
		seriesRepo: seriesRepo,
		cfg:        cfg,
		logger:     logger,
		client:     buildTMDbHTTPClient(cfg, logger),
		douban:     NewDoubanService(mediaRepo, cfg, logger),
		bangumi:    NewBangumiService(mediaRepo, seriesRepo, cfg, logger),
	}
	return s
}

// buildTMDbHTTPClient 构建专用于 TMDb 的 HTTP 客户端
// 解决国内网络环境下直连 api.themoviedb.org / image.tmdb.org 超时/被墙的核心问题
// 策略：
//  1. 自定义 DNS 解析器 + 超时控制
//  2. 连接级超时 5s（快速失败），总请求超时 12s
//  3. 启用 KeepAlive 复用连接，减少 TLS 握手开销
func buildTMDbHTTPClient(cfg *config.Config, logger *zap.SugaredLogger) *http.Client {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// 如果配置了代理，不做额外处理，直接连
			return dialer.DialContext(ctx, network, addr)
		},
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	logger.Infof("TMDb HTTP 客户端已初始化 (API代理: %s, 图片代理: %s)",
		defaultIfEmpty(cfg.Secrets.TMDbAPIProxy, "官方直连"),
		defaultIfEmpty(cfg.Secrets.TMDbImageProxy, "官方直连"))

	return &http.Client{
		Timeout:   12 * time.Second,
		Transport: transport,
	}
}

// getTMDbAPIBase 获取 TMDb API 基础地址（支持代理）
func (s *MetadataService) getTMDbAPIBase() string {
	if proxy := s.cfg.Secrets.TMDbAPIProxy; proxy != "" {
		return strings.TrimRight(proxy, "/")
	}
	return "https://api.themoviedb.org"
}

// getTMDbImageBase 获取 TMDb 图片基础地址（支持代理）
func (s *MetadataService) getTMDbImageBase() string {
	if proxy := s.cfg.Secrets.TMDbImageProxy; proxy != "" {
		return strings.TrimRight(proxy, "/")
	}
	return "https://image.tmdb.org"
}

// defaultIfEmpty 如果字符串为空则返回默认值
func defaultIfEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// tmdbGetWithRetry 带重试的 TMDb GET 请求
// 超时后自动重试 1 次（共 2 次尝试），仍然失败则返回错误
func (s *MetadataService) tmdbGetWithRetry(url string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			s.logger.Debugf("TMDb 请求重试 (%d/2): %s", attempt+1, url)
			time.Sleep(500 * time.Millisecond)
		}
		resp, err := s.client.Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		resp.Body.Close()
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		// 非临时性错误（如 401/404）不重试
		if resp.StatusCode == 401 || resp.StatusCode == 404 {
			return nil, lastErr
		}
	}
	return nil, fmt.Errorf("TMDb 请求失败（已重试）: %w", lastErr)
}

// SetWSHub 设置WebSocket Hub（延迟注入）
func (s *MetadataService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// SetAIService 设置 AI 服务（延迟注入）
func (s *MetadataService) SetAIService(ai *AIService) {
	s.ai = ai
}

// SetProviderChain 设置多数据源调度链（延迟注入）
func (s *MetadataService) SetProviderChain(chain *ProviderChain) {
	s.providerChain = chain
}

// ==================== TMDb API 数据结构 ====================

// TMDbSearchResult TMDb搜索结果
type TMDbSearchResult struct {
	Page         int         `json:"page"`
	TotalResults int         `json:"total_results"`
	Results      []TMDbMovie `json:"results"`
}

// TMDbMovie TMDb电影/剧集信息
type TMDbMovie struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	Name          string  `json:"name"` // 用于TV
	OriginalTitle string  `json:"original_title"`
	OriginalName  string  `json:"original_name"` // 用于TV
	Overview      string  `json:"overview"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	ReleaseDate   string  `json:"release_date"`
	FirstAirDate  string  `json:"first_air_date"` // 用于TV
	VoteAverage   float64 `json:"vote_average"`
	GenreIDs      []int   `json:"genre_ids"`
	Runtime       int     `json:"runtime"`
}

// TMDbMovieDetail TMDb电影详情
type TMDbMovieDetail struct {
	ID            int             `json:"id"`
	Title         string          `json:"title"`
	OriginalTitle string          `json:"original_title"`
	Overview      string          `json:"overview"`
	PosterPath    string          `json:"poster_path"`
	BackdropPath  string          `json:"backdrop_path"`
	ReleaseDate   string          `json:"release_date"`
	VoteAverage   float64         `json:"vote_average"`
	Runtime       int             `json:"runtime"`
	Genres        []TMDbGenre     `json:"genres"`
	Videos        *TMDbVideosWrap `json:"videos,omitempty"` // append_to_response=videos
}

// TMDbVideosWrap TMDb视频包装
type TMDbVideosWrap struct {
	Results []TMDbVideo `json:"results"`
}

// TMDbVideo TMDb视频（预告片/花絮）
type TMDbVideo struct {
	Key      string `json:"key"`      // YouTube video ID
	Site     string `json:"site"`     // "YouTube"
	Type     string `json:"type"`     // "Trailer", "Teaser", "Featurette"
	Official bool   `json:"official"` // 是否官方
}

// TMDbGenre TMDb类型
type TMDbGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ==================== 核心方法 ====================

// ScrapeMedia 为单个媒体项刮削元数据
func (s *MetadataService) ScrapeMedia(mediaID string) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}

	s.logger.Infof("开始刮削元数据: %s", media.Title)

	// 从标题中提取搜索关键词和年份
	searchTitle, year := s.parseTitle(media.Title)

	// 如果已配置 ProviderChain，使用新的多数据源调度链
	if s.providerChain != nil {
		err := s.providerChain.ScrapeMedia(media, searchTitle, year)
		if err != nil {
			s.logger.Debugf("多数据源调度链刮削失败: %s - %v", media.Title, err)
			return err
		}
		// 保存更新
		if saveErr := s.mediaRepo.Update(media); saveErr != nil {
			s.logger.Errorf("保存元数据失败: %s - %v", media.Title, saveErr)
			return saveErr
		}
		s.logger.Infof("元数据刮削完成 (多数据源): %s", media.Title)
		time.Sleep(200 * time.Millisecond)
		return nil
	}

	// === 兼容旧版逻辑（未配置 ProviderChain 时的 Fallback） ===

	// 第一步：尝试TMDb刮削（超时 5s 快速失败，不拖慢整体流程）
	var tmdbErr error
	if s.cfg.Secrets.TMDbAPIKey != "" {
		if media.MediaType == "movie" {
			tmdbErr = s.scrapeMovie(media, searchTitle, year)
		} else {
			tmdbErr = s.scrapeTV(media, searchTitle, year)
		}
		if tmdbErr != nil {
			s.logger.Debugf("TMDb 刮削失败: %s - %v", media.Title, tmdbErr)
		}
	} else {
		tmdbErr = fmt.Errorf("TMDb API Key未配置")
	}

	// 第二步：如果TMDb失败或信息不完整，自动 Fallback 到豆瓣补充
	needFallback := tmdbErr != nil || media.Overview == "" || media.PosterPath == "" || media.Rating == 0

	if needFallback {
		s.logger.Debugf("TMDb刮削%s，尝试补充源: %s",
			map[bool]string{true: "失败", false: "信息不完整"}[tmdbErr != nil],
			media.Title)

		// 重新获取最新的media数据（TMDb可能已部分更新）
		if tmdbErr == nil {
			if updated, err := s.mediaRepo.FindByID(mediaID); err == nil {
				media = updated
			}
		}

		// 尝试豆瓣补充
		if doubanErr := s.douban.ScrapeMedia(media, searchTitle, year); doubanErr != nil {
			s.logger.Debugf("豆瓣刮削也失败: %s - %v", media.Title, doubanErr)
		} else {
			s.logger.Infof("豆瓣补充刮削成功: %s", media.Title)
		}

		// 第三步：如果仍然不完整，继续 Fallback 到 Bangumi 补充
		// 重新获取数据（豆瓣可能已部分更新）
		if updated, err := s.mediaRepo.FindByID(mediaID); err == nil {
			media = updated
		}
		needBangumi := media.Overview == "" || media.PosterPath == "" || media.Rating == 0
		if needBangumi && s.bangumi.IsEnabled() {
			s.logger.Debugf("信息仍不完整，尝试 Bangumi 补充: %s", media.Title)
			if bangumiErr := s.bangumi.ScrapeMedia(media, searchTitle, year); bangumiErr != nil {
				s.logger.Debugf("Bangumi 刮削也失败: %s - %v", media.Title, bangumiErr)
			} else {
				s.logger.Infof("Bangumi 补充刮削成功: %s", media.Title)
			}
		}

		// 如果所有数据源都失败，返回原始TMDb错误
		if tmdbErr != nil {
			if updated, err := s.mediaRepo.FindByID(mediaID); err == nil {
				if updated.Overview == "" && updated.PosterPath == "" {
					// 第四步：AI 元数据增强（最后的 Fallback）
					if s.ai != nil && s.ai.IsMetadataEnhanceEnabled() {
						s.logger.Debugf("尝试 AI 元数据增强: %s", media.Title)
						if aiErr := s.ai.EnrichMetadata(updated, searchTitle); aiErr != nil {
							s.logger.Debugf("AI 元数据增强也失败: %s - %v", media.Title, aiErr)
						} else {
							s.logger.Infof("AI 元数据增强成功: %s", media.Title)
							s.mediaRepo.Update(updated)
							return nil
						}
					}
					return tmdbErr
				}
			}
		}
	}

	// 等待一下避免限流
	time.Sleep(200 * time.Millisecond)

	return nil
}

// ScrapeLibrary 为整个媒体库刮削元数据
func (s *MetadataService) ScrapeLibrary(libraryID string, mediaList []model.Media) (int, int) {
	if s.cfg.Secrets.TMDbAPIKey == "" && s.providerChain == nil {
		s.logger.Warn("TMDb API Key未配置且无可用数据源，跳过元数据刮削")
		return 0, 0
	}

	// 计算需要刮削的总数
	var needScrape []model.Media
	for _, media := range mediaList {
		if media.Overview == "" || media.PosterPath == "" {
			needScrape = append(needScrape, media)
		}
	}

	if len(needScrape) == 0 {
		return 0, 0
	}

	total := len(needScrape)
	success := 0
	failed := 0

	// 发送刮削开始事件
	s.broadcastScrapeEvent(EventScrapeStarted, &ScrapeProgressData{
		LibraryID: libraryID,
		Total:     total,
		Message:   fmt.Sprintf("开始元数据刮削，共 %d 个媒体待处理", total),
	})

	for i, media := range needScrape {
		if err := s.ScrapeMedia(media.ID); err != nil {
			s.logger.Debugf("刮削失败 [%d/%d]: %s - %v", i+1, total, media.Title, err)
			failed++
		} else {
			success++
		}

		// 发送刮削进度事件
		s.broadcastScrapeEvent(EventScrapeProgress, &ScrapeProgressData{
			LibraryID:  libraryID,
			Current:    i + 1,
			Total:      total,
			Success:    success,
			Failed:     failed,
			MediaTitle: media.Title,
			Message:    fmt.Sprintf("刮削进度: [%d/%d] %s", i+1, total, media.Title),
		})

		// TMDb限速：每秒最多40次请求，我们保守一点
		time.Sleep(300 * time.Millisecond)
	}

	// 发送刮削完成事件
	s.broadcastScrapeEvent(EventScrapeCompleted, &ScrapeProgressData{
		LibraryID: libraryID,
		Current:   total,
		Total:     total,
		Success:   success,
		Failed:    failed,
		Message:   fmt.Sprintf("元数据刮削完成: 成功 %d, 失败 %d", success, failed),
	})

	s.logger.Infof("元数据刮削完成: 成功 %d, 失败 %d", success, failed)
	return success, failed
}

// broadcastScrapeEvent 广播刮削事件
func (s *MetadataService) broadcastScrapeEvent(eventType string, data *ScrapeProgressData) {
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(eventType, data)
	}
}

// scrapeMovie 刮削电影元数据
func (s *MetadataService) scrapeMovie(media *model.Media, searchTitle string, year int) error {
	// 搜索电影
	results, err := s.searchTMDb("movie", searchTitle, year)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		// 无年份重试
		if year > 0 {
			results, err = s.searchTMDb("movie", searchTitle, 0)
			if err != nil {
				return err
			}
		}
		if len(results) == 0 {
			return fmt.Errorf("未找到匹配结果: %s", searchTitle)
		}
	}

	// 取第一个结果
	best := results[0]

	// 获取详情
	detail, err := s.getMovieDetail(best.ID)
	if err != nil {
		// 用搜索结果的信息也可以
		s.applySearchResult(media, &best)
	} else {
		s.applyMovieDetail(media, detail)
	}

	// 下载海报
	if media.PosterPath != "" || best.PosterPath != "" {
		posterURL := best.PosterPath
		if media.PosterPath == "" {
			posterURL = best.PosterPath
		}
		if posterURL != "" {
			localPath, err := s.downloadPoster(media, posterURL)
			if err == nil {
				media.PosterPath = localPath
			}
		}
	}

	// 下载背景图
	if best.BackdropPath != "" {
		localPath, err := s.downloadBackdrop(media, best.BackdropPath)
		if err == nil {
			media.BackdropPath = localPath
		}
	}

	return s.mediaRepo.Update(media)
}

// scrapeTV 刮削剧集元数据
func (s *MetadataService) scrapeTV(media *model.Media, searchTitle string, year int) error {
	results, err := s.searchTMDb("tv", searchTitle, year)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		if year > 0 {
			results, err = s.searchTMDb("tv", searchTitle, 0)
			if err != nil {
				return err
			}
		}
		if len(results) == 0 {
			return fmt.Errorf("未找到匹配结果: %s", searchTitle)
		}
	}

	best := results[0]

	// 应用搜索结果
	title := best.Name
	if title == "" {
		title = best.Title
	}
	origTitle := best.OriginalName
	if origTitle == "" {
		origTitle = best.OriginalTitle
	}

	media.OrigTitle = origTitle
	if best.Overview != "" {
		media.Overview = best.Overview
	}
	media.Rating = best.VoteAverage

	dateStr := best.FirstAirDate
	if dateStr == "" {
		dateStr = best.ReleaseDate
	}
	if len(dateStr) >= 4 {
		media.Year, _ = strconv.Atoi(dateStr[:4])
	}

	// 下载海报
	if best.PosterPath != "" {
		localPath, err := s.downloadPoster(media, best.PosterPath)
		if err == nil {
			media.PosterPath = localPath
		}
	}

	if best.BackdropPath != "" {
		localPath, err := s.downloadBackdrop(media, best.BackdropPath)
		if err == nil {
			media.BackdropPath = localPath
		}
	}

	return s.mediaRepo.Update(media)
}

// ==================== TMDb API 调用 ====================

// searchTMDb 搜索TMDb
func (s *MetadataService) searchTMDb(mediaType, query string, year int) ([]TMDbMovie, error) {
	params := url.Values{}
	params.Set("api_key", s.cfg.Secrets.TMDbAPIKey)
	params.Set("query", query)
	params.Set("language", "zh-CN")
	params.Set("include_adult", "false")

	if year > 0 {
		if mediaType == "movie" {
			params.Set("year", strconv.Itoa(year))
		} else {
			params.Set("first_air_date_year", strconv.Itoa(year))
		}
	}

	apiURL := fmt.Sprintf("%s/3/search/%s?%s", s.getTMDbAPIBase(), mediaType, params.Encode())
	resp, err := s.tmdbGetWithRetry(apiURL)
	if err != nil {
		return nil, fmt.Errorf("TMDb搜索请求失败: %w", err)
	}
	defer resp.Body.Close()

	var result TMDbSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析TMDb响应失败: %w", err)
	}

	return result.Results, nil
}

// getMovieDetail 获取电影详情
func (s *MetadataService) getMovieDetail(tmdbID int) (*TMDbMovieDetail, error) {
	apiURL := fmt.Sprintf("%s/3/movie/%d?api_key=%s&language=zh-CN&append_to_response=videos",
		s.getTMDbAPIBase(), tmdbID, s.cfg.Secrets.TMDbAPIKey)

	resp, err := s.tmdbGetWithRetry(apiURL)
	if err != nil {
		return nil, fmt.Errorf("TMDb详情请求失败: %w", err)
	}
	defer resp.Body.Close()

	var detail TMDbMovieDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, err
	}

	return &detail, nil
}

// ==================== 数据应用 ====================

// applySearchResult 将搜索结果应用到媒体
func (s *MetadataService) applySearchResult(media *model.Media, result *TMDbMovie) {
	title := result.Title
	if title == "" {
		title = result.Name
	}
	origTitle := result.OriginalTitle
	if origTitle == "" {
		origTitle = result.OriginalName
	}

	media.OrigTitle = origTitle
	if result.Overview != "" {
		media.Overview = result.Overview
	}
	media.Rating = result.VoteAverage

	dateStr := result.ReleaseDate
	if dateStr == "" {
		dateStr = result.FirstAirDate
	}
	if len(dateStr) >= 4 {
		media.Year, _ = strconv.Atoi(dateStr[:4])
	}

	// 类型映射
	media.Genres = s.mapGenreIDs(result.GenreIDs)
}

// applyMovieDetail 将电影详情应用到媒体
func (s *MetadataService) applyMovieDetail(media *model.Media, detail *TMDbMovieDetail) {
	media.OrigTitle = detail.OriginalTitle
	if detail.Overview != "" {
		media.Overview = detail.Overview
	}
	media.Rating = detail.VoteAverage
	if detail.Runtime > 0 {
		media.Runtime = detail.Runtime
	}

	if len(detail.ReleaseDate) >= 4 {
		media.Year, _ = strconv.Atoi(detail.ReleaseDate[:4])
	}

	// 类型
	var genres []string
	for _, g := range detail.Genres {
		genres = append(genres, g.Name)
	}
	if len(genres) > 0 {
		media.Genres = strings.Join(genres, ",")
	}

	// 提取预告片 YouTube 链接
	if detail.Videos != nil {
		for _, v := range detail.Videos.Results {
			if v.Site == "YouTube" && (v.Type == "Trailer" || v.Type == "Teaser") {
				media.TrailerURL = "https://www.youtube.com/watch?v=" + v.Key
				break
			}
		}
	}
}

// ==================== 图片下载 ====================

// downloadPoster 下载海报到本地
func (s *MetadataService) downloadPoster(media *model.Media, tmdbPath string) (string, error) {
	return s.downloadImage(media, tmdbPath, "poster", "w500")
}

// downloadBackdrop 下载背景图到本地
func (s *MetadataService) downloadBackdrop(media *model.Media, tmdbPath string) (string, error) {
	return s.downloadImage(media, tmdbPath, "backdrop", "w1280")
}

// downloadImage 下载TMDb图片到本地
func (s *MetadataService) downloadImage(media *model.Media, tmdbPath, imageType, size string) (string, error) {
	if tmdbPath == "" {
		return "", fmt.Errorf("图片路径为空")
	}

	imageURL := fmt.Sprintf("%s/t/p/%s%s", s.getTMDbImageBase(), size, tmdbPath)

	resp, err := s.tmdbGetWithRetry(imageURL)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}
	defer resp.Body.Close()

	// 保存到媒体文件同目录
	mediaDir := filepath.Dir(media.FilePath)
	baseName := strings.TrimSuffix(filepath.Base(media.FilePath), filepath.Ext(media.FilePath))
	ext := filepath.Ext(tmdbPath)
	if ext == "" {
		ext = ".jpg"
	}

	localPath := filepath.Join(mediaDir, fmt.Sprintf("%s-%s%s", baseName, imageType, ext))

	file, err := os.Create(localPath)
	if err != nil {
		// 如果媒体目录不可写，保存到缓存目录
		cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", media.ID)
		os.MkdirAll(cacheDir, 0755)
		localPath = filepath.Join(cacheDir, fmt.Sprintf("%s%s", imageType, ext))
		file, err = os.Create(localPath)
		if err != nil {
			return "", fmt.Errorf("创建图片文件失败: %w", err)
		}
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("保存图片失败: %w", err)
	}

	s.logger.Debugf("已下载%s: %s", imageType, localPath)
	return localPath, nil
}

// ==================== 辅助方法 ====================

// parseTitle 从文件标题中提取搜索关键词和年份
func (s *MetadataService) parseTitle(title string) (string, int) {
	// 尝试提取年份，如 "The Matrix 1999" 或 "黑客帝国 (1999)"
	yearRegex := regexp.MustCompile(`[\s\.(]\s*((?:19|20)\d{2})\s*[\s\).]?`)
	matches := yearRegex.FindStringSubmatch(title)

	var year int
	cleanTitle := title

	if len(matches) >= 2 {
		year, _ = strconv.Atoi(matches[1])
		// 移除年份部分
		cleanTitle = yearRegex.ReplaceAllString(title, " ")
	}

	// 清理常见标记
	cleanPatterns := []string{
		`(?i)\b(BluRay|BDRip|HDRip|WEB-?DL|WEBRip|DVDRip|HDTV|HDCam)\b`,
		`(?i)\b(x264|x265|h\.?264|h\.?265|HEVC|AVC|AAC|DTS|AC3|FLAC)\b`,
		`(?i)\b(1080p|720p|480p|2160p|4K|UHD)\b`,
		`(?i)\b(REMUX|PROPER|REPACK|EXTENDED|UNRATED|DIRECTORS\.?CUT)\b`,
		`(?i)\[.*?\]`,
		`(?i)\(.*?\)`,
	}

	for _, pattern := range cleanPatterns {
		re := regexp.MustCompile(pattern)
		cleanTitle = re.ReplaceAllString(cleanTitle, " ")
	}

	// 清理多余空格
	cleanTitle = regexp.MustCompile(`\s+`).ReplaceAllString(cleanTitle, " ")
	cleanTitle = strings.TrimSpace(cleanTitle)

	return cleanTitle, year
}

// SearchTMDb 公开的TMDb搜索方法（用于手动元数据匹配）
func (s *MetadataService) SearchTMDb(mediaType, query string, year int) ([]TMDbMovie, error) {
	if s.cfg.Secrets.TMDbAPIKey == "" {
		return nil, fmt.Errorf("TMDb API Key未配置")
	}
	return s.searchTMDb(mediaType, query, year)
}

// TMDbImage TMDb 图片信息
type TMDbImage struct {
	FilePath    string  `json:"file_path"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	AspectRatio float64 `json:"aspect_ratio"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
	Language    string  `json:"iso_639_1"`
}

// TMDbImagesResult TMDb 图片搜索结果
type TMDbImagesResult struct {
	Posters   []TMDbImage `json:"posters"`
	Backdrops []TMDbImage `json:"backdrops"`
}

// SearchTMDbImages 获取TMDb条目的所有可用图片（海报+背景图）
func (s *MetadataService) SearchTMDbImages(mediaType string, tmdbID int) (*TMDbImagesResult, error) {
	if s.cfg.Secrets.TMDbAPIKey == "" {
		return nil, fmt.Errorf("TMDb API Key 未配置")
	}

	apiURL := fmt.Sprintf("%s/3/%s/%d/images?api_key=%s&include_image_language=zh,en,null",
		s.getTMDbAPIBase(), mediaType, tmdbID, s.cfg.Secrets.TMDbAPIKey)

	resp, err := s.tmdbGetWithRetry(apiURL)
	if err != nil {
		return nil, fmt.Errorf("TMDb 图片查询失败: %w", err)
	}
	defer resp.Body.Close()

	var result TMDbImagesResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 TMDb 图片响应失败: %w", err)
	}

	return &result, nil
}

// DownloadTMDbImageForMedia 从TMDb下载指定图片并保存到本地，更新Media的图片路径
func (s *MetadataService) DownloadTMDbImageForMedia(mediaID string, tmdbPath string, imageType string) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}

	size := "w500"
	if imageType == "backdrop" {
		size = "w1280"
	}

	localPath, err := s.downloadImage(media, tmdbPath, imageType, size)
	if err != nil {
		return "", err
	}

	if imageType == "poster" {
		media.PosterPath = localPath
	} else {
		media.BackdropPath = localPath
	}

	if err := s.mediaRepo.Update(media); err != nil {
		return "", fmt.Errorf("更新媒体数据失败: %w", err)
	}

	return localPath, nil
}

// DownloadTMDbImageForSeries 从TMDb下载指定图片并保存到本地，更新Series的图片路径
func (s *MetadataService) DownloadTMDbImageForSeries(seriesID string, tmdbPath string, imageType string) (string, error) {
	series, err := s.seriesRepo.FindByID(seriesID)
	if err != nil {
		return "", fmt.Errorf("剧集合集不存在")
	}

	size := "w500"
	if imageType == "backdrop" {
		size = "w1280"
	}

	imageURL := fmt.Sprintf("%s/t/p/%s%s", s.getTMDbImageBase(), size, tmdbPath)
	ext := filepath.Ext(tmdbPath)
	if ext == "" {
		ext = ".jpg"
	}

	cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", "series", series.ID)
	os.MkdirAll(cacheDir, 0755)
	localPath := filepath.Join(cacheDir, imageType+ext)

	if err := s.downloadToFile(imageURL, localPath); err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}

	if imageType == "poster" {
		series.PosterPath = localPath
	} else {
		series.BackdropPath = localPath
	}

	if err := s.seriesRepo.Update(series); err != nil {
		return "", fmt.Errorf("更新剧集数据失败: %w", err)
	}

	return localPath, nil
}

// SaveUploadedImageForMedia 保存上传的图片文件到本地，更新Media的图片路径
func (s *MetadataService) SaveUploadedImageForMedia(mediaID string, imageData []byte, ext string, imageType string) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}

	// 先尝试保存到媒体文件同目录
	mediaDir := filepath.Dir(media.FilePath)
	baseName := strings.TrimSuffix(filepath.Base(media.FilePath), filepath.Ext(media.FilePath))
	localPath := filepath.Join(mediaDir, fmt.Sprintf("%s-%s%s", baseName, imageType, ext))

	if err := os.WriteFile(localPath, imageData, 0644); err != nil {
		// 回退到缓存目录
		cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", media.ID)
		os.MkdirAll(cacheDir, 0755)
		localPath = filepath.Join(cacheDir, imageType+ext)
		if err := os.WriteFile(localPath, imageData, 0644); err != nil {
			return "", fmt.Errorf("保存图片文件失败: %w", err)
		}
	}

	if imageType == "poster" {
		media.PosterPath = localPath
	} else {
		media.BackdropPath = localPath
	}

	if err := s.mediaRepo.Update(media); err != nil {
		return "", fmt.Errorf("更新媒体数据失败: %w", err)
	}

	return localPath, nil
}

// SaveUploadedImageForSeries 保存上传的图片文件到本地，更新Series的图片路径
func (s *MetadataService) SaveUploadedImageForSeries(seriesID string, imageData []byte, ext string, imageType string) (string, error) {
	series, err := s.seriesRepo.FindByID(seriesID)
	if err != nil {
		return "", fmt.Errorf("剧集合集不存在")
	}

	cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", "series", series.ID)
	os.MkdirAll(cacheDir, 0755)
	localPath := filepath.Join(cacheDir, imageType+ext)

	if err := os.WriteFile(localPath, imageData, 0644); err != nil {
		return "", fmt.Errorf("保存图片文件失败: %w", err)
	}

	if imageType == "poster" {
		series.PosterPath = localPath
	} else {
		series.BackdropPath = localPath
	}

	if err := s.seriesRepo.Update(series); err != nil {
		return "", fmt.Errorf("更新剧集数据失败: %w", err)
	}

	return localPath, nil
}

// DownloadURLImageForMedia 从URL下载图片并保存到本地，更新Media的图片路径
func (s *MetadataService) DownloadURLImageForMedia(mediaID string, imageURL string, imageType string) (string, error) {
	// 先验证媒体是否存在
	if _, err := s.mediaRepo.FindByID(mediaID); err != nil {
		return "", ErrMediaNotFound
	}

	resp, err := s.client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载图片失败，HTTP状态码: %d", resp.StatusCode)
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取图片数据失败: %w", err)
	}

	// 检查大小限制（10MB）
	if len(imageData) > 10*1024*1024 {
		return "", fmt.Errorf("图片文件过大，最大支持10MB")
	}

	// 根据 Content-Type 确定扩展名
	ext := ".jpg"
	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "png"):
		ext = ".png"
	case strings.Contains(ct, "webp"):
		ext = ".webp"
	case strings.Contains(ct, "gif"):
		ext = ".gif"
	}

	return s.SaveUploadedImageForMedia(mediaID, imageData, ext, imageType)
}

// DownloadURLImageForSeries 从URL下载图片并保存到本地，更新Series的图片路径
func (s *MetadataService) DownloadURLImageForSeries(seriesID string, imageURL string, imageType string) (string, error) {
	resp, err := s.client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载图片失败，HTTP状态码: %d", resp.StatusCode)
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取图片数据失败: %w", err)
	}

	if len(imageData) > 10*1024*1024 {
		return "", fmt.Errorf("图片文件过大，最大支持10MB")
	}

	ext := ".jpg"
	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "png"):
		ext = ".png"
	case strings.Contains(ct, "webp"):
		ext = ".webp"
	case strings.Contains(ct, "gif"):
		ext = ".gif"
	}

	return s.SaveUploadedImageForSeries(seriesID, imageData, ext, imageType)
}

// ScrapeSeries 为剧集合集刮削元数据（以合集名称搜索，并将元数据同步到合集和各集）
func (s *MetadataService) ScrapeSeries(seriesID string) error {
	if s.seriesRepo == nil {
		return fmt.Errorf("剧集仓储未初始化")
	}

	series, err := s.seriesRepo.FindByID(seriesID)
	if err != nil {
		return fmt.Errorf("剧集合集不存在: %s", seriesID)
	}

	s.logger.Infof("开始刮削剧集合集元数据: %s", series.Title)

	// 从标题中提取搜索关键词和年份
	searchTitle, year := s.parseTitle(series.Title)

	// 如果已配置 ProviderChain，使用新的多数据源调度链
	if s.providerChain != nil {
		err := s.providerChain.ScrapeSeries(series, searchTitle, year)
		if err != nil {
			s.logger.Debugf("多数据源调度链剧集刮削失败: %s - %v", series.Title, err)
			// 不直接返回错误，继续尝试保存已有数据
		}

		// 保存合集元数据
		if saveErr := s.seriesRepo.Update(series); saveErr != nil {
			return fmt.Errorf("更新剧集合集失败: %w", saveErr)
		}

		// 将合集元数据同步到各集
		s.syncSeriesToEpisodes(seriesID, series)

		s.logger.Infof("剧集合集元数据刮削完成 (多数据源): %s", series.Title)
		return err
	}

	// === 兼容旧版逻辑（未配置 ProviderChain 时的 Fallback） ===

	// 第一步：TMDb 搜索 TV 类型
	var tmdbResult *TMDbMovie
	if s.cfg.Secrets.TMDbAPIKey != "" {
		results, err := s.searchTMDb("tv", searchTitle, year)
		if err == nil && len(results) > 0 {
			tmdbResult = &results[0]
		} else if year > 0 {
			results, err = s.searchTMDb("tv", searchTitle, 0)
			if err == nil && len(results) > 0 {
				tmdbResult = &results[0]
			}
		}
	}

	// 应用TMDb结果到合集
	if tmdbResult != nil {
		title := tmdbResult.Name
		if title == "" {
			title = tmdbResult.Title
		}
		origTitle := tmdbResult.OriginalName
		if origTitle == "" {
			origTitle = tmdbResult.OriginalTitle
		}

		series.OrigTitle = origTitle
		if tmdbResult.Overview != "" {
			series.Overview = tmdbResult.Overview
		}
		series.Rating = tmdbResult.VoteAverage

		dateStr := tmdbResult.FirstAirDate
		if dateStr == "" {
			dateStr = tmdbResult.ReleaseDate
		}
		if len(dateStr) >= 4 {
			series.Year, _ = strconv.Atoi(dateStr[:4])
		}

		series.Genres = s.mapGenreIDs(tmdbResult.GenreIDs)

		// 下载海报
		if tmdbResult.PosterPath != "" {
			imageURL := fmt.Sprintf("%s/t/p/w500%s", s.getTMDbImageBase(), tmdbResult.PosterPath)
			ext := filepath.Ext(tmdbResult.PosterPath)
			if ext == "" {
				ext = ".jpg"
			}
			cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", "series", series.ID)
			os.MkdirAll(cacheDir, 0755)
			localPath := filepath.Join(cacheDir, "poster"+ext)
			if err := s.downloadToFile(imageURL, localPath); err == nil {
				series.PosterPath = localPath
			}
		}

		// 下载背景图
		if tmdbResult.BackdropPath != "" {
			imageURL := fmt.Sprintf("%s/t/p/w1280%s", s.getTMDbImageBase(), tmdbResult.BackdropPath)
			ext := filepath.Ext(tmdbResult.BackdropPath)
			if ext == "" {
				ext = ".jpg"
			}
			cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", "series", series.ID)
			os.MkdirAll(cacheDir, 0755)
			localPath := filepath.Join(cacheDir, "backdrop"+ext)
			if err := s.downloadToFile(imageURL, localPath); err == nil {
				series.BackdropPath = localPath
			}
		}
	}

	// 第二步：豆瓣补充（如果TMDb信息不完整）
	// 注意：不能调用 douban.ScrapeMedia()，因为它最后会调用 mediaRepo.Update()
	// 把 tempMedia 写入 Media 表，导致 Series 数据被重复创建为独立的 movie 记录
	if series.Overview == "" || series.PosterPath == "" || series.Rating == 0 {
		s.logger.Debugf("尝试豆瓣补充剧集元数据: %s", series.Title)
		// 直接调用豆瓣搜索，只在内存中应用结果，不写入 Media 表
		tempMedia := &model.Media{
			Title:    series.Title,
			FilePath: series.FolderPath + "/placeholder",
		}
		s.douban.ApplyDoubanData(tempMedia, searchTitle, year)
		if series.Overview == "" && tempMedia.Overview != "" {
			series.Overview = tempMedia.Overview
		}
		if series.Rating == 0 && tempMedia.Rating > 0 {
			series.Rating = tempMedia.Rating
		}
		if series.Year == 0 && tempMedia.Year > 0 {
			series.Year = tempMedia.Year
		}
		if series.Genres == "" && tempMedia.Genres != "" {
			series.Genres = tempMedia.Genres
		}
		if series.PosterPath == "" && tempMedia.PosterPath != "" {
			series.PosterPath = tempMedia.PosterPath
		}
	}

	// 第三步：Bangumi 补充（如果仍不完整）
	if (series.Overview == "" || series.PosterPath == "" || series.Rating == 0) && s.bangumi.IsEnabled() {
		s.logger.Debugf("尝试 Bangumi 补充剧集元数据: %s", series.Title)
		tempMedia := &model.Media{
			Title:    series.Title,
			FilePath: series.FolderPath + "/placeholder",
		}
		s.bangumi.ApplyBangumiData(tempMedia, searchTitle, year)
		if series.Overview == "" && tempMedia.Overview != "" {
			series.Overview = tempMedia.Overview
		}
		if series.Rating == 0 && tempMedia.Rating > 0 {
			series.Rating = tempMedia.Rating
		}
		if series.Year == 0 && tempMedia.Year > 0 {
			series.Year = tempMedia.Year
		}
		if series.Genres == "" && tempMedia.Genres != "" {
			series.Genres = tempMedia.Genres
		}
		if series.OrigTitle == "" && tempMedia.OrigTitle != "" {
			series.OrigTitle = tempMedia.OrigTitle
		}
		if series.Country == "" && tempMedia.Country != "" {
			series.Country = tempMedia.Country
		}
		if series.Language == "" && tempMedia.Language != "" {
			series.Language = tempMedia.Language
		}
		if series.Studio == "" && tempMedia.Studio != "" {
			series.Studio = tempMedia.Studio
		}
		if series.PosterPath == "" && tempMedia.PosterPath != "" {
			series.PosterPath = tempMedia.PosterPath
		}
	}

	// 保存合集元数据
	if err := s.seriesRepo.Update(series); err != nil {
		return fmt.Errorf("更新剧集合集失败: %w", err)
	}

	// 第三步：将合集元数据同步到各集（海报、评分、类型等）
	s.syncSeriesToEpisodes(seriesID, series)

	s.logger.Infof("剧集合集元数据刮削完成: %s", series.Title)
	return nil
}

// syncSeriesToEpisodes 将合集元数据同步到各集（海报、评分、类型等）
func (s *MetadataService) syncSeriesToEpisodes(seriesID string, series *model.Series) {
	episodes, _ := s.mediaRepo.ListBySeriesID(seriesID)
	for _, ep := range episodes {
		updated := false
		if ep.Overview == "" && series.Overview != "" {
			ep.Overview = series.Overview
			updated = true
		}
		if ep.PosterPath == "" && series.PosterPath != "" {
			ep.PosterPath = series.PosterPath
			updated = true
		}
		if ep.Rating == 0 && series.Rating > 0 {
			ep.Rating = series.Rating
			updated = true
		}
		if ep.Genres == "" && series.Genres != "" {
			ep.Genres = series.Genres
			updated = true
		}
		if ep.Year == 0 && series.Year > 0 {
			ep.Year = series.Year
			updated = true
		}
		if ep.OrigTitle == "" && series.OrigTitle != "" {
			ep.OrigTitle = series.OrigTitle
			updated = true
		}
		if updated {
			s.mediaRepo.Update(&ep)
		}
	}
	s.logger.Debugf("已同步合集元数据到 %d 集", len(episodes))
}

// ScrapeAllSeries 为媒体库下所有剧集合集刮削元数据
func (s *MetadataService) ScrapeAllSeries(libraryID string) (int, int) {
	if s.seriesRepo == nil {
		return 0, 0
	}

	seriesList, err := s.seriesRepo.ListByLibraryID(libraryID)
	if err != nil {
		s.logger.Errorf("获取剧集列表失败: %v", err)
		return 0, 0
	}

	// 过滤出需要刮削的剧集
	var needScrape []model.Series
	for _, s := range seriesList {
		if s.Overview == "" || s.PosterPath == "" {
			needScrape = append(needScrape, s)
		}
	}

	if len(needScrape) == 0 {
		return 0, 0
	}

	success, failed := 0, 0
	for _, series := range needScrape {
		if err := s.ScrapeSeries(series.ID); err != nil {
			s.logger.Debugf("剧集刮削失败: %s - %v", series.Title, err)
			failed++
		} else {
			success++
		}
		// 限速
		time.Sleep(500 * time.Millisecond)
	}

	return success, failed
}

// downloadToFile 下载文件到指定路径（带重试）
func (s *MetadataService) downloadToFile(url, filePath string) error {
	resp, err := s.tmdbGetWithRetry(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

// UnmatchMedia 解除媒体的元数据匹配，清除刮削获取的所有信息
func (s *MetadataService) UnmatchMedia(mediaID string) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}

	// 清除所有从 TMDb/豆瓣获取的元数据（保留文件扫描的原始信息）
	media.TMDbID = 0
	media.DoubanID = ""
	media.Overview = ""
	media.PosterPath = ""
	media.BackdropPath = ""
	media.Rating = 0
	media.Runtime = 0
	media.Genres = ""
	media.OrigTitle = ""
	media.Country = ""
	media.Language = ""
	media.Tagline = ""
	media.Studio = ""
	media.TrailerURL = ""

	s.logger.Infof("已解除媒体元数据匹配: %s (%s)", media.Title, mediaID)
	return s.mediaRepo.Update(media)
}

// UnmatchSeries 解除剧集合集的元数据匹配，清除刮削获取的所有信息
func (s *MetadataService) UnmatchSeries(seriesID string) error {
	series, err := s.seriesRepo.FindByID(seriesID)
	if err != nil {
		return fmt.Errorf("剧集合集不存在")
	}

	series.TMDbID = 0
	series.DoubanID = ""
	series.Overview = ""
	series.PosterPath = ""
	series.BackdropPath = ""
	series.Rating = 0
	series.Genres = ""
	series.OrigTitle = ""
	series.Country = ""
	series.Language = ""
	series.Studio = ""

	s.logger.Infof("已解除剧集合集元数据匹配: %s (%s)", series.Title, seriesID)
	return s.seriesRepo.Update(series)
}

// MatchSeriesWithTMDb 手动关联TMDb结果到指定剧集合集
func (s *MetadataService) MatchSeriesWithTMDb(seriesID string, tmdbID int) error {
	series, err := s.seriesRepo.FindByID(seriesID)
	if err != nil {
		return fmt.Errorf("剧集合集不存在")
	}

	// 从 TMDb 获取 TV 详情
	apiKey := s.cfg.Secrets.TMDbAPIKey
	if apiKey == "" {
		return fmt.Errorf("TMDb API Key 未配置")
	}

	reqURL := fmt.Sprintf("%s/3/tv/%d?api_key=%s&language=zh-CN",
		s.getTMDbAPIBase(), tmdbID, apiKey)
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return fmt.Errorf("TMDb 请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var detail struct {
		ID           int     `json:"id"`
		Name         string  `json:"name"`
		OriginalName string  `json:"original_name"`
		Overview     string  `json:"overview"`
		VoteAverage  float64 `json:"vote_average"`
		FirstAirDate string  `json:"first_air_date"`
		PosterPath   string  `json:"poster_path"`
		BackdropPath string  `json:"backdrop_path"`
		Genres       []struct {
			Name string `json:"name"`
		} `json:"genres"`
		OriginCountry       []string `json:"origin_country"`
		OriginalLanguage    string   `json:"original_language"`
		ProductionCompanies []struct {
			Name string `json:"name"`
		} `json:"production_companies"`
	}
	if err := json.Unmarshal(body, &detail); err != nil {
		return fmt.Errorf("解析 TMDb 响应失败")
	}

	// 应用数据
	series.TMDbID = detail.ID
	if detail.Name != "" {
		series.OrigTitle = detail.OriginalName
	}
	if detail.Overview != "" {
		series.Overview = detail.Overview
	}
	series.Rating = detail.VoteAverage
	if len(detail.FirstAirDate) >= 4 {
		series.Year, _ = strconv.Atoi(detail.FirstAirDate[:4])
	}

	// 类型
	var genres []string
	for _, g := range detail.Genres {
		genres = append(genres, g.Name)
	}
	if len(genres) > 0 {
		series.Genres = strings.Join(genres, ",")
	}

	// 国家 / 语言 / 出品公司
	if len(detail.OriginCountry) > 0 {
		series.Country = strings.Join(detail.OriginCountry, ",")
	}
	if detail.OriginalLanguage != "" {
		series.Language = detail.OriginalLanguage
	}
	var studios []string
	for _, c := range detail.ProductionCompanies {
		studios = append(studios, c.Name)
	}
	if len(studios) > 0 {
		series.Studio = strings.Join(studios, ",")
	}

	// 下载海报
	if detail.PosterPath != "" {
		imageURL := fmt.Sprintf("%s/t/p/w500%s", s.getTMDbImageBase(), detail.PosterPath)
		ext := filepath.Ext(detail.PosterPath)
		if ext == "" {
			ext = ".jpg"
		}
		cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", "series", series.ID)
		os.MkdirAll(cacheDir, 0755)
		localPath := filepath.Join(cacheDir, "poster"+ext)
		if err := s.downloadToFile(imageURL, localPath); err == nil {
			series.PosterPath = localPath
		}
	}

	// 下载背景图
	if detail.BackdropPath != "" {
		imageURL := fmt.Sprintf("%s/t/p/w1280%s", s.getTMDbImageBase(), detail.BackdropPath)
		ext := filepath.Ext(detail.BackdropPath)
		if ext == "" {
			ext = ".jpg"
		}
		cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", "series", series.ID)
		os.MkdirAll(cacheDir, 0755)
		localPath := filepath.Join(cacheDir, "backdrop"+ext)
		if err := s.downloadToFile(imageURL, localPath); err == nil {
			series.BackdropPath = localPath
		}
	}

	s.logger.Infof("已手动匹配剧集合集: %s -> TMDb ID %d", series.Title, tmdbID)
	return s.seriesRepo.Update(series)
}

// MatchMediaWithTMDb 手动关联TMDb结果到指定媒体
func (s *MetadataService) MatchMediaWithTMDb(mediaID string, tmdbID int) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}

	if s.cfg.Secrets.TMDbAPIKey == "" {
		return fmt.Errorf("TMDb API Key未配置")
	}

	// 获取TMDb详情
	detail, err := s.getMovieDetail(tmdbID)
	if err != nil {
		return fmt.Errorf("获取TMDb详情失败: %w", err)
	}

	// 应用详情
	s.applyMovieDetail(media, detail)

	// 下载海报
	if detail.PosterPath != "" {
		localPath, err := s.downloadPoster(media, detail.PosterPath)
		if err == nil {
			media.PosterPath = localPath
		}
	}

	// 下载背景图
	if detail.BackdropPath != "" {
		localPath, err := s.downloadBackdrop(media, detail.BackdropPath)
		if err == nil {
			media.BackdropPath = localPath
		}
	}

	return s.mediaRepo.Update(media)
}

// mapGenreIDs 将TMDb类型ID映射为中文名
func (s *MetadataService) mapGenreIDs(ids []int) string {
	genreMap := map[int]string{
		28: "动作", 12: "冒险", 16: "动画", 35: "喜剧", 80: "犯罪",
		99: "纪录", 18: "剧情", 10751: "家庭", 14: "奇幻", 36: "历史",
		27: "恐怖", 10402: "音乐", 9648: "悬疑", 10749: "爱情",
		878: "科幻", 10770: "电视电影", 53: "惊悚", 10752: "战争", 37: "西部",
		10759: "动作冒险", 10762: "儿童", 10763: "新闻", 10764: "真人秀",
		10765: "科幻奇幻", 10766: "肥皂剧", 10767: "脱口秀", 10768: "战争政治",
	}

	var genres []string
	for _, id := range ids {
		if name, ok := genreMap[id]; ok {
			genres = append(genres, name)
		}
	}
	return strings.Join(genres, ",")
}

// ==================== Bangumi 公共方法 ====================

// SearchBangumi 公开的 Bangumi 搜索方法（用于手动元数据匹配）
func (s *MetadataService) SearchBangumi(query string, subjectType int, year int) ([]BangumiSubject, error) {
	return s.bangumi.SearchSubjects(query, subjectType, year)
}

// GetBangumiSubjectDetail 获取 Bangumi 条目详情
func (s *MetadataService) GetBangumiSubjectDetail(subjectID int) (*BangumiSubject, error) {
	return s.bangumi.GetSubjectDetail(subjectID)
}

// MatchMediaWithBangumi 手动关联 Bangumi 结果到指定媒体
func (s *MetadataService) MatchMediaWithBangumi(mediaID string, bangumiID int) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}
	return s.bangumi.MatchMediaWithBangumi(media, bangumiID)
}

// MatchSeriesWithBangumi 手动关联 Bangumi 结果到指定剧集合集
func (s *MetadataService) MatchSeriesWithBangumi(seriesID string, bangumiID int) error {
	series, err := s.seriesRepo.FindByID(seriesID)
	if err != nil {
		return fmt.Errorf("剧集合集不存在")
	}
	return s.bangumi.MatchSeriesWithBangumi(series, bangumiID)
}
