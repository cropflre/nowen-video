package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/handler"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

type fullPlaybackRuntime struct {
	sessions *service.PlaybackSessionService
	plan     *handler.PlaybackPlanHandler
	handler  *handler.PlaybackSessionHandler
}

func newFullPlaybackRuntime(
	cfg *config.Config,
	services *service.Services,
	repos *repository.Repositories,
	logger *zap.SugaredLogger,
) (*fullPlaybackRuntime, error) {
	sessions, err := service.NewPlaybackSessionService(repos.Media, services.Transcode, cfg, logger)
	if err != nil {
		return nil, err
	}
	return &fullPlaybackRuntime{
		sessions: sessions,
		plan:     handler.NewPlaybackPlanHandler(services.Stream, logger),
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
	return r.sessions.Shutdown(ctx)
}
