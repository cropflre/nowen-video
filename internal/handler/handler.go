package handler

import (
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// Handlers 聚合所有HTTP处理器
type Handlers struct {
	Auth      *AuthHandler
	Library   *LibraryHandler
	Media     *MediaHandler
	Series    *SeriesHandler
	Stream    *StreamHandler
	User      *UserHandler
	Admin     *AdminHandler
	Subtitle  *SubtitleHandler
	Metadata  *MetadataHandler
	Playlist  *PlaylistHandler
	Recommend *RecommendHandler
	Cast      *CastHandler
	WS        *WSHandler
	Bookmark  *BookmarkHandler
	Comment   *CommentHandler
}

func NewHandlers(services *service.Services, cfg *config.Config, logger *zap.SugaredLogger) *Handlers {
	return &Handlers{
		Auth:    &AuthHandler{authService: services.Auth, logger: logger},
		Library: &LibraryHandler{libService: services.Library, logger: logger},
		Media:   &MediaHandler{mediaService: services.Media, logger: logger},
		Series:  &SeriesHandler{seriesService: services.Series, logger: logger},
		Stream:  &StreamHandler{streamService: services.Stream, logger: logger},
		User:    &UserHandler{userService: services.User, mediaService: services.Media, logger: logger},
		Admin: &AdminHandler{
			userService:       services.User,
			transcodeService:  services.Transcode,
			monitorService:    services.Monitor,
			schedulerService:  services.Scheduler,
			permissionService: services.Permission,
			libraryService:    services.Library,
			metadataService:   services.Metadata,
			cfg:               cfg,
			logger:            logger,
		},
		Subtitle:  &SubtitleHandler{scanner: services.Scanner, streamService: services.Stream, logger: logger},
		Metadata:  &MetadataHandler{metadataService: services.Metadata, logger: logger},
		Playlist:  &PlaylistHandler{playlistService: services.Playlist, logger: logger},
		Recommend: &RecommendHandler{recommendService: services.Recommend, logger: logger},
		Cast:      &CastHandler{castService: services.Cast, logger: logger},
		WS:        &WSHandler{hub: services.WSHub, logger: logger},
		Bookmark:  &BookmarkHandler{bookmarkService: services.Bookmark, logger: logger},
		Comment:   &CommentHandler{commentService: services.Comment, logger: logger},
	}
}
