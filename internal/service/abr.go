package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nowen-video/nowen-video/internal/config"
	transcodeprofile "github.com/nowen-video/nowen-video/internal/transcode/profile"
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

// ABRProfile remains as a compatibility alias for preprocess and API callers.
// The ordered ladder and bitrate policies live in internal/transcode/profile.
type ABRProfile = transcodeprofile.EncodingProfile

var abrProfiles = transcodeprofile.PersistentProfiles()

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
	return &ABRStatus{
		Enabled:    true,
		GPU:        *gpu,
		MaxStreams: gpu.MaxStreams,
		Profiles:   transcodeprofile.Names(),
	}
}
