package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// SchedulerService 定时任务调度服务
type SchedulerService struct {
	taskRepo            *repository.ScheduledTaskRepo
	libRepo             *repository.LibraryRepo
	libService          *LibraryService
	subtitlePreprocess  *SubtitlePreprocessService
	subtitleTargetLangs string // 字幕目标翻译语言配置
	wsHub               *WSHub
	logger              *zap.SugaredLogger
	stopCh              chan struct{}
	mu                  sync.RWMutex
	running             map[string]bool // 正在运行的任务ID
}

// NewSchedulerService 创建调度服务
func NewSchedulerService(
	taskRepo *repository.ScheduledTaskRepo,
	libRepo *repository.LibraryRepo,
	logger *zap.SugaredLogger,
) *SchedulerService {
	return &SchedulerService{
		taskRepo: taskRepo,
		libRepo:  libRepo,
		logger:   logger,
		stopCh:   make(chan struct{}),
		running:  make(map[string]bool),
	}
}

// SetLibraryService 延迟注入Library服务（避免循环依赖）
func (s *SchedulerService) SetLibraryService(libService *LibraryService) {
	s.libService = libService
}

// SetWSHub 设置WebSocket Hub
func (s *SchedulerService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// SetSubtitlePreprocessService 延迟注入字幕预处理服务
func (s *SchedulerService) SetSubtitlePreprocessService(svc *SubtitlePreprocessService, targetLangs string) {
	s.subtitlePreprocess = svc
	s.subtitleTargetLangs = targetLangs
}

// Start 启动调度器，每分钟检查一次到期任务
func (s *SchedulerService) Start() {
	s.logger.Info("定时任务调度器已启动")
	go s.loop()
}

// Stop 停止调度器
func (s *SchedulerService) Stop() {
	close(s.stopCh)
}

// loop 调度循环
func (s *SchedulerService) loop() {
	// 启动后先初始化所有任务的下次运行时间
	s.initNextRuns()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			s.logger.Info("定时任务调度器已停止")
			return
		case <-ticker.C:
			s.checkAndRun()
		}
	}
}

// initNextRuns 初始化所有启用任务的下次运行时间
func (s *SchedulerService) initNextRuns() {
	tasks, err := s.taskRepo.ListEnabled()
	if err != nil {
		s.logger.Warnf("加载定时任务失败: %v", err)
		return
	}

	now := time.Now()
	for _, task := range tasks {
		if task.NextRun == nil {
			next := s.calcNextRun(task.Schedule, now)
			task.NextRun = &next
			s.taskRepo.Update(&task)
		}
	}
}

// checkAndRun 检查并执行到期的任务
func (s *SchedulerService) checkAndRun() {
	tasks, err := s.taskRepo.ListEnabled()
	if err != nil {
		return
	}

	now := time.Now()
	for _, task := range tasks {
		if task.NextRun == nil {
			continue
		}
		if now.After(*task.NextRun) {
			// 防止重复执行
			s.mu.Lock()
			if s.running[task.ID] {
				s.mu.Unlock()
				continue
			}
			s.running[task.ID] = true
			s.mu.Unlock()

			go s.executeTask(task)
		}
	}
}

// executeTask 执行定时任务
func (s *SchedulerService) executeTask(task model.ScheduledTask) {
	defer func() {
		s.mu.Lock()
		delete(s.running, task.ID)
		s.mu.Unlock()
	}()

	s.logger.Infof("执行定时任务: %s (类型=%s)", task.Name, task.Type)

	// 更新状态为运行中
	now := time.Now()
	task.Status = "running"
	task.LastRun = &now
	s.taskRepo.Update(&task)

	var taskErr error

	switch task.Type {
	case "scan":
		taskErr = s.executeScanTask(&task)
	case "scrape":
		taskErr = s.executeScrapeTask(&task)
	case "cleanup":
		taskErr = s.executeCleanupTask(&task)
	case "subtitle_preprocess":
		taskErr = s.executeSubtitlePreprocessTask(&task)
	default:
		taskErr = fmt.Errorf("未知任务类型: %s", task.Type)
	}

	// 更新状态
	if taskErr != nil {
		task.Status = "error"
		task.LastError = taskErr.Error()
		s.logger.Warnf("定时任务执行失败: %s - %v", task.Name, taskErr)
	} else {
		task.Status = "idle"
		task.LastError = ""
		s.logger.Infof("定时任务执行完成: %s", task.Name)
	}

	// 计算下次运行时间
	nextRun := s.calcNextRun(task.Schedule, time.Now())
	task.NextRun = &nextRun
	s.taskRepo.Update(&task)

	// 广播通知
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent("scheduled_task_completed", map[string]interface{}{
			"task_id":   task.ID,
			"task_name": task.Name,
			"status":    task.Status,
			"error":     task.LastError,
		})
	}
}

// executeScanTask 执行扫描任务
func (s *SchedulerService) executeScanTask(task *model.ScheduledTask) error {
	if s.libService == nil {
		return fmt.Errorf("LibraryService未初始化")
	}

	if task.TargetID != "" {
		// 扫描指定媒体库
		return s.libService.Scan(task.TargetID)
	}

	// 扫描所有媒体库
	libs, err := s.libRepo.List()
	if err != nil {
		return err
	}
	for _, lib := range libs {
		if err := s.libService.Scan(lib.ID); err != nil {
			s.logger.Warnf("定时扫描媒体库 %s 失败: %v", lib.Name, err)
		}
	}
	return nil
}

// executeScrapeTask 执行刮削任务
func (s *SchedulerService) executeScrapeTask(task *model.ScheduledTask) error {
	if s.libService == nil {
		return fmt.Errorf("LibraryService未初始化")
	}
	// 扫描即包含刮削，直接复用
	return s.executeScanTask(task)
}

// executeCleanupTask 执行清理任务（清理过期缓存、日志等）
func (s *SchedulerService) executeCleanupTask(task *model.ScheduledTask) error {
	s.logger.Info("执行缓存清理任务")
	// 此处可扩展：清理转码缓存、过期的访问日志等
	return nil
}

// executeSubtitlePreprocessTask 执行字幕预处理定时任务
func (s *SchedulerService) executeSubtitlePreprocessTask(task *model.ScheduledTask) error {
	if s.subtitlePreprocess == nil {
		return fmt.Errorf("SubtitlePreprocessService 未初始化")
	}

	if task.TargetID != "" {
		// 处理指定媒体库
		count, err := s.subtitlePreprocess.SubmitLibrary(task.TargetID, s.getSubtitleTargetLangs(), false)
		if err != nil {
			return err
		}
		s.logger.Infof("定时字幕预处理: 媒体库 %s 提交 %d 个任务", task.TargetID, count)
		return nil
	}

	// 处理所有媒体库
	libs, err := s.libRepo.List()
	if err != nil {
		return err
	}
	totalCount := 0
	for _, lib := range libs {
		count, err := s.subtitlePreprocess.SubmitLibrary(lib.ID, s.getSubtitleTargetLangs(), false)
		if err != nil {
			s.logger.Warnf("定时字幕预处理媒体库 %s 失败: %v", lib.Name, err)
			continue
		}
		totalCount += count
	}
	if totalCount > 0 {
		s.logger.Infof("定时字幕预处理: 共提交 %d 个任务", totalCount)
	}
	return nil
}

// getSubtitleTargetLangs 获取字幕目标翻译语言列表
func (s *SchedulerService) getSubtitleTargetLangs() []string {
	if s.subtitleTargetLangs == "" {
		return nil
	}
	var langs []string
	for _, lang := range strings.Split(s.subtitleTargetLangs, ",") {
		lang = strings.TrimSpace(lang)
		if lang != "" {
			langs = append(langs, lang)
		}
	}
	return langs
}

// calcNextRun 根据调度表达式计算下次运行时间
// 支持简单的间隔表达式：@every 1h, @every 6h, @every 24h, @daily, @weekly
func (s *SchedulerService) calcNextRun(schedule string, from time.Time) time.Time {
	schedule = strings.TrimSpace(strings.ToLower(schedule))

	switch {
	case schedule == "@daily":
		// 每天凌晨2点
		next := time.Date(from.Year(), from.Month(), from.Day()+1, 2, 0, 0, 0, from.Location())
		return next

	case schedule == "@weekly":
		// 每周日凌晨3点
		daysUntilSunday := (7 - int(from.Weekday())) % 7
		if daysUntilSunday == 0 {
			daysUntilSunday = 7
		}
		next := time.Date(from.Year(), from.Month(), from.Day()+daysUntilSunday, 3, 0, 0, 0, from.Location())
		return next

	case strings.HasPrefix(schedule, "@every "):
		intervalStr := strings.TrimPrefix(schedule, "@every ")
		duration, err := time.ParseDuration(intervalStr)
		if err != nil {
			s.logger.Warnf("解析调度间隔失败: %s, 默认1小时", intervalStr)
			duration = time.Hour
		}
		return from.Add(duration)

	default:
		// 默认6小时后
		return from.Add(6 * time.Hour)
	}
}

// ==================== CRUD 操作 ====================

// CreateTask 创建定时任务
func (s *SchedulerService) CreateTask(name, taskType, schedule, targetID string) (*model.ScheduledTask, error) {
	now := time.Now()
	nextRun := s.calcNextRun(schedule, now)

	task := &model.ScheduledTask{
		Name:     name,
		Type:     taskType,
		Schedule: schedule,
		TargetID: targetID,
		Enabled:  true,
		NextRun:  &nextRun,
		Status:   "idle",
	}

	if err := s.taskRepo.Create(task); err != nil {
		return nil, err
	}
	return task, nil
}

// ListTasks 获取所有定时任务
func (s *SchedulerService) ListTasks() ([]model.ScheduledTask, error) {
	return s.taskRepo.List()
}

// UpdateTask 更新定时任务
func (s *SchedulerService) UpdateTask(id string, name, schedule string, enabled bool) error {
	task, err := s.taskRepo.FindByID(id)
	if err != nil {
		return err
	}
	task.Name = name
	task.Schedule = schedule
	task.Enabled = enabled

	if enabled {
		nextRun := s.calcNextRun(schedule, time.Now())
		task.NextRun = &nextRun
	}

	return s.taskRepo.Update(task)
}

// DeleteTask 删除定时任务
func (s *SchedulerService) DeleteTask(id string) error {
	return s.taskRepo.Delete(id)
}

// RunTaskNow 立即运行某个任务
func (s *SchedulerService) RunTaskNow(id string) error {
	task, err := s.taskRepo.FindByID(id)
	if err != nil {
		return err
	}

	s.mu.RLock()
	if s.running[task.ID] {
		s.mu.RUnlock()
		return fmt.Errorf("任务正在运行中")
	}
	s.mu.RUnlock()

	go s.executeTask(*task)
	return nil
}
