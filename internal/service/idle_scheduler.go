package service

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== 潮汐调度器 IdleScheduler ====================
//
// 思路：
//   - 无人在线 → idle：放开 ResourceLimit 到 idle_cpu_percent（默认 80%），预处理全力跑
//   - 有人在线但未播放 → busy：限制为 busy_cpu_percent（默认 30%），小火慢炖
//   - 有人正在播放 → playing：根据策略执行：
//       * pause（默认）：暂停所有正在运行的预处理任务，0% 占用，保障播放流畅
//       * throttle：降到 playing_cpu_percent（默认 5%），继续但不影响播放
//
// 实现方式：完全外部驱动，不修改 worker 代码。通过 PreprocessService.ApplyTidalResourceLimit
// 覆盖运行时资源上限，原有的 dynamicLoadAdjuster 会自动响应。

// 系统设置键名
const (
	TidalSettingEnabled        = "idle_scheduler_enabled"      // 总开关，默认 true
	TidalSettingIdleCPU        = "tidal_idle_cpu_percent"      // 空闲时 CPU 上限（默认 80）
	TidalSettingBusyCPU        = "tidal_busy_cpu_percent"      // 在线时 CPU 上限（默认 30）
	TidalSettingPlayingCPU     = "tidal_playing_cpu_percent"   // 播放时 CPU 上限（throttle 模式用，默认 5）
	TidalSettingPlayingAction  = "tidal_playing_action"        // 播放时动作："pause" / "throttle"，默认 pause
	TidalSettingDebounceSec    = "tidal_debounce_sec"          // 防抖动秒数（默认 10）
)

// 潮汐默认值
const (
	DefaultTidalIdleCPU       = 80
	DefaultTidalBusyCPU       = 30
	DefaultTidalPlayingCPU    = 5
	DefaultTidalPlayingAction = "pause"
	DefaultTidalDebounceSec   = 10
)

// IdleScheduler 潮汐调度器
type IdleScheduler struct {
	preprocess *PreprocessService
	wsHub      *WSHub
	transcode  *TranscodeService
	settings   *repository.SystemSettingRepo
	logger     *zap.SugaredLogger

	// 当前模式（保护状态切换）
	mu          sync.Mutex
	currentMode string

	// 运行控制
	running int32 // atomic
	stopCh  chan struct{}

	// 当切换为 playing 时，记录被暂停的任务 ID，恢复时用
	pausedByTidal []string

	// 稳定性跟踪：pendingMode 需要连续出现 debounceSec 秒才会真正切换
	pendingMode    string
	pendingSince   time.Time
}

// NewIdleScheduler 创建潮汐调度器
func NewIdleScheduler(
	preprocess *PreprocessService,
	wsHub *WSHub,
	transcode *TranscodeService,
	settings *repository.SystemSettingRepo,
	logger *zap.SugaredLogger,
) *IdleScheduler {
	return &IdleScheduler{
		preprocess:  preprocess,
		wsHub:       wsHub,
		transcode:   transcode,
		settings:    settings,
		logger:      logger,
		currentMode: TidalModeIdle,
		stopCh:      make(chan struct{}),
	}
}

// Start 启动潮汐调度协程
func (s *IdleScheduler) Start() {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return // 已启动
	}
	go s.loop()
	s.logger.Info("潮汐调度器已启动")
}

// Stop 停止潮汐调度
func (s *IdleScheduler) Stop() {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		return
	}
	close(s.stopCh)
	// 退出前清除覆盖值，避免后续预处理一直被卡
	if s.preprocess != nil {
		s.preprocess.ApplyTidalResourceLimit(-1)
	}
	s.logger.Info("潮汐调度器已停止")
}

// loop 主循环：每 3 秒检测一次活跃状态
func (s *IdleScheduler) loop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

// tick 单次检测
func (s *IdleScheduler) tick() {
	// 先检查总开关
	if !s.isEnabled() {
		// 被关闭时，清除覆盖，恢复配置默认值
		if s.preprocess != nil {
			s.preprocess.ApplyTidalResourceLimit(-1)
			s.preprocess.SetTidalMode(TidalModeIdle)
		}
		return
	}

	// 探测当前应处于哪个模式
	target := s.detectMode()

	// 防抖动：连续 debounceSec 秒都是同一目标，才真正切换
	debounce := s.getIntSetting(TidalSettingDebounceSec, DefaultTidalDebounceSec)
	now := time.Now()

	s.mu.Lock()
	if target != s.currentMode {
		if s.pendingMode != target {
			s.pendingMode = target
			s.pendingSince = now
			s.mu.Unlock()
			return
		}
		if now.Sub(s.pendingSince) < time.Duration(debounce)*time.Second {
			s.mu.Unlock()
			return // 稳定性不够，继续观察
		}
		// 执行切换
		s.transit(s.currentMode, target)
		s.currentMode = target
		s.pendingMode = ""
	} else {
		s.pendingMode = ""
	}
	s.mu.Unlock()
}

// detectMode 根据当前系统状态判断应处于哪个潮汐模式
func (s *IdleScheduler) detectMode() string {
	// 优先级：playing > busy > idle
	if s.transcode != nil {
		if jobs := s.transcode.GetRunningJobs(); len(jobs) > 0 {
			return TidalModePlaying
		}
	}
	if s.wsHub != nil && s.wsHub.ClientCount() > 0 {
		return TidalModeBusy
	}
	return TidalModeIdle
}

// transit 执行模式切换的实际操作
func (s *IdleScheduler) transit(from, to string) {
	s.logger.Infof("潮汐调度: %s -> %s", from, to)

	if s.preprocess == nil {
		return
	}

	switch to {
	case TidalModeIdle:
		// 空闲模式：放开限制
		limit := s.getIntSetting(TidalSettingIdleCPU, DefaultTidalIdleCPU)
		s.preprocess.ApplyTidalResourceLimit(limit)
		s.preprocess.SetTidalMode(TidalModeIdle)
		// 恢复之前因潮汐被暂停的任务
		if len(s.pausedByTidal) > 0 {
			s.preprocess.ResumePausedTasks(s.pausedByTidal)
			s.pausedByTidal = nil
		}

	case TidalModeBusy:
		// 有用户但未播放：适度限制
		limit := s.getIntSetting(TidalSettingBusyCPU, DefaultTidalBusyCPU)
		s.preprocess.ApplyTidalResourceLimit(limit)
		s.preprocess.SetTidalMode(TidalModeBusy)
		// 若之前是 playing 被暂停的任务，此时可恢复（让路期已过）
		if len(s.pausedByTidal) > 0 {
			s.preprocess.ResumePausedTasks(s.pausedByTidal)
			s.pausedByTidal = nil
		}

	case TidalModePlaying:
		// 有人在播放：根据策略处理
		action := s.getStrSetting(TidalSettingPlayingAction, DefaultTidalPlayingAction)
		switch action {
		case "throttle":
			limit := s.getIntSetting(TidalSettingPlayingCPU, DefaultTidalPlayingCPU)
			s.preprocess.ApplyTidalResourceLimit(limit)
			s.preprocess.SetTidalMode(TidalModePlaying)
		default: // pause
			// 直接把资源限制拉到 1%（几乎不干活），同时暂停所有运行中任务
			s.preprocess.ApplyTidalResourceLimit(1)
			s.preprocess.SetTidalMode(TidalModePlaying)
			s.pausedByTidal = s.preprocess.PauseAllRunningTasks()
		}
	}
}

// ==================== 配置读取 ====================

// isEnabled 是否启用潮汐调度（默认 true）
func (s *IdleScheduler) isEnabled() bool {
	if s.settings == nil {
		return true
	}
	v, err := s.settings.Get(TidalSettingEnabled)
	if err != nil || v == "" {
		return true // 默认启用
	}
	return v == "true" || v == "1"
}

func (s *IdleScheduler) getIntSetting(key string, defVal int) int {
	if s.settings == nil {
		return defVal
	}
	v, err := s.settings.Get(key)
	if err != nil || v == "" {
		return defVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 || n > 100 {
		return defVal
	}
	return n
}

func (s *IdleScheduler) getStrSetting(key string, defVal string) string {
	if s.settings == nil {
		return defVal
	}
	v, err := s.settings.Get(key)
	if err != nil || v == "" {
		return defVal
	}
	return v
}

// ==================== 公开 API ====================

// TidalConfig 潮汐调度配置
type TidalConfig struct {
	Enabled        bool   `json:"enabled"`
	IdleCPU        int    `json:"idle_cpu_percent"`
	BusyCPU        int    `json:"busy_cpu_percent"`
	PlayingCPU     int    `json:"playing_cpu_percent"`
	PlayingAction  string `json:"playing_action"`
	DebounceSec    int    `json:"debounce_sec"`
}

// GetConfig 返回当前潮汐配置（含默认值合并）
func (s *IdleScheduler) GetConfig() TidalConfig {
	return TidalConfig{
		Enabled:       s.isEnabled(),
		IdleCPU:       s.getIntSetting(TidalSettingIdleCPU, DefaultTidalIdleCPU),
		BusyCPU:       s.getIntSetting(TidalSettingBusyCPU, DefaultTidalBusyCPU),
		PlayingCPU:    s.getIntSetting(TidalSettingPlayingCPU, DefaultTidalPlayingCPU),
		PlayingAction: s.getStrSetting(TidalSettingPlayingAction, DefaultTidalPlayingAction),
		DebounceSec:   s.getIntSetting(TidalSettingDebounceSec, DefaultTidalDebounceSec),
	}
}

// UpdateConfig 更新配置并持久化
func (s *IdleScheduler) UpdateConfig(cfg TidalConfig) error {
	if s.settings == nil {
		return nil
	}
	kvs := map[string]string{}
	kvs[TidalSettingEnabled] = boolStr(cfg.Enabled)
	if cfg.IdleCPU > 0 && cfg.IdleCPU <= 100 {
		kvs[TidalSettingIdleCPU] = strconv.Itoa(cfg.IdleCPU)
	}
	if cfg.BusyCPU > 0 && cfg.BusyCPU <= 100 {
		kvs[TidalSettingBusyCPU] = strconv.Itoa(cfg.BusyCPU)
	}
	if cfg.PlayingCPU > 0 && cfg.PlayingCPU <= 100 {
		kvs[TidalSettingPlayingCPU] = strconv.Itoa(cfg.PlayingCPU)
	}
	if cfg.PlayingAction == "pause" || cfg.PlayingAction == "throttle" {
		kvs[TidalSettingPlayingAction] = cfg.PlayingAction
	}
	if cfg.DebounceSec >= 1 && cfg.DebounceSec <= 300 {
		kvs[TidalSettingDebounceSec] = strconv.Itoa(cfg.DebounceSec)
	}
	return s.settings.SetMulti(kvs)
}

// GetStatus 返回当前调度运行状态
func (s *IdleScheduler) GetStatus() map[string]interface{} {
	s.mu.Lock()
	mode := s.currentMode
	pending := s.pendingMode
	pendingFor := time.Duration(0)
	if pending != "" {
		pendingFor = time.Since(s.pendingSince)
	}
	paused := len(s.pausedByTidal)
	s.mu.Unlock()

	result := map[string]interface{}{
		"running":          atomic.LoadInt32(&s.running) == 1,
		"current_mode":     mode,
		"pending_mode":     pending,
		"pending_for_ms":   pendingFor.Milliseconds(),
		"paused_task_cnt":  paused,
		"config":           s.GetConfig(),
	}
	// 接入当前活跃状态，方便前端展示
	online := 0
	playing := 0
	if s.wsHub != nil {
		online = s.wsHub.ClientCount()
	}
	if s.transcode != nil {
		playing = len(s.transcode.GetRunningJobs())
	}
	result["online_users"] = online
	result["playing_jobs"] = playing

	if s.preprocess != nil {
		result["preprocess"] = s.preprocess.GetTidalStatus()
	}
	return result
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
