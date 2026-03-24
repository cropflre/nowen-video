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

// ==================== 豆瓣 API 数据结构 ====================

// DoubanSearchResult 豆瓣搜索结果（网页解析）
type DoubanSearchResult struct {
	ID       string  `json:"id"`       // 豆瓣ID
	Title    string  `json:"title"`    // 标题
	Year     int     `json:"year"`     // 年份
	Rating   float64 `json:"rating"`   // 评分
	Cover    string  `json:"cover"`    // 封面URL
	Overview string  `json:"overview"` // 简介
	Genres   string  `json:"genres"`   // 类型
}

// DoubanSubjectDetail 豆瓣条目详情
type DoubanSubjectDetail struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	OrigTitle string         `json:"original_title"`
	Year      string         `json:"year"`
	Rating    DoubanRating   `json:"rating"`
	Cover     string         `json:"pic"`
	Overview  string         `json:"intro"`
	Genres    []string       `json:"genres"`
	Directors []DoubanPerson `json:"directors"`
	Actors    []DoubanPerson `json:"actors"`
}

// DoubanRating 豆瓣评分
type DoubanRating struct {
	Average float64 `json:"average"`
	Stars   string  `json:"stars"`
}

// DoubanPerson 豆瓣人物
type DoubanPerson struct {
	Name string `json:"name"`
}

// DoubanService 豆瓣元数据刮削服务
type DoubanService struct {
	mediaRepo *repository.MediaRepo
	cfg       *config.Config
	logger    *zap.SugaredLogger
	client    *http.Client
}

// NewDoubanService 创建豆瓣刮削服务
func NewDoubanService(mediaRepo *repository.MediaRepo, cfg *config.Config, logger *zap.SugaredLogger) *DoubanService {
	return &DoubanService{
		mediaRepo: mediaRepo,
		cfg:       cfg,
		logger:    logger,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ==================== 核心方法 ====================

// ScrapeMedia 为单个媒体刮削豆瓣元数据（作为TMDb的补充）
func (s *DoubanService) ScrapeMedia(media *model.Media, searchTitle string, year int) error {
	s.logger.Debugf("豆瓣刮削: %s (year=%d)", searchTitle, year)

	// 搜索豆瓣
	results, err := s.searchDouban(searchTitle, year)
	if err != nil {
		return fmt.Errorf("豆瓣搜索失败: %w", err)
	}

	if len(results) == 0 {
		// 不带年份重试
		if year > 0 {
			results, err = s.searchDouban(searchTitle, 0)
			if err != nil {
				return fmt.Errorf("豆瓣搜索失败: %w", err)
			}
		}
		if len(results) == 0 {
			return fmt.Errorf("豆瓣未找到匹配: %s", searchTitle)
		}
	}

	best := results[0]

	// 应用豆瓣数据（仅补充缺失的字段）
	s.applyDoubanResult(media, &best)

	return s.mediaRepo.Update(media)
}

// applyDoubanResult 将豆瓣结果应用到媒体（仅补充缺失字段）
func (s *DoubanService) applyDoubanResult(media *model.Media, result *DoubanSearchResult) {
	// 仅补充缺失的评分（豆瓣评分作为参考）
	if media.Rating == 0 && result.Rating > 0 {
		// 豆瓣满分10分，直接使用
		media.Rating = result.Rating
	}

	// 补充缺失的简介
	if media.Overview == "" && result.Overview != "" {
		media.Overview = result.Overview
	}

	// 补充缺失的年份
	if media.Year == 0 && result.Year > 0 {
		media.Year = result.Year
	}

	// 补充缺失的类型
	if media.Genres == "" && result.Genres != "" {
		media.Genres = result.Genres
	}

	// 补充缺失的海报
	if media.PosterPath == "" && result.Cover != "" {
		localPath, err := s.downloadDoubanCover(media, result.Cover)
		if err == nil {
			media.PosterPath = localPath
		}
	}
}

// ==================== 豆瓣 API 调用 ====================

// searchDouban 通过豆瓣搜索API获取结果
// 使用豆瓣 frodo API（移动端接口，较稳定）
func (s *DoubanService) searchDouban(query string, year int) ([]DoubanSearchResult, error) {
	// 使用豆瓣公开的搜索建议API
	params := url.Values{}
	params.Set("q", query)
	params.Set("count", "5")

	apiURL := fmt.Sprintf("https://movie.douban.com/j/subject_suggest?%s", params.Encode())

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	// 设置请求头，模拟浏览器请求
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://movie.douban.com/")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("豆瓣请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("豆瓣返回状态码: %d", resp.StatusCode)
	}

	// 豆瓣搜索建议API返回格式
	var suggestions []struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Year    string `json:"year"`
		SubType string `json:"type"` // movie
		Img     string `json:"img"`
		Episode string `json:"episode"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&suggestions); err != nil {
		return nil, fmt.Errorf("解析豆瓣响应失败: %w", err)
	}

	var results []DoubanSearchResult
	for _, s := range suggestions {
		// 仅保留电影/电视类型
		if s.SubType != "movie" {
			continue
		}

		yearInt, _ := strconv.Atoi(s.Year)

		// 年份过滤
		if year > 0 && yearInt > 0 && abs(yearInt-year) > 1 {
			continue
		}

		results = append(results, DoubanSearchResult{
			ID:    s.ID,
			Title: s.Title,
			Year:  yearInt,
			Cover: s.Img,
		})
	}

	// 获取第一个结果的详细信息
	if len(results) > 0 && results[0].ID != "" {
		detail, err := s.getSubjectDetail(results[0].ID)
		if err == nil {
			results[0].Rating = detail.Rating.Average
			results[0].Overview = detail.Overview
			if len(detail.Genres) > 0 {
				results[0].Genres = strings.Join(detail.Genres, ",")
			}
			if detail.Cover != "" {
				results[0].Cover = detail.Cover
			}
		}
	}

	return results, nil
}

// getSubjectDetail 获取豆瓣条目详情
func (s *DoubanService) getSubjectDetail(doubanID string) (*DoubanSubjectDetail, error) {
	// 使用网页版解析方式获取详情（因为官方API需要apikey）
	apiURL := fmt.Sprintf("https://movie.douban.com/j/subject_abstract?subject_id=%s", doubanID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://movie.douban.com/subject/%s/", doubanID))

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("豆瓣详情请求失败: %d", resp.StatusCode)
	}

	var abstractResp struct {
		Subject struct {
			Rating struct {
				Count int     `json:"count"`
				Max   int     `json:"max"`
				Value float64 `json:"value"`
			} `json:"rating"`
			Title  string   `json:"title"`
			Intro  string   `json:"short_info"`
			Genres []string `json:"genres"`
			PicURL string   `json:"pic_url"`
		} `json:"subject"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&abstractResp); err != nil {
		return nil, fmt.Errorf("解析豆瓣详情失败: %w", err)
	}

	subject := abstractResp.Subject

	detail := &DoubanSubjectDetail{
		ID:    doubanID,
		Title: subject.Title,
		Rating: DoubanRating{
			Average: subject.Rating.Value,
		},
		Cover:    subject.PicURL,
		Overview: subject.Intro,
		Genres:   subject.Genres,
	}

	return detail, nil
}

// ==================== 辅助方法 ====================

// downloadDoubanCover 下载豆瓣封面图到本地
func (s *DoubanService) downloadDoubanCover(media *model.Media, coverURL string) (string, error) {
	if coverURL == "" {
		return "", fmt.Errorf("封面URL为空")
	}

	req, err := http.NewRequest("GET", coverURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://movie.douban.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载封面失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("封面请求失败: %d", resp.StatusCode)
	}

	// 保存到媒体文件同目录
	mediaDir := filepath.Dir(media.FilePath)
	baseName := strings.TrimSuffix(filepath.Base(media.FilePath), filepath.Ext(media.FilePath))

	// 从URL推断扩展名
	ext := ".jpg"
	urlPath := strings.Split(coverURL, "?")[0]
	if e := filepath.Ext(urlPath); e != "" {
		ext = e
	}

	localPath := filepath.Join(mediaDir, fmt.Sprintf("%s-poster-douban%s", baseName, ext))

	file, err := os.Create(localPath)
	if err != nil {
		// 如果媒体目录不可写，保存到缓存目录
		cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", media.ID)
		os.MkdirAll(cacheDir, 0755)
		localPath = filepath.Join(cacheDir, fmt.Sprintf("poster-douban%s", ext))
		file, err = os.Create(localPath)
		if err != nil {
			return "", fmt.Errorf("创建封面文件失败: %w", err)
		}
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("保存封面失败: %w", err)
	}

	s.logger.Debugf("已下载豆瓣封面: %s", localPath)
	return localPath, nil
}

// parseDoubanTitle 从文件标题中提取搜索关键词（与metadata.go相同逻辑）
func (s *DoubanService) parseDoubanTitle(title string) (string, int) {
	yearRegex := regexp.MustCompile(`[\s\.(]\s*((?:19|20)\d{2})\s*[\s\).]?`)
	matches := yearRegex.FindStringSubmatch(title)

	var year int
	cleanTitle := title

	if len(matches) >= 2 {
		year, _ = strconv.Atoi(matches[1])
		cleanTitle = yearRegex.ReplaceAllString(title, " ")
	}

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

	cleanTitle = regexp.MustCompile(`\s+`).ReplaceAllString(cleanTitle, " ")
	cleanTitle = strings.TrimSpace(cleanTitle)

	return cleanTitle, year
}

// abs 取绝对值
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
