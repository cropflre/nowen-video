package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== Bangumi API 数据结构 ====================

// BangumiSubjectType Bangumi 条目类型
const (
	BangumiTypeBook  = 1 // 书籍
	BangumiTypeAnime = 2 // 动画
	BangumiTypeMusic = 3 // 音乐
	BangumiTypeGame  = 4 // 游戏
	BangumiTypeReal  = 6 // 三次元（电视剧/电影/综艺）
)

// BangumiSearchResult Bangumi 搜索结果（旧API）
type BangumiSearchResult struct {
	Results int              `json:"results"`
	List    []BangumiSubject `json:"list"`
}

// BangumiSubject Bangumi 条目
type BangumiSubject struct {
	ID       int               `json:"id"`
	Type     int               `json:"type"`     // 1=书籍 2=动画 3=音乐 4=游戏 6=三次元
	Name     string            `json:"name"`     // 原始名称（日文/英文）
	NameCN   string            `json:"name_cn"`  // 中文名称
	Summary  string            `json:"summary"`  // 简介
	AirDate  string            `json:"air_date"` // 首播日期
	URL      string            `json:"url"`      // Bangumi 页面URL
	Images   *BangumiImages    `json:"images"`   // 图片
	Rating   *BangumiRating    `json:"rating"`   // 评分
	Tags     []BangumiTag      `json:"tags"`     // 标签
	Eps      int               `json:"eps"`      // 总话数
	Platform string            `json:"platform"` // 平台（TV/Web/OVA等）
	Infobox  []BangumiInfoItem `json:"infobox"`  // 详细信息框
}

// BangumiImages Bangumi 图片
type BangumiImages struct {
	Large  string `json:"large"`
	Common string `json:"common"`
	Medium string `json:"medium"`
	Small  string `json:"small"`
	Grid   string `json:"grid"`
}

// BangumiRating Bangumi 评分
type BangumiRating struct {
	Total int     `json:"total"` // 评分人数
	Score float64 `json:"score"` // 评分（满分10）
	Rank  int     `json:"rank"`  // 排名
}

// BangumiTag Bangumi 标签
type BangumiTag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// BangumiInfoItem Bangumi 信息框项
type BangumiInfoItem struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"` // 可能是 string 或 []map[string]string
}

// BangumiSearchResponse Bangumi 新 API 搜索响应
type BangumiSearchResponse struct {
	Total int              `json:"total"`
	Limit int              `json:"limit"`
	Data  []BangumiSubject `json:"data"`
}

// ==================== BangumiService ====================

// BangumiService Bangumi 元数据刮削服务
type BangumiService struct {
	mediaRepo  *repository.MediaRepo
	seriesRepo *repository.SeriesRepo
	cfg        *config.Config
	logger     *zap.SugaredLogger
	client     *http.Client
}

// NewBangumiService 创建 Bangumi 刮削服务
func NewBangumiService(mediaRepo *repository.MediaRepo, seriesRepo *repository.SeriesRepo, cfg *config.Config, logger *zap.SugaredLogger) *BangumiService {
	return &BangumiService{
		mediaRepo:  mediaRepo,
		seriesRepo: seriesRepo,
		cfg:        cfg,
		logger:     logger,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// IsEnabled 检查 Bangumi 数据源是否可用（有 Access Token 即可用）
func (s *BangumiService) IsEnabled() bool {
	// Bangumi API 不强制要求 Token，但有 Token 可以提高速率限制
	// 即使没有 Token，也可以使用（只需提供 User-Agent）
	return true
}

// ==================== 核心方法 ====================

// ScrapeMedia 为单个媒体刮削 Bangumi 元数据（作为 TMDb/豆瓣的补充）
func (s *BangumiService) ScrapeMedia(media *model.Media, searchTitle string, year int) error {
	s.logger.Debugf("Bangumi 刮削: %s (year=%d)", searchTitle, year)

	// 根据媒体类型选择搜索类型
	subjectType := BangumiTypeReal // 默认三次元
	if media.MediaType == "episode" || strings.Contains(strings.ToLower(media.Genres), "动画") ||
		strings.Contains(strings.ToLower(media.Genres), "anime") {
		subjectType = BangumiTypeAnime
	}

	results, err := s.SearchSubjects(searchTitle, subjectType, year)
	if err != nil {
		return fmt.Errorf("Bangumi 搜索失败: %w", err)
	}

	if len(results) == 0 {
		// 尝试动画类型
		if subjectType != BangumiTypeAnime {
			randomDelay(1500, 3000) // 重试前等待
			results, err = s.SearchSubjects(searchTitle, BangumiTypeAnime, year)
			if err != nil {
				return fmt.Errorf("Bangumi 搜索失败: %w", err)
			}
		}
		// 不带年份重试
		if len(results) == 0 && year > 0 {
			randomDelay(1500, 3000) // 重试前等待
			results, err = s.SearchSubjects(searchTitle, subjectType, 0)
			if err != nil {
				return fmt.Errorf("Bangumi 搜索失败: %w", err)
			}
		}
		if len(results) == 0 {
			return fmt.Errorf("Bangumi 未找到匹配: %s", searchTitle)
		}
	}

	best := results[0]

	// 获取详情以获取更完整的信息（搜索与详情请求之间添加随机延迟）
	randomDelay(1500, 3000)
	detail, err := s.GetSubjectDetail(best.ID)
	if err == nil {
		best = *detail
	}

	// 应用 Bangumi 数据（仅补充缺失字段）
	s.applyBangumiResult(media, &best)

	return s.mediaRepo.Update(media)
}

// ApplyBangumiData 搜索 Bangumi 并将结果应用到 media 对象（仅修改内存，不写数据库）
// 用于 Series 刮削时的 Bangumi 补充
func (s *BangumiService) ApplyBangumiData(media *model.Media, searchTitle string, year int) {
	s.logger.Debugf("Bangumi 数据补充（内存模式）: %s (year=%d)", searchTitle, year)

	results, err := s.SearchSubjects(searchTitle, BangumiTypeAnime, year)
	if err != nil {
		s.logger.Debugf("Bangumi 搜索失败: %v", err)
		return
	}

	if len(results) == 0 {
		// 尝试三次元类型
		randomDelay(1500, 3000)
		results, _ = s.SearchSubjects(searchTitle, BangumiTypeReal, year)
	}

	if len(results) == 0 && year > 0 {
		randomDelay(1500, 3000)
		results, _ = s.SearchSubjects(searchTitle, BangumiTypeAnime, 0)
	}

	if len(results) == 0 {
		s.logger.Debugf("Bangumi 未找到匹配: %s", searchTitle)
		return
	}

	best := results[0]
	randomDelay(1500, 3000) // 搜索与详情请求之间添加随机延迟
	detail, err := s.GetSubjectDetail(best.ID)
	if err == nil {
		best = *detail
	}

	s.applyBangumiResult(media, &best)
}

// applyBangumiResult 将 Bangumi 结果应用到媒体（仅补充缺失字段）
func (s *BangumiService) applyBangumiResult(media *model.Media, subject *BangumiSubject) {
	// 仅补充缺失字段；锁定字段即使为空也不填。
	guard := newScrapeGuard(ScrapeModeFillMissing, media.LockedFields)
	// 补充原始标题
	if media.OrigTitle == "" && subject.Name != "" && !guard.locked("orig_title") {
		media.OrigTitle = subject.Name
	}

	// 补充简介
	if media.Overview == "" && subject.Summary != "" && !guard.locked("overview") {
		media.Overview = subject.Summary
	}

	// 补充评分（Bangumi 满分 10 分，直接使用）
	if media.Rating == 0 && subject.Rating != nil && subject.Rating.Score > 0 && !guard.locked("rating") {
		media.Rating = subject.Rating.Score
	}

	// 补充年份
	if media.Year == 0 && subject.AirDate != "" && !guard.locked("year") {
		if len(subject.AirDate) >= 4 {
			media.Year, _ = strconv.Atoi(subject.AirDate[:4])
		}
	}

	// 补充类型标签
	if media.Genres == "" && len(subject.Tags) > 0 && !guard.locked("genres") {
		var tags []string
		for i, tag := range subject.Tags {
			if i >= 5 {
				break // 最多取5个标签
			}
			tags = append(tags, tag.Name)
		}
		media.Genres = strings.Join(tags, ",")
	}

	// 从 Infobox 中提取额外信息
	if subject.Infobox != nil {
		for _, item := range subject.Infobox {
			switch item.Key {
			case "制片国家/地区":
				if media.Country == "" {
					if str, ok := item.Value.(string); ok {
						media.Country = str
					}
				}
			case "语言":
				if media.Language == "" {
					if str, ok := item.Value.(string); ok {
						media.Language = str
					}
				}
			case "动画制作", "制作", "出品":
				if media.Studio == "" {
					if str, ok := item.Value.(string); ok {
						media.Studio = str
					}
				}
			}
		}
	}

	// 补充海报图片
	if media.PosterPath == "" && subject.Images != nil && subject.Images.Large != "" {
		localPath, err := s.downloadBangumiCover(media, subject.Images.Large)
		if err == nil {
			media.PosterPath = localPath
		}
	}
}

// ==================== Bangumi API 调用 ====================

// SearchSubjects 通过 Bangumi API 搜索条目
func (s *BangumiService) SearchSubjects(query string, subjectType int, year int) ([]BangumiSubject, error) {
	// 使用旧版搜索 API（更稳定，不需要 Token）
	// GET https://api.bgm.tv/search/subject/{keyword}?type={type}&responseGroup=small
	encodedQuery := url.PathEscape(query)
	apiURL := fmt.Sprintf("https://api.bgm.tv/search/subject/%s?type=%d&responseGroup=small&max_results=10",
		encodedQuery, subjectType)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Bangumi 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == 404 {
		return nil, nil // 无结果
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bangumi 返回状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// Bangumi 搜索无结果时可能返回 {"results":0}
	var searchResult BangumiSearchResult
	if err := json.Unmarshal(body, &searchResult); err != nil {
		// 可能是空结果或格式异常
		s.logger.Debugf("Bangumi 搜索解析失败（可能无结果）: %s", string(body[:min(len(body), 200)]))
		return nil, nil
	}

	if searchResult.Results == 0 || len(searchResult.List) == 0 {
		return nil, nil
	}

	// 年份过滤
	var filtered []BangumiSubject
	for _, subject := range searchResult.List {
		if year > 0 && subject.AirDate != "" && len(subject.AirDate) >= 4 {
			subYear, _ := strconv.Atoi(subject.AirDate[:4])
			if subYear > 0 && absInt(subYear-year) > 1 {
				continue
			}
		}
		filtered = append(filtered, subject)
	}

	if len(filtered) == 0 {
		// 如果所有结果都被年份过滤掉了，返回原始结果
		return searchResult.List, nil
	}

	return filtered, nil
}

// SearchSubjectsV0 通过 Bangumi 新 API (v0) 搜索条目（需要 User-Agent，可选 Token）
func (s *BangumiService) SearchSubjectsV0(query string, subjectType int) ([]BangumiSubject, error) {
	apiURL := "https://api.bgm.tv/v0/search/subjects"

	// 构建请求体
	reqBody := map[string]interface{}{
		"keyword": query,
		"filter": map[string]interface{}{
			"type": []int{subjectType},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Bangumi v0 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bangumi v0 返回状态码: %d", resp.StatusCode)
	}

	var searchResp BangumiSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("解析 Bangumi v0 搜索响应失败: %w", err)
	}

	return searchResp.Data, nil
}

// GetSubjectDetail 获取 Bangumi 条目详情
func (s *BangumiService) GetSubjectDetail(subjectID int) (*BangumiSubject, error) {
	apiURL := fmt.Sprintf("https://api.bgm.tv/v0/subjects/%d", subjectID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Bangumi 详情请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bangumi 详情返回状态码: %d", resp.StatusCode)
	}

	var subject BangumiSubject
	if err := json.NewDecoder(resp.Body).Decode(&subject); err != nil {
		return nil, fmt.Errorf("解析 Bangumi 详情失败: %w", err)
	}

	return &subject, nil
}

// setHeaders 设置 Bangumi API 请求头
func (s *BangumiService) setHeaders(req *http.Request) {
	// Bangumi API 强制要求 User-Agent，使用项目标识 + 真实浏览器 UA 混合
	req.Header.Set("User-Agent", "nowen-video/1.0 (https://github.com/nowen-video)")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")

	// 如果配置了 Access Token，添加认证头
	token := s.cfg.Secrets.BangumiAccessToken
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// ==================== 图片下载 ====================

// downloadBangumiCover 下载 Bangumi 封面图到本地
func (s *BangumiService) downloadBangumiCover(media *model.Media, coverURL string) (string, error) {
	if coverURL == "" {
		return "", fmt.Errorf("封面URL为空")
	}

	req, err := http.NewRequest("GET", coverURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "nowen-video/1.0 (https://github.com/nowen-video)")
	req.Header.Set("Referer", "https://bgm.tv/")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载封面失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("封面请求失败: %d", resp.StatusCode)
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

	// 保存到媒体文件同目录
	mediaDir := filepath.Dir(media.FilePath)
	baseName := strings.TrimSuffix(filepath.Base(media.FilePath), filepath.Ext(media.FilePath))
	localPath := filepath.Join(mediaDir, fmt.Sprintf("%s-poster-bangumi%s", baseName, ext))

	file, err := os.Create(localPath)
	if err != nil {
		// 回退到缓存目录
		cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", media.ID)
		os.MkdirAll(cacheDir, 0755)
		localPath = filepath.Join(cacheDir, fmt.Sprintf("poster-bangumi%s", ext))
		file, err = os.Create(localPath)
		if err != nil {
			return "", fmt.Errorf("创建封面文件失败: %w", err)
		}
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("保存封面失败: %w", err)
	}

	s.logger.Debugf("已下载 Bangumi 封面: %s", localPath)
	return localPath, nil
}

// DownloadBangumiCoverForSeries 下载 Bangumi 封面图到剧集合集
func (s *BangumiService) DownloadBangumiCoverForSeries(series *model.Series, coverURL string) (string, error) {
	if coverURL == "" {
		return "", fmt.Errorf("封面URL为空")
	}

	req, err := http.NewRequest("GET", coverURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "nowen-video/1.0 (https://github.com/nowen-video)")
	req.Header.Set("Referer", "https://bgm.tv/")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载封面失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("封面请求失败: %d", resp.StatusCode)
	}

	ext := ".jpg"
	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "png"):
		ext = ".png"
	case strings.Contains(ct, "webp"):
		ext = ".webp"
	}

	cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "images", "series", series.ID)
	os.MkdirAll(cacheDir, 0755)
	localPath := filepath.Join(cacheDir, "poster-bangumi"+ext)

	file, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("创建封面文件失败: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("保存封面失败: %w", err)
	}

	s.logger.Debugf("已下载 Bangumi 封面 (Series): %s", localPath)
	return localPath, nil
}

// ==================== 手动匹配方法 ====================

// MatchMediaWithBangumi 手动关联 Bangumi 条目到指定媒体
func (s *BangumiService) MatchMediaWithBangumi(media *model.Media, subjectID int) error {
	subject, err := s.GetSubjectDetail(subjectID)
	if err != nil {
		return fmt.Errorf("获取 Bangumi 条目详情失败: %w", err)
	}

	// 应用标题：手动匹配固定走全部替换，但仍受字段锁定约束。
	matchGuard := newScrapeGuard(ScrapeModeReplace, media.LockedFields)
	if subject.NameCN != "" && !matchGuard.locked("title") {
		media.Title = subject.NameCN
	}
	if !matchGuard.locked("orig_title") {
		media.OrigTitle = subject.Name
	}

	// 应用简介
	if subject.Summary != "" && !matchGuard.locked("overview") {
		media.Overview = subject.Summary
	}

	// 应用评分
	if subject.Rating != nil && subject.Rating.Score > 0 && !matchGuard.locked("rating") {
		media.Rating = subject.Rating.Score
	}

	// 应用年份
	if subject.AirDate != "" && len(subject.AirDate) >= 4 && !matchGuard.locked("year") {
		media.Year, _ = strconv.Atoi(subject.AirDate[:4])
	}

	// 应用类型标签
	if len(subject.Tags) > 0 && !matchGuard.locked("genres") {
		var tags []string
		for i, tag := range subject.Tags {
			if i >= 5 {
				break
			}
			tags = append(tags, tag.Name)
		}
		media.Genres = strings.Join(tags, ",")
	}

	// 提取 Infobox 信息
	s.applyInfobox(media, subject.Infobox)

	// 下载海报
	if subject.Images != nil && subject.Images.Large != "" && !matchGuard.locked("poster") {
		localPath, err := s.downloadBangumiCover(media, subject.Images.Large)
		if err == nil {
			media.PosterPath = localPath
		}
	}

	s.logger.Infof("已手动匹配 Bangumi: %s -> ID %d", media.Title, subjectID)
	return s.mediaRepo.Update(media)
}

// MatchSeriesWithBangumi 手动关联 Bangumi 条目到指定剧集合集
func (s *BangumiService) MatchSeriesWithBangumi(series *model.Series, subjectID int) error {
	subject, err := s.GetSubjectDetail(subjectID)
	if err != nil {
		return fmt.Errorf("获取 Bangumi 条目详情失败: %w", err)
	}

	// 应用标题：手动匹配固定走全部替换，但仍受字段锁定约束。
	matchGuard := newScrapeGuard(ScrapeModeReplace, series.LockedFields)
	if subject.NameCN != "" && !matchGuard.locked("title") {
		series.Title = subject.NameCN
	}
	if !matchGuard.locked("orig_title") {
		series.OrigTitle = subject.Name
	}

	// 应用简介
	if subject.Summary != "" && !matchGuard.locked("overview") {
		series.Overview = subject.Summary
	}

	// 应用评分
	if subject.Rating != nil && subject.Rating.Score > 0 && !matchGuard.locked("rating") {
		series.Rating = subject.Rating.Score
	}

	// 应用年份
	if subject.AirDate != "" && len(subject.AirDate) >= 4 && !matchGuard.locked("year") {
		series.Year, _ = strconv.Atoi(subject.AirDate[:4])
	}

	// 应用类型标签
	if len(subject.Tags) > 0 && !matchGuard.locked("genres") {
		var tags []string
		for i, tag := range subject.Tags {
			if i >= 5 {
				break
			}
			tags = append(tags, tag.Name)
		}
		series.Genres = strings.Join(tags, ",")
	}

	// 提取 Infobox 信息到 Series
	for _, item := range subject.Infobox {
		switch item.Key {
		case "制片国家/地区":
			if str, ok := item.Value.(string); ok {
				series.Country = str
			}
		case "语言":
			if str, ok := item.Value.(string); ok {
				series.Language = str
			}
		case "动画制作", "制作", "出品":
			if str, ok := item.Value.(string); ok {
				series.Studio = str
			}
		}
	}

	// 下载海报
	if subject.Images != nil && subject.Images.Large != "" && !matchGuard.locked("poster") {
		localPath, err := s.DownloadBangumiCoverForSeries(series, subject.Images.Large)
		if err == nil {
			series.PosterPath = localPath
		}
	}

	s.logger.Infof("已手动匹配 Bangumi (Series): %s -> ID %d", series.Title, subjectID)
	return s.seriesRepo.Update(series)
}

// applyInfobox 从 Infobox 提取额外信息
func (s *BangumiService) applyInfobox(media *model.Media, infobox []BangumiInfoItem) {
	if infobox == nil {
		return
	}
	for _, item := range infobox {
		switch item.Key {
		case "制片国家/地区":
			if media.Country == "" {
				if str, ok := item.Value.(string); ok {
					media.Country = str
				}
			}
		case "语言":
			if media.Language == "" {
				if str, ok := item.Value.(string); ok {
					media.Language = str
				}
			}
		case "动画制作", "制作", "出品":
			if media.Studio == "" {
				if str, ok := item.Value.(string); ok {
					media.Studio = str
				}
			}
		case "话数":
			if media.Runtime == 0 {
				if str, ok := item.Value.(string); ok {
					if eps, e := strconv.Atoi(str); e == nil {
						media.Runtime = eps
					}
				}
			}
		}
	}
}

// min 返回较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
