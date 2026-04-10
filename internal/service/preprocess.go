package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/shirou/gopsutil/v3/cpu"
	"go.uber.org/zap"
)

// P3 优化：预编译正则表达式，避免每次转码调用都重新编译
var (
	ffmpegTimeRegex  = regexp.MustCompile(`time=(\d{2}):(\d{2}):(\d{2})\.(\d{2})`)
	ffmpegSpeedRegex = regexp.MustCompile(`speed=\s*([\d.]+)x`)
)

// ==================== 预处理事件常量 ====================

const (
	EventPreprocessStarted   = "preprocess_started"
	EventPreprocessProgress  = "preprocess_progress"
	EventPreprocessCompleted = "preprocess_completed"
	EventPreprocessFailed    = "preprocess_failed"
	EventPreprocessPaused    = "preprocess_paused"
	EventPreprocessCancelled = "preprocess_cancelled"
)

// PreprocessProgressData 预处理进度事件数据
type PreprocessProgressData struct {
	TaskID     string  `json:"task_id"`
	MediaID    string  `json:"media_id"`
	MediaTitle string  `json:"media_title"`
	Status     string  `json:"status"`
	Phase      string  `json:"phase"`
	Progress   float64 `json:"progress"`
	Speed      string  `json:"speed,omitempty"`
	Message    string  `json:"message"`
	Error      string  `json:"error,omitempty"`
}

// PreprocessService 视频预处理服务
type PreprocessService struct {
	cfg        *config.Config
	repo       *repository.PreprocessRepo
	mediaRepo  *repository.MediaRepo
	abrService *ABRService
	logger     *zap.SugaredLogger
	wsHub      *WSHub

	// 工作协程控制
	workerCount int32       // 当前活跃工作协程数
	maxWorkers  int32       // 最大并发工作协程数（上限，固定为 1）
	curWorkers  int32       // 当前动态调整后的并发数
	jobQueue    chan string // 任务 ID 队列（单任务模式，仅允许 1 个任务同时处理）
	pausedJobs  sync.Map    // 暂停的任务 ID 集合
	cancelJobs  sync.Map    // 取消的任务 ID 集合
	runningJobs sync.Map    // 正在运行的任务 ID -> *exec.Cmd
	mu          sync.RWMutex

	// 动态负载调整
	lastLoadCheck time.Time
	hwAccel       string // 硬件加速模式

	// 进度广播节流
	lastBroadcast sync.Map // taskID -> time.Time（上次广播时间）
	lastDBWrite   sync.Map // taskID -> time.Time（上次数据库写入时间）

	// GPU 安全监控
	gpuMonitor *GPUMonitor
}

// NewPreprocessService 创建预处理服务
func NewPreprocessService(
	cfg *config.Config,
	repo *repository.PreprocessRepo,
	mediaRepo *repository.MediaRepo,
	abrService *ABRService,
	hwAccel string,
	logger *zap.SugaredLogger,
) *PreprocessService {
	// 根据配置设置 worker 数量
	maxWorkers := int32(cfg.App.MaxTranscodeJobs)
	if maxWorkers <= 0 {
		maxWorkers = 1
	}

	resourceLimit := cfg.App.ResourceLimit
	if resourceLimit <= 0 || resourceLimit > 80 {
		resourceLimit = 80
	}

	logger.Infof("预处理服务启动: maxWorkers=%d, resourceLimit=%d%%, hwAccel=%s",
		maxWorkers, resourceLimit, hwAccel)

	s := &PreprocessService{
		cfg:        cfg,
		repo:       repo,
		mediaRepo:  mediaRepo,
		abrService: abrService,
		logger:     logger,
		maxWorkers: maxWorkers,
		curWorkers: maxWorkers,
		jobQueue:   make(chan string, 500),
		hwAccel:    hwAccel,
	}

	// 启动工作协程池
	for i := int32(0); i < maxWorkers; i++ {
		go s.worker(int(i))
	}

	// 启动自动重试协程
	go s.retryLoop()

	// 恢复未完成的任务（按优先级排序入队）
	go s.recoverPendingTasks()

	// 启动动态负载调整协程
	go s.dynamicLoadAdjuster()

	return s
}

// SetGPUMonitor 设置 GPU 安全监控服务
func (s *PreprocessService) SetGPUMonitor(monitor *GPUMonitor) {
	s.gpuMonitor = monitor
}

// SetWSHub 设置 WebSocket Hub
func (s *PreprocessService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// ==================== 公开 API ====================

// SubmitMedia 提交单个媒体进行预处理
func (s *PreprocessService) SubmitMedia(mediaID string, priority int) (*model.PreprocessTask, error) {
	// 检查是否已有活跃任务
	existing, err := s.repo.FindActiveByMediaID(mediaID)
	if err == nil && existing != nil {
		return existing, nil // 已有任务，直接返回
	}

	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, fmt.Errorf("媒体不存在: %w", err)
	}

	// STRM 远程流不支持预处理
	if media.StreamURL != "" {
		return nil, fmt.Errorf("STRM 远程流不支持预处理")
	}

	// 检查文件是否存在
	if _, err := os.Stat(media.FilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("媒体文件不存在: %s", media.FilePath)
	}

	outputDir := filepath.Join(s.cfg.Cache.CacheDir, "preprocess", mediaID)
	os.MkdirAll(outputDir, 0755)

	task := &model.PreprocessTask{
		MediaID:    mediaID,
		Status:     "pending",
		Priority:   priority,
		Message:    "等待处理...",
		InputPath:  media.FilePath,
		OutputDir:  outputDir,
		MediaTitle: media.DisplayTitle(),
		MaxRetry:   3,
	}

	if err := s.repo.Create(task); err != nil {
		return nil, fmt.Errorf("创建预处理任务失败: %w", err)
	}

	// 优先级入队：高优先级任务直接入队，低优先级任务在队列满时等待
	// 任务优先级规则：
	//   priority > 0: 高优先级（用户手动提交）
	//   priority = 0: 普通优先级（默认）
	//   priority < 0: 低优先级（后台自动任务）
	// 注意：由于单任务模式，队列中的任务会按 FIFO 顺序消费，
	// 但 recoverPendingTasks 和 retryLoop 会按优先级排序重新入队
	select {
	case s.jobQueue <- task.ID:
		s.logger.Infof("任务已入队: %s (优先级=%d)", task.MediaTitle, priority)
	default:
		s.logger.Warnf("预处理队列已满，任务 %s 将在下次调度时处理", task.ID)
	}

	return task, nil
}

// BatchSubmit 批量提交预处理任务
func (s *PreprocessService) BatchSubmit(mediaIDs []string, priority int) ([]*model.PreprocessTask, error) {
	var tasks []*model.PreprocessTask
	for _, id := range mediaIDs {
		task, err := s.SubmitMedia(id, priority)
		if err != nil {
			s.logger.Warnf("批量提交跳过 %s: %v", id, err)
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// SubmitLibrary 提交整个媒体库的所有视频进行预处理
func (s *PreprocessService) SubmitLibrary(libraryID string, priority int) (int, error) {
	// 查找媒体库中所有视频
	medias, err := s.mediaRepo.ListByLibraryID(libraryID)
	if err != nil {
		return 0, fmt.Errorf("查询媒体库失败: %w", err)
	}

	count := 0
	for _, media := range medias {
		// 跳过 STRM 远程流
		if media.StreamURL != "" {
			continue
		}
		// 跳过已有活跃任务的
		if _, err := s.repo.FindActiveByMediaID(media.ID); err == nil {
			continue
		}
		// 跳过已完成预处理的
		if existing, err := s.repo.FindByMediaID(media.ID); err == nil && existing.Status == "completed" {
			continue
		}

		if _, err := s.SubmitMedia(media.ID, priority); err == nil {
			count++
		}
	}

	return count, nil
}

// PauseTask 暂停任务
func (s *PreprocessService) PauseTask(taskID string) error {
	task, err := s.repo.FindByID(taskID)
	if err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	if task.Status != "running" && task.Status != "pending" && task.Status != "queued" {
		return fmt.Errorf("任务状态 %s 不可暂停", task.Status)
	}

	s.pausedJobs.Store(taskID, true)
	task.Status = "paused"
	task.Message = "已暂停"
	s.repo.Update(task)

	s.broadcastEvent(EventPreprocessPaused, task)
	return nil
}

// ResumeTask 恢复任务
func (s *PreprocessService) ResumeTask(taskID string) error {
	task, err := s.repo.FindByID(taskID)
	if err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	if task.Status != "paused" {
		return fmt.Errorf("任务状态 %s 不可恢复", task.Status)
	}

	s.pausedJobs.Delete(taskID)
	task.Status = "queued"
	task.Message = "已恢复，等待处理..."
	s.repo.Update(task)

	// 重新入队
	select {
	case s.jobQueue <- task.ID:
	default:
	}

	return nil
}

// CancelTask 取消任务
func (s *PreprocessService) CancelTask(taskID string) error {
	task, err := s.repo.FindByID(taskID)
	if err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	if task.Status == "completed" || task.Status == "cancelled" {
		return fmt.Errorf("任务状态 %s 不可取消", task.Status)
	}

	s.cancelJobs.Store(taskID, true)
	s.pausedJobs.Delete(taskID)

	// 终止正在运行的 FFmpeg 进程
	if cmdVal, ok := s.runningJobs.Load(taskID); ok {
		if cmd, ok := cmdVal.(*exec.Cmd); ok && cmd.Process != nil {
			cmd.Process.Kill()
			s.logger.Infof("已终止预处理任务 %s 的 FFmpeg 进程", taskID)
		}
	}

	task.Status = "cancelled"
	task.Message = "已取消"
	s.repo.Update(task)

	s.broadcastEvent(EventPreprocessCancelled, task)
	return nil
}

// RetryTask 重试失败的任务
func (s *PreprocessService) RetryTask(taskID string) error {
	task, err := s.repo.FindByID(taskID)
	if err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	if task.Status != "failed" {
		return fmt.Errorf("只有失败的任务可以重试")
	}

	task.Status = "queued"
	task.Error = ""
	task.Message = "重试中..."
	task.Retries++
	s.repo.Update(task)

	select {
	case s.jobQueue <- task.ID:
	default:
	}

	return nil
}

// GetTask 获取任务详情
func (s *PreprocessService) GetTask(taskID string) (*model.PreprocessTask, error) {
	return s.repo.FindByID(taskID)
}

// GetMediaTask 获取媒体的预处理任务
func (s *PreprocessService) GetMediaTask(mediaID string) (*model.PreprocessTask, error) {
	return s.repo.FindByMediaID(mediaID)
}

// ListTasks 分页获取任务列表
func (s *PreprocessService) ListTasks(page, pageSize int, status string) ([]model.PreprocessTask, int64, error) {
	tasks, total, err := s.repo.ListAll(page, pageSize, status)
	if err != nil {
		return tasks, total, err
	}
	// 用关联的 Media 信息补充/修正 media_title（兼容旧任务缺少集数信息的情况）
	for i := range tasks {
		if tasks[i].Media.ID != "" {
			tasks[i].MediaTitle = tasks[i].Media.DisplayTitle()
		}
	}
	return tasks, total, err
}

// GetStatistics 获取预处理统计
func (s *PreprocessService) GetStatistics() map[string]interface{} {
	counts, _ := s.repo.CountByStatus()
	running, _ := s.repo.ListRunning()

	return map[string]interface{}{
		"status_counts":  counts,
		"running_count":  len(running),
		"max_workers":    atomic.LoadInt32(&s.maxWorkers),
		"active_workers": atomic.LoadInt32(&s.workerCount),
		"queue_size":     len(s.jobQueue),
		"hw_accel":       s.hwAccel,
		"mode":           "dynamic", // 动态资源调整模式
		"resource_limit": s.cfg.App.ResourceLimit,
	}
}

// DeleteTask 删除任务（仅终态任务可删除）
func (s *PreprocessService) DeleteTask(taskID string) error {
	task, err := s.repo.FindByID(taskID)
	if err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	if task.Status == "running" {
		return fmt.Errorf("运行中的任务不可删除，请先取消")
	}

	return s.repo.DeleteByID(taskID)
}

// BatchDeleteTasks 批量删除任务（跳过运行中的任务）
func (s *PreprocessService) BatchDeleteTasks(taskIDs []string) (int64, error) {
	if len(taskIDs) == 0 {
		return 0, nil
	}
	return s.repo.DeleteByIDs(taskIDs)
}

// BatchCancelTasks 批量取消任务
func (s *PreprocessService) BatchCancelTasks(taskIDs []string) (int, error) {
	if len(taskIDs) == 0 {
		return 0, nil
	}
	cancelled := 0
	for _, id := range taskIDs {
		if err := s.CancelTask(id); err == nil {
			cancelled++
		}
	}
	return cancelled, nil
}

// BatchRetryTasks 批量重试任务
func (s *PreprocessService) BatchRetryTasks(taskIDs []string) (int, error) {
	if len(taskIDs) == 0 {
		return 0, nil
	}
	retried := 0
	for _, id := range taskIDs {
		if err := s.RetryTask(id); err == nil {
			retried++
		}
	}
	return retried, nil
}

// IsPreprocessed 检查媒体是否已完成预处理
func (s *PreprocessService) IsPreprocessed(mediaID string) bool {
	task, err := s.repo.FindByMediaID(mediaID)
	if err != nil {
		return false
	}
	return task.Status == "completed" && task.HLSMasterPath != ""
}

// GetPreprocessedMasterPath 获取预处理后的 HLS 主播放列表路径
func (s *PreprocessService) GetPreprocessedMasterPath(mediaID string) (string, error) {
	task, err := s.repo.FindByMediaID(mediaID)
	if err != nil {
		return "", fmt.Errorf("未找到预处理任务")
	}
	if task.Status != "completed" || task.HLSMasterPath == "" {
		return "", fmt.Errorf("预处理未完成")
	}
	if _, err := os.Stat(task.HLSMasterPath); os.IsNotExist(err) {
		return "", fmt.Errorf("预处理文件已丢失")
	}
	return task.HLSMasterPath, nil
}

// CleanPreprocessCache 清理指定媒体的预处理缓存
func (s *PreprocessService) CleanPreprocessCache(mediaID string) error {
	cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "preprocess", mediaID)
	if err := os.RemoveAll(cacheDir); err != nil {
		return err
	}
	// 更新任务状态
	task, err := s.repo.FindByMediaID(mediaID)
	if err == nil {
		task.Status = "pending"
		task.HLSMasterPath = ""
		task.Variants = ""
		task.ThumbnailPath = ""
		task.KeyframesDir = ""
		task.Progress = 0
		task.Message = "缓存已清理，等待重新处理"
		s.repo.Update(task)
	}
	return nil
}

// ==================== 工作协程 ====================

func (s *PreprocessService) worker(id int) {
	s.logger.Infof("预处理工作协程 #%d 启动", id)
	for taskID := range s.jobQueue {
		// 检查是否已取消
		if _, cancelled := s.cancelJobs.LoadAndDelete(taskID); cancelled {
			continue
		}
		// 检查是否已暂停
		if _, paused := s.pausedJobs.Load(taskID); paused {
			continue
		}

		// 动态并发控制：当 curWorkers 不足时，等待负载下降
		for int32(id) >= atomic.LoadInt32(&s.curWorkers) {
			time.Sleep(5 * time.Second)
			// 再次检查取消状态
			if _, cancelled := s.cancelJobs.Load(taskID); cancelled {
				break
			}
		}

		// 任务优先级检查：在开始处理前，查看是否有更高优先级的任务在等待
		if s.shouldYieldToHigherPriority(taskID) {
			select {
			case s.jobQueue <- taskID:
			default:
			}
			continue
		}

		atomic.AddInt32(&s.workerCount, 1)
		s.processTask(taskID)
		atomic.AddInt32(&s.workerCount, -1)

		// 清理节流缓存
		s.lastBroadcast.Delete(taskID)
		s.lastBroadcast.Delete(taskID + "_phase")
		s.lastDBWrite.Delete(taskID)
	}
}

// shouldYieldToHigherPriority 检查是否应该让位给更高优先级的任务
// 查询数据库中是否有优先级更高的等待任务
func (s *PreprocessService) shouldYieldToHigherPriority(currentTaskID string) bool {
	currentTask, err := s.repo.FindByID(currentTaskID)
	if err != nil {
		return false
	}

	// 查询队列中优先级最高的任务
	pendingTasks, err := s.repo.ListPending(1)
	if err != nil || len(pendingTasks) == 0 {
		return false
	}

	// 如果队列中有优先级更高的任务，当前任务应让位
	if pendingTasks[0].Priority > currentTask.Priority && pendingTasks[0].ID != currentTaskID {
		s.logger.Infof("任务优先级调度: %s(优先级=%d) 让位给 %s(优先级=%d)",
			currentTask.MediaTitle, currentTask.Priority,
			pendingTasks[0].MediaTitle, pendingTasks[0].Priority)
		return true
	}

	return false
}

func (s *PreprocessService) processTask(taskID string) {
	task, err := s.repo.FindByID(taskID)
	if err != nil {
		s.logger.Warnf("预处理任务不存在: %s", taskID)
		return
	}

	// 再次检查状态
	if task.Status == "cancelled" || task.Status == "completed" {
		return
	}

	now := time.Now()
	task.Status = "running"
	task.StartedAt = &now
	task.Message = "开始预处理..."
	s.repo.Update(task)

	s.broadcastEvent(EventPreprocessStarted, task)
	s.logger.Infof("开始预处理: %s (%s)", task.MediaTitle, task.MediaID)

	// ========== Phase 1: 探测视频信息 ==========
	if s.isCancelled(taskID) {
		return
	}
	s.updatePhase(task, "probe", 5, "正在探测视频信息...")

	probeInfo, err := s.probeVideo(task.InputPath)
	if err != nil {
		s.failTask(task, fmt.Sprintf("视频探测失败: %v", err))
		return
	}

	task.SourceWidth = probeInfo.width
	task.SourceHeight = probeInfo.height
	task.SourceCodec = probeInfo.codec
	task.SourceDuration = probeInfo.duration
	task.SourceSize = probeInfo.size
	s.repo.Update(task)

	// ========== Phase 2+3: 封面提取 & 关键帧预览（P2 优化：并行执行） ==========
	if s.isCancelled(taskID) {
		return
	}
	s.updatePhase(task, "thumbnail_keyframes", 10, "正在并行提取封面和关键帧...")

	var (
		thumbnailPath string
		keyframesDir  string
		thumbnailErr  error
		keyframesErr  error
		phase23Wg     sync.WaitGroup
	)

	// P2 优化：Phase 2 和 Phase 3 互不依赖，并行执行可节省约 50% 的预处理前期时间
	phase23Wg.Add(2)

	go func() {
		defer phase23Wg.Done()
		thumbnailPath, thumbnailErr = s.extractThumbnail(task)
	}()

	go func() {
		defer phase23Wg.Done()
		keyframesDir, keyframesErr = s.extractKeyframes(task)
	}()

	phase23Wg.Wait()

	if thumbnailErr != nil {
		s.logger.Warnf("封面提取失败（非致命）: %v", thumbnailErr)
	} else {
		task.ThumbnailPath = thumbnailPath
	}
	if keyframesErr != nil {
		s.logger.Warnf("关键帧提取失败（非致命）: %v", keyframesErr)
	} else {
		task.KeyframesDir = keyframesDir
	}
	s.repo.Update(task)

	// ========== Phase 4: 多码率 HLS 并行转码（P0 优化） ==========
	if s.isCancelled(taskID) {
		return
	}

	// 确定需要生成的变体（不超过源分辨率）
	variants := s.determineVariants(probeInfo.height)
	totalVariants := len(variants)
	completedVariants := []string{}

	// 根据资源限制配置动态计算并行度
	resourceLimit := s.cfg.App.ResourceLimit
	if resourceLimit <= 0 || resourceLimit > 80 {
		resourceLimit = 80
	}
	// 资源限制低于 30% 时串行转码，否则允许并行
	maxParallel := 1
	if resourceLimit >= 30 && s.hwAccel != "none" {
		maxParallel = 2 // GPU 加速时允许更多并行
	}
	if maxParallel > totalVariants {
		maxParallel = totalVariants
	}

	type variantResult struct {
		index int
		name  string
		err   error
	}

	resultCh := make(chan variantResult, totalVariants)
	sem := make(chan struct{}, maxParallel)
	var transcodeWg sync.WaitGroup

	for i, variant := range variants {
		if s.isCancelled(taskID) {
			break
		}
		if s.isPaused(taskID) {
			task.Status = "paused"
			task.Message = fmt.Sprintf("已暂停（已提交 %d/%d 变体）", i, totalVariants)
			s.repo.Update(task)
			s.broadcastEvent(EventPreprocessPaused, task)
			// 等待已启动的变体完成
			transcodeWg.Wait()
			return
		}

		transcodeWg.Add(1)
		sem <- struct{}{} // 获取信号量

		go func(idx int, v ABRProfile) {
			defer transcodeWg.Done()
			defer func() { <-sem }() // 释放信号量

			// 计算总体进度：20% ~ 95%（转码占 75%）
			baseProgress := 20.0 + float64(idx)/float64(totalVariants)*75.0
			phaseName := fmt.Sprintf("transcode_%s", v.Name)

			s.updatePhase(task, phaseName, baseProgress,
				fmt.Sprintf("正在转码 %s (%d/%d)...", v.Name, idx+1, totalVariants))

			err := s.transcodeVariant(task, v, func(progress float64, speed string) {
				// 变体内部进度映射到总体进度
				variantProgress := baseProgress + (progress/100.0)*(75.0/float64(totalVariants))
				s.updatePhase(task, phaseName, variantProgress,
					fmt.Sprintf("转码 %s: %.1f%% (速度: %s)", v.Name, progress, speed))
			})

			resultCh <- variantResult{index: idx, name: v.Name, err: err}
		}(i, variant)
	}

	// 等待所有并行转码完成
	transcodeWg.Wait()
	close(resultCh)

	// 收集结果
	for r := range resultCh {
		if r.err != nil {
			s.logger.Errorf("转码变体 %s 失败: %v", r.name, r.err)
			continue
		}
		completedVariants = append(completedVariants, r.name)
	}

	if len(completedVariants) == 0 {
		s.failTask(task, "所有转码变体均失败")
		return
	}

	// ========== Phase 5: 生成 ABR 主播放列表 ==========
	if s.isCancelled(taskID) {
		return
	}
	s.updatePhase(task, "abr_master", 96, "正在生成自适应码率播放列表...")

	masterPath, err := s.generateMasterPlaylist(task, completedVariants)
	if err != nil {
		s.failTask(task, fmt.Sprintf("生成主播放列表失败: %v", err))
		return
	}

	// ========== 完成 ==========
	completedAt := time.Now()
	elapsed := completedAt.Sub(*task.StartedAt).Seconds()

	variantsJSON, _ := json.Marshal(completedVariants)
	task.Status = "completed"
	task.Phase = "done"
	task.Progress = 100
	task.HLSMasterPath = masterPath
	task.Variants = string(variantsJSON)
	task.CompletedAt = &completedAt
	task.ElapsedSec = elapsed
	if probeInfo.duration > 0 {
		task.SpeedRatio = math.Round(probeInfo.duration/elapsed*100) / 100
	}
	task.Message = fmt.Sprintf("预处理完成（%d 个变体，耗时 %.0f 秒）", len(completedVariants), elapsed)
	s.repo.Update(task)

	s.broadcastEvent(EventPreprocessCompleted, task)
	s.logger.Infof("预处理完成: %s, 变体: %v, 耗时: %.1fs", task.MediaTitle, completedVariants, elapsed)
}

// ==================== 内部方法 ====================

type videoProbeInfo struct {
	width    int
	height   int
	codec    string
	duration float64
	size     int64
}

// probeVideo 使用 FFprobe 探测视频信息
func (s *PreprocessService) probeVideo(inputPath string) (*videoProbeInfo, error) {
	cmd := exec.Command(s.cfg.App.FFprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-select_streams", "v:0",
		inputPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("FFprobe 执行失败: %w", err)
	}

	var probeResult struct {
		Streams []struct {
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			CodecName string `json:"codec_name"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			Size     string `json:"size"`
		} `json:"format"`
	}

	if err := json.Unmarshal(output, &probeResult); err != nil {
		return nil, fmt.Errorf("解析 FFprobe 输出失败: %w", err)
	}

	info := &videoProbeInfo{}
	if len(probeResult.Streams) > 0 {
		info.width = probeResult.Streams[0].Width
		info.height = probeResult.Streams[0].Height
		info.codec = probeResult.Streams[0].CodecName
	}
	if d, err := strconv.ParseFloat(probeResult.Format.Duration, 64); err == nil {
		info.duration = d
	}
	if sz, err := strconv.ParseInt(probeResult.Format.Size, 10, 64); err == nil {
		info.size = sz
	}

	return info, nil
}

// extractThumbnail 提取视频封面（取 10% 位置的帧）
// P2 优化：将 -ss 放在 -i 之前实现 input seeking（快速跳转，避免解码前面所有帧）
func (s *PreprocessService) extractThumbnail(task *model.PreprocessTask) (string, error) {
	thumbnailPath := filepath.Join(task.OutputDir, "thumbnail.jpg")

	// 取视频 10% 位置的帧作为封面
	seekPos := "10"
	if task.SourceDuration > 0 {
		seekPos = fmt.Sprintf("%.0f", task.SourceDuration*0.1)
	}

	// P2 优化：-ss 在 -i 之前 = input seeking（基于关键帧快速跳转）
	// 比 output seeking（-ss 在 -i 之后）快 10~100x，尤其对长视频
	cmd := exec.Command(s.cfg.App.FFmpegPath,
		"-y",
		"-ss", seekPos,
		"-i", task.InputPath,
		"-frames:v", "1",
		"-q:v", "2",
		"-vf", "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease",
		thumbnailPath,
	)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("封面提取失败: %w", err)
	}

	return thumbnailPath, nil
}

// extractKeyframes 提取关键帧预览（每 N 秒一帧，最多 20 帧）
// P1 优化：使用 -skip_frame nokey 跳过非关键帧解码，大幅减少解码开销
func (s *PreprocessService) extractKeyframes(task *model.PreprocessTask) (string, error) {
	keyframesDir := filepath.Join(task.OutputDir, "keyframes")
	os.MkdirAll(keyframesDir, 0755)

	duration := task.SourceDuration
	if duration <= 0 {
		duration = 600 // 默认 10 分钟
	}

	// 计算间隔：确保最多 20 帧
	interval := duration / 20
	if interval < 30 {
		interval = 30
	}

	// P1 优化：使用 -skip_frame nokey 让解码器跳过非关键帧，
	// 配合 fps 滤镜大幅减少解码工作量（对长视频提升 5~10x）
	cmd := exec.Command(s.cfg.App.FFmpegPath,
		"-y",
		"-skip_frame", "nokey",
		"-i", task.InputPath,
		"-vf", fmt.Sprintf("fps=1/%.0f,scale=320:-1", interval),
		"-vsync", "vfr",
		"-q:v", "5",
		"-frames:v", "20",
		filepath.Join(keyframesDir, "kf_%03d.jpg"),
	)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("关键帧提取失败: %w", err)
	}

	return keyframesDir, nil
}

// transcodeVariant 转码单个变体，支持硬件加速失败时回退到软件转码
func (s *PreprocessService) transcodeVariant(
	task *model.PreprocessTask,
	variant ABRProfile,
	onProgress func(progress float64, speed string),
) error {
	variantDir := filepath.Join(task.OutputDir, "hls", variant.Name)
	os.MkdirAll(variantDir, 0755)

	m3u8Path := filepath.Join(variantDir, "stream.m3u8")

	// 检查是否已有缓存
	if _, err := os.Stat(m3u8Path); err == nil {
		onProgress(100, "cached")
		return nil
	}

	segmentPath := filepath.Join(variantDir, "seg%04d.ts")

	// HLS 输出参数
	hlsArgs := []string{
		"-f", "hls",
		"-hls_time", "4",
		"-hls_list_size", "0",
		"-hls_segment_filename", segmentPath,
		"-hls_flags", "independent_segments",
		m3u8Path,
	}

	audioArgs := []string{"-c:a", "aac", "-b:a", variant.AudioBitrate, "-ac", "2"}

	// 尝试硬件加速转码，失败时回退到软件转码
	return s.transcodeWithFallback(task, variant, hlsArgs, audioArgs, m3u8Path, segmentPath, onProgress)
}

// transcodeWithFallback 尝试硬件加速，失败时回退到软件转码
func (s *PreprocessService) transcodeWithFallback(
	task *model.PreprocessTask,
	variant ABRProfile,
	hlsArgs, audioArgs []string,
	m3u8Path, segmentPath string,
	onProgress func(progress float64, speed string),
) error {
	// 尝试硬件加速配置
	hwAccelAttempts := []struct {
		name   string
		config func() ([]string, []string, []string)
	}{
		{
			name: "qsv",
			config: func() ([]string, []string, []string) {
				baseArgs := []string{"-y", "-hwaccel", "qsv", "-hwaccel_output_format", "qsv", "-i", task.InputPath}
				videoArgs := []string{
					"-c:v", "h264_qsv",
					"-preset", "medium",
					"-b:v", variant.VideoBitrate,
					"-maxrate", variant.MaxBitrate,
					"-bufsize", variant.BufSize,
					"-vf", fmt.Sprintf("scale_qsv=%d:%d", variant.Width, variant.Height),
					"-g", "48",
					"-keyint_min", "48",
				}
				return baseArgs, videoArgs, audioArgs
			},
		},
		{
			name: "vaapi",
			config: func() ([]string, []string, []string) {
				baseArgs := []string{
					"-y",
					"-hwaccel", "vaapi",
					"-hwaccel_output_format", "vaapi",
					"-vaapi_device", s.cfg.App.VAAPIDevice,
					"-i", task.InputPath,
				}
				videoArgs := []string{
					"-c:v", "h264_vaapi",
					"-b:v", variant.VideoBitrate,
					"-maxrate", variant.MaxBitrate,
					"-bufsize", variant.BufSize,
					"-vf", fmt.Sprintf("scale_vaapi=w=%d:h=%d", variant.Width, variant.Height),
					"-g", "48",
					"-keyint_min", "48",
				}
				return baseArgs, videoArgs, audioArgs
			},
		},
		{
			name: "none",
			config: func() ([]string, []string, []string) {
				baseArgs := []string{"-y", "-i", task.InputPath}
				videoArgs := []string{
					"-c:v", "libx264",
					"-preset", s.cfg.App.TranscodePreset,
					"-b:v", variant.VideoBitrate,
					"-maxrate", variant.MaxBitrate,
					"-bufsize", variant.BufSize,
					"-vf", fmt.Sprintf("scale=%d:%d", variant.Width, variant.Height),
					"-g", "48",
					"-keyint_min", "48",
					"-sc_threshold", "0",
				}
				return baseArgs, videoArgs, audioArgs
			},
		},
	}

	// GPU 安全保护：检查是否需要降级为 CPU 编码
	actualHWAccel := s.hwAccel
	if s.gpuMonitor != nil && s.hwAccel != "none" {
		useGPU, accel := s.gpuMonitor.ShouldUseGPU(s.hwAccel)
		if !useGPU {
			actualHWAccel = accel // 降级为 "none"（CPU 编码）
			s.logger.Warnf("GPU 安全保护: 任务 %s 降级为 CPU 编码", task.MediaTitle)
		}
	}

	// 根据当前硬件加速模式确定尝试顺序
	var attempts []string
	switch actualHWAccel {
	case "qsv":
		attempts = []string{"qsv", "vaapi", "none"}
	case "vaapi":
		attempts = []string{"vaapi", "qsv", "none"}
	default:
		attempts = []string{"none"}
	}

	// 尝试不同的转码方式
	for _, attemptName := range attempts {
		var configFunc func() ([]string, []string, []string)
		for _, hw := range hwAccelAttempts {
			if hw.name == attemptName {
				configFunc = hw.config
				break
			}
		}
		
		if configFunc == nil {
			continue
		}

		baseArgs, videoArgs, audioArgs := configFunc()
		
		// 根据资源限制配置动态计算 FFmpeg 线程数
		ffmpegThreads := s.calcFFmpegThreads()
		args := append(baseArgs, "-threads", fmt.Sprintf("%d", ffmpegThreads))
		args = append(args, videoArgs...)
		args = append(args, audioArgs...)
		args = append(args, hlsArgs...)

		cmd := exec.Command(s.cfg.App.FFmpegPath, args...)

		// 极低资源模式：设置 FFmpeg 进程为最低优先级（nice 19）
		// 确保转码进程不会抢占其他系统进程的 CPU 时间
		setLowPriority(cmd)

		// 解析进度
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			s.logger.Warnf("创建 stderr 管道失败: %v", err)
			continue
		}

		if err := cmd.Start(); err != nil {
			s.logger.Debugf("硬件加速 %s 启动失败: %v", attemptName, err)
			continue // 尝试下一种方式
		}

		// 存储进程引用用于取消
		s.runningJobs.Store(task.ID, cmd)
		
		// 启动进度解析协程
		progressDone := make(chan struct{})
		go func() {
			defer close(progressDone)
			s.parseFFmpegProgress(stderrPipe, task.SourceDuration, onProgress)
		}()

		// 等待命令完成
		err = cmd.Wait()
		s.runningJobs.Delete(task.ID)
		<-progressDone // 等待进度解析完成

		if err != nil {
			// 检查是否是被取消
			if _, cancelled := s.cancelJobs.Load(task.ID); cancelled {
				return fmt.Errorf("任务已取消")
			}
			
			s.logger.Debugf("硬件加速 %s 转码失败: %v", attemptName, err)
			
			// 如果不是最后一次尝试，继续尝试下一种方式
			if attemptName != attempts[len(attempts)-1] {
				// 清理当前尝试的文件
				os.Remove(m3u8Path)
				os.RemoveAll(filepath.Dir(segmentPath))
				continue
			}
			
			// 所有尝试都失败了
			return fmt.Errorf("所有转码方式均失败，最后错误: %w", err)
		}

		// 成功完成转码
		s.logger.Infof("使用 %s 成功转码变体 %s", attemptName, variant.Name)
		return nil
	}

	return fmt.Errorf("所有转码方式均失败")
}

// parseFFmpegProgress 解析 FFmpeg stderr 进度输出
// P2 优化：使用 bufio.Scanner 按行扫描，避免原始 Read 截断行导致正则匹配失败
// P3 优化：使用预编译正则 + 进度变化阈值过滤，减少无效回调
func (s *PreprocessService) parseFFmpegProgress(stderr io.ReadCloser, totalDuration float64, onProgress func(float64, string)) {
	scanner := bufio.NewScanner(stderr)
	// FFmpeg 的 stderr 输出使用 \r 作为行分隔符，需要自定义 split 函数
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		// 查找 \r 或 \n 作为行分隔符
		for i := 0; i < len(data); i++ {
			if data[i] == '\n' || data[i] == '\r' {
				return i + 1, data[:i], nil
			}
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	})

	var lastProgress float64

	for scanner.Scan() {
		line := scanner.Text()

		timeMatches := ffmpegTimeRegex.FindStringSubmatch(line)
		if len(timeMatches) < 5 || totalDuration <= 0 {
			continue
		}

		hours, _ := strconv.ParseFloat(timeMatches[1], 64)
		minutes, _ := strconv.ParseFloat(timeMatches[2], 64)
		seconds, _ := strconv.ParseFloat(timeMatches[3], 64)
		centis, _ := strconv.ParseFloat(timeMatches[4], 64)
		currentTime := hours*3600 + minutes*60 + seconds + centis/100
		progress := (currentTime / totalDuration) * 100
		if progress > 100 {
			progress = 100
		}

		// P3 优化：进度变化不超过 0.5% 时跳过，减少无效回调和下游节流压力
		if progress-lastProgress < 0.5 {
			continue
		}
		lastProgress = progress

		speed := ""
		speedMatches := ffmpegSpeedRegex.FindStringSubmatch(line)
		if len(speedMatches) >= 2 {
			speed = speedMatches[1] + "x"
		}

		onProgress(progress, speed)
	}
}

// generateMasterPlaylist 生成 ABR 主播放列表
func (s *PreprocessService) generateMasterPlaylist(task *model.PreprocessTask, variants []string) (string, error) {
	hlsDir := filepath.Join(task.OutputDir, "hls")
	masterPath := filepath.Join(hlsDir, "master.m3u8")

	var content strings.Builder
	content.WriteString("#EXTM3U\n")
	content.WriteString("#EXT-X-VERSION:3\n\n")

	for _, name := range variants {
		// 查找对应的 ABR profile
		var profile *ABRProfile
		for _, p := range abrProfiles {
			if p.Name == name {
				profile = &p
				break
			}
		}
		if profile == nil {
			continue
		}

		bandwidth := parseBitrate(profile.VideoBitrate) + parseBitrate(profile.AudioBitrate)
		content.WriteString(fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,NAME=\"%s\"\n",
			bandwidth, profile.Width, profile.Height, profile.Name,
		))
		content.WriteString(fmt.Sprintf("%s/stream.m3u8\n\n", profile.Name))
	}

	if err := os.WriteFile(masterPath, []byte(content.String()), 0644); err != nil {
		return "", fmt.Errorf("写入主播放列表失败: %w", err)
	}

	return masterPath, nil
}

// determineVariants 根据源视频高度确定需要生成的变体
func (s *PreprocessService) determineVariants(sourceHeight int) []ABRProfile {
	var variants []ABRProfile
	for _, p := range abrProfiles {
		if p.Height <= sourceHeight {
			variants = append(variants, p)
		}
	}
	if len(variants) == 0 {
		variants = abrProfiles[:1] // 至少保留 360p
	}
	return variants
}

// ==================== 辅助方法 ====================

func (s *PreprocessService) isCancelled(taskID string) bool {
	_, cancelled := s.cancelJobs.Load(taskID)
	return cancelled
}

func (s *PreprocessService) isPaused(taskID string) bool {
	_, paused := s.pausedJobs.Load(taskID)
	return paused
}

// updatePhase 更新任务阶段和进度
// P1 优化：数据库写入和 WebSocket 广播分别节流，减少 80% 的数据库写操作
func (s *PreprocessService) updatePhase(task *model.PreprocessTask, phase string, progress float64, message string) {
	task.Phase = phase
	task.Progress = progress
	task.Message = message

	now := time.Now()

	// P1 优化：数据库写入节流 —— 每 5 秒最多写一次（阶段变化时强制写入）
	forceDBWrite := false
	if lastPhase, ok := s.lastBroadcast.Load(task.ID + "_phase"); !ok || lastPhase.(string) != phase {
		forceDBWrite = true // 阶段变化时强制写入
		s.lastBroadcast.Store(task.ID+"_phase", phase)
	}

	if forceDBWrite {
		s.repo.Update(task)
		s.lastDBWrite.Store(task.ID, now)
	} else if lastDB, ok := s.lastDBWrite.Load(task.ID); !ok || now.Sub(lastDB.(time.Time)) >= 5*time.Second {
		s.repo.Update(task)
		s.lastDBWrite.Store(task.ID, now)
	}

	// WebSocket 广播节流：每 2 秒最多广播一次
	if lastTime, ok := s.lastBroadcast.Load(task.ID); ok {
		if t, isTime := lastTime.(time.Time); isTime && now.Sub(t) < 2*time.Second {
			return // 跳过本次广播
		}
	}
	s.lastBroadcast.Store(task.ID, now)
	s.broadcastEvent(EventPreprocessProgress, task)
}

func (s *PreprocessService) failTask(task *model.PreprocessTask, errMsg string) {
	task.Status = "failed"
	task.Error = errMsg
	task.Message = errMsg
	s.repo.Update(task)

	s.broadcastEvent(EventPreprocessFailed, task)
	s.logger.Warnf("预处理失败: %s - %s", task.MediaTitle, errMsg)
}

func (s *PreprocessService) broadcastEvent(eventType string, task *model.PreprocessTask) {
	if s.wsHub == nil {
		return
	}
	s.wsHub.BroadcastEvent(eventType, PreprocessProgressData{
		TaskID:     task.ID,
		MediaID:    task.MediaID,
		MediaTitle: task.MediaTitle,
		Status:     task.Status,
		Phase:      task.Phase,
		Progress:   task.Progress,
		Message:    task.Message,
		Error:      task.Error,
	})
}

// retryLoop 自动重试失败的任务
func (s *PreprocessService) retryLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		tasks, err := s.repo.FindNeedRetry(3)
		if err != nil {
			continue
		}
		for _, task := range tasks {
			s.logger.Infof("自动重试预处理任务: %s (%s)", task.MediaTitle, task.ID)
			task.Status = "queued"
			task.Error = ""
			task.Message = fmt.Sprintf("自动重试（第 %d 次）...", task.Retries+1)
			task.Retries++
			s.repo.Update(&task)

			select {
			case s.jobQueue <- task.ID:
			default:
			}
		}
	}
}

// recoverPendingTasks 恢复服务重启前未完成的任务
func (s *PreprocessService) recoverPendingTasks() {
	time.Sleep(5 * time.Second) // 等待服务完全启动

	tasks, err := s.repo.ListPending(50)
	if err != nil {
		return
	}

	// 将之前 running 的任务重置为 queued
	running, _ := s.repo.ListRunning()
	for _, task := range running {
		task.Status = "queued"
		task.Message = "服务重启后恢复..."
		s.repo.Update(&task)
		tasks = append(tasks, task)
	}

	for _, task := range tasks {
		select {
		case s.jobQueue <- task.ID:
		default:
		}
	}

	if len(tasks) > 0 {
		s.logger.Infof("恢复 %d 个未完成的预处理任务", len(tasks))
	}
}

// GetSystemLoad 获取系统负载信息（用于动态调整并发）
func (s *PreprocessService) GetSystemLoad() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 获取 CPU 使用率
	cpuPercent := 0.0
	if percents, err := cpu.Percent(0, false); err == nil && len(percents) > 0 {
		cpuPercent = percents[0]
	}

	// 计算 FFmpeg 线程数（供前端展示）
	ffmpegThreads := s.calcFFmpegThreads()

	result := map[string]interface{}{
		"cpu_count":      runtime.NumCPU(),
		"cpu_percent":    cpuPercent,
		"goroutines":     runtime.NumGoroutine(),
		"mem_alloc_mb":   float64(memStats.Alloc) / 1024 / 1024,
		"mem_sys_mb":     float64(memStats.Sys) / 1024 / 1024,
		"active_workers": atomic.LoadInt32(&s.workerCount),
		"max_workers":    atomic.LoadInt32(&s.maxWorkers),
		"cur_workers":    atomic.LoadInt32(&s.curWorkers),
		"queue_size":     len(s.jobQueue),
		"resource_limit": s.cfg.App.ResourceLimit,
		"ffmpeg_threads": ffmpegThreads,
		"hw_accel":       s.hwAccel,
		"suggestions":    s.getPerformanceSuggestions(cpuPercent),
	}

	// 添加 GPU 实时指标
	if s.gpuMonitor != nil {
		gpuStatus := s.gpuMonitor.GetSafetyStatus()
		result["gpu_status"] = gpuStatus
	}

	return result
}

// getPerformanceSuggestions 根据当前系统状态生成性能优化建议
func (s *PreprocessService) getPerformanceSuggestions(cpuPercent float64) []string {
	var suggestions []string
	cpuCount := runtime.NumCPU()
	resourceLimit := s.cfg.App.ResourceLimit

	// CPU 核心数建议
	if cpuCount >= 8 && s.cfg.App.MaxTranscodeJobs == 1 {
		suggestions = append(suggestions, fmt.Sprintf("检测到 %d 核 CPU，建议将并行任务数提升至 2~%d 以充分利用多核性能", cpuCount, min(cpuCount/4, 4)))
	}

	// GPU 加速建议
	if s.hwAccel == "none" {
		suggestions = append(suggestions, "未检测到 GPU 硬件加速，建议安装 GPU 驱动以大幅提升转码速度")
	}

	// 资源限制建议
	if resourceLimit <= 20 {
		suggestions = append(suggestions, "当前资源限制较低（≤20%），转码速度可能较慢。如果系统负载允许，建议适当提高")
	}

	// CPU 使用率建议
	if cpuPercent > float64(resourceLimit)*0.9 && cpuPercent > 50 {
		suggestions = append(suggestions, "CPU 使用率接近资源限制上限，系统可能会频繁暂停转码任务")
	}

	// 转码预设建议
	if s.cfg.App.TranscodePreset == "medium" || s.cfg.App.TranscodePreset == "slow" {
		if cpuCount < 8 {
			suggestions = append(suggestions, "当前转码预设较慢，低核心数 CPU 建议使用 veryfast 或 fast 预设")
		}
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "当前配置良好，系统运行正常")
	}

	return suggestions
}

// GetPerformanceConfig 获取当前性能配置
func (s *PreprocessService) GetPerformanceConfig() map[string]interface{} {
	result := map[string]interface{}{
		"resource_limit":     s.cfg.App.ResourceLimit,
		"max_transcode_jobs": s.cfg.App.MaxTranscodeJobs,
		"transcode_preset":   s.cfg.App.TranscodePreset,
		"hw_accel":           s.cfg.App.HWAccel,
		"detected_hw_accel":  s.hwAccel, // 实际检测到的硬件加速模式
		"vaapi_device":       s.cfg.App.VAAPIDevice,
		"cpu_count":          runtime.NumCPU(),
		"ffmpeg_threads":     s.calcFFmpegThreads(),
		"max_workers":        atomic.LoadInt32(&s.maxWorkers),
	}

	// 添加 GPU 安全保护配置
	result["gpu_safety_enabled"] = s.cfg.App.GPUSafetyEnabled
	result["gpu_utilization_threshold"] = s.cfg.App.GPUUtilizationThreshold
	result["gpu_temperature_threshold"] = s.cfg.App.GPUTemperatureThreshold
	result["gpu_recovery_threshold"] = s.cfg.App.GPURecoveryThreshold
	result["gpu_temperature_recovery"] = s.cfg.App.GPUTemperatureRecovery

	// 添加 GPU 实时状态
	if s.gpuMonitor != nil {
		status := s.gpuMonitor.GetSafetyStatus()
		result["gpu_status"] = status
	}

	return result
}

// UpdatePerformanceConfig 更新性能配置（热更新，无需重启）
func (s *PreprocessService) UpdatePerformanceConfig(updates map[string]interface{}) error {
	// 持久化到配置文件
	if err := s.cfg.UpdatePerformanceConfig(updates); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	// 热更新运行时参数
	// max_transcode_jobs 的变更需要重启服务才能完全生效（worker 协程数量固定）
	// 但我们更新 maxWorkers 以确保前端显示一致
	if v, ok := updates["max_transcode_jobs"]; ok {
		if fv, ok := v.(float64); ok {
			newMax := int32(fv)
			if newMax >= 1 && newMax <= 16 {
				atomic.StoreInt32(&s.maxWorkers, newMax)
				// 同步调整当前并发数上限
				if atomic.LoadInt32(&s.curWorkers) > newMax {
					atomic.StoreInt32(&s.curWorkers, newMax)
				}
				s.logger.Infof("最大并行任务数已更新: %d（完全生效需重启服务）", newMax)
			}
		}
	}

	// 热更新 GPU 监控阈值（无需重启）
	if s.gpuMonitor != nil {
		needUpdateGPU := false
		for key := range updates {
			if strings.HasPrefix(key, "gpu_") {
				needUpdateGPU = true
				break
			}
		}
		if needUpdateGPU {
			newCfg := GPUThresholdConfig{
				UtilizationThreshold: s.cfg.App.GPUUtilizationThreshold,
				TemperatureThreshold: s.cfg.App.GPUTemperatureThreshold,
				RecoveryThreshold:    s.cfg.App.GPURecoveryThreshold,
				TemperatureRecovery:  s.cfg.App.GPUTemperatureRecovery,
				MonitorInterval:      s.cfg.App.GPUMonitorInterval,
				Enabled:              s.cfg.App.GPUSafetyEnabled,
			}
			s.gpuMonitor.UpdateConfig(newCfg)
		}
	}

	s.logger.Infof("性能配置已更新: %v", updates)
	return nil
}

// dynamicLoadAdjuster 动态负载调整协程
// 根据 CPU 使用率动态调整并发 worker 数量
// P1 优化：缩短检测周期到 15 秒，使用非阻塞 CPU 采样
func (s *PreprocessService) dynamicLoadAdjuster() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		percents, err := cpu.Percent(0, false)
		if err != nil || len(percents) == 0 {
			continue
		}

		cpuUsage := percents[0]
		current := atomic.LoadInt32(&s.curWorkers)
		max := atomic.LoadInt32(&s.maxWorkers)

		// 根据用户配置的资源限制动态调整 worker 数量
		resourceLimit := float64(s.cfg.App.ResourceLimit)
		if resourceLimit <= 0 || resourceLimit > 80 {
			resourceLimit = 80
		}

		var newWorkers int32
		switch {
		case cpuUsage > resourceLimit:
			// CPU 超过资源限制，暂停所有 worker
			newWorkers = 0
		case cpuUsage > resourceLimit*0.8:
			// CPU 接近限制（80%），减半 worker
			newWorkers = max / 2
			if newWorkers < 1 {
				newWorkers = 1
			}
		case cpuUsage > resourceLimit*0.6:
			// CPU 中等负载（60%），使用 75% worker
			newWorkers = max * 3 / 4
			if newWorkers < 1 {
				newWorkers = 1
			}
		default:
			// CPU 空闲，使用全部 worker
			newWorkers = max
		}

		if newWorkers != current {
			atomic.StoreInt32(&s.curWorkers, newWorkers)
			s.logger.Infof("动态调整并发: CPU %.1f%% (限制 %.0f%%), workers %d -> %d",
				cpuUsage, resourceLimit, current, newWorkers)
		}
	}
}

// calcFFmpegThreads 根据资源限制配置动态计算 FFmpeg 线程数
func (s *PreprocessService) calcFFmpegThreads() int {
	resourceLimit := s.cfg.App.ResourceLimit
	if resourceLimit <= 0 || resourceLimit > 80 {
		resourceLimit = 80
	}

	cpuCount := runtime.NumCPU()
	// 根据资源限制比例计算线程数
	// 例如: 28核 * 80% = 22 线程，28核 * 20% = 5 线程
	threads := cpuCount * resourceLimit / 100
	if threads < 1 {
		threads = 1
	}
	// 上限不超过 CPU 核心数
	if threads > cpuCount {
		threads = cpuCount
	}
	return threads
}
