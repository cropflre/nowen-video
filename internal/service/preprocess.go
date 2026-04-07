package service

import (
	"encoding/json"
	"fmt"
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
	maxWorkers  int32       // 最大并发工作协程数（上限）
	curWorkers  int32       // 当前动态调整后的并发数
	jobQueue    chan string // 任务 ID 队列
	pausedJobs  sync.Map    // 暂停的任务 ID 集合
	cancelJobs  sync.Map    // 取消的任务 ID 集合
	runningJobs sync.Map    // 正在运行的任务 ID -> *exec.Cmd
	mu          sync.RWMutex

	// 动态负载调整
	lastLoadCheck time.Time
	hwAccel       string // 硬件加速模式

	// 进度广播节流
	lastBroadcast sync.Map // taskID -> time.Time（上次广播时间）
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
	maxWorkers := int32(cfg.App.MaxTranscodeJobs)
	if maxWorkers <= 0 {
		maxWorkers = 2
	}

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

	// 恢复未完成的任务
	go s.recoverPendingTasks()

	// 启动动态负载调整协程
	go s.dynamicLoadAdjuster()

	return s
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
		MediaTitle: media.Title,
		MaxRetry:   3,
	}

	if err := s.repo.Create(task); err != nil {
		return nil, fmt.Errorf("创建预处理任务失败: %w", err)
	}

	// 入队
	select {
	case s.jobQueue <- task.ID:
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
	return s.repo.ListAll(page, pageSize, status)
}

// GetStatistics 获取预处理统计
func (s *PreprocessService) GetStatistics() map[string]interface{} {
	counts, _ := s.repo.CountByStatus()
	running, _ := s.repo.ListRunning()

	return map[string]interface{}{
		"status_counts":  counts,
		"running_count":  len(running),
		"max_workers":    s.maxWorkers,
		"active_workers": atomic.LoadInt32(&s.workerCount),
		"queue_size":     len(s.jobQueue),
		"hw_accel":       s.hwAccel,
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

		// 动态并发控制：如果当前 worker ID 超过动态限制，等待
		for int32(id) >= atomic.LoadInt32(&s.curWorkers) {
			time.Sleep(5 * time.Second)
			// 再次检查取消状态
			if _, cancelled := s.cancelJobs.Load(taskID); cancelled {
				break
			}
		}

		atomic.AddInt32(&s.workerCount, 1)
		s.processTask(taskID)
		atomic.AddInt32(&s.workerCount, -1)

		// 清理广播节流缓存
		s.lastBroadcast.Delete(taskID)
	}
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

	// ========== Phase 2: 提取封面缩略图 ==========
	if s.isCancelled(taskID) {
		return
	}
	s.updatePhase(task, "thumbnail", 10, "正在提取视频封面...")

	thumbnailPath, err := s.extractThumbnail(task)
	if err != nil {
		s.logger.Warnf("封面提取失败（非致命）: %v", err)
	} else {
		task.ThumbnailPath = thumbnailPath
		s.repo.Update(task)
	}

	// ========== Phase 3: 生成关键帧预览 ==========
	if s.isCancelled(taskID) {
		return
	}
	s.updatePhase(task, "keyframes", 15, "正在生成关键帧预览...")

	keyframesDir, err := s.extractKeyframes(task)
	if err != nil {
		s.logger.Warnf("关键帧提取失败（非致命）: %v", err)
	} else {
		task.KeyframesDir = keyframesDir
		s.repo.Update(task)
	}

	// ========== Phase 4: 多码率 HLS 转码 ==========
	if s.isCancelled(taskID) {
		return
	}

	// 确定需要生成的变体（不超过源分辨率）
	variants := s.determineVariants(probeInfo.height)
	totalVariants := len(variants)
	completedVariants := []string{}

	for i, variant := range variants {
		if s.isCancelled(taskID) {
			return
		}
		if s.isPaused(taskID) {
			task.Status = "paused"
			task.Message = fmt.Sprintf("已暂停（已完成 %d/%d 变体）", i, totalVariants)
			s.repo.Update(task)
			s.broadcastEvent(EventPreprocessPaused, task)
			return
		}

		// 计算总体进度：20% ~ 95%（转码占 75%）
		baseProgress := 20.0 + float64(i)/float64(totalVariants)*75.0
		phaseName := fmt.Sprintf("transcode_%s", variant.Name)
		s.updatePhase(task, phaseName, baseProgress,
			fmt.Sprintf("正在转码 %s (%d/%d)...", variant.Name, i+1, totalVariants))

		err := s.transcodeVariant(task, variant, func(progress float64, speed string) {
			// 变体内部进度映射到总体进度
			variantProgress := baseProgress + (progress/100.0)*(75.0/float64(totalVariants))
			s.updatePhase(task, phaseName, variantProgress,
				fmt.Sprintf("转码 %s: %.1f%% (速度: %s)", variant.Name, progress, speed))
		})

		if err != nil {
			s.logger.Errorf("转码变体 %s 失败: %v", variant.Name, err)
			// 单个变体失败不中断整体流程
			continue
		}

		completedVariants = append(completedVariants, variant.Name)
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
func (s *PreprocessService) extractThumbnail(task *model.PreprocessTask) (string, error) {
	thumbnailPath := filepath.Join(task.OutputDir, "thumbnail.jpg")

	// 取视频 10% 位置的帧作为封面
	seekPos := "10"
	if task.SourceDuration > 0 {
		seekPos = fmt.Sprintf("%.0f", task.SourceDuration*0.1)
	}

	cmd := exec.Command(s.cfg.App.FFmpegPath,
		"-y",
		"-ss", seekPos,
		"-i", task.InputPath,
		"-vframes", "1",
		"-q:v", "2",
		"-vf", "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease",
		thumbnailPath,
	)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("封面提取失败: %w", err)
	}

	return thumbnailPath, nil
}

// extractKeyframes 提取关键帧预览（每 30 秒一帧，最多 20 帧）
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

	cmd := exec.Command(s.cfg.App.FFmpegPath,
		"-y",
		"-i", task.InputPath,
		"-vf", fmt.Sprintf("fps=1/%.0f,scale=320:-1", interval),
		"-q:v", "5",
		"-frames:v", "20",
		filepath.Join(keyframesDir, "kf_%03d.jpg"),
	)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("关键帧提取失败: %w", err)
	}

	return keyframesDir, nil
}

// transcodeVariant 转码单个变体
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

	// 构建 FFmpeg 参数
	baseArgs := []string{"-y", "-i", task.InputPath}

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

	var videoArgs []string

	switch s.hwAccel {
	case "nvenc":
		baseArgs = append([]string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}, baseArgs...)
		videoArgs = []string{
			"-c:v", "h264_nvenc",
			"-preset", "p4",
			"-b:v", variant.VideoBitrate,
			"-maxrate", variant.MaxBitrate,
			"-bufsize", variant.BufSize,
			"-vf", fmt.Sprintf("scale_cuda=%d:%d", variant.Width, variant.Height),
			"-g", "48",
			"-keyint_min", "48",
			"-sc_threshold", "0",
		}
	case "qsv":
		baseArgs = append([]string{"-hwaccel", "qsv", "-hwaccel_output_format", "qsv"}, baseArgs...)
		videoArgs = []string{
			"-c:v", "h264_qsv",
			"-preset", "medium",
			"-b:v", variant.VideoBitrate,
			"-maxrate", variant.MaxBitrate,
			"-bufsize", variant.BufSize,
			"-vf", fmt.Sprintf("scale_qsv=%d:%d", variant.Width, variant.Height),
			"-g", "48",
			"-keyint_min", "48",
		}
	case "vaapi":
		baseArgs = append([]string{
			"-hwaccel", "vaapi",
			"-hwaccel_output_format", "vaapi",
			"-vaapi_device", s.cfg.App.VAAPIDevice,
		}, baseArgs...)
		videoArgs = []string{
			"-c:v", "h264_vaapi",
			"-b:v", variant.VideoBitrate,
			"-maxrate", variant.MaxBitrate,
			"-bufsize", variant.BufSize,
			"-vf", fmt.Sprintf("scale_vaapi=w=%d:h=%d", variant.Width, variant.Height),
			"-g", "48",
			"-keyint_min", "48",
		}
	default:
		videoArgs = []string{
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
	}

	args := append(baseArgs, videoArgs...)
	args = append(args, audioArgs...)
	args = append(args, hlsArgs...)

	cmd := exec.Command(s.cfg.App.FFmpegPath, args...)

	// 解析进度
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建 stderr 管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("FFmpeg 启动失败: %w", err)
	}

	// 存储进程引用用于取消
	s.runningJobs.Store(task.ID, cmd)
	defer s.runningJobs.Delete(task.ID)

	// 异步解析进度
	timeRegex := regexp.MustCompile(`time=(\d{2}):(\d{2}):(\d{2})\.(\d{2})`)
	speedRegex := regexp.MustCompile(`speed=\s*([\d.]+)x`)

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				line := string(buf[:n])
				timeMatches := timeRegex.FindStringSubmatch(line)
				if len(timeMatches) >= 5 && task.SourceDuration > 0 {
					hours, _ := strconv.ParseFloat(timeMatches[1], 64)
					minutes, _ := strconv.ParseFloat(timeMatches[2], 64)
					seconds, _ := strconv.ParseFloat(timeMatches[3], 64)
					centis, _ := strconv.ParseFloat(timeMatches[4], 64)
					currentTime := hours*3600 + minutes*60 + seconds + centis/100
					progress := (currentTime / task.SourceDuration) * 100
					if progress > 100 {
						progress = 100
					}

					speed := ""
					speedMatches := speedRegex.FindStringSubmatch(line)
					if len(speedMatches) >= 2 {
						speed = speedMatches[1] + "x"
					}

					onProgress(progress, speed)
				}
			}
			if err != nil {
				break
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		// 检查是否是被取消
		if _, cancelled := s.cancelJobs.Load(task.ID); cancelled {
			return fmt.Errorf("任务已取消")
		}
		return fmt.Errorf("FFmpeg 转码失败: %w", err)
	}

	return nil
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

func (s *PreprocessService) updatePhase(task *model.PreprocessTask, phase string, progress float64, message string) {
	task.Phase = phase
	task.Progress = progress
	task.Message = message
	s.repo.Update(task)

	// 节流广播：每 2 秒最多广播一次进度（避免 WebSocket 消息过于频繁）
	now := time.Now()
	if lastTime, ok := s.lastBroadcast.Load(task.ID); ok {
		if now.Sub(lastTime.(time.Time)) < 2*time.Second {
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
		tasks, err := s.repo.FindNeedRetry(5)
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
	time.Sleep(3 * time.Second) // 等待服务完全启动

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

	return map[string]interface{}{
		"cpu_count":      runtime.NumCPU(),
		"cpu_percent":    cpuPercent,
		"goroutines":     runtime.NumGoroutine(),
		"mem_alloc_mb":   float64(memStats.Alloc) / 1024 / 1024,
		"mem_sys_mb":     float64(memStats.Sys) / 1024 / 1024,
		"active_workers": atomic.LoadInt32(&s.workerCount),
		"max_workers":    s.maxWorkers,
		"cur_workers":    atomic.LoadInt32(&s.curWorkers),
		"queue_size":     len(s.jobQueue),
	}
}

// dynamicLoadAdjuster 动态负载调整协程
// 根据 CPU 使用率动态调整并发 worker 数量
func (s *PreprocessService) dynamicLoadAdjuster() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		percents, err := cpu.Percent(time.Second, false)
		if err != nil || len(percents) == 0 {
			continue
		}

		cpuUsage := percents[0]
		current := atomic.LoadInt32(&s.curWorkers)
		max := s.maxWorkers

		var newWorkers int32
		switch {
		case cpuUsage > 90:
			// CPU 过载，减少到 1 个 worker
			newWorkers = 1
		case cpuUsage > 75:
			// CPU 较高，减半
			newWorkers = max / 2
			if newWorkers < 1 {
				newWorkers = 1
			}
		case cpuUsage > 50:
			// CPU 中等，使用 75% 的 worker
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
			s.logger.Infof("动态调整并发: CPU %.1f%%, workers %d -> %d", cpuUsage, current, newWorkers)
		}
	}
}

// 确保 runtime 包被使用
var _ = runtime.NumCPU
