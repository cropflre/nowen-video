package service

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
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

	return ts
}

// detectHWAccel 检测可用的硬件加速方式
func (s *TranscodeService) detectHWAccel() string {
	if s.cfg.App.HWAccel != "auto" && s.cfg.App.HWAccel != "" {
		return s.cfg.App.HWAccel
	}

	// 尝试检测NVIDIA GPU
	if s.detectNvidiaSmi() {
		// 验证nvenc是否可用
		cmd := exec.Command(s.cfg.App.FFmpegPath, "-hide_banner", "-encoders")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), "h264_nvenc") {
			s.logger.Info("检测到NVIDIA GPU，使用NVENC硬件加速")
			return "nvenc"
		}
	}

	// 检测Intel QSV（群晖NAS常见）
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
			cmd := exec.Command(s.cfg.App.FFmpegPath, "-hide_banner", "-encoders")
			output, err := cmd.Output()
			if err == nil {
				if strings.Contains(string(output), "h264_qsv") {
					s.logger.Info("检测到Intel QSV，使用QSV硬件加速")
					return "qsv"
				}
				if strings.Contains(string(output), "h264_vaapi") {
					s.logger.Info("检测到VAAPI，使用VAAPI硬件加速")
					return "vaapi"
				}
			}
		}
	}

	s.logger.Warn("未检测到硬件加速，使用软件编码")
	return "none"
}

// detectNvidiaSmi 检测 nvidia-smi 是否可用
// 在 Windows 上 LookPath 可能因 PATH 不完整而找不到 nvidia-smi，
// 因此额外检查常见安装路径并尝试直接执行验证
func (s *TranscodeService) detectNvidiaSmi() bool {
	// 优先通过 PATH 查找
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		return true
	}

	// Windows 下 nvidia-smi 可能不在 PATH 中，尝试常见路径
	if runtime.GOOS == "windows" {
		commonPaths := []string{
			filepath.Join(os.Getenv("SystemRoot"), "System32", "nvidia-smi.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe"),
		}
		for _, p := range commonPaths {
			if _, err := os.Stat(p); err == nil {
				// 找到文件，尝试执行验证
				if out, err := exec.Command(p, "--query-gpu=name", "--format=csv,noheader").Output(); err == nil {
					gpuName := strings.TrimSpace(string(out))
					s.logger.Infof("通过路径 %s 检测到 GPU: %s", p, gpuName)
					return true
				}
			}
		}

		// 最后尝试直接执行（某些环境下 System32 不在 Go 的 LookPath 搜索范围内，但 CreateProcess 可以找到）
		if out, err := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader").Output(); err == nil {
			gpuName := strings.TrimSpace(string(out))
			s.logger.Infof("直接执行 nvidia-smi 检测到 GPU: %s", gpuName)
			return true
		}
	}

	return false
}

// buildFFmpegArgs 根据硬件加速模式构建FFmpeg参数
func (s *TranscodeService) buildFFmpegArgs(inputPath, outputDir, quality string) []string {
	qc, ok := qualityPresets[quality]
	if !ok {
		qc = qualityPresets["720p"]
	}

	outputPath := filepath.Join(outputDir, "stream.m3u8")
	segmentPath := filepath.Join(outputDir, "seg%04d.ts")

	// 基础参数
	baseArgs := []string{"-y", "-i", inputPath}

	// HLS输出参数
	hlsArgs := []string{
		"-f", "hls",
		"-hls_time", "6",
		"-hls_list_size", "0",
		"-hls_segment_filename", segmentPath,
		"-hls_flags", "independent_segments",
		outputPath,
	}

	// 音频参数（通用）
	audioArgs := []string{"-c:a", "aac", "-b:a", qc.AudioBitrate, "-ac", "2"}

	var videoArgs []string

	switch s.hwAccel {
	case "nvenc":
		// NVIDIA NVENC
		baseArgs = append([]string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}, baseArgs...)
		videoArgs = []string{
			"-c:v", "h264_nvenc",
			"-preset", "p4", // 平衡质量和速度
			"-b:v", qc.VideoBitrate,
			"-pix_fmt", "yuv420p", // 强制 8-bit 输出，确保所有设备兼容
			"-vf", fmt.Sprintf("scale_cuda=%d:%d", qc.Width, qc.Height),
		}

	case "qsv":
		// Intel Quick Sync Video
		baseArgs = append([]string{"-hwaccel", "qsv", "-hwaccel_output_format", "qsv"}, baseArgs...)
		videoArgs = []string{
			"-c:v", "h264_qsv",
			"-preset", s.cfg.App.TranscodePreset,
			"-global_quality", "23",
			"-pix_fmt", "yuv420p", // 强制 8-bit 输出，确保所有设备兼容
			"-vf", fmt.Sprintf("scale_qsv=%d:%d", qc.Width, qc.Height),
		}

	case "vaapi":
		// VAAPI
		baseArgs = append([]string{
			"-hwaccel", "vaapi",
			"-hwaccel_output_format", "vaapi",
			"-vaapi_device", s.cfg.App.VAAPIDevice,
		}, baseArgs...)
		videoArgs = []string{
			"-c:v", "h264_vaapi",
			"-b:v", qc.VideoBitrate,
			"-pix_fmt", "yuv420p", // 强制 8-bit 输出，确保所有设备兼容
			"-vf", fmt.Sprintf("scale_vaapi=w=%d:h=%d", qc.Width, qc.Height),
		}

	default:
		// 软件编码
		videoArgs = []string{
			"-c:v", "libx264",
			"-preset", s.cfg.App.TranscodePreset,
			"-crf", "23",
			"-pix_fmt", "yuv420p", // 强制 8-bit 输出，确保所有设备兼容
			"-vf", fmt.Sprintf("scale=%d:%d", qc.Width, qc.Height),
		}
	}

	// 根据资源限制配置动态计算 FFmpeg 线程数
	ffmpegThreads := s.calcFFmpegThreads()
	args := append(baseArgs, "-threads", fmt.Sprintf("%d", ffmpegThreads))
	args = append(args, videoArgs...)
	args = append(args, audioArgs...)
	args = append(args, hlsArgs...)

	return args
}

// GetOutputDir 获取转码输出目录
func (s *TranscodeService) GetOutputDir(mediaID, quality string) string {
	return filepath.Join(s.cfg.Cache.CacheDir, "transcode", mediaID, quality)
}

// StartTranscode 发起转码任务
func (s *TranscodeService) StartTranscode(media *model.Media, quality string) (*model.TranscodeTask, error) {
	// 检查是否已有完成的转码
	if task, err := s.repo.FindByMediaAndQuality(media.ID, quality); err == nil {
		outputDir := s.GetOutputDir(media.ID, quality)
		m3u8Path := filepath.Join(outputDir, "stream.m3u8")
		if _, err := os.Stat(m3u8Path); err == nil {
			return task, nil // 已有缓存
		}
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
		Task:    task,
		Media:   media,
		Quality: quality,
		Cancel:  make(chan struct{}),
	}

	s.mu.Lock()
	s.running[task.ID] = job
	s.mu.Unlock()

	s.jobs <- job

	return task, nil
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

	args := s.buildFFmpegArgs(job.Media.FilePath, job.Task.OutputDir, job.Quality)
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
		s.logger.Errorf("转码失败: %s, 错误: %v", job.Media.Title, waitErr)
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
func (s *TranscodeService) parseFFmpegProgress(stderr io.ReadCloser, job *TranscodeJob) {
	// FFmpeg进度输出格式: frame=  120 fps= 60 ... time=00:00:05.00 ... speed=2.50x
	timeRegex := regexp.MustCompile(`time=(\d{2}):(\d{2}):(\d{2})\.(\d{2})`)
	speedRegex := regexp.MustCompile(`speed=\s*([\d.]+)x`)

	totalDuration := job.Media.Duration // 总时长（秒）
	scanner := bufio.NewScanner(stderr)
	scanner.Split(bufio.ScanLines)

	var lastProgress float64

	for scanner.Scan() {
		line := scanner.Text()

		// 解析当前时间位置
		timeMatches := timeRegex.FindStringSubmatch(line)
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

		// 只在进度变化超过1%时更新，避免频繁广播
		if progress-lastProgress < 1 {
			continue
		}
		lastProgress = progress

		// 解析速度
		speed := ""
		speedMatches := speedRegex.FindStringSubmatch(line)
		if len(speedMatches) >= 2 {
			speed = speedMatches[1] + "x"
		}

		// 更新任务进度
		job.Task.Progress = progress
		s.repo.Update(job.Task)

		// 广播进度事件
		s.broadcastTranscodeEvent(EventTranscodeProgress, &TranscodeProgressData{
			TaskID:   job.Task.ID,
			MediaID:  job.Media.ID,
			Title:    job.Media.Title,
			Quality:  job.Quality,
			Progress: progress,
			Speed:    speed,
			Message:  fmt.Sprintf("转码中: %.1f%% (速度: %s)", progress, speed),
		})
	}
}

// broadcastTranscodeEvent 广播转码事件
func (s *TranscodeService) broadcastTranscodeEvent(eventType string, data *TranscodeProgressData) {
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(eventType, data)
	}
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
func (s *TranscodeService) calcFFmpegThreads() int {
	resourceLimit := s.cfg.App.ResourceLimit
	if resourceLimit <= 0 || resourceLimit > 80 {
		resourceLimit = 80
	}
	cpuCount := runtime.NumCPU()
	threads := cpuCount * resourceLimit / 100
	if threads < 1 {
		threads = 1
	}
	if threads > cpuCount {
		threads = cpuCount
	}
	return threads
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
