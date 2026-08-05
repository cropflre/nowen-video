package service

import (
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// NewTaskCenterServiceWithoutRuntimeTranscode keeps storage incidents and
// Artifact cleanup visible while removing the historical transcode_tasks read
// model. Runtime playback is session-scoped and is not an administrator task.
func NewTaskCenterServiceWithoutRuntimeTranscode(
	library *LibraryService,
	transcodeRepo *repository.TranscodeRepo,
	scrapeRepo *repository.ScrapeTaskRepo,
	logger *zap.SugaredLogger,
) *TaskCenterService {
	service := NewTaskCenterService(library, transcodeRepo, scrapeRepo, logger)
	service.transcodeRepo = nil
	return service
}

// NewTaskActionDispatcherWithoutRuntimeTranscode preserves scrape retries and
// Artifact cleanup recovery but removes cancel/retry operations for the retired
// persistent Runtime queue.
func NewTaskActionDispatcherWithoutRuntimeTranscode(
	maintenance *ArtifactMaintenanceService,
	scrape *ScrapeManagerService,
	transcodeRepo *repository.TranscodeRepo,
	scrapeRepo *repository.ScrapeTaskRepo,
	mediaRepo *repository.MediaRepo,
	wsHub *WSHub,
	logger *zap.SugaredLogger,
) *TaskActionDispatcher {
	dispatcher := NewTaskActionDispatcher(
		maintenance,
		scrape,
		transcodeRepo,
		scrapeRepo,
		mediaRepo,
		wsHub,
		logger,
	)
	dispatcher.transcode = nil
	dispatcher.transcodeLookup = nil
	dispatcher.mediaResolver = nil
	return dispatcher
}
