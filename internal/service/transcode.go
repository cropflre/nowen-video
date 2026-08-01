package service

import (
	"context"
	"errors"
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
	transcodeartifactstore "github.com/nowen-video/nowen-video/internal/transcode/artifactstore"
	transcodeexecutor "github.com/nowen-video/nowen-video/internal/transcode/executor"
	transcodegovernor "github.com/nowen-video/nowen-video/internal/transcode/governor"
	transcodeprobe "github.com/nowen-video/nowen-video/internal/transcode/probe"
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
	mediaProbe       *transcodeprobe.Service
	probeWarmup      *MediaProbeWarmupService
	probeWarmupOnce  sync.Once
	artifactStore    *transcodeartifactstore.Store

	throttleSuspendSeconds atomic.Uint64
	throttleSuspendCount   atomic.Uint64

	diskUsageMu    sync.RWMutex
	diskUsageBytes int64
	diskUsageAt    time.Time
	diskUsageTTL   time.Duration
}

type TranscodeJob struct {
	Task            *model.TranscodeTask
	ExecutionJob    *model.TranscodeJobRecord
	CurrentAttempt  *model.TranscodeAttemptRecord
	CurrentArtifact *model.TranscodeArtifactRecord
	Media           *model.Media
	Probe           *model.MediaProbeRecord
	Quality         string

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
	mediaProbe, err := transcodeprobe.NewService(repo.DB(), cfg.App.FFprobePath, logger)
	if err != nil {
		panic(fmt.Sprintf("initialize media probe service: %v", err))
	}
	artifactStore, err := transcodeartifactstore.New(filepath.Join(cfg.Cache.CacheDir, "transcode"))
	if err != nil {
		panic(fmt.Sprintf("initialize transcode artifact store: %v", err))
	}
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
		mediaProbe:             mediaProbe,
		artifactStore:          artifactStore,
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

func (s *TranscodeService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
	s.attachProbeWarmup(hub)
}

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
			if s.hasPublishedHLSArtifact(media, quality) {
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

// WaitForFirstSegment is retained for legacy callers. New playback paths use
// WaitForFirstSegmentForMedia so Artifact resolution includes source identity.
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
		closed := s.jobs.IsClosed()
		s.mu.Unlock()
		if closed {
			if s.jobs.releaseClaimAfterClose(job.ExecutionJob) {
				s.markJobQueuedForRecovery(job)
			}
			job.RequestCancel()
			s.mu.Lock()
			delete(s.running, job.Task.ID)
			s.mu.Unlock()
			continue
		}
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

	// Close the small Claim-to-local-registration race before starting FFmpeg.
	// Cancellation changes desired_state, so the same lease predicate rejects
	// this preflight and the process never starts. A transient database error
	// leaves the claim untouched for the recovery loop instead of executing
	// without verified ownership.
	renewed, renewErr := s.executionRepo.RenewJobLease(
		job.ExecutionJob.ID,
		job.leaseToken,
		time.Now(),
		s.leaseDuration,
	)
	if renewErr != nil {
		s.logger.Warnf("启动前验证转码 Lease 失败 job=%s worker=%s: %v", job.ExecutionJob.ID, workerID, renewErr)
		return
	}
	if !renewed {
		job.RequestCancel()
		s.finalizeCancelled(job)
		return
	}
	go s.leaseHeartbeatLoop(job)

	if s.mediaProbe != nil {
		probeRecord, probeErr := s.mediaProbe.Probe(job.ctx, job.Media)
		if probeErr != nil {
			if !errors.Is(probeErr, transcodeprobe.ErrUnsupportedSource) {
				s.logger.Warnf("媒体 Probe 失败，使用兼容转码参数 media=%s: %v", job.Media.ID, probeErr)
			}
		} else {
			job.Probe = probeRecord
			transcodeprobe.ApplyToMedia(job.Media, probeRecord)
		}
	}
	if job.CancellationRequested() {
		s.finalizeCancelled(job)
		return
	}

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

	backend := s.preferredAttemptBackend(job)
	result := s.runAttempt(job, backend, nextAttempt)
	if result.Cancelled || result.TimedOut || job.CancellationRequested() {
		s.finalizeCancelled(job)
		return
	}

	if result.Err != nil && backend != ffmpeg.HWAccelNone {
		s.logger.Warnf("硬件转码失败 backend=%s media=%s: %v，切换独立软件 Attempt", backend, job.Media.ID, result.Err)
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

	execution, err := s.prepareAttemptExecution(job, attemptNumber, backend)
	if err != nil {
		return transcodeexecutor.Result{Err: fmt.Errorf("创建 Attempt 工作区失败: %w", err)}
	}
	attempt := execution.Attempt
	args := execution.Args
	s.logger.Debugf("FFmpeg attempt=%d backend=%s workspace=%s command=%s %s", attemptNumber, backend, execution.Workspace, s.cfg.App.FFmpegPath, strings.Join(redactFFmpegArgs(args), " "))
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
	s.completeAttemptArtifact(job, execution, result)
	return result
}
