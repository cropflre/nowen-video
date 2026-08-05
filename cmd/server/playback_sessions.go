package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/handler"
	embyh "github.com/nowen-video/nowen-video/internal/handler/emby"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

type fullPlaybackRuntime struct {
	sessions  *service.PlaybackSessionService
	execution *service.MediaExecutionService
	plan      *handler.PlaybackPlanHandler
	handler   *handler.PlaybackSessionHandler
}

func newFullPlaybackRuntime(
	cfg *config.Config,
	services *service.Services,
	repos *repository.Repositories,
	logger *zap.SugaredLogger,
) (*fullPlaybackRuntime, error) {
	mediaExecution, err := service.NewMediaExecutionService(repos.DB(), cfg, logger)
	if err != nil {
		return nil, err
	}
	if services != nil && services.Preprocess != nil {
		if err := services.Preprocess.BindMediaExecution(mediaExecution); err != nil {
			return nil, err
		}
	}
	sessions, err := service.NewPlaybackSessionServiceWithExecution(
		repos.Media,
		mediaExecution,
		cfg,
		logger,
	)
	if err != nil {
		return nil, err
	}
	// Emby/Infuse compatibility, first-party playback and explicit administrator
	// preprocessing share one stateless execution platform. No consumer can
	// reach the retired Runtime queue, Lease store or playback Artifact resolver.
	embyh.SetDefaultPlaybackSessionService(sessions)
	return &fullPlaybackRuntime{
		sessions:  sessions,
		execution: mediaExecution,
		plan:      handler.NewPlaybackPlanHandler(services.Stream, logger),
		handler: handler.NewPlaybackSessionHandler(
			sessions,
			services.Permission,
			repos.Media,
			logger,
		),
	}, nil
}

func (r *fullPlaybackRuntime) Register(api *gin.RouterGroup, guardByMediaID gin.HandlerFunc) {
	if r == nil {
		return
	}
	api.GET("/stream/:id/plan", guardByMediaID, r.plan.Get)
	api.POST("/playback/sessions", r.handler.Create)
	api.GET("/playback/sessions/:sessionID/status", r.handler.Status)
	api.POST("/playback/sessions/:sessionID/heartbeat", r.handler.Heartbeat)
	api.POST("/playback/sessions/:sessionID/restart", r.handler.Restart)
	api.DELETE("/playback/sessions/:sessionID", r.handler.Close)
	api.GET("/playback/sessions/:sessionID/generations/:generationID/stream.m3u8", r.handler.Playlist)
	api.GET("/playback/sessions/:sessionID/generations/:generationID/:file", r.handler.Segment)
}

func (r *fullPlaybackRuntime) Shutdown(ctx context.Context) error {
	if r == nil || r.sessions == nil {
		return nil
	}
	err := r.sessions.Shutdown(ctx)
	embyh.SetDefaultPlaybackSessionService(nil)
	return err
}
