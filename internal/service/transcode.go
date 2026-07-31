package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	transcodeexecutor "github.com/nowen-video/nowen-video/internal/transcode/executor"
	transcodegovernor "github.com/nowen-video/nowen-video/internal/transcode/governor"
	transcoderuntime "github.com/nowen-video/nowen-video/internal/transcode/runtime"
	"go.uber.org/zap"
)

const EventTranscodeCancelled = "transcode_cancelled"

type TranscodeService struct {
	repo          *repository.TranscodeRepo
	executionRepo *repository.TranscodeExecutionRepo
	cfg           *config.Config
	logger        *zap.SugaredLogger

	jobs        *transcodePriorityQueue
	workerCount int
	submitMu    sync.Mutex
	mu          sync.RWMutex
	running     map[string]*TranscodeJob

	instanceID             string
	leaseDuration          time.Duration
	leaseHeartbeatInterval time.Duration
	leaseRecoveryInterval  time.Duration

	hwAccel   string
	hwAccelMu sync.Once
	wsHub     *WSHub

	executionRuntime *transcoderuntime.Runtime

	throttleSuspendSeconds atomic.Uint64
	throttleSuspendCount   atomic.Uint64

	diskUsageMu    sync.RWMutex
	diskUsageBytes int64
	diskUsageAt    time.Time
	diskUsageTTL   time.Duration
}

type TranscodeJob struct {
	Task         *model.TranscodeTask
	ExecutionJob *model.TranscodeJobRecord
	Media        *model.Media
	Quality      string

	ctx        context.Context
	cancel     context.CancelFunc
	cancelOnce sync.Once
	taskMu     sync.Mutex

	workerID   string
	leaseToken string
	leaseDone  chan struct{}
	leaseStop  sync.Once

	processMu sync.RWMutex
	process   *os.Process

	playbackPos          atomic.Uint64
	transcodedPos        atomic.Uint64
	lastDBProgress       atomic.Uint64
	lastAttemptHeartbeat atomic.Int64
	suspended            atomic.Int32
	startOffset          float64

	throttleDone    chan struct{}
	throttleStop    sync.Once
	throttleStarted sync.Once
}

func (j *TranscodeJob) RequestCancel() {
	if j == nil {
		return
	}
	j.cancelOnce.Do(func() {
		if j.cancel != nil {
			j.cancel()
		}
	})
}

func (j *TranscodeJob) CancellationRequested() bool {
	return j == nil || j.ctx == nil || j.ctx.Err() != nil
}

func (j *TranscodeJob) setProcess(process *os.Process) {
	j.processMu.Lock()
	j.process = process
	j.processMu.Unlock()
}

func (j *TranscodeJob) currentProcess() *os.Process {
	j.processMu.RLock()
	defer j.processMu.RUnlock()
	return j.process
}

func (j *TranscodeJob) stopThrottle() {
	j.throttleStop.Do(func() { close(j.throttleDone) })
}

func (j *TranscodeJob) stopLeaseHeartbeat() {
	j.leaseStop.Do(func() { close(j.leaseDone) })
}

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
	if repo == nil || repo.DB() == nil {
		panic("transcode repository is required")
	}
	if err := model.AutoMigrateTranscodeExecution(repo.DB()); err != nil {
		panic(fmt.Sprintf("migrate transcode execution schema: %v", err))
	}
	executionRepo := repository.NewTranscodeExecutionRepo(repo.DB())
	service := &TranscodeService{
		repo:                   repo,
		executionRepo:          executionRepo,
		cfg:                    cfg,
		logger:                 logger,
		jobs:                   newTranscodePriorityQueue(executionRepo, repo, 100, logger),
		running:                make(map[string]*TranscodeJob),
		instanceID:             newTranscodeInstanceID(),
		leaseDuration:          defaultTranscodeLeaseDuration,
		leaseHeartbeatInterval: defaultTranscodeLeaseHeartbeatInterval,
		leaseRecoveryInterval:  defaultTranscodeLeaseRecoveryInterval,
		executionRuntime:       transcoderuntime.Default(),
		diskUsageTTL:           30 * time.Second,
	}

	service.hwAccelMu.Do(func() {
		service.hwAccel = service.detectHWAccel()
		logger.Infof("硬件加速模式: %s", service.hwAccel)
	})

	service.recoverPendingTasks()

	service.workerCount = 1
	if service.hwAccel != "" && service.hwAccel != ffmpeg.HWAccelNone {
		service.workerCount = 2
	}
	for workerIndex := 0; workerIndex < service.workerCount; workerIndex++ {
		go service.worker(workerIndex)
	}
	go service.leaseRecoveryLoop()
	return service
}

func (s *TranscodeService) SetWSHub(hub *WSHub)                         { s.wsHub = hub }
func (s *TranscodeService) GetHWAccelInfo() string                      { return s.hwAccel }
func (s *TranscodeService) ExecutionRuntime() *transcoderuntime.Runtime { return s.executionRuntime }

func (s *TranscodeService) detectHWAccel() string {
	return ffmpeg.DetectHWAccel(s.cfg, s.logger)
}

func (s *TranscodeService) StartTranscode(media *model.Media, quality string) (*model.TranscodeTask, error) {
	return s.startTranscodeInternal(media, quality, 0)
}

func (s *TranscodeService) StartABRTranscode(media *model.Media, qualities []string) ([]*model.TranscodeTask, error) {
	if media == nil {
		return nil, fmt.Errorf("media 不能为空")
	}
	startupQuality := ""
	for _, quality := range qualities {
		if _, ok := qualityPresets[quality]; ok {
			startupQuality = quality
			break
		}
	}
	if startupQuality == "" {
		return nil, fmt.Errorf("没有有效的 ABR 档位")
	}
	task, err := s.startTranscodeInternal(media, startupQuality, 0)
	if err != nil {
		return nil, err
	}
	s.logger.Infof("ABR 渐进预热已提交: media=%s startup=%s deferred=%d", media.ID, startupQuality, len(qualities)-1)
	return []*model.TranscodeTask{task}, nil
}

func (s *TranscodeService) StartTranscodeWithStart(media *model.Media, quality string, startOffset float64) (*model.TranscodeTask, error) {
	return s.startTranscodeInternal(media, quality, startOffset)
}

func (s *TranscodeService) startTranscodeInternal(media *model.Media, quality string, startOffset float64) (*model.TranscodeTask, error) {
	return s.startTranscodeWithPriority(media, quality, startOffset, TranscodePriorityInteractive)
}

func (s *TranscodeService) startTranscodeWithPriority(media *model.Media, quality string, startOffset float64, priority int) (*model.TranscodeTask, error) {
	if media == nil || strings.TrimSpace(media.ID) == "" {
		return nil, fmt.Errorf("媒体不能为空")
	}
	if _, ok := qualityPresets[quality]; !ok {
		return nil, fmt.Errorf("未知转码档位: %s", quality)
	}
	if priority <= 0 {
		priority = TranscodePriorityBackground
	}

	s.submitMu.Lock()
	defer s.submitMu.Unlock()

	if task, err := s.findActiveExecutionTask(media, quality, startOffset, priority); err == nil {
		return task, nil
	}
	if startOffset == 0 {
		if task, err := s.repo.FindByMediaAndQuality(media.ID, quality); err == nil {
			m3u8Path := filepath.Join(s.GetOutputDir(media.ID, quality), "stream.m3u8")
			if _, statErr := os.Stat(m3u8Path); statErr == nil {
				return task, nil
			}
		}
		s.mu.RLock()
		for _, job := range s.running {
			if job.Media.ID == media.ID && job.Quality == quality && job.startOffset == 0 {
				s.mu.RUnlock()
				return job.Task, nil
			}
		}
		s.mu.RUnlock()
	}

	if !s.jobs.CanAccept() {
		return nil, fmt.Errorf("转码队列已满或服务正在关闭")
	}

	outputDir := s.GetOutputDir(media.ID, quality)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建转码目录失败: %w", err)
	}
	task := &model.TranscodeTask{
		MediaID:    media.ID,
		Quality:    quality,
		Status:     "pending",
		OutputDir:  outputDir,
		MediaTitle: media.DescriptiveTitle(),
		Priority:   priority,
	}
	if err := s.repo.Create(task); err != nil {
		return nil, err
	}
	executionJob, err := s.createExecutionJob(media, quality, startOffset, task.ID, priority)
	if err != nil {
		_ = s.repo.DeleteByID(task.ID)
		return nil, fmt.Errorf("创建持久化转码 Job 失败: %w", err)
	}

	wakeJob := &TranscodeJob{Task: task, ExecutionJob: executionJob}
	if s.jobs.Push(wakeJob) {
		return task, nil
	}

	now := time.Now()
	if completed, completeErr := s.executionRepo.CompleteQueuedJob(executionJob.ID, "failed", now); completeErr != nil {
		s.logger.Warnf("回滚未进入持久队列的转码 Job 失败 job=%s: %v", executionJob.ID, completeErr)
	} else if completed {
		task.Status = "failed"
		task.Error = "转码队列已满或服务正在关闭"
		task.CompletedAt = &now
		_ = s.repo.Update(task)
	}
	return nil, fmt.Errorf("转码队列已满或服务正在关闭")
}

func (s *TranscodeService) WaitForFirstSegment(ctx context.Context, mediaID, quality string) error {
	m3u8Path := filepath.Join(s.GetOutputDir(mediaID, quality), "stream.m3u8")
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(m3u8Path); err == nil {
			content, readErr := os.ReadFile(m3u8Path)
			if readErr == nil && strings.Contains(string(content), ".ts") {
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

func (s *TranscodeService) worker(index int) {
	workerID := fmt.Sprintf("%s/worker-%d", s.instanceID, index)
	s.logger.Infof("转码 Worker 启动: %s", workerID)
	for {
		job, ok := s.jobs.Pop(workerID, s.leaseDuration)
		if !ok {
			return
		}
		s.mu.Lock()
		s.running[job.Task.ID] = job
		s.mu.Unlock()
		s.processJob(job, workerID)
	}
}

func (s *TranscodeService) processJob(job *TranscodeJob, workerID string) {
	defer func() {
		job.stopLeaseHeartbeat()
		job.stopThrottle()
		job.setProcess(nil)
		s.mu.Lock()
		delete(s.running, job.Task.ID)
		s.mu.Unlock()
	}()

	if job.CancellationRequested() {
		s.finalizeCancelled(job)
		return
	}
	go s.leaseHeartbeatLoop(job)

	if !s.markJobRunning(job) {
		s.finalizeCancelled(job)
		return
	}
	s.logger.Infof("开始转码: %s (%s), worker=%s priority=%d", job.Media.Title, job.Quality, workerID, job.ExecutionJob.Priority)
	s.broadcastTranscodeEvent(EventTranscodeStarted, &TranscodeProgressData{
		TaskID:  job.Task.ID,
		MediaID: job.Media.ID,
		Title:   job.Media.Title,
		Quality: job.Quality,
		Message: fmt.Sprintf("开始转码: %s (%s)", job.Media.Title, job.Quality),
	})

	nextAttempt, err := s.executionRepo.NextAttemptNumber(job.ExecutionJob.ID)
	if err != nil {
		s.finalizeFailed(job, transcodeexecutor.Result{Err: fmt.Errorf("读取下一 Attempt 编号失败: %w", err)})
		return
	}
	if nextAttempt > 1 {
		s.cleanAttemptOutput(job.Task.OutputDir)
	}

	backend := s.hwAccel
	if backend == "" {
		backend = ffmpeg.HWAccelNone
	}
	result := s.runAttempt(job, backend, nextAttempt)
	if result.Cancelled || result.TimedOut || job.CancellationRequested() {
		s.finalizeCancelled(job)
		return
	}

	if result.Err != nil && backend != ffmpeg.HWAccelNone {
		s.logger.Warnf("硬件转码失败 backend=%s media=%s: %v，切换独立软件 Attempt", backend, job.Media.ID, result.Err)
		s.cleanAttemptOutput(job.Task.OutputDir)
		result = s.runAttempt(job, ffmpeg.HWAccelNone, nextAttempt+1)
		if result.Cancelled || result.TimedOut || job.CancellationRequested() {
			s.finalizeCancelled(job)
			return
		}
	}
	if result.Err != nil {
		s.finalizeFailed(job, result)
		return
	}
	s.finalizeCompleted(job)
}

func (s *TranscodeService) runAttempt(job *TranscodeJob, backend string, attemptNumber int) transcodeexecutor.Result {
	kind := transcodegovernor.KindSoftwareTranscode
	if backend != "" && backend != ffmpeg.HWAccelNone {
		kind = transcodegovernor.KindHardwareTranscode
	}

	args := s.buildFFmpegArgsForBackend(job.Media, job.Media.FilePath, job.Task.OutputDir, job.Quality, job.startOffset, backend)
	attempt, err := s.createAttempt(job, attemptNumber, backend, args)
	if err != nil {
		return transcodeexecutor.Result{Err: fmt.Errorf("创建 Attempt 记录失败: %w", err)}
	}
	s.logger.Debugf("FFmpeg attempt=%d backend=%s command=%s %s", attemptNumber, backend, s.cfg.App.FFmpegPath, strings.Join(redactFFmpegArgs(args), " "))
	result := s.executionRuntime.Run(job.ctx, kind, transcodeexecutor.Command{
		Path:       s.cfg.App.FFmpegPath,
		Args:       args,
		StderrTail: 60,
		Prepare: func(cmd *exec.Cmd) {
			setLowPriority(cmd)
		},
	}, transcodeexecutor.Callbacks{
		OnStarted: func(process *os.Process) {
			job.setProcess(process)
			now := time.Now()
			job.lastAttemptHeartbeat.Store(now.UnixNano())
			s.markAttemptStarted(job, attempt, process.Pid, now)
			job.throttleStarted.Do(func() { go s.throttleLoop(job) })
		},
		OnProgress: func(progress transcodeexecutor.Progress) {
			s.recordProgress(job, progress)
			now := time.Now()
			last := time.Unix(0, job.lastAttemptHeartbeat.Load())
			if now.Sub(last) >= 10*time.Second {
				job.lastAttemptHeartbeat.Store(now.UnixNano())
				s.touchAttempt(attempt, now)
			}
		},
	})
	job.setProcess(nil)
	s.completeAttempt(attempt, result)
	return result
}

func (s *TranscodeService) cleanAttemptOutput(outputDir string) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(outputDir, entry.Name())); err != nil {
			s.logger.Warnf("清理失败 Attempt 产物失败 path=%s: %v", entry.Name(), err)
		}
	}
	s.InvalidateCacheDiskUsage()
}
