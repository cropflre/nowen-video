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

// 质量预设
var qualityPresets = map[string]QualityConfig{
	"360p":  {Width: 640, Height: 360, VideoBitrate: "800k", AudioBitrate: "96k"},
	"480p":  {Width: 854, Height: 480, VideoBitrate: "1500k", AudioBitrate: "128k"},
	"720p":  {Width: 1280, Height: 720, VideoBitrate: "3000k", AudioBitrate: "128k"},
	"1080p": {Width: 1920, Height: 1080, VideoBitrate: "6000k", AudioBitrate: "192k"},
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
	repo    *repository.TranscodeRepo
	cfg     *config.Config
	logger  *zap.SugaredLogger
	jobs    chan *TranscodeJob
	mu      sync.RWMutex
	running map[string]*TranscodeJob
	hwAccel string // 检测到的硬件加速方式
	wsHub   *WSHub // WebSocket事件广播
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
}

func NewTranscodeService(repo *repository.TranscodeRepo, cfg *config.Config, logger *zap.SugaredLogger) *TranscodeService {
	ts := &TranscodeService{
		repo:    repo,
		cfg:     cfg,
		logger:  logger,
		jobs:    make(chan *TranscodeJob, 100),
		running: make(map[string]*TranscodeJob),
	}

	// 检测硬件加速能力
	ts.hwAccel = ts.detectHWAccel()
	logger.Infof("硬件加速模式: %s", ts.hwAccel)

	// 启动转码工作协程
	for i := 0; i < cfg.App.MaxTranscodeJobs; i++ {
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
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
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
			"-vf", fmt.Sprintf("scale_cuda=%d:%d", qc.Width, qc.Height),
		}

	case "qsv":
		// Intel Quick Sync Video
		baseArgs = append([]string{"-hwaccel", "qsv", "-hwaccel_output_format", "qsv"}, baseArgs...)
		videoArgs = []string{
			"-c:v", "h264_qsv",
			"-preset", s.cfg.App.TranscodePreset,
			"-global_quality", "23",
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
			"-vf", fmt.Sprintf("scale_vaapi=w=%d:h=%d", qc.Width, qc.Height),
		}

	default:
		// 软件编码
		videoArgs = []string{
			"-c:v", "libx264",
			"-preset", s.cfg.App.TranscodePreset,
			"-crf", "23",
			"-vf", fmt.Sprintf("scale=%d:%d", qc.Width, qc.Height),
		}
	}

	args := append(baseArgs, videoArgs...)
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

	if err := cmd.Wait(); err != nil {
		s.logger.Errorf("转码失败: %s, 错误: %v", job.Media.Title, err)
		job.Task.Status = "failed"
		job.Task.Error = err.Error()
		s.repo.Update(job.Task)
		s.broadcastTranscodeEvent(EventTranscodeFailed, &TranscodeProgressData{
			TaskID:  job.Task.ID,
			MediaID: job.Media.ID,
			Title:   job.Media.Title,
			Quality: job.Quality,
			Message: fmt.Sprintf("转码失败: %v", err),
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
