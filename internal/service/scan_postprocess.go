package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== 扫描后处理：统一规则处理流水线 ====================
//
// 在影视文件扫描入库后，对每条 Media 顺序执行：
//   阶段 1 · 识别（identify）   - filename_parser 规则解析；置信度 < 阈值 时调用 AI Fallback
//   阶段 2 · 归类（classify）   - 类别 / 地区 / 年代 / 类型标签 / 质量档 / 虚拟路径
//   阶段 3 · 命名（naming）     - 仅 DB 记录的 Jellyfin/Emby 风格建议命名
//
// 安全约束（强保证）：
//   - 该服务**不导入 os 包**，不会调用任何 os.Rename/Create/Remove/MkdirAll 等磁盘写入 API；
//   - 所有结果仅写入 media_classifications 表；
//   - 与 SmartRenameService 完全独立，不会触发其 Execute 流程。
//
// 触发：
//   - 整库扫描完成后（ScannerService.SetOnScanComplete 钩子），异步入队整库的 Media；
//   - 用户在管理后台主动调用 reprocess 接口；
//   - 单条 Media 通过 ProcessMedia 直接处理（用于测试或单条修复）。

// ============================ Constants ============================

// ScanPostProcess 队列默认配置
const (
	scanPostProcessDefaultWorkers   = 1
	scanPostProcessDefaultQueueSize = 4096
)

// ============================ Config ============================

// ScanPostProcessConfig 服务级配置
type ScanPostProcessConfig struct {
	NamingStyle           string  // jellyfin / plex
	AIConfidenceThreshold float64 // 触发 AI Fallback 阈值（默认 0.7；全自动托管下使用 1.0 强制走 AI）
	EnableAIFallback      bool    // 是否启用 AI Fallback
	ForceAIIdentify       bool    // 🚀 全自动托管：强制每条都调 AI（不看规则置信度）
	Workers               int     // 队列消费协程数
	QueueSize             int     // 队列容量
}

// DefaultScanPostProcessConfig 默认配置
func DefaultScanPostProcessConfig() ScanPostProcessConfig {
	return ScanPostProcessConfig{
		NamingStyle:           NamingStyleJellyfin,
		AIConfidenceThreshold: 0.7,
		EnableAIFallback:      true,
		Workers:               scanPostProcessDefaultWorkers,
		QueueSize:             scanPostProcessDefaultQueueSize,
	}
}

// ============================ Service ============================

// scanPostParsed 内部使用的"识别融合结果"，扩展了 ParsedFilename（后者没有 MediaType/Season/Episode）。
type scanPostParsed struct {
	Title     string
	TitleAlt  string
	Year      int
	TMDbID    int
	IMDbID    string
	MediaType string // movie / episode / unknown
	Season    int
	Episode   int
}

// ScanPostProcessService 扫描后处理服务
type ScanPostProcessService struct {
	repo        *repository.ScanClassificationRepo
	mediaRepo   *repository.MediaRepo
	libraryRepo *repository.LibraryRepo
	ai          *AIService
	cfg         ScanPostProcessConfig
	logger      *zap.SugaredLogger

	// 异步队列
	queue   chan string // 仅传 mediaID
	stopCh  chan struct{}
	started bool // 避免重复启动 worker；Stop 后重建 stopCh 可重启
	workWG  sync.WaitGroup
	mu      sync.Mutex
}

// NewScanPostProcessService 构造服务
func NewScanPostProcessService(
	repo *repository.ScanClassificationRepo,
	mediaRepo *repository.MediaRepo,
	libraryRepo *repository.LibraryRepo,
	ai *AIService,
	cfg ScanPostProcessConfig,
	logger *zap.SugaredLogger,
) *ScanPostProcessService {
	if cfg.AIConfidenceThreshold <= 0 {
		cfg.AIConfidenceThreshold = 0.7
	}
	if cfg.NamingStyle == "" {
		cfg.NamingStyle = NamingStyleJellyfin
	}
	if cfg.Workers <= 0 {
		cfg.Workers = scanPostProcessDefaultWorkers
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = scanPostProcessDefaultQueueSize
	}
	return &ScanPostProcessService{
		repo:        repo,
		mediaRepo:   mediaRepo,
		libraryRepo: libraryRepo,
		ai:          ai,
		cfg:         cfg,
		logger:      logger,
		queue:       make(chan string, cfg.QueueSize),
		stopCh:      make(chan struct{}),
	}
}

// Start 启动后台 worker（幂等）。
// 之前使用 sync.Once 限制为「进程内仅能启动一次」，Stop 后无法重启。
// 现改为 started 标志位 + 重建 stopCh，支持「Stop -> Start」循环（如热重载场景）。
func (s *ScanPostProcessService) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	// 重建可能被 Stop 关闭过的 stopCh，避免 worker 启动后立即退出
	select {
	case <-s.stopCh:
		s.stopCh = make(chan struct{})
	default:
	}
	s.started = true
	for i := 0; i < s.cfg.Workers; i++ {
		s.workWG.Add(1)
		go s.worker(i)
	}
	s.logger.Infof("[ScanPostProcess] 后台 worker 已启动 workers=%d queue=%d", s.cfg.Workers, s.cfg.QueueSize)
}

// Stop 同步停止后台 worker：广播 close(stopCh) 后等待所有 worker 退出。
// 调用后不会丢业已取出但未处理完的任务。
func (s *ScanPostProcessService) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.started = false
	close(s.stopCh)
	s.mu.Unlock()
	s.workWG.Wait()
	s.logger.Infof("[ScanPostProcess] 后台 worker 已全部退出")
}

// worker 队列消费协程
func (s *ScanPostProcessService) worker(idx int) {
	defer s.workWG.Done()
	for {
		select {
		case <-s.stopCh:
			s.logger.Infof("[ScanPostProcess] worker#%d 退出", idx)
			return
		case mediaID, ok := <-s.queue:
			if !ok {
				return
			}
			if err := s.ProcessMedia(mediaID); err != nil {
				s.logger.Warnf("[ScanPostProcess] worker#%d 处理失败 media_id=%s err=%v", idx, mediaID, err)
			}
		}
	}
}

// EnqueueMedia 把单条 Media 入队（非阻塞，队列满时丢弃并记录日志）
func (s *ScanPostProcessService) EnqueueMedia(mediaID string) {
	if mediaID == "" {
		return
	}
	select {
	case s.queue <- mediaID:
	default:
		s.logger.Warnf("[ScanPostProcess] 队列已满，丢弃 media_id=%s", mediaID)
	}
}

// EnqueueLibrary 把整库 Media 入队（异步执行，不阻塞调用方）
// 该方法用于 ScannerService.SetOnScanComplete 回调。
func (s *ScanPostProcessService) EnqueueLibrary(libraryID string) (int, error) {
	if s.mediaRepo == nil {
		return 0, errors.New("mediaRepo 未注入")
	}
	medias, err := s.mediaRepo.ListByLibraryID(libraryID)
	if err != nil {
		return 0, err
	}
	for _, m := range medias {
		s.EnqueueMedia(m.ID)
	}
	s.logger.Infof("[ScanPostProcess] 入队整库 library_id=%s count=%d", libraryID, len(medias))
	return len(medias), nil
}

// ============================ 单条处理（核心） ============================

// ProcessMedia 处理单条 Media。流程：标记 running -> 识别 -> 归类 -> 命名 -> Upsert。
// 返回的 error 仅指底层数据库或 mediaRepo 异常；阶段内的 AI 调用失败会降级为 partial 状态。
func (s *ScanPostProcessService) ProcessMedia(mediaID string) error {
	if mediaID == "" {
		return errors.New("mediaID 为空")
	}
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		// Media 已被删除（如重建索引/删除媒体库等竞态情况），清理关联的分类记录
		_ = s.repo.DeleteByMediaID(mediaID)
		return nil // 静默跳过，不记 WARN 日志
	}
	if media == nil || media.ID == "" {
		return nil // 同理静默跳过
	}

	// 占位记录（确保前端在 running 期也能看到状态）
	_ = s.repo.Upsert(&model.MediaClassification{
		MediaID:   media.ID,
		LibraryID: media.LibraryID,
		Status:    model.ClassificationStatusRunning,
	})

	classification := &model.MediaClassification{
		MediaID:     media.ID,
		LibraryID:   media.LibraryID,
		NamingStyle: s.cfg.NamingStyle,
	}

	// =============== 阶段 1：识别 ===============
	parsed := s.identify(media, classification)

	// =============== 阶段 2：归类 ===============
	s.classify(media, parsed, classification)

	// =============== 阶段 3：命名映射 ===============
	s.naming(media, parsed, classification)

	// =============== 状态收尾 ===============
	now := time.Now()
	classification.ProcessedAt = &now
	if classification.Status == "" {
		classification.Status = model.ClassificationStatusProcessed
	}
	return s.repo.Upsert(classification)
}

// ProcessBatch 批量处理；返回成功数量
func (s *ScanPostProcessService) ProcessBatch(mediaIDs []string) (int, error) {
	ok := 0
	for _, id := range mediaIDs {
		if err := s.ProcessMedia(id); err != nil {
			s.logger.Warnf("[ScanPostProcess] 批量处理失败 media_id=%s err=%v", id, err)
			continue
		}
		ok++
	}
	return ok, nil
}

// ReprocessLibrary 整库重跑：清理旧记录并重新入队
//
// libraryID 为空时进入「全部媒体库」模式：枚举所有 Library 并逐个重跑，
// 满足傻瓜化「一键重新识别全部」的需求。
func (s *ScanPostProcessService) ReprocessLibrary(libraryID string, async bool) (int, error) {
	// ---------- 模式 A：全部媒体库（傻瓜化一键重跑） ----------
	if libraryID == "" {
		if s.libraryRepo == nil {
			return 0, errors.New("libraryRepo 未注入，无法执行全库重跑")
		}
		libs, err := s.libraryRepo.List()
		if err != nil {
			return 0, err
		}
		total := 0
		for _, lib := range libs {
			n, err := s.ReprocessLibrary(lib.ID, async)
			if err != nil {
				s.logger.Warnf("[ScanPostProcess] 重跑媒体库失败 library_id=%s name=%s err=%v",
					lib.ID, lib.Name, err)
				continue
			}
			total += n
		}
		s.logger.Infof("[ScanPostProcess] 全部媒体库重跑完成 libraries=%d total_media=%d async=%v",
			len(libs), total, async)
		return total, nil
	}

	// ---------- 模式 B：指定单个媒体库 ----------
	if _, err := s.repo.DeleteByLibraryID(libraryID); err != nil {
		s.logger.Warnf("[ScanPostProcess] 清理旧记录失败 library_id=%s err=%v", libraryID, err)
	}
	if async {
		return s.EnqueueLibrary(libraryID)
	}
	medias, err := s.mediaRepo.ListByLibraryID(libraryID)
	if err != nil {
		return 0, err
	}
	ids := make([]string, 0, len(medias))
	for _, m := range medias {
		ids = append(ids, m.ID)
	}
	return s.ProcessBatch(ids)
}

// ManualCorrect 单条人工修正：用户在前端修改识别结果（标题/年份/TMDb/类别）后保存。
//
// 安全约束：
//   - 仅写入 media_classifications 表，不会触发任何磁盘文件改动；
//   - 状态会被强制置为 processed，并打上 manual_corrected 标记（记录在 error_msg 字段头部）；
//   - 用户传入字段为空时保留旧值（避免误清空）。
type ManualCorrectInput struct {
	MediaID  string
	Title    string // 可选
	Year     int    // 可选；0 表示不更新
	TMDbID   int    // 可选；0 表示不更新
	IMDbID   string // 可选
	Category string // 可选 movie / tvshow / anime / ...
	Region   string // 可选
}

// ManualCorrect 执行单条人工修正
func (s *ScanPostProcessService) ManualCorrect(in ManualCorrectInput) (*model.MediaClassification, error) {
	if in.MediaID == "" {
		return nil, errors.New("media_id 不能为空")
	}
	existing, err := s.repo.FindByMediaID(in.MediaID)
	if err != nil {
		return nil, fmt.Errorf("分类记录不存在: %w", err)
	}
	if in.Title != "" {
		existing.ParsedTitle = strings.TrimSpace(in.Title)
	}
	if in.Year > 0 {
		existing.ParsedYear = in.Year
	}
	if in.TMDbID > 0 {
		existing.ParsedTMDbID = in.TMDbID
	}
	if in.IMDbID != "" {
		existing.ParsedIMDbID = strings.TrimSpace(in.IMDbID)
	}
	if in.Category != "" {
		existing.Category = strings.TrimSpace(in.Category)
	}
	if in.Region != "" {
		existing.Region = strings.TrimSpace(in.Region)
	}
	// 人工确认 → 置信度直接拉满，状态置为已处理
	existing.Confidence = 1.0
	existing.Status = model.ClassificationStatusProcessed
	existing.ErrorMsg = "[manual_corrected] " + time.Now().Format(time.RFC3339)
	now := time.Now()
	existing.ProcessedAt = &now
	if err := s.repo.Upsert(existing); err != nil {
		return nil, err
	}
	s.logger.Infof("[ScanPostProcess] 人工修正完成 media_id=%s title=%q tmdb_id=%d",
		in.MediaID, existing.ParsedTitle, existing.ParsedTMDbID)
	return existing, nil
}

// ============================ Stage 1: 识别 ============================

// identify 综合规则解析 + 数据库已有字段 + 必要时 AI Fallback。
// 返回融合后的结果（最终采用），并把过程结果写入 classification。
func (s *ScanPostProcessService) identify(media *model.Media, c *model.MediaClassification) scanPostParsed {
	// 优先级：DB 中已有字段 > 规则解析 > AI Fallback
	parsed := scanPostParsed{
		Title:     media.Title,
		TitleAlt:  media.OrigTitle,
		Year:      media.Year,
		TMDbID:    media.TMDbID,
		IMDbID:    media.IMDbID,
		MediaType: media.MediaType,
		Season:    media.SeasonNum,
		Episode:   media.EpisodeNum,
	}

	// 规则解析（基于文件路径补全）
	srcName := filepath.Base(media.FilePath)
	if srcName != "" {
		ruleParsed := ParseMovieFilename(srcName)
		if parsed.Title == "" {
			parsed.Title = ruleParsed.Title
		}
		if parsed.TitleAlt == "" {
			parsed.TitleAlt = ruleParsed.TitleAlt
		}
		if parsed.Year == 0 {
			parsed.Year = ruleParsed.Year
		}
		if parsed.TMDbID == 0 {
			parsed.TMDbID = ruleParsed.TMDbID
		}
		if parsed.IMDbID == "" {
			parsed.IMDbID = ruleParsed.IMDbID
		}
	}

	// 计算规则置信度
	confidence := scoreClassificationConfidence(parsed, media)
	c.Confidence = confidence

	// AI Fallback：
	//   - 默认模式：规则置信度 < AIConfidenceThreshold 才走 AI
	//   - 全自动托管 (AutoPilot)：强制每条都调 AI，不依赖阈值
	forceAI := s.cfg.ForceAIIdentify || (s.ai != nil && s.ai.IsAutoPilotEnabled())
	shouldRunAI := s.cfg.EnableAIFallback &&
		(forceAI || confidence < s.cfg.AIConfidenceThreshold) &&
		s.ai != nil && s.ai.IsEnabled() &&
		srcName != ""

	if shouldRunAI {
		if aiOut, raw, err := s.callAIIdentify(srcName, parsed); err == nil && aiOut != nil {
			c.AIInvoked = true
			c.AIRawResponse = raw
			// 记录使用的 AI 服务商与模型（来自 AI 配置中心当前生效项）
			c.AIProvider = s.ai.Provider()
			c.AIModel = s.ai.Model()
			// AutoPilot 下以 AI 为准，默认模式仅在 AI 自评高于规则时采纳
			accept := forceAI || aiOut.Confidence > confidence
			if accept {
				if aiOut.Title != "" {
					parsed.Title = aiOut.Title
				}
				if aiOut.TitleAlt != "" {
					parsed.TitleAlt = aiOut.TitleAlt
				}
				if aiOut.Year > 0 {
					parsed.Year = aiOut.Year
				}
				if aiOut.TMDbID > 0 {
					parsed.TMDbID = aiOut.TMDbID
				}
				if aiOut.IMDbID != "" {
					parsed.IMDbID = aiOut.IMDbID
				}
				if aiOut.MediaType != "" && aiOut.MediaType != "unknown" {
					parsed.MediaType = aiOut.MediaType
				}
				if aiOut.Season > 0 {
					parsed.Season = aiOut.Season
				}
				if aiOut.Episode > 0 {
					parsed.Episode = aiOut.Episode
				}
				if aiOut.Confidence > 0 {
					c.Confidence = aiOut.Confidence
				}
			}
		} else if err != nil {
			// AI 失败 → 不影响整体流程，仅降级为 partial
			s.logger.Warnf("[ScanPostProcess] AI 识别失败 file=%s err=%v", srcName, err)
			c.Status = model.ClassificationStatusPartial
		}
	}

	// 最终值写入 classification 的解析字段
	c.ParsedTitle = parsed.Title
	c.ParsedTitleAlt = parsed.TitleAlt
	c.ParsedYear = parsed.Year
	c.ParsedTMDbID = parsed.TMDbID
	c.ParsedIMDbID = parsed.IMDbID

	return parsed
}

// scoreClassificationConfidence 基于解析结果与 Media 状态计算置信度（0~1）
//
// 规则（与 SmartRename 的 scoreConfidence 思路一致但加入"DB 字段加权"）：
//   - 有 TMDbID                    +0.5
//   - 有 IMDbID                    +0.3
//   - 有 Year                      +0.15
//   - Title 含中文                 +0.1
//   - 已被刮削（scrape_status=scraped/manual） +0.2
func scoreClassificationConfidence(p scanPostParsed, media *model.Media) float64 {
	score := 0.0
	if p.TMDbID > 0 {
		score += 0.5
	}
	if p.IMDbID != "" {
		score += 0.3
	}
	if p.Year > 0 {
		score += 0.15
	}
	if containsCJK(p.Title) {
		score += 0.1
	}
	if media != nil && (media.ScrapeStatus == "scraped" || media.ScrapeStatus == "manual") {
		score += 0.2
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// containsCJK 是否包含中日韩字符（粗略判断中文标题）
func containsCJK(s string) bool {
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
			(r >= 0x3040 && r <= 0x309F) || // Hiragana
			(r >= 0x30A0 && r <= 0x30FF) || // Katakana
			(r >= 0xAC00 && r <= 0xD7AF) { // Hangul
			return true
		}
	}
	return false
}

// callAIIdentify 调 AI 做兜底识别。复用 SmartRename 的 prompt 逻辑（保持一致）。
// AI 服务商/模型完全跟随 AI 配置中心当前生效项（管理员在 AI 配置 Tab 切换后立即生效）。
func (s *ScanPostProcessService) callAIIdentify(srcName string, hint scanPostParsed) (*SmartRenameAIResult, string, error) {
	if s.ai == nil || !s.ai.IsEnabled() {
		return nil, "", errors.New("AI 服务未启用")
	}
	if s.logger != nil {
		s.logger.Debugf("扫描后处理 AI 识别启动: provider=%s model=%s file=%s",
			s.ai.Provider(), s.ai.Model(), srcName)
	}
	sysPrompt := `你是影视命名识别专家。根据用户给出的文件名，识别影视作品的元数据。

严格按以下 JSON Schema 返回，不要任何额外解释、不要 Markdown 代码块：
{
  "title": "中文主标题（无则填英文/原始）",
  "title_alt": "英文别名（可空）",
  "year": 1999,
  "tmdb_id": 0,
  "imdb_id": "tt1234567（可空）",
  "media_type": "movie|episode|unknown",
  "season": 0,
  "episode": 0,
  "confidence": 0.85
}

约束：
- 仅识别已知影视作品，不要编造；不确定的字段留空 / 0 / unknown。
- confidence 取 0.0~1.0；若文件名信息严重不足，给 < 0.5。
- 不要输出文件名中明显的 PT 发布组、编码标签等噪声。`

	userPrompt := fmt.Sprintf(`文件名：%s
当前规则解析（可能不准）：title=%q title_alt=%q year=%d tmdb=%d imdb=%q
请按 JSON Schema 返回最终识别结果。`,
		srcName, hint.Title, hint.TitleAlt, hint.Year, hint.TMDbID, hint.IMDbID)

	raw, err := s.ai.ChatCompletion(sysPrompt, userPrompt, 0.2, 512)
	if err != nil {
		return nil, "", err
	}
	cleaned := stripJSONFence(raw) // 复用 smart_rename.go 中的同包函数
	var out SmartRenameAIResult
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		return nil, raw, fmt.Errorf("AI 返回 JSON 解析失败: %w", err)
	}
	if out.Confidence <= 0 || out.Confidence > 1 {
		out.Confidence = 0.5
	}
	return &out, raw, nil
}

// ============================ Stage 2: 归类 ============================

// classify 推导 Category / Region / Decade / GenreTags / LanguageTag / QualityTier / VirtualPath
func (s *ScanPostProcessService) classify(media *model.Media, parsed scanPostParsed, c *model.MediaClassification) {
	c.Category = inferCategory(media)
	c.Region = inferRegion(media)
	c.Decade = inferDecade(parsed.Year)
	c.GenreTags = normalizeGenres(media.Genres, media.Tags)
	c.LanguageTag = inferLanguage(media)
	c.QualityTier = inferQualityTier(media)
	c.VirtualPath = buildVirtualPath(c.Category, c.Region, c.Decade, c.GenreTags)
}

// inferCategory 推导一级类别
func inferCategory(m *model.Media) string {
	// 优先：剧集类型
	if m.MediaType == "episode" || m.SeriesID != "" {
		return "tvshow"
	}
	// 番号字段非空 → 成人
	if strings.TrimSpace(m.Num) != "" {
		return "adult"
	}
	genres := strings.ToLower(m.Genres)
	tags := strings.ToLower(m.Tags)
	all := genres + "," + tags

	switch {
	case strings.Contains(all, "动画") || strings.Contains(all, "animation") || strings.Contains(all, "anime"):
		return "anime"
	case strings.Contains(all, "纪录") || strings.Contains(all, "documentary"):
		return "documentary"
	case strings.Contains(all, "综艺") || strings.Contains(all, "talk-show") || strings.Contains(all, "reality"):
		return "variety"
	case strings.Contains(all, "音乐") || strings.Contains(all, "music") || strings.Contains(all, "concert"):
		return "music"
	}
	return "movie"
}

// inferRegion 按 country / country_code / language 推导地区桶
func inferRegion(m *model.Media) string {
	if cc := strings.ToUpper(strings.TrimSpace(m.CountryCode)); cc != "" {
		switch cc {
		case "CN", "HK", "TW", "JP", "KR", "US", "IN":
			return cc
		}
	}
	country := strings.ToLower(m.Country)
	if country != "" {
		switch {
		case strings.Contains(country, "中国大陆") || strings.Contains(country, "china") || strings.Contains(country, "cn"):
			return "CN"
		case strings.Contains(country, "香港") || strings.Contains(country, "hong kong"):
			return "HK"
		case strings.Contains(country, "台湾") || strings.Contains(country, "taiwan"):
			return "TW"
		case strings.Contains(country, "日本") || strings.Contains(country, "japan"):
			return "JP"
		case strings.Contains(country, "韩国") || strings.Contains(country, "korea"):
			return "KR"
		case strings.Contains(country, "美国") || strings.Contains(country, "united states") || strings.Contains(country, "usa"):
			return "US"
		case strings.Contains(country, "印度") || strings.Contains(country, "india"):
			return "IN"
		case strings.Contains(country, "英国") || strings.Contains(country, "法国") ||
			strings.Contains(country, "德国") || strings.Contains(country, "意大利") ||
			strings.Contains(country, "uk") || strings.Contains(country, "france") ||
			strings.Contains(country, "germany") || strings.Contains(country, "italy"):
			return "EU"
		}
	}
	// 回退：按 language
	lang := strings.ToLower(m.Language)
	switch {
	case strings.Contains(lang, "zh") || strings.Contains(lang, "汉语") || strings.Contains(lang, "中文"):
		return "CN"
	case strings.Contains(lang, "ja") || strings.Contains(lang, "日"):
		return "JP"
	case strings.Contains(lang, "ko") || strings.Contains(lang, "韩"):
		return "KR"
	case strings.Contains(lang, "en"):
		return "US"
	}
	return "OTHER"
}

// inferDecade 按年份推导年代档位（如 2020s）
func inferDecade(year int) string {
	if year <= 0 {
		return ""
	}
	d := (year / 10) * 10
	return fmt.Sprintf("%ds", d)
}

// inferLanguage 推导语言短码
func inferLanguage(m *model.Media) string {
	lang := strings.ToLower(strings.TrimSpace(m.Language))
	if lang == "" {
		return ""
	}
	// 取常见前缀
	for _, prefix := range []string{"zh", "ja", "ko", "en", "fr", "de", "es", "ru", "th", "vi"} {
		if strings.HasPrefix(lang, prefix) {
			return prefix
		}
	}
	return lang
}

// inferQualityTier 推导画质档（基于 resolution 字段，回退到文件大小启发）
func inferQualityTier(m *model.Media) string {
	res := strings.ToLower(strings.TrimSpace(m.Resolution))
	switch {
	case strings.Contains(res, "2160") || strings.Contains(res, "4k"):
		return "4K"
	case strings.Contains(res, "1080"):
		return "1080p"
	case strings.Contains(res, "720"):
		return "720p"
	case res != "":
		return "SD"
	}
	// 回退：按文件大小（粗略）
	gb := m.FileSize / (1024 * 1024 * 1024)
	switch {
	case gb >= 25:
		return "4K"
	case gb >= 4:
		return "1080p"
	case gb >= 1:
		return "720p"
	case gb > 0:
		return "SD"
	}
	return ""
}

// normalizeGenres 合并 genres + tags 并去重排序，逗号分隔
func normalizeGenres(genres, tags string) string {
	seen := map[string]bool{}
	out := make([]string, 0, 8)
	for _, raw := range []string{genres, tags} {
		if raw == "" {
			continue
		}
		// 同时处理逗号 / 中文逗号 / 分号 分隔
		for _, sep := range []string{"，", ";", "；", "|", "/"} {
			raw = strings.ReplaceAll(raw, sep, ",")
		}
		for _, item := range strings.Split(raw, ",") {
			item = strings.TrimSpace(item)
			if item == "" || seen[item] {
				continue
			}
			seen[item] = true
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return ""
	}
	// 简单稳定，不排序，保留来源顺序
	return strings.Join(out, ",")
}

// buildVirtualPath 构造虚拟分类路径（用于前端显示与潜在的虚拟文件夹组织）
//
// 形如：电影/华语/2020s/科幻,动作   或   剧集/日本/2010s/动画
func buildVirtualPath(category, region, decade, genreTags string) string {
	parts := make([]string, 0, 4)
	parts = append(parts, categoryDisplay(category))
	parts = append(parts, regionDisplay(region))
	if decade != "" {
		parts = append(parts, decade)
	}
	primary := primaryGenre(genreTags)
	if primary != "" {
		parts = append(parts, primary)
	}
	// 过滤空段
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			clean = append(clean, p)
		}
	}
	return strings.Join(clean, "/")
}

func categoryDisplay(c string) string {
	switch c {
	case "movie":
		return "电影"
	case "tvshow":
		return "剧集"
	case "anime":
		return "动画"
	case "documentary":
		return "纪录片"
	case "variety":
		return "综艺"
	case "music":
		return "音乐"
	case "adult":
		return "成人"
	}
	return "其他"
}

func regionDisplay(r string) string {
	switch r {
	case "CN", "HK", "TW":
		return "华语"
	case "JP":
		return "日本"
	case "KR":
		return "韩国"
	case "US":
		return "欧美"
	case "EU":
		return "欧美"
	case "IN":
		return "印度"
	}
	return "其他"
}

func primaryGenre(genreTags string) string {
	if genreTags == "" {
		return ""
	}
	for _, g := range strings.Split(genreTags, ",") {
		g = strings.TrimSpace(g)
		if g != "" {
			return g
		}
	}
	return ""
}

// ============================ Stage 3: 命名映射（仅 DB） ============================

// naming 生成 Jellyfin/Emby 风格的建议命名 / 子目录 / 完整路径。
// 全程仅写入 classification 字段，不动磁盘。
func (s *ScanPostProcessService) naming(media *model.Media, parsed scanPostParsed, c *model.MediaClassification) {
	style := strings.ToLower(strings.TrimSpace(c.NamingStyle))
	if style != NamingStyleJellyfin && style != NamingStylePlex {
		style = s.cfg.NamingStyle
	}
	c.NamingStyle = style

	ext := strings.ToLower(filepath.Ext(media.FilePath))
	title := sanitizeTitle(parsed.Title) // 复用 smart_rename.go 同包函数
	if title == "" {
		title = sanitizeTitle(strings.TrimSuffix(filepath.Base(media.FilePath), ext))
	}

	var suggestedName, suggestedDir string
	if isEpisode(media, parsed) {
		// 剧集：Title (Year) S01E02 [tmdbid-xxx].ext
		base := fmt.Sprintf("%s S%02dE%02d", title, parsed.Season, parsed.Episode)
		if parsed.Year > 0 {
			base = fmt.Sprintf("%s (%d) S%02dE%02d", title, parsed.Year, parsed.Season, parsed.Episode)
		}
		base += renderIDTag(style, parsed.TMDbID, parsed.IMDbID)
		suggestedName = collapseWhitespace(base) + ext

		// 子目录：Title (Year)/Season 01
		seriesDir := title
		if parsed.Year > 0 {
			seriesDir = fmt.Sprintf("%s (%d)", title, parsed.Year)
		}
		seasonDir := fmt.Sprintf("Season %02d", parsed.Season)
		suggestedDir = filepath.ToSlash(filepath.Join(seriesDir, seasonDir))
	} else {
		// 电影：Title (Year) [tmdbid-xxx].ext
		yearTag := ""
		if parsed.Year > 0 {
			yearTag = fmt.Sprintf(" (%d)", parsed.Year)
		}
		idTag := renderIDTag(style, parsed.TMDbID, parsed.IMDbID)
		suggestedName = collapseWhitespace(title+yearTag+idTag) + ext

		// 子目录：Title (Year)
		movieDir := title
		if parsed.Year > 0 {
			movieDir = fmt.Sprintf("%s (%d)", title, parsed.Year)
		}
		suggestedDir = filepath.ToSlash(movieDir)
	}

	c.SuggestedName = suggestedName
	c.SuggestedDir = suggestedDir

	// 完整路径（仅作展示，参考 Library 的主路径；真实落盘绝不发生）
	if s.libraryRepo != nil && media.LibraryID != "" {
		if lib, err := s.libraryRepo.FindByID(media.LibraryID); err == nil && lib != nil {
			root := lib.Path
			if root != "" {
				c.SuggestedFullPath = filepath.ToSlash(filepath.Join(root, suggestedDir, suggestedName))
			}
		}
	}
	if c.SuggestedFullPath == "" {
		// 回退：使用源文件目录
		c.SuggestedFullPath = filepath.ToSlash(filepath.Join(filepath.Dir(media.FilePath), suggestedDir, suggestedName))
	}
}

// isEpisode 判定是否按剧集格式输出
func isEpisode(media *model.Media, p scanPostParsed) bool {
	if media.MediaType == "episode" && media.SeasonNum > 0 && media.EpisodeNum > 0 {
		return true
	}
	if p.MediaType == "episode" && p.Season > 0 && p.Episode > 0 {
		return true
	}
	return false
}
