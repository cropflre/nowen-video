package service

import (
	"compress/gzip"
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

// applyDoubanAuth 统一注入豆瓣 Cookie 认证信息（如用户配置了 Cookie）
// 传入 nil 时会自动跳过。Cookie 以原始字符串形式注入请求头。
func (s *DoubanService) applyDoubanAuth(req *http.Request) {
	if req == nil {
		return
	}
	cookie := s.cfg.GetDoubanCookie()
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
}

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
			randomDelay(3000, 6000) // 重试前等待 3-6 秒
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

// ApplyDoubanData 搜索豆瓣并将结果应用到 media 对象（仅修改内存，不写数据库）
// 用于 Series 刮削时的豆瓣补充，避免意外向 Media 表写入记录
func (s *DoubanService) ApplyDoubanData(media *model.Media, searchTitle string, year int) {
	s.logger.Debugf("豆瓣数据补充（内存模式）: %s (year=%d)", searchTitle, year)

	results, err := s.searchDouban(searchTitle, year)
	if err != nil {
		s.logger.Debugf("豆瓣搜索失败: %v", err)
		return
	}

	if len(results) == 0 && year > 0 {
		randomDelay(3000, 6000) // 重试前等待 3-6 秒
		results, err = s.searchDouban(searchTitle, 0)
		if err != nil {
			return
		}
	}

	if len(results) == 0 {
		s.logger.Debugf("豆瓣未找到匹配: %s", searchTitle)
		return
	}

	s.applyDoubanResult(media, &results[0])
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

	// 设置请求头，模拟浏览器请求（使用随机 User-Agent 和完整的浏览器指纹）
	setBrowserHeaders(req, "https://movie.douban.com/")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	s.applyDoubanAuth(req)

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
		if year > 0 && yearInt > 0 && absInt(yearInt-year) > 1 {
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
		// 搜索和详情请求之间添加随机延迟，模拟真实用户浏览行为
		randomDelay(3000, 6000)

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

	req.Header.Set("User-Agent", getRandomUserAgent())
	req.Header.Set("Referer", fmt.Sprintf("https://movie.douban.com/subject/%s/", doubanID))
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	s.applyDoubanAuth(req)

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
	req.Header.Set("User-Agent", getRandomUserAgent())
	req.Header.Set("Referer", "https://movie.douban.com/")
	s.applyDoubanAuth(req)

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

// absInt 取绝对值（int版本）
func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ==================== Cookie 有效性校验 ====================

// fetchDoubanNickname 访问豆瓣用户主页，尽量解析出昵称
// 失败时返回空字符串，不会中断登录校验流程
func (s *DoubanService) fetchDoubanNickname(peopleURL, cookie string) string {
	req, err := http.NewRequest("GET", peopleURL, nil)
	if err != nil {
		return ""
	}
	setBrowserHeaders(req, "https://www.douban.com/")
	req.Header.Set("Cookie", cookie)

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Debugf("[fetchDoubanNickname] 请求失败：%v", err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	// gzip 兜底
	var reader io.Reader = io.LimitReader(resp.Body, 128*1024)
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		if gz, gzErr := gzip.NewReader(reader); gzErr == nil {
			defer gz.Close()
			reader = gz
		}
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return ""
	}
	html := string(body)

	// 豆瓣个人主页通常 <title>{昵称}的广播</title> 或 <title>{昵称}</title>
	if m := regexp.MustCompile(`<title>\s*([^<]+?)(?:的广播|的帐号|的日记)?\s*</title>`).FindStringSubmatch(html); len(m) >= 2 {
		name := strings.TrimSpace(m[1])
		if name != "" && name != "豆瓣" {
			return name
		}
	}
	// 兜底：<h1>{昵称}</h1>
	if m := regexp.MustCompile(`<h1[^>]*>\s*([^<]+?)\s*</h1>`).FindStringSubmatch(html); len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// ValidateDoubanCookie 校验当前配置的豆瓣 Cookie 是否有效
// 返回值：
//   - valid: 是否有效（登录态正常）
//   - username: 识别到的豆瓣昵称（可能为空）
//   - err: 网络错误或其他非预期错误
//
// 判定规则：
//  1. 若未配置 Cookie，直接返回 valid=false（匿名模式）
//  2. 访问 https://www.douban.com/mine/，若被重定向到登录页或 accounts.douban.com 视为失效
//  3. 若页面中能提取到用户昵称（<a class="bn-more">...</a>）视为有效
func (s *DoubanService) ValidateDoubanCookie() (valid bool, username string, err error) {
	cookie := s.cfg.GetDoubanCookie()
	if cookie == "" {
		return false, "", nil
	}

	// 禁止自动重定向，以便观测 302 跳转到登录页
	noRedirectClient := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("GET", "https://www.douban.com/mine/", nil)
	if err != nil {
		return false, "", err
	}
	setBrowserHeaders(req, "https://www.douban.com/")
	req.Header.Set("Cookie", cookie)

	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("请求豆瓣失败: %w", err)
	}
	defer resp.Body.Close()

	// 记录调试日志：真实 StatusCode / Location / Content-Encoding
	// 线上排查"Cookie 过期或被风控"时，先看这条日志判断是 302 风控还是 200 后解析失败
	location := resp.Header.Get("Location")
	contentEncoding := resp.Header.Get("Content-Encoding")
	s.logger.Infof("[ValidateDoubanCookie] status=%d location=%q content-encoding=%q cookie-len=%d",
		resp.StatusCode, location, contentEncoding, len(cookie))

	// 3xx 跳转处理
	//
	// 豆瓣 /mine/ 的行为：
	//   - 未登录：302 -> https://accounts.douban.com/passport/login?...
	//   - 已登录：302 -> https://www.douban.com/people/{uid}/   ← 这是登录成功的标志！
	//   - 被风控：302 -> https://sec.douban.com/... 之类
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		// 情况 1：跳到登录页/验证页 → 失效
		if strings.Contains(location, "accounts.douban.com") ||
			strings.Contains(location, "passport") ||
			strings.Contains(location, "sec.douban.com") ||
			strings.Contains(location, "/login") {
			s.logger.Warnf("[ValidateDoubanCookie] 登录态失效/被风控：重定向到 %s", location)
			return false, "", nil
		}
		// 情况 2：跳到 https://www.douban.com/people/{uid}/ → 已登录 ✅
		// 顺带抓一次 people 页把昵称读出来
		if strings.Contains(location, "www.douban.com/people/") {
			s.logger.Infof("[ValidateDoubanCookie] 登录成功，跳转到用户主页 %s", location)
			uname := s.fetchDoubanNickname(location, cookie)
			return true, uname, nil
		}
		// 情况 3：其他未知跳转，保守视为失效并记日志便于排查
		s.logger.Warnf("[ValidateDoubanCookie] 未知重定向 %s -> %s，保守视为失效", req.URL, location)
		return false, "", nil
	}

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("豆瓣返回状态码: %d", resp.StatusCode)
	}

	// 读取响应体前 128KB 用于解析昵称
	// 注意：Go net/http 会根据请求头决定是否自动解压：
	//   - 若本方未手动设 Accept-Encoding，Go 会自动加 gzip 并自动解压（resp.Uncompressed=true，Content-Encoding 被清空）
	//   - 若本方手动设了 Accept-Encoding，Go 不会解压，这里需要手动 gunzip
	// 这里做一次兜底：如果 Content-Encoding 仍为 gzip，则手动解压
	var reader io.Reader = io.LimitReader(resp.Body, 128*1024)
	if strings.EqualFold(contentEncoding, "gzip") {
		gz, gzErr := gzip.NewReader(reader)
		if gzErr != nil {
			s.logger.Warnf("[ValidateDoubanCookie] gzip 解压失败：%v（可能是误判的压缩头）", gzErr)
		} else {
			defer gz.Close()
			reader = gz
		}
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return false, "", err
	}
	html := string(body)

	// 调试日志：响应体长度 + 前 200 字节（脱敏用，便于定位是"HTML 页面"还是"二进制乱码"）
	preview := html
	if len(preview) > 200 {
		preview = preview[:200]
	}
	s.logger.Debugf("[ValidateDoubanCookie] body-len=%d preview=%q", len(html), preview)

	// 页面中若包含登录提示视为失效
	if strings.Contains(html, "passport/login") || strings.Contains(html, "登录豆瓣") {
		s.logger.Warnf("[ValidateDoubanCookie] 页面中出现登录提示，判定为失效")
		return false, "", nil
	}

	// 尝试提取昵称：<a href="https://www.douban.com/people/{uid}/" class="bn-more">...<span>昵称</span></a>
	nicknameRegex := regexp.MustCompile(`<a[^>]*class="bn-more"[^>]*>\s*<span>([^<]+)</span>`)
	if m := nicknameRegex.FindStringSubmatch(html); len(m) >= 2 {
		return true, strings.TrimSpace(m[1]), nil
	}
	// 兜底：title 中含有"的帐号"表示已登录
	titleRegex := regexp.MustCompile(`<title>\s*([^<]+?)的帐号\s*</title>`)
	if m := titleRegex.FindStringSubmatch(html); len(m) >= 2 {
		return true, strings.TrimSpace(m[1]), nil
	}

	// 页面返回 200 但提取不到昵称：保守视为有效（避免误杀）
	s.logger.Warnf("[ValidateDoubanCookie] 200 但未能提取昵称，保守视为有效")
	return true, "", nil
}
