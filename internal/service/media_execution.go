package service

import (
	"fmt"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	transcodeprobe "github.com/nowen-video/nowen-video/internal/transcode/probe"
	transcoderuntime "github.com/nowen-video/nowen-video/internal/transcode/runtime"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MediaExecutionService owns the stateless execution capabilities shared by
// ephemeral playback and explicit administrator preprocessing. It has no Job
// queue, Lease loop, Artifact store, legacy task projection or recovery path.
type MediaExecutionService struct {
	cfg              *config.Config
	logger           *zap.SugaredLogger
	executionRuntime *transcoderuntime.Runtime
	mediaProbe       *transcodeprobe.Service
	hwAccel          string
}

func NewMediaExecutionService(
	db *gorm.DB,
	cfg *config.Config,
	logger *zap.SugaredLogger,
) (*MediaExecutionService, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	probe, err := transcodeprobe.NewService(db, cfg.App.FFprobePath, logger)
	if err != nil {
		return nil, fmt.Errorf("initialize media execution probe: %w", err)
	}
	return &MediaExecutionService{
		cfg:              cfg,
		logger:           logger,
		executionRuntime: transcoderuntime.Default(),
		mediaProbe:       probe,
		hwAccel:          ffmpeg.DetectHWAccel(cfg, logger),
	}, nil
}

func (s *MediaExecutionService) ExecutionRuntime() *transcoderuntime.Runtime {
	if s == nil {
		return nil
	}
	return s.executionRuntime
}

func (s *MediaExecutionService) GetHWAccelInfo() string {
	if s == nil {
		return ffmpeg.HWAccelNone
	}
	return s.hwAccel
}

func (s *MediaExecutionService) GetCachedMediaProbe(media *model.Media) *model.MediaProbeRecord {
	if s == nil || s.mediaProbe == nil || media == nil {
		return nil
	}
	record, err := s.mediaProbe.Cached(media)
	if err != nil {
		return nil
	}
	return record
}

func (s *MediaExecutionService) GetMediaProbeStats() transcodeprobe.Stats {
	if s == nil || s.mediaProbe == nil {
		return transcodeprobe.Stats{}
	}
	return s.mediaProbe.Stats()
}

// playbackCompatibilityAdapter is deliberately process-local and contains no
// persistent repositories or queue. It exists only while the public playback
// constructor is migrated from the historical concrete TranscodeService type.
func (s *MediaExecutionService) playbackCompatibilityAdapter() *TranscodeService {
	if s == nil {
		return nil
	}
	return &TranscodeService{
		cfg:              s.cfg,
		logger:           s.logger,
		executionRuntime: s.executionRuntime,
		mediaProbe:       s.mediaProbe,
		hwAccel:          s.hwAccel,
	}
}

// NewPlaybackSessionServiceWithExecution is the new construction boundary.
// Playback depends on MediaExecutionService and cannot reach the persistent
// Runtime queue, Artifact store or legacy task repository.
func NewPlaybackSessionServiceWithExecution(
	mediaRepo *repository.MediaRepo,
	execution *MediaExecutionService,
	cfg *config.Config,
	logger *zap.SugaredLogger,
) (*PlaybackSessionService, error) {
	if execution == nil {
		return nil, fmt.Errorf("media execution service is required")
	}
	return NewPlaybackSessionService(
		mediaRepo,
		execution.playbackCompatibilityAdapter(),
		cfg,
		logger,
	)
}
