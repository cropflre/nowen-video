package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// ==================== GPU 安全阈值配置 ====================

// GPUThresholdConfig GPU 安全阈值配置
type GPUThresholdConfig struct {
	// GPU 使用率安全上限（百分比，默认 85）
	// 超过此值将暂停新的 GPU 任务并降级为 CPU 处理
	UtilizationThreshold int `json:"utilization_threshold" mapstructure:"gpu_utilization_threshold"`

	// GPU 温度安全上限（摄氏度，默认 80）
	// 超过此值将立即暂停所有 GPU 任务
	TemperatureThreshold int `json:"temperature_threshold" mapstructure:"gpu_temperature_threshold"`

	// 冷却恢复阈值（百分比，默认 60）
	// GPU 使用率降至此值以下时恢复 GPU 加速
	RecoveryThreshold int `json:"recovery_threshold" mapstructure:"gpu_recovery_threshold"`

	// 温度冷却恢复阈值（摄氏度，默认 70）
	// GPU 温度降至此值以下时恢复 GPU 加速
	TemperatureRecovery int `json:"temperature_recovery" mapstructure:"gpu_temperature_recovery"`

	// 监控采样间隔（秒，默认 5）
	MonitorInterval int `json:"monitor_interval" mapstructure:"gpu_monitor_interval"`

	// 是否启用 GPU 安全保护（默认 true）
	Enabled bool `json:"enabled" mapstructure:"gpu_safety_enabled"`
}

// DefaultGPUThresholdConfig 返回默认的 GPU 安全阈值配置
func DefaultGPUThresholdConfig() GPUThresholdConfig {
	return GPUThresholdConfig{
		UtilizationThreshold: 85,
		TemperatureThreshold: 80,
		RecoveryThreshold:    60,
		TemperatureRecovery:  70,
		MonitorInterval:      5,
		Enabled:              true,
	}
}

// ==================== GPU 实时状态 ====================

// GPUMetrics GPU 实时指标
type GPUMetrics struct {
	Utilization    float64 `json:"utilization"`      // GPU 使用率（%）
	MemoryUsed     int     `json:"memory_used"`      // 已用显存（MB）
	MemoryTotal    int     `json:"memory_total"`     // 总显存（MB）
	MemoryPercent  float64 `json:"memory_percent"`   // 显存使用率（%）
	Temperature    int     `json:"temperature"`      // GPU 温度（°C）
	PowerDraw      float64 `json:"power_draw"`       // 功耗（W）
	PowerLimit     float64 `json:"power_limit"`      // 功耗上限（W）
	EncoderUtil    float64 `json:"encoder_util"`     // 编码器使用率（%）
	DecoderUtil    float64 `json:"decoder_util"`     // 解码器使用率（%）
	FanSpeed       int     `json:"fan_speed"`        // 风扇转速（%）
	GPUName        string  `json:"gpu_name"`         // GPU 名称
	DriverVersion  string  `json:"driver_version"`   // 驱动版本
	Available      bool    `json:"available"`        // GPU 是否可用
	LastUpdateTime int64   `json:"last_update_time"` // 最后更新时间戳
}

// GPUSafetyStatus GPU 安全状态
type GPUSafetyStatus struct {
	// 当前是否处于降级模式（GPU 过载，任务降级为 CPU）
	Degraded bool `json:"degraded"`
	// 降级原因
	DegradeReason string `json:"degrade_reason,omitempty"`
	// 降级开始时间
	DegradeSince int64 `json:"degrade_since,omitempty"`
	// 被降级的任务数量
	DegradedTaskCount int32 `json:"degraded_task_count"`
	// 被暂停等待 GPU 的任务数量
	PendingGPUTasks int32 `json:"pending_gpu_tasks"`
	// 当前 GPU 指标
	Metrics GPUMetrics `json:"metrics"`
	// 安全阈值配置
	Thresholds GPUThresholdConfig `json:"thresholds"`
}

// ==================== GPU 监控服务 ====================

// GPUMonitor GPU 资源安全监控服务
// 实时监控 GPU 使用率和温度，在超过安全阈值时自动降级任务为 CPU 处理，
// 当 GPU 资源恢复到安全水平后自动恢复 GPU 加速。
type GPUMonitor struct {
	logger        *zap.SugaredLogger
	hwAccel       string // 硬件加速类型：nvenc / qsv / vaapi / none
	config        GPUThresholdConfig
	mu            sync.RWMutex
	metrics       GPUMetrics
	degraded      int32 // 原子操作：0=正常, 1=降级
	degradeReason string
	degradeSince  int64 // 降级开始时间戳
	degradedTasks int32 // 被降级为 CPU 的任务计数
	pendingTasks  int32 // 等待 GPU 恢复的任务计数
	stopCh        chan struct{}
	wsHub         *WSHub
	nvidiaSmiPath string // nvidia-smi 可执行文件路径（缓存）

	// 回调：当 GPU 状态变化时通知订阅者
	onDegradeCallbacks []func(degraded bool, reason string)
	callbackMu         sync.RWMutex
}

// NewGPUMonitor 创建 GPU 监控服务
func NewGPUMonitor(hwAccel string, config GPUThresholdConfig, logger *zap.SugaredLogger) *GPUMonitor {
	m := &GPUMonitor{
		logger:  logger,
		hwAccel: hwAccel,
		config:  config,
		stopCh:  make(chan struct{}),
	}

	// 预先查找 nvidia-smi 路径
	if hwAccel == "nvenc" {
		m.nvidiaSmiPath = m.findNvidiaSmi()
	}

	return m
}

// SetWSHub 设置 WebSocket Hub
func (m *GPUMonitor) SetWSHub(hub *WSHub) {
	m.wsHub = hub
}

// Start 启动 GPU 监控循环
func (m *GPUMonitor) Start() {
	if !m.config.Enabled || m.hwAccel == "none" {
		m.logger.Info("GPU 安全监控未启用（GPU 不可用或已禁用）")
		return
	}

	interval := m.config.MonitorInterval
	if interval < 2 {
		interval = 2
	}

	m.logger.Infof("GPU 安全监控已启动: 采样间隔=%ds, 使用率阈值=%d%%, 温度阈值=%d°C, 恢复阈值=%d%%",
		interval, m.config.UtilizationThreshold, m.config.TemperatureThreshold, m.config.RecoveryThreshold)

	go m.monitorLoop(time.Duration(interval) * time.Second)
}

// Stop 停止 GPU 监控
func (m *GPUMonitor) Stop() {
	close(m.stopCh)
}

// OnDegradeChange 注册降级状态变化回调
func (m *GPUMonitor) OnDegradeChange(callback func(degraded bool, reason string)) {
	m.callbackMu.Lock()
	defer m.callbackMu.Unlock()
	m.onDegradeCallbacks = append(m.onDegradeCallbacks, callback)
}

// IsDegraded 检查当前是否处于降级模式（线程安全）
// 当 GPU 过载时返回 true，调用方应使用 CPU 编码替代
func (m *GPUMonitor) IsDegraded() bool {
	return atomic.LoadInt32(&m.degraded) == 1
}

// ShouldUseGPU 判断当前任务是否应该使用 GPU
// 返回 (是否使用GPU, 实际使用的加速模式)
func (m *GPUMonitor) ShouldUseGPU(requestedAccel string) (bool, string) {
	// 如果请求的不是 GPU 加速，直接返回
	if requestedAccel == "none" || requestedAccel == "" {
		return false, "none"
	}

	// 如果监控未启用，始终允许 GPU
	if !m.config.Enabled {
		return true, requestedAccel
	}

	// 检查是否处于降级模式
	if m.IsDegraded() {
		atomic.AddInt32(&m.degradedTasks, 1)
		m.logger.Warnf("GPU 过载保护: 任务降级为 CPU 编码 (原因: %s)", m.getDegradeReason())
		return false, "none"
	}

	return true, requestedAccel
}

// WaitForGPU 等待 GPU 资源可用（阻塞，带超时）
// 当 GPU 过载时，任务可以选择等待 GPU 恢复而非降级
// 返回 true 表示 GPU 已恢复可用，false 表示超时
func (m *GPUMonitor) WaitForGPU(timeout time.Duration) bool {
	if !m.IsDegraded() {
		return true
	}

	atomic.AddInt32(&m.pendingTasks, 1)
	defer atomic.AddInt32(&m.pendingTasks, -1)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return false // 超时
		case <-ticker.C:
			if !m.IsDegraded() {
				return true // GPU 已恢复
			}
		case <-m.stopCh:
			return false // 服务停止
		}
	}
}

// GetMetrics 获取当前 GPU 指标（线程安全）
func (m *GPUMonitor) GetMetrics() GPUMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

// GetSafetyStatus 获取完整的 GPU 安全状态
func (m *GPUMonitor) GetSafetyStatus() GPUSafetyStatus {
	m.mu.RLock()
	metrics := m.metrics
	m.mu.RUnlock()

	return GPUSafetyStatus{
		Degraded:          m.IsDegraded(),
		DegradeReason:     m.getDegradeReason(),
		DegradeSince:      atomic.LoadInt64(&m.degradeSince),
		DegradedTaskCount: atomic.LoadInt32(&m.degradedTasks),
		PendingGPUTasks:   atomic.LoadInt32(&m.pendingTasks),
		Metrics:           metrics,
		Thresholds:        m.config,
	}
}

// UpdateConfig 动态更新阈值配置
func (m *GPUMonitor) UpdateConfig(config GPUThresholdConfig) {
	m.mu.Lock()
	m.config = config
	m.mu.Unlock()
	m.logger.Infof("GPU 安全阈值已更新: 使用率=%d%%, 温度=%d°C, 恢复=%d%%",
		config.UtilizationThreshold, config.TemperatureThreshold, config.RecoveryThreshold)
}

// ==================== 内部方法 ====================

// monitorLoop GPU 监控主循环
func (m *GPUMonitor) monitorLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			m.logger.Info("GPU 安全监控已停止")
			return
		case <-ticker.C:
			m.sampleGPU()
			m.evaluateSafety()
		}
	}
}

// sampleGPU 采样 GPU 指标
func (m *GPUMonitor) sampleGPU() {
	var metrics GPUMetrics

	switch m.hwAccel {
	case "nvenc":
		metrics = m.sampleNvidia()
	case "qsv":
		metrics = m.sampleIntelQSV()
	case "vaapi":
		metrics = m.sampleVAAPI()
	default:
		return
	}

	metrics.LastUpdateTime = time.Now().Unix()

	m.mu.Lock()
	m.metrics = metrics
	m.mu.Unlock()
}

// sampleNvidia 通过 nvidia-smi 采样 NVIDIA GPU 指标
func (m *GPUMonitor) sampleNvidia() GPUMetrics {
	metrics := GPUMetrics{Available: false}

	if m.nvidiaSmiPath == "" {
		return metrics
	}

	// 查询 GPU 使用率、显存、温度、功耗、编码器/解码器使用率、风扇转速
	cmd := exec.Command(m.nvidiaSmiPath,
		"--query-gpu=utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw,power.limit,utilization.encoder,utilization.decoder,fan.speed,name,driver_version",
		"--format=csv,noheader,nounits",
	)

	output, err := cmd.Output()
	if err != nil {
		m.logger.Debugf("nvidia-smi 查询失败: %v", err)
		return metrics
	}

	line := strings.TrimSpace(string(output))
	// 处理多 GPU 情况：只取第一个 GPU
	if idx := strings.Index(line, "\n"); idx > 0 {
		line = line[:idx]
	}

	parts := strings.Split(line, ", ")
	if len(parts) < 9 {
		m.logger.Debugf("nvidia-smi 输出格式异常: %s", line)
		return metrics
	}

	metrics.Available = true
	metrics.Utilization, _ = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	metrics.MemoryUsed, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	metrics.MemoryTotal, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
	metrics.Temperature, _ = strconv.Atoi(strings.TrimSpace(parts[3]))
	metrics.PowerDraw, _ = strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
	metrics.PowerLimit, _ = strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)
	metrics.EncoderUtil, _ = strconv.ParseFloat(strings.TrimSpace(parts[6]), 64)
	metrics.DecoderUtil, _ = strconv.ParseFloat(strings.TrimSpace(parts[7]), 64)
	metrics.FanSpeed, _ = strconv.Atoi(strings.TrimSpace(parts[8]))

	if len(parts) >= 10 {
		metrics.GPUName = strings.TrimSpace(parts[9])
	}
	if len(parts) >= 11 {
		metrics.DriverVersion = strings.TrimSpace(parts[10])
	}

	if metrics.MemoryTotal > 0 {
		metrics.MemoryPercent = float64(metrics.MemoryUsed) / float64(metrics.MemoryTotal) * 100
	}

	return metrics
}

// sampleIntelQSV 采样 Intel QSV GPU 指标（Linux）
func (m *GPUMonitor) sampleIntelQSV() GPUMetrics {
	metrics := GPUMetrics{Available: false, GPUName: "Intel QSV"}

	if runtime.GOOS != "linux" {
		return metrics
	}

	// 尝试通过 intel_gpu_top 获取使用率
	cmd := exec.Command("intel_gpu_top", "-l", "-s", "100")
	output, err := cmd.Output()
	if err != nil {
		// intel_gpu_top 可能未安装，标记为可用但无详细指标
		metrics.Available = true
		return metrics
	}

	// 解析 intel_gpu_top 输出
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Render/3D") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Render/3D" && i+1 < len(fields) {
					metrics.Utilization, _ = strconv.ParseFloat(fields[i+1], 64)
					break
				}
			}
		}
	}

	metrics.Available = true
	return metrics
}

// sampleVAAPI 采样 VAAPI GPU 指标（Linux）
func (m *GPUMonitor) sampleVAAPI() GPUMetrics {
	metrics := GPUMetrics{Available: false, GPUName: "VA-API"}

	if runtime.GOOS != "linux" {
		return metrics
	}

	// VAAPI 没有标准的使用率查询工具，标记为可用
	if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
		metrics.Available = true
	}

	return metrics
}

// evaluateSafety 评估 GPU 安全状态，决定是否需要降级或恢复
func (m *GPUMonitor) evaluateSafety() {
	m.mu.RLock()
	metrics := m.metrics
	config := m.config
	m.mu.RUnlock()

	if !metrics.Available {
		return
	}

	wasDegraded := m.IsDegraded()

	// 检查是否需要降级
	if !wasDegraded {
		reason := ""

		// 检查 GPU 使用率
		if metrics.Utilization >= float64(config.UtilizationThreshold) {
			reason = fmt.Sprintf("GPU 使用率 %.1f%% 超过阈值 %d%%", metrics.Utilization, config.UtilizationThreshold)
		}

		// 检查温度（优先级更高）
		if metrics.Temperature >= config.TemperatureThreshold {
			reason = fmt.Sprintf("GPU 温度 %d°C 超过安全阈值 %d°C", metrics.Temperature, config.TemperatureThreshold)
		}

		// 检查编码器使用率（NVENC 编码器有并发限制）
		if metrics.EncoderUtil >= 95 {
			reason = fmt.Sprintf("GPU 编码器使用率 %.1f%% 接近饱和", metrics.EncoderUtil)
		}

		if reason != "" {
			m.setDegraded(true, reason)
			m.logger.Warnf("⚠️ GPU 安全保护触发: %s → 新任务将降级为 CPU 编码", reason)

			// 广播降级事件
			if m.wsHub != nil {
				m.wsHub.BroadcastEvent("gpu_safety_degraded", map[string]interface{}{
					"reason":      reason,
					"utilization": metrics.Utilization,
					"temperature": metrics.Temperature,
					"encoder":     metrics.EncoderUtil,
				})
			}
		}
	}

	// 检查是否可以恢复
	if wasDegraded {
		canRecover := true
		recoverMsg := ""

		// 使用率需降至恢复阈值以下
		if metrics.Utilization > float64(config.RecoveryThreshold) {
			canRecover = false
		}

		// 温度需降至恢复阈值以下
		if metrics.Temperature > config.TemperatureRecovery {
			canRecover = false
		}

		// 编码器使用率需降至安全水平
		if metrics.EncoderUtil > 70 {
			canRecover = false
		}

		if canRecover {
			recoverMsg = fmt.Sprintf("GPU 已恢复安全水平: 使用率 %.1f%%, 温度 %d°C", metrics.Utilization, metrics.Temperature)
			m.setDegraded(false, "")
			m.logger.Infof("✅ GPU 安全保护解除: %s → 恢复 GPU 硬件加速", recoverMsg)

			// 广播恢复事件
			if m.wsHub != nil {
				m.wsHub.BroadcastEvent("gpu_safety_recovered", map[string]interface{}{
					"message":     recoverMsg,
					"utilization": metrics.Utilization,
					"temperature": metrics.Temperature,
				})
			}
		}
	}
}

// setDegraded 设置降级状态
func (m *GPUMonitor) setDegraded(degraded bool, reason string) {
	if degraded {
		atomic.StoreInt32(&m.degraded, 1)
		atomic.StoreInt64(&m.degradeSince, time.Now().Unix())
		m.mu.Lock()
		m.degradeReason = reason
		m.mu.Unlock()
	} else {
		atomic.StoreInt32(&m.degraded, 0)
		atomic.StoreInt64(&m.degradeSince, 0)
		atomic.StoreInt32(&m.degradedTasks, 0)
		m.mu.Lock()
		m.degradeReason = ""
		m.mu.Unlock()
	}

	// 通知回调
	m.callbackMu.RLock()
	callbacks := m.onDegradeCallbacks
	m.callbackMu.RUnlock()

	for _, cb := range callbacks {
		go cb(degraded, reason)
	}
}

// getDegradeReason 获取降级原因
func (m *GPUMonitor) getDegradeReason() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.degradeReason
}

// findNvidiaSmi 查找 nvidia-smi 可执行文件路径
func (m *GPUMonitor) findNvidiaSmi() string {
	// 优先通过 PATH 查找
	if path, err := exec.LookPath("nvidia-smi"); err == nil {
		return path
	}

	// Windows 下尝试常见路径
	if runtime.GOOS == "windows" {
		commonPaths := []string{
			filepath.Join(os.Getenv("SystemRoot"), "System32", "nvidia-smi.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe"),
		}
		for _, p := range commonPaths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	// 最后尝试直接使用命令名（依赖 CreateProcess 搜索）
	if runtime.GOOS == "windows" {
		if _, err := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader").Output(); err == nil {
			return "nvidia-smi"
		}
	}

	return ""
}
