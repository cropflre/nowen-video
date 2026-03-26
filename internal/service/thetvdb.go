package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== TheTVDB API 数据结构 ====================

// TheTVDBAuthResponse 认证响应
type TheTVDBAuthResponse struct {
	Status string `json:"status"`
	Data   struct {
		Token string `json:"token"`
	} `json:"data"`
}

// TheTVDBSearchResult 搜索结果
type TheTVDBSearchResult struct {
	Status string          `json:"status"`
	Data   []TheTVDBSeries `json:"data"`
}

// TheTVDBSeries 剧集信息
type TheTVDBSeries struct {
	ID         int      `json:"id"`
	SeriesName string   `json:"seriesName"`
	Aliases    []string `json:"aliases"`
	Banner     string   `json:"banner"`
	Overview   string   `json:"overview"`
	FirstAired string   `json:"firstAired"`
	Network    string   `json:"network"`
	Status     string   `json:"status"`
	Rating     string   `json:"siteRating"`
	Genre      []string `json:"genre"`
	Runtime    string   `json:"runtime"`
	Language   string   `json:"language"`
	Poster     string   `json:"poster"`
	Fanart     string   `json:"fanart"`
	// V4 API 字段
	Name                 string   `json:"name"`
	OriginalName         string   `json:"originalName"`
	Image                string   `json:"image"`
	Year                 string   `json:"year"`
	OriginalCountry      string   `json:"originalCountry"`
	OriginalLanguage     string   `json:"originalLanguage"`
	PrimaryLanguage      string   `json:"primaryLanguage"`
	TVDbID               string   `json:"tvdb_id"` // 字符串形式的 ID
	ObjectID             string   `json:"objectID"`
	Country              string   `json:"country"`
	Slug                 string   `json:"slug"`
	OverviewTranslations []string `json:"overviewTranslations"`
	NameTranslations     []string `json:"nameTranslations"`
}

// TheTVDBEpisode 单集信息
type TheTVDBEpisode struct {
	ID              int     `json:"id"`
	AiredSeason     int     `json:"airedSeason"`
	AiredEpisodeNum int     `json:"airedEpisodeNumber"`
	EpisodeName     string  `json:"episodeName"`
	Overview        string  `json:"overview"`
	FirstAired      string  `json:"firstAired"`
	Filename        string  `json:"filename"` // 剧照路径
	Rating          float64 `json:"siteRating"`
	// V4 API 字段
	Name         string `json:"name"`
	Number       int    `json:"number"`
	SeasonNumber int    `json:"seasonNumber"`
	Image        string `json:"image"`
}

// TheTVDBEpisodesResponse 剧集列表响应
type TheTVDBEpisodesResponse struct {
	Status string           `json:"status"`
	Data   []TheTVDBEpisode `json:"data"`
	Links  struct {
		First int `json:"first"`
		Last  int `json:"last"`
		Next  int `json:"next"`
		Prev  int `json:"prev"`
	} `json:"links"`
}

// TheTVDBSeriesDetailResponse 剧集详情响应（V4 API）
type TheTVDBSeriesDetailResponse struct {
	Status string        `json:"status"`
	Data   TheTVDBSeries `json:"data"`
}

// ==================== TheTVDBService ====================

// TheTVDBService TheTVDB 元数据刮削服务
// 专门用于电视剧集的详细信息补充，提供精确到每集的标题、简介和元数据
type TheTVDBService struct {
	mediaRepo  *repository.MediaRepo
	seriesRepo *repository.SeriesRepo
	cfg        *config.Config
	logger     *zap.SugaredLogger
	client     *http.Client

	// JWT Token 管理
	token       string
	tokenMu     sync.RWMutex
	tokenExpiry time.Time
}

// NewTheTVDBService 创建 TheTVDB 刮削服务
func NewTheTVDBService(mediaRepo *repository.MediaRepo, seriesRepo *repository.SeriesRepo, cfg *config.Config, logger *zap.SugaredLogger) *TheTVDBService {
	return &TheTVDBService{
		mediaRepo:  mediaRepo,
		seriesRepo: seriesRepo,
		cfg:        cfg,
		logger:     logger,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// IsEnabled 检查 TheTVDB 数据源是否可用
func (s *TheTVDBService) IsEnabled() bool {
	return s.cfg.Secrets.TheTVDBAPIKey != ""
}

// ==================== 认证 ====================

// authenticate 获取或刷新 JWT Token
func (s *TheTVDBService) authenticate() error {
	s.tokenMu.RLock()
	if s.token != "" && time.Now().Before(s.tokenExpiry) {
		s.tokenMu.RUnlock()
		return nil
	}
	s.tokenMu.RUnlock()

	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	// 双重检查
	if s.token != "" && time.Now().Before(s.tokenExpiry) {
		return nil
	}

	apiKey := s.cfg.Secrets.TheTVDBAPIKey
	if apiKey == "" {
		return fmt.Errorf("TheTVDB API Key 未配置")
	}

	// V4 API 认证
	authURL := "https://api4.thetvdb.com/v4/login"
	authBody := fmt.Sprintf(`{"apikey":"%s"}`, apiKey)

	req, err := http.NewRequest("POST", authURL, strings.NewReader(authBody))
	if err != nil {
		return fmt.Errorf("创建认证请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("TheTVDB 认证请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TheTVDB 认证失败 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var authResp TheTVDBAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("解析认证响应失败: %w", err)
	}

	s.token = authResp.Data.Token
	// Token 有效期 24 小时，提前 1 小时刷新
	s.tokenExpiry = time.Now().Add(23 * time.Hour)

	s.logger.Debugf("TheTVDB 认证成功，Token 有效期至 %s", s.tokenExpiry.Format(time.RFC3339))
	return nil
}

// doRequest 执行带认证的 HTTP 请求
func (s *TheTVDBService) doRequest(method, url string) (*http.Response, error) {
	if err := s.authenticate(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	s.tokenMu.RLock()
	req.Header.Set("Authorization", "Bearer "+s.token)
	s.tokenMu.RUnlock()
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TheTVDB 请求失败: %w", err)
	}

	// Token 过期，重新认证并重试
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		s.tokenMu.Lock()
		s.token = ""
		s.tokenMu.Unlock()

		if err := s.authenticate(); err != nil {
			return nil, err
		}

		req2, _ := http.NewRequest(method, url, nil)
		s.tokenMu.RLock()
		req2.Header.Set("Authorization", "Bearer "+s.token)
		s.tokenMu.RUnlock()
		req2.Header.Set("Accept", "application/json")

		return s.client.Do(req2)
	}

	return resp, nil
}

// ==================== 核心方法 ====================

// SearchSeries 搜索剧集
func (s *TheTVDBService) SearchSeries(query string, year int) ([]TheTVDBSeries, error) {
	searchURL := fmt.Sprintf("https://api4.thetvdb.com/v4/search?query=%s&type=series",
		strings.ReplaceAll(query, " ", "%20"))
	if year > 0 {
		searchURL += fmt.Sprintf("&year=%d", year)
	}

	resp, err := s.doRequest("GET", searchURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TheTVDB 搜索返回状态码: %d", resp.StatusCode)
	}

	var result TheTVDBSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 TheTVDB 搜索响应失败: %w", err)
	}

	return result.Data, nil
}

// GetSeriesDetail 获取剧集详情
func (s *TheTVDBService) GetSeriesDetail(seriesID int) (*TheTVDBSeries, error) {
	detailURL := fmt.Sprintf("https://api4.thetvdb.com/v4/series/%d/extended", seriesID)

	resp, err := s.doRequest("GET", detailURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TheTVDB 详情返回状态码: %d", resp.StatusCode)
	}

	var result TheTVDBSeriesDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 TheTVDB 详情失败: %w", err)
	}

	return &result.Data, nil
}

// GetEpisodes 获取剧集的所有集信息
func (s *TheTVDBService) GetEpisodes(seriesID int, seasonNum int) ([]TheTVDBEpisode, error) {
	episodesURL := fmt.Sprintf("https://api4.thetvdb.com/v4/series/%d/episodes/default?season=%d",
		seriesID, seasonNum)

	resp, err := s.doRequest("GET", episodesURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TheTVDB 剧集列表返回状态码: %d", resp.StatusCode)
	}

	var result TheTVDBEpisodesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 TheTVDB 剧集列表失败: %w", err)
	}

	return result.Data, nil
}

// ==================== 刮削方法 ====================

// ScrapeSeriesMetadata 为剧集合集刮削 TheTVDB 元数据
func (s *TheTVDBService) ScrapeSeriesMetadata(series *model.Series, searchTitle string, year int) error {
	s.logger.Debugf("TheTVDB 刮削剧集: %s (year=%d)", searchTitle, year)

	results, err := s.SearchSeries(searchTitle, year)
	if err != nil {
		return fmt.Errorf("TheTVDB 搜索失败: %w", err)
	}

	if len(results) == 0 {
		// 不带年份重试
		if year > 0 {
			results, err = s.SearchSeries(searchTitle, 0)
			if err != nil {
				return fmt.Errorf("TheTVDB 搜索失败: %w", err)
			}
		}
		if len(results) == 0 {
			return fmt.Errorf("TheTVDB 未找到匹配: %s", searchTitle)
		}
	}

	best := results[0]

	// 应用基础信息（仅补充缺失字段）
	s.applySeriesResult(series, &best)

	return nil
}

// ScrapeEpisodeMetadata 为单集补充 TheTVDB 元数据（标题、简介等）
func (s *TheTVDBService) ScrapeEpisodeMetadata(media *model.Media, searchTitle string, year int) error {
	if media.SeriesID == "" {
		return fmt.Errorf("非剧集类型，跳过 TheTVDB")
	}

	s.logger.Debugf("TheTVDB 刮削单集: %s S%02dE%02d", searchTitle, media.SeasonNum, media.EpisodeNum)

	// 搜索剧集
	results, err := s.SearchSeries(searchTitle, year)
	if err != nil || len(results) == 0 {
		if year > 0 {
			results, err = s.SearchSeries(searchTitle, 0)
		}
		if err != nil || len(results) == 0 {
			return fmt.Errorf("TheTVDB 未找到匹配: %s", searchTitle)
		}
	}

	best := results[0]
	tvdbID := best.ID
	if tvdbID == 0 {
		// V4 API 搜索结果中 ID 可能在 tvdb_id 字段
		if best.TVDbID != "" {
			tvdbID, _ = strconv.Atoi(best.TVDbID)
		}
	}

	if tvdbID == 0 {
		return fmt.Errorf("无法获取 TheTVDB ID")
	}

	// 获取指定季的剧集列表
	seasonNum := media.SeasonNum
	if seasonNum == 0 {
		seasonNum = 1
	}

	episodes, err := s.GetEpisodes(tvdbID, seasonNum)
	if err != nil {
		return fmt.Errorf("获取剧集列表失败: %w", err)
	}

	// 查找匹配的集
	for _, ep := range episodes {
		epNum := ep.AiredEpisodeNum
		if epNum == 0 {
			epNum = ep.Number
		}
		if epNum == media.EpisodeNum {
			// 补充单集信息
			epName := ep.EpisodeName
			if epName == "" {
				epName = ep.Name
			}
			if media.EpisodeTitle == "" && epName != "" {
				media.EpisodeTitle = epName
			}
			if media.Overview == "" && ep.Overview != "" {
				media.Overview = ep.Overview
			}
			s.logger.Debugf("TheTVDB 单集匹配成功: S%02dE%02d - %s", seasonNum, epNum, epName)
			return nil
		}
	}

	return fmt.Errorf("TheTVDB 未找到匹配的集: S%02dE%02d", seasonNum, media.EpisodeNum)
}

// applySeriesResult 将 TheTVDB 搜索结果应用到剧集合集（仅补充缺失字段）
func (s *TheTVDBService) applySeriesResult(series *model.Series, result *TheTVDBSeries) {
	// 补充原始标题
	origName := result.OriginalName
	if origName == "" {
		origName = result.SeriesName
		if origName == "" {
			origName = result.Name
		}
	}
	if series.OrigTitle == "" && origName != "" {
		series.OrigTitle = origName
	}

	// 补充简介
	if series.Overview == "" && result.Overview != "" {
		series.Overview = result.Overview
	}

	// 补充评分
	if series.Rating == 0 && result.Rating != "" {
		if rating, err := strconv.ParseFloat(result.Rating, 64); err == nil {
			series.Rating = rating
		}
	}

	// 补充年份
	yearStr := result.Year
	if yearStr == "" && result.FirstAired != "" && len(result.FirstAired) >= 4 {
		yearStr = result.FirstAired[:4]
	}
	if series.Year == 0 && yearStr != "" {
		series.Year, _ = strconv.Atoi(yearStr)
	}

	// 补充类型
	if series.Genres == "" && len(result.Genre) > 0 {
		series.Genres = strings.Join(result.Genre, ",")
	}

	// 补充国家
	country := result.OriginalCountry
	if country == "" {
		country = result.Country
	}
	if series.Country == "" && country != "" {
		series.Country = country
	}

	// 补充语言
	lang := result.OriginalLanguage
	if lang == "" {
		lang = result.PrimaryLanguage
		if lang == "" {
			lang = result.Language
		}
	}
	if series.Language == "" && lang != "" {
		series.Language = lang
	}

	// 补充制作公司
	if series.Studio == "" && result.Network != "" {
		series.Studio = result.Network
	}

	// 补充海报
	imageURL := result.Image
	if imageURL == "" {
		imageURL = result.Poster
	}
	if series.PosterPath == "" && imageURL != "" {
		localPath, err := s.downloadTheTVDBImage(series.ID, imageURL, "poster")
		if err == nil {
			series.PosterPath = localPath
		}
	}

	// 补充背景图
	if series.BackdropPath == "" && result.Fanart != "" {
		localPath, err := s.downloadTheTVDBImage(series.ID, result.Fanart, "backdrop")
		if err == nil {
			series.BackdropPath = localPath
		}
	}
}

// ==================== 图片下载 ====================

// downloadTheTVDBImage 下载 TheTVDB 图片到本地
func (s *TheTVDBService) downloadTheTVDBImage(entityID, imageURL, imageType string) (string, error) {
	if imageURL == "" {
		return "", fmt.Errorf("图片URL为空")
	}

	// TheTVDB V4 API 图片 URL 可能是相对路径
	if !strings.HasPrefix(imageURL, "http") {
		imageURL = "https://artworks.thetvdb.com" + imageURL
	}

	resp, err := s.client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("图片请求失败: %d", resp.StatusCode)
	}

	// 确定扩展名
	ext := ".jpg"
	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "png"):
		ext = ".png"
	case strings.Contains(ct, "webp"):
		ext = ".webp"
	}

	cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", "series", entityID)
	os.MkdirAll(cacheDir, 0755)
	localPath := filepath.Join(cacheDir, fmt.Sprintf("%s-thetvdb%s", imageType, ext))

	file, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("创建图片文件失败: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("保存图片失败: %w", err)
	}

	s.logger.Debugf("已下载 TheTVDB 图片: %s", localPath)
	return localPath, nil
}

// ==================== MetadataProvider 接口实现 ====================

// TheTVDBProvider TheTVDB 数据源适配器（仅剧集内容）
type TheTVDBProvider struct {
	thetvdb *TheTVDBService
}

func NewTheTVDBProvider(thetvdb *TheTVDBService) *TheTVDBProvider {
	return &TheTVDBProvider{thetvdb: thetvdb}
}

func (p *TheTVDBProvider) Name() string             { return "TheTVDB" }
func (p *TheTVDBProvider) IsEnabled() bool          { return p.thetvdb.IsEnabled() }
func (p *TheTVDBProvider) Priority() int            { return 25 } // 在豆瓣之后，Bangumi 之前
func (p *TheTVDBProvider) SupportedTypes() []string { return []string{"tvshow"} }

func (p *TheTVDBProvider) ScrapeMedia(media *model.Media, searchTitle string, year int, mode string) error {
	return p.thetvdb.ScrapeEpisodeMetadata(media, searchTitle, year)
}

func (p *TheTVDBProvider) ScrapeSeries(series *model.Series, searchTitle string, year int, mode string) error {
	return p.thetvdb.ScrapeSeriesMetadata(series, searchTitle, year)
}
