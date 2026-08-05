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

// ArtifactMaintenanceService is the formal owner of historical migration and durable Artifact cleanup.
type ArtifactMaintenanceService = TranscodeService

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

func NewArtifactMaintenanceService(repo *repository.TranscodeRepo, cfg *config.Config, logger *zap.SugaredLogger) *ArtifactMaintenanceService {
	if repo == nil || repo.DB() == nil {
		panic("transcode repository is required")
	}
	if cfg == nil {
		panic("configuration is required")
	}
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	if err := model.AutoMigrateTranscodeExecution(repo.DB()); err != nil {
		panic(fmt.Sprintf("migrate transcode execution schema: %v", err))
	}
	executionRepo := repository.NewTranscodeExecutionRepo(repo.DB())
	artifactStore, err := transcodeartifactstore.New(filepath.Join(cfg.Cache.CacheDir, "transcode"))
	if err != nil {
		panic(fmt.Sprintf("initialize transcode artifact store: %v", err))
	}
	service := &TranscodeService{
		repo:                   repo,
		executionRepo:          executionRepo,
		cfg:                    cfg,
		logger:                 logger,
		jobs:                   newTranscodePriorityQueue(executionRepo, repo, 1, logger),
		running:                make(map[string]*TranscodeJob),
		instanceID:             newTranscodeInstanceID(),
		leaseDuration:          defaultTranscodeLeaseDuration,
		leaseHeartbeatInterval: defaultTranscodeLeaseHeartbeatInterval,
		leaseRecoveryInterval:  defaultTranscodeLeaseRecoveryInterval,
		artifactStore:          artifactStore,
		diskUsageTTL:           30 * time.Second,
	}
	service.recoverPendingTasks()
	go service.leaseRecoveryLoop()
	return service
}

// NewTranscodeService is retained only for source compatibility. It returns a
// maintenance-only service and cannot start Runtime execution.
func NewTranscodeService(repo *repository.TranscodeRepo, cfg *config.Config, logger *zap.SugaredLogger) *TranscodeService {
	return NewArtifactMaintenanceService(repo, cfg, logger)
}

func (s *TranscodeService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

func (s *TranscodeService) GetHWAccelInfo() string                      { return ffmpeg.HWAccelNone }
func (s *TranscodeService) ExecutionRuntime() *transcoderuntime.Runtime { return nil }

func (s *TranscodeService) detectHWAccel() string {
	return ffmpeg.DetectHWAccel(s.cfg, s.logger)
}

func (s *TranscodeService) StartTranscode(*model.Media, string) (*model.TranscodeTask, error) {
	return nil, ErrPersistentRuntimeTranscodeRetired
}

func (s *TranscodeService) StartABRTranscode(*model.Media, []string) ([]*model.TranscodeTask, error) {
	return nil, ErrPersistentRuntimeTranscodeRetired
}

func (s *TranscodeService) StartTranscodeWithStart(*model.Media, string, float64) (*model.TranscodeTask, error) {
	return nil, ErrPersistentRuntimeTranscodeRetired
}

func (s *TranscodeService) startTranscodeInternal(*model.Media, string, float64) (*model.TranscodeTask, error) {
	return nil, ErrPersistentRuntimeTranscodeRetired
}

func (s *TranscodeService) startTranscodeWithPriority(*model.Media, string, float64, int) (*model.TranscodeTask, error) {
	return nil, ErrPersistentRuntimeTranscodeRetired
}

// WaitForFirstSegment is retired with media-keyed Runtime HLS.
func (s *TranscodeService) WaitForFirstSegment(context.Context, string, string) error {
	return ErrPersistentRuntimeTranscodeRetired
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
