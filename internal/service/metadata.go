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
	"sync"
	"sync/atomic"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// MetadataService 元数据刮削服务
type MetadataService struct {
	mediaRepo       *repository.MediaRepo
	seriesRepo      *repository.SeriesRepo
	personRepo      *repository.PersonRepo      // 演员信息仓储
	mediaPersonRepo *repository.MediaPersonRepo // 媒体-演员关联仓储
	cfg             *config.Config
	logger          *zap.SugaredLogger
	client          *http.Client
	wsHub           *WSHub          // WebSocket事件广播
	douban          *DoubanService  // 豆瓣刮削服务（补充源）
	bangumi         *BangumiService // Bangumi刮削服务（补充源）
	ai              *AIService      // AI 元数据增强（第四层 Fallback）
	providerChain   *ProviderChain  // 多数据源调度链（第三阶段）
	thetvdb         *TheTVDBService // TheTVDB 剧集增强源
}

func NewMetadataService(mediaRepo *repository.MediaRepo, seriesRepo *repository.SeriesRepo, personRepo *repository.PersonRepo, mediaPersonRepo *repository.MediaPersonRepo, cfg *config.Config, logger *zap.SugaredLogger) *MetadataService {
	s := &MetadataService{
		mediaRepo:       mediaRepo,
		seriesRepo:      seriesRepo,
		personRepo:      personRepo,
		mediaPersonRepo: mediaPersonRepo,
		cfg:             cfg,
		logger:          logger,
		client:          buildTMDbHTTPClient(cfg, logger),
		douban:          NewDoubanService(mediaRepo, cfg, logger),
		bangumi:         NewBangumiService(mediaRepo, seriesRepo, cfg, logger),
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
// 每次请求携带随机 User-Agent 和浏览器级请求头，重试间隔 2-4 秒随机化
func (s *MetadataService) tmdbGetWithRetry(url string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			s.logger.Debugf("TMDb 请求重试 (%d/2): %s", attempt+1, url)
			randomDelay(2000, 4000) // 重试间隔 2-4 秒随机化
		}
		req, reqErr := http.NewRequest("GET", url, nil)
		if reqErr != nil {
			lastErr = reqErr
			continue
		}
		setAPIHeaders(req) // 设置随机 User-Agent + 浏览器级请求头
		resp, err := s.client.Do(req)
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

// SetTheTVDBService 设置 TheTVDB 服务（延迟注入）
func (s *MetadataService) SetTheTVDBService(thetvdb *TheTVDBService) {
	s.thetvdb = thetvdb
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

// TMDbTVSeasonDetail TMDb TV 季详情（包含每集信息）
type TMDbTVSeasonDetail struct {
	ID           int             `json:"id"`
	SeasonNumber int             `json:"season_number"`
	Name         string          `json:"name"`
	Overview     string          `json:"overview"`
	AirDate      string          `json:"air_date"`
	Episodes     []TMDbTVEpisode `json:"episodes"`
}

// TMDbTVEpisode TMDb TV 单集信息
type TMDbTVEpisode struct {
	ID            int     `json:"id"`
	EpisodeNumber int     `json:"episode_number"`
	SeasonNumber  int     `json:"season_number"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	AirDate       string  `json:"air_date"`
	StillPath     string  `json:"still_path"`
	VoteAverage   float64 `json:"vote_average"`
	Runtime       int     `json:"runtime"`
}

// TMDbFindResult TMDb Find API 返回结果（通过外部 ID 查找）
type TMDbFindResult struct {
	MovieResults []TMDbMovie `json:"movie_results"`
	TVResults    []TMDbMovie `json:"tv_results"`
}

// FindByIMDbID 通过 IMDB ID 查找 TMDb 条目
// 使用 TMDb 的 /3/find/{external_id} API，支持 IMDB ID (tt开头) 转换为 TMDb ID
// 返回: (TMDb搜索结果, 媒体类型 "movie"/"tv", 错误)
func (s *MetadataService) FindByIMDbID(imdbID string) (*TMDbMovie, string, error) {
	if s.cfg.Secrets.TMDbAPIKey == "" {
		return nil, "", fmt.Errorf("TMDb API Key 未配置")
	}

	apiURL := fmt.Sprintf("%s/3/find/%s?api_key=%s&external_source=imdb_id&language=zh-CN",
		s.getTMDbAPIBase(), imdbID, s.cfg.Secrets.TMDbAPIKey)

	resp, err := s.tmdbGetWithRetry(apiURL)
	if err != nil {
		return nil, "", fmt.Errorf("TMDb Find API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	var result TMDbFindResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("解析 TMDb Find 响应失败: %w", err)
	}

	// 优先返回电影结果
	if len(result.MovieResults) > 0 {
		s.logger.Infof("IMDB ID %s -> TMDb 电影 ID %d (%s)", imdbID, result.MovieResults[0].ID, result.MovieResults[0].Title)
		return &result.MovieResults[0], "movie", nil
	}

	// 其次返回剧集结果
	if len(result.TVResults) > 0 {
		name := result.TVResults[0].Name
		if name == "" {
			name = result.TVResults[0].Title
		}
		s.logger.Infof("IMDB ID %s -> TMDb 剧集 ID %d (%s)", imdbID, result.TVResults[0].ID, name)
		return &result.TVResults[0], "tv", nil
	}

	return nil, "", fmt.Errorf("IMDB ID %s 在 TMDb 中未找到匹配结果", imdbID)
}

// ConvertIMDbToTMDbID 将 IMDB ID 转换为 TMDb ID
// 返回: (tmdbID, mediaType, error)
func (s *MetadataService) ConvertIMDbToTMDbID(imdbID string) (int, string, error) {
	movie, mediaType, err := s.FindByIMDbID(imdbID)
	if err != nil {
		return 0, "", err
	}
	return movie.ID, mediaType, nil
}

// TMDbCredits TMDb 演职人员信息（/movie/{id}/credits 或 /tv/{id}/credits 返回）
type TMDbCredits struct {
	Cast []TMDbCastMember `json:"cast"`
	Crew []TMDbCrewMember `json:"crew"`
}

// TMDbCastMember TMDb 演员信息
type TMDbCastMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
	Order       int    `json:"order"`
}

// TMDbCrewMember TMDb 剧组成员信息
type TMDbCrewMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
}

// ==================== 核心方法 ====================

// ScrapeMedia 为单个媒体项刮削元数据
func (s *MetadataService) ScrapeMedia(mediaID string) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}

	s.logger.Infof("开始刮削元数据: %s", media.Title)

	// P3: 更新刮削尝试次数和时间
	now := time.Now()
	media.ScrapeAttempts++
	media.LastScrapeAt = &now

	// P1: 如果扫描阶段已从文件名解析到 IMDB ID，先转换为 TMDb ID
	if media.IMDbID != "" && media.TMDbID == 0 {
		s.logger.Infof("检测到 IMDbID=%s（来自文件名），通过 TMDb Find API 转换: %s", media.IMDbID, media.Title)
		tmdbID, mediaType, err := s.ConvertIMDbToTMDbID(media.IMDbID)
		if err == nil {
			media.TMDbID = tmdbID
			if mediaType == "tv" && media.MediaType == "movie" {
				media.MediaType = "episode"
			}
			s.logger.Infof("IMDB ID %s -> TMDb ID %d (%s)", media.IMDbID, tmdbID, mediaType)
		} else {
			s.logger.Debugf("IMDB ID %s 转换失败，回退到搜索模式: %v", media.IMDbID, err)
		}
	}

	// P1: 如果扫描阶段已从文件名解析到 TMDb ID，直接用 ID 获取详情，跳过搜索步骤
	if media.TMDbID > 0 && media.Overview == "" {
		s.logger.Infof("检测到 TMDbID=%d（来自文件名），直接使用 ID 刮削: %s", media.TMDbID, media.Title)
		var idErr error
		if media.MediaType == "movie" {
			idErr = s.scrapeMovieByTMDbID(media, media.TMDbID)
		} else {
			idErr = s.scrapeTVByTMDbID(media, media.TMDbID)
		}
		if idErr == nil {
			media.ScrapeStatus = "scraped" // P3: 标记刮削成功
			if saveErr := s.mediaRepo.Update(media); saveErr != nil {
				s.logger.Errorf("保存元数据失败: %s - %v", media.Title, saveErr)
				return saveErr
			}
			s.logger.Infof("TMDb ID 直连刮削成功: %s", media.Title)
			randomDelay(1500, 3000)
			return nil
		}
		s.logger.Debugf("TMDb ID 直连刮削失败，回退到搜索模式: %s - %v", media.Title, idErr)
	}

	// 从标题中提取搜索关键词和年份
	searchTitle, year := s.parseTitle(media.Title)
	// 如果扫描阶段已提取到年份但标题解析未提取到，使用扫描阶段的年份
	if year == 0 && media.Year > 0 {
		year = media.Year
	}

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
		media.ScrapeStatus = "scraped" // P3: 标记刮削成功
		s.mediaRepo.UpdateFields(media.ID, map[string]interface{}{"scrape_status": "scraped"})
		s.logger.Infof("元数据刮削完成 (多数据源): %s", media.Title)
		randomDelay(1500, 3000) // 单次刮削完成后等待 1.5-3 秒，防止限流
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
							updated.ScrapeStatus = "scraped" // P3: AI 增强成功
							s.mediaRepo.Update(updated)
							return nil
						}
					}
					// P3: 所有数据源都失败，标记为 failed
					s.mediaRepo.UpdateFields(mediaID, map[string]interface{}{
						"scrape_status": "failed",
					})
					return tmdbErr
				}
			}
		}
	}

	// P3: 旧版路径刮削成功，标记状态
	s.mediaRepo.UpdateFields(media.ID, map[string]interface{}{
		"scrape_status": "scraped",
	})

	// 等待一下避免限流（随机 1.5-3 秒）
	randomDelay(1500, 3000)

	return nil
}

// ScrapeLibrary 为整个媒体库刮削元数据（P3: 并发刮削池 + 失败重试窗口）
func (s *MetadataService) ScrapeLibrary(libraryID string, mediaList []model.Media) (int, int) {
	if s.cfg.Secrets.TMDbAPIKey == "" && s.providerChain == nil {
		s.logger.Warn("TMDb API Key未配置且无可用数据源，跳过元数据刮削")
		return 0, 0
	}

	// P3: 使用增强的过滤逻辑（排除最近 7 天内已失败的记录）
	var needScrape []model.Media
	for _, media := range mediaList {
		// 跳过已成功刮削的记录（避免重复刮削已有元数据的电影）
		if media.ScrapeStatus == "scraped" {
			continue
		}
		if media.Overview == "" || media.PosterPath == "" {
			// P3: 跳过最近 7 天内已尝试刮削但失败的记录
			if media.ScrapeStatus == "failed" && media.LastScrapeAt != nil {
				if time.Since(*media.LastScrapeAt) < 7*24*time.Hour {
					continue
				}
			}
			// P3: 跳过手动标记为不需要刮削的记录
			if media.ScrapeStatus == "manual" {
				continue
			}
			needScrape = append(needScrape, media)
		}
	}

	if len(needScrape) == 0 {
		return 0, 0
	}

	total := len(needScrape)
	var success int32
	var failed int32

	// 发送刮削开始事件
	s.broadcastScrapeEvent(EventScrapeStarted, &ScrapeProgressData{
		LibraryID: libraryID,
		Total:     total,
		Message:   fmt.Sprintf("开始元数据刮削，共 %d 个媒体待处理", total),
	})

	// P3: 并发刮削池（默认 3 个 worker，考虑 TMDb 限速 40 req/10s）
	workerCount := 3
	jobs := make(chan int, total)
	var wg sync.WaitGroup
	var mu sync.Mutex
	processed := 0

	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				media := needScrape[idx]
				if err := s.ScrapeMedia(media.ID); err != nil {
					s.logger.Debugf("刮削失败: %s - %v", media.Title, err)
					// P3: 标记刮削失败状态
					s.mediaRepo.UpdateFields(media.ID, map[string]interface{}{
						"scrape_status": "failed",
					})
					atomic.AddInt32(&failed, 1)
				} else {
					atomic.AddInt32(&success, 1)
				}

				// 发送刮削进度事件（加锁保证顺序）
				mu.Lock()
				processed++
				currentProcessed := processed
				currentSuccess := int(atomic.LoadInt32(&success))
				currentFailed := int(atomic.LoadInt32(&failed))
				mu.Unlock()

				s.broadcastScrapeEvent(EventScrapeProgress, &ScrapeProgressData{
					LibraryID:  libraryID,
					Current:    currentProcessed,
					Total:      total,
					Success:    currentSuccess,
					Failed:     currentFailed,
					MediaTitle: media.Title,
					Message:    fmt.Sprintf("刮削进度: [%d/%d] %s", currentProcessed, total, media.Title),
				})

				// TMDb限速保护：每个 worker 内部间隔 2-5 秒随机化
				randomDelay(2000, 5000)
			}
		}()
	}

	// 分发任务
	for i := range needScrape {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	finalSuccess := int(atomic.LoadInt32(&success))
	finalFailed := int(atomic.LoadInt32(&failed))

	// 发送刮削完成事件
	s.broadcastScrapeEvent(EventScrapeCompleted, &ScrapeProgressData{
		LibraryID: libraryID,
		Current:   total,
		Total:     total,
		Success:   finalSuccess,
		Failed:    finalFailed,
		Message:   fmt.Sprintf("元数据刮削完成: 成功 %d, 失败 %d", finalSuccess, finalFailed),
	})

	s.logger.Infof("元数据刮削完成: 成功 %d, 失败 %d (并发 %d workers)", finalSuccess, finalFailed, workerCount)
	return finalSuccess, finalFailed
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

	// P1: 使用置信度排序选择最佳匹配结果
	best := s.bestMatchResult(results, searchTitle, year)

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

	// 获取并保存演职人员
	if s.cfg.Secrets.TMDbAPIKey != "" && best.ID > 0 {
		if credits, err := s.getTMDbMovieCredits(best.ID); err == nil {
			s.saveCreditsForMedia(media.ID, credits)
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

	// P1: 使用置信度排序选择最佳匹配结果
	best := s.bestMatchResult(results, searchTitle, year)

	// 应用搜索结果
	title := best.Name
	if title == "" {
		title = best.Title
	}
	origTitle := best.OriginalName
	if origTitle == "" {
		origTitle = best.OriginalTitle
	}

	if title != "" {
		media.Title = title
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
	if best.ReleaseDate != "" {
		media.Premiered = best.ReleaseDate
	} else if best.FirstAirDate != "" {
		media.Premiered = best.FirstAirDate
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

// scrapeMovieByTMDbID 通过 TMDb ID 直接获取电影详情（跳过搜索步骤）
// P1: 当文件名中包含 [tmdbid=xxx] 标签时调用，100% 精确匹配
func (s *MetadataService) scrapeMovieByTMDbID(media *model.Media, tmdbID int) error {
	detail, err := s.getMovieDetail(tmdbID)
	if err != nil {
		return fmt.Errorf("TMDb ID=%d 获取电影详情失败: %w", tmdbID, err)
	}
	s.applyMovieDetail(media, detail)
	media.TMDbID = tmdbID

	// 下载海报
	if detail.PosterPath != "" {
		localPath, dlErr := s.downloadPoster(media, detail.PosterPath)
		if dlErr == nil {
			media.PosterPath = localPath
		}
	}
	// 下载背景图
	if detail.BackdropPath != "" {
		localPath, dlErr := s.downloadBackdrop(media, detail.BackdropPath)
		if dlErr == nil {
			media.BackdropPath = localPath
		}
	}
	// 获取演职人员
	if credits, credErr := s.getTMDbMovieCredits(tmdbID); credErr == nil {
		s.saveCreditsForMedia(media.ID, credits)
	}

	return s.mediaRepo.Update(media)
}

// scrapeTVByTMDbID 通过 TMDb ID 直接获取剧集详情（跳过搜索步骤）
func (s *MetadataService) scrapeTVByTMDbID(media *model.Media, tmdbID int) error {
	// 使用 TMDb TV Detail API
	apiURL := fmt.Sprintf("%s/3/tv/%d?api_key=%s&language=zh-CN",
		s.getTMDbAPIBase(), tmdbID, s.cfg.Secrets.TMDbAPIKey)
	resp, err := s.tmdbGetWithRetry(apiURL)
	if err != nil {
		return fmt.Errorf("TMDb TV ID=%d 获取详情失败: %w", tmdbID, err)
	}
	defer resp.Body.Close()

	var tvDetail struct {
		ID           int     `json:"id"`
		Name         string  `json:"name"`
		OriginalName string  `json:"original_name"`
		Overview     string  `json:"overview"`
		PosterPath   string  `json:"poster_path"`
		BackdropPath string  `json:"backdrop_path"`
		FirstAirDate string  `json:"first_air_date"`
		VoteAverage  float64 `json:"vote_average"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tvDetail); err != nil {
		return fmt.Errorf("解析 TMDb TV 详情失败: %w", err)
	}

	if tvDetail.Name != "" {
		media.Title = tvDetail.Name
	}
	media.OrigTitle = tvDetail.OriginalName
	if tvDetail.Overview != "" {
		media.Overview = tvDetail.Overview
	}
	media.Rating = tvDetail.VoteAverage
	media.TMDbID = tmdbID

	if len(tvDetail.FirstAirDate) >= 4 {
		media.Year, _ = strconv.Atoi(tvDetail.FirstAirDate[:4])
	}
	if tvDetail.FirstAirDate != "" {
		media.Premiered = tvDetail.FirstAirDate
	}

	// 下载海报
	if tvDetail.PosterPath != "" {
		localPath, dlErr := s.downloadPoster(media, tvDetail.PosterPath)
		if dlErr == nil {
			media.PosterPath = localPath
		}
	}
	if tvDetail.BackdropPath != "" {
		localPath, dlErr := s.downloadBackdrop(media, tvDetail.BackdropPath)
		if dlErr == nil {
			media.BackdropPath = localPath
		}
	}

	return s.mediaRepo.Update(media)
}

// bestMatchResult 从搜索结果中选择最佳匹配（P1: 置信度排序替代简单取 results[0]）
// 综合考虑标题匹配度、年份匹配、数据完整性等因素
func (s *MetadataService) bestMatchResult(results []TMDbMovie, searchTitle string, year int) TMDbMovie {
	if len(results) == 1 {
		return results[0]
	}

	type scored struct {
		index int
		score float64
	}

	searchLower := strings.ToLower(strings.TrimSpace(searchTitle))
	var candidates []scored

	for i, r := range results {
		score := 0.0

		// 获取标题
		title := r.Title
		if title == "" {
			title = r.Name
		}
		origTitle := r.OriginalTitle
		if origTitle == "" {
			origTitle = r.OriginalName
		}

		titleLower := strings.ToLower(strings.TrimSpace(title))
		origTitleLower := strings.ToLower(strings.TrimSpace(origTitle))

		// 标题完全匹配 → +50 分
		if titleLower == searchLower || origTitleLower == searchLower {
			score += 50
		} else if strings.Contains(titleLower, searchLower) || strings.Contains(origTitleLower, searchLower) {
			// 标题包含搜索词 → +30 分
			score += 30
		} else if strings.Contains(searchLower, titleLower) && len(titleLower) > 2 {
			// 搜索词包含标题 → +20 分
			score += 20
		}

		// 年份匹配 → +30 分（精确）或 +15 分（相差 1 年）
		if year > 0 {
			dateStr := r.ReleaseDate
			if dateStr == "" {
				dateStr = r.FirstAirDate
			}
			if len(dateStr) >= 4 {
				resultYear, _ := strconv.Atoi(dateStr[:4])
				if resultYear == year {
					score += 30
				} else if resultYear > 0 && absInt(resultYear-year) == 1 {
					score += 15
				}
			}
		}

		// 数据完整性加分
		if r.PosterPath != "" {
			score += 5
		}
		if r.Overview != "" {
			score += 5
		}
		if r.VoteAverage > 0 {
			score += 5
		}

		// 排名权重（搜索结果本身有排序，前面的结果轻微加分）
		if i < 3 {
			score += float64(3-i) * 2
		}

		candidates = append(candidates, scored{i, score})
	}

	// 选择得分最高的
	bestIdx := 0
	bestScore := 0.0
	for _, c := range candidates {
		if c.score > bestScore {
			bestScore = c.score
			bestIdx = c.index
		}
	}

	s.logger.Debugf("搜索结果匹配排序: 搜索=%q year=%d, 最佳=#%d (%.1f分), 共%d个候选",
		searchTitle, year, bestIdx+1, bestScore, len(results))

	return results[bestIdx]
}

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

// getTVSeasonDetail 获取 TMDb TV 季详情（包含每集的标题、简介等）
// API: /3/tv/{tv_id}/season/{season_number}
func (s *MetadataService) getTVSeasonDetail(tvID int, seasonNum int) (*TMDbTVSeasonDetail, error) {
	apiURL := fmt.Sprintf("%s/3/tv/%d/season/%d?api_key=%s&language=zh-CN",
		s.getTMDbAPIBase(), tvID, seasonNum, s.cfg.Secrets.TMDbAPIKey)

	resp, err := s.tmdbGetWithRetry(apiURL)
	if err != nil {
		return nil, fmt.Errorf("TMDb 季详情请求失败: %w", err)
	}
	defer resp.Body.Close()

	var detail TMDbTVSeasonDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("解析 TMDb 季详情失败: %w", err)
	}

	return &detail, nil
}

// scrapeSeriesEpisodes 从 TMDb 获取逐集元数据（每集独立的标题和简介）
// 遍历系列中所有季，调用 /3/tv/{id}/season/{num} 获取每集数据，并更新到对应的 Media 记录
func (s *MetadataService) scrapeSeriesEpisodes(seriesID string, tmdbID int) {
	if tmdbID == 0 || s.cfg.Secrets.TMDbAPIKey == "" {
		return
	}

	episodes, err := s.mediaRepo.ListBySeriesID(seriesID)
	if err != nil || len(episodes) == 0 {
		return
	}

	// 收集所有需要刮削的季号（去重）
	seasonSet := make(map[int]bool)
	for _, ep := range episodes {
		sn := ep.SeasonNum
		if sn == 0 {
			sn = 1
		}
		seasonSet[sn] = true
	}

	// 按季构建 episode 查找索引：seasonNum -> episodeNum -> *Media
	epIndex := make(map[int]map[int]*model.Media)
	for i := range episodes {
		ep := &episodes[i]
		sn := ep.SeasonNum
		if sn == 0 {
			sn = 1
		}
		if epIndex[sn] == nil {
			epIndex[sn] = make(map[int]*model.Media)
		}
		epIndex[sn][ep.EpisodeNum] = ep
	}

	totalUpdated := 0

	for seasonNum := range seasonSet {
		seasonDetail, err := s.getTVSeasonDetail(tmdbID, seasonNum)
		if err != nil {
			s.logger.Debugf("获取 TMDb 季 %d 详情失败 (tmdb_id=%d): %v", seasonNum, tmdbID, err)
			continue
		}

		seasonEpMap := epIndex[seasonNum]
		if seasonEpMap == nil {
			continue
		}

		for _, tmdbEp := range seasonDetail.Episodes {
			localEp, ok := seasonEpMap[tmdbEp.EpisodeNumber]
			if !ok {
				continue
			}

			updated := false

			// 更新单集简介（核心功能：确保每集有独立简介）
			if tmdbEp.Overview != "" && localEp.Overview == "" {
				localEp.Overview = tmdbEp.Overview
				updated = true
			}

			// 更新单集标题
			if tmdbEp.Name != "" && localEp.EpisodeTitle == "" {
				localEp.EpisodeTitle = tmdbEp.Name
				updated = true
			}

			// 更新单集评分（如果有独立评分且本地未设置）
			if tmdbEp.VoteAverage > 0 && localEp.Rating == 0 {
				localEp.Rating = tmdbEp.VoteAverage
				updated = true
			}

			// 更新单集时长
			if tmdbEp.Runtime > 0 && localEp.Runtime == 0 {
				localEp.Runtime = tmdbEp.Runtime
				updated = true
			}

			// 下载每集独立截图（still）作为该集的封面
			if tmdbEp.StillPath != "" && localEp.PosterPath == "" {
				stillURL := fmt.Sprintf("%s/t/p/w500%s", s.getTMDbImageBase(), tmdbEp.StillPath)
				ext := filepath.Ext(tmdbEp.StillPath)
				if ext == "" {
					ext = ".jpg"
				}
				cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", localEp.ID)
				if err := os.MkdirAll(cacheDir, 0755); err == nil {
					localPath := filepath.Join(cacheDir, "poster"+ext)
					if err := s.downloadToFile(stillURL, localPath); err == nil {
						localEp.PosterPath = localPath
						updated = true
						s.logger.Debugf("下载剧集 S%02dE%02d 独立封面成功: %s", seasonNum, tmdbEp.EpisodeNumber, tmdbEp.StillPath)
					} else {
						s.logger.Debugf("下载剧集 S%02dE%02d 独立封面失败: %v", seasonNum, tmdbEp.EpisodeNumber, err)
					}
				}
			}

			if updated {
				if err := s.mediaRepo.Update(localEp); err != nil {
					s.logger.Debugf("更新剧集 S%02dE%02d 元数据失败: %v", seasonNum, tmdbEp.EpisodeNumber, err)
				} else {
					totalUpdated++
				}
			}
		}

		// 避免 TMDb API 速率限制（季间间隔 2-4 秒随机化）
		randomDelay(2000, 4000)
	}

	if totalUpdated > 0 {
		s.logger.Infof("TMDb 逐集刮削完成: 共更新 %d 集的独立元数据 (tmdb_id=%d)", totalUpdated, tmdbID)
	}
}

// ==================== 演职人员刮削 ====================

// getTMDbMovieCredits 获取电影演职人员
func (s *MetadataService) getTMDbMovieCredits(tmdbID int) (*TMDbCredits, error) {
	apiURL := fmt.Sprintf("%s/3/movie/%d/credits?api_key=%s&language=zh-CN",
		s.getTMDbAPIBase(), tmdbID, s.cfg.Secrets.TMDbAPIKey)

	resp, err := s.tmdbGetWithRetry(apiURL)
	if err != nil {
		return nil, fmt.Errorf("TMDb 电影演职人员请求失败: %w", err)
	}
	defer resp.Body.Close()

	var credits TMDbCredits
	if err := json.NewDecoder(resp.Body).Decode(&credits); err != nil {
		return nil, fmt.Errorf("解析 TMDb 演职人员失败: %w", err)
	}
	return &credits, nil
}

// getTMDbTVCredits 获取剧集演职人员
func (s *MetadataService) getTMDbTVCredits(tmdbID int) (*TMDbCredits, error) {
	apiURL := fmt.Sprintf("%s/3/tv/%d/credits?api_key=%s&language=zh-CN",
		s.getTMDbAPIBase(), tmdbID, s.cfg.Secrets.TMDbAPIKey)

	resp, err := s.tmdbGetWithRetry(apiURL)
	if err != nil {
		return nil, fmt.Errorf("TMDb 剧集演职人员请求失败: %w", err)
	}
	defer resp.Body.Close()

	var credits TMDbCredits
	if err := json.NewDecoder(resp.Body).Decode(&credits); err != nil {
		return nil, fmt.Errorf("解析 TMDb 演职人员失败: %w", err)
	}
	return &credits, nil
}

// saveCreditsForMedia 将 TMDb Credits 保存为 Media 的演职人员记录
func (s *MetadataService) saveCreditsForMedia(mediaID string, credits *TMDbCredits) {
	if s.personRepo == nil || s.mediaPersonRepo == nil || credits == nil {
		return
	}

	// 先清除旧的关联数据，避免重复
	s.mediaPersonRepo.DeleteByMediaID(mediaID)

	saved := 0

	// 保存导演（从 crew 中筛选 job=Director）
	for _, crew := range credits.Crew {
		if crew.Job != "Director" {
			continue
		}
		person, err := s.personRepo.FindOrCreate(crew.Name, crew.ID)
		if err != nil {
			continue
		}
		// 更新头像
		if crew.ProfilePath != "" && person.ProfileURL == "" {
			person.ProfileURL = fmt.Sprintf("%s/t/p/w185%s", s.getTMDbImageBase(), crew.ProfilePath)
			s.personRepo.Update(person)
		}
		mp := &model.MediaPerson{
			MediaID:   mediaID,
			PersonID:  person.ID,
			Role:      "director",
			SortOrder: saved,
		}
		if err := s.mediaPersonRepo.Create(mp); err == nil {
			saved++
		}
	}

	// 保存编剧（从 crew 中筛选 job=Writer 或 Screenplay）
	for _, crew := range credits.Crew {
		if crew.Job != "Writer" && crew.Job != "Screenplay" && crew.Job != "Novel" {
			continue
		}
		person, err := s.personRepo.FindOrCreate(crew.Name, crew.ID)
		if err != nil {
			continue
		}
		if crew.ProfilePath != "" && person.ProfileURL == "" {
			person.ProfileURL = fmt.Sprintf("%s/t/p/w185%s", s.getTMDbImageBase(), crew.ProfilePath)
			s.personRepo.Update(person)
		}
		mp := &model.MediaPerson{
			MediaID:   mediaID,
			PersonID:  person.ID,
			Role:      "writer",
			SortOrder: saved,
		}
		if err := s.mediaPersonRepo.Create(mp); err == nil {
			saved++
		}
	}

	// 保存演员（最多 20 个）
	maxActors := 20
	for i, cast := range credits.Cast {
		if i >= maxActors {
			break
		}
		person, err := s.personRepo.FindOrCreate(cast.Name, cast.ID)
		if err != nil {
			continue
		}
		if cast.ProfilePath != "" && person.ProfileURL == "" {
			person.ProfileURL = fmt.Sprintf("%s/t/p/w185%s", s.getTMDbImageBase(), cast.ProfilePath)
			s.personRepo.Update(person)
		}
		mp := &model.MediaPerson{
			MediaID:   mediaID,
			PersonID:  person.ID,
			Role:      "actor",
			Character: cast.Character,
			SortOrder: saved,
		}
		if err := s.mediaPersonRepo.Create(mp); err == nil {
			saved++
		}
	}

	if saved > 0 {
		s.logger.Debugf("已保存 %d 个演职人员 (media_id=%s)", saved, mediaID)
	}
}

// saveCreditsForSeries 将 TMDb Credits 保存为 Series 的演职人员记录
func (s *MetadataService) saveCreditsForSeries(seriesID string, credits *TMDbCredits) {
	if s.personRepo == nil || s.mediaPersonRepo == nil || credits == nil {
		return
	}

	// 先清除旧的关联数据，避免重复
	s.mediaPersonRepo.DeleteBySeriesID(seriesID)

	saved := 0

	// 保存导演 / 剧集创建者
	for _, crew := range credits.Crew {
		if crew.Job != "Director" && crew.Job != "Executive Producer" {
			continue
		}
		role := "director"
		if crew.Job == "Executive Producer" {
			role = "writer" // 剧集中 EP 近似于编剧/创作者
		}
		person, err := s.personRepo.FindOrCreate(crew.Name, crew.ID)
		if err != nil {
			continue
		}
		if crew.ProfilePath != "" && person.ProfileURL == "" {
			person.ProfileURL = fmt.Sprintf("%s/t/p/w185%s", s.getTMDbImageBase(), crew.ProfilePath)
			s.personRepo.Update(person)
		}
		mp := &model.MediaPerson{
			SeriesID:  seriesID,
			PersonID:  person.ID,
			Role:      role,
			SortOrder: saved,
		}
		if err := s.mediaPersonRepo.Create(mp); err == nil {
			saved++
		}
	}

	// 保存演员（最多 20 个）
	maxActors := 20
	for i, cast := range credits.Cast {
		if i >= maxActors {
			break
		}
		person, err := s.personRepo.FindOrCreate(cast.Name, cast.ID)
		if err != nil {
			continue
		}
		if cast.ProfilePath != "" && person.ProfileURL == "" {
			person.ProfileURL = fmt.Sprintf("%s/t/p/w185%s", s.getTMDbImageBase(), cast.ProfilePath)
			s.personRepo.Update(person)
		}
		mp := &model.MediaPerson{
			SeriesID:  seriesID,
			PersonID:  person.ID,
			Role:      "actor",
			Character: cast.Character,
			SortOrder: saved,
		}
		if err := s.mediaPersonRepo.Create(mp); err == nil {
			saved++
		}
	}

	if saved > 0 {
		s.logger.Infof("已保存 %d 个演职人员 (series_id=%s)", saved, seriesID)
	}
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

	if title != "" {
		media.Title = title
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
	if result.ReleaseDate != "" {
		media.Premiered = result.ReleaseDate
	}

	// 类型映射
	media.Genres = s.mapGenreIDs(result.GenreIDs)
}

// applyMovieDetail 将电影详情应用到媒体
func (s *MetadataService) applyMovieDetail(media *model.Media, detail *TMDbMovieDetail) {
	if detail.Title != "" {
		media.Title = detail.Title
	}
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
	if detail.ReleaseDate != "" {
		media.Premiered = detail.ReleaseDate
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

		// 获取并保存演职人员
		if series.TMDbID > 0 && s.cfg.Secrets.TMDbAPIKey != "" {
			if credits, credErr := s.getTMDbTVCredits(series.TMDbID); credErr == nil {
				s.saveCreditsForSeries(seriesID, credits)
			}
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
		series.TMDbID = tmdbResult.ID // 保存 TMDb ID，用于后续逐集刮削
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

	// 获取并保存演职人员
	if series.TMDbID > 0 && s.cfg.Secrets.TMDbAPIKey != "" {
		if credits, err := s.getTMDbTVCredits(series.TMDbID); err == nil {
			s.saveCreditsForSeries(seriesID, credits)
		}
	}

	// 第三步：将合集元数据同步到各集（海报、评分、类型等）
	s.syncSeriesToEpisodes(seriesID, series)

	s.logger.Infof("剧集合集元数据刮削完成: %s", series.Title)
	return nil
}

// syncSeriesToEpisodes 将合集元数据同步到各集（海报、评分、类型等）
// 先尝试 TMDb 逐集刮削获取每集独立简介/标题，然后用合集元数据补充仍然为空的字段
func (s *MetadataService) syncSeriesToEpisodes(seriesID string, series *model.Series) {
	// 第一步：尝试 TMDb 逐集刮削（获取每集独立的简介和标题）
	if series.TMDbID > 0 {
		s.scrapeSeriesEpisodes(seriesID, series.TMDbID)
	}

	// 第二步：用合集元数据补充仍然为空的字段（作为 fallback）
	episodes, _ := s.mediaRepo.ListBySeriesID(seriesID)
	for _, ep := range episodes {
		updated := false
		// 仅当逐集刮削未获取到简介时，才用合集简介补充
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

	// 过滤出需要刮削的剧集（跳过已成功刮削的）
	var needScrape []model.Series
	for _, s := range seriesList {
		// 已有完整元数据的剧集跳过（Overview 和 PosterPath 都不为空）
		if s.Overview != "" && s.PosterPath != "" {
			continue
		}
		needScrape = append(needScrape, s)
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
		// 限速保护（剧集间隔 3-6 秒随机化）
		randomDelay(3000, 6000)
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
	media.IMDbID = ""
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
	series.IMDbID = ""
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
		series.Title = detail.Name
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
	if err := s.seriesRepo.Update(series); err != nil {
		return err
	}

	// 获取并保存演职人员
	if credits, err := s.getTMDbTVCredits(tmdbID); err == nil {
		s.saveCreditsForSeries(seriesID, credits)
	}

	// 手动匹配后也执行逐集刮削，获取每集独立简介和标题
	s.scrapeSeriesEpisodes(seriesID, tmdbID)

	// 将合集元数据同步到仍然为空的字段
	s.syncSeriesToEpisodes(seriesID, series)

	return nil
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

	// 获取并保存演职人员
	if credits, err := s.getTMDbMovieCredits(tmdbID); err == nil {
		s.saveCreditsForMedia(media.ID, credits)
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

// ==================== 豆瓣公共方法 ====================

// SearchDouban 公开的豆瓣搜索方法（用于手动元数据匹配）
func (s *MetadataService) SearchDouban(query string, year int) ([]DoubanSearchResult, error) {
	return s.douban.searchDouban(query, year)
}

// MatchMediaWithDouban 手动关联豆瓣结果到指定媒体
func (s *MetadataService) MatchMediaWithDouban(mediaID string, doubanID string) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}

	// 获取豆瓣详情
	detail, err := s.douban.getSubjectDetail(doubanID)
	if err != nil {
		return fmt.Errorf("获取豆瓣详情失败: %w", err)
	}

	// 应用豆瓣数据（覆盖写入）
	if detail.Title != "" {
		media.Title = detail.Title
	}
	if detail.Rating.Average > 0 {
		media.Rating = detail.Rating.Average
	}
	if detail.Overview != "" {
		media.Overview = detail.Overview
	}
	if len(detail.Genres) > 0 {
		media.Genres = strings.Join(detail.Genres, ",")
	}
	if detail.Year != "" {
		if y, err := strconv.Atoi(detail.Year); err == nil {
			media.Year = y
		}
	}
	media.DoubanID = doubanID

	// 下载豆瓣封面
	if detail.Cover != "" && media.PosterPath == "" {
		localPath, err := s.douban.downloadDoubanCover(media, detail.Cover)
		if err == nil {
			media.PosterPath = localPath
		}
	}

	return s.mediaRepo.Update(media)
}

// MatchSeriesWithDouban 手动关联豆瓣结果到指定剧集合集
func (s *MetadataService) MatchSeriesWithDouban(seriesID string, doubanID string) error {
	series, err := s.seriesRepo.FindByID(seriesID)
	if err != nil {
		return fmt.Errorf("剧集合集不存在")
	}

	detail, err := s.douban.getSubjectDetail(doubanID)
	if err != nil {
		return fmt.Errorf("获取豆瓣详情失败: %w", err)
	}

	if detail.Title != "" {
		series.Title = detail.Title
	}
	if detail.Rating.Average > 0 {
		series.Rating = detail.Rating.Average
	}
	if detail.Overview != "" {
		series.Overview = detail.Overview
	}
	if len(detail.Genres) > 0 {
		series.Genres = strings.Join(detail.Genres, ",")
	}
	if detail.Year != "" {
		if y, err := strconv.Atoi(detail.Year); err == nil {
			series.Year = y
		}
	}
	series.DoubanID = doubanID

	return s.seriesRepo.Update(series)
}

// ==================== TheTVDB 公共方法 ====================

// SearchTheTVDB 公开的 TheTVDB 搜索方法（用于手动元数据匹配）
func (s *MetadataService) SearchTheTVDB(query string, year int) ([]TheTVDBSeries, error) {
	if s.thetvdb == nil || !s.thetvdb.IsEnabled() {
		return nil, fmt.Errorf("TheTVDB 未配置或不可用")
	}
	return s.thetvdb.SearchSeries(query, year)
}

// MatchSeriesWithTheTVDB 手动关联 TheTVDB 结果到指定剧集合集
func (s *MetadataService) MatchSeriesWithTheTVDB(seriesID string, tvdbID int) error {
	if s.thetvdb == nil || !s.thetvdb.IsEnabled() {
		return fmt.Errorf("TheTVDB 未配置或不可用")
	}

	series, err := s.seriesRepo.FindByID(seriesID)
	if err != nil {
		return fmt.Errorf("剧集合集不存在")
	}

	// 获取 TheTVDB 详情
	detail, err := s.thetvdb.GetSeriesDetail(tvdbID)
	if err != nil {
		return fmt.Errorf("获取 TheTVDB 详情失败: %w", err)
	}

	// 应用 TheTVDB 数据
	s.thetvdb.applySeriesResult(series, detail)

	return s.seriesRepo.Update(series)
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
