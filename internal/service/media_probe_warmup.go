package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodeprobe "github.com/nowen-video/nowen-video/internal/transcode/probe"
	"go.uber.org/zap"
)

const (
	defaultProbeWarmupWorkers  = 2
	defaultProbeWarmupCapacity = 16
	defaultProbeWarmupPageSize = 64
)

type mediaProbeWarmupRepository interface {
	ListProbeCandidatesByLibrary(libraryID, afterID string, limit int) ([]model.Media, error)
	UpdateTechnicalSummary(mediaID, videoCodec, audioCodec, resolution string, duration float64, fileSize int64) error
}

type mediaProbeProvider interface {
	Probe(ctx context.Context, media *model.Media) (*model.MediaProbeRecord, error)
}

type mediaProbeWarmupHook func(media *model.Media, probe *model.MediaProbeRecord) (submitted bool, err error)

type MediaProbeWarmupStats struct {
	QueueDepth       int    `json:"queue_depth"`
	PendingLibraries int    `json:"pending_libraries"`
	ActiveWorkers    int64  `json:"active_workers"`
	SubmittedRuns    uint64 `json:"submitted_runs"`
	CompletedRuns    uint64 `json:"completed_runs"`
	FailedRuns       uint64 `json:"failed_runs"`
	ProcessedMedia   uint64 `json:"processed_media"`
	SkippedMedia     uint64 `json:"skipped_media"`
	FailedMedia      uint64 `json:"failed_media"`
	StartupSubmitted uint64 `json:"startup_submitted"`
	StartupSkipped   uint64 `json:"startup_skipped"`
	StartupFailed    uint64 `json:"startup_failed"`
}

// MediaProbeWarmupService removes cold FFprobe work from first playback. Scan
// completion submits a library ID, while this service pages media records and
// populates the same persistent Probe cache used by runtime HLS and the
// Playback Planner.
type MediaProbeWarmupService struct {
	repo     mediaProbeWarmupRepository
	provider mediaProbeProvider
	logger   *zap.SugaredLogger

	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
	queue    chan string
	wg       sync.WaitGroup

	mu       sync.Mutex
	pending  map[string]struct{}
	onProbed mediaProbeWarmupHook
	closed   atomic.Bool

	activeWorkers   atomic.Int64
	submittedRuns   atomic.Uint64
	completedRuns   atomic.Uint64
	failedRuns      atomic.Uint64
	processedMedia  atomic.Uint64
	skippedMedia    atomic.Uint64
	failedMedia     atomic.Uint64
	startupSubmitted atomic.Uint64
	startupSkipped   atomic.Uint64
	startupFailed    atomic.Uint64
}

func NewMediaProbeWarmupService(
	repo mediaProbeWarmupRepository,
	provider mediaProbeProvider,
	logger *zap.SugaredLogger,
	parentDone ...<-chan struct{},
) *MediaProbeWarmupService {
	ctx, cancel := context.WithCancel(context.Background())
	service := &MediaProbeWarmupService{
		repo:     repo,
		provider: provider,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		queue:    make(chan string, defaultProbeWarmupCapacity),
		pending:  make(map[string]struct{}),
	}
	for index := 0; index < defaultProbeWarmupWorkers; index++ {
		service.wg.Add(1)
		go service.worker(index)
	}
	if len(parentDone) > 0 && parentDone[0] != nil {
		go func(done <-chan struct{}) {
			select {
			case <-done:
				service.stop()
			case <-service.ctx.Done():
			}
		}(parentDone[0])
	}
	return service
}

// SetOnProbed installs the next orchestration stage. The callback runs after
// Probe persistence and legacy summary synchronization, and must only submit a
// durable Job; it must not execute FFmpeg on the warmup worker.
func (s *MediaProbeWarmupService) SetOnProbed(callback mediaProbeWarmupHook) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.onProbed = callback
	s.mu.Unlock()
}

// SubmitLibrary is idempotent while the same library is queued or running.
// The scan completion path never blocks on FFprobe work.
func (s *MediaProbeWarmupService) SubmitLibrary(libraryID string) (bool, error) {
	libraryID = strings.TrimSpace(libraryID)
	if libraryID == "" {
		return false, fmt.Errorf("library id is required")
	}
	if s == nil || s.repo == nil || s.provider == nil || s.closed.Load() {
		return false, fmt.Errorf("media probe warmup service is unavailable")
	}

	s.mu.Lock()
	if _, exists := s.pending[libraryID]; exists {
		s.mu.Unlock()
		return false, nil
	}
	s.pending[libraryID] = struct{}{}
	s.mu.Unlock()

	select {
	case <-s.ctx.Done():
		s.removePending(libraryID)
		return false, context.Canceled
	case s.queue <- libraryID:
		s.submittedRuns.Add(1)
		return true, nil
	default:
		s.removePending(libraryID)
		return false, fmt.Errorf("media probe warmup queue is full")
	}
}

func (s *MediaProbeWarmupService) worker(index int) {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case libraryID := <-s.queue:
			s.activeWorkers.Add(1)
			err := s.warmLibrary(libraryID)
			s.activeWorkers.Add(-1)
			s.removePending(libraryID)
			if err != nil && !errors.Is(err, context.Canceled) {
				s.failedRuns.Add(1)
				if s.logger != nil {
					s.logger.Warnf("媒体 Probe 预热失败 worker=%d library=%s: %v", index, libraryID, err)
				}
				continue
			}
			if err == nil {
				s.completedRuns.Add(1)
				if s.logger != nil {
					s.logger.Infof("媒体 Probe 预热完成 worker=%d library=%s", index, libraryID)
				}
			}
		}
	}
}

func (s *MediaProbeWarmupService) warmLibrary(libraryID string) error {
	afterID := ""
	for {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		rows, err := s.repo.ListProbeCandidatesByLibrary(libraryID, afterID, defaultProbeWarmupPageSize)
		if err != nil {
			return fmt.Errorf("list probe candidates: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		for index := range rows {
			if err := s.ctx.Err(); err != nil {
				return err
			}
			media := &rows[index]
			record, probeErr := s.provider.Probe(s.ctx, media)
			if probeErr != nil {
				if errors.Is(probeErr, transcodeprobe.ErrUnsupportedSource) {
					s.skippedMedia.Add(1)
					continue
				}
				s.failedMedia.Add(1)
				if s.logger != nil {
					s.logger.Warnf("媒体 Probe 预热单项失败 media=%s: %v", media.ID, probeErr)
				}
				continue
			}
			transcodeprobe.ApplyToMedia(media, record)
			if err := s.repo.UpdateTechnicalSummary(
				media.ID,
				media.VideoCodec,
				media.AudioCodec,
				media.Resolution,
				media.Duration,
				record.SourceSize,
			); err != nil {
				s.failedMedia.Add(1)
				if s.logger != nil {
					s.logger.Warnf("同步 Probe 技术摘要失败 media=%s: %v", media.ID, err)
				}
				continue
			}
			s.processedMedia.Add(1)
			s.runOnProbed(media, record)
		}
		afterID = rows[len(rows)-1].ID
		if len(rows) < defaultProbeWarmupPageSize {
			return nil
		}
	}
}

func (s *MediaProbeWarmupService) runOnProbed(media *model.Media, record *model.MediaProbeRecord) {
	s.mu.Lock()
	callback := s.onProbed
	s.mu.Unlock()
	if callback == nil {
		return
	}
	submitted, err := callback(media, record)
	if err != nil {
		s.startupFailed.Add(1)
		if s.logger != nil {
			s.logger.Warnf("提交 Startup Stream 失败 media=%s: %v", media.ID, err)
		}
		return
	}
	if submitted {
		s.startupSubmitted.Add(1)
	} else {
		s.startupSkipped.Add(1)
	}
}

func (s *MediaProbeWarmupService) Stats() MediaProbeWarmupStats {
	if s == nil {
		return MediaProbeWarmupStats{}
	}
	s.mu.Lock()
	pending := len(s.pending)
	s.mu.Unlock()
	return MediaProbeWarmupStats{
		QueueDepth:       len(s.queue),
		PendingLibraries: pending,
		ActiveWorkers:    s.activeWorkers.Load(),
		SubmittedRuns:    s.submittedRuns.Load(),
		CompletedRuns:    s.completedRuns.Load(),
		FailedRuns:       s.failedRuns.Load(),
		ProcessedMedia:   s.processedMedia.Load(),
		SkippedMedia:     s.skippedMedia.Load(),
		FailedMedia:      s.failedMedia.Load(),
		StartupSubmitted: s.startupSubmitted.Load(),
		StartupSkipped:   s.startupSkipped.Load(),
		StartupFailed:    s.startupFailed.Load(),
	}
}

func (s *MediaProbeWarmupService) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.stop()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *MediaProbeWarmupService) stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		s.closed.Store(true)
		s.cancel()
	})
}

func (s *MediaProbeWarmupService) removePending(libraryID string) {
	s.mu.Lock()
	delete(s.pending, libraryID)
	s.mu.Unlock()
}
