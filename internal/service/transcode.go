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

// TranscodeService remains the compatibility facade used by handlers and
// clients. The execution job tables are authoritative; transcode_tasks is kept
// as the existing API projection while clients migrate.
type TranscodeService struct {
	repo          *repository.TranscodeRepo
	executionRepo *repository.TranscodeExecutionRepo
	cfg           *config.Config
	logger        *zap.SugaredLogger

	jobs        chan *TranscodeJob
	workerCount int
	submitMu    sync.Mutex
	mu          sync.RWMutex
	running     map[string]*TranscodeJob

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

// TranscodeJob owns one durable cancellation context from queue admission until
// finalization. ExecutionJob persists orchestration state and Attempt records
// preserve every concrete process run, including hardware fallback failures.
type TranscodeJob struct {
	Task         *model.TranscodeTask
	ExecutionJob *model.TranscodeJobRecord
	Media        *model.Media
	Quality      string

	ctx        context.Context
	cancel     context.CancelFunc
	cancelOnce sync.Once
	taskMu     sync.Mutex

	processMu sync.RWMutex
	process   *os.Process

	playbackPos    atomic.Uint64
	transcodedPos  atomic.Uint64
	lastDBProgress atomic.Uint64
	lastHeartbeat  atomic.Int64
	suspended      atomic.Int32
	startOffset    float64

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
	service := &TranscodeService{
		repo:             repo,
		executionRepo:    repository.NewTranscodeExecutionRepo(repo.DB()),
		cfg:              cfg,
		logger:           logger,
		jobs:             make(chan *TranscodeJob, 100),
		running:          make(map[string]*TranscodeJob),
		executionRuntime: transcoderuntime.Default(),
		diskUsageTTL:     30 * time.Second,
	}

	service.hwAccelMu.Do(func() {
		service.hwAccel = service.detectHWAccel()
		logger.Infof("硬件加速模式: %s", service.hwAccel)
	})

	service.workerCount = 1
	if service.hwAccel != "" && service.hwAccel != ffmpeg.HWAccelNone {
		service.workerCount = 2
	}
	for workerID := 0; workerID < service.workerCount; workerID++ {
		go service.worker(workerID)
	}
	go service.recoverPendingTasks()
	return service
}

func (s *TranscodeService) SetWSHub(hub *WSHub) { s.wsHub = hub }
func (s *TranscodeService) GetHWAccelInfo() string { return s.hwAccel }
func (s *TranscodeService) ExecutionRuntime() *transcoderuntime.Runtime { return s.executionRuntime }

func (s *TranscodeService) detectHWAccel() string {
	return ffmpeg.DetectHWAccel(s.cfg, s.logger)
}

func (s *TranscodeService) recoverPendingTasks() {
	rows, err := s.repo.ListRunning()
	if err != nil {
		s.logger.Warnf("恢复转码任务状态失败: %v", err)
		return
	}
	for i := range rows {
		rows[i].Status = "failed"
		rows[i].Error = "服务重启导致执行租约失效，请重新提交"
		if err := s.repo.Update(&rows[i]); err != nil {
			s.logger.Warnf("重置中断任务状态失败 task=%s: %v", rows[i].ID, err)
		}
		if executionJob, findErr := s.executionRepo.FindActiveByLegacyTaskID(rows[i].ID); findErr == nil {
			_ = s.executionRepo.CompleteJob(executionJob.ID, "failed", time.Now())
		}
	}
	if len(rows) > 0 {
		s.logger.Infof("已回收 %d 个重启前未完成的转码任务及执行租约", len(rows))
	}
}

func (s *TranscodeService) StartTranscode(media *model.Media, quality string) (*model.TranscodeTask, error) {
	return s.startTranscodeInternal(media, quality, 0)
}

// StartABRTranscode performs progressive warmup: only the first valid rendition
// is queued immediately. Other renditions start when requested by the client.
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
	if media == nil || strings.TrimSpace(media.ID) == "" {
		return nil, fmt.Errorf("媒体不能为空")
	}
	if _, ok := qualityPresets[quality]; !ok {
		return nil, fmt.Errorf("未知转码档位: %s", quality)
	}

	s.submitMu.Lock()
	defer s.submitMu.Unlock()

	if task, err := s.findActiveExecutionTask(media, quality, startOffset); err == nil {
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
	}
	if err := s.repo.Create(task); err != nil {
		return nil, err
	}
	executionJob, err := s.createExecutionJob(media, quality, startOffset, task.ID)
	if err != nil {
		_ = s.repo.DeleteByID(task.ID)
		return nil, fmt.Errorf("创建持久化转码 Job 失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &TranscodeJob{
		Task:         task,
		ExecutionJob: executionJob,
		Media:        media,
		Quality:      quality,
		ctx:          ctx,
		cancel:       cancel,
		startOffset:  startOffset,
		throttleDone: make(chan struct{}),
	}
	s.mu.Lock()
	s.running[task.ID] = job
	s.mu.Unlock()

	select {
	case s.jobs <- job:
		return task, nil
	default:
		job.RequestCancel()
		s.mu.Lock()
		delete(s.running, task.ID)
		s.mu.Unlock()
		task.Status = "failed"
		task.Error = "转码队列已满"
		_ = s.repo.Update(task)
		s.persistJobTerminal(job, "failed", time.Now())
		return nil, fmt.Errorf("转码队列已满")
	}
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

func (s *TranscodeService) worker(id int) {
	s.logger.Infof("转码工作协程 #%d 启动", id)
	for job := range s.jobs {
		s.processJob(job)
	}
}

func (s *TranscodeService) processJob(job *TranscodeJob) {
	defer func() {
		job.stopThrottle()
		job.setProcess(nil)
		s.mu.Lock()
		delete(s.running, job.Task.ID)
		s.mu.Unlock()
	}()

	if !s.markJobRunning(job) {
		s.finalizeCancelled(job)
		return
	}
	s.logger.Infof("开始转码: %s (%s)", job.Media.Title, job.Quality)
	s.broadcastTranscodeEvent(EventTranscodeStarted, &TranscodeProgressData{
		TaskID:  job.Task.ID,
		MediaID: job.Media.ID,
		Title:   job.Media.Title,
		Quality: job.Quality,
		Message: fmt.Sprintf("开始转码: %s (%s)", job.Media.Title, job.Quality),
	})

	backend := s.hwAccel
	if backend == "" {
		backend = ffmpeg.HWAccelNone
	}
	result := s.runAttempt(job, backend, 1)
	if result.Cancelled || result.TimedOut || job.CancellationRequested() {
		s.finalizeCancelled(job)
		return
	}

	if result.Err != nil && backend != ffmpeg.HWAccelNone {
		s.logger.Warnf("硬件转码失败 backend=%s media=%s: %v，切换独立软件 Attempt", backend, job.Media.ID, result.Err)
		s.cleanAttemptOutput(job.Task.OutputDir)
		result = s.runAttempt(job, ffmpeg.HWAccelNone, 2)
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
			job.lastHeartbeat.Store(now.UnixNano())
			s.markAttemptStarted(job, attempt, process.Pid, now)
			job.throttleStarted.Do(func() { go s.throttleLoop(job) })
		},
		OnProgress: func(progress transcodeexecutor.Progress) {
			s.recordProgress(job, progress)
			now := time.Now()
			last := time.Unix(0, job.lastHeartbeat.Load())
			if now.Sub(last) >= 10*time.Second {
				job.lastHeartbeat.Store(now.UnixNano())
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
