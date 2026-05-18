package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// cnSeasonNumMap 中文数字 → 季号
var cnSeasonNumMap = map[string]int{
	"一": 1, "二": 2, "三": 3, "四": 4, "五": 5,
	"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
}

// reclaimSeasonFromTitle 从一个可能带季号尾缀的 Title 中回收季号（仅当 currentSeason<=0 时）。
// 例如："一拳超人 第二季" → 返回 ("一拳超人", 2)
// 例如："Breaking Bad Season 2" → 返回 ("Breaking Bad", 2)
// 如果识别不出，返回 (原标题, 0)
func reclaimSeasonFromTitle(title string, currentSeason int) (string, int) {
	if title == "" {
		return title, currentSeason
	}
	newSeason := currentSeason

	// 中文阿拉伯季号 第X季
	if m := regexp.MustCompile(`第\s*(\d{1,2})\s*[季部]\s*$`).FindStringSubmatch(title); len(m) >= 2 {
		if n, _ := strconv.Atoi(m[1]); n > 0 && n <= 50 && newSeason <= 0 {
			newSeason = n
		}
	}
	// 中文中文季号 第X季
	if m := regexp.MustCompile(`第\s*([一二三四五六七八九十]+)\s*[季部]\s*$`).FindStringSubmatch(title); len(m) >= 2 {
		if n, ok := cnSeasonNumMap[m[1]]; ok && newSeason <= 0 {
			newSeason = n
		}
	}
	// 英文 Season N
	if m := regexp.MustCompile(`(?i)\bSeason\s*(\d{1,2})\s*$`).FindStringSubmatch(title); len(m) >= 2 {
		if n, _ := strconv.Atoi(m[1]); n > 0 && n <= 50 && newSeason <= 0 {
			newSeason = n
		}
	}
	// 末尾 S2/S02
	if m := regexp.MustCompile(`(?i)\s+S(\d{1,2})\s*$`).FindStringSubmatch(title); len(m) >= 2 {
		if n, _ := strconv.Atoi(m[1]); n > 0 && n <= 50 && newSeason <= 0 {
			newSeason = n
		}
	}

	// 用 NormalizeSeriesTitle 剥离尾缀
	cleaned := NormalizeSeriesTitle(title)
	if cleaned == "" {
		cleaned = title
	}
	return cleaned, newSeason
}

// ErrCrossVolumeNoLink 跨卷或目标卷不支持硬链接时返回的哨兵错误。
// 懒人入库策略：仅使用硬链接（瞬时、零空间、源文件 0 风险）。
// 一旦无法硬链，立即放弃，不会回退到 copy（避免占双倍空间 + 长耗时）。
var ErrCrossVolumeNoLink = errors.New("跨卷或目标卷不支持硬链接")

// ==================== 懒人入库（Lazy Ingest）服务 ====================
//
// 设计目标：用户只给一个源目录，AI 自动完成「分类 → 命名 → 入库」全流程。
//
// 编排：
//   Phase 1 Scanning    : SmartRename.Scan(源目录)，得到一组按 Jellyfin/Emby 命名规则
//                         产出的目标文件名（不动磁盘）。
//   Phase 2 Classifying : 为每条 RenamePlanItem 选择目标子目录：
//                           - movie  -> {target_root}/Movies/Title (Year) [tmdbid-xxx]/...
//                           - episode-> {target_root}/TV Shows/Title/Season XX/...
//                           - 低置信 -> {target_root}/_unsorted/...
//   Phase 3 Executing   : 仅使用 hardlink（KeepOriginal=true 不删源）。跨卷/不支持硬链时
//                         任务在 Phase 0 预检即 fail-fast，不会进入 Phase 3。
//   Phase 4 Library     : 自动创建/复用媒体库（type=mixed），Path = target_root；
//                         触发 LibraryService.Scan，由扫描器 + ScanPostProcess 接管。
//
// 安全：
//   - 不调用 SmartRename.Execute（其行为是 os.Rename，会移除原文件，不符合"懒人"安全期望）；
//   - 永远不 RemoveAll 源；只用 hardlink（不 copy），目标已存在则跳过并标记 skipped；
//   - 跨卷/不支持硬链 -> Phase 0 预检即拒绝任务，源文件 0 风险；
//   - 单 Job 只允许并发 1 个（同一 source 串行处理，避免 race）。

// ==================== 配置 ====================

// LazyIngestConfig 懒人入库默认行为
type LazyIngestConfig struct {
	// 目标根目录子目录命名（前端可改，后端定常量足矣）
	MoviesDir   string // 默认 "Movies"
	TVShowsDir  string // 默认 "TV Shows"
	UnsortedDir string // 默认 "_unsorted"

	// 置信度阈值，低于则进 _unsorted 并标记需要人工
	UnsortedThreshold float64

	// 单次任务最大文件数（保护机制）
	MaxFiles int
}

// DefaultLazyIngestConfig 默认配置
func DefaultLazyIngestConfig() LazyIngestConfig {
	return LazyIngestConfig{
		MoviesDir:         "Movies",
		TVShowsDir:        "TV Shows",
		UnsortedDir:       "_unsorted",
		UnsortedThreshold: 0.5,
		MaxFiles:          5000,
	}
}

// LazyIngestStats 懒人入库实时统计快照
type LazyIngestStats struct {
	Scanned    int `json:"scanned"`    // 扫描到的视频文件数
	Classified int `json:"classified"` // 已分类（完成 Plan 阶段）
	Planned    int `json:"planned"`    // 已生成最终落盘路径
	Executed   int `json:"executed"`   // 已成功落盘
	Skipped    int `json:"skipped"`    // 跳过（目标已存在等）
	Failed     int `json:"failed"`     // 失败
	Unsorted   int `json:"unsorted"`   // 进入 _unsorted 的条目

	// 体积统计（用于前端展示「视觉占用 vs 真实占用」）
	BytesLogical  int64 `json:"bytes_logical"`  // 视觉占用：所有落盘文件大小累加（hardlink 也计入）
	BytesPhysical int64 `json:"bytes_physical"` // 真实占用：仅 move/copy 模式累加；hardlink 模式恒为 0
}

// ==================== 服务 ====================

// LazyIngestService 懒人入库主服务
type LazyIngestService struct {
	db       *gorm.DB
	smart    *SmartRenameService
	library  *LibraryService
	scanPost *ScanPostProcessService
	cfg      LazyIngestConfig
	wsHub    *WSHub
	logger   *zap.SugaredLogger

	// 任务并发互斥（同一时刻只允许 1 个 Job 运行；后续如需多 Job 并行，可改为 keyed mutex）
	runMu sync.Mutex
}

// NewLazyIngestService 构造服务
func NewLazyIngestService(
	db *gorm.DB,
	smart *SmartRenameService,
	library *LibraryService,
	scanPost *ScanPostProcessService,
	cfg LazyIngestConfig,
	logger *zap.SugaredLogger,
) *LazyIngestService {
	if cfg.MoviesDir == "" {
		cfg.MoviesDir = "Movies"
	}
	if cfg.TVShowsDir == "" {
		cfg.TVShowsDir = "TV Shows"
	}
	if cfg.UnsortedDir == "" {
		cfg.UnsortedDir = "_unsorted"
	}
	if cfg.UnsortedThreshold <= 0 {
		cfg.UnsortedThreshold = 0.5
	}
	if cfg.MaxFiles <= 0 {
		cfg.MaxFiles = 5000
	}
	return &LazyIngestService{
		db:       db,
		smart:    smart,
		library:  library,
		scanPost: scanPost,
		cfg:      cfg,
		logger:   logger,
	}
}

// SetWSHub 注入 WS（进度推送）
func (s *LazyIngestService) SetWSHub(hub *WSHub) { s.wsHub = hub }

// ==================== 公共入参/出参 ====================

// LazyIngestInput 一键入库入参（用户只需要一个 SourcePath）
type LazyIngestInput struct {
	SourcePath string `json:"source_path"`
	// 可选：用户显式指定目标根；为空则默认 = {SourcePath}/_organized
	TargetRoot string `json:"target_root"`
	// 可选：用户希望的命名风格（默认 jellyfin）
	NamingStyle string `json:"naming_style"`
	// 可选：落盘模式 hardlink（默认）/ move（专家模式：原地移动 + journal 回滚）
	Mode string `json:"mode"`
	// 强制重新入库：即便 SourcePath 已被历史 Job 处理过也允许提交
	Force bool `json:"force"`
	// 创建者（审计）
	CreatedBy string `json:"-"`
}

// ErrAlreadyIngested 重复提交检测：当 SourcePath 已被一个已完成 Job 处理过时返回。
// handler 层据此返回 HTTP 409，前端弹「已入库到 X，是否强制重新入库 / 删除旧目标 / 取消」。
type ErrAlreadyIngested struct {
	ExistingJobs []ExistingIngestRef `json:"existing_jobs"`
}

// ExistingIngestRef 已存在的入库引用（用于前端 409 响应）
type ExistingIngestRef struct {
	JobID       string    `json:"job_id"`
	TargetRoot  string    `json:"target_root"`
	Mode        string    `json:"mode"`
	CompletedAt time.Time `json:"completed_at"`
}

// Error 实现 error
func (e *ErrAlreadyIngested) Error() string {
	if len(e.ExistingJobs) == 0 {
		return "该源目录已被入库过"
	}
	return fmt.Sprintf("该源目录已入库到 %s（共 %d 次历史记录）", e.ExistingJobs[0].TargetRoot, len(e.ExistingJobs))
}

// ==================== 主入口：Submit ====================

// Submit 创建并立刻异步启动一个懒人入库任务。返回 IngestJob（pending → 启动后变 scanning）。
//
// 调用方拿到 Job.ID 后可以通过 WS 或轮询 GET 任务详情查看进度。
func (s *LazyIngestService) Submit(in LazyIngestInput) (*model.IngestJob, error) {
	// === 入参校验 ===
	src := strings.TrimSpace(in.SourcePath)
	if src == "" {
		return nil, errors.New("source_path 必填")
	}
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return nil, fmt.Errorf("source_path 非法: %w", err)
	}
	st, err := os.Stat(absSrc)
	if err != nil {
		return nil, fmt.Errorf("source_path 不可访问: %w", err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("source_path 不是目录: %s", absSrc)
	}

	// 目标根（默认 = {src}/_organized）
	tgtRoot := strings.TrimSpace(in.TargetRoot)
	if tgtRoot == "" {
		tgtRoot = filepath.Join(absSrc, "_organized")
	} else {
		tgtRoot, err = filepath.Abs(tgtRoot)
		if err != nil {
			return nil, fmt.Errorf("target_root 非法: %w", err)
		}
	}
	// 防呆：禁止 target_root 等于 source_path（会和扫描结果撞车）
	if pathEqual(tgtRoot, absSrc) {
		return nil, errors.New("target_root 不能等于 source_path，请使用 source_path/_organized 或其它独立目录")
	}

	style := strings.ToLower(strings.TrimSpace(in.NamingStyle))
	if style != NamingStyleJellyfin && style != NamingStylePlex {
		style = NamingStyleJellyfin
	}

	// 落盘模式（默认 hardlink；move 是专家模式，会真实移动源文件）
	mode := strings.ToLower(strings.TrimSpace(in.Mode))
	if mode != string(model.IngestJobModeMove) {
		mode = string(model.IngestJobModeHardlink)
	}

	// === 重复入库检测（除非 Force） ===
	if !in.Force {
		if refs, _ := s.findCompletedJobsBySource(absSrc); len(refs) > 0 {
			return nil, &ErrAlreadyIngested{ExistingJobs: refs}
		}
	}

	// === 持久化 Job（pending） ===
	job := &model.IngestJob{
		ID:           uuid.New().String(),
		SourcePath:   absSrc,
		TargetRoot:   tgtRoot,
		KeepOriginal: mode != string(model.IngestJobModeMove),
		NamingStyle:  style,
		Mode:         mode,
		Status:       model.IngestJobStatusPending,
		Phase:        "等待执行",
		Progress:     0,
		CreatedBy:    in.CreatedBy,
		Stats:        s.encodeStats(LazyIngestStats{}),
	}
	if err := s.db.Create(job).Error; err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	// Move 模式预先分配 journal 路径（写在 target_root/.nowen/journal-<jobID>.log）
	if mode == string(model.IngestJobModeMove) {
		jp := filepath.Join(tgtRoot, ".nowen", "journal-"+job.ID+".log")
		job.JournalPath = jp
		_ = s.updateFields(job.ID, map[string]interface{}{"journal_path": jp})
	}

	// === Phase 0: 跨卷预检（hardlink/move 都强制要求同卷）===
	// hardlink: 跨卷直接失败
	// move    : 跨卷会触发 EXDEV，os.Rename 会失败 -> 同样要求同卷
	// 在异步任务启动前同步探测，让用户立刻拿到可执行错误（而不是任务跑完才发现 0 文件落盘）
	if err := preflightSameVolume(absSrc, tgtRoot); err != nil {
		// 探测失败时把 Job 直接置为 failed，便于前端列表展示
		now := time.Now()
		_ = s.updateFields(job.ID, map[string]interface{}{
			"status":        model.IngestJobStatusFailed,
			"phase":         "跨卷预检失败",
			"error_message": err.Error(),
			"completed_at":  &now,
		})
		s.logger.Warnf("[LazyIngest] 跨卷预检失败 job=%s src=%s target=%s err=%v", job.ID, absSrc, tgtRoot, err)
		return nil, err
	}

	// === 异步执行 ===
	go s.run(job.ID)

	s.logger.Infof("[LazyIngest] 任务已创建 job=%s src=%s target=%s", job.ID, absSrc, tgtRoot)
	return job, nil
}

// preflightSameVolume 跨卷预检：在源目录写一个临时探测文件，尝试 hardlink 到目标根。
//
// 成功 -> 同卷且支持硬链接，返回 nil；
// 失败 -> 跨卷 / FAT32 / exFAT / 网络盘，返回带可执行建议的中文错误。
//
// 探测文件以 ".nowen_probe_<uuid>" 命名，无论成功失败都会清理。
func preflightSameVolume(src, tgt string) error {
	if err := os.MkdirAll(tgt, 0o755); err != nil {
		return fmt.Errorf("创建 target_root 失败: %w", err)
	}
	probeID := uuid.New().String()
	probeSrc := filepath.Join(src, ".nowen_probe_"+probeID)
	probeDst := filepath.Join(tgt, ".nowen_probe_"+probeID)

	if err := os.WriteFile(probeSrc, []byte("probe"), 0o644); err != nil {
		return fmt.Errorf("源目录无写权限，无法预检: %w", err)
	}
	defer os.Remove(probeSrc)

	err := os.Link(probeSrc, probeDst)
	if err == nil {
		_ = os.Remove(probeDst)
		return nil
	}

	srcVol := filepath.VolumeName(src)
	if srcVol == "" {
		srcVol = src
	}
	if isCrossDeviceLinkError(err) {
		return fmt.Errorf("源目录与目标根不在同一卷，无法硬链接。请将目标根改到 %s 下的某个目录（懒人入库仅使用硬链接，不会复制大文件）", srcVol)
	}
	return fmt.Errorf("目标卷不支持硬链接（可能是 FAT32 / exFAT / 网络盘）：%w；建议把目标根换到与源同卷的 NTFS / ext4 / APFS 目录", err)
}

// isCrossDeviceLinkError 判定一个 os.Link 的错误是否属于"跨设备"类。
//
// Linux/Unix: syscall.EXDEV ("invalid cross-device link")
// Windows   : ERROR_NOT_SAME_DEVICE = 0x11，错误信息一般包含 "different disk drive" / "不在同一"
func isCrossDeviceLinkError(err error) bool {
	if err == nil {
		return false
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		if errors.Is(linkErr.Err, syscall.EXDEV) {
			return true
		}
		msg := strings.ToLower(linkErr.Err.Error())
		if strings.Contains(msg, "different disk drive") ||
			strings.Contains(msg, "cross-device") ||
			strings.Contains(msg, "not the same") ||
			strings.Contains(msg, "不在同一") {
			return true
		}
	}
	return false
}

// ==================== 任务编排 ====================

// run 主流程（在 goroutine 中执行）。任何阶段失败都会把 Job 标记为 failed。
func (s *LazyIngestService) run(jobID string) {
	// 串行：避免多个 Job 同时操作磁盘相互冲突
	s.runMu.Lock()
	defer s.runMu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("[LazyIngest] 任务 panic job=%s err=%v", jobID, r)
			_ = s.markFailed(jobID, fmt.Sprintf("内部错误：%v", r))
		}
	}()

	job, err := s.loadJob(jobID)
	if err != nil {
		s.logger.Errorf("[LazyIngest] 任务不存在 job=%s: %v", jobID, err)
		return
	}

	// Move 模式：创建 journal writer，全过程复用
	var journal *ingestJournalWriter
	if job.Mode == string(model.IngestJobModeMove) && job.JournalPath != "" {
		if jw, jerr := newIngestJournalWriter(job.JournalPath); jerr == nil {
			journal = jw
			defer journal.Close()
		} else {
			s.logger.Warnf("[LazyIngest] journal 创建失败 job=%s err=%v", jobID, jerr)
			_ = s.markFailed(jobID, fmt.Sprintf("journal 创建失败：%v", jerr))
			return
		}
	}

	now := time.Now()
	_ = s.updateFields(jobID, map[string]interface{}{
		"status":     model.IngestJobStatusScanning,
		"phase":      "扫描源目录",
		"progress":   5,
		"started_at": &now,
	})
	s.broadcast(job)

	stats := LazyIngestStats{}

	// ========== Phase 1: 扫描 + 智能识别（复用 SmartRename.Scan） ==========
	plan, err := s.smart.Scan(ScanInput{
		RootPath:    job.SourcePath,
		NamingStyle: job.NamingStyle,
		CreatedBy:   job.CreatedBy,
	})
	if err != nil {
		_ = s.markFailed(jobID, fmt.Sprintf("扫描失败：%v", err))
		return
	}
	stats.Scanned = len(plan.Items)
	if stats.Scanned == 0 {
		_ = s.updateFields(jobID, map[string]interface{}{
			"status":   model.IngestJobStatusCompleted,
			"phase":    "未发现视频文件",
			"progress": 100,
			"plan_ids": s.encodeJSONArray([]string{plan.ID}),
			"stats":    s.encodeStats(stats),
		})
		now := time.Now()
		_ = s.updateFields(jobID, map[string]interface{}{"completed_at": &now})
		s.broadcast(s.mustLoadJob(jobID))
		s.logger.Infof("[LazyIngest] 任务完成（无视频）job=%s", jobID)
		return
	}

	_ = s.updateFields(jobID, map[string]interface{}{
		"status":   model.IngestJobStatusClassifying,
		"phase":    fmt.Sprintf("识别完成，扫到 %d 个文件，正在分类", stats.Scanned),
		"progress": 30,
		"plan_ids": s.encodeJSONArray([]string{plan.ID}),
		"stats":    s.encodeStats(stats),
	})
	s.broadcast(s.mustLoadJob(jobID))

	// ========== Phase 2: 重写每条目的 TargetPath -> 落到 target_root 下 ==========
	plannedItems := make([]ingestPlannedItem, 0, len(plan.Items))
	for i := range plan.Items {
		it := &plan.Items[i]
		// SmartRename 失败/跳过的条目直接计入并跳过
		if it.Status == model.RenameItemStatusFailed {
			stats.Failed++
			continue
		}
		// 已经是目标命名 + 仍需归类 -> 仍要按规则放入 _organized
		// （这里我们覆盖 SmartRename 的 Skipped 判断，因为我们要的是"重组目录结构"而不仅是改名）
		dest, kind := s.resolveDestPath(job.TargetRoot, it)
		if dest == "" {
			stats.Failed++
			continue
		}
		plannedItems = append(plannedItems, ingestPlannedItem{
			Item:       it,
			DestPath:   dest,
			Kind:       kind,
			IsUnsorted: kind == "unsorted",
		})
		if kind == "unsorted" {
			stats.Unsorted++
		}
	}
	stats.Classified = len(plannedItems)
	stats.Planned = len(plannedItems)
	_ = s.updateFields(jobID, map[string]interface{}{
		"status":   model.IngestJobStatusPlanning,
		"phase":    fmt.Sprintf("已规划 %d 条（含 %d 条待人工）", stats.Planned, stats.Unsorted),
		"progress": 45,
		"stats":    s.encodeStats(stats),
	})
	s.broadcast(s.mustLoadJob(jobID))

	// ========== Phase 3: 落盘（hardlink / copy） ==========
	_ = s.updateFields(jobID, map[string]interface{}{
		"status":   model.IngestJobStatusExecuting,
		"phase":    "正在整理文件（仅使用硬链接，源文件不变）",
		"progress": 50,
	})

	executor := newLazyIngestExecutor(s.logger)
	executor.mode = job.Mode
	executor.journal = journal

	// === 并发落盘（仅 hardlink） ===
	// hardlink 是微秒级的元数据操作，不读写文件内容。多 goroutine 主要是并行
	// MkdirAll/Stat/Link 以压住元数据调度开销。
	// 安全：placeFile 内部仅读 src/写 dst，不同条目的 dst 各不相同，天然没有写冲突。
	numWorkers := runtime.NumCPU()
	if numWorkers < 4 {
		numWorkers = 4
	}
	if numWorkers > 16 {
		numWorkers = 16 // hardlink 只是元数据操作，上限可以放到 16
	}

	type execResult struct {
		idx     int
		skipped bool
		failed  bool
		bytes   int64
		err     error
	}

	jobs := make(chan int, len(plannedItems))
	results := make(chan execResult, len(plannedItems))
	var wgExec sync.WaitGroup

	worker := func() {
		defer wgExec.Done()
		for idx := range jobs {
			p := plannedItems[idx]
			res := execResult{idx: idx}

			// 主视频
			if err := executor.placeFile(p.Item.SourcePath, p.DestPath); err != nil {
				res.failed = true
				res.err = err
				s.logger.Warnf("[LazyIngest] 落盘失败 src=%s dst=%s: %v", p.Item.SourcePath, p.DestPath, err)
				results <- res
				continue
			}
			res.skipped = executor.lastSkipped
			res.bytes = executor.lastBytes

			// 关联资源（字幕、海报、nfo）跟随主视频
			var related []SmartRenameRelatedFile
			if p.Item.RelatedFilesJSON != "" {
				_ = json.Unmarshal([]byte(p.Item.RelatedFilesJSON), &related)
			}
			mainDir := filepath.Dir(p.DestPath)
			mainStem := strings.TrimSuffix(filepath.Base(p.DestPath), filepath.Ext(p.DestPath))
			for _, r := range related {
				ext := filepath.Ext(r.Source)
				origStem := strings.TrimSuffix(filepath.Base(p.Item.SourcePath), filepath.Ext(p.Item.SourcePath))
				origRelStem := strings.TrimSuffix(filepath.Base(r.Source), filepath.Ext(r.Source))
				suffix := strings.TrimPrefix(origRelStem, origStem)
				relDest := filepath.Join(mainDir, mainStem+suffix+ext)
				if err := executor.placeFile(r.Source, relDest); err != nil {
					s.logger.Warnf("[LazyIngest] 关联资源落盘失败 src=%s dst=%s: %v", r.Source, relDest, err)
				}
			}
			results <- res
		}
	}

	wgExec.Add(numWorkers)
	for w := 0; w < numWorkers; w++ {
		go worker()
	}
	for i := range plannedItems {
		jobs <- i
	}
	close(jobs)

	// 进度节流：最多每 800ms 推一次、或者每 5% 推一次
	lastBroadcast := time.Now()
	lastProgress := 50
	done := 0
	total := len(plannedItems)

	go func() {
		wgExec.Wait()
		close(results)
	}()

	for r := range results {
		if r.failed {
			stats.Failed++
		} else if r.skipped {
			stats.Skipped++
		} else {
			stats.Executed++
			stats.BytesLogical += r.bytes
			// move 模式会真实占用目标卷空间（同卷 rename 只动元数据不占额外空间，但释放了源）
			// hardlink 模式不占任何额外空间。为了让 UI 展示「真实占用」有意义：
			//   - hardlink: BytesPhysical = 0（不加）
			//   - move    : BytesPhysical = 0（同卷也不多占，但源被释放，总体中性）
			// 仅未来如果出现 copy 降级才会记。
		}
		done++

		if total > 0 {
			prog := 50 + int(35*float64(done)/float64(total))
			now := time.Now()
			if prog-lastProgress >= 5 || now.Sub(lastBroadcast) >= 800*time.Millisecond || done == total {
				_ = s.updateFields(jobID, map[string]interface{}{
					"progress": prog,
					"stats":    s.encodeStats(stats),
				})
				s.broadcast(s.mustLoadJob(jobID))
				lastProgress = prog
				lastBroadcast = now
			}
		}
	}

	// ========== Phase 4: 建库 + 触发扫描 ==========
	_ = s.updateFields(jobID, map[string]interface{}{
		"phase":    "创建媒体库并触发扫描",
		"progress": 88,
		"stats":    s.encodeStats(stats),
	})
	s.broadcast(s.mustLoadJob(jobID))

	libIDs, err := s.ensureLibrary(job.SourcePath, job.TargetRoot)
	if err != nil {
		// 建库失败但文件已落盘 -> 记入 error_message，但仍标记 completed（用户可手动建库）
		s.logger.Warnf("[LazyIngest] 建库失败 job=%s: %v", jobID, err)
		_ = s.updateFields(jobID, map[string]interface{}{
			"error_message": fmt.Sprintf("文件已整理至 %s，但建库失败：%v；请手动添加媒体库", job.TargetRoot, err),
		})
	}
	if len(libIDs) > 0 {
		_ = s.updateFields(jobID, map[string]interface{}{
			"library_ids": s.encodeJSONArray(libIDs),
		})
		// 异步扫描即可（LibraryService.Scan 内部已 goroutine）
		for _, lid := range libIDs {
			if err := s.library.Scan(lid); err != nil {
				s.logger.Warnf("[LazyIngest] 触发扫描失败 lib=%s: %v", lid, err)
			}
		}
	}

	// ========== 收尾 ==========
	completedAt := time.Now()
	finalStatus := model.IngestJobStatusCompleted
	finalPhase := "全部完成"
	if stats.Executed == 0 && stats.Skipped == 0 {
		finalStatus = model.IngestJobStatusFailed
		finalPhase = "未能整理任何文件"
	} else if stats.Unsorted > 0 {
		finalPhase = fmt.Sprintf("完成（%d 条待人工确认见 _unsorted）", stats.Unsorted)
	}
	_ = s.updateFields(jobID, map[string]interface{}{
		"status":       finalStatus,
		"phase":        finalPhase,
		"progress":     100,
		"stats":        s.encodeStats(stats),
		"completed_at": &completedAt,
	})
	s.broadcast(s.mustLoadJob(jobID))
	s.logger.Infof("[LazyIngest] 任务完成 job=%s status=%s stats=%+v", jobID, finalStatus, stats)
}

// ==================== 分类与目标路径 ====================

// ingestPlannedItem 落盘前的规划条目
type ingestPlannedItem struct {
	Item       *model.RenamePlanItem
	DestPath   string // 最终落盘绝对路径
	Kind       string // movie / episode / unsorted
	IsUnsorted bool
}

// resolveDestPath 决定一个条目应当落到 target_root 下的哪个位置。
//
// 规则：
//   - 置信度 < UnsortedThreshold 或 Item.SafetyOK=false 或 MediaType 未知 -> _unsorted
//   - episode (S/E 已知)  -> {target}/TV Shows/<show>/Season XX/<TargetName>
//   - movie               -> {target}/Movies/<movie folder>/<TargetName>
//
// 返回 destPath, kind。kind 用于后续统计。
func (s *LazyIngestService) resolveDestPath(targetRoot string, item *model.RenamePlanItem) (string, string) {
	if item.TargetName == "" {
		return "", ""
	}

	// 低置信度或安全检查未通过 -> _unsorted（保留原文件名）
	if item.Confidence < s.cfg.UnsortedThreshold || (!item.SafetyOK && item.SafetyNote != "已是目标命名") {
		return filepath.Join(targetRoot, s.cfg.UnsortedDir, item.SourceName), "unsorted"
	}

	switch item.MediaType {
	case "episode":
		// 季号兜底：从 ParsedTitle 中回收（处理 AI 返回的"一拳超人 第二季"这类情况）
		if item.ParsedTitle != "" {
			if cleaned, season := reclaimSeasonFromTitle(item.ParsedTitle, item.SeasonNum); season > 0 || cleaned != item.ParsedTitle {
				item.ParsedTitle = cleaned
				if item.SeasonNum <= 0 && season > 0 {
					item.SeasonNum = season
				}
			}
		}
		// 季号仍未知 -> 默认 Season 01（避免大量剧集进 _unsorted）
		if item.SeasonNum <= 0 {
			item.SeasonNum = 1
		}
		if item.EpisodeNum <= 0 {
			return filepath.Join(targetRoot, s.cfg.UnsortedDir, item.SourceName), "unsorted"
		}
		// 剧集合集目录名：去掉 SxxExx 后的标题 + 年份 + idtag
		showFolder := s.deriveShowFolderName(item)
		seasonDir := fmt.Sprintf("Season %02d", item.SeasonNum)
		return filepath.Join(targetRoot, s.cfg.TVShowsDir, showFolder, seasonDir, item.TargetName), "episode"

	case "movie", "":
		// 电影：每部一个文件夹（Emby/Jellyfin 推荐结构）
		movieFolder := strings.TrimSuffix(item.TargetName, filepath.Ext(item.TargetName))
		return filepath.Join(targetRoot, s.cfg.MoviesDir, movieFolder, item.TargetName), "movie"

	default:
		return filepath.Join(targetRoot, s.cfg.UnsortedDir, item.SourceName), "unsorted"
	}
}

// deriveShowFolderName 从 RenamePlanItem 中派生 TV Show 文件夹名
//
// 内部委托 BuildStandardNames，与一键入库/智能重命名/扫描归类三处保持一致。
func (s *LazyIngestService) deriveShowFolderName(item *model.RenamePlanItem) string {
	title := strings.TrimSpace(item.ParsedTitle)
	if title == "" {
		// 去掉 TargetName 中的 " S01E01" 后缀
		stem := strings.TrimSuffix(item.TargetName, filepath.Ext(item.TargetName))
		// 截断到第一个 SxxExx 处
		if idx := strings.Index(strings.ToUpper(stem), " S"); idx > 0 {
			title = stem[:idx]
		} else {
			title = stem
		}
	}
	names := BuildStandardNames(StandardNameInput{
		SourcePath: item.SourcePath,
		SourceName: item.SourceName,
		MediaType:  "episode",
		Title:      title,
		Year:       item.ParsedYear,
		TMDbID:     item.ParsedTMDbID,
		IMDbID:     item.ParsedIMDbID,
		SeasonNum:  item.SeasonNum,
		EpisodeNum: item.EpisodeNum,
		Style:      NamingStyleJellyfin,
	})
	if names.ShowFolder != "" {
		return names.ShowFolder
	}
	return collapseWhitespace(title)
}

// ==================== 建库 ====================

// ensureLibrary 在 TargetRoot 下创建（或复用）一个 mixed 媒体库。
//
// 库名 = 源目录的 basename（保持用户语义）；如同名已存在则在末尾加 " (N)"。
func (s *LazyIngestService) ensureLibrary(srcPath, targetRoot string) ([]string, error) {
	if s.library == nil {
		return nil, errors.New("library service 未注入")
	}

	// 确保 target_root 存在（即使一条文件都没落盘也不致于扫描时报错）
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return nil, fmt.Errorf("创建 target_root 失败: %w", err)
	}

	// 1) 已有库：path 完全匹配 -> 直接复用
	libs, err := s.library.List()
	if err != nil {
		return nil, err
	}
	for _, lib := range libs {
		for _, p := range lib.AllPaths() {
			if pathEqual(p, targetRoot) {
				s.logger.Infof("[LazyIngest] 复用已有媒体库 id=%s path=%s", lib.ID, targetRoot)
				return []string{lib.ID}, nil
			}
		}
	}

	// 2) 新建库
	baseName := filepath.Base(srcPath)
	if baseName == "" || baseName == "." || baseName == string(filepath.Separator) {
		baseName = "新媒体库"
	}
	name := baseName
	taken := map[string]bool{}
	for _, lib := range libs {
		taken[lib.Name] = true
	}
	for n := 1; taken[name] && n < 1000; n++ {
		name = fmt.Sprintf("%s (%d)", baseName, n)
	}

	lib, err := s.library.Create(name, targetRoot, "mixed")
	if err != nil {
		return nil, err
	}
	s.logger.Infof("[LazyIngest] 已创建媒体库 id=%s name=%s path=%s", lib.ID, name, targetRoot)
	return []string{lib.ID}, nil
}

// ==================== 数据访问辅助 ====================

func (s *LazyIngestService) loadJob(id string) (*model.IngestJob, error) {
	var job model.IngestJob
	if err := s.db.First(&job, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *LazyIngestService) mustLoadJob(id string) *model.IngestJob {
	job, _ := s.loadJob(id)
	return job
}

func (s *LazyIngestService) updateFields(id string, fields map[string]interface{}) error {
	return s.db.Model(&model.IngestJob{}).Where("id = ?", id).Updates(fields).Error
}

func (s *LazyIngestService) markFailed(id, msg string) error {
	now := time.Now()
	return s.updateFields(id, map[string]interface{}{
		"status":        model.IngestJobStatusFailed,
		"phase":         "失败",
		"error_message": msg,
		"completed_at":  &now,
	})
}

func (s *LazyIngestService) encodeStats(stats LazyIngestStats) string {
	b, _ := json.Marshal(stats)
	return string(b)
}

func (s *LazyIngestService) encodeJSONArray(arr []string) string {
	b, _ := json.Marshal(arr)
	return string(b)
}

// ==================== 查询接口 ====================

// GetJob 取单个任务
func (s *LazyIngestService) GetJob(id string) (*model.IngestJob, error) {
	return s.loadJob(id)
}

// ListJobs 列出最近 N 条
func (s *LazyIngestService) ListJobs(limit int) ([]model.IngestJob, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var jobs []model.IngestJob
	err := s.db.Order("created_at desc").Limit(limit).Find(&jobs).Error
	return jobs, err
}

// GetJobItems 取一个 Job 关联的所有明细条目（来自 RenamePlanItem）。
//
// 实现：先取 IngestJob.PlanIDs（JSON 数组），再 query rename_plan_items where plan_id IN (...)。
// 用途：前端"任务详情页"展示文件级明细（成功/跳过/失败/待人工 + SafetyNote）。
func (s *LazyIngestService) GetJobItems(jobID string) ([]model.RenamePlanItem, error) {
	job, err := s.loadJob(jobID)
	if err != nil {
		return nil, err
	}
	var planIDs []string
	if strings.TrimSpace(job.PlanIDs) != "" {
		_ = json.Unmarshal([]byte(job.PlanIDs), &planIDs)
	}
	if len(planIDs) == 0 {
		return []model.RenamePlanItem{}, nil
	}
	var items []model.RenamePlanItem
	if err := s.db.Where("plan_id IN ?", planIDs).
		Order("status asc, source_path asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// CancelJob 取消（仅 pending/scanning 状态可取消）
//
// 注：当前实现是"标记取消"，不会强行中断 goroutine（中断风险大，先简单处理）。
func (s *LazyIngestService) CancelJob(id string) error {
	job, err := s.loadJob(id)
	if err != nil {
		return err
	}
	switch job.Status {
	case model.IngestJobStatusPending, model.IngestJobStatusScanning:
		return s.updateFields(id, map[string]interface{}{
			"status": model.IngestJobStatusCanceled,
			"phase":  "已取消",
		})
	default:
		return fmt.Errorf("当前状态 %s 不可取消", job.Status)
	}
}

// findCompletedJobsBySource 查询同一 SourcePath 已完成（或失败）的历史 Job
//
// 用于重复入库检测：同源已被处理过 → handler 返回 409 让用户确认
func (s *LazyIngestService) findCompletedJobsBySource(absSrc string) ([]ExistingIngestRef, error) {
	var jobs []model.IngestJob
	err := s.db.
		Where("source_path = ? AND status IN ?", absSrc,
			[]model.IngestJobStatus{model.IngestJobStatusCompleted, model.IngestJobStatusFailed}).
		Order("created_at desc").
		Limit(10).
		Find(&jobs).Error
	if err != nil {
		return nil, err
	}
	refs := make([]ExistingIngestRef, 0, len(jobs))
	for _, j := range jobs {
		// 只关心成功完成的（failed 但产生过部分文件也算"已动过"，保守也纳入）
		var ts time.Time
		if j.CompletedAt != nil {
			ts = *j.CompletedAt
		}
		refs = append(refs, ExistingIngestRef{
			JobID:       j.ID,
			TargetRoot:  j.TargetRoot,
			Mode:        j.Mode,
			CompletedAt: ts,
		})
	}
	return refs, nil
}

// RollbackJob 回滚一个 Move 模式的 Job：按 journal 倒序还原文件位置
//
// 行为：
//   - 仅 mode=move 且 journal 存在的 Job 可回滚
//   - 仅 status=completed/failed 的 Job 可回滚（运行中拒绝）
//   - 回滚后把 status 改为 "canceled"，phase 标注"已回滚"
//   - 30 天前的 Job 拒绝回滚（防误操作；由 caller 检查 CompletedAt）
//
// 返回回滚统计明细，供 UI 展示。
func (s *LazyIngestService) RollbackJob(jobID string) (*IngestRollbackResult, error) {
	job, err := s.loadJob(jobID)
	if err != nil {
		return nil, err
	}
	if job.Mode != string(model.IngestJobModeMove) {
		return nil, errors.New("仅 move 模式的任务支持回滚")
	}
	if job.JournalPath == "" {
		return nil, errors.New("该任务未记录 journal，无法回滚")
	}
	switch job.Status {
	case model.IngestJobStatusCompleted, model.IngestJobStatusFailed:
		// ok
	default:
		return nil, fmt.Errorf("当前状态 %s 不可回滚（仅已完成/失败可回滚）", job.Status)
	}
	// 30 天保护
	if job.CompletedAt != nil && time.Since(*job.CompletedAt) > 30*24*time.Hour {
		return nil, errors.New("距任务完成已超过 30 天，禁止回滚以防误操作")
	}
	// 文件存在校验
	if _, err := os.Stat(job.JournalPath); err != nil {
		return nil, fmt.Errorf("journal 文件丢失或不可访问: %w", err)
	}

	res, err := RollbackIngestJournal(job.JournalPath)
	if err != nil {
		return nil, err
	}

	// 更新 Job 状态
	now := time.Now()
	errMsg := ""
	if len(res.Errors) > 0 {
		errMsg = fmt.Sprintf("回滚部分失败：%d 处冲突", len(res.Errors))
	}
	_ = s.updateFields(jobID, map[string]interface{}{
		"status":        model.IngestJobStatusCanceled,
		"phase":         fmt.Sprintf("已回滚（还原 %d / 跳过 %d）", res.RestoredMv, res.SkippedMv),
		"error_message": errMsg,
		"completed_at":  &now,
	})
	s.broadcast(s.mustLoadJob(jobID))
	s.logger.Infof("[LazyIngest] 回滚完成 job=%s 还原=%d 跳过=%d 移除目录=%d",
		jobID, res.RestoredMv, res.SkippedMv, res.RemovedDir)
	return res, nil
}

// ==================== 进度推送 ====================

func (s *LazyIngestService) broadcast(job *model.IngestJob) {
	if s.wsHub == nil || job == nil {
		return
	}
	s.wsHub.BroadcastEvent("ingest_progress", job)
}

// ==================== 落盘执行器 ====================

// lazyIngestExecutor 仅负责“把一个文件放到目标位置”。根据 mode 选择 hardlink 或 move。
//
// 与 smart_rename_executor 的区别：
//   - hardlink: 不删除源（KeepOriginal=true，源文件 0 风险）、仅硬链、不 copy
//   - move    : 专家模式，os.Rename 原地移动、同卷瞬间完成、源文件被释放；journal 记录每次操作以供回滚
//   - 失败 / 目标已存在 时安全跳过，不抛回滚（懒人模式：宁少做不破坏）。
type lazyIngestExecutor struct {
	logger      *zap.SugaredLogger
	mode        string               // hardlink / move；为空默认 hardlink
	journal     *ingestJournalWriter // 仅 move 模式使用
	lastSkipped bool                 // 给上层统计跳过条目用
	lastBytes   int64                // 本次落盘文件大小（成功且未跳过时为文件实际字节数）
}

func newLazyIngestExecutor(logger *zap.SugaredLogger) *lazyIngestExecutor {
	return &lazyIngestExecutor{logger: logger}
}

// placeFile 把 src 放到 dst：按 e.mode 选择 hardlink 或 move。
//
// 行为：
//   - src/dst 为空                              -> 错误
//   - dst 与 src 完全相同（pathEqual）          -> noop, skipped=true
//   - dst 已存在（无论 size 是否一致）           -> skipped=true（不覆盖）
//   - 父目录不存在                              -> MkdirAll（move 模式下会 journal.AppendMkdir）
//   - hardlink 失败：跨卷                        -> ErrCrossVolumeNoLink
//   - hardlink 失败：其他                        -> 包装错误返回
//   - move 失败：跨卷                            -> ErrCrossVolumeNoLink（与 hardlink 一致语义）
//   - move 失败：其他                            -> 包装错误返回
//
// 注意：move 模式下，源文件会被真实移动；UI 必须二次确认后才允许启用。
func (e *lazyIngestExecutor) placeFile(src, dst string) error {
	e.lastSkipped = false
	e.lastBytes = 0
	if src == "" || dst == "" {
		return errors.New("src/dst 不能为空")
	}
	if pathEqual(src, dst) {
		e.lastSkipped = true
		return nil
	}

	// 父目录
	parent := filepath.Dir(dst)
	parentExisted := true
	if _, err := os.Stat(parent); err != nil {
		parentExisted = false
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	if !parentExisted && e.journal != nil {
		// move 模式下记录新建目录，回滚时如果为空可以清理
		_ = e.journal.AppendMkdir(parent)
	}

	// 目标已存在—跳过（不覆盖，懒人安全策略）
	if _, err := os.Stat(dst); err == nil {
		e.lastSkipped = true
		return nil
	}

	// 记录大小（为了 stats.BytesLogical）
	if st, err := os.Stat(src); err == nil && !st.IsDir() {
		e.lastBytes = st.Size()
	}

	switch e.mode {
	case string(model.IngestJobModeMove):
		// 专家模式：os.Rename + journal
		if e.journal != nil {
			// 先写 journal 再执行 rename（崩溃后能识别未完成的操作）
			if jerr := e.journal.AppendRename(src, dst); jerr != nil {
				return fmt.Errorf("journal 写入失败: %w", jerr)
			}
		}
		if err := os.Rename(src, dst); err != nil {
			if isCrossDeviceLinkError(err) {
				return ErrCrossVolumeNoLink
			}
			return fmt.Errorf("移动失败: %w", err)
		}
		return nil
	default:
		// hardlink（同卷瞬间完成、零额外空间、源文件不变）
		if err := os.Link(src, dst); err != nil {
			if isCrossDeviceLinkError(err) {
				return ErrCrossVolumeNoLink
			}
			return fmt.Errorf("硬链接失败: %w", err)
		}
		return nil
	}
}
