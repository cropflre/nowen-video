package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	"go.uber.org/zap"
)

// ==================== 节流（Throttling）配置 ====================
//
// 参考 Emby：转码 FFmpeg 领先播放进度一定秒数后挂起，播放进度逼近缓冲尾部时恢复。
// 这种策略的核心目的是：
//  1) 节省 GPU/CPU：点播时不需要提前把整部片都编完；
//  2) 降低带宽/内存：避免生成大量未播放的 .ts 缓存；
//  3) 响应用户 seek：被挂起的进程可以快速 Kill，由新进程接力。
const (
	// 缓冲领先超过此秒数则挂起 ffmpeg
	throttleAheadHighWatermark = 60.0
	// 缓冲领先低于此秒数则恢复 ffmpeg
	throttleAheadLowWatermark = 15.0
	// 节流轮询间隔
	throttleTickInterval = 2 * time.Second
	// 首个分片目标 2s（HLS），实现秒开。低于 2s 会显著放大码流。
	hlsTargetSegmentSeconds = 2
)

// 质量预设（包含 4K 和 2K）
var qualityPresets = map[string]QualityConfig{
	"360p":  {Width: 640, Height: 360, VideoBitrate: "800k", AudioBitrate: "96k"},
	"480p":  {Width: 854, Height: 480, VideoBitrate: "1500k", AudioBitrate: "128k"},
	"720p":  {Width: 1280, Height: 720, VideoBitrate: "3000k", AudioBitrate: "128k"},
	"1080p": {Width: 1920, Height: 1080, VideoBitrate: "6000k", AudioBitrate: "192k"},
	"2K":    {Width: 2560, Height: 1440, VideoBitrate: "12000k", AudioBitrate: "192k"},
	"4K":    {Width: 3840, Height: 2160, VideoBitrate: "25000k", AudioBitrate: "256k"},
}

// QualityConfig 质量配置
type QualityConfig struct {
	Width        int
	Height       int
	VideoBitrate string
	AudioBitrate string
}

// TranscodeService 转码服务
type TranscodeService struct {
	repo      *repository.TranscodeRepo
	cfg       *config.Config
	logger    *zap.SugaredLogger
	jobs      chan *TranscodeJob
	mu        sync.RWMutex
	running   map[string]*TranscodeJob
	hwAccel   string    // 检测到的硬件加速方式
	hwAccelMu sync.Once // 硬件加速检测只执行一次
	wsHub     *WSHub    // WebSocket事件广播
}

// SetWSHub 设置WebSocket Hub（延迟注入）
func (s *TranscodeService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// TranscodeJob 转码任务
type TranscodeJob struct {
	Task    *model.TranscodeTask
	Media   *model.Media
	Quality string
	Cancel  chan struct{}
	cmd     *exec.Cmd // 当前正在执行的 FFmpeg 进程，用于取消

	// ==================== 节流相关 ====================
	// 播放器上报的当前播放位置（秒），原子更新。
	playbackPos atomic.Uint64 // 以 *100 的定点数存储，避免 float64 的原子操作
	// ffmpeg 当前已转码到的位置（秒），由 parseFFmpegProgress 实时更新。
	transcodedPos atomic.Uint64
	// 节流状态：0=运行，1=已挂起
	suspended atomic.Int32
	// 起始偏移（-ss 参数），秒。用户 seek 后会重建 job 并把该值置非零。
	startOffset float64
	// 节流 ticker 退出信号
	throttleDone chan struct{}
}

// SetPlaybackPosition 由 handler 层在收到播放进度上报时调用。
// 单位秒，支持 float 精度；节流协程会周期性读取。
func (j *TranscodeJob) SetPlaybackPosition(sec float64) {
	if sec < 0 {
		sec = 0
	}
	j.playbackPos.Store(uint64(sec * 100))
}

func (j *TranscodeJob) getPlaybackPosition() float64 {
	return float64(j.playbackPos.Load()) / 100
}

func (j *TranscodeJob) getTranscodedPosition() float64 {
	return float64(j.transcodedPos.Load()) / 100
}

func NewTranscodeService(repo *repository.TranscodeRepo, cfg *config.Config, logger *zap.SugaredLogger) *TranscodeService {
	ts := &TranscodeService{
		repo:    repo,
		cfg:     cfg,
		logger:  logger,
		jobs:    make(chan *TranscodeJob, 100),
		running: make(map[string]*TranscodeJob),
	}

	// 检测硬件加速能力（只检测一次，缓存结果）
	ts.hwAccelMu.Do(func() {
		ts.hwAccel = ts.detectHWAccel()
		logger.Infof("硬件加速模式: %s", ts.hwAccel)
	})

	// 根据配置启动转码工作协程
	transcodeWorkers := cfg.App.MaxTranscodeJobs
	if transcodeWorkers <= 0 {
		transcodeWorkers = 1
	}
	for i := 0; i < transcodeWorkers; i++ {
		go ts.worker(i)
	}

	// 恢复重启前未完成的任务：把 DB 中仍为 pending/running 的任务置为 failed
	// 避免前端看到僵尸"运行中"任务（因为 goroutine 已随进程退出而消亡）
	go ts.recoverPendingTasks()

	return ts
}

// recoverPendingTasks 清理服务重启前的僵尸任务
// TranscodeService 的 running map 是纯内存态，重启后原来的 goroutine 全部消失，
// 但 DB 中的任务状态仍是 running/pending，会导致：
//  1. 前端显示永远"运行中"；
//  2. StartTranscode 检查缓存时因 FindByMediaAndQuality 只认 "done" 状态，重复提交任务；
//  3. 但 OutputDir 已有残留文件，可能造成脏数据。
//
// 策略：简单粗暴地把所有 pending/running 标为 failed，让用户重新触发即可。
func (s *TranscodeService) recoverPendingTasks() {
	// 稍等片刻确保服务完全启动
	// 此处不阻塞主流程，放 goroutine 中异步执行
	running, err := s.repo.ListRunning()
	if err != nil {
		s.logger.Warnf("恢复转码任务状态失败: %v", err)
		return
	}
	if len(running) == 0 {
		return
	}
	for i := range running {
		running[i].Status = "failed"
		running[i].Error = "服务重启导致任务中断"
		if err := s.repo.Update(&running[i]); err != nil {
			s.logger.Warnf("重置僵尸任务状态失败: %s, %v", running[i].ID, err)
		}
	}
	s.logger.Infof("已重置 %d 个重启前未完成的转码任务为 failed", len(running))
}

// detectHWAccel 检测可用的硬件加速方式（委托给 ffmpeg 子包的公共实现）
func (s *TranscodeService) detectHWAccel() string {
	return ffmpeg.DetectHWAccel(s.cfg, s.logger)
}

// buildFFmpegArgs 根据硬件加速模式构建FFmpeg参数
// media 参数用于 HDR 检测：HEVC/VP9/AV1 源视频会自动追加 tonemap 滤镜，避免 SDR 设备播放偏灰
// startOffset>0 时启用 -ss input seeking，实现 seek 后快速续转（Emby 秒开关键）
//
// 实现：底层参数组装委托给 ffmpeg.BuildHLSArgs，这里只负责把 transcode 场景
// 特有的配置（实时秒开 / HDR tonemap / CRF 质量模式 / HLS event playlist）
// 映射为 BuildOptions 字段。
func (s *TranscodeService) buildFFmpegArgs(media *model.Media, inputPath, outputDir, quality string, startOffset float64) []string {
	qc, ok := qualityPresets[quality]
	if !ok {
		qc = qualityPresets["720p"]
	}

	// 只有软件编码分支需要 HDR tonemap 滤镜；硬件加速分支保持默认 scale_xxx。
	var videoFilter string
	if s.hwAccel == ffmpeg.HWAccelNone || s.hwAccel == "" {
		videoFilter = s.buildFFmpegHDRTonemapFilter(media, qc.Width, qc.Height)
	}

	// seek 场景下按分片粒度算 start_number，避免客户端误复用旧编号的分片
	startNumber := 0
	if startOffset > 0 {
		startNumber = int(startOffset / float64(hlsTargetSegmentSeconds))
	}

	// QSV 场景在 transcode 中使用 CQP 质量模式（不锁码率），与 preprocess 码率模式不同
	qsvGlobalQuality := 0
	if s.hwAccel == ffmpeg.HWAccelQSV {
		qsvGlobalQuality = 23
	}

	return ffmpeg.BuildHLSArgs(ffmpeg.BuildOptions{
		InputPath:       inputPath,
		OutputDir:       outputDir,
		HWAccel:         s.hwAccel,
		Profile:         ffmpeg.Profile{Width: qc.Width, Height: qc.Height, VideoBitrate: qc.VideoBitrate, AudioBitrate: qc.AudioBitrate},
		VAAPIDevice:     s.cfg.App.VAAPIDevice,
		X264Preset:      s.cfg.App.TranscodePreset,
		QSVPreset:       s.cfg.App.TranscodePreset,
		Threads:         ffmpeg.CalcThreads(s.cfg),
		UseCRF:          true,
		CRF:             23,
		SoftwareTune:    "zerolatency",
		NvencTune:       "ll",
		QSVAttachOutputFormat: false, // 允许 QSV 解码失败时回退软件解码
		QSVGlobalQuality: qsvGlobalQuality,
		VideoFilter:     videoFilter,
		HLSTime:         hlsTargetSegmentSeconds,
		HLSFlags:        "independent_segments+append_list+program_date_time",
		HLSPlaylistType: "event",
		StartNumber:     startNumber,
		ForceKeyFrames:  true,
		StartOffsetSec:  startOffset,
		GOPSize:         hlsTargetSegmentSeconds * 25,
		SkipVAAPIRateLimits: true, // 保持 transcode 历史 VAAPI 行为一致
	})
}

// GetOutputDir 获取转码输出目录
func (s *TranscodeService) GetOutputDir(mediaID, quality string) string {
	return filepath.Join(s.cfg.Cache.CacheDir, "transcode", mediaID, quality)
}

// StartTranscode 发起转码任务
func (s *TranscodeService) StartTranscode(media *model.Media, quality string) (*model.TranscodeTask, error) {
	return s.startTranscodeInternal(media, quality, 0)
}

// StartTranscodeWithStart 从指定秒数开始转码（用于播放器 seek 场景的快速预热）
// 与 StartTranscode 的区别：不复用缓存（因为 startOffset 不同，m3u8 内容不同）。
// 调用方应确保 outputDir 目录独立，避免和 0 起点缓存冲突。
func (s *TranscodeService) StartTranscodeWithStart(media *model.Media, quality string, startOffset float64) (*model.TranscodeTask, error) {
	return s.startTranscodeInternal(media, quality, startOffset)
}

func (s *TranscodeService) startTranscodeInternal(media *model.Media, quality string, startOffset float64) (*model.TranscodeTask, error) {
	// 只有 startOffset=0 时才复用已完成缓存；seek 场景直接新建
	if startOffset == 0 {
		if task, err := s.repo.FindByMediaAndQuality(media.ID, quality); err == nil {
			outputDir := s.GetOutputDir(media.ID, quality)
			m3u8Path := filepath.Join(outputDir, "stream.m3u8")
			if _, err := os.Stat(m3u8Path); err == nil {
				return task, nil // 已有缓存
			}
		}
	}

	// 检查是否已有该 media+quality 的 job 正在运行；如有则直接返回，避免重复启动
	// 这对防止首片就绪轮询期间重复触发转码很关键
	if startOffset == 0 {
		s.mu.RLock()
		for _, j := range s.running {
			if j.Media.ID == media.ID && j.Quality == quality && j.startOffset == 0 {
				s.mu.RUnlock()
				return j.Task, nil
			}
		}
		s.mu.RUnlock()
	}

	outputDir := s.GetOutputDir(media.ID, quality)
	os.MkdirAll(outputDir, 0755)

	task := &model.TranscodeTask{
		MediaID:   media.ID,
		Quality:   quality,
		Status:    "pending",
		OutputDir: outputDir,
	}

	if err := s.repo.Create(task); err != nil {
		return nil, err
	}

	job := &TranscodeJob{
		Task:         task,
		Media:        media,
		Quality:      quality,
		Cancel:       make(chan struct{}),
		startOffset:  startOffset,
		throttleDone: make(chan struct{}),
	}

	s.mu.Lock()
	s.running[task.ID] = job
	s.mu.Unlock()

	s.jobs <- job

	return task, nil
}

// WaitForFirstSegment 等待指定 media+quality 的首片生成。
// 相对于之前 GetSegmentPlaylist 里的 500ms*120 轮询，这里采用短间隔轮询
// 并且只等待到 ".ts" 出现即返回，配合 hls_time=2 首片通常 1~3 秒内就绪。
// ctx 过期时返回 ctx 错误。
func (s *TranscodeService) WaitForFirstSegment(ctx context.Context, mediaID, quality string) error {
	outputDir := s.GetOutputDir(mediaID, quality)
	m3u8Path := filepath.Join(outputDir, "stream.m3u8")

	// 快速 100ms 间隔轮询，降低首包延迟
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if _, err := os.Stat(m3u8Path); err == nil {
			content, err := os.ReadFile(m3u8Path)
			if err == nil && strings.Contains(string(content), ".ts") {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// worker 转码工作协程
func (s *TranscodeService) worker(id int) {
	s.logger.Infof("转码工作协程 #%d 启动", id)
	for job := range s.jobs {
		s.processJob(job)
	}
}

// processJob 处理转码任务
func (s *TranscodeService) processJob(job *TranscodeJob) {
	s.logger.Infof("开始转码: %s (%s)", job.Media.Title, job.Quality)

	job.Task.Status = "running"
	s.repo.Update(job.Task)

	// 广播转码开始事件
	s.broadcastTranscodeEvent(EventTranscodeStarted, &TranscodeProgressData{
		TaskID:  job.Task.ID,
		MediaID: job.Media.ID,
		Title:   job.Media.Title,
		Quality: job.Quality,
		Message: fmt.Sprintf("开始转码: %s (%s)", job.Media.Title, job.Quality),
	})

	args := s.buildFFmpegArgs(job.Media, job.Media.FilePath, job.Task.OutputDir, job.Quality, job.startOffset)
	s.logger.Debugf("FFmpeg命令: %s %s", s.cfg.App.FFmpegPath, strings.Join(args, " "))

	cmd := exec.Command(s.cfg.App.FFmpegPath, args...)
	setLowPriority(cmd) // 极低资源模式：FFmpeg 以最低优先级运行
	job.cmd = cmd       // 保存引用，用于取消

	// 捕获stderr以解析进度
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		s.logger.Errorf("创建stderr管道失败: %v", err)
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		s.logger.Errorf("转码启动失败: %s, 错误: %v", job.Media.Title, err)
		job.Task.Status = "failed"
		job.Task.Error = err.Error()
		s.repo.Update(job.Task)
		s.broadcastTranscodeEvent(EventTranscodeFailed, &TranscodeProgressData{
			TaskID:  job.Task.ID,
			MediaID: job.Media.ID,
			Title:   job.Media.Title,
			Quality: job.Quality,
			Message: fmt.Sprintf("转码启动失败: %v", err),
		})
		return
	}

	// 异步解析FFmpeg进度输出
	if stderrPipe != nil {
		go s.parseFFmpegProgress(stderrPipe, job)
	}

	// 启动节流协程：根据播放位置 vs 转码位置决定挂起/恢复 ffmpeg
	go s.throttleLoop(job)
	// 无论后续走到 cancel / failed / done 哪个分支，都必须停掉节流协程
	defer func() {
		select {
		case <-job.throttleDone: // 已关闭
		default:
			close(job.throttleDone)
		}
	}()

	// 支持取消：监听 Cancel channel
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var waitErr error
	select {
	case <-job.Cancel:
		// 任务被取消，强制终止 FFmpeg 进程
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		s.logger.Infof("转码任务已取消: %s (%s)", job.Media.Title, job.Quality)
		job.Task.Status = "cancelled"
		s.repo.Update(job.Task)
		s.mu.Lock()
		delete(s.running, job.Task.ID)
		s.mu.Unlock()
		s.broadcastTranscodeEvent(EventTranscodeFailed, &TranscodeProgressData{
			TaskID:  job.Task.ID,
			MediaID: job.Media.ID,
			Title:   job.Media.Title,
			Quality: job.Quality,
			Message: fmt.Sprintf("转码已取消: %s (%s)", job.Media.Title, job.Quality),
		})
		return
	case waitErr = <-done:
	}

	if waitErr != nil {
		s.logger.Warnf("转码失败(hwAccel=%s): %s, 错误: %v，尝试回退到软件编码", s.hwAccel, job.Media.Title, waitErr)

		// 硬件加速失败时自动回退到软件编码（仅非 software 模式时回退）
		if s.hwAccel != "none" {
			// 清理已生成的分片文件，保留目录以便重试
			entries, _ := os.ReadDir(job.Task.OutputDir)
			for _, e := range entries {
				os.Remove(filepath.Join(job.Task.OutputDir, e.Name()))
			}

			// 用软件编码重新构建 FFmpeg 参数
			fallbackHwAccel := "none"
			origHwAccel := s.hwAccel
			s.hwAccel = fallbackHwAccel
			fallbackArgs := s.buildFFmpegArgs(job.Media, job.Media.FilePath, job.Task.OutputDir, job.Quality, job.startOffset)
			s.hwAccel = origHwAccel // 恢复原始设置，不影响后续任务

			s.logger.Infof("回退到软件编码: %s (%s)", job.Media.Title, job.Quality)
			s.logger.Debugf("回退FFmpeg命令: %s %s", s.cfg.App.FFmpegPath, strings.Join(fallbackArgs, " "))

			fallbackCmd := exec.Command(s.cfg.App.FFmpegPath, fallbackArgs...)
			setLowPriority(fallbackCmd)
			job.cmd = fallbackCmd

			// 捕获 stderr
			if stderrPipe, err := fallbackCmd.StderrPipe(); err == nil {
				go s.parseFFmpegProgress(stderrPipe, job)
			}

			if err := fallbackCmd.Start(); err != nil {
				s.logger.Errorf("回退软件编码启动失败: %s, 错误: %v", job.Media.Title, err)
				job.Task.Status = "failed"
				job.Task.Error = fmt.Sprintf("硬件加速失败(%v)，软件编码启动也失败(%v)", waitErr, err)
				s.repo.Update(job.Task)
				s.broadcastTranscodeEvent(EventTranscodeFailed, &TranscodeProgressData{
					TaskID:  job.Task.ID,
					MediaID: job.Media.ID,
					Title:   job.Media.Title,
					Quality: job.Quality,
					Message: fmt.Sprintf("转码失败（回退也失败）: %v", err),
				})
				return
			}

			// 等待回退转码完成
			fallbackDone := make(chan error, 1)
			go func() {
				fallbackDone <- fallbackCmd.Wait()
			}()

			select {
			case <-job.Cancel:
				if fallbackCmd.Process != nil {
					fallbackCmd.Process.Kill()
				}
				s.logger.Infof("回退转码任务已取消: %s (%s)", job.Media.Title, job.Quality)
				job.Task.Status = "cancelled"
				s.repo.Update(job.Task)
				s.mu.Lock()
				delete(s.running, job.Task.ID)
				s.mu.Unlock()
				return
			case fallbackErr := <-fallbackDone:
				if fallbackErr != nil {
					s.logger.Errorf("回退软件编码也失败: %s, 错误: %v", job.Media.Title, fallbackErr)
					job.Task.Status = "failed"
					job.Task.Error = fmt.Sprintf("硬件加速失败(%v)，软件编码也失败(%v)", waitErr, fallbackErr)
					s.repo.Update(job.Task)
					s.broadcastTranscodeEvent(EventTranscodeFailed, &TranscodeProgressData{
						TaskID:  job.Task.ID,
						MediaID: job.Media.ID,
						Title:   job.Media.Title,
						Quality: job.Quality,
						Message: fmt.Sprintf("转码失败（回退也失败）: %v", fallbackErr),
					})
					return
				}
				// 回退成功
				goto success
			}
		}

		job.Task.Status = "failed"
		job.Task.Error = waitErr.Error()
		s.repo.Update(job.Task)
		s.broadcastTranscodeEvent(EventTranscodeFailed, &TranscodeProgressData{
			TaskID:  job.Task.ID,
			MediaID: job.Media.ID,
			Title:   job.Media.Title,
			Quality: job.Quality,
			Message: fmt.Sprintf("转码失败: %v", waitErr),
		})
		return
	}

success:

	job.Task.Status = "done"
	job.Task.Progress = 100
	s.repo.Update(job.Task)

	s.mu.Lock()
	delete(s.running, job.Task.ID)
	s.mu.Unlock()

	s.logger.Infof("转码完成: %s (%s)", job.Media.Title, job.Quality)

	// 广播转码完成事件
	s.broadcastTranscodeEvent(EventTranscodeCompleted, &TranscodeProgressData{
		TaskID:   job.Task.ID,
		MediaID:  job.Media.ID,
		Title:    job.Media.Title,
		Quality:  job.Quality,
		Progress: 100,
		Message:  fmt.Sprintf("转码完成: %s (%s)", job.Media.Title, job.Quality),
	})
}

// parseFFmpegProgress 解析FFmpeg stderr输出中的进度信息
// 底层解析委托给 ffmpeg.ParseProgress，这里只负责把事件转换成 WS/DB 写入。
func (s *TranscodeService) parseFFmpegProgress(stderr io.ReadCloser, job *TranscodeJob) {
	totalDuration := job.Media.Duration // 总时长（秒）
	var lastDBProgress float64

	ffmpeg.ParseProgress(stderr, totalDuration, ffmpeg.ProgressOptions{
		MinDeltaPct: 1.0, // WS 广播：每 1% 更新一次（低延迟，前端体验好）
	}, func(ev ffmpeg.ProgressEvent) {
		// 同步更新节流所需的"已转码位置"（包含 startOffset）
		job.transcodedPos.Store(uint64((ev.CurrentSec + job.startOffset) * 100))

		progress := ev.Progress
		// DB 持久化：每 5% 才写一次（减少 SQLite 写锁竞争，避免 SLOW SQL）
		// WS 广播仍然每 1% 触发，前端进度条不受影响
		job.Task.Progress = progress
		if progress-lastDBProgress >= 5 || progress >= 99.5 {
			lastDBProgress = progress
			s.repo.UpdateProgress(job.Task.ID, progress)
		}

		// 广播进度事件
		s.broadcastTranscodeEvent(EventTranscodeProgress, &TranscodeProgressData{
			TaskID:   job.Task.ID,
			MediaID:  job.Media.ID,
			Title:    job.Media.Title,
			Quality:  job.Quality,
			Progress: progress,
			Speed:    ev.Speed,
			Message:  fmt.Sprintf("转码中: %.1f%% (速度: %s)", progress, ev.Speed),
		})
	})
}

// broadcastTranscodeEvent 广播转码事件
func (s *TranscodeService) broadcastTranscodeEvent(eventType string, data *TranscodeProgressData) {
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(eventType, data)
	}
}

// ==================== Throttling（Emby 风格的转码节流）====================

// throttleLoop 周期性对比 "已转码位置" vs "播放位置"，
// 当领先 > throttleAheadHighWatermark 时挂起 ffmpeg，领先 < throttleAheadLowWatermark 时恢复。
// 节流目标：点播时节省 GPU/CPU，避免提前把整部片编完。
//
// 注意：只有客户端通过 SetPlaybackPosition 上报了播放位置时，节流才会生效；
// 未上报时保持常规全速转码，行为与旧版一致，不破坏向后兼容。
func (s *TranscodeService) throttleLoop(job *TranscodeJob) {
	ticker := time.NewTicker(throttleTickInterval)
	defer ticker.Stop()

	// 未收到任何播放位置上报前，不进行节流（避免在 prebuffer 阶段误暂停）
	// 策略：playbackPos==0 且 transcodedPos<30 时视为"刚开始"，跳过节流判断。
	for {
		select {
		case <-job.throttleDone:
			return
		case <-job.Cancel:
			return
		case <-ticker.C:
		}

		playback := job.getPlaybackPosition()
		transcoded := job.getTranscodedPosition()

		// 无上报或转码刚起步，不节流
		if playback <= 0 {
			continue
		}

		ahead := transcoded - playback
		wasSuspended := job.suspended.Load() == 1

		switch {
		case !wasSuspended && ahead > throttleAheadHighWatermark:
			// 领先太多 → 挂起
			if job.cmd != nil && job.cmd.Process != nil {
				if err := suspendProcess(job.cmd.Process); err == nil {
					job.suspended.Store(1)
					s.logger.Debugf("[throttle] suspend ffmpeg media=%s quality=%s ahead=%.1fs",
						job.Media.ID, job.Quality, ahead)
				} else {
					s.logger.Warnf("[throttle] suspend failed: %v", err)
				}
			}
		case wasSuspended && ahead < throttleAheadLowWatermark:
			// 缓冲不足 → 恢复
			if job.cmd != nil && job.cmd.Process != nil {
				if err := resumeProcess(job.cmd.Process); err == nil {
					job.suspended.Store(0)
					s.logger.Debugf("[throttle] resume ffmpeg media=%s quality=%s ahead=%.1fs",
						job.Media.ID, job.Quality, ahead)
				} else {
					s.logger.Warnf("[throttle] resume failed: %v", err)
				}
			}
		}
	}
}

// SetPlaybackPosition 对外暴露：客户端通过 /api/stream/:id/playback 上报播放进度时调用。
// 会更新该 media 正在运行的所有 quality 的 playback position。
func (s *TranscodeService) SetPlaybackPosition(mediaID string, positionSec float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, j := range s.running {
		if j.Media.ID == mediaID {
			j.SetPlaybackPosition(positionSec)
		}
	}
}

// FindRunningJob 根据 media+quality 查找正在运行的 job（用于 Throttle 调试等）
func (s *TranscodeService) FindRunningJob(mediaID, quality string) *TranscodeJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, j := range s.running {
		if j.Media.ID == mediaID && j.Quality == quality {
			return j
		}
	}
	return nil
}

// GetRunningJobs 获取正在运行的转码任务
func (s *TranscodeService) GetRunningJobs() []*TranscodeJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*TranscodeJob, 0, len(s.running))
	for _, job := range s.running {
		jobs = append(jobs, job)
	}
	return jobs
}

// GetHWAccelInfo 获取硬件加速信息
func (s *TranscodeService) GetHWAccelInfo() string {
	return s.hwAccel
}

// GetAvailableQualities 根据原始视频分辨率动态计算可用的转码质量等级
// 规则：只提供低于或等于原始分辨率的质量选项
func (s *TranscodeService) GetAvailableQualities(media *model.Media) []string {
	// 解析原始分辨率
	origHeight := parseResolutionHeight(media.Resolution)
	if origHeight <= 0 {
		// 无法判断原始分辨率，返回默认选项
		return []string{"360p", "480p", "720p", "1080p"}
	}

	// 按从低到高排序的质量等级
	orderedQualities := []struct {
		name   string
		height int
	}{
		{"360p", 360},
		{"480p", 480},
		{"720p", 720},
		{"1080p", 1080},
		{"2K", 1440},
		{"4K", 2160},
	}

	var available []string
	for _, q := range orderedQualities {
		if q.height <= origHeight {
			available = append(available, q.name)
		}
	}

	if len(available) == 0 {
		available = []string{"360p"}
	}

	return available
}

// parseResolutionHeight 解析分辨率字符串获取高度
func parseResolutionHeight(resolution string) int {
	switch resolution {
	case "4K":
		return 2160
	case "2K":
		return 1440
	case "1080p":
		return 1080
	case "720p":
		return 720
	case "480p":
		return 480
	case "360p":
		return 360
	default:
		// 尝试解析 "NNNp" 格式
		if strings.HasSuffix(resolution, "p") {
			if h, err := strconv.Atoi(strings.TrimSuffix(resolution, "p")); err == nil {
				return h
			}
		}
		return 0
	}
}

// CancelTranscode 取消正在运行的转码任务
func (s *TranscodeService) CancelTranscode(taskID string) error {
	s.mu.RLock()
	job, exists := s.running[taskID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("转码任务不存在或已完成: %s", taskID)
	}

	// 发送取消信号
	select {
	case job.Cancel <- struct{}{}:
		s.logger.Infof("已发送取消信号: %s", taskID)
	default:
		// 已经取消过了
	}

	return nil
}

// calcFFmpegThreads 根据资源限制配置动态计算 FFmpeg 线程数
// Deprecated: 保留兼容接口，实际委托给 ffmpeg.CalcThreads。
func (s *TranscodeService) calcFFmpegThreads() int {
	return ffmpeg.CalcThreads(s.cfg)
}

// buildFFmpegHDRTonemapFilter 构建 HDR→SDR 色调映射滤镜
// 当源视频为 HDR（如 HEVC/HDR10）且目标设备不支持 HDR 时，
// 使用 FFmpeg 的 tonemap 滤镜进行色调映射，避免画面偏灰
func (s *TranscodeService) buildFFmpegHDRTonemapFilter(media *model.Media, width, height int) string {
	// 检测是否为 HDR 视频（基于编码格式推断）
	codec := strings.ToLower(media.VideoCodec)
	isHDR := codec == "hevc" || codec == "h265" || codec == "vp9" || codec == "av1"

	if !isHDR {
		return fmt.Sprintf("scale=%d:%d", width, height)
	}

	// HDR → SDR 色调映射滤镜链
	// zscale: 颜色空间转换 → tonemap: 色调映射 → zscale: 输出格式 → scale: 缩放
	return fmt.Sprintf(
		"zscale=t=linear:npl=100,format=gbrpf32le,"+
			"tonemap=hable:desat=0,"+
			"zscale=p=bt709:t=bt709:m=bt709:r=tv,"+
			"format=yuv420p,scale=%d:%d",
		width, height,
	)
}

// CleanupStaleCache 清理过期的转码缓存（供 Scheduler cleanup 任务调用）
//
// 清理规则：
//   - done 且 updated_at < now - doneRetainDays 天 → 删除目录 + DB 记录
//   - failed/cancelled 且 updated_at < now - failedRetainDays 天 → 删除目录 + DB 记录
//   - 正在运行（running/pending）的任务不会被触碰
//
// 返回：(清理的目录数, 清理的 DB 记录数, 错误)
func (s *TranscodeService) CleanupStaleCache(doneRetainDays, failedRetainDays int) (int, int, error) {
	if doneRetainDays <= 0 {
		doneRetainDays = 30
	}
	if failedRetainDays <= 0 {
		failedRetainDays = 7
	}

	now := time.Now()
	dirsCleaned := 0
	recordsCleaned := 0

	// 清理 done 状态
	doneStale, err := s.repo.ListStaleDone(now.AddDate(0, 0, -doneRetainDays))
	if err != nil {
		return 0, 0, fmt.Errorf("查询过期完成任务失败: %w", err)
	}
	for _, t := range doneStale {
		// 安全检查：如果任务还在 running map 里（理论上不该，但兜底），跳过
		s.mu.RLock()
		_, stillRunning := s.running[t.ID]
		s.mu.RUnlock()
		if stillRunning {
			continue
		}
		dir := s.GetOutputDir(t.MediaID, t.Quality)
		if err := os.RemoveAll(dir); err == nil {
			dirsCleaned++
		} else {
			s.logger.Warnf("清理目录失败 %s: %v", dir, err)
		}
		if err := s.repo.DeleteByID(t.ID); err == nil {
			recordsCleaned++
		}
	}

	// 清理 failed/cancelled 状态
	failedStale, err := s.repo.ListStaleFailed(now.AddDate(0, 0, -failedRetainDays))
	if err != nil {
		return dirsCleaned, recordsCleaned, fmt.Errorf("查询过期失败任务失败: %w", err)
	}
	for _, t := range failedStale {
		dir := s.GetOutputDir(t.MediaID, t.Quality)
		if err := os.RemoveAll(dir); err == nil {
			dirsCleaned++
		}
		if err := s.repo.DeleteByID(t.ID); err == nil {
			recordsCleaned++
		}
	}

	s.logger.Infof("缓存清理完成: 删除 %d 个目录, %d 条 DB 记录", dirsCleaned, recordsCleaned)
	return dirsCleaned, recordsCleaned, nil
}
