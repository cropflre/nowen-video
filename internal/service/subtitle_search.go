package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/saintfish/chardet"
	"go.uber.org/zap"
	"golang.org/x/net/html/charset"
)

// SubtitleSearchService 字幕在线搜索服务。
// OpenSubtitles 保留为可选 API Provider；SubtitleCat 无需 API Key，默认始终可用。
type SubtitleSearchService struct {
	logger      *zap.SugaredLogger
	client      *http.Client
	mu          sync.RWMutex
	apiKey      string
	apiBase     string
	token       string
	tokenExpiry time.Time
	cacheDir    string
	subtitleCat SubtitleProvider
}

// SubtitleSearchResult 字幕搜索结果。
// DownloadURL 保留兼容字段，但外部 Provider 不会把真实下载 URL 暴露给前端。
type SubtitleSearchResult struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	FileName           string   `json:"file_name"`
	Language           string   `json:"language"`
	LanguageName       string   `json:"language_name"`
	Format             string   `json:"format"`
	Rating             float64  `json:"rating"`
	DownloadCount      int      `json:"download_count"`
	Source             string   `json:"source"`
	DownloadURL        string   `json:"download_url"`
	MovieHash          string   `json:"movie_hash,omitempty"`
	MatchType          string   `json:"match_type"` // hash / title / imdb / filename / episode
	FileSize           string   `json:"file_size,omitempty"`
	MatchScore         int      `json:"match_score,omitempty"`
	SourceURL          string   `json:"source_url,omitempty"`
	AvailableLanguages []string `json:"available_languages,omitempty"`
}

// SubtitleDownloadResult 字幕下载结果。
type SubtitleDownloadResult struct {
	FilePath string `json:"file_path"`
	FileName string `json:"file_name"`
	Language string `json:"language"`
	Format   string `json:"format"`
	Source   string `json:"source,omitempty"`
}

// OpenSubtitles API 响应结构。
type osSearchResponse struct {
	TotalPages int              `json:"total_pages"`
	TotalCount int              `json:"total_count"`
	Data       []osSearchResult `json:"data"`
}

type osSearchResult struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		SubtitleID       string  `json:"subtitle_id"`
		Language         string  `json:"language"`
		DownloadCount    int     `json:"download_count"`
		NewDownloadCount int     `json:"new_download_count"`
		HearingImpaired  bool    `json:"hearing_impaired"`
		HD               bool    `json:"hd"`
		FPS              float64 `json:"fps"`
		Votes            int     `json:"votes"`
		Ratings          float64 `json:"ratings"`
		FromTrusted      bool    `json:"from_trusted"`
		ForeignPartsOnly bool    `json:"foreign_parts_only"`
		MovieHashMatch   bool    `json:"moviehash_match"`
		Release          string  `json:"release"`
		FeatureDetails   struct {
			Title     string `json:"title"`
			Year      int    `json:"year"`
			MovieName string `json:"movie_name"`
		} `json:"feature_details"`
		Files []struct {
			FileID   int    `json:"file_id"`
			FileName string `json:"file_name"`
		} `json:"files"`
	} `json:"attributes"`
}

type osDownloadResponse struct {
	Link      string `json:"link"`
	FileName  string `json:"file_name"`
	Remaining int    `json:"remaining"`
}

var srtTimestampPattern = regexp.MustCompile(`(?m)\d{1,2}:\d{2}:\d{2}[,.]\d{3}\s*-->\s*\d{1,2}:\d{2}:\d{2}[,.]\d{3}`)

func NewSubtitleSearchService(apiKey string, cacheDir string, logger *zap.SugaredLogger) *SubtitleSearchService {
	return &SubtitleSearchService{
		logger: logger,
		client: &http.Client{Timeout: 30 * time.Second},
		apiKey: apiKey,
		apiBase: "https://api.opensubtitles.com/api/v1",
		cacheDir: filepath.Join(cacheDir, "subtitles"),
		subtitleCat: NewSubtitleCatProvider(logger),
	}
}

// SetAPIKey 设置 OpenSubtitles API Key。
func (s *SubtitleSearchService) SetAPIKey(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apiKey = key
}

// IsConfigured 检查 OpenSubtitles 是否已配置 API Key。
func (s *SubtitleSearchService) IsConfigured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.apiKey != ""
}

// SearchByTitle 根据标题搜索字幕。
// SubtitleCat 总是参与；OpenSubtitles 仅在 API Key 已配置时参与。
func (s *SubtitleSearchService) SearchByTitle(title string, year int, language string, mediaType string) ([]SubtitleSearchResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	languages := parseSubtitleLanguages(language)
	queries := []string{}
	if strings.TrimSpace(title) != "" && year > 0 {
		queries = append(queries, fmt.Sprintf("%s %d", strings.TrimSpace(title), year))
	}
	if strings.TrimSpace(title) != "" {
		queries = append(queries, strings.TrimSpace(title))
	}

	catResults, catErr := s.subtitleCat.Search(ctx, SubtitleProviderSearchRequest{
		Queries: queries,
		FileName: title,
		Title: title,
		Year: year,
		MediaType: mediaType,
		Languages: languages,
	})

	var osResults []SubtitleSearchResult
	var osErr error
	if s.IsConfigured() {
		osResults, osErr = s.searchOpenSubtitlesByTitle(title, year, language, mediaType)
	}

	results := mergeSubtitleResults(catResults, osResults)
	if len(results) > 0 {
		return results, nil
	}
	if catErr != nil {
		return nil, catErr
	}
	if osErr != nil {
		return nil, osErr
	}
	return nil, nil
}

// SearchByHash 根据媒体文件搜索字幕。
// 对 SubtitleCat 使用当前视频文件名生成多级搜索词；OpenSubtitles 仍保留原有 hash 搜索。
func (s *SubtitleSearchService) SearchByHash(filePath string, language string) ([]SubtitleSearchResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	fileName := filepath.Base(filePath)
	ep := ParseEpisodeFilename(fileName)
	mediaType := "movie"
	title := ""
	year := 0
	if ep.EpisodeNum > 0 {
		mediaType = "episode"
		title = ep.SeriesTitle
		year = ep.Year
	} else {
		parsed := ParseMovieFilename(fileName)
		title = parsed.Title
		year = parsed.Year
	}
	queries := BuildSubtitleSearchQueries(filePath, title, year, mediaType)
	catResults, catErr := s.subtitleCat.Search(ctx, SubtitleProviderSearchRequest{
		Queries: queries,
		FileName: fileName,
		Title: title,
		Year: year,
		MediaType: mediaType,
		Languages: parseSubtitleLanguages(language),
	})

	var osResults []SubtitleSearchResult
	var osErr error
	if s.IsConfigured() {
		osResults, osErr = s.searchOpenSubtitlesByHash(filePath, language)
	}

	results := mergeSubtitleResults(catResults, osResults)
	if len(results) > 0 {
		return results, nil
	}
	if catErr != nil {
		return nil, catErr
	}
	if osErr != nil {
		return nil, osErr
	}
	return nil, nil
}

func (s *SubtitleSearchService) searchOpenSubtitlesByTitle(title string, year int, language string, mediaType string) ([]SubtitleSearchResult, error) {
	s.mu.RLock()
	apiKey := s.apiKey
	s.mu.RUnlock()
	if apiKey == "" {
		return nil, fmt.Errorf("OpenSubtitles API Key 未配置")
	}

	params := url.Values{}
	params.Set("query", title)
	if year > 0 {
		params.Set("year", fmt.Sprintf("%d", year))
	}
	if language != "" {
		params.Set("languages", language)
	}
	if mediaType == "episode" {
		params.Set("type", "episode")
	} else {
		params.Set("type", "movie")
	}
	return s.doOpenSubtitlesSearch(params)
}

func (s *SubtitleSearchService) searchOpenSubtitlesByHash(filePath string, language string) ([]SubtitleSearchResult, error) {
	hash, err := computeOpenSubtitlesHash(filePath)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("计算文件哈希失败: %v，OpenSubtitles hash 搜索跳过", err)
		}
		return nil, err
	}
	params := url.Values{}
	params.Set("moviehash", hash)
	if language != "" {
		params.Set("languages", language)
	}
	return s.doOpenSubtitlesSearch(params)
}

func (s *SubtitleSearchService) doOpenSubtitlesSearch(params url.Values) ([]SubtitleSearchResult, error) {
	s.mu.RLock()
	apiKey := s.apiKey
	s.mu.RUnlock()
	if apiKey == "" {
		return nil, fmt.Errorf("OpenSubtitles API Key 未配置")
	}

	reqURL := fmt.Sprintf("%s/subtitles?%s", s.apiBase, params.Encode())
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "nowen-video v1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenSubtitles API 返回错误: HTTP %d", resp.StatusCode)
	}
	var osResp osSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&osResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return s.convertResults(osResp.Data), nil
}

// Download 下载字幕文件。
// SubtitleCat 使用 opaque ID 在服务端解析真实 URL；OpenSubtitles 保持原有 file_id 流程。
func (s *SubtitleSearchService) Download(fileID string, mediaFilePath string) (*SubtitleDownloadResult, error) {
	if strings.HasPrefix(fileID, subtitleCatProviderName+":download:") {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		download, err := s.subtitleCat.Download(ctx, fileID)
		if err != nil {
			return nil, err
		}
		content, err := normalizeDownloadedSRT(download.Content)
		if err != nil {
			return nil, fmt.Errorf("SubtitleCat 字幕校验失败: %w", err)
		}
		return s.saveSubtitleCatSidecar(mediaFilePath, download.FileName, download.Language, content)
	}
	return s.downloadOpenSubtitles(fileID, mediaFilePath)
}

func (s *SubtitleSearchService) downloadOpenSubtitles(fileID string, mediaFilePath string) (*SubtitleDownloadResult, error) {
	s.mu.RLock()
	apiKey := s.apiKey
	s.mu.RUnlock()
	if apiKey == "" {
		return nil, fmt.Errorf("OpenSubtitles API Key 未配置")
	}

	payload := map[string]interface{}{"file_id": fileID}
	body, _ := json.Marshal(payload)
	reqURL := fmt.Sprintf("%s/download", s.apiBase)
	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("创建下载请求失败: %w", err)
	}
	req.Header.Set("Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "nowen-video v1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载请求返回错误: HTTP %d", resp.StatusCode)
	}
	var dlResp osDownloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&dlResp); err != nil {
		return nil, fmt.Errorf("解析下载响应失败: %w", err)
	}

	subResp, err := s.client.Get(dlResp.Link)
	if err != nil {
		return nil, fmt.Errorf("下载字幕文件失败: %w", err)
	}
	defer subResp.Body.Close()
	if subResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载字幕文件返回错误: HTTP %d", subResp.StatusCode)
	}

	mediaDir := filepath.Dir(mediaFilePath)
	mediaBase := strings.TrimSuffix(filepath.Base(mediaFilePath), filepath.Ext(mediaFilePath))
	subExt := filepath.Ext(dlResp.FileName)
	if subExt == "" {
		subExt = ".srt"
	}
	subFileName := fmt.Sprintf("%s%s", mediaBase, subExt)
	subFilePath := filepath.Join(mediaDir, subFileName)
	outFile, err := os.Create(subFilePath)
	if err != nil {
		if mkErr := os.MkdirAll(s.cacheDir, 0755); mkErr != nil {
			return nil, fmt.Errorf("创建字幕缓存目录失败: %w", mkErr)
		}
		subFilePath = filepath.Join(s.cacheDir, subFileName)
		outFile, err = os.Create(subFilePath)
		if err != nil {
			return nil, fmt.Errorf("创建字幕文件失败: %w", err)
		}
	}
	defer outFile.Close()
	if _, err := io.Copy(outFile, io.LimitReader(subResp.Body, subtitleCatMaxSRTBytes+1)); err != nil {
		return nil, fmt.Errorf("写入字幕文件失败: %w", err)
	}
	if s.logger != nil {
		s.logger.Infof("字幕下载成功: %s -> %s", dlResp.FileName, subFilePath)
	}
	return &SubtitleDownloadResult{
		FilePath: subFilePath,
		FileName: subFileName,
		Language: "",
		Format: strings.TrimPrefix(subExt, "."),
		Source: "opensubtitles",
	}, nil
}

func (s *SubtitleSearchService) saveSubtitleCatSidecar(mediaFilePath, sourceName, language string, content []byte) (*SubtitleDownloadResult, error) {
	if strings.TrimSpace(mediaFilePath) == "" || mediaFilePath == "__strm__" {
		return nil, fmt.Errorf("当前媒体不是可写的本地文件，无法保存外挂字幕")
	}
	mediaDir := filepath.Dir(mediaFilePath)
	mediaBase := strings.TrimSuffix(filepath.Base(mediaFilePath), filepath.Ext(mediaFilePath))
	langTag := subtitleSidecarLanguageTag(language)
	subFileName := fmt.Sprintf("%s.%s.subtitlecat.srt", mediaBase, langTag)
	subFilePath := filepath.Join(mediaDir, subFileName)

	tmp, err := os.CreateTemp(mediaDir, ".nowen-subtitle-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("媒体目录不可写，无法持久化外挂字幕: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(content); err != nil {
		return nil, fmt.Errorf("写入字幕临时文件失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return nil, fmt.Errorf("同步字幕文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("关闭字幕临时文件失败: %w", err)
	}
	_ = os.Chmod(tmpPath, 0644)
	_ = os.Remove(subFilePath)
	if err := os.Rename(tmpPath, subFilePath); err != nil {
		return nil, fmt.Errorf("保存外挂字幕失败: %w", err)
	}
	committed = true
	if s.logger != nil {
		s.logger.Infow("subtitle provider download saved",
			"provider", subtitleCatProviderName,
			"operation", "download",
			"source_file", sourceName,
			"language", language,
			"path", subFilePath,
		)
	}
	return &SubtitleDownloadResult{
		FilePath: subFilePath,
		FileName: subFileName,
		Language: normalizeSubtitleLanguageCode(language),
		Format: "srt",
		Source: subtitleCatProviderName,
	}, nil
}

func normalizeDownloadedSRT(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("字幕内容为空")
	}
	if int64(len(raw)) > subtitleCatMaxSRTBytes {
		return nil, fmt.Errorf("字幕文件超过大小限制")
	}
	trimmed := bytes.TrimSpace(raw)
	lowerPrefix := strings.ToLower(string(trimmed[:minInt(len(trimmed), 512)]))
	if strings.HasPrefix(lowerPrefix, "<!doctype html") || strings.HasPrefix(lowerPrefix, "<html") || strings.Contains(lowerPrefix, "<body") {
		return nil, fmt.Errorf("返回内容是 HTML 页面而不是 SRT")
	}

	text, err := decodeSubtitleBytes(raw)
	if err != nil {
		return nil, err
	}
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSpace(text) + "\n"
	if !srtTimestampPattern.MatchString(text) {
		return nil, fmt.Errorf("内容不符合 SRT 时间轴格式")
	}
	return []byte(text), nil
}

func decodeSubtitleBytes(raw []byte) (string, error) {
	data := raw
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		data = data[3:]
	}
	if utf8.Valid(data) {
		return string(data), nil
	}

	detector := chardet.NewTextDetector()
	if result, err := detector.DetectBest(raw); err == nil && result != nil && result.Charset != "" {
		if reader, convErr := charset.NewReaderLabel(result.Charset, bytes.NewReader(raw)); convErr == nil {
			converted, readErr := io.ReadAll(io.LimitReader(reader, subtitleCatMaxSRTBytes+1))
			if readErr == nil && utf8.Valid(converted) {
				return string(converted), nil
			}
		}
	}
	for _, label := range []string{"utf-16le", "utf-16be", "gb18030", "big5"} {
		reader, err := charset.NewReaderLabel(label, bytes.NewReader(raw))
		if err != nil {
			continue
		}
		converted, err := io.ReadAll(io.LimitReader(reader, subtitleCatMaxSRTBytes+1))
		if err == nil && utf8.Valid(converted) && srtTimestampPattern.Match(converted) {
			return string(converted), nil
		}
	}
	return "", fmt.Errorf("无法识别字幕字符编码")
}

func subtitleSidecarLanguageTag(language string) string {
	switch normalizeSubtitleLanguageCode(language) {
	case "zh-CN":
		return "chs"
	case "zh-TW":
		return "cht"
	case "en":
		return "eng"
	case "ja":
		return "jpn"
	case "ko":
		return "kor"
	default:
		tag := regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(language, "-")
		tag = strings.Trim(strings.ToLower(tag), "-")
		if tag == "" {
			return "und"
		}
		return tag
	}
}

func mergeSubtitleResults(groups ...[]SubtitleSearchResult) []SubtitleSearchResult {
	seen := make(map[string]bool)
	merged := make([]SubtitleSearchResult, 0)
	for _, group := range groups {
		for _, item := range group {
			key := strings.ToLower(item.Source + "|" + item.ID)
			if key == "|" || seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, item)
		}
	}
	sortSubtitleSearchResults(merged)
	return merged
}

// convertResults 转换 OpenSubtitles 结果为统一格式。
func (s *SubtitleSearchService) convertResults(data []osSearchResult) []SubtitleSearchResult {
	results := make([]SubtitleSearchResult, 0, len(data))
	for _, item := range data {
		attrs := item.Attributes
		fileName := ""
		fileID := ""
		if len(attrs.Files) > 0 {
			fileName = attrs.Files[0].FileName
			fileID = fmt.Sprintf("%d", attrs.Files[0].FileID)
		}
		matchType := "title"
		matchScore := 70
		if attrs.MovieHashMatch {
			matchType = "hash"
			matchScore = 100
		}
		results = append(results, SubtitleSearchResult{
			ID: fileID,
			Title: attrs.FeatureDetails.Title,
			FileName: fileName,
			Language: normalizeSubtitleLanguageCode(attrs.Language),
			LanguageName: getLanguageName(attrs.Language),
			Format: getSubtitleFormat(fileName),
			Rating: attrs.Ratings,
			DownloadCount: attrs.DownloadCount,
			Source: "opensubtitles",
			DownloadURL: "",
			MatchType: matchType,
			MatchScore: matchScore,
			AvailableLanguages: []string{normalizeSubtitleLanguageCode(attrs.Language)},
		})
	}
	return results
}

// computeOpenSubtitlesHash 计算 OpenSubtitles 文件哈希。
// 这是项目现有实现，保持兼容；SubtitleCat 不依赖该哈希。
func computeOpenSubtitlesHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	if fi.Size() < 131072 {
		return "", fmt.Errorf("文件太小，无法计算哈希")
	}
	h := md5.New()
	buf := make([]byte, 65536)
	if _, err := io.ReadFull(f, buf); err != nil {
		return "", err
	}
	h.Write(buf)
	if _, err := f.Seek(-65536, io.SeekEnd); err != nil {
		return "", err
	}
	if _, err := io.ReadFull(f, buf); err != nil {
		return "", err
	}
	h.Write(buf)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func getLanguageName(code string) string {
	names := map[string]string{
		"zh-cn": "简体中文", "zh-tw": "繁体中文", "en": "English",
		"ja": "日本語", "ko": "한국어", "fr": "Français",
		"de": "Deutsch", "es": "Español", "pt": "Português",
		"ru": "Русский", "it": "Italiano", "ar": "العربية",
		"th": "ไทย", "vi": "Tiếng Việt",
	}
	if name, ok := names[strings.ToLower(normalizeSubtitleLanguageCode(code))]; ok {
		return name
	}
	return code
}

func getSubtitleFormat(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".srt":
		return "srt"
	case ".ass", ".ssa":
		return "ass"
	case ".vtt":
		return "vtt"
	case ".sub":
		return "sub"
	default:
		return "srt"
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
