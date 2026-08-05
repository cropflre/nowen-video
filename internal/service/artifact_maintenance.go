package service

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	transcodeartifactstore "github.com/nowen-video/nowen-video/internal/transcode/artifactstore"
	"go.uber.org/zap"
)

const artifactMaintenanceInterval = 30 * time.Second

// ArtifactMaintenanceService owns only historical Runtime retirement, durable
// Artifact cleanup and storage-health evidence. It cannot submit, claim or run
// media work and contains no FFmpeg runtime, worker queue or playback state.
type ArtifactMaintenanceService struct {
	repo          *repository.TranscodeRepo
	executionRepo *repository.TranscodeExecutionRepo
	cfg           *config.Config
	logger        *zap.SugaredLogger
	wsHub         *WSHub
	artifactStore *transcodeartifactstore.Store

	diskUsageMu    sync.RWMutex
	diskUsageBytes int64
	diskUsageAt    time.Time
	diskUsageTTL   time.Duration

	done         chan struct{}
	shutdownOnce sync.Once
	wg           sync.WaitGroup
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
	artifactStore, err := transcodeartifactstore.New(filepath.Join(cfg.Cache.CacheDir, "transcode"))
	if err != nil {
		panic(fmt.Sprintf("initialize transcode artifact store: %v", err))
	}
	service := &ArtifactMaintenanceService{
		repo:          repo,
		executionRepo: repository.NewTranscodeExecutionRepo(repo.DB()),
		cfg:           cfg,
		logger:        logger,
		artifactStore: artifactStore,
		diskUsageTTL:  30 * time.Second,
		done:          make(chan struct{}),
	}
	if err := service.initializeStorageHealth(); err != nil {
		panic(fmt.Sprintf("initialize artifact storage health: %v", err))
	}
	if report, inventoryErr := service.inventoryLegacyTranscodeProjection(time.Now()); inventoryErr != nil {
		logger.Warnf("启动登记 Legacy 转码目录失败: %v", inventoryErr)
	} else if report.Changed() {
		logger.Infof("启动登记 Legacy 转码目录 tasks=%d jobs=%d queued=%d blocked=%d missing=%d", report.TasksFound, report.JobsImported, report.ArtifactsQueued, report.ArtifactsBlocked, report.MissingPaths)
	}
	if report, retireErr := service.retirePersistentRuntimePlayback(time.Now()); retireErr != nil {
		logger.Warnf("启动退役持久 Runtime 播放状态失败: %v", retireErr)
	} else if report.Changed() {
		logger.Infof("启动退役持久 Runtime 播放状态 cancelled=%d artifacts=%d attempts=%d paths=%d", report.JobsCancelled, report.ArtifactsDeleted, report.AttemptsRetired, report.PathsRemoved)
	}
	service.runDiskPressureGovernorTick(time.Now(), true)
	service.wg.Add(1)
	go service.maintenanceLoop()
	return service
}

func (s *ArtifactMaintenanceService) SetWSHub(hub *WSHub) {
	if s != nil {
		s.wsHub = hub
	}
}

func (s *ArtifactMaintenanceService) maintenanceLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(artifactMaintenanceInterval)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			s.runStorageHealthTick(now, true)
			if report, err := s.inventoryLegacyTranscodeProjection(now); err != nil {
				s.logger.Warnf("周期登记 Legacy 转码目录失败: %v", err)
			} else if report.Changed() {
				s.logger.Infof("周期登记 Legacy 转码目录 tasks=%d jobs=%d queued=%d blocked=%d missing=%d", report.TasksFound, report.JobsImported, report.ArtifactsQueued, report.ArtifactsBlocked, report.MissingPaths)
			}
			s.runDiskPressureGovernorTick(now, false)
			report, err := s.retirePersistentRuntimePlayback(now)
			if err != nil {
				s.logger.Warnf("周期退役持久 Runtime 播放状态失败: %v", err)
			} else if report.Changed() {
				s.logger.Infof("周期退役持久 Runtime 播放状态 cancelled=%d artifacts=%d attempts=%d paths=%d", report.JobsCancelled, report.ArtifactsDeleted, report.AttemptsRetired, report.PathsRemoved)
			}
		case <-s.done:
			return
		}
	}
}

func (s *ArtifactMaintenanceService) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.shutdownOnce.Do(func() { close(s.done) })
	complete := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(complete)
	}()
	select {
	case <-complete:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *ArtifactMaintenanceService) legacyOutputDir(mediaID, quality string) string {
	if s == nil || s.cfg == nil {
		return ""
	}
	return filepath.Join(s.cfg.Cache.CacheDir, "transcode", mediaID, quality)
}
