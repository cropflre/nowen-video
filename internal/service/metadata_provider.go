package service

import (
	"fmt"
	"os"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"go.uber.org/zap"
)

// ==================== 统一 MetadataProvider 接口 ====================

// MetadataResult 通用元数据结果（各数据源统一输出格式）
type MetadataResult struct {
	// 基础信息
	Title     string  `json:"title"`
	OrigTitle string  `json:"orig_title"`
	Overview  string  `json:"overview"`
	Year      int     `json:"year"`
	Rating    float64 `json:"rating"`
	Genres    string  `json:"genres"`
	// 图片
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
	LogoPath     string `json:"logo_path"`     // ClearLogo（Fanart.tv 专属）
	ThumbPath    string `json:"thumb_path"`    // 缩略图
	DiscArtPath  string `json:"disc_art_path"` // 光碟封面（Fanart.tv 专属）
	BannerPath   string `json:"banner_path"`   // 横幅图
	// 扩展信息
	Country    string `json:"country"`
	Language   string `json:"language"`
	Studio     string `json:"studio"`
	Tagline    string `json:"tagline"`
	TrailerURL string `json:"trailer_url"`
	Runtime    int    `json:"runtime"`
	// 外部 ID
	TMDbID    int    `json:"tmdb_id"`
	DoubanID  string `json:"douban_id"`
	BangumiID int    `json:"bangumi_id"`
	TVDbID    int    `json:"tvdb_id"`
	IMDbID    string `json:"imdb_id"`
	// 数据源标识
	Source     string  `json:"source"`     // 数据来源标识（tmdb/douban/bangumi/thetvdb/fanart/ai）
	Confidence float64 `json:"confidence"` // 匹配置信度（0-1）
}

// MetadataProvider 元数据提供者接口
// 所有数据源（TMDb、豆瓣、Bangumi、TheTVDB、Fanart.tv、AI）都实现此接口
type MetadataProvider interface {
	// Name 返回数据源名称
	Name() string

	// IsEnabled 检查数据源是否可用（API Key 已配置等）
	IsEnabled() bool

	// Priority 返回数据源优先级（数字越小优先级越高）
	Priority() int

	// SupportedTypes 返回支持的媒体类型（"movie", "tvshow", "anime", "all"）
	SupportedTypes() []string

	// ScrapeMedia 为单个媒体刮削元数据
	// mode: "primary" 表示主数据源（覆盖写入），"supplement" 表示补充源（仅填充缺失字段）
	ScrapeMedia(media *model.Media, searchTitle string, year int, mode string) error

	// ScrapeSeries 为剧集合集刮削元数据
	ScrapeSeries(series *model.Series, searchTitle string, year int, mode string) error
}

// ==================== 动画内容识别 ====================

// 动画类型关键词（用于从 Genres、文件名、路径中识别动画内容）
var animeGenreKeywords = []string{
	"动画", "anime", "animation", "アニメ",
}

// 动画文件名关键词
var animeFileKeywords = []string{
	"[SubsPlease]", "[Erai-raws]", "[Moozzi2]", "[VCB-Studio]",
	"[CASO]", "[Nekomoe kissaten]", "[Lilith-Raws]",
	"BDRip", "BDRemux", // 常见于动画发布组
}

// 动画路径关键词
var animePathKeywords = []string{
	"动画", "动漫", "anime", "番剧", "新番", "OVA", "OAD",
}

// IsAnimeContent 判断媒体是否为动画内容
// 综合 TMDb genre、文件名分析、路径分析、用户标记四种方式
func IsAnimeContent(media *model.Media) bool {
	// 1. 通过 Genres 判断（TMDb 返回的类型信息）
	if media.Genres != "" {
		genresLower := strings.ToLower(media.Genres)
		for _, kw := range animeGenreKeywords {
			if strings.Contains(genresLower, strings.ToLower(kw)) {
				return true
			}
		}
	}

	// 2. 通过文件名判断（动画发布组特征）
	fileNameLower := strings.ToLower(media.FilePath)
	for _, kw := range animeFileKeywords {
		if strings.Contains(fileNameLower, strings.ToLower(kw)) {
			return true
		}
	}

	// 3. 通过路径判断（目录名包含动画关键词）
	for _, kw := range animePathKeywords {
		if strings.Contains(fileNameLower, strings.ToLower(kw)) {
			return true
		}
	}

	// 4. 通过 BangumiID 判断（已经关联了 Bangumi 条目）
	if media.BangumiID > 0 {
		return true
	}

	return false
}

// IsAnimeContentFromSeries 判断剧集合集是否为动画内容
func IsAnimeContentFromSeries(series *model.Series) bool {
	// 1. 通过 Genres 判断
	if series.Genres != "" {
		genresLower := strings.ToLower(series.Genres)
		for _, kw := range animeGenreKeywords {
			if strings.Contains(genresLower, strings.ToLower(kw)) {
				return true
			}
		}
	}

	// 2. 通过路径判断
	folderLower := strings.ToLower(series.FolderPath)
	for _, kw := range animePathKeywords {
		if strings.Contains(folderLower, strings.ToLower(kw)) {
			return true
		}
	}

	// 3. 通过 BangumiID 判断
	if series.BangumiID > 0 {
		return true
	}

	return false
}

// ==================== ProviderChain 数据源调度器 ====================

// ProviderChain 多数据源调度链
// 按优先级依次调用各数据源，实现 Fallback 和补充逻辑
type ProviderChain struct {
	providers []MetadataProvider
	logger    *zap.SugaredLogger
}

// NewProviderChain 创建数据源调度链
func NewProviderChain(logger *zap.SugaredLogger) *ProviderChain {
	return &ProviderChain{
		logger: logger,
	}
}

// Register 注册数据源（按优先级自动排序）
func (c *ProviderChain) Register(provider MetadataProvider) {
	c.providers = append(c.providers, provider)
	// 按优先级排序（插入排序，数量少）
	for i := len(c.providers) - 1; i > 0; i-- {
		if c.providers[i].Priority() < c.providers[i-1].Priority() {
			c.providers[i], c.providers[i-1] = c.providers[i-1], c.providers[i]
		}
	}
}

// ScrapeMedia 按优先级链式刮削媒体元数据
// 策略：
//  1. 主数据源（TMDb）先执行，获取基础信息
//  2. 如果是剧集内容，TheTVDB 补充每集详情
//  3. 如果是动画内容，Bangumi 补充动画专项信息
//  4. Fanart.tv 补充高质量图片
//  5. 如果所有数据源都失败，AI 兜底
func (c *ProviderChain) ScrapeMedia(media *model.Media, searchTitle string, year int) error {
	isAnime := IsAnimeContent(media)
	isTVShow := media.MediaType == "episode" || media.SeriesID != ""

	var primaryErr error
	primaryDone := false
	providerCallCount := 0

	for _, provider := range c.providers {
		if !provider.IsEnabled() {
			continue
		}

		// 检查数据源是否支持当前媒体类型
		if !c.isProviderApplicable(provider, media.MediaType, isAnime, isTVShow) {
			continue
		}

		// 数据源调用之间添加随机延迟（首次调用不延迟）
		if providerCallCount > 0 {
			randomDelay(2000, 4000)
		}
		providerCallCount++

		mode := "supplement"
		if !primaryDone {
			mode = "primary"
		}

		c.logger.Debugf("调用数据源 [%s] (mode=%s) 刮削: %s", provider.Name(), mode, searchTitle)

		if err := provider.ScrapeMedia(media, searchTitle, year, mode); err != nil {
			c.logger.Debugf("数据源 [%s] 刮削失败: %v", provider.Name(), err)
			if !primaryDone {
				primaryErr = err
			}
			continue
		}

		if !primaryDone {
			primaryDone = true
			primaryErr = nil
		}

		c.logger.Debugf("数据源 [%s] 刮削成功: %s", provider.Name(), searchTitle)

		// 检查信息是否已经足够完整，如果完整则跳过后续补充源
		if c.isMetadataComplete(media) {
			break
		}
	}

	if !primaryDone && primaryErr != nil {
		return primaryErr
	}

	return nil
}

// ScrapeSeries 按优先级链式刮削剧集合集元数据
func (c *ProviderChain) ScrapeSeries(series *model.Series, searchTitle string, year int) error {
	isAnime := IsAnimeContentFromSeries(series)

	var primaryErr error
	primaryDone := false
	providerCallCount := 0

	for _, provider := range c.providers {
		if !provider.IsEnabled() {
			continue
		}

		// 检查数据源是否支持剧集类型
		if !c.isProviderApplicable(provider, "tvshow", isAnime, true) {
			continue
		}

		// 数据源调用之间添加随机延迟（首次调用不延迟）
		if providerCallCount > 0 {
			randomDelay(2000, 4000)
		}
		providerCallCount++

		mode := "supplement"
		if !primaryDone {
			mode = "primary"
		}

		c.logger.Debugf("调用数据源 [%s] (mode=%s) 刮削剧集: %s", provider.Name(), mode, searchTitle)

		if err := provider.ScrapeSeries(series, searchTitle, year, mode); err != nil {
			c.logger.Debugf("数据源 [%s] 剧集刮削失败: %v", provider.Name(), err)
			if !primaryDone {
				primaryErr = err
			}
			continue
		}

		if !primaryDone {
			primaryDone = true
			primaryErr = nil
		}

		c.logger.Debugf("数据源 [%s] 剧集刮削成功: %s", provider.Name(), searchTitle)

		if c.isSeriesMetadataComplete(series) {
			break
		}
	}

	if !primaryDone && primaryErr != nil {
		return primaryErr
	}

	return nil
}

// isProviderApplicable 检查数据源是否适用于当前媒体类型
func (c *ProviderChain) isProviderApplicable(provider MetadataProvider, mediaType string, isAnime, isTVShow bool) bool {
	supported := provider.SupportedTypes()

	for _, t := range supported {
		switch t {
		case "all":
			return true
		case "anime":
			if isAnime {
				return true
			}
		case "tvshow":
			if isTVShow || mediaType == "episode" || mediaType == "tvshow" {
				return true
			}
		case "movie":
			if mediaType == "movie" {
				return true
			}
		case "image":
			// 图片增强源，始终适用
			return true
		}
	}

	return false
}

// isMetadataComplete 检查媒体元数据是否足够完整
func (c *ProviderChain) isMetadataComplete(media *model.Media) bool {
	score := 0
	if media.Overview != "" {
		score += 30
	}
	if media.PosterPath != "" {
		score += 25
	}
	if media.Rating > 0 {
		score += 15
	}
	if media.Genres != "" {
		score += 10
	}
	if media.Year > 0 {
		score += 10
	}
	if media.BackdropPath != "" {
		score += 10
	}
	// 80分以上认为足够完整
	return score >= 80
}

// isSeriesMetadataComplete 检查剧集合集元数据是否足够完整
func (c *ProviderChain) isSeriesMetadataComplete(series *model.Series) bool {
	score := 0
	if series.Overview != "" {
		score += 30
	}
	if series.PosterPath != "" {
		score += 25
	}
	if series.Rating > 0 {
		score += 15
	}
	if series.Genres != "" {
		score += 10
	}
	if series.Year > 0 {
		score += 10
	}
	if series.BackdropPath != "" {
		score += 10
	}
	return score >= 80
}

// GetProviders 获取所有已注册的数据源（用于状态展示）
func (c *ProviderChain) GetProviders() []ProviderInfo {
	var infos []ProviderInfo
	for _, p := range c.providers {
		infos = append(infos, ProviderInfo{
			Name:           p.Name(),
			Enabled:        p.IsEnabled(),
			Priority:       p.Priority(),
			SupportedTypes: p.SupportedTypes(),
		})
	}
	return infos
}

// ProviderInfo 数据源信息（用于 API 展示）
type ProviderInfo struct {
	Name           string   `json:"name"`
	Enabled        bool     `json:"enabled"`
	Priority       int      `json:"priority"`
	SupportedTypes []string `json:"supported_types"`
}

// ==================== Provider 适配器 ====================
// 以下为各现有数据源的 MetadataProvider 接口适配器

// TMDbProvider TMDb 数据源适配器
type TMDbProvider struct {
	metadata *MetadataService
}

func NewTMDbProvider(metadata *MetadataService) *TMDbProvider {
	return &TMDbProvider{metadata: metadata}
}

func (p *TMDbProvider) Name() string             { return "TMDb" }
func (p *TMDbProvider) IsEnabled() bool          { return p.metadata.cfg.Secrets.TMDbAPIKey != "" }
func (p *TMDbProvider) Priority() int            { return 10 }
func (p *TMDbProvider) SupportedTypes() []string { return []string{"all"} }

func (p *TMDbProvider) ScrapeMedia(media *model.Media, searchTitle string, year int, mode string) error {
	if media.MediaType == "movie" {
		return p.metadata.scrapeMovie(media, searchTitle, year)
	}
	return p.metadata.scrapeTV(media, searchTitle, year)
}

func (p *TMDbProvider) ScrapeSeries(series *model.Series, searchTitle string, year int, mode string) error {
	// 复用 MetadataService 中的 TMDb 搜索逻辑
	results, err := p.metadata.searchTMDb("tv", searchTitle, year)
	if err != nil || len(results) == 0 {
		if year > 0 {
			results, err = p.metadata.searchTMDb("tv", searchTitle, 0)
		}
		if err != nil || len(results) == 0 {
			return fmt.Errorf("TMDb 未找到匹配: %s", searchTitle)
		}
	}

	best := results[0]
	series.TMDbID = best.ID // 保存 TMDb ID，用于后续逐集刮削
	title := best.Name
	if title == "" {
		title = best.Title
	}
	origTitle := best.OriginalName
	if origTitle == "" {
		origTitle = best.OriginalTitle
	}

	if mode == "primary" {
		if title != "" {
			series.Title = title
		}
		series.OrigTitle = origTitle
		if best.Overview != "" {
			series.Overview = best.Overview
		}
		series.Rating = best.VoteAverage
		series.Genres = p.metadata.mapGenreIDs(best.GenreIDs)
	} else {
		// 补充模式：仅填充缺失字段
		if series.Title == "" && title != "" {
			series.Title = title
		}
		if series.OrigTitle == "" {
			series.OrigTitle = origTitle
		}
		if series.Overview == "" && best.Overview != "" {
			series.Overview = best.Overview
		}
		if series.Rating == 0 {
			series.Rating = best.VoteAverage
		}
		if series.Genres == "" {
			series.Genres = p.metadata.mapGenreIDs(best.GenreIDs)
		}
	}

	dateStr := best.FirstAirDate
	if dateStr == "" {
		dateStr = best.ReleaseDate
	}
	if len(dateStr) >= 4 && (mode == "primary" || series.Year == 0) {
		series.Year, _ = fmt.Sscan(dateStr[:4])
		fmt.Sscanf(dateStr[:4], "%d", &series.Year)
	}

	// 下载海报
	if best.PosterPath != "" && (mode == "primary" || series.PosterPath == "") {
		imageURL := fmt.Sprintf("%s/t/p/w500%s", p.metadata.getTMDbImageBase(), best.PosterPath)
		ext := ".jpg"
		cacheDir := fmt.Sprintf("%s/images/series/%s", p.metadata.cfg.Cache.CacheDir, series.ID)
		localPath := cacheDir + "/poster" + ext
		if err := mkdirAndDownload(p.metadata, imageURL, cacheDir, localPath); err == nil {
			series.PosterPath = localPath
		}
	}

	// 下载背景图
	if best.BackdropPath != "" && (mode == "primary" || series.BackdropPath == "") {
		imageURL := fmt.Sprintf("%s/t/p/w1280%s", p.metadata.getTMDbImageBase(), best.BackdropPath)
		ext := ".jpg"
		cacheDir := fmt.Sprintf("%s/images/series/%s", p.metadata.cfg.Cache.CacheDir, series.ID)
		localPath := cacheDir + "/backdrop" + ext
		if err := mkdirAndDownload(p.metadata, imageURL, cacheDir, localPath); err == nil {
			series.BackdropPath = localPath
		}
	}

	return nil
}

// mkdirAndDownload 创建目录并下载文件
func mkdirAndDownload(metadata *MetadataService, imageURL, dir, filePath string) error {
	os.MkdirAll(dir, 0755)
	return metadata.downloadToFile(imageURL, filePath)
}

// DoubanProvider 豆瓣数据源适配器
type DoubanProvider struct {
	douban *DoubanService
}

func NewDoubanProvider(douban *DoubanService) *DoubanProvider {
	return &DoubanProvider{douban: douban}
}

func (p *DoubanProvider) Name() string             { return "豆瓣" }
func (p *DoubanProvider) IsEnabled() bool          { return true } // 豆瓣不需要 API Key
func (p *DoubanProvider) Priority() int            { return 20 }
func (p *DoubanProvider) SupportedTypes() []string { return []string{"all"} }

func (p *DoubanProvider) ScrapeMedia(media *model.Media, searchTitle string, year int, mode string) error {
	if mode == "primary" {
		return p.douban.ScrapeMedia(media, searchTitle, year)
	}
	// 补充模式：仅在内存中应用，不写数据库
	p.douban.ApplyDoubanData(media, searchTitle, year)
	return nil
}

func (p *DoubanProvider) ScrapeSeries(series *model.Series, searchTitle string, year int, mode string) error {
	tempMedia := &model.Media{
		Title:    series.Title,
		FilePath: series.FolderPath + "/placeholder",
	}
	p.douban.ApplyDoubanData(tempMedia, searchTitle, year)

	// 将结果应用到 Series（仅补充缺失字段）
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

	return nil
}

// BangumiProvider Bangumi 数据源适配器（仅动画内容）
type BangumiProvider struct {
	bangumi *BangumiService
}

func NewBangumiProvider(bangumi *BangumiService) *BangumiProvider {
	return &BangumiProvider{bangumi: bangumi}
}

func (p *BangumiProvider) Name() string             { return "Bangumi" }
func (p *BangumiProvider) IsEnabled() bool          { return p.bangumi.IsEnabled() }
func (p *BangumiProvider) Priority() int            { return 30 }
func (p *BangumiProvider) SupportedTypes() []string { return []string{"anime"} } // 仅动画

func (p *BangumiProvider) ScrapeMedia(media *model.Media, searchTitle string, year int, mode string) error {
	if mode == "primary" {
		return p.bangumi.ScrapeMedia(media, searchTitle, year)
	}
	// 补充模式
	p.bangumi.ApplyBangumiData(media, searchTitle, year)
	return nil
}

func (p *BangumiProvider) ScrapeSeries(series *model.Series, searchTitle string, year int, mode string) error {
	tempMedia := &model.Media{
		Title:    series.Title,
		FilePath: series.FolderPath + "/placeholder",
	}
	p.bangumi.ApplyBangumiData(tempMedia, searchTitle, year)

	// 将结果应用到 Series
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

	return nil
}

// AIProvider AI 元数据增强适配器（最后的 Fallback）
type AIProvider struct {
	ai *AIService
}

func NewAIProvider(ai *AIService) *AIProvider {
	return &AIProvider{ai: ai}
}

func (p *AIProvider) Name() string { return "AI" }
func (p *AIProvider) IsEnabled() bool {
	return p.ai != nil && p.ai.IsMetadataEnhanceEnabled()
}
func (p *AIProvider) Priority() int            { return 100 } // 最低优先级
func (p *AIProvider) SupportedTypes() []string { return []string{"all"} }

func (p *AIProvider) ScrapeMedia(media *model.Media, searchTitle string, year int, mode string) error {
	return p.ai.EnrichMetadata(media, searchTitle)
}

func (p *AIProvider) ScrapeSeries(series *model.Series, searchTitle string, year int, mode string) error {
	return p.ai.EnrichSeriesMetadata(series, searchTitle)
}
