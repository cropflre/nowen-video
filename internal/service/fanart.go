package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== Fanart.tv API 数据结构 ====================

// FanartImage 单张图片信息
type FanartImage struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Lang  string `json:"lang"`
	Likes string `json:"likes"`
	// 部分图片类型有额外字段
	Disc     string `json:"disc,omitempty"`
	DiscType string `json:"disc_type,omitempty"`
	Season   string `json:"season,omitempty"`
}

// FanartMovieImages 电影图片集合
type FanartMovieImages struct {
	// 电影专属
	MoviePoster     []FanartImage `json:"movieposter"`
	MovieBackground []FanartImage `json:"moviebackground"`
	MovieLogo       []FanartImage `json:"hdmovielogo"`
	MovieClearArt   []FanartImage `json:"hdmovieclearart"`
	MovieDisc       []FanartImage `json:"moviedisc"`
	MovieThumb      []FanartImage `json:"moviethumb"`
	MovieBanner     []FanartImage `json:"moviebanner"`
}

// FanartTVImages 剧集图片集合
type FanartTVImages struct {
	// 剧集专属
	TVPoster     []FanartImage `json:"tvposter"`
	TVBackground []FanartImage `json:"showbackground"`
	TVLogo       []FanartImage `json:"hdtvlogo"`
	TVClearArt   []FanartImage `json:"hdclearart"`
	TVThumb      []FanartImage `json:"tvthumb"`
	TVBanner     []FanartImage `json:"tvbanner"`
	SeasonPoster []FanartImage `json:"seasonposter"`
	SeasonThumb  []FanartImage `json:"seasonthumb"`
	SeasonBanner []FanartImage `json:"seasonbanner"`
	CharacterArt []FanartImage `json:"characterart"`
}

// ==================== FanartService ====================

// FanartService Fanart.tv 图片增强服务
// 专门提供高质量的 ClearLogo、背景图、光碟封面等图片资源
type FanartService struct {
	mediaRepo  *repository.MediaRepo
	seriesRepo *repository.SeriesRepo
	cfg        *config.Config
	logger     *zap.SugaredLogger
	client     *http.Client
}

// NewFanartService 创建 Fanart.tv 服务
func NewFanartService(mediaRepo *repository.MediaRepo, seriesRepo *repository.SeriesRepo, cfg *config.Config, logger *zap.SugaredLogger) *FanartService {
	return &FanartService{
		mediaRepo:  mediaRepo,
		seriesRepo: seriesRepo,
		cfg:        cfg,
		logger:     logger,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// IsEnabled 检查 Fanart.tv 数据源是否可用
func (s *FanartService) IsEnabled() bool {
	return s.cfg.Secrets.FanartTVAPIKey != ""
}

// ==================== 核心方法 ====================

// GetMovieImages 获取电影的所有 Fanart.tv 图片
// tmdbID: TMDb 电影 ID（Fanart.tv 使用 TMDb ID 作为查询键）
func (s *FanartService) GetMovieImages(tmdbID int) (*FanartMovieImages, error) {
	apiURL := fmt.Sprintf("https://webservice.fanart.tv/v3/movies/%d?api_key=%s",
		tmdbID, s.cfg.Secrets.FanartTVAPIKey)

	resp, err := s.client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("Fanart.tv 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("Fanart.tv 未找到该电影: TMDb ID %d", tmdbID)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Fanart.tv 返回状态码: %d", resp.StatusCode)
	}

	var images FanartMovieImages
	if err := json.NewDecoder(resp.Body).Decode(&images); err != nil {
		return nil, fmt.Errorf("解析 Fanart.tv 响应失败: %w", err)
	}

	return &images, nil
}

// GetTVImages 获取剧集的所有 Fanart.tv 图片
// tvdbID: TheTVDB 剧集 ID（Fanart.tv 使用 TVDB ID 作为查询键）
func (s *FanartService) GetTVImages(tvdbID int) (*FanartTVImages, error) {
	apiURL := fmt.Sprintf("https://webservice.fanart.tv/v3/tv/%d?api_key=%s",
		tvdbID, s.cfg.Secrets.FanartTVAPIKey)

	resp, err := s.client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("Fanart.tv 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("Fanart.tv 未找到该剧集: TVDB ID %d", tvdbID)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Fanart.tv 返回状态码: %d", resp.StatusCode)
	}

	var images FanartTVImages
	if err := json.NewDecoder(resp.Body).Decode(&images); err != nil {
		return nil, fmt.Errorf("解析 Fanart.tv 响应失败: %w", err)
	}

	return &images, nil
}

// ==================== 刮削方法 ====================

// EnhanceMovieImages 为电影增强图片（ClearLogo、高质量背景图等）
func (s *FanartService) EnhanceMovieImages(media *model.Media) error {
	if media.TMDbID == 0 {
		return fmt.Errorf("缺少 TMDb ID，无法查询 Fanart.tv")
	}

	s.logger.Debugf("Fanart.tv 图片增强: %s (TMDb ID: %d)", media.Title, media.TMDbID)

	images, err := s.GetMovieImages(media.TMDbID)
	if err != nil {
		return err
	}

	updated := false

	// 补充高质量海报（如果当前没有或需要更好的）
	if media.PosterPath == "" && len(images.MoviePoster) > 0 {
		bestPoster := s.selectBestImage(images.MoviePoster, "zh", "en")
		if bestPoster != nil {
			localPath, err := s.downloadFanartImage(media.ID, bestPoster.URL, "poster")
			if err == nil {
				media.PosterPath = localPath
				updated = true
			}
		}
	}

	// 补充高质量背景图
	if media.BackdropPath == "" && len(images.MovieBackground) > 0 {
		bestBg := s.selectBestImage(images.MovieBackground, "", "")
		if bestBg != nil {
			localPath, err := s.downloadFanartImage(media.ID, bestBg.URL, "backdrop")
			if err == nil {
				media.BackdropPath = localPath
				updated = true
			}
		}
	}

	if updated {
		return s.mediaRepo.Update(media)
	}

	return nil
}

// EnhanceTVImages 为剧集增强图片
func (s *FanartService) EnhanceTVImages(series *model.Series, tvdbID int) error {
	if tvdbID == 0 {
		return fmt.Errorf("缺少 TVDB ID，无法查询 Fanart.tv")
	}

	s.logger.Debugf("Fanart.tv 剧集图片增强: %s (TVDB ID: %d)", series.Title, tvdbID)

	images, err := s.GetTVImages(tvdbID)
	if err != nil {
		return err
	}

	updated := false

	// 补充海报
	if series.PosterPath == "" && len(images.TVPoster) > 0 {
		bestPoster := s.selectBestImage(images.TVPoster, "zh", "en")
		if bestPoster != nil {
			localPath, err := s.downloadFanartImageForSeries(series.ID, bestPoster.URL, "poster")
			if err == nil {
				series.PosterPath = localPath
				updated = true
			}
		}
	}

	// 补充背景图
	if series.BackdropPath == "" && len(images.TVBackground) > 0 {
		bestBg := s.selectBestImage(images.TVBackground, "", "")
		if bestBg != nil {
			localPath, err := s.downloadFanartImageForSeries(series.ID, bestBg.URL, "backdrop")
			if err == nil {
				series.BackdropPath = localPath
				updated = true
			}
		}
	}

	if updated {
		return s.seriesRepo.Update(series)
	}

	return nil
}

// ==================== 辅助方法 ====================

// selectBestImage 从图片列表中选择最佳图片
// 优先选择指定语言的图片，然后按 likes 排序
func (s *FanartService) selectBestImage(images []FanartImage, preferLang, fallbackLang string) *FanartImage {
	if len(images) == 0 {
		return nil
	}

	// 优先选择首选语言
	if preferLang != "" {
		for i := range images {
			if images[i].Lang == preferLang {
				return &images[i]
			}
		}
	}

	// 其次选择备选语言
	if fallbackLang != "" {
		for i := range images {
			if images[i].Lang == fallbackLang {
				return &images[i]
			}
		}
	}

	// 选择无语言标记的（通常是通用图片）
	for i := range images {
		if images[i].Lang == "" || images[i].Lang == "00" {
			return &images[i]
		}
	}

	// 返回第一张
	return &images[0]
}

// downloadFanartImage 下载 Fanart.tv 图片到本地（媒体）
func (s *FanartService) downloadFanartImage(entityID, imageURL, imageType string) (string, error) {
	return s.downloadImage(entityID, imageURL, imageType, "media")
}

// downloadFanartImageForSeries 下载 Fanart.tv 图片到本地（剧集合集）
func (s *FanartService) downloadFanartImageForSeries(entityID, imageURL, imageType string) (string, error) {
	return s.downloadImage(entityID, imageURL, imageType, "series")
}

// downloadImage 通用图片下载方法
func (s *FanartService) downloadImage(entityID, imageURL, imageType, entityType string) (string, error) {
	if imageURL == "" {
		return "", fmt.Errorf("图片URL为空")
	}

	resp, err := s.client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("下载 Fanart.tv 图片失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Fanart.tv 图片请求失败: %d", resp.StatusCode)
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

	var cacheDir string
	if entityType == "series" {
		cacheDir = filepath.Join(s.cfg.Cache.CacheDir, "images", "series", entityID)
	} else {
		cacheDir = filepath.Join(s.cfg.Cache.CacheDir, "images", entityID)
	}
	os.MkdirAll(cacheDir, 0755)

	localPath := filepath.Join(cacheDir, fmt.Sprintf("%s-fanart%s", imageType, ext))

	file, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("创建图片文件失败: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("保存图片失败: %w", err)
	}

	s.logger.Debugf("已下载 Fanart.tv 图片: %s", localPath)
	return localPath, nil
}

// ==================== MetadataProvider 接口实现 ====================

// FanartProvider Fanart.tv 数据源适配器（图片增强，适用于所有类型）
type FanartProvider struct {
	fanart *FanartService
}

func NewFanartProvider(fanart *FanartService) *FanartProvider {
	return &FanartProvider{fanart: fanart}
}

func (p *FanartProvider) Name() string             { return "Fanart.tv" }
func (p *FanartProvider) IsEnabled() bool          { return p.fanart.IsEnabled() }
func (p *FanartProvider) Priority() int            { return 50 } // 在所有元数据源之后，AI 之前
func (p *FanartProvider) SupportedTypes() []string { return []string{"image"} }

func (p *FanartProvider) ScrapeMedia(media *model.Media, searchTitle string, year int, mode string) error {
	// Fanart.tv 需要 TMDb ID，如果没有则跳过
	if media.TMDbID == 0 {
		return fmt.Errorf("缺少 TMDb ID，跳过 Fanart.tv 图片增强")
	}
	return p.fanart.EnhanceMovieImages(media)
}

func (p *FanartProvider) ScrapeSeries(series *model.Series, searchTitle string, year int, mode string) error {
	// Fanart.tv 剧集图片需要 TVDB ID
	// 如果没有 TVDB ID，尝试使用 TMDb ID（Fanart.tv 也支持部分 TMDb ID 查询）
	if series.TMDbID == 0 {
		return fmt.Errorf("缺少 TMDb/TVDB ID，跳过 Fanart.tv 图片增强")
	}
	return p.fanart.EnhanceTVImages(series, series.TMDbID)
}
