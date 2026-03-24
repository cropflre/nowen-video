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
	mediaRepo  *repository.MediaRepo
	seriesRepo *repository.SeriesRepo
	cfg        *config.Config
	logger     *zap.SugaredLogger
	client     *http.Client
	wsHub      *WSHub         // WebSocket事件广播
	douban     *DoubanService // 豆瓣刮削服务（补充源）
}

func NewMetadataService(mediaRepo *repository.MediaRepo, seriesRepo *repository.SeriesRepo, cfg *config.Config, logger *zap.SugaredLogger) *MetadataService {
	return &MetadataService{
		mediaRepo:  mediaRepo,
		seriesRepo: seriesRepo,
		cfg:        cfg,
		logger:     logger,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		douban: NewDoubanService(mediaRepo, cfg, logger),
	}
}

// SetWSHub 设置WebSocket Hub（延迟注入）
func (s *MetadataService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
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
	ID            int         `json:"id"`
	Title         string      `json:"title"`
	OriginalTitle string      `json:"original_title"`
	Overview      string      `json:"overview"`
	PosterPath    string      `json:"poster_path"`
	BackdropPath  string      `json:"backdrop_path"`
	ReleaseDate   string      `json:"release_date"`
	VoteAverage   float64     `json:"vote_average"`
	Runtime       int         `json:"runtime"`
	Genres        []TMDbGenre `json:"genres"`
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

	// 第一步：尝试TMDb刮削
	var tmdbErr error
	if s.cfg.Secrets.TMDbAPIKey != "" {
		if media.MediaType == "movie" {
			tmdbErr = s.scrapeMovie(media, searchTitle, year)
		} else {
			tmdbErr = s.scrapeTV(media, searchTitle, year)
		}
	} else {
		tmdbErr = fmt.Errorf("TMDb API Key未配置")
	}

	// 第二步：如果TMDb失败或信息不完整，用豆瓣补充
	needDouban := tmdbErr != nil || media.Overview == "" || media.PosterPath == "" || media.Rating == 0

	if needDouban {
		s.logger.Debugf("TMDb刮削%s，尝试豆瓣补充: %s",
			map[bool]string{true: "失败", false: "信息不完整"}[tmdbErr != nil],
			media.Title)

		// 重新获取最新的media数据（TMDb可能已部分更新）
		if tmdbErr == nil {
			if updated, err := s.mediaRepo.FindByID(mediaID); err == nil {
				media = updated
			}
		}

		if doubanErr := s.douban.ScrapeMedia(media, searchTitle, year); doubanErr != nil {
			s.logger.Debugf("豆瓣刮削也失败: %s - %v", media.Title, doubanErr)
			// 如果TMDb和豆瓣都失败，返回原始TMDb错误
			if tmdbErr != nil {
				return tmdbErr
			}
		} else {
			s.logger.Infof("豆瓣补充刮削成功: %s", media.Title)
		}
	}

	// 等待一下避免豆瓣限流
	time.Sleep(200 * time.Millisecond)

	return nil
}

// ScrapeLibrary 为整个媒体库刮削元数据
func (s *MetadataService) ScrapeLibrary(libraryID string, mediaList []model.Media) (int, int) {
	if s.cfg.Secrets.TMDbAPIKey == "" {
		s.logger.Warn("TMDb API Key未配置，跳过元数据刮削")
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

	apiURL := fmt.Sprintf("https://api.themoviedb.org/3/search/%s?%s", mediaType, params.Encode())
	resp, err := s.client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("TMDb请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDb返回状态码: %d", resp.StatusCode)
	}

	var result TMDbSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析TMDb响应失败: %w", err)
	}

	return result.Results, nil
}

// getMovieDetail 获取电影详情
func (s *MetadataService) getMovieDetail(tmdbID int) (*TMDbMovieDetail, error) {
	apiURL := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d?api_key=%s&language=zh-CN",
		tmdbID, s.cfg.Secrets.TMDbAPIKey)

	resp, err := s.client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDb详情请求失败: %d", resp.StatusCode)
	}

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

	imageURL := fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", size, tmdbPath)

	resp, err := s.client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("图片请求失败: %d", resp.StatusCode)
	}

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
			imageURL := fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", tmdbResult.PosterPath)
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
			imageURL := fmt.Sprintf("https://image.tmdb.org/t/p/w1280%s", tmdbResult.BackdropPath)
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

	// 保存合集元数据
	if err := s.seriesRepo.Update(series); err != nil {
		return fmt.Errorf("更新剧集合集失败: %w", err)
	}

	// 第三步：将合集元数据同步到各集（海报、评分、类型等）
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

	s.logger.Infof("剧集合集元数据刮削完成: %s, 同步了 %d 集", series.Title, len(episodes))
	return nil
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

// downloadToFile 下载文件到指定路径
func (s *MetadataService) downloadToFile(url, filePath string) error {
	resp, err := s.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
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
