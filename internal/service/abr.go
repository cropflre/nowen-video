package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"go.uber.org/zap"
)

// ABRService 自适应码率（Adaptive Bitrate）服务
// 引入 ABR 技术，支持 GPU 加速的多路并行转码
type ABRService struct {
	cfg      *config.Config
	logger   *zap.SugaredLogger
	mu       sync.RWMutex
	wsHub    *WSHub
	cacheDir string
	hwAccel  string
}

// ABRProfile ABR 配置文件
type ABRProfile struct {
	Name         string `json:"name"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	VideoBitrate string `json:"video_bitrate"`
	AudioBitrate string `json:"audio_bitrate"`
	MaxBitrate   string `json:"max_bitrate"`
	BufSize      string `json:"buf_size"`
}

// ABR 质量阶梯
var abrProfiles = []ABRProfile{
	{Name: "360p", Width: 640, Height: 360, VideoBitrate: "800k", AudioBitrate: "96k", MaxBitrate: "1200k", BufSize: "1600k"},
	{Name: "480p", Width: 854, Height: 480, VideoBitrate: "1400k", AudioBitrate: "128k", MaxBitrate: "2100k", BufSize: "2800k"},
	{Name: "720p", Width: 1280, Height: 720, VideoBitrate: "2800k", AudioBitrate: "128k", MaxBitrate: "4200k", BufSize: "5600k"},
	{Name: "1080p", Width: 1920, Height: 1080, VideoBitrate: "5000k", AudioBitrate: "192k", MaxBitrate: "7500k", BufSize: "10000k"},
	{Name: "2K", Width: 2560, Height: 1440, VideoBitrate: "10000k", AudioBitrate: "192k", MaxBitrate: "15000k", BufSize: "20000k"},
	{Name: "4K", Width: 3840, Height: 2160, VideoBitrate: "20000k", AudioBitrate: "256k", MaxBitrate: "30000k", BufSize: "40000k"},
}

// GPUInfo GPU 信息
type GPUInfo struct {
	Available   bool     `json:"available"`
	Type        string   `json:"type"` // nvenc / qsv / vaapi / none
	Name        string   `json:"name"`
	Encoders    []string `json:"encoders"`
	MaxStreams  int      `json:"max_streams"` // 最大并行流数
	MemoryMB    int      `json:"memory_mb"`
	Utilization float64  `json:"utilization"`
}

// ABRStatus ABR 状态
type ABRStatus struct {
	Enabled       bool     `json:"enabled"`
	GPU           GPUInfo  `json:"gpu"`
	ActiveStreams int      `json:"active_streams"`
	MaxStreams    int      `json:"max_streams"`
	Profiles      []string `json:"profiles"`
}

func NewABRService(cfg *config.Config, hwAccel string, logger *zap.SugaredLogger) *ABRService {
	cacheDir := filepath.Join(cfg.Cache.CacheDir, "abr")
	os.MkdirAll(cacheDir, 0755)

	return &ABRService{
		cfg:      cfg,
		logger:   logger,
		cacheDir: cacheDir,
		hwAccel:  hwAccel,
	}
}

// SetWSHub 设置WebSocket Hub
func (s *ABRService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// GenerateABRPlaylist 生成 ABR 主播放列表（Master Playlist）
// 为指定媒体生成多码率 HLS 主播放列表
func (s *ABRService) GenerateABRPlaylist(mediaID, inputPath string, maxHeight int) (string, error) {
	outputDir := filepath.Join(s.cacheDir, mediaID)
	masterPath := filepath.Join(outputDir, "master.m3u8")

	// 检查是否已有缓存
	if _, err := os.Stat(masterPath); err == nil {
		return masterPath, nil
	}

	os.MkdirAll(outputDir, 0755)

	// 确定可用的质量等级（不超过原始分辨率）
	var profiles []ABRProfile
	for _, p := range abrProfiles {
		if p.Height <= maxHeight {
			profiles = append(profiles, p)
		}
	}
	if len(profiles) == 0 {
		profiles = abrProfiles[:1] // 至少保留 360p
	}

	// 生成 Master Playlist
	var masterContent strings.Builder
	masterContent.WriteString("#EXTM3U\n")
	masterContent.WriteString("#EXT-X-VERSION:3\n\n")

	for _, p := range profiles {
		bandwidth := parseBitrate(p.VideoBitrate) + parseBitrate(p.AudioBitrate)
		masterContent.WriteString(fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,NAME=\"%s\"\n",
			bandwidth, p.Width, p.Height, p.Name,
		))
		masterContent.WriteString(fmt.Sprintf("%s/stream.m3u8\n\n", p.Name))
	}

	if err := os.WriteFile(masterPath, []byte(masterContent.String()), 0644); err != nil {
		return "", fmt.Errorf("写入主播放列表失败: %w", err)
	}

	// 异步生成各质量等级的 HLS 流
	go s.generateAllVariants(mediaID, inputPath, outputDir, profiles)

	return masterPath, nil
}

// generateAllVariants 并行生成所有质量变体
func (s *ABRService) generateAllVariants(mediaID, inputPath, outputDir string, profiles []ABRProfile) {
	var wg sync.WaitGroup

	// 根据资源限制配置动态计算并行度
	resourceLimit := s.cfg.App.ResourceLimit
	if resourceLimit <= 0 || resourceLimit > 80 {
		resourceLimit = 80
	}
	maxParallel := 1
	if resourceLimit >= 30 && s.hwAccel != "none" {
		maxParallel = 2
	}
	sem := make(chan struct{}, maxParallel)

	for _, p := range profiles {
		wg.Add(1)
		sem <- struct{}{}

		go func(profile ABRProfile) {
			defer wg.Done()
			defer func() { <-sem }()

			variantDir := filepath.Join(outputDir, profile.Name)
			os.MkdirAll(variantDir, 0755)

			// 检查是否已存在
			m3u8Path := filepath.Join(variantDir, "stream.m3u8")
			if _, err := os.Stat(m3u8Path); err == nil {
				return
			}

			args := s.buildABRFFmpegArgs(inputPath, variantDir, profile)
			s.logger.Debugf("ABR 转码: %s -> %s", profile.Name, strings.Join(args, " "))

			cmd := exec.Command(s.cfg.App.FFmpegPath, args...)
			setLowPriority(cmd) // 极低资源模式：FFmpeg 以最低优先级运行
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				s.logger.Errorf("ABR 转码失败 (%s): %v", profile.Name, err)
				return
			}

			s.logger.Infof("ABR 转码完成: %s (%s)", mediaID, profile.Name)
		}(p)
	}

	wg.Wait()
	s.logger.Infof("ABR 所有变体生成完成: %s", mediaID)
}

// buildABRFFmpegArgs 构建 ABR 转码 FFmpeg 参数
func (s *ABRService) buildABRFFmpegArgs(inputPath, outputDir string, profile ABRProfile) []string {
	outputPath := filepath.Join(outputDir, "stream.m3u8")
	segmentPath := filepath.Join(outputDir, "seg%04d.ts")

	baseArgs := []string{"-y", "-i", inputPath}

	// HLS 输出参数
	hlsArgs := []string{
		"-f", "hls",
		"-hls_time", "4", // 4秒分片（ABR 推荐更短）
		"-hls_list_size", "0",
		"-hls_segment_filename", segmentPath,
		"-hls_flags", "independent_segments",
		outputPath,
	}

	audioArgs := []string{"-c:a", "aac", "-b:a", profile.AudioBitrate, "-ac", "2"}

	var videoArgs []string

	switch s.hwAccel {
	case "nvenc":
		baseArgs = append([]string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}, baseArgs...)
		videoArgs = []string{
			"-c:v", "h264_nvenc",
			"-preset", "p4",
			"-b:v", profile.VideoBitrate,
			"-maxrate", profile.MaxBitrate,
			"-bufsize", profile.BufSize,
			"-vf", fmt.Sprintf("scale_cuda=%d:%d", profile.Width, profile.Height),
			"-g", "48", // GOP 大小（关键帧间隔）
			"-keyint_min", "48",
			"-sc_threshold", "0",
		}

	case "qsv":
		baseArgs = append([]string{"-hwaccel", "qsv", "-hwaccel_output_format", "qsv"}, baseArgs...)
		videoArgs = []string{
			"-c:v", "h264_qsv",
			"-preset", "medium",
			"-b:v", profile.VideoBitrate,
			"-maxrate", profile.MaxBitrate,
			"-bufsize", profile.BufSize,
			"-vf", fmt.Sprintf("scale_qsv=%d:%d", profile.Width, profile.Height),
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
			"-b:v", profile.VideoBitrate,
			"-maxrate", profile.MaxBitrate,
			"-bufsize", profile.BufSize,
			"-vf", fmt.Sprintf("scale_vaapi=w=%d:h=%d", profile.Width, profile.Height),
			"-g", "48",
			"-keyint_min", "48",
		}

	default:
		videoArgs = []string{
			"-c:v", "libx264",
			"-preset", "medium",
			"-b:v", profile.VideoBitrate,
			"-maxrate", profile.MaxBitrate,
			"-bufsize", profile.BufSize,
			"-vf", fmt.Sprintf("scale=%d:%d", profile.Width, profile.Height),
			"-g", "48",
			"-keyint_min", "48",
			"-sc_threshold", "0",
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

// GetGPUInfo 获取 GPU 信息
func (s *ABRService) GetGPUInfo() *GPUInfo {
	info := &GPUInfo{
		Type: s.hwAccel,
	}

	if s.hwAccel == "none" {
		return info
	}

	info.Available = true

	switch s.hwAccel {
	case "nvenc":
		info.Name = "NVIDIA GPU"
		info.Encoders = []string{"h264_nvenc", "hevc_nvenc"}
		info.MaxStreams = 4
		// 尝试获取 GPU 利用率
		if out, err := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu,memory.total", "--format=csv,noheader,nounits").Output(); err == nil {
			fmt.Sscanf(strings.TrimSpace(string(out)), "%f, %d", &info.Utilization, &info.MemoryMB)
		}

	case "qsv":
		info.Name = "Intel Quick Sync Video"
		info.Encoders = []string{"h264_qsv", "hevc_qsv"}
		info.MaxStreams = 3

	case "vaapi":
		info.Name = "VA-API"
		info.Encoders = []string{"h264_vaapi", "hevc_vaapi"}
		info.MaxStreams = 2
	}

	return info
}

// GetABRStatus 获取 ABR 状态
func (s *ABRService) GetABRStatus() *ABRStatus {
	gpu := s.GetGPUInfo()

	profileNames := make([]string, len(abrProfiles))
	for i, p := range abrProfiles {
		profileNames[i] = p.Name
	}

	return &ABRStatus{
		Enabled:    true,
		GPU:        *gpu,
		MaxStreams: gpu.MaxStreams,
		Profiles:   profileNames,
	}
}

// CleanABRCache 清理 ABR 缓存
func (s *ABRService) CleanABRCache(mediaID string) error {
	cacheDir := filepath.Join(s.cacheDir, mediaID)
	return os.RemoveAll(cacheDir)
}

// CleanAllABRCache 清理所有 ABR 缓存
func (s *ABRService) CleanAllABRCache() (int64, error) {
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil {
		return 0, err
	}

	var totalSize int64
	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(s.cacheDir, entry.Name())
			size := getDirSize(dirPath)
			totalSize += size
			os.RemoveAll(dirPath)
		}
	}

	return totalSize, nil
}

// getDirSize 获取目录大小
func getDirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// parseBitrate 解析比特率字符串为数值（bps）
func parseBitrate(s string) int {
	s = strings.TrimSpace(s)
	multiplier := 1
	if strings.HasSuffix(s, "k") {
		multiplier = 1000
		s = strings.TrimSuffix(s, "k")
	} else if strings.HasSuffix(s, "M") {
		multiplier = 1000000
		s = strings.TrimSuffix(s, "M")
	}

	var val int
	fmt.Sscanf(s, "%d", &val)
	return val * multiplier
}

// 确保 time 包被使用
var _ = time.Now

// calcFFmpegThreads 根据资源限制配置动态计算 FFmpeg 线程数
func (s *ABRService) calcFFmpegThreads() int {
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
