package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ScrapeManagerService 刮削数据管理服务
type ScrapeManagerService struct {
	taskRepo    *repository.ScrapeTaskRepo
	historyRepo *repository.ScrapeHistoryRepo
	mediaRepo   *repository.MediaRepo
	seriesRepo  *repository.SeriesRepo
	metadata    *MetadataService
	ai          *AIService
	wsHub       *WSHub
	logger      *zap.SugaredLogger
	client      *http.Client

	// 任务队列
	taskQueue chan string
	queueMu   sync.Mutex
	running   bool
}

// NewScrapeManagerService 创建刮削管理服务
func NewScrapeManagerService(
	taskRepo *repository.ScrapeTaskRepo,
	historyRepo *repository.ScrapeHistoryRepo,
	mediaRepo *repository.MediaRepo,
	seriesRepo *repository.SeriesRepo,
	metadata *MetadataService,
	ai *AIService,
	logger *zap.SugaredLogger,
) *ScrapeManagerService {
	s := &ScrapeManagerService{
		taskRepo:    taskRepo,
		historyRepo: historyRepo,
		mediaRepo:   mediaRepo,
		seriesRepo:  seriesRepo,
		metadata:    metadata,
		ai:          ai,
		logger:      logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		taskQueue: make(chan string, 100),
	}

	// 启动后台任务处理器
	go s.processQueue()

	return s
}

// SetWSHub 设置 WebSocket Hub
func (s *ScrapeManagerService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// ==================== 任务创建 ====================

// CreateTask 创建单个刮削任务
func (s *ScrapeManagerService) CreateTask(urlStr, source, mediaType, userID string) (*model.ScrapeTask, error) {
	// 验证URL
	if urlStr == "" {
		return nil, fmt.Errorf("URL不能为空")
	}

	// 检查重复
	existing, err := s.taskRepo.FindByURL(urlStr)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("该URL已存在刮削任务 (ID: %s, 状态: %s)", existing.ID, existing.Status)
	}

	// 自动识别数据源
	if source == "" {
		source = s.detectSource(urlStr)
	}

	// 从URL提取标题信息
	title := s.extractTitleFromURL(urlStr)

	task := &model.ScrapeTask{
		URL:       urlStr,
		Source:    source,
		Title:     title,
		MediaType: mediaType,
		Status:    "pending",
		CreatedBy: userID,
	}

	if err := s.taskRepo.Create(task); err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	// 记录历史
	s.addHistory(task.ID, "created", fmt.Sprintf("创建刮削任务: %s", urlStr), userID)

	return task, nil
}

// BatchCreateTasks 批量创建刮削任务
func (s *ScrapeManagerService) BatchCreateTasks(urls []string, source, mediaType, userID string) (created int, skipped int, errors []string) {
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		_, err := s.CreateTask(u, source, mediaType, userID)
		if err != nil {
			skipped++
			errors = append(errors, fmt.Sprintf("%s: %s", u, err.Error()))
		} else {
			created++
		}
	}
	return
}

// ==================== 刮削执行 ====================

// StartScrape 开始刮削指定任务
func (s *ScrapeManagerService) StartScrape(taskID, userID string) error {
	task, err := s.taskRepo.FindByID(taskID)
	if err != nil {
		return fmt.Errorf("任务不存在")
	}

	if task.Status == "scraping" || task.Status == "translating" {
		return fmt.Errorf("任务正在执行中")
	}

	// 加入队列
	task.Status = "pending"
	s.taskRepo.Update(task)

	s.addHistory(taskID, "scrape_start", "开始刮削", userID)

	// 异步执行
	go s.executeScrape(task)

	return nil
}

// BatchStartScrape 批量开始刮削
func (s *ScrapeManagerService) BatchStartScrape(taskIDs []string, userID string) (started int, errors []string) {
	for _, id := range taskIDs {
		if err := s.StartScrape(id, userID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", id, err.Error()))
		} else {
			started++
		}
	}
	return
}

// executeScrape 执行单个刮削任务
func (s *ScrapeManagerService) executeScrape(task *model.ScrapeTask) {
	task.Status = "scraping"
	task.Progress = 10
	s.taskRepo.Update(task)
	s.broadcastTaskUpdate(task)

	var err error

	switch task.Source {
	case "tmdb":
		err = s.scrapeTMDb(task)
	case "imdb":
		err = s.scrapeIMDb(task)
	case "douban":
		err = s.scrapeDouban(task)
	case "bangumi":
		err = s.scrapeBangumi(task)
	default:
		// 通用URL刮削 - 尝试从URL中提取信息
		err = s.scrapeGenericURL(task)
	}

	if err != nil {
		task.Status = "failed"
		task.ErrorMessage = err.Error()
		task.Progress = 0
		s.taskRepo.Update(task)
		s.addHistory(task.ID, "scrape_fail", err.Error(), "")
		s.broadcastTaskUpdate(task)
		return
	}

	// 计算数据质量评分
	task.QualityScore = s.calculateQualityScore(task)
	task.Status = "scraped"
	task.Progress = 100
	task.ErrorMessage = ""
	s.taskRepo.Update(task)
	s.addHistory(task.ID, "scrape_done", fmt.Sprintf("刮削完成, 质量评分: %d", task.QualityScore), "")
	s.broadcastTaskUpdate(task)
}

// scrapeTMDb 从TMDb刮削
func (s *ScrapeManagerService) scrapeTMDb(task *model.ScrapeTask) error {
	// 从URL中提取TMDb ID
	tmdbID := s.extractTMDbID(task.URL)
	title := task.Title

	if tmdbID > 0 {
		// 直接通过ID获取
		results, err := s.metadata.SearchTMDb(task.MediaType, fmt.Sprintf("tmdb:%d", tmdbID), 0)
		if err == nil && len(results) > 0 {
			s.applyTMDbResult(task, &results[0])
			return nil
		}
	}

	// 通过标题搜索
	if title == "" {
		title = s.extractTitleFromURL(task.URL)
	}
	if title == "" {
		return fmt.Errorf("无法从URL中提取标题信息")
	}

	searchType := "movie"
	if task.MediaType == "tvshow" {
		searchType = "tv"
	}

	results, err := s.metadata.SearchTMDb(searchType, title, 0)
	if err != nil {
		return fmt.Errorf("TMDb搜索失败: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("TMDb未找到匹配结果: %s", title)
	}

	s.applyTMDbResult(task, &results[0])
	return nil
}

// scrapeIMDb 从 IMDB URL 刮削（通过 TMDb Find API 桥接）
// 工作流程: IMDB URL -> 提取 tt 编号 -> TMDb Find API -> TMDb ID -> 获取完整元数据
func (s *ScrapeManagerService) scrapeIMDb(task *model.ScrapeTask) error {
	// 从 URL 中提取 IMDB ID
	imdbID := s.extractIMDbID(task.URL)
	if imdbID == "" {
		return fmt.Errorf("无法从 IMDB URL 中提取 ID: %s", task.URL)
	}

	s.logger.Infof("IMDB 刮削: 提取到 ID %s, 开始通过 TMDb Find API 查询", imdbID)

	// 通过 TMDb Find API 将 IMDB ID 转换为 TMDb 条目
	movie, mediaType, err := s.metadata.FindByIMDbID(imdbID)
	if err != nil {
		return fmt.Errorf("IMDB ID %s 转换失败: %w", imdbID, err)
	}

	// 更新任务的媒体类型（如果用户未指定）
	if task.MediaType == "" {
		if mediaType == "tv" {
			task.MediaType = "tvshow"
		} else {
			task.MediaType = "movie"
		}
	}

	// 应用 TMDb 搜索结果
	s.applyTMDbResult(task, movie)

	s.logger.Infof("IMDB 刮削成功: %s -> TMDb ID %d (%s)", imdbID, movie.ID, task.ResultTitle)
	return nil
}

// scrapeDouban 从豆瓣刮削
func (s *ScrapeManagerService) scrapeDouban(task *model.ScrapeTask) error {
	title := task.Title
	if title == "" {
		title = s.extractTitleFromURL(task.URL)
	}
	if title == "" {
		return fmt.Errorf("无法从URL中提取标题信息")
	}

	// 使用TMDb作为主要搜索引擎，豆瓣作为补充
	searchType := "movie"
	if task.MediaType == "tvshow" {
		searchType = "tv"
	}

	results, err := s.metadata.SearchTMDb(searchType, title, 0)
	if err != nil || len(results) == 0 {
		// 如果TMDb搜索失败，尝试AI增强
		if s.ai != nil && s.ai.IsEnabled() {
			return s.scrapeWithAI(task, title)
		}
		return fmt.Errorf("搜索失败: %v", err)
	}

	s.applyTMDbResult(task, &results[0])
	return nil
}

// scrapeBangumi 从Bangumi刮削
func (s *ScrapeManagerService) scrapeBangumi(task *model.ScrapeTask) error {
	title := task.Title
	if title == "" {
		title = s.extractTitleFromURL(task.URL)
	}
	if title == "" {
		return fmt.Errorf("无法从URL中提取标题信息")
	}

	// 通过Bangumi搜索
	subjects, err := s.metadata.SearchBangumi(title, 2, 0)
	if err != nil {
		return fmt.Errorf("Bangumi搜索失败: %w", err)
	}
	if len(subjects) == 0 {
		return fmt.Errorf("Bangumi未找到匹配结果: %s", title)
	}

	// 应用第一个结果
	sub := subjects[0]
	task.ResultTitle = sub.NameCN
	if task.ResultTitle == "" {
		task.ResultTitle = sub.Name
	}
	task.ResultOrigTitle = sub.Name
	task.ResultOverview = sub.Summary
	if sub.Rating != nil {
		task.ResultRating = sub.Rating.Score
	}
	if sub.Images != nil {
		task.ResultPoster = sub.Images.Large
	}

	return nil
}

// scrapeGenericURL 通用URL刮削
func (s *ScrapeManagerService) scrapeGenericURL(task *model.ScrapeTask) error {
	title := task.Title
	if title == "" {
		title = s.extractTitleFromURL(task.URL)
	}

	// 先尝试TMDb搜索
	if title != "" {
		searchType := "movie"
		if task.MediaType == "tvshow" {
			searchType = "tv"
		}
		results, err := s.metadata.SearchTMDb(searchType, title, 0)
		if err == nil && len(results) > 0 {
			s.applyTMDbResult(task, &results[0])
			return nil
		}
	}

	// 尝试AI增强
	if s.ai != nil && s.ai.IsEnabled() && title != "" {
		return s.scrapeWithAI(task, title)
	}

	return fmt.Errorf("无法识别URL内容，请手动输入标题")
}

// scrapeWithAI 使用AI增强刮削
func (s *ScrapeManagerService) scrapeWithAI(task *model.ScrapeTask, title string) error {
	if s.ai == nil || !s.ai.IsEnabled() {
		return fmt.Errorf("AI服务未启用")
	}

	systemPrompt := `你是一个影视元数据专家。根据给定的标题，返回该影视作品的详细信息。
请严格返回以下JSON格式（不要包含其他文字）：
{
  "title": "中文标题",
  "orig_title": "原始标题",
  "year": 2024,
  "overview": "简介（100-200字）",
  "genres": "类型1,类型2",
  "rating": 7.5,
  "country": "制片国家",
  "language": "语言"
}`

	userPrompt := fmt.Sprintf("请为以下影视作品生成元数据：\n标题: %s\n类型: %s", title, task.MediaType)

	result, err := s.ai.ChatCompletion(systemPrompt, userPrompt, 0.3, 500)
	if err != nil {
		return fmt.Errorf("AI生成失败: %w", err)
	}

	result = cleanJSONResponse(result)

	var metadata struct {
		Title     string  `json:"title"`
		OrigTitle string  `json:"orig_title"`
		Year      int     `json:"year"`
		Overview  string  `json:"overview"`
		Genres    string  `json:"genres"`
		Rating    float64 `json:"rating"`
		Country   string  `json:"country"`
		Language  string  `json:"language"`
	}

	if err := json.Unmarshal([]byte(result), &metadata); err != nil {
		return fmt.Errorf("AI结果解析失败: %w", err)
	}

	task.ResultTitle = metadata.Title
	task.ResultOrigTitle = metadata.OrigTitle
	task.ResultYear = metadata.Year
	task.ResultOverview = metadata.Overview
	task.ResultGenres = metadata.Genres
	task.ResultRating = metadata.Rating
	task.ResultCountry = metadata.Country
	task.ResultLanguage = metadata.Language

	return nil
}

// ==================== 翻译功能 ====================

// TranslateTask 翻译刮削结果
func (s *ScrapeManagerService) TranslateTask(taskID, targetLang, userID string, fields []string) error {
	task, err := s.taskRepo.FindByID(taskID)
	if err != nil {
		return fmt.Errorf("任务不存在")
	}

	if task.Status != "scraped" && task.Status != "completed" {
		return fmt.Errorf("任务尚未完成刮削，无法翻译")
	}

	if s.ai == nil || !s.ai.IsEnabled() {
		return fmt.Errorf("AI服务未启用，无法进行翻译")
	}

	task.TranslateStatus = "translating"
	task.TranslateLang = targetLang
	s.taskRepo.Update(task)
	s.addHistory(taskID, "translate_start", fmt.Sprintf("开始翻译为 %s", targetLang), userID)
	s.broadcastTaskUpdate(task)

	// 异步执行翻译
	go s.executeTranslation(task, targetLang, fields)

	return nil
}

// BatchTranslate 批量翻译
func (s *ScrapeManagerService) BatchTranslate(taskIDs []string, targetLang, userID string, fields []string) (started int, errors []string) {
	for _, id := range taskIDs {
		if err := s.TranslateTask(id, targetLang, userID, fields); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", id, err.Error()))
		} else {
			started++
		}
	}
	return
}

// executeTranslation 执行翻译
func (s *ScrapeManagerService) executeTranslation(task *model.ScrapeTask, targetLang string, fields []string) {
	langMap := map[string]string{
		"zh-CN": "简体中文",
		"zh-TW": "繁体中文",
		"en":    "英文",
		"ja":    "日文",
		"ko":    "韩文",
	}

	langName := langMap[targetLang]
	if langName == "" {
		langName = targetLang
	}

	// 构建需要翻译的内容
	translateFields := make(map[string]string)
	fieldSet := make(map[string]bool)
	for _, f := range fields {
		fieldSet[f] = true
	}

	// 如果没有指定字段，翻译所有有内容的字段
	if len(fields) == 0 {
		fieldSet["title"] = true
		fieldSet["overview"] = true
		fieldSet["genres"] = true
		fieldSet["tagline"] = true
	}

	if fieldSet["title"] && task.ResultTitle != "" {
		translateFields["title"] = task.ResultTitle
	}
	if fieldSet["overview"] && task.ResultOverview != "" {
		translateFields["overview"] = task.ResultOverview
	}
	if fieldSet["genres"] && task.ResultGenres != "" {
		translateFields["genres"] = task.ResultGenres
	}

	if len(translateFields) == 0 {
		task.TranslateStatus = "done"
		s.taskRepo.Update(task)
		s.addHistory(task.ID, "translate_done", "无需翻译的内容", "")
		s.broadcastTaskUpdate(task)
		return
	}

	// 构建翻译请求
	fieldsJSON, _ := json.Marshal(translateFields)

	systemPrompt := fmt.Sprintf(`你是一个专业的影视翻译专家。请将以下影视元数据翻译为%s。
请严格返回JSON格式，key保持不变，value翻译为目标语言。
注意：
1. 标题翻译要符合目标语言的习惯用法
2. 简介翻译要通顺自然
3. 类型标签使用目标语言的标准表述
4. 如果原文已经是目标语言，保持不变`, langName)

	userPrompt := fmt.Sprintf("请翻译以下内容为%s：\n%s", langName, string(fieldsJSON))

	result, err := s.ai.ChatCompletion(systemPrompt, userPrompt, 0.2, 1000)
	if err != nil {
		task.TranslateStatus = "failed"
		task.ErrorMessage = fmt.Sprintf("翻译失败: %v", err)
		s.taskRepo.Update(task)
		s.addHistory(task.ID, "translate_fail", err.Error(), "")
		s.broadcastTaskUpdate(task)
		return
	}

	result = cleanJSONResponse(result)

	var translated map[string]string
	if err := json.Unmarshal([]byte(result), &translated); err != nil {
		task.TranslateStatus = "failed"
		task.ErrorMessage = fmt.Sprintf("翻译结果解析失败: %v", err)
		s.taskRepo.Update(task)
		s.addHistory(task.ID, "translate_fail", err.Error(), "")
		s.broadcastTaskUpdate(task)
		return
	}

	// 应用翻译结果
	if v, ok := translated["title"]; ok {
		task.TranslatedTitle = v
	}
	if v, ok := translated["overview"]; ok {
		task.TranslatedOverview = v
	}
	if v, ok := translated["genres"]; ok {
		task.TranslatedGenres = v
	}
	if v, ok := translated["tagline"]; ok {
		task.TranslatedTagline = v
	}

	task.TranslateStatus = "done"
	task.Status = "completed"
	task.ErrorMessage = ""
	s.taskRepo.Update(task)
	s.addHistory(task.ID, "translate_done", fmt.Sprintf("翻译完成: %s", targetLang), "")
	s.broadcastTaskUpdate(task)
}

// ==================== 数据管理 ====================

// GetTask 获取任务详情
func (s *ScrapeManagerService) GetTask(id string) (*model.ScrapeTask, error) {
	return s.taskRepo.FindByID(id)
}

// ListTasks 列表查询
func (s *ScrapeManagerService) ListTasks(page, size int, status, source string) ([]model.ScrapeTask, int64, error) {
	return s.taskRepo.List(page, size, status, source)
}

// UpdateTask 更新任务（编辑刮削结果）
func (s *ScrapeManagerService) UpdateTask(id string, updates map[string]interface{}, userID string) (*model.ScrapeTask, error) {
	task, err := s.taskRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("任务不存在")
	}

	// 应用更新
	if v, ok := updates["result_title"].(string); ok {
		task.ResultTitle = v
	}
	if v, ok := updates["result_orig_title"].(string); ok {
		task.ResultOrigTitle = v
	}
	if v, ok := updates["result_overview"].(string); ok {
		task.ResultOverview = v
	}
	if v, ok := updates["result_genres"].(string); ok {
		task.ResultGenres = v
	}
	if v, ok := updates["result_country"].(string); ok {
		task.ResultCountry = v
	}
	if v, ok := updates["result_language"].(string); ok {
		task.ResultLanguage = v
	}
	if v, ok := updates["result_year"].(float64); ok {
		task.ResultYear = int(v)
	}
	if v, ok := updates["result_rating"].(float64); ok {
		task.ResultRating = v
	}

	// 重新计算质量评分
	task.QualityScore = s.calculateQualityScore(task)

	if err := s.taskRepo.Update(task); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}

	s.addHistory(id, "edited", "手动编辑刮削结果", userID)
	return task, nil
}

// DeleteTask 删除任务
func (s *ScrapeManagerService) DeleteTask(id, userID string) error {
	s.addHistory(id, "deleted", "删除任务", userID)
	return s.taskRepo.Delete(id)
}

// BatchDeleteTasks 批量删除
func (s *ScrapeManagerService) BatchDeleteTasks(ids []string, userID string) (int64, error) {
	for _, id := range ids {
		s.addHistory(id, "deleted", "批量删除", userID)
	}
	return s.taskRepo.BatchDelete(ids)
}

// ExportTasks 导出任务数据
func (s *ScrapeManagerService) ExportTasks(ids []string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	for _, id := range ids {
		task, err := s.taskRepo.FindByID(id)
		if err != nil {
			continue
		}
		item := map[string]interface{}{
			"url":           task.URL,
			"source":        task.Source,
			"title":         task.ResultTitle,
			"orig_title":    task.ResultOrigTitle,
			"year":          task.ResultYear,
			"overview":      task.ResultOverview,
			"genres":        task.ResultGenres,
			"rating":        task.ResultRating,
			"country":       task.ResultCountry,
			"language":      task.ResultLanguage,
			"poster":        task.ResultPoster,
			"quality_score": task.QualityScore,
		}
		if task.TranslateStatus == "done" {
			item["translated_title"] = task.TranslatedTitle
			item["translated_overview"] = task.TranslatedOverview
			item["translated_genres"] = task.TranslatedGenres
		}
		results = append(results, item)
	}
	return results, nil
}

// GetStatistics 获取统计信息
func (s *ScrapeManagerService) GetStatistics() (map[string]interface{}, error) {
	counts, err := s.taskRepo.CountByStatus()
	if err != nil {
		return nil, err
	}

	var total int64
	for _, c := range counts {
		total += c
	}

	return map[string]interface{}{
		"total":       total,
		"pending":     counts["pending"],
		"scraping":    counts["scraping"],
		"scraped":     counts["scraped"],
		"failed":      counts["failed"],
		"translating": counts["translating"],
		"completed":   counts["completed"],
	}, nil
}

// GetHistory 获取操作历史
func (s *ScrapeManagerService) GetHistory(taskID string, limit int) ([]model.ScrapeHistory, error) {
	if taskID != "" {
		return s.historyRepo.ListByTaskID(taskID, limit)
	}
	return s.historyRepo.ListRecent(limit)
}

// ==================== 辅助方法 ====================

// detectSource 自动检测数据源
func (s *ScrapeManagerService) detectSource(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "url"
	}
	host := strings.ToLower(u.Host)

	switch {
	case strings.Contains(host, "themoviedb.org") || strings.Contains(host, "tmdb"):
		return "tmdb"
	case strings.Contains(host, "douban.com"):
		return "douban"
	case strings.Contains(host, "imdb.com"):
		return "imdb" // IMDB 通过 TMDb Find API 桥接查询
	case strings.Contains(host, "bgm.tv") || strings.Contains(host, "bangumi"):
		return "bangumi"
	case strings.Contains(host, "javbus"):
		return "javbus"
	case strings.Contains(host, "javdb"):
		return "javdb"
	case strings.Contains(host, "javlibrary"):
		return "javlibrary"
	default:
		return "url"
	}
}

// extractTitleFromURL 从URL中提取标题
func (s *ScrapeManagerService) extractTitleFromURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	// 尝试从路径中提取
	path := u.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		// 去除ID等数字
		last = strings.ReplaceAll(last, "-", " ")
		last = strings.ReplaceAll(last, "_", " ")
		return strings.TrimSpace(last)
	}

	// 尝试从查询参数中提取
	q := u.Query().Get("q")
	if q == "" {
		q = u.Query().Get("query")
	}
	if q == "" {
		q = u.Query().Get("title")
	}
	return q
}

// extractIMDbID 从 IMDB URL 中提取 tt 编号
// 支持格式: https://www.imdb.com/title/tt1234567/ 或 https://www.imdb.com/title/tt1234567
func (s *ScrapeManagerService) extractIMDbID(urlStr string) string {
	re := regexp.MustCompile(`/title/(tt\d+)`)
	matches := re.FindStringSubmatch(urlStr)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractTMDbID 从TMDb URL中提取ID
func (s *ScrapeManagerService) extractTMDbID(urlStr string) int {
	re := regexp.MustCompile(`/(?:movie|tv)/(\d+)`)
	matches := re.FindStringSubmatch(urlStr)
	if len(matches) > 1 {
		var id int
		fmt.Sscanf(matches[1], "%d", &id)
		return id
	}
	return 0
}

// applyTMDbResult 应用TMDb搜索结果
func (s *ScrapeManagerService) applyTMDbResult(task *model.ScrapeTask, result *TMDbMovie) {
	task.ResultTitle = result.Title
	if task.ResultTitle == "" {
		task.ResultTitle = result.Name
	}
	task.ResultOrigTitle = result.OriginalTitle
	if task.ResultOrigTitle == "" {
		task.ResultOrigTitle = result.OriginalName
	}
	task.ResultOverview = result.Overview
	task.ResultRating = result.VoteAverage
	if result.PosterPath != "" {
		task.ResultPoster = "https://image.tmdb.org/t/p/w500" + result.PosterPath
	}

	// 解析年份
	dateStr := result.ReleaseDate
	if dateStr == "" {
		dateStr = result.FirstAirDate
	}
	if len(dateStr) >= 4 {
		fmt.Sscanf(dateStr[:4], "%d", &task.ResultYear)
	}
}

// calculateQualityScore 计算数据质量评分
func (s *ScrapeManagerService) calculateQualityScore(task *model.ScrapeTask) int {
	score := 0
	if task.ResultTitle != "" {
		score += 20
	}
	if task.ResultOverview != "" {
		score += 25
		if len(task.ResultOverview) > 50 {
			score += 5
		}
	}
	if task.ResultGenres != "" {
		score += 15
	}
	if task.ResultRating > 0 {
		score += 10
	}
	if task.ResultPoster != "" {
		score += 10
	}
	if task.ResultYear > 0 {
		score += 5
	}
	if task.ResultCountry != "" {
		score += 5
	}
	if task.ResultOrigTitle != "" {
		score += 5
	}
	if score > 100 {
		score = 100
	}
	return score
}

// addHistory 添加操作历史
func (s *ScrapeManagerService) addHistory(taskID, action, detail, userID string) {
	history := &model.ScrapeHistory{
		TaskID: taskID,
		Action: action,
		Detail: detail,
		UserID: userID,
	}
	if err := s.historyRepo.Create(history); err != nil {
		s.logger.Errorf("记录刮削历史失败: %v", err)
	}
}

// broadcastTaskUpdate 广播任务更新
func (s *ScrapeManagerService) broadcastTaskUpdate(task *model.ScrapeTask) {
	if s.wsHub == nil {
		return
	}
	s.wsHub.BroadcastEvent("scrape_task_update", task)
}

// processQueue 后台任务队列处理器
func (s *ScrapeManagerService) processQueue() {
	for taskID := range s.taskQueue {
		task, err := s.taskRepo.FindByID(taskID)
		if err != nil {
			continue
		}
		s.executeScrape(task)
		randomDelay(3000, 6000) // 任务间隔 3-6 秒随机化，防止过快请求
	}
}
