package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== SmartRename 智能扫描重命名服务 ====================
//
// 模块职责：
//   P0 识别评分：基于规则的命名解析（复用 ParseMovieFilename）+ 文件落库匹配 -> 置信度
//   P1 AI Fallback：低置信度时调用 AIService.ChatCompletion 补全识别
//   P2 命名模板：Jellyfin/Emby `[tmdbid-xxx]` 默认；可切 Plex `{tmdb-xxx}`
//   P3 关联资源：同名 .nfo/.srt/.ass/.sub/-poster.jpg/-fanart.jpg/-thumb.jpg/.idx 等随主迁移
//   P4 安全检测：跨卷、目标已存在、磁盘空间、硬链接计数、相对路径越界
//   P5 事务执行：plan + journal，按条目串行原子，遇错回滚已成功部分
//   P6 默认 dry-run：仅 confirm=true 才真正动盘
//
// 该服务不修改 FileManagerService 现有 API，独立运转。

// ================================ Constants ================================

// 常见视频扩展名
var smartRenameVideoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".wmv": true,
	".flv": true, ".webm": true, ".m4v": true, ".ts": true, ".m2ts": true,
	".mpg": true, ".mpeg": true, ".rmvb": true, ".rm": true, ".3gp": true,
	".vob": true, ".iso": true,
}

// 关联资源扩展名（不带前缀的尾缀） -> kind
var smartRenameRelatedExts = map[string]string{
	".nfo":  "nfo",
	".srt":  "subtitle",
	".ass":  "subtitle",
	".ssa":  "subtitle",
	".sub":  "subtitle",
	".idx":  "subtitle",
	".vtt":  "subtitle",
	".sup":  "subtitle",
	".lrc":  "subtitle",
	".chs":  "subtitle",
	".cht":  "subtitle",
	".chi":  "subtitle",
	".eng":  "subtitle",
	".jpg":  "image",
	".jpeg": "image",
	".png":  "image",
	".webp": "image",
	".tbn":  "image",
}

// 媒体伴生图片的命名后缀（去扩展名后的最末段）
var smartRenameImageSuffix = map[string]string{
	"poster":    "poster",
	"fanart":    "fanart",
	"thumb":     "thumb",
	"banner":    "banner",
	"clearlogo": "clearlogo",
	"landscape": "landscape",
	"disc":      "disc",
	"backdrop":  "fanart",
}

// 命名模板风格
const (
	NamingStyleJellyfin = "jellyfin" // Title (Year) [tmdbid-12345].ext
	NamingStylePlex     = "plex"     // Title (Year) {tmdb-12345}.ext
)

// 安全 / 标题字符清洗：去除 NTFS / ext4 禁用字符
var smartRenameUnsafeCharPattern = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// ================================ Types ====================================

// SmartRenameConfig 服务级配置（来自全局 config 注入）
type SmartRenameConfig struct {
	DefaultStyle          string  // jellyfin / plex
	AIConfidenceThreshold float64 // 触发 AI 阈值（默认 0.7）
	EnableAIFallback      bool    // 是否启用 AI Fallback
	MaxScanFiles          int     // 单次扫描最大文件数（防爆，默认 5000）
	SafeRoots             []string // 安全根目录白名单：若非空，所有改名必须在白名单内
	RequireConfirm        bool    // 是否强制 confirm（即使前端传 false）
}

// DefaultSmartRenameConfig 默认配置
func DefaultSmartRenameConfig() SmartRenameConfig {
	return SmartRenameConfig{
		DefaultStyle:          NamingStyleJellyfin,
		AIConfidenceThreshold: 0.7,
		EnableAIFallback:      true,
		MaxScanFiles:          5000,
		SafeRoots:             nil,
		RequireConfirm:        true,
	}
}

// SmartRenameRelatedFile 单个关联资源
type SmartRenameRelatedFile struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"` // nfo / subtitle / poster / fanart / thumb / other
}

// SmartRenameSafetyReport 安全检测结果
type SmartRenameSafetyReport struct {
	OK             bool     `json:"ok"`
	CrossVolume    bool     `json:"cross_volume"`     // 跨卷
	TargetExists   bool     `json:"target_exists"`    // 目标已存在
	HardlinkCount  uint64   `json:"hardlink_count"`   // 硬链接数（>1 警告）
	OutsideSafeRoot bool    `json:"outside_safe_root"`
	NotEnoughSpace bool     `json:"not_enough_space"`
	Issues         []string `json:"issues"` // 人类可读问题列表
}

// SmartRenameAIResult AI Fallback 输出结构（强制 JSON Schema）
type SmartRenameAIResult struct {
	Title      string  `json:"title"`
	TitleAlt   string  `json:"title_alt"`
	Year       int     `json:"year"`
	TMDbID     int     `json:"tmdb_id"`
	IMDbID     string  `json:"imdb_id"`
	MediaType  string  `json:"media_type"`  // movie / episode / unknown
	Season     int     `json:"season"`
	Episode    int     `json:"episode"`
	Confidence float64 `json:"confidence"` // AI 自评 0~1
}

// ScanInput 扫描入参
type ScanInput struct {
	RootPath              string   // 待扫描根目录（绝对路径）
	LibraryID             string   // 可选：限定到媒体库
	NamingStyle           string   // 可选：jellyfin / plex
	Template              string   // 可选：自定义模板（空则按 style 取默认）
	EnableAIFallback      *bool    // 可选：覆盖默认
	AIConfidenceThreshold *float64 // 可选：覆盖默认
	SafeRoots             []string // 可选：本次扫描覆盖的安全根
	CreatedBy             string   // 当前用户 ID
}

// ExecuteInput 执行入参
type ExecuteInput struct {
	PlanID    string   // 必填
	Confirm   bool     // 必须 true 才真正落盘
	ItemIDs   []string // 可选：仅执行指定条目（空表示全部 pending+safety_ok 条目）
	IgnoreSafety bool  // 可选：用户显式忽略安全警告（默认 false）
}

// SmartRenameService 智能扫描重命名服务
type SmartRenameService struct {
	repo       *repository.RenameRepo
	mediaRepo  *repository.MediaRepo
	seriesRepo *repository.SeriesRepo
	ai         *AIService
	cfg        SmartRenameConfig
	logger     *zap.SugaredLogger

	// dry-run 模拟的目标占用集合，避免同一规划内两个条目目标冲突
	mu sync.Mutex
}

// NewSmartRenameService 构造服务
func NewSmartRenameService(
	repo *repository.RenameRepo,
	mediaRepo *repository.MediaRepo,
	seriesRepo *repository.SeriesRepo,
	ai *AIService,
	cfg SmartRenameConfig,
	logger *zap.SugaredLogger,
) *SmartRenameService {
	if cfg.AIConfidenceThreshold <= 0 {
		cfg.AIConfidenceThreshold = 0.7
	}
	if cfg.MaxScanFiles <= 0 {
		cfg.MaxScanFiles = 5000
	}
	if cfg.DefaultStyle == "" {
		cfg.DefaultStyle = NamingStyleJellyfin
	}
	return &SmartRenameService{
		repo:       repo,
		mediaRepo:  mediaRepo,
		seriesRepo: seriesRepo,
		ai:         ai,
		cfg:        cfg,
		logger:     logger,
	}
}

// ================================ P0+P1: 扫描 + 规划 ==========================

// Scan 扫描目录、识别每个视频文件、生成规划任务（draft 状态）。
//
// 不会动磁盘，仅在 DB 中创建 RenamePlan + 一组 RenamePlanItem。
func (s *SmartRenameService) Scan(in ScanInput) (*model.RenamePlan, error) {
	if in.RootPath == "" {
		return nil, errors.New("root_path 必填")
	}
	absRoot, err := filepath.Abs(in.RootPath)
	if err != nil {
		return nil, fmt.Errorf("根目录非法: %w", err)
	}
	st, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("根目录不可访问: %w", err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("根目录不是目录: %s", absRoot)
	}

	// 合并配置
	style := strings.ToLower(strings.TrimSpace(in.NamingStyle))
	if style != NamingStyleJellyfin && style != NamingStylePlex {
		style = s.cfg.DefaultStyle
	}
	enableAI := s.cfg.EnableAIFallback
	if in.EnableAIFallback != nil {
		enableAI = *in.EnableAIFallback
	}
	threshold := s.cfg.AIConfidenceThreshold
	if in.AIConfidenceThreshold != nil && *in.AIConfidenceThreshold > 0 {
		threshold = *in.AIConfidenceThreshold
	}
	safeRoots := in.SafeRoots
	if len(safeRoots) == 0 {
		safeRoots = s.cfg.SafeRoots
	}

	// 1) 扫描视频文件
	videoFiles, err := s.collectVideoFiles(absRoot)
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}
	s.logger.Infof("[SmartRename] 扫描完成：发现 %d 个视频文件 root=%s", len(videoFiles), absRoot)

	// 2) 持久化 Plan
	planID := uuid.New().String()
	plan := &model.RenamePlan{
		ID:                    planID,
		LibraryID:             in.LibraryID,
		RootPath:              absRoot,
		NamingStyle:           style,
		Template:              in.Template,
		EnableAIFallback:      enableAI,
		AIConfidenceThreshold: threshold,
		Status:                model.RenamePlanStatusDraft,
		DryRun:                true,
		TotalItems:            len(videoFiles),
		CreatedBy:             in.CreatedBy,
	}
	if err := s.repo.CreatePlan(plan); err != nil {
		return nil, fmt.Errorf("持久化规划失败: %w", err)
	}

	// 3) 逐个识别 + 生成条目
	items := make([]model.RenamePlanItem, 0, len(videoFiles))
	usedTargets := map[string]bool{}
	stats := struct {
		need, skipped, unsafe, ai int
	}{}

	for _, src := range videoFiles {
		item, err := s.buildItem(planID, src, style, in.Template, enableAI, threshold, safeRoots, usedTargets)
		if err != nil {
			s.logger.Warnf("[SmartRename] 生成条目失败 file=%s: %v", src, err)
			item = &model.RenamePlanItem{
				ID:         uuid.New().String(),
				PlanID:     planID,
				SourcePath: src,
				SourceName: filepath.Base(src),
				Status:     model.RenameItemStatusFailed,
				ErrorMsg:   err.Error(),
			}
		}
		if item.AIInvoked {
			stats.ai++
		}
		switch item.Status {
		case model.RenameItemStatusPending:
			stats.need++
		case model.RenameItemStatusSkipped:
			stats.skipped++
		case model.RenameItemStatusUnsafe:
			stats.unsafe++
		}
		items = append(items, *item)
	}

	if err := s.repo.CreateItems(items); err != nil {
		return nil, fmt.Errorf("持久化条目失败: %w", err)
	}

	// 4) 更新统计
	plan.NeedRename = stats.need
	plan.SkippedItems = stats.skipped
	plan.UnsafeItems = stats.unsafe
	plan.AIInvocations = stats.ai
	if err := s.repo.UpdatePlanFields(planID, map[string]interface{}{
		"need_rename":    stats.need,
		"skipped_items":  stats.skipped,
		"unsafe_items":   stats.unsafe,
		"ai_invocations": stats.ai,
	}); err != nil {
		s.logger.Warnf("[SmartRename] 更新规划统计失败: %v", err)
	}

	// 重新加载（带 items 返回）
	return s.repo.GetPlanWithItems(planID)
}

// collectVideoFiles 递归扫描目录，仅收集视频文件
func (s *SmartRenameService) collectVideoFiles(root string) ([]string, error) {
	var files []string
	maxFiles := s.cfg.MaxScanFiles
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			s.logger.Warnf("[SmartRename] 跳过不可访问路径 %s: %v", p, err)
			return nil
		}
		if d.IsDir() {
			// 忽略以 . / @eaDir 开头的目录
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "@eaDir" || name == "$RECYCLE.BIN" || name == "System Volume Information" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if !smartRenameVideoExts[ext] {
			return nil
		}
		files = append(files, p)
		if maxFiles > 0 && len(files) >= maxFiles {
			return errStopWalk
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopWalk) {
		return nil, err
	}
	return files, nil
}

var errStopWalk = errors.New("stop walk")

// buildItem 针对单个视频文件生成 RenamePlanItem
func (s *SmartRenameService) buildItem(
	planID, src, style, customTpl string,
	enableAI bool, aiThreshold float64,
	safeRoots []string,
	usedTargets map[string]bool,
) (*model.RenamePlanItem, error) {
	srcName := filepath.Base(src)
	item := &model.RenamePlanItem{
		ID:         uuid.New().String(),
		PlanID:     planID,
		SourcePath: src,
		SourceName: srcName,
		Status:     model.RenameItemStatusPending,
	}

	// === P0: 规则解析 + 置信度评分 ===
	parsed := ParseMovieFilename(srcName)
	conf := s.scoreConfidence(parsed)

	// === 关联落库的 Media (若有)：优先用 DB 信息覆盖 ===
	mediaInfo, _ := s.lookupMediaByPath(src)
	if mediaInfo != nil {
		item.MediaID = mediaInfo.ID
		// 用 DB 中的精确字段强化识别
		if mediaInfo.Title != "" {
			parsed.Title = mediaInfo.Title
		}
		if mediaInfo.OrigTitle != "" && parsed.TitleAlt == "" {
			parsed.TitleAlt = mediaInfo.OrigTitle
		}
		if mediaInfo.Year > 0 {
			parsed.Year = mediaInfo.Year
		}
		if mediaInfo.TMDbID > 0 {
			parsed.TMDbID = mediaInfo.TMDbID
		}
		if mediaInfo.IMDbID != "" {
			parsed.IMDbID = mediaInfo.IMDbID
		}
		// DB 已有准确的 TMDb 识别 -> 置信度拉满
		if mediaInfo.TMDbID > 0 {
			conf = 0.99
		} // 否则保持规则评分，让 AI 有机会纠正文件名不含真实标题的情况
		item.MediaType = mediaInfo.MediaType
		item.SeasonNum = mediaInfo.SeasonNum
		item.EpisodeNum = mediaInfo.EpisodeNum
	}

	// === P1: AI Fallback ===
	// 规则置信度不足且 AI 可用时触发。当 AI 返回结果的置信度高于规则时，
	// AI 结果完全覆盖规则结果（解决文件名不含真实标题、标题在目录名中的场景）。
	if enableAI && conf < aiThreshold && s.ai != nil && s.ai.IsEnabled() {
		// 把父目录名也传给 AI，增加识别上下文（如文件名不含标题但目录名包含剧名）
		parentDir := filepath.Base(filepath.Dir(src))
		aiRes, aiRaw, aiErr := s.callAIFallback(srcName, parentDir, parsed)
		item.AIInvoked = true
		item.AIRawResponse = aiRaw
		if aiErr == nil && aiRes != nil {
			// AI 置信度高于规则时，完全采纳 AI 结果
			if aiRes.Confidence >= conf {
				if aiRes.Title != "" {
					parsed.Title = aiRes.Title
				}
				if aiRes.TitleAlt != "" {
					parsed.TitleAlt = aiRes.TitleAlt
				}
				if aiRes.Year > 0 {
					parsed.Year = aiRes.Year
				}
				if aiRes.TMDbID > 0 {
					parsed.TMDbID = aiRes.TMDbID
				}
				if aiRes.IMDbID != "" {
					parsed.IMDbID = aiRes.IMDbID
				}
				if aiRes.MediaType != "" {
					item.MediaType = aiRes.MediaType
				}
				if aiRes.Season > 0 {
					item.SeasonNum = aiRes.Season
				}
				if aiRes.Episode > 0 {
					item.EpisodeNum = aiRes.Episode
				}
				conf = aiRes.Confidence
			} else {
				// AI 置信度低于规则：仅在原字段为空时补充
				if parsed.Title == "" && aiRes.Title != "" {
					parsed.Title = aiRes.Title
				}
				if parsed.TitleAlt == "" && aiRes.TitleAlt != "" {
					parsed.TitleAlt = aiRes.TitleAlt
				}
				if parsed.Year == 0 && aiRes.Year > 0 {
					parsed.Year = aiRes.Year
				}
				if parsed.TMDbID == 0 && aiRes.TMDbID > 0 {
					parsed.TMDbID = aiRes.TMDbID
				}
				if parsed.IMDbID == "" && aiRes.IMDbID != "" {
					parsed.IMDbID = aiRes.IMDbID
				}
				if item.MediaType == "" && aiRes.MediaType != "" {
					item.MediaType = aiRes.MediaType
				}
				if item.SeasonNum == 0 && aiRes.Season > 0 {
					item.SeasonNum = aiRes.Season
				}
				if item.EpisodeNum == 0 && aiRes.Episode > 0 {
					item.EpisodeNum = aiRes.Episode
				}
				if aiRes.Confidence > conf {
					conf = aiRes.Confidence
				}
			}
		} else if aiErr != nil {
			s.logger.Warnf("[SmartRename] AI Fallback 失败 file=%s: %v", srcName, aiErr)
		}
	}

	// 兜底：未识别 Title 时使用文件名主体
	if parsed.Title == "" {
		parsed.Title = strings.TrimSuffix(srcName, filepath.Ext(srcName))
	}
	if item.MediaType == "" {
		// 默认按电影；若文件名中检测出 SxxExx 则改为 episode（粗略再扫一次）
		if s, e := extractSxxExx(srcName); s > 0 && e > 0 {
			item.MediaType = "episode"
			item.SeasonNum = s
			item.EpisodeNum = e
		} else {
			item.MediaType = "movie"
		}
	}

	item.ParsedTitle = parsed.Title
	item.ParsedTitleAlt = parsed.TitleAlt
	item.ParsedYear = parsed.Year
	item.ParsedTMDbID = parsed.TMDbID
	item.ParsedIMDbID = parsed.IMDbID
	item.Confidence = conf

	// === P2: 渲染目标命名 ===
	targetName, err := s.renderTargetName(style, customTpl, parsed, item)
	if err != nil {
		return nil, err
	}
	targetPath := filepath.Join(filepath.Dir(src), targetName)
	item.TargetName = targetName
	item.TargetPath = targetPath

	// 如果目标名等于源名 -> 跳过（已是目标格式）
	if filepath.Base(src) == targetName {
		item.Status = model.RenameItemStatusSkipped
		item.SafetyOK = true
		item.SafetyNote = "已是目标命名"
		return item, nil
	}

	// === P3: 关联资源 ===
	relatedRaw, relatedTargets := s.collectRelatedFiles(src, targetPath)
	if buf, err := json.Marshal(relatedRaw); err == nil {
		item.RelatedFilesJSON = string(buf)
	}

	// === P4: 安全检测 ===
	allTargets := append([]string{targetPath}, relatedTargets...)
	safety := s.checkSafety(src, targetPath, allTargets, safeRoots, usedTargets)
	if buf, err := json.Marshal(safety); err == nil {
		item.SafetyJSON = string(buf)
	}
	item.SafetyOK = safety.OK
	if !safety.OK {
		item.SafetyNote = strings.Join(safety.Issues, "; ")
		item.Status = model.RenameItemStatusUnsafe
	} else {
		// 标记目标已占用
		for _, t := range allTargets {
			usedTargets[strings.ToLower(t)] = true
		}
	}

	return item, nil
}

// scoreConfidence 基于解析结果计算 0~1 的置信度
//
// 评分模型（最高 1.0）：
//   - 有 TMDbID：+0.5（强证据）
//   - 有 IMDbID：+0.4
//   - Title 非空且非全 ASCII 噪声：+0.25
//   - Year > 0：+0.2
//   - TitleAlt 非空：+0.05
func (s *SmartRenameService) scoreConfidence(p ParsedFilename) float64 {
	score := 0.0
	if p.TMDbID > 0 {
		score += 0.5
	}
	if p.IMDbID != "" {
		score += 0.4
	}
	if p.Title != "" && len([]rune(strings.TrimSpace(p.Title))) >= 2 {
		score += 0.25
	}
	if p.Year > 0 {
		score += 0.2
	}
	if p.TitleAlt != "" {
		score += 0.05
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// lookupMediaByPath 用源路径反查 Media（不强求一定能查到）
func (s *SmartRenameService) lookupMediaByPath(src string) (*model.Media, error) {
	if s.mediaRepo == nil {
		return nil, nil
	}
	// repo_media.go 没有公开的 ByPath；用底层 DB 查询
	// 这里通过 ListFilesAdvanced 不合适，直接以 FilePath 精确匹配
	// 为了不强依赖额外方法，采用 keyword 兜底
	// （后续可在 MediaRepo 加一个 GetByFilePath 优化）
	media, _, err := s.mediaRepo.ListFilesAdvanced(1, 1, "", "", src, "", "", nil)
	if err != nil || len(media) == 0 {
		return nil, err
	}
	if media[0].FilePath == src {
		return &media[0], nil
	}
	return nil, nil
}

// extractSxxExx 从字符串中找出 SxxExx 集数
func extractSxxExx(s string) (int, int) {
	re := regexp.MustCompile(`(?i)\bS(\d{1,3})[\.\s]?E(\d{1,3})\b`)
	m := re.FindStringSubmatch(s)
	if len(m) < 3 {
		return 0, 0
	}
	se, _ := strconv.Atoi(m[1])
	ep, _ := strconv.Atoi(m[2])
	return se, ep
}

// ================================ P1: AI Fallback ============================

// callAIFallback 调用 LLM 还原元数据
func (s *SmartRenameService) callAIFallback(srcName, parentDir string, hint ParsedFilename) (*SmartRenameAIResult, string, error) {
	if s.ai == nil || !s.ai.IsEnabled() {
		return nil, "", errors.New("AI 服务未启用")
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
所在目录：%s
当前规则解析（可能不准）：title=%q title_alt=%q year=%d tmdb=%d imdb=%q
请按 JSON Schema 返回最终识别结果。`,
		srcName, parentDir, hint.Title, hint.TitleAlt, hint.Year, hint.TMDbID, hint.IMDbID)

	raw, err := s.ai.ChatCompletion(sysPrompt, userPrompt, 0.2, 512)
	if err != nil {
		return nil, "", err
	}

	// 清洗：模型可能仍然带 ```json fence
	cleaned := stripJSONFence(raw)
	var out SmartRenameAIResult
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		return nil, raw, fmt.Errorf("AI 返回 JSON 解析失败: %w", err)
	}
	if out.Confidence <= 0 || out.Confidence > 1 {
		out.Confidence = 0.5
	}
	return &out, raw, nil
}

// stripJSONFence 剥离 Markdown 代码围栏
func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	// 去除前导非 { 字符（模型可能加额外引导词）
	if idx := strings.Index(s, "{"); idx > 0 {
		s = s[idx:]
	}
	if idx := strings.LastIndex(s, "}"); idx >= 0 && idx < len(s)-1 {
		s = s[:idx+1]
	}
	return s
}

// ================================ P2: 命名模板 ===============================

// renderTargetName 渲染目标文件名（不带目录）
func (s *SmartRenameService) renderTargetName(style, customTpl string, p ParsedFilename, item *model.RenamePlanItem) (string, error) {
	ext := filepath.Ext(item.SourceName)
	// 标题清洗：去掉 NTFS 禁止字符 + 折叠空白
	title := sanitizeTitle(p.Title)
	if title == "" {
		title = sanitizeTitle(strings.TrimSuffix(item.SourceName, ext))
	}

	// 剧集格式：Title S01E02.ext（Jellyfin/Plex 等都识别）
	if item.MediaType == "episode" && item.SeasonNum > 0 && item.EpisodeNum > 0 {
		base := fmt.Sprintf("%s S%02dE%02d", title, item.SeasonNum, item.EpisodeNum)
		if p.Year > 0 {
			base = fmt.Sprintf("%s (%d) S%02dE%02d", title, p.Year, item.SeasonNum, item.EpisodeNum)
		}
		// ID 标签
		base += renderIDTag(style, p.TMDbID, p.IMDbID)
		return base + strings.ToLower(ext), nil
	}

	// 电影格式
	year := ""
	if p.Year > 0 {
		year = fmt.Sprintf(" (%d)", p.Year)
	}

	// 优先采用用户自定义模板（支持占位符 {title}/{year}/{tmdb}/{imdb}/{ext}）
	if strings.TrimSpace(customTpl) != "" {
		out := customTpl
		out = strings.ReplaceAll(out, "{title}", title)
		if p.Year > 0 {
			out = strings.ReplaceAll(out, "{year}", strconv.Itoa(p.Year))
			out = strings.ReplaceAll(out, "({year})", fmt.Sprintf("(%d)", p.Year))
		} else {
			out = strings.ReplaceAll(out, "{year}", "")
			out = strings.ReplaceAll(out, "({year})", "")
		}
		if p.TMDbID > 0 {
			out = strings.ReplaceAll(out, "{tmdb}", strconv.Itoa(p.TMDbID))
		} else {
			out = strings.ReplaceAll(out, "{tmdb}", "")
		}
		out = strings.ReplaceAll(out, "{imdb}", p.IMDbID)
		out = strings.ReplaceAll(out, "{ext}", strings.TrimPrefix(strings.ToLower(ext), "."))
		// 折叠多余空白
		out = collapseWhitespace(out)
		// 如果模板没指定扩展名，自动补
		if !strings.HasSuffix(strings.ToLower(out), strings.ToLower(ext)) {
			out += strings.ToLower(ext)
		}
		return out, nil
	}

	base := fmt.Sprintf("%s%s%s", title, year, renderIDTag(style, p.TMDbID, p.IMDbID))
	base = collapseWhitespace(base)
	return base + strings.ToLower(ext), nil
}

// renderIDTag 按风格生成 ID 标签
func renderIDTag(style string, tmdbID int, imdbID string) string {
	if tmdbID == 0 && imdbID == "" {
		return ""
	}
	switch style {
	case NamingStylePlex:
		// Plex: {tmdb-12345} / {imdb-tt123}
		if tmdbID > 0 {
			return fmt.Sprintf(" {tmdb-%d}", tmdbID)
		}
		return fmt.Sprintf(" {imdb-%s}", imdbID)
	default:
		// Jellyfin/Emby: [tmdbid-12345] / [imdbid-tt123]
		if tmdbID > 0 {
			return fmt.Sprintf(" [tmdbid-%d]", tmdbID)
		}
		return fmt.Sprintf(" [imdbid-%s]", imdbID)
	}
}

// sanitizeTitle 标题中的 NTFS/ext4 禁用字符替换为空格
func sanitizeTitle(s string) string {
	s = smartRenameUnsafeCharPattern.ReplaceAllString(s, " ")
	return collapseWhitespace(s)
}

// collapseWhitespace 把多空白合一并 trim
func collapseWhitespace(s string) string {
	return strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(s, " "))
}

// ================================ P3: 关联资源 ===============================

// collectRelatedFiles 收集同名/同前缀的关联资源；返回明细 + 目标路径列表
func (s *SmartRenameService) collectRelatedFiles(srcVideo, targetVideo string) ([]SmartRenameRelatedFile, []string) {
	dir := filepath.Dir(srcVideo)
	srcBase := strings.TrimSuffix(filepath.Base(srcVideo), filepath.Ext(srcVideo))
	tgtBase := strings.TrimSuffix(filepath.Base(targetVideo), filepath.Ext(targetVideo))
	tgtDir := filepath.Dir(targetVideo)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	var related []SmartRenameRelatedFile
	var targets []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// 跳过自身
		if name == filepath.Base(srcVideo) {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		stem := strings.TrimSuffix(name, filepath.Ext(name))

		// 1) 完全同名前缀（srcBase.ext / srcBase.zh.srt / srcBase-poster.jpg ...）
		if !strings.HasPrefix(stem, srcBase) {
			continue
		}
		suffix := stem[len(srcBase):] // 比如 "-poster" 或 ".zh"
		// 决定 kind
		kind, ok := smartRenameRelatedExts[ext]
		if !ok {
			continue
		}
		// 对于 image 类型，区分海报 / 背景 / 缩略
		if kind == "image" {
			// 去掉前导分隔符
			suffixCore := strings.TrimLeft(suffix, "-._")
			suffixCore = strings.ToLower(suffixCore)
			if k, ok2 := smartRenameImageSuffix[suffixCore]; ok2 {
				kind = k
			} else if suffixCore == "" {
				kind = "thumb" // 同名图（无后缀）
			} else {
				kind = "image"
			}
		}

		newName := tgtBase + suffix + ext
		newPath := filepath.Join(tgtDir, newName)
		related = append(related, SmartRenameRelatedFile{
			Source: filepath.Join(dir, name),
			Target: newPath,
			Kind:   kind,
		})
		targets = append(targets, newPath)
	}
	// 排序，便于前端展示稳定
	sort.SliceStable(related, func(i, j int) bool {
		return related[i].Source < related[j].Source
	})
	return related, targets
}

// ================================ P4: 安全检测 ===============================

// checkSafety 对源/目标进行安全审查
func (s *SmartRenameService) checkSafety(src, tgt string, allTargets, safeRoots []string, usedTargets map[string]bool) SmartRenameSafetyReport {
	report := SmartRenameSafetyReport{OK: true}

	// 1) 跨卷检测（Windows 看盘符；POSIX 看 device id）
	if isCrossVolume(src, tgt) {
		report.CrossVolume = true
		report.Issues = append(report.Issues, "源与目标位于不同卷/盘符")
	}

	// 2) 目标已存在
	for _, t := range allTargets {
		if t == src {
			continue
		}
		if _, err := os.Stat(t); err == nil {
			report.TargetExists = true
			report.Issues = append(report.Issues, "目标已存在: "+filepath.Base(t))
			break
		}
		// 同一规划中目标已被占用
		if usedTargets[strings.ToLower(t)] {
			report.TargetExists = true
			report.Issues = append(report.Issues, "目标与同一规划内其他条目冲突: "+filepath.Base(t))
			break
		}
	}

	// 3) 硬链接计数（POSIX）
	if hlc := getHardlinkCount(src); hlc > 1 {
		report.HardlinkCount = hlc
		report.Issues = append(report.Issues, fmt.Sprintf("源文件硬链接数=%d，重命名可能影响其他位置", hlc))
	}

	// 4) 安全根白名单
	if len(safeRoots) > 0 {
		ok := false
		for _, root := range safeRoots {
			absRoot, _ := filepath.Abs(root)
			absTgt, _ := filepath.Abs(tgt)
			if absRoot != "" && (strings.HasPrefix(strings.ToLower(absTgt), strings.ToLower(absRoot+string(os.PathSeparator))) ||
				strings.EqualFold(absRoot, absTgt)) {
				ok = true
				break
			}
		}
		if !ok {
			report.OutsideSafeRoot = true
			report.Issues = append(report.Issues, "目标位于安全根白名单之外")
		}
	}

	// 5) 磁盘空间（粗略：只在跨卷时检查；同卷重命名不消耗空间）
	if report.CrossVolume {
		if !hasEnoughSpace(filepath.Dir(tgt), getFileSize(src)) {
			report.NotEnoughSpace = true
			report.Issues = append(report.Issues, "目标卷可用空间不足")
		}
	}

	report.OK = len(report.Issues) == 0
	return report
}

// ================================ P5+P6: 执行（plan -> journal） ===============

// Execute 落盘执行（confirm=false 仅做 dry-run 校验，不真正动盘）
func (s *SmartRenameService) Execute(in ExecuteInput) (*model.RenamePlan, error) {
	plan, err := s.repo.GetPlanWithItems(in.PlanID)
	if err != nil {
		return nil, fmt.Errorf("规划不存在: %w", err)
	}
	if plan.Status != model.RenamePlanStatusDraft && plan.Status != model.RenamePlanStatusFailed {
		return nil, fmt.Errorf("规划状态不允许执行: %s", plan.Status)
	}

	// 强制 confirm
	if s.cfg.RequireConfirm && !in.Confirm {
		// dry-run：把 plan 状态保留 draft，但更新一次校验时间
		_ = s.repo.UpdatePlanFields(plan.ID, map[string]interface{}{
			"dry_run": true,
		})
		return plan, nil
	}

	// 标记执行中
	now := time.Now()
	_ = s.repo.UpdatePlanFields(plan.ID, map[string]interface{}{
		"status":      model.RenamePlanStatusExecuting,
		"dry_run":     false,
		"executed_at": &now,
	})

	// 过滤需要执行的条目
	itemFilter := map[string]bool{}
	for _, id := range in.ItemIDs {
		itemFilter[id] = true
	}

	executed := 0
	failed := 0
	executor := newRenameExecutor(s.repo, s.logger)

	for i := range plan.Items {
		it := &plan.Items[i]
		if len(itemFilter) > 0 && !itemFilter[it.ID] {
			continue
		}
		if it.Excluded {
			continue
		}
		if it.Status != model.RenameItemStatusPending && it.Status != model.RenameItemStatusFailed {
			continue
		}
		if !it.SafetyOK && !in.IgnoreSafety {
			continue
		}
		// 取 OverrideName 覆盖
		if it.OverrideName != "" {
			it.TargetName = it.OverrideName
			it.TargetPath = filepath.Join(filepath.Dir(it.SourcePath), it.OverrideName)
		}

		var related []SmartRenameRelatedFile
		if it.RelatedFilesJSON != "" {
			_ = json.Unmarshal([]byte(it.RelatedFilesJSON), &related)
		}

		if err := executor.executeItem(plan.ID, it, related); err != nil {
			failed++
			it.Status = model.RenameItemStatusFailed
			it.ErrorMsg = err.Error()
			_ = s.repo.UpdateItem(it)
			s.logger.Errorf("[SmartRename] 执行条目失败 plan=%s item=%s: %v", plan.ID, it.ID, err)
			continue
		}
		executed++
		it.Status = model.RenameItemStatusExecuted
		_ = s.repo.UpdateItem(it)
	}

	completedAt := time.Now()
	finalStatus := model.RenamePlanStatusCompleted
	if failed > 0 && executed == 0 {
		finalStatus = model.RenamePlanStatusFailed
	}
	_ = s.repo.UpdatePlanFields(plan.ID, map[string]interface{}{
		"status":         finalStatus,
		"executed_items": executed,
		"failed_items":   failed,
		"completed_at":   &completedAt,
	})
	s.logger.Infof("[SmartRename] 规划执行完成 plan=%s executed=%d failed=%d", plan.ID, executed, failed)

	return s.repo.GetPlanWithItems(plan.ID)
}

// Rollback 回滚一次规划（按 journal 倒序逆操作）
func (s *SmartRenameService) Rollback(planID string) (*model.RenamePlan, error) {
	plan, err := s.repo.GetPlanWithItems(planID)
	if err != nil {
		return nil, err
	}
	if plan.Status != model.RenamePlanStatusCompleted &&
		plan.Status != model.RenamePlanStatusFailed {
		return nil, fmt.Errorf("规划状态不可回滚: %s", plan.Status)
	}

	journals, err := s.repo.ListJournalByPlan(planID)
	if err != nil {
		return nil, err
	}
	executor := newRenameExecutor(s.repo, s.logger)
	if err := executor.rollback(journals); err != nil {
		return nil, err
	}

	// 把对应条目标记为 reverted
	for i := range plan.Items {
		it := &plan.Items[i]
		if it.Status == model.RenameItemStatusExecuted {
			it.Status = model.RenameItemStatusReverted
			_ = s.repo.UpdateItem(it)
		}
	}

	_ = s.repo.UpdatePlanFields(planID, map[string]interface{}{
		"status": model.RenamePlanStatusRolledBack,
	})
	return s.repo.GetPlanWithItems(planID)
}

// Cancel 取消（仅 draft 状态可取消）
func (s *SmartRenameService) Cancel(planID string) error {
	plan, err := s.repo.GetPlan(planID)
	if err != nil {
		return err
	}
	if plan.Status != model.RenamePlanStatusDraft {
		return fmt.Errorf("仅 draft 状态可取消，当前: %s", plan.Status)
	}
	return s.repo.UpdatePlanFields(planID, map[string]interface{}{
		"status": model.RenamePlanStatusCanceled,
	})
}

// UpdateItemOverride 用户修改单条目标名 / 排除标记
func (s *SmartRenameService) UpdateItemOverride(itemID, overrideName string, excluded *bool) (*model.RenamePlanItem, error) {
	it, err := s.repo.GetItem(itemID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if overrideName != "" {
		updates["override_name"] = overrideName
		updates["target_name"] = overrideName
		updates["target_path"] = filepath.Join(filepath.Dir(it.SourcePath), overrideName)
	}
	if excluded != nil {
		updates["excluded"] = *excluded
	}
	if len(updates) == 0 {
		return it, nil
	}
	if err := s.repo.UpdateItemFields(itemID, updates); err != nil {
		return nil, err
	}
	return s.repo.GetItem(itemID)
}

// ListPlans 列出
func (s *SmartRenameService) ListPlans(page, size int) ([]model.RenamePlan, int64, error) {
	return s.repo.ListPlans(page, size)
}

// GetPlan 取详情
func (s *SmartRenameService) GetPlan(planID string) (*model.RenamePlan, error) {
	return s.repo.GetPlanWithItems(planID)
}

// DeletePlan 删除（仅非执行中）
func (s *SmartRenameService) DeletePlan(planID string) error {
	plan, err := s.repo.GetPlan(planID)
	if err != nil {
		return err
	}
	if plan.Status == model.RenamePlanStatusExecuting {
		return errors.New("执行中的规划不能删除")
	}
	return s.repo.DeletePlan(planID)
}
