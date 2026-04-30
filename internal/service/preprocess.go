package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	"github.com/shirou/gopsutil/v3/cpu"
	"go.uber.org/zap"
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

	// 系统设置仓储（延迟注入，用于读取 ABR 策略等全局设置）
	settingRepo *repository.SystemSettingRepo

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

	// V2.1: VFS 管理器（用于 WebDAV 路径验证）
	vfsMgr *VFSManager
}

// SetVFSManager 设置 VFS 管理器（V2.1）
func (s *PreprocessService) SetVFSManager(vfsMgr *VFSManager) {
	s.vfsMgr = vfsMgr
}

// SetSettingRepo 注入系统设置仓储（延迟注入，用于读取 ABR 策略等全局配置）
func (s *PreprocessService) SetSettingRepo(repo *repository.SystemSettingRepo) {
	s.settingRepo = repo
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
	// 【火力全开策略（方案1完整版）】worker 数量 = NumCPU，
	// 让每个 CPU 核心都能同时跑一个预处理任务。
	// FFmpeg 内部 threads=0（auto）和操作系统调度器会自动平衡状态，
	// 不会因为并发过度导致 context switch 爆炸。
	maxWorkers := int32(runtime.NumCPU())
	if maxWorkers < 1 {
		maxWorkers = 1
	}

	logger.Infof("预处理服务启动（火力全开模式）: maxWorkers=%d, hwAccel=%s, ffmpeg_threads=0(auto)",
		maxWorkers, hwAccel)

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

// SubmitMedia 提交单个媒体进行预处理。
//
// force 语义：
//   - false（自动调度路径使用）：如果媒体已经可以浏览器零转码直接播放
//     （mp4/H.264+AAC 等），则直接返回错误不入队，避免无谓的 CPU/GPU 消耗。
//   - true（用户手动点击"预处理/强制转码"按钮）：绕过上述判定，一律入队。
func (s *PreprocessService) SubmitMedia(mediaID string, priority int, force bool) (*model.PreprocessTask, error) {
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

	// 自动场景下，若浏览器可零转码直接播放则跳过，避免无谓消耗
	// 用户通过前端按钮手动触发（force=true）可以绕过该限制
	if !force && canMediaPlayDirectly(media) {
		return nil, fmt.Errorf("该媒体可在浏览器直接播放（%s / %s），无需预处理",
			media.VideoCodec, media.AudioCodec)
	}

	// V2.1: WebDAV 路径 → HTTP URL（FFmpeg 会直接流式读取，不需要全文件下载）
	inputPath := media.FilePath
	if IsWebDAVPath(inputPath) {
		// 通过 VFS 检查文件存在性
		if s.vfsMgr != nil {
			if _, err := s.vfsMgr.Stat(inputPath); err != nil {
				return nil, fmt.Errorf("WebDAV 媒体文件不可访问: %s, %w", inputPath, err)
			}
		}
		// 翻译为 HTTP URL（ffmpeg 直读）
		inputPath = ResolveRemoteFFmpegURL(s.cfg, inputPath)
		s.logger.Infof("WebDAV 预处理输入: %s", SprintSafeFFmpegURL(inputPath))
	} else {
		// 本地文件存在性检查
		if _, err := os.Stat(media.FilePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("媒体文件不存在: %s", media.FilePath)
		}
	}

	outputDir := filepath.Join(s.cfg.Cache.CacheDir, "preprocess", mediaID)
	os.MkdirAll(outputDir, 0755)

	task := &model.PreprocessTask{
		MediaID:    mediaID,
		Status:     "pending",
		Priority:   priority,
		Message:    "等待处理...",
		InputPath:  inputPath,
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
//
// force 语义同 SubmitMedia：
//   - false：自动跳过可直接播放的媒体；
//   - true：用户显式要求强制预处理所有指定媒体。
func (s *PreprocessService) BatchSubmit(mediaIDs []string, priority int, force bool) ([]*model.PreprocessTask, error) {
	var tasks []*model.PreprocessTask
	for _, id := range mediaIDs {
		task, err := s.SubmitMedia(id, priority, force)
		if err != nil {
			s.logger.Warnf("批量提交跳过 %s: %v", id, err)
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// SubmitLibrary 提交整个媒体库的所有视频进行预处理。
//
// 该路径始终以 force=false 调用 SubmitMedia：
// 已经能浏览器零转码播放的媒体（mp4/H.264+AAC/MKV+H.264+AAC 等）
// 不会被纳入预处理队列，避免 CPU/GPU 的无谓消耗。
// 用户如需强制预处理某个媒体，应走 MediaDetailPage 的"预处理/强制转码"按钮。
func (s *PreprocessService) SubmitLibrary(libraryID string, priority int) (int, error) {
	// 查找媒体库中所有视频
	medias, err := s.mediaRepo.ListByLibraryID(libraryID)
	if err != nil {
		return 0, fmt.Errorf("查询媒体库失败: %w", err)
	}

	count := 0
	skippedDirect := 0
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
		// 跳过浏览器可零转码直接播放的媒体（由 SubmitMedia 内部判定，这里提前剪枝减少日志噪声）
		if canMediaPlayDirectly(&media) {
			skippedDirect++
			continue
		}

		if _, err := s.SubmitMedia(media.ID, priority, false); err == nil {
			count++
		}
	}

	if skippedDirect > 0 {
		s.logger.Infof("媒体库 %s 自动预处理：跳过 %d 个可直接播放的媒体", libraryID, skippedDirect)
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
		"mode":           "best-performance", // 最佳性能模式
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

		// 【火力全开】移除原本的动态降速等待循环（原逻辑会 Sleep 5s）。
		// curWorkers 字段保留仅用于接口展示，不再影响实际调度。

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
		spritePath    string
		vttPath       string
		thumbnailErr  error
		keyframesErr  error
		spriteErr     error
		phase23Wg     sync.WaitGroup
	)

	// P2 优化：Phase 2、3、sprite 互不依赖，并行执行
	phase23Wg.Add(3)

	go func() {
		defer phase23Wg.Done()
		thumbnailPath, thumbnailErr = s.extractThumbnail(task)
	}()

	go func() {
		defer phase23Wg.Done()
		keyframesDir, keyframesErr = s.extractKeyframes(task)
	}()

	go func() {
		defer phase23Wg.Done()
		spritePath, vttPath, spriteErr = s.generateSprite(task)
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
	if spriteErr != nil {
		s.logger.Warnf("雪碧图生成失败（非致命）: %v", spriteErr)
	} else {
		task.SpritePath = spritePath
		task.SpriteVTTPath = vttPath
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

	// 【方案 1 全速策略】不再限制变体并行度：
	// 所有档位（1080p/720p/480p 等）同时编码，让 FFmpeg / GPU 自己调度。
	maxParallel := totalVariants
	if maxParallel < 1 {
		maxParallel = 1
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
	ffprobeArgs := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-select_streams", "v:0",
	}
	// V2.1: HTTP 输入时注入超时与重连参数
	if httpArgs := BuildFFmpegInputArgs(inputPath); len(httpArgs) > 0 {
		ffprobeArgs = append(ffprobeArgs, httpArgs...)
	}
	ffprobeArgs = append(ffprobeArgs, inputPath)
	cmd := exec.Command(s.cfg.App.FFprobePath, ffprobeArgs...)

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
	ffmpegArgs := []string{
		"-y",
		"-ss", seekPos,
	}
	// V2.1: HTTP 输入时注入重连与超时参数
	if httpArgs := BuildFFmpegInputArgs(task.InputPath); len(httpArgs) > 0 {
		ffmpegArgs = append(ffmpegArgs, httpArgs...)
	}
	ffmpegArgs = append(ffmpegArgs,
		"-i", task.InputPath,
		"-frames:v", "1",
		"-q:v", "2",
		"-vf", "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease",
		thumbnailPath,
	)
	cmd := exec.Command(s.cfg.App.FFmpegPath, ffmpegArgs...)

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
	kfArgs := []string{
		"-y",
		"-skip_frame", "nokey",
	}
	if httpArgs := BuildFFmpegInputArgs(task.InputPath); len(httpArgs) > 0 {
		kfArgs = append(kfArgs, httpArgs...)
	}
	kfArgs = append(kfArgs,
		"-i", task.InputPath,
		"-vf", fmt.Sprintf("fps=1/%.0f,scale=320:-1", interval),
		"-vsync", "vfr",
		"-q:v", "5",
		"-frames:v", "20",
		filepath.Join(keyframesDir, "kf_%03d.jpg"),
	)
	cmd := exec.Command(s.cfg.App.FFmpegPath, kfArgs...)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("关键帧提取失败: %w", err)
	}

	return keyframesDir, nil
}

// generateSprite 生成进度条预览雪碧图和 WebVTT 索引文件
// 每 10 秒一帧，拼成一张大图（10 列），同时生成 WebVTT 文件供前端进度条悬停预览
func (s *PreprocessService) generateSprite(task *model.PreprocessTask) (spritePath string, vttPath string, err error) {
	spriteDir := filepath.Join(task.OutputDir, "sprite")
	os.MkdirAll(spriteDir, 0755)

	spritePath = filepath.Join(spriteDir, "sprite.jpg")
	vttPath = filepath.Join(spriteDir, "sprite.vtt")

	duration := task.SourceDuration
	if duration <= 0 {
		duration = 600
	}

	// 每 10 秒一帧，最多 100 帧（避免雪碧图过大）
	interval := 10.0
	if duration > 1000 {
		interval = math.Ceil(duration / 100)
	}
	frameCount := int(math.Ceil(duration / interval))
	if frameCount < 1 {
		frameCount = 1
	}
	if frameCount > 100 {
		frameCount = 100
	}

	// 每帧缩略图尺寸：160x90（16:9）
	const thumbW, thumbH = 160, 90
	// 每行 10 帧
	const cols = 10
	rows := int(math.Ceil(float64(frameCount) / float64(cols)))

	// 使用 FFmpeg tile 滤镜一次性生成雪碧图
	// -vf "fps=1/10,scale=160:90,tile=10xN" 高效生成
	tileFilter := fmt.Sprintf("fps=1/%.0f,scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,tile=%dx%d",
		interval, thumbW, thumbH, thumbW, thumbH, cols, rows)

	spriteArgs := []string{
		"-y",
		"-skip_frame", "nokey",
	}
	if httpArgs := BuildFFmpegInputArgs(task.InputPath); len(httpArgs) > 0 {
		spriteArgs = append(spriteArgs, httpArgs...)
	}
	spriteArgs = append(spriteArgs,
		"-i", task.InputPath,
		"-vf", tileFilter,
		"-frames:v", "1",
		"-q:v", "5",
		spritePath,
	)

	cmd := exec.Command(s.cfg.App.FFmpegPath, spriteArgs...)
	if runErr := cmd.Run(); runErr != nil {
		return "", "", fmt.Errorf("雪碧图生成失败: %w", runErr)
	}

	// 生成 WebVTT 文件
	vttContent := "WEBVTT\n\n"
	for i := 0; i < frameCount; i++ {
		startSec := float64(i) * interval
		endSec := startSec + interval
		if endSec > duration {
			endSec = duration
		}

		col := i % cols
		row := i / cols
		xOffset := col * thumbW
		yOffset := row * thumbH

		// WebVTT 时间格式：HH:MM:SS.mmm
		vttContent += fmt.Sprintf("%s --> %s\n",
			formatVTTTime(startSec),
			formatVTTTime(endSec),
		)
		// 使用相对路径，前端通过 /api/preprocess/media/:id/sprite 获取
		vttContent += fmt.Sprintf("sprite.jpg#xywh=%d,%d,%d,%d\n\n",
			xOffset, yOffset, thumbW, thumbH)
	}

	if writeErr := os.WriteFile(vttPath, []byte(vttContent), 0644); writeErr != nil {
		return "", "", fmt.Errorf("WebVTT 写入失败: %w", writeErr)
	}

	return spritePath, vttPath, nil
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

	// V2.1: 尝试硬件加速转码，失败时回退到软件转码；
	// 如果 InputPath 是 WebDAV HTTP URL 且全部尝试都失败，再降级到"先下载到本地再转码"
	return s.transcodeVariantWithWebDAVFallback(task, variant, hlsArgs, audioArgs, m3u8Path, segmentPath, onProgress)
}

// transcodeVariantWithWebDAVFallback 包装 transcodeWithFallback，在 WebDAV 源失败时降级为本地下载再转码。
//
// 流程：
//  1. 第一轮：使用原 InputPath（HTTP URL 直读）执行完整回退链
//  2. 若失败且 InputPath 是 HTTP 源 → 完整下载到本地缓存
//  3. 第二轮：用本地路径重跑一次 transcodeWithFallback（此时 BuildFFmpegInputArgs 返回空，走本地文件流）
//  4. 下载文件在最后通过 defer 清理
func (s *PreprocessService) transcodeVariantWithWebDAVFallback(
	task *model.PreprocessTask,
	variant ABRProfile,
	hlsArgs, audioArgs []string,
	m3u8Path, segmentPath string,
	onProgress func(progress float64, speed string),
) error {
	// 第一轮：HTTP 直读尝试
	err := s.transcodeWithFallback(task, variant, hlsArgs, audioArgs, m3u8Path, segmentPath, onProgress)
	if err == nil {
		return nil
	}

	// 用户取消，不降级
	if _, cancelled := s.cancelJobs.Load(task.ID); cancelled {
		return err
	}

	// 判断是否具备降级条件
	if !shouldFallbackToLocalDownload(s.cfg, task.InputPath) {
		return err
	}

	s.logger.Warnf("WebDAV 源转码失败，尝试降级为本地下载: task=%s, err=%v", task.MediaTitle, err)

	// 下载到本地
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 同时监听取消信号
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
				if _, cancelled := s.cancelJobs.Load(task.ID); cancelled {
					cancel()
					return
				}
			}
		}
	}()

	localPath, cleanup, dlErr := downloadWebDAVToLocal(ctx, task.InputPath, s.cfg.Cache.CacheDir, task.MediaID, s.logger)
	if dlErr != nil {
		return fmt.Errorf("WebDAV 降级下载失败: %w (原转码错误: %v)", dlErr, err)
	}
	defer cleanup()

	// 第二轮：用本地路径重跑
	// 先清理前次失败残留
	os.Remove(m3u8Path)
	cleanupDir := filepath.Dir(segmentPath)
	if entries, err := os.ReadDir(cleanupDir); err == nil {
		for _, entry := range entries {
			os.Remove(filepath.Join(cleanupDir, entry.Name()))
		}
	}
	os.MkdirAll(cleanupDir, 0755)

	// 临时替换 InputPath
	originalInput := task.InputPath
	task.InputPath = localPath
	defer func() { task.InputPath = originalInput }()

	s.logger.Infof("WebDAV 降级：使用本地文件重新转码 %s", localPath)
	return s.transcodeWithFallback(task, variant, hlsArgs, audioArgs, m3u8Path, segmentPath, onProgress)
}

// transcodeWithFallback 尝试硬件加速转码，失败时自动回退到软件转码。
//
// 回退策略：
//   - NVENC 模式: nvenc → qsv → vaapi → software
//   - QSV 模式: qsv → vaapi → none(software)
//   - VAAPI 模式: vaapi → qsv → none(software)
//   - 其他/none: 仅 software
//
// 每次尝试失败后会清理已生成的分片文件（保留目录），确保下次尝试从干净状态开始。
// 如果任务被用户取消（通过 cancelJobs），会立即返回而不继续尝试。
//
// 参数 hlsArgs / audioArgs 为兼容旧签名保留，但内部实际参数由 ffmpeg.BuildHLSArgs 统一生成。
func (s *PreprocessService) transcodeWithFallback(
	task *model.PreprocessTask,
	variant ABRProfile,
	_hlsArgs, _audioArgs []string,
	m3u8Path, segmentPath string,
	onProgress func(progress float64, speed string),
) error {
	_ = _hlsArgs
	_ = _audioArgs

	// V2.1: 为 HTTP 源构造超时/重连前置参数（非 HTTP 返回 nil，不影响本地文件）
	httpInputArgs := BuildFFmpegInputArgs(task.InputPath)

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
	// 修复：Windows 上不存在 QSV/VAAPI 设备，跳过无意义的回退尝试，
	// 避免每次回退都要启动 FFmpeg、等待超时、清理分片，浪费大量时间
	var attempts []string
	switch actualHWAccel {
	case ffmpeg.HWAccelNVENC:
		if runtime.GOOS == "windows" {
			attempts = []string{ffmpeg.HWAccelNVENC, ffmpeg.HWAccelNone}
		} else {
			attempts = []string{ffmpeg.HWAccelNVENC, ffmpeg.HWAccelQSV, ffmpeg.HWAccelVAAPI, ffmpeg.HWAccelNone}
		}
	case ffmpeg.HWAccelQSV:
		if runtime.GOOS == "windows" {
			attempts = []string{ffmpeg.HWAccelQSV, ffmpeg.HWAccelNone}
		} else {
			attempts = []string{ffmpeg.HWAccelQSV, ffmpeg.HWAccelVAAPI, ffmpeg.HWAccelNone}
		}
	case ffmpeg.HWAccelVAAPI:
		attempts = []string{ffmpeg.HWAccelVAAPI, ffmpeg.HWAccelQSV, ffmpeg.HWAccelNone}
	default:
		attempts = []string{ffmpeg.HWAccelNone}
	}

	// 提前计算 FFmpeg 线程数（循环内不变，避免重复调用）
	ffmpegThreads := ffmpeg.CalcThreads(s.cfg)

	// 变体目录即 segmentPath 所在目录（stream.m3u8 也在此）
	outputDir := filepath.Dir(segmentPath)

	// 尝试不同的转码方式
	for _, attemptName := range attempts {
		// 确保输出目录存在（回退清理后需要重建）
		os.MkdirAll(outputDir, 0755)

		args := ffmpeg.BuildHLSArgs(ffmpeg.BuildOptions{
			InputPath:  task.InputPath,
			OutputDir:  outputDir,
			ExtraInput: httpInputArgs,
			HWAccel:    attemptName,
			Profile: ffmpeg.Profile{
				Width:        variant.Width,
				Height:       variant.Height,
				VideoBitrate: variant.VideoBitrate,
				AudioBitrate: variant.AudioBitrate,
				MaxBitrate:   variant.MaxBitrate,
				BufSize:      variant.BufSize,
			},
			VAAPIDevice:           s.cfg.App.VAAPIDevice,
			X264Preset:            "ultrafast",
			QSVPreset:             "medium",
			Threads:               ffmpegThreads,
			QSVAttachOutputFormat: true, // 预处理走全 GPU 管线
			HLSTime:               4,
			GOPSize:               48,
		})

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
			s.logger.Warnf("硬件加速 %s 启动失败: %v", attemptName, err)
			continue // 尝试下一种方式
		}

		// 存储进程引用用于取消
		s.runningJobs.Store(task.ID, cmd)

		// 启动进度解析协程，同时收集 stderr 最后几行用于错误诊断
		type stderrResult struct {
			lastLines []string
		}
		resultCh := make(chan stderrResult, 1)
		progressDone := make(chan struct{})
		go func() {
			defer close(progressDone)
			defer func() {
				if r := recover(); r != nil {
					s.logger.Errorf("进度解析 panic: %v", r)
				}
			}()
			lastLines := s.parseFFmpegProgressWithStderr(stderrPipe, task.SourceDuration, onProgress)
			resultCh <- stderrResult{lastLines: lastLines}
		}()

		// 等待命令完成
		err = cmd.Wait()
		s.runningJobs.Delete(task.ID)
		<-progressDone // 等待进度解析完成

		// 收集 stderr 输出
		var stderrLines []string
		select {
		case res := <-resultCh:
			stderrLines = res.lastLines
		default:
		}

		if err != nil {
			// 检查是否是被取消
			if _, cancelled := s.cancelJobs.Load(task.ID); cancelled {
				return fmt.Errorf("任务已取消")
			}

			// 使用 Warn 级别记录回退信息，确保生产环境可见
			if len(stderrLines) > 0 {
				s.logger.Warnf("硬件加速 %s 转码失败: %v\nFFmpeg 输出:\n%s",
					attemptName, err, strings.Join(stderrLines, "\n"))
			} else {
				s.logger.Warnf("硬件加速 %s 转码失败: %v", attemptName, err)
			}

			// 如果不是最后一次尝试，继续尝试下一种方式
			if attemptName != attempts[len(attempts)-1] {
				// 只清理已生成的文件，保留目录结构（避免后续尝试因目录不存在而失败）
				os.Remove(m3u8Path)
				cleanupDir := filepath.Dir(segmentPath)
				if entries, err := os.ReadDir(cleanupDir); err == nil {
					for _, entry := range entries {
						os.Remove(filepath.Join(cleanupDir, entry.Name()))
					}
				}
				s.logger.Warnf("回退: %s 失败，尝试下一种方式", attemptName)
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

// parseFFmpegProgressWithStderr 解析 FFmpeg stderr 进度输出，同时收集最后 N 行 stderr 用于错误诊断。
// 底层解析委托给 ffmpeg.ParseProgress，这里仅负责转接回调。
// 返回值：stderr 最后 10 行输出（用于转码失败时的错误诊断）
func (s *PreprocessService) parseFFmpegProgressWithStderr(stderr io.ReadCloser, totalDuration float64, onProgress func(float64, string)) []string {
	return ffmpeg.ParseProgress(stderr, totalDuration, ffmpeg.ProgressOptions{
		MinDeltaPct:        0.5,
		CollectStderrLines: 10,
	}, func(ev ffmpeg.ProgressEvent) {
		if onProgress != nil {
			onProgress(ev.Progress, ev.Speed)
		}
	})
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
//
// 预处理策略：仅生成源分辨率最近的单档（避免伪高清），不再生成多档 ABR 阶梯。
// 理由：
//   - 本平台主力场景为家庭/私人 NAS 内网播放，弱网适配收益低
//   - 多档预处理会显著放大磁盘占用和转码耗时（N 倍代价）
//   - 如需弱网自适应，可在播放时走实时转码路径（GetMasterPlaylistFiltered）
func (s *PreprocessService) determineVariants(sourceHeight int) []ABRProfile {
	// 找出"源档"：≤源高度里最高的那一档（避免伪高清）
	//   例：源 1080p → 源档=1080p；源 1440p → 源档=2K；源 720p → 源档=720p
	var source *ABRProfile
	for i := range abrProfiles {
		p := abrProfiles[i]
		if p.Height <= sourceHeight {
			if source == nil || p.Height > source.Height {
				source = &p
			}
		}
	}
	if source == nil {
		// 源比最低档还低（< 360p 的稀有情况），退回最低档
		fallback := abrProfiles[0]
		s.logger.Infof("ABR 变体决策: 源=%dp（过低），使用最低档=%s", sourceHeight, fallback.Name)
		return []ABRProfile{fallback}
	}

	s.logger.Infof("ABR 变体决策: 源=%dp, 单档模式, 生成档位=%s", sourceHeight, source.Name)
	return []ABRProfile{*source}
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
		"hw_accel":       s.hwAccel,
	}

	// 添加 GPU 实时指标
	if s.gpuMonitor != nil {
		gpuStatus := s.gpuMonitor.GetSafetyStatus()
		result["gpu_status"] = gpuStatus
	}

	return result
}
