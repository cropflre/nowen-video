// Package service 番号刮削定时任务
// 功能：
//   - 每日定时扫描未刮削的番号媒体并自动补刮
//   - 支持失败重试窗口（7 天内失败的不重复尝试，避免浪费 request）
//   - 配置化：cron 表达式、并发度、是否启用
package service

import (
	"sync"
	"time"
)

// AdultSchedulerConfig 调度器运行时配置
type AdultSchedulerConfig struct {
	Enabled      bool   // 是否启用
	DailyHour    int    // 每日几点执行（0-23，默认 3 点）
	DailyMinute  int    // 分钟（默认 30）
	Concurrency  int    // 并发度
	OnlyUnscraped bool  // 只处理未成功刮削的
	Aggregated   bool   // 聚合模式
}

// AdultScheduler 定时调度器
type AdultScheduler struct {
	cfg         AdultSchedulerConfig
	batch       *AdultBatchService
	proxy       *AdultProxyManager
	scraper     *AdultScraperService

	mu          sync.Mutex
	running     bool
	lastRunAt   time.Time
	lastTaskID  string
	stopCh      chan struct{}
}

// NewAdultScheduler 创建调度器
func NewAdultScheduler(batch *AdultBatchService, proxy *AdultProxyManager, scraper *AdultScraperService) *AdultScheduler {
	return &AdultScheduler{
		batch:   batch,
		proxy:   proxy,
		scraper: scraper,
		cfg: AdultSchedulerConfig{
			Enabled:       false,
			DailyHour:     3,
			DailyMinute:   30,
			Concurrency:   2,
			OnlyUnscraped: true,
			Aggregated:    false,
		},
		stopCh: make(chan struct{}),
	}
}

// SetConfig 更新配置
func (s *AdultScheduler) SetConfig(cfg AdultSchedulerConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 参数校验
	if cfg.DailyHour < 0 || cfg.DailyHour > 23 {
		cfg.DailyHour = 3
	}
	if cfg.DailyMinute < 0 || cfg.DailyMinute > 59 {
		cfg.DailyMinute = 30
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 2
	}
	s.cfg = cfg
}

// Config 返回当前配置
func (s *AdultScheduler) Config() AdultSchedulerConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// LastRunAt 最近一次执行时间
func (s *AdultScheduler) LastRunAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRunAt
}

// LastTaskID 最近一次任务 ID
func (s *AdultScheduler) LastTaskID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastTaskID
}

// Start 启动调度器（后台 goroutine）
func (s *AdultScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	go s.loop()
}

// Stop 停止调度器
func (s *AdultScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	close(s.stopCh)
	s.running = false
}

// ==================== 主循环 ====================

func (s *AdultScheduler) loop() {
	// 每分钟检查一次是否到达执行时间
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// 提示性日志
	if s.scraper != nil && s.scraper.logger != nil {
		s.scraper.logger.Infof("番号批量刮削调度器已启动")
	}

	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.tick(now)
		}
	}
}

// tick 单次 tick 检查
func (s *AdultScheduler) tick(now time.Time) {
	s.mu.Lock()
	cfg := s.cfg
	last := s.lastRunAt
	s.mu.Unlock()

	if !cfg.Enabled {
		return
	}
	// 检查是否到达目标时间（每日 HH:MM）
	if now.Hour() != cfg.DailyHour || now.Minute() != cfg.DailyMinute {
		return
	}
	// 避免同一分钟重复触发
	if !last.IsZero() && now.Sub(last) < 2*time.Minute {
		return
	}

	s.mu.Lock()
	s.lastRunAt = now
	s.mu.Unlock()

	go s.runDailyTask()
}

// runDailyTask 执行每日任务
func (s *AdultScheduler) runDailyTask() {
	if s.scraper == nil || s.batch == nil {
		return
	}
	// 预先健康检查所有镜像
	if s.proxy != nil {
		total, healthy := s.proxy.HealthCheckAll()
		if s.scraper.logger != nil {
			s.scraper.logger.Infof("定时任务镜像预检: %d/%d 可用", healthy, total)
		}
	}

	cfg := s.Config()
	taskID, err := s.batch.Start(AdultBatchOptions{
		OnlyUnscraped: cfg.OnlyUnscraped,
		Concurrency:   cfg.Concurrency,
		Aggregated:    cfg.Aggregated,
	})
	s.mu.Lock()
	s.lastTaskID = taskID
	s.mu.Unlock()

	if err != nil {
		if s.scraper.logger != nil {
			s.scraper.logger.Warnf("定时刮削任务启动失败: %v", err)
		}
		return
	}
	if s.scraper.logger != nil {
		s.scraper.logger.Infof("定时刮削任务已启动: %s", taskID)
	}
}

// RunOnce 手动立即触发一次（不影响下次定时）
func (s *AdultScheduler) RunOnce() (string, error) {
	cfg := s.Config()
	if s.batch == nil {
		return "", fmtErrf("批量服务未初始化")
	}
	taskID, err := s.batch.Start(AdultBatchOptions{
		OnlyUnscraped: cfg.OnlyUnscraped,
		Concurrency:   cfg.Concurrency,
		Aggregated:    cfg.Aggregated,
	})
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.lastRunAt = time.Now()
	s.lastTaskID = taskID
	s.mu.Unlock()
	return taskID, nil
}
