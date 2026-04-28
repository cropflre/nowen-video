// Package service 番号批量刮削服务
// 借鉴自 mdcx-master 的批量处理流程，但整合 WebSocket 实时进度推送
//
// 核心能力：
//   - 扫描全库或指定库，自动识别所有番号媒体
//   - 并发刮削（可配置并发度）
//   - 支持暂停/恢复/取消
//   - 实时通过 WebSocket 广播进度、成功数、失败数
//   - 每条记录的结果写入 TaskStore，供失败重试查询
package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
)

// ==================== WebSocket 事件常量（番号批量）====================

const (
	EventAdultBatchStarted   = "adult_batch_started"   // 批量任务开始
	EventAdultBatchProgress  = "adult_batch_progress"  // 每条进度
	EventAdultBatchCompleted = "adult_batch_completed" // 任务完成
	EventAdultBatchFailed    = "adult_batch_failed"    // 任务整体失败
	EventAdultBatchPaused    = "adult_batch_paused"    // 暂停
	EventAdultBatchResumed   = "adult_batch_resumed"   // 恢复
	EventAdultBatchCancelled = "adult_batch_cancelled" // 取消
)

// AdultBatchProgressData 批量刮削进度事件数据
type AdultBatchProgressData struct {
	TaskID       string `json:"task_id"`
	Total        int    `json:"total"`
	Current      int    `json:"current"`
	Success      int    `json:"success"`
	Failed       int    `json:"failed"`
	Skipped      int    `json:"skipped"`
	MediaID      string `json:"media_id"`
	MediaTitle   string `json:"media_title"`
	Code         string `json:"code"`
	Status       string `json:"status"` // scraping / success / failed / skipped
	ErrMsg       string `json:"err_msg,omitempty"`
	ElapsedMS    int64  `json:"elapsed_ms"`
	EstimateLeft int64  `json:"estimate_left_ms"`
}

// ==================== 批量任务控制 ====================

// AdultBatchStatus 任务运行状态
type AdultBatchStatus string

const (
	BatchStatusRunning   AdultBatchStatus = "running"
	BatchStatusPaused    AdultBatchStatus = "paused"
	BatchStatusCancelled AdultBatchStatus = "cancelled"
	BatchStatusCompleted AdultBatchStatus = "completed"
	BatchStatusFailed    AdultBatchStatus = "failed"
)

// AdultBatchTask 单个批量刮削任务
type AdultBatchTask struct {
	ID          string           `json:"id"`
	Status      AdultBatchStatus `json:"status"`
	Total       int              `json:"total"`
	Current     int32            `json:"current"` // 原子操作
	Success     int32            `json:"success"`
	Failed      int32            `json:"failed"`
	Skipped     int32            `json:"skipped"`
	StartedAt   time.Time        `json:"started_at"`
	FinishedAt  *time.Time       `json:"finished_at,omitempty"`
	LibraryID   string           `json:"library_id,omitempty"`
	DryRun      bool             `json:"dry_run"` // 仅识别不下载
	Concurrency int              `json:"concurrency"`
	Aggregated  bool             `json:"aggregated"` // 使用聚合模式

	// 运行时控制
	cancelCtx  context.Context
	cancelFn   context.CancelFunc
	pauseCh    chan struct{} // 关闭=恢复
	pauseMutex sync.Mutex
	isPaused   bool

	// 结果记录
	Results []AdultBatchItemResult `json:"results"`
	resMu   sync.Mutex
}

// AdultBatchItemResult 单条刮削结果
type AdultBatchItemResult struct {
	MediaID    string    `json:"media_id"`
	MediaTitle string    `json:"media_title"`
	Code       string    `json:"code"`
	Status     string    `json:"status"` // success / failed / skipped
	ErrMsg     string    `json:"err_msg,omitempty"`
	Source     string    `json:"source,omitempty"`
	FinishedAt time.Time `json:"finished_at"`
}

// ==================== 批量服务 ====================

// AdultBatchService 批量刮削服务
type AdultBatchService struct {
	scraper  *AdultScraperService
	wsHub    *WSHub
	store    *AdultTaskStore // P3：任务持久化（可选）
	taskMu   sync.RWMutex
	tasks    map[string]*AdultBatchTask
	history  []*AdultBatchTask // 历史任务（最多 100 条）
	histLock sync.Mutex
}

// NewAdultBatchService 创建批量刮削服务
func NewAdultBatchService(scraper *AdultScraperService, wsHub *WSHub) *AdultBatchService {
	return &AdultBatchService{
		scraper: scraper,
		wsHub:   wsHub,
		tasks:   make(map[string]*AdultBatchTask),
	}
}

// SetTaskStore 注入任务持久化存储（P3）
func (s *AdultBatchService) SetTaskStore(store *AdultTaskStore) {
	s.store = store
}

// ==================== 任务创建 / 启动 ====================

// AdultBatchOptions 启动批量任务的选项
type AdultBatchOptions struct {
	LibraryID    string   // 空=全库
	MediaIDs     []string // 若非空，只处理这些 ID（忽略 LibraryID）
	OnlyUnscraped bool    // 只处理尚未刮削成功的媒体
	DryRun       bool     // 仅识别番号不实际刮削
	Concurrency  int      // 并发度（默认 2）
	Aggregated   bool     // 使用聚合模式
}

// Start 启动批量刮削任务，返回任务 ID
func (s *AdultBatchService) Start(opts AdultBatchOptions) (string, error) {
	if s.scraper == nil || !s.scraper.IsEnabled() {
		return "", fmtErrf("番号刮削服务未启用")
	}

	// 选择待处理的媒体列表
	mediaList, err := s.selectMediaList(opts)
	if err != nil {
		return "", err
	}
	if len(mediaList) == 0 {
		return "", fmtErrf("没有符合条件的番号媒体可刮削")
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 2
	}
	if concurrency > 8 {
		concurrency = 8 // 上限保护，避免触发反爬
	}

	ctx, cancel := context.WithCancel(context.Background())
	task := &AdultBatchTask{
		ID:          uuid.NewString(),
		Status:      BatchStatusRunning,
		Total:       len(mediaList),
		StartedAt:   time.Now(),
		LibraryID:   opts.LibraryID,
		DryRun:      opts.DryRun,
		Concurrency: concurrency,
		Aggregated:  opts.Aggregated,
		cancelCtx:   ctx,
		cancelFn:    cancel,
		pauseCh:     make(chan struct{}),
		Results:     make([]AdultBatchItemResult, 0, len(mediaList)),
	}
	close(task.pauseCh) // 初始状态=非暂停（已关闭的 chan 可随时读）

	s.taskMu.Lock()
	s.tasks[task.ID] = task
	s.taskMu.Unlock()

	// 广播开始
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(EventAdultBatchStarted, map[string]interface{}{
			"task_id":     task.ID,
			"total":       task.Total,
			"concurrency": concurrency,
			"dry_run":     opts.DryRun,
			"aggregated":  opts.Aggregated,
		})
	}

	// 异步执行
	go s.run(task, mediaList, opts)
	return task.ID, nil
}

// selectMediaList 选择待批量处理的媒体
func (s *AdultBatchService) selectMediaList(opts AdultBatchOptions) ([]model.Media, error) {
	repo := s.scraper.mediaRepo
	if repo == nil {
		return nil, fmtErrf("MediaRepo 未初始化")
	}

	// 指定 ID 列表优先
	if len(opts.MediaIDs) > 0 {
		return repo.FindByIDs(opts.MediaIDs)
	}

	// 全库或指定库
	var (
		all []model.Media
		err error
	)
	if opts.LibraryID != "" {
		all, err = repo.ListByLibraryID(opts.LibraryID)
	} else {
		// 全库：用 List 分页累加
		all = []model.Media{}
		page := 1
		pageSize := 500
		for {
			list, _, e := repo.List(page, pageSize, "")
			if e != nil {
				err = e
				break
			}
			if len(list) == 0 {
				break
			}
			all = append(all, list...)
			if len(list) < pageSize {
				break
			}
			page++
		}
	}
	if err != nil {
		return nil, err
	}

	// 过滤：只挑含番号的媒体
	out := make([]model.Media, 0, len(all))
	for _, m := range all {
		input := m.Title
		if m.FilePath != "" {
			input = m.FilePath
		}
		code, _ := ParseCode(input)
		if code == "" {
			continue
		}
		if opts.OnlyUnscraped && m.ScrapeStatus == "scraped" {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// ==================== 主执行循环 ====================

func (s *AdultBatchService) run(task *AdultBatchTask, mediaList []model.Media, opts AdultBatchOptions) {
	defer func() {
		if r := recover(); r != nil {
			s.scraper.logger.Errorf("批量刮削任务 panic: %v", r)
			task.Status = BatchStatusFailed
		}
		finishedAt := time.Now()
		task.FinishedAt = &finishedAt
		if task.Status == BatchStatusRunning {
			task.Status = BatchStatusCompleted
		}
		// 归档到历史
		s.archive(task)
		// 广播完成
		if s.wsHub != nil {
			s.wsHub.BroadcastEvent(EventAdultBatchCompleted, map[string]interface{}{
				"task_id":   task.ID,
				"total":     task.Total,
				"success":   atomic.LoadInt32(&task.Success),
				"failed":    atomic.LoadInt32(&task.Failed),
				"skipped":   atomic.LoadInt32(&task.Skipped),
				"status":    task.Status,
				"elapsed":   time.Since(task.StartedAt).Milliseconds(),
			})
		}
	}()

	sem := make(chan struct{}, task.Concurrency)
	var wg sync.WaitGroup

	for i := range mediaList {
		// 取消检查
		if task.cancelCtx.Err() != nil {
			task.Status = BatchStatusCancelled
			return
		}

		// 暂停检查（阻塞直到恢复）
		task.pauseMutex.Lock()
		pauseCh := task.pauseCh
		task.pauseMutex.Unlock()
		select {
		case <-pauseCh:
			// 未暂停（已关闭 chan）
		case <-task.cancelCtx.Done():
			task.Status = BatchStatusCancelled
			return
		}

		media := mediaList[i]
		sem <- struct{}{}
		wg.Add(1)
		go func(m model.Media) {
			defer wg.Done()
			defer func() { <-sem }()
			s.processOne(task, &m, opts)
		}(media)
	}
	wg.Wait()
}

// processOne 处理单条媒体
func (s *AdultBatchService) processOne(task *AdultBatchTask, media *model.Media, opts AdultBatchOptions) {
	start := time.Now()
	current := atomic.AddInt32(&task.Current, 1)

	// 识别番号
	input := media.Title
	if media.FilePath != "" {
		input = media.FilePath
	}
	code, _ := ParseCode(input)
	if code == "" {
		atomic.AddInt32(&task.Skipped, 1)
		s.recordResult(task, media, code, "skipped", "未识别到番号", "")
		s.broadcastProgress(task, media, code, "skipped", "未识别到番号", start)
		return
	}

	// DryRun 模式：仅识别不刮削
	if opts.DryRun {
		atomic.AddInt32(&task.Success, 1)
		s.recordResult(task, media, code, "success", "", "dry_run")
		s.broadcastProgress(task, media, code, "success", "（识别成功，未刮削）", start)
		return
	}

	// 实际刮削
	var (
		meta *AdultMetadata
		err  error
	)
	if opts.Aggregated {
		meta, _, err = s.scraper.ScrapeByCodeAggregated(code)
	} else {
		meta, err = s.scraper.ScrapeByCode(code)
	}

	if err != nil || meta == nil {
		atomic.AddInt32(&task.Failed, 1)
		errMsg := "刮削失败"
		if err != nil {
			errMsg = err.Error()
		}
		s.recordResult(task, media, code, "failed", errMsg, "")
		s.broadcastProgress(task, media, code, "failed", errMsg, start)
		return
	}

	// 应用到媒体（写入数据库 + 下载资源 + NFO）
	if err := s.scraper.ApplyToMedia(media, meta, "replace"); err != nil {
		atomic.AddInt32(&task.Failed, 1)
		s.recordResult(task, media, code, "failed", err.Error(), meta.Source)
		s.broadcastProgress(task, media, code, "failed", err.Error(), start)
		return
	}

	atomic.AddInt32(&task.Success, 1)
	s.recordResult(task, media, code, "success", "", meta.Source)
	s.broadcastProgress(task, media, code, "success", meta.Title, start)

	_ = current // 已通过 atomic 计数
}

// ==================== 进度广播 ====================

func (s *AdultBatchService) broadcastProgress(task *AdultBatchTask, media *model.Media, code, status, msg string, start time.Time) {
	if s.wsHub == nil {
		return
	}
	cur := atomic.LoadInt32(&task.Current)
	succ := atomic.LoadInt32(&task.Success)
	fail := atomic.LoadInt32(&task.Failed)
	skip := atomic.LoadInt32(&task.Skipped)

	// 估算剩余时间（线性外推）
	elapsed := time.Since(task.StartedAt).Milliseconds()
	var estLeft int64
	if cur > 0 {
		avg := elapsed / int64(cur)
		estLeft = avg * int64(task.Total-int(cur))
	}

	data := AdultBatchProgressData{
		TaskID:       task.ID,
		Total:        task.Total,
		Current:      int(cur),
		Success:      int(succ),
		Failed:       int(fail),
		Skipped:      int(skip),
		MediaID:      media.ID,
		MediaTitle:   media.Title,
		Code:         code,
		Status:       status,
		ErrMsg:       msg,
		ElapsedMS:    time.Since(start).Milliseconds(),
		EstimateLeft: estLeft,
	}
	s.wsHub.BroadcastEvent(EventAdultBatchProgress, data)
}

// recordResult 记录单条结果到任务
func (s *AdultBatchService) recordResult(task *AdultBatchTask, media *model.Media, code, status, errMsg, source string) {
	task.resMu.Lock()
	defer task.resMu.Unlock()
	// 保留最近 500 条，避免内存爆炸
	if len(task.Results) >= 500 {
		task.Results = task.Results[1:]
	}
	task.Results = append(task.Results, AdultBatchItemResult{
		MediaID:    media.ID,
		MediaTitle: media.Title,
		Code:       code,
		Status:     status,
		ErrMsg:     errMsg,
		Source:     source,
		FinishedAt: time.Now(),
	})
}

// ==================== 任务控制 ====================

// Pause 暂停任务
func (s *AdultBatchService) Pause(taskID string) error {
	t := s.getTask(taskID)
	if t == nil {
		return fmtErrf("任务不存在: %s", taskID)
	}
	t.pauseMutex.Lock()
	defer t.pauseMutex.Unlock()
	if t.isPaused {
		return nil
	}
	t.pauseCh = make(chan struct{}) // 新建未关闭的 chan，阻塞 runloop
	t.isPaused = true
	t.Status = BatchStatusPaused
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(EventAdultBatchPaused, map[string]string{"task_id": taskID})
	}
	return nil
}

// Resume 恢复任务
func (s *AdultBatchService) Resume(taskID string) error {
	t := s.getTask(taskID)
	if t == nil {
		return fmtErrf("任务不存在: %s", taskID)
	}
	t.pauseMutex.Lock()
	defer t.pauseMutex.Unlock()
	if !t.isPaused {
		return nil
	}
	close(t.pauseCh) // 关闭 chan -> 所有阻塞的 select 立即通过
	t.isPaused = false
	t.Status = BatchStatusRunning
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(EventAdultBatchResumed, map[string]string{"task_id": taskID})
	}
	return nil
}

// Cancel 取消任务
func (s *AdultBatchService) Cancel(taskID string) error {
	t := s.getTask(taskID)
	if t == nil {
		return fmtErrf("任务不存在: %s", taskID)
	}
	t.cancelFn()
	t.Status = BatchStatusCancelled
	// 顺便解除暂停阻塞
	t.pauseMutex.Lock()
	if t.isPaused {
		close(t.pauseCh)
		t.isPaused = false
	}
	t.pauseMutex.Unlock()
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(EventAdultBatchCancelled, map[string]string{"task_id": taskID})
	}
	return nil
}

// ==================== 查询 ====================

// Get 获取任务状态
func (s *AdultBatchService) Get(taskID string) *AdultBatchTask {
	return s.getTask(taskID)
}

// List 列出活跃任务
func (s *AdultBatchService) List() []*AdultBatchTask {
	s.taskMu.RLock()
	defer s.taskMu.RUnlock()
	out := make([]*AdultBatchTask, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, t)
	}
	return out
}

// History 历史任务列表（最近 100 条）
func (s *AdultBatchService) History() []*AdultBatchTask {
	s.histLock.Lock()
	defer s.histLock.Unlock()
	out := make([]*AdultBatchTask, len(s.history))
	copy(out, s.history)
	return out
}

func (s *AdultBatchService) getTask(id string) *AdultBatchTask {
	s.taskMu.RLock()
	defer s.taskMu.RUnlock()
	return s.tasks[id]
}

func (s *AdultBatchService) archive(task *AdultBatchTask) {
	s.histLock.Lock()
	if len(s.history) >= 100 {
		s.history = s.history[1:]
	}
	s.history = append(s.history, task)
	s.histLock.Unlock()

	// 从活跃列表移除
	s.taskMu.Lock()
	delete(s.tasks, task.ID)
	s.taskMu.Unlock()

	// P3：持久化到磁盘
	if s.store != nil {
		s.store.Record(task)
	}
}
