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
//   阶段 4 · 硬链接（hardlink） - 可选：按虚拟路径+建议命名在输出目录中创建硬链接
//
// 安全约束（强保证）：
//   - 阶段 1~3 不导入 os 包，不会调用任何 os.Rename/Create/Remove/MkdirAll 等磁盘写入 API；
//   - 阶段 4 仅在 Library.OrganizeOutputDir 非空时触发，通过 OrganizeHardlinkService 执行；
//   - 硬链接不修改/移动源文件，仅创建指向源文件的硬链接；
//   - 所有分类结果写入 media_classifications 表；
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
	seriesRepo  *repository.SeriesRepo
	ai          *AIService
	cfg         ScanPostProcessConfig
	logger      *zap.SugaredLogger
	wsHub       *WSHub

	// 硬链接整理（可选，注入后自动在分类完成后创建硬链接）
	organizeHardlink *OrganizeHardlinkService

	// 目录级 AI 识别缓存：仅对 AI 已确认的剧集根目录复用，避免电影合集目录被误复用成同一标题。
	aiIdentifyCache   map[string]*SmartRenameAIResult
	aiIdentifyCacheMu sync.Mutex

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
		repo:            repo,
		mediaRepo:       mediaRepo,
		libraryRepo:     libraryRepo,
		ai:              ai,
		cfg:             cfg,
		logger:          logger,
		aiIdentifyCache: make(map[string]*SmartRenameAIResult),
		queue:           make(chan string, cfg.QueueSize),
		stopCh:          make(chan struct{}),
	}
}

// SetOrganizeHardlinkService 注入硬链接整理服务（延迟注入，可选）
func (s *ScanPostProcessService) SetOrganizeHardlinkService(ohl *OrganizeHardlinkService) {
	s.organizeHardlink = ohl
}

// SetSeriesRepo 注入剧集仓储，用于媒体类型纠正后重算合集统计。
func (s *ScanPostProcessService) SetSeriesRepo(seriesRepo *repository.SeriesRepo) {
	s.seriesRepo = seriesRepo
}

// SetWSHub 注入 WebSocket Hub，用于整理完成后通知前端刷新。
func (s *ScanPostProcessService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
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
	ids, err := s.ListUnprocessedMediaIDs(libraryID)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		s.EnqueueMedia(id)
	}
	s.logger.Infof("[ScanPostProcess] 入队媒体库待整理项 library_id=%s count=%d", libraryID, len(ids))
	return len(ids), nil
}

// ListUnprocessedMediaIDs 返回尚未完成 AI 整理的媒体 ID。
// 普通扫描只调用该方法处理新增项；显式重跑会先删除旧分类记录，因此仍会全量处理。
func (s *ScanPostProcessService) ListUnprocessedMediaIDs(libraryID string) ([]string, error) {
	if s.repo == nil {
		return nil, errors.New("scanClassificationRepo 未注入")
	}
	return s.repo.ListUnprocessedMediaIDsByLibrary(libraryID)
}

// ProcessUnprocessedLibraryWithProgress 只处理指定媒体库尚未整理过的媒体。
func (s *ScanPostProcessService) ProcessUnprocessedLibraryWithProgress(libraryID string, onProgress func(current, total, okCount int)) (int, int, error) {
	ids, err := s.ListUnprocessedMediaIDs(libraryID)
	if err != nil {
		return 0, 0, err
	}
	ok, err := s.ProcessBatchWithProgress(ids, onProgress)
	return ok, len(ids), err
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

	// 读取所属媒体库的"AI 自动整理"模式（off / rule_only / ai_assisted）。
	// 该字段在用户创建/编辑媒体库时勾选，是判定是否调用 AI 的最高优先级开关。
	mode := model.AutoOrganizeAIAssisted
	if media.LibraryID != "" && s.libraryRepo != nil {
		if lib, errLib := s.libraryRepo.FindByID(media.LibraryID); errLib == nil && lib != nil && lib.AutoOrganizeMode != "" {
			if model.IsValidAutoOrganizeMode(lib.AutoOrganizeMode) {
				mode = lib.AutoOrganizeMode
			}
		}
	}

	// mode=off：用户在该库关闭了自动整理。写一条 skipped 占位便于审计/前端展示。
	if mode == model.AutoOrganizeOff {
		now := time.Now()
		return s.repo.Upsert(&model.MediaClassification{
			MediaID:     media.ID,
			LibraryID:   media.LibraryID,
			NamingStyle: s.cfg.NamingStyle,
			Status:      model.ClassificationStatusSkipped,
			ErrorMsg:    "媒体库已关闭 AI 自动整理（auto_organize_mode=off）",
			ProcessedAt: &now,
		})
	}
	// rule_only：仅规则识别 + 归类 + 命名建议，禁止任何 AI 调用。
	allowAI := mode == model.AutoOrganizeAIAssisted

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
	parsed := s.identify(media, classification, allowAI)

	// =============== 阶段 2：归类 ===============
	s.classify(media, parsed, classification)

	// =============== 阶段 3：命名映射 ===============
	s.naming(media, parsed, classification)

	// 将可信识别结果同步回 Media 主表，避免媒体墙/TMDb 刮削继续使用扫描阶段的脏文件名。
	s.syncParsedToMedia(media, parsed, classification)

	// =============== 状态收尾 ===============
	now := time.Now()
	classification.ProcessedAt = &now
	if classification.Status == "" {
		classification.Status = model.ClassificationStatusProcessed
	}
	if err := s.repo.Upsert(classification); err != nil {
		return err
	}

	// =============== 阶段 4：硬链接落盘（可选） ===============
	// 直接复用当前 media/classification，避免每集再额外查 media + classification 两次 DB。
	if s.organizeHardlink != nil && media.LibraryID != "" && s.libraryRepo != nil &&
		classification.Status == model.ClassificationStatusProcessed {
		if lib, errLib := s.libraryRepo.FindByID(media.LibraryID); errLib == nil && lib != nil {
			outputDir := strings.TrimSpace(lib.OrganizeOutputDir)
			if outputDir != "" {
				if r, errHL := s.organizeHardlink.ProcessResolvedMediaHardlink(media, classification, outputDir); errHL != nil {
					s.logger.Warnf("[ScanPostProcess] 硬链接失败 media_id=%s err=%v", media.ID, errHL)
				} else if r != nil && r.ErrorMsg != "" {
					s.logger.Warnf("[ScanPostProcess] 硬链接警告 media_id=%s msg=%s", media.ID, r.ErrorMsg)
				} else if r != nil && s.wsHub != nil {
					s.wsHub.BroadcastEvent(EventLibraryUpdated, &LibraryChangedData{
						LibraryID:   media.LibraryID,
						LibraryName: lib.Name,
						Action:      "ai_organized",
						Message:     "AI 整理数据已同步",
					})
				}
			}
		}
	}

	return nil
}

// ProcessBatch 批量处理；返回成功数量
func (s *ScanPostProcessService) ProcessBatch(mediaIDs []string) (int, error) {
	return s.ProcessBatchWithProgress(mediaIDs, nil)
}

// ProcessBatchWithProgress 批量处理并回调当前阶段进度。
func (s *ScanPostProcessService) ProcessBatchWithProgress(mediaIDs []string, onProgress func(current, total, okCount int)) (int, error) {
	ok := 0
	total := len(mediaIDs)
	for i, id := range mediaIDs {
		if err := s.ProcessMedia(id); err != nil {
			s.logger.Warnf("[ScanPostProcess] 批量处理失败 media_id=%s err=%v", id, err)
		} else {
			ok++
		}
		if onProgress != nil {
			onProgress(i+1, total, ok)
		}
	}
	return ok, nil
}

// CancelResult Cancel 方法的返回结构
type CancelResult struct {
	Drained      int   `json:"drained"`       // 从内存队列丢弃的待处理任务数
	Marked       int64 `json:"marked"`        // 数据库中由 pending/running 改为 failed 的记录数
	StillRunning bool  `json:"still_running"` // 是否有 worker 仍在处理已取出的任务（最多 1 条短时收尾）
}

// Cancel 停止当前进行中的扫描归类任务。
//
// 实现策略（务实派）：
//   - 内存队列：非阻塞地 drain 掉所有待消费 mediaID（最多 QueueSize 次）；
//   - 数据库：把对应范围内 pending/running 的分类记录改为 failed，并打上「[cancelled]」标记；
//   - 正在被 worker 处理的那一条不强行打断（一条耗时通常 < 数秒），让其自然结束；
//     若它收尾时仍写回 processed，则视同已完成，不再被覆盖；这是可接受的边缘情况。
//
// libraryID 为空 → 影响所有库；非空 → 仅影响该库。
func (s *ScanPostProcessService) Cancel(libraryID string) (CancelResult, error) {
	res := CancelResult{}

	// 1) Drain 内存队列
	drained := 0
DrainLoop:
	for {
		select {
		case <-s.queue:
			drained++
		default:
			break DrainLoop
		}
	}
	res.Drained = drained

	// 2) 把 DB 中 pending/running 状态的记录改写为 failed（带取消标记）
	marked, err := s.repo.MarkPendingCancelled(libraryID)
	if err != nil {
		s.logger.Warnf("[ScanPostProcess] 取消时回写 DB 失败 library_id=%s err=%v", libraryID, err)
		return res, err
	}
	res.Marked = marked

	// 3) 标记是否还有 worker 在处理（最多 1 条；用 worker 数判断）
	s.mu.Lock()
	res.StillRunning = s.started && s.cfg.Workers > 0 && len(s.queue) == 0
	s.mu.Unlock()

	s.logger.Infof("[ScanPostProcess] 已取消扫描 library_id=%s drained=%d marked_failed=%d", libraryID, drained, marked)
	return res, nil
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
func (s *ScanPostProcessService) identify(media *model.Media, c *model.MediaClassification, allowAI bool) scanPostParsed {
	srcName := filepath.Base(media.FilePath)
	ruleParsed := ParsedFilename{}
	if srcName != "" {
		ruleParsed = ParseMovieFilename(srcName)
	}

	// 优先级：可信 DB 字段 > 规则解析 > AI Fallback。
	// 扫描初始写入的 Media.Title 可能是 DBD-Raws 这类发布组脏标题，不能无条件压过规则/AI。
	parsed := scanPostParsed{
		Year:      media.Year,
		TMDbID:    media.TMDbID,
		IMDbID:    media.IMDbID,
		MediaType: media.MediaType,
		Season:    media.SeasonNum,
		Episode:   media.EpisodeNum,
	}
	if isTrustedMediaTitle(media.Title, media) {
		parsed.Title = media.Title
		parsed.TitleAlt = media.OrigTitle
	} else {
		parsed.Title = ruleParsed.Title
		parsed.TitleAlt = ruleParsed.TitleAlt
		if media.Title != "" && media.OrigTitle == "" {
			parsed.TitleAlt = media.Title
		}
	}

	// 规则解析（基于文件路径补全）
	if srcName != "" {
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

	// 剧集优先使用源根目录稳定剧名（如 D:\\test\\拔作岛 或 [2.5次元的诱惑]），避免每集 AI 输出不同标题。
	episodeLike := parsed.MediaType == "episode" || parsed.Season > 0 || parsed.Episode > 0
	stableLocked := false
	if episodeLike {
		if s.organizeHardlink != nil {
			if stableTitle := s.organizeHardlink.stableEpisodeTitle(media, nil); stableTitle != "" {
				parsed.Title = stableTitle
				// 同一源根目录下的剧集/NCOP/NCED 等特殊篇统一锁定父目录；
				// 缺集号的特殊文件后续会落到 _unsorted，不再为每个文件反复调用 AI。
				stableLocked = true
			}
		} else if stableTitle := stableEpisodeTitleFromPath(media.FilePath); stableTitle != "" {
			parsed.Title = stableTitle
			stableLocked = true
		}
	}

	// 计算规则置信度
	confidence := scoreClassificationConfidence(parsed, media)
	if stableLocked && confidence < 0.85 {
		confidence = 0.85
	}
	c.Confidence = confidence

	// AI 识别：
	//   - ai_assisted 模式下不再按规则置信度跳过 AI，所有媒体都进入 AI 识别路径；
	//   - 仅“AI 已确认是剧集”的同一源根目录允许复用目录级缓存；
	//   - rule_only / off 仍按媒体库设置禁用 AI。
	shouldRunAI := allowAI &&
		s.cfg.EnableAIFallback &&
		s.ai != nil && s.ai.IsEnabled() &&
		srcName != ""

	cacheKey := ""
	if shouldRunAI {
		cacheKey = s.aiIdentifyCacheKey(media, parsed)
		if cacheKey != "" {
			if cached := s.getAIIdentifyCache(cacheKey); cached != nil && cached.MediaType == "episode" {
				s.applyAIIdentifyResult(&parsed, cached, false)
				if cached.Confidence > c.Confidence {
					c.Confidence = cached.Confidence
				}
				shouldRunAI = false
			}
		}
	}

	if shouldRunAI {
		parentDir := filepath.Base(filepath.Dir(media.FilePath))
		if aiOut, raw, err := s.callAIIdentify(srcName, parentDir, parsed); err == nil && aiOut != nil {
			c.AIInvoked = true
			c.AIRawResponse = raw
			// 记录使用的 AI 服务商与模型（来自 AI 配置中心当前生效项）
			c.AIProvider = s.ai.Provider()
			c.AIModel = s.ai.Model()
			// AI 辅助模式下以 AI 识别为准，不再要求 AI 置信度高于规则置信度。
			s.applyAIIdentifyResult(&parsed, aiOut, true)
			if aiOut.Confidence > 0 {
				c.Confidence = aiOut.Confidence
			}
			if cacheKey != "" && aiOut.MediaType == "episode" {
				s.setAIIdentifyCache(cacheKey, aiOut)
			}
		} else if err != nil {
			// AI 失败 → 不影响整体流程；
			// 但若规则解析已经拿到核心字段（标题 + 年份 / 季集），
			// 就保留 processed 状态，避免列表上一片"待修正"假阳性。
			s.logger.Warnf("[ScanPostProcess] AI 识别失败 file=%s err=%v", srcName, err)
			ruleHasCoreFields := parsed.Title != "" &&
				(parsed.Year > 0 || parsed.Season > 0 || parsed.Episode > 0)
			if !ruleHasCoreFields {
				c.Status = model.ClassificationStatusPartial
			}
		}
	}

	// AI 可能对同一番剧不同集输出中/英/别名不同；最终仍以源根目录稳定剧名为准。
	if parsed.MediaType == "episode" || parsed.Season > 0 || parsed.Episode > 0 {
		if s.organizeHardlink != nil {
			if stableTitle := s.organizeHardlink.stableEpisodeTitle(media, nil); stableTitle != "" {
				parsed.Title = stableTitle
			}
		} else if stableTitle := stableEpisodeTitleFromPath(media.FilePath); stableTitle != "" {
			parsed.Title = stableTitle
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

func isTrustedMediaTitle(title string, media *model.Media) bool {
	t := strings.TrimSpace(title)
	if t == "" {
		return false
	}
	if media != nil && (media.ScrapeStatus == "scraped" || media.ScrapeStatus == "manual") {
		return true
	}
	if media != nil && (media.TMDbID > 0 || media.IMDbID != "") {
		return true
	}
	lower := strings.ToLower(t)
	noiseTokens := []string{"dbd-raws", "bdrip", "webrip", "web-dl", "hevc", "x264", "x265", "flac", "1080p", "720p", "2160p", "全集", "简繁", "外挂"}
	for _, token := range noiseTokens {
		if strings.Contains(lower, token) {
			return false
		}
	}
	// 标题里出现多段方括号通常是发布组文件名，不是作品名。
	if strings.Count(t, "[")+strings.Count(t, "【") >= 2 {
		return false
	}
	return true
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

// callAIIdentify 调 AI 做兜底识别。使用紧凑 JSON 提示词，降低 deepseek-v4-flash 输出 token。
// AI 服务商/模型完全跟随 AI 配置中心当前生效项（管理员在 AI 配置 Tab 切换后立即生效）。
func (s *ScanPostProcessService) callAIIdentify(srcName string, parentDir string, hint scanPostParsed) (*SmartRenameAIResult, string, error) {
	if s.ai == nil || !s.ai.IsEnabled() {
		return nil, "", errors.New("AI 服务未启用")
	}
	if s.logger != nil {
		s.logger.Debugf("扫描后处理 AI 识别启动: provider=%s model=%s file=%s",
			s.ai.Provider(), s.ai.Model(), srcName)
	}
	sysPrompt := `影视文件名识别器。只输出单行JSON。禁止解释/Markdown/<think>/多候选。必须区分电影和剧集：电影/电影合集/系列电影返回movie；只有电视剧/番剧的单集返回episode。未知填空或0，不编造ID。`

	userPrompt := fmt.Sprintf(`file=%q
dir=%q
hint={"t":%q,"alt":%q,"y":%d,"mt":%q,"s":%d,"e":%d}
media_type规则: standalone film/movie collection/sequel=movie; TV/anime episode=episode; unsure=unknown
return {"title":"","title_alt":"","year":0,"tmdb_id":0,"imdb_id":"","media_type":"movie|episode|unknown","season":0,"episode":0,"confidence":0.0}`,
		srcName, parentDir, hint.Title, hint.TitleAlt, hint.Year, hint.MediaType, hint.Season, hint.Episode)

	// JSON 模式：提示词保持短，但 token 预算给足，避免 deepseek-v4-flash 在文件名较长时截断。
	raw, err := s.ai.ChatCompletion(sysPrompt, userPrompt, 0.0, 768)
	if err != nil {
		return nil, "", err
	}
	out, parseErr := parseSmartRenameAIResult(raw)
	if parseErr != nil {
		// 解析失败时再用更强约束重试一次，使用更高 max_tokens 防止 JSON 半截被截断。
		retryPrompt := fmt.Sprintf(`只输出一个JSON对象，不要Markdown，不要解释，不要<think>。
file=%q
dir=%q
media_type规则: standalone film/movie collection/sequel=movie; TV/anime episode=episode; unsure=unknown
schema={"title":"","title_alt":"","year":0,"tmdb_id":0,"imdb_id":"","media_type":"movie|episode|unknown","season":0,"episode":0,"confidence":0.0}`, srcName, parentDir)
		if retryRaw, retryErr := s.ai.ChatCompletion("只输出合法JSON对象。", retryPrompt, 0.0, 1024); retryErr == nil {
			if retryOut, retryParseErr := parseSmartRenameAIResult(retryRaw); retryParseErr == nil {
				return retryOut, retryRaw, nil
			}
		}
		return nil, raw, fmt.Errorf("AI 返回 JSON 解析失败: %w", parseErr)
	}
	return out, raw, nil
}

func parseSmartRenameAIResult(raw string) (*SmartRenameAIResult, error) {
	cleaned := stripJSONFence(raw) // 复用 smart_rename.go 中的同包函数
	var out SmartRenameAIResult
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		return nil, err
	}
	out.MediaType = normalizeAIMediaType(out.MediaType)
	if out.MediaType == "movie" {
		out.Season = 0
		out.Episode = 0
	}
	if out.Confidence <= 0 || out.Confidence > 1 {
		out.Confidence = 0.5
	}
	return &out, nil
}

func normalizeAIMediaType(mediaType string) string {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "movie", "film", "movies", "feature":
		return "movie"
	case "episode", "tv", "tvshow", "tv_show", "series", "show", "anime":
		return "episode"
	default:
		return "unknown"
	}
}

func (s *ScanPostProcessService) aiIdentifyCacheKey(media *model.Media, parsed scanPostParsed) string {
	if media == nil || media.FilePath == "" {
		return ""
	}
	if parsed.MediaType != "episode" && parsed.Season <= 0 && parsed.Episode <= 0 {
		return ""
	}
	root := filepath.Dir(media.FilePath)
	if media.LibraryID != "" && s.libraryRepo != nil {
		if lib, err := s.libraryRepo.FindByID(media.LibraryID); err == nil && lib != nil {
			cleanPath := filepath.Clean(media.FilePath)
			for _, libRoot := range lib.AllPaths() {
				libRoot = filepath.Clean(strings.TrimSpace(libRoot))
				if libRoot == "" || !isPathUnderRoot(cleanPath, libRoot) {
					continue
				}
				if rel, err := filepath.Rel(libRoot, cleanPath); err == nil {
					parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
					if len(parts) >= 2 {
						root = filepath.Join(libRoot, parts[0])
					}
				}
				break
			}
		}
	}
	return "scanpost:episode:" + pathKey(root)
}

func (s *ScanPostProcessService) getAIIdentifyCache(key string) *SmartRenameAIResult {
	if key == "" {
		return nil
	}
	s.aiIdentifyCacheMu.Lock()
	defer s.aiIdentifyCacheMu.Unlock()
	return s.aiIdentifyCache[key]
}

func (s *ScanPostProcessService) setAIIdentifyCache(key string, in *SmartRenameAIResult) {
	if key == "" || in == nil || in.MediaType != "episode" {
		return
	}
	cp := *in
	cp.Episode = 0 // 目录级缓存不缓存具体集号，避免 E01 复用到所有集
	s.aiIdentifyCacheMu.Lock()
	s.aiIdentifyCache[key] = &cp
	s.aiIdentifyCacheMu.Unlock()
}

func (s *ScanPostProcessService) applyAIIdentifyResult(parsed *scanPostParsed, aiOut *SmartRenameAIResult, includeEpisode bool) {
	if parsed == nil || aiOut == nil {
		return
	}
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
		if aiOut.MediaType == "movie" {
			parsed.Season = 0
			parsed.Episode = 0
		}
	}
	if parsed.MediaType != "movie" && aiOut.Season > 0 {
		parsed.Season = aiOut.Season
	}
	if parsed.MediaType != "movie" && includeEpisode && aiOut.Episode > 0 {
		parsed.Episode = aiOut.Episode
	}
}

func (s *ScanPostProcessService) syncParsedToMedia(media *model.Media, parsed scanPostParsed, c *model.MediaClassification) {
	if media == nil || media.ID == "" || c == nil {
		return
	}
	fields := map[string]any{}
	if title := strings.TrimSpace(parsed.Title); title != "" && title != media.Title {
		fields["title"] = title
		if strings.TrimSpace(media.OrigTitle) == "" && strings.TrimSpace(media.Title) != "" {
			fields["orig_title"] = media.Title
		}
	}
	if parsed.Year > 0 && parsed.Year != media.Year {
		fields["year"] = parsed.Year
	}
	if parsed.MediaType != "" && parsed.MediaType != "unknown" && parsed.MediaType != media.MediaType {
		fields["media_type"] = parsed.MediaType
	}
	if parsed.MediaType == "movie" {
		if media.SeriesID != "" {
			fields["series_id"] = ""
		}
		if media.SeasonNum != 0 {
			fields["season_num"] = 0
		}
		if media.EpisodeNum != 0 {
			fields["episode_num"] = 0
		}
		if media.EpisodeTitle != "" {
			fields["episode_title"] = ""
		}
	} else {
		if parsed.Season > 0 && parsed.Season != media.SeasonNum {
			fields["season_num"] = parsed.Season
		}
		if parsed.Episode > 0 && parsed.Episode != media.EpisodeNum {
			fields["episode_num"] = parsed.Episode
		}
	}
	// 电影 ID 稳定性较高；剧集单集 AI ID 易抖动，不在这里回写。
	if parsed.MediaType != "episode" {
		if parsed.TMDbID > 0 && parsed.TMDbID != media.TMDbID {
			fields["tmdb_id"] = parsed.TMDbID
		}
		if parsed.IMDbID != "" && parsed.IMDbID != media.IMDbID {
			fields["imdb_id"] = parsed.IMDbID
		}
	}
	if len(fields) == 0 {
		return
	}
	oldSeriesID := media.SeriesID
	if err := s.mediaRepo.UpdateOrganizedFields(media.ID, fields); err != nil {
		s.logger.Warnf("[ScanPostProcess] 同步识别结果到 Media 失败 media_id=%s err=%v", media.ID, err)
		return
	}
	applyScanPostFields(media, fields)
	if oldSeriesID != "" && oldSeriesID != media.SeriesID {
		s.recalculateSeriesAfterMediaMove(oldSeriesID)
	}
}

func (s *ScanPostProcessService) recalculateSeriesAfterMediaMove(seriesID string) {
	if s.seriesRepo == nil || seriesID == "" {
		return
	}
	if err := s.seriesRepo.RecalculateCounts(seriesID); err != nil {
		s.logger.Warnf("[ScanPostProcess] 重算剧集合集统计失败 series_id=%s err=%v", seriesID, err)
		return
	}
	if removed, err := s.seriesRepo.CleanEmptySeries(); err != nil {
		s.logger.Warnf("[ScanPostProcess] 清理空剧集合集失败 err=%v", err)
	} else if removed > 0 {
		s.logger.Infof("[ScanPostProcess] 已清理 %d 个空剧集合集", removed)
	}
}

func applyScanPostFields(media *model.Media, fields map[string]any) {
	if v, ok := fields["title"].(string); ok {
		media.Title = v
	}
	if v, ok := fields["orig_title"].(string); ok {
		media.OrigTitle = v
	}
	if v, ok := fields["year"].(int); ok {
		media.Year = v
	}
	if v, ok := fields["media_type"].(string); ok {
		media.MediaType = v
	}
	if v, ok := fields["series_id"].(string); ok {
		media.SeriesID = v
	}
	if v, ok := fields["season_num"].(int); ok {
		media.SeasonNum = v
	}
	if v, ok := fields["episode_num"].(int); ok {
		media.EpisodeNum = v
	}
	if v, ok := fields["episode_title"].(string); ok {
		media.EpisodeTitle = v
	}
	if v, ok := fields["tmdb_id"].(int); ok {
		media.TMDbID = v
	}
	if v, ok := fields["imdb_id"].(string); ok {
		media.IMDbID = v
	}
}

// ============================ Stage 2: 归类 ============================

// classify 推导 Category / Region / Decade / GenreTags / LanguageTag / QualityTier / VirtualPath
func (s *ScanPostProcessService) classify(media *model.Media, parsed scanPostParsed, c *model.MediaClassification) {
	switch parsed.MediaType {
	case "episode":
		c.Category = "tvshow"
	case "movie":
		if strings.TrimSpace(media.Num) != "" {
			c.Category = "adult"
		} else {
			c.Category = "movie"
		}
	default:
		c.Category = inferCategory(media)
	}
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
//
// 命名规则与一键入库（LazyIngest）/ 智能重命名（SmartRename）三处统一，
// 全部委托 BuildStandardNames 渲染（含季号兜底、季尾缀剥离、目录名带 idtag 等修正）。
func (s *ScanPostProcessService) naming(media *model.Media, parsed scanPostParsed, c *model.MediaClassification) {
	style := strings.ToLower(strings.TrimSpace(c.NamingStyle))
	if style != NamingStyleJellyfin && style != NamingStylePlex {
		style = s.cfg.NamingStyle
	}
	c.NamingStyle = style

	// 路径 ID 兜底：源路径目录上有 [tmdbid-X] 时回收
	tmdbID := parsed.TMDbID
	imdbID := parsed.IMDbID
	if tmdbID == 0 || imdbID == "" {
		t, i := ExtractIDsFromPath(media.FilePath)
		if tmdbID == 0 && t > 0 {
			tmdbID = t
		}
		if imdbID == "" && i != "" {
			imdbID = i
		}
	}

	mediaType := "movie"
	if isEpisode(media, parsed) {
		mediaType = "episode"
	}

	names := BuildStandardNames(StandardNameInput{
		SourcePath: media.FilePath,
		SourceName: filepath.Base(media.FilePath),
		MediaType:  mediaType,
		Title:      parsed.Title,
		Year:       parsed.Year,
		TMDbID:     tmdbID,
		IMDbID:     imdbID,
		SeasonNum:  parsed.Season,
		EpisodeNum: parsed.Episode,
		Style:      style,
	})

	var suggestedName, suggestedDir string
	if mediaType == "episode" {
		suggestedName = names.FileName
		// 集号未知 → FileName 为空时退化用 SourceName 占位（不影响目录建议）
		if suggestedName == "" {
			suggestedName = filepath.Base(media.FilePath)
		}
		suggestedDir = filepath.ToSlash(filepath.Join(names.ShowFolder, names.SeasonDir))
	} else {
		suggestedName = names.FileName
		suggestedDir = filepath.ToSlash(names.MovieFolder)
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
	if p.MediaType == "movie" {
		return false
	}
	if p.MediaType == "episode" {
		return (p.Season > 0 && p.Episode > 0) || (media.MediaType == "episode" && media.SeasonNum > 0 && media.EpisodeNum > 0)
	}
	if media.MediaType == "episode" && media.SeasonNum > 0 && media.EpisodeNum > 0 {
		return true
	}
	return false
}
