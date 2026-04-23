package service

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"go.uber.org/zap"
)

// SystemMetrics 系统指标快照
type SystemMetrics struct {
	Timestamp   int64            `json:"timestamp"`
	CPU         CPUMetrics       `json:"cpu"`
	Memory      MemoryMetrics    `json:"memory"`
	Disk        DiskMetrics      `json:"disk"`
	Transcode   TranscodeMetrics `json:"transcode"`
	Connections int              `json:"connections"` // WebSocket连接数
}

// CPUMetrics CPU指标
type CPUMetrics struct {
	UsagePercent float64 `json:"usage_percent"` // CPU使用率
	Cores        int     `json:"cores"`         // CPU核心数
	Goroutines   int     `json:"goroutines"`    // Go协程数
}

// MemoryMetrics 内存指标
type MemoryMetrics struct {
	TotalMB     float64 `json:"total_mb"`
	UsedMB      float64 `json:"used_mb"`
	FreeMB      float64 `json:"free_mb"`
	UsedPercent float64 `json:"used_percent"`
	// Go运行时内存
	GoAllocMB      float64 `json:"go_alloc_mb"`
	GoSysMB        float64 `json:"go_sys_mb"`
	GoTotalAllocMB float64 `json:"go_total_alloc_mb"`
}

// DiskMetrics 磁盘指标
type DiskMetrics struct {
	TotalGB     float64 `json:"total_gb"`
	UsedGB      float64 `json:"used_gb"`
	FreeGB      float64 `json:"free_gb"`
	UsedPercent float64 `json:"used_percent"`
	CacheSizeMB float64 `json:"cache_size_mb"` // 转码缓存大小
}

// TranscodeMetrics 转码指标
type TranscodeMetrics struct {
	ActiveJobs int    `json:"active_jobs"` // 活跃转码任务数
	QueueSize  int    `json:"queue_size"`  // 队列中的任务数
	HWAccel    string `json:"hw_accel"`    // 硬件加速模式
}

// MonitorService 系统监控服务
type MonitorService struct {
	cfg       *config.Config
	wsHub     *WSHub
	transcode *TranscodeService
	logger    *zap.SugaredLogger
	stopCh    chan struct{}
	metrics   *SystemMetrics
	mu        sync.RWMutex
}

// NewMonitorService 创建监控服务
func NewMonitorService(cfg *config.Config, transcode *TranscodeService, logger *zap.SugaredLogger) *MonitorService {
	return &MonitorService{
		cfg:       cfg,
		transcode: transcode,
		logger:    logger,
		stopCh:    make(chan struct{}),
		metrics:   &SystemMetrics{},
	}
}

// SetWSHub 设置WebSocket Hub
func (s *MonitorService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// Start 启动监控，定期采集并广播
func (s *MonitorService) Start() {
	s.logger.Info("系统监控服务已启动")
	go s.collectLoop()
}

// Stop 停止监控
func (s *MonitorService) Stop() {
	close(s.stopCh)
}

// collectLoop 指标采集循环
func (s *MonitorService) collectLoop() {
	// 启动时立即采集一次
	s.collect()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.collect()
		}
	}
}

// collect 采集系统指标
func (s *MonitorService) collect() {
	metrics := &SystemMetrics{
		Timestamp: time.Now().UnixMilli(),
	}

	// CPU
	metrics.CPU = s.collectCPU()

	// Memory
	metrics.Memory = s.collectMemory()

	// Disk
	metrics.Disk = s.collectDisk()

	// Transcode
	metrics.Transcode = s.collectTranscode()

	// WebSocket连接数
	if s.wsHub != nil {
		metrics.Connections = s.wsHub.ClientCount()
	}

	s.mu.Lock()
	s.metrics = metrics
	s.mu.Unlock()

	// 广播到前端（仅当有WebSocket客户端时）
	if s.wsHub != nil && s.wsHub.ClientCount() > 0 {
		s.wsHub.BroadcastEvent("system_metrics", metrics)
	}
}

// GetMetrics 获取当前指标快照（深拷贝，避免数据竞争）
func (s *MonitorService) GetMetrics() *SystemMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.metrics == nil {
		return nil
	}
	cp := *s.metrics
	return &cp
}

// collectCPU 采集CPU指标
func (s *MonitorService) collectCPU() CPUMetrics {
	return CPUMetrics{
		UsagePercent: s.getCPUUsage(),
		Cores:        runtime.NumCPU(),
		Goroutines:   runtime.NumGoroutine(),
	}
}

// collectMemory 采集内存指标
func (s *MonitorService) collectMemory() MemoryMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	mem := MemoryMetrics{
		GoAllocMB:      float64(m.Alloc) / 1024 / 1024,
		GoSysMB:        float64(m.Sys) / 1024 / 1024,
		GoTotalAllocMB: float64(m.TotalAlloc) / 1024 / 1024,
	}

	// 获取系统内存（跨平台）
	totalMB, usedMB := s.getSystemMemory()
	mem.TotalMB = totalMB
	mem.UsedMB = usedMB
	if totalMB > 0 {
		mem.FreeMB = totalMB - usedMB
		mem.UsedPercent = usedMB / totalMB * 100
	}

	return mem
}

// collectDisk 采集磁盘指标
func (s *MonitorService) collectDisk() DiskMetrics {
	disk := DiskMetrics{}

	// 获取数据目录所在磁盘使用情况
	totalGB, usedGB, freeGB := s.getDiskUsage(s.cfg.App.DataDir)
	disk.TotalGB = totalGB
	disk.UsedGB = usedGB
	disk.FreeGB = freeGB
	if totalGB > 0 {
		disk.UsedPercent = usedGB / totalGB * 100
	}

	// 计算缓存大小
	disk.CacheSizeMB = s.getDirSizeMB(s.cfg.Cache.CacheDir)

	return disk
}

// collectTranscode 采集转码指标
func (s *MonitorService) collectTranscode() TranscodeMetrics {
	tm := TranscodeMetrics{
		HWAccel: s.transcode.GetHWAccelInfo(),
	}

	jobs := s.transcode.GetRunningJobs()
	tm.ActiveJobs = len(jobs)

	return tm
}

// ==================== 平台相关的指标采集 ====================

// getCPUUsage 获取CPU使用率（简化实现）
func (s *MonitorService) getCPUUsage() float64 {
	if runtime.GOOS == "linux" {
		// 通过 /proc/stat 获取
		data, err := os.ReadFile("/proc/stat")
		if err == nil {
			lines := strings.Split(string(data), "\n")
			if len(lines) > 0 {
				fields := strings.Fields(lines[0])
				if len(fields) >= 5 {
					user, _ := strconv.ParseFloat(fields[1], 64)
					system, _ := strconv.ParseFloat(fields[3], 64)
					idle, _ := strconv.ParseFloat(fields[4], 64)
					total := user + system + idle
					if total > 0 {
						return (user + system) / total * 100
					}
				}
			}
		}
	}

	if runtime.GOOS == "windows" {
		// 优先使用 PowerShell 获取 CPU 使用率（wmic 已弃用）
		if out, err := exec.Command("powershell", "-NoProfile", "-Command",
			"(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average").Output(); err == nil {
			if v, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil && v > 0 {
				return v
			}
		}
		// 降级到 wmic
		if output, err := exec.Command("wmic", "cpu", "get", "loadpercentage").Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if v, err := strconv.ParseFloat(line, 64); err == nil {
					return v
				}
			}
		}
	}

	// 降级：用goroutine数估算
	return float64(runtime.NumGoroutine()) / float64(runtime.NumCPU()) * 10
}

// getSystemMemory 获取系统内存（MB，保留小数精度）
func (s *MonitorService) getSystemMemory() (totalMB, usedMB float64) {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/meminfo")
		if err == nil {
			lines := strings.Split(string(data), "\n")
			var total, available float64
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) < 2 {
					continue
				}
				val, _ := strconv.ParseFloat(fields[1], 64)
				switch fields[0] {
				case "MemTotal:":
					total = val / 1024 // KB -> MB
				case "MemAvailable:":
					available = val / 1024
				}
			}
			if total > 0 {
				return total, total - available
			}
		}
	}

	if runtime.GOOS == "windows" {
		// 优先使用 PowerShell（wmic 已弃用）
		if out, err := exec.Command("powershell", "-NoProfile", "-Command",
			"(Get-CimInstance Win32_OperatingSystem | Select-Object TotalVisibleMemorySize,FreePhysicalMemory | ConvertTo-Json)").Output(); err == nil {
			var result struct {
				TotalVisibleMemorySize float64 `json:"TotalVisibleMemorySize"`
				FreePhysicalMemory     float64 `json:"FreePhysicalMemory"`
			}
			if err := json.Unmarshal(out, &result); err == nil && result.TotalVisibleMemorySize > 0 {
				totalMB = result.TotalVisibleMemorySize / 1024 // KB -> MB
				freeMB := result.FreePhysicalMemory / 1024
				return totalMB, totalMB - freeMB
			}
		}

		// 降级到 wmic
		if out, err := exec.Command("wmic", "OS", "get", "TotalVisibleMemorySize,FreePhysicalMemory", "/format:list").Output(); err == nil {
			var total, free float64
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "TotalVisibleMemorySize=") {
					total, _ = strconv.ParseFloat(strings.TrimPrefix(line, "TotalVisibleMemorySize="), 64)
					total /= 1024 // KB -> MB
				} else if strings.HasPrefix(line, "FreePhysicalMemory=") {
					free, _ = strconv.ParseFloat(strings.TrimPrefix(line, "FreePhysicalMemory="), 64)
					free /= 1024 // KB -> MB
				}
			}
			if total > 0 {
				return total, total - free
			}
		}
	}

	// 降级到Go运行时信息
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Sys) / 1024 / 1024, float64(m.Alloc) / 1024 / 1024
}

// getDiskUsage 获取磁盘使用情况（GB）
func (s *MonitorService) getDiskUsage(path string) (totalGB, usedGB, freeGB float64) {
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		cmd := exec.Command("df", "-BG", path)
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 4 {
					parseGB := func(s string) float64 {
						s = strings.TrimSuffix(s, "G")
						v, _ := strconv.ParseFloat(s, 64)
						return v
					}
					totalGB = parseGB(fields[1])
					usedGB = parseGB(fields[2])
					freeGB = parseGB(fields[3])
					return
				}
			}
		}
	}

	if runtime.GOOS == "windows" {
		absPath, _ := filepath.Abs(path)
		if len(absPath) >= 2 {
			drive := absPath[:2] // 如 "C:"
			// 优先使用 PowerShell（wmic 已弃用）
			if out, err := exec.Command("powershell", "-NoProfile", "-Command",
				"$d = Get-CimInstance Win32_LogicalDisk -Filter \"DeviceID='"+drive+"'\"; [PSCustomObject]@{Free=$d.FreeSpace;Size=$d.Size} | ConvertTo-Json").Output(); err == nil {
				var result struct {
					Free float64 `json:"Free"`
					Size float64 `json:"Size"`
				}
				if err := json.Unmarshal(out, &result); err == nil && result.Size > 0 {
					freeGB = result.Free / 1024 / 1024 / 1024
					totalGB = result.Size / 1024 / 1024 / 1024
					usedGB = totalGB - freeGB
					return
				}
			}
			// 降级到 wmic
			cmd := exec.Command("wmic", "logicaldisk", "where", "DeviceID='"+drive+"'", "get", "FreeSpace,Size")
			output, err := cmd.Output()
			if err == nil {
				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				for _, line := range lines[1:] {
					fields := strings.Fields(strings.TrimSpace(line))
					if len(fields) >= 2 {
						free, _ := strconv.ParseFloat(fields[0], 64)
						total, _ := strconv.ParseFloat(fields[1], 64)
						freeGB = free / 1024 / 1024 / 1024
						totalGB = total / 1024 / 1024 / 1024
						usedGB = totalGB - freeGB
						return
					}
				}
			}
		}
	}

	return 0, 0, 0
}

// getDirSizeMB 获取目录大小（MB）
func (s *MonitorService) getDirSizeMB(dir string) float64 {
	var size int64
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return float64(size) / 1024 / 1024
}
