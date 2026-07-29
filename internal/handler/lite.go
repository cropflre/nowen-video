package handler

import (
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// NewLiteHandlers wires only handlers exposed by the NAS-oriented lite server.
// Optional full-profile handlers remain nil and therefore cannot accidentally
// leak routes from the full server into the lightweight runtime.
func NewLiteHandlers(services *service.Services, repos *repository.Repositories, cfg *config.Config, logger *zap.SugaredLogger) *Handlers {
	return &Handlers{
		Auth:    &AuthHandler{authService: services.Auth, serverName: cfg.Emby.ServerName, logger: logger},
		Library: &LibraryHandler{libService: services.Library, permSvc: services.Permission, logger: logger},
		Media:   &MediaHandler{mediaService: services.Media, personRepo: repos.Person, mediaPersonRepo: repos.MediaPerson, logger: logger},
		Series:  &SeriesHandler{seriesService: services.Series, mediaPersonRepo: repos.MediaPerson, logger: logger},
		Stream:  &StreamHandler{streamService: services.Stream, transcodeService: services.Transcode, logger: logger},
		User:    &UserHandler{userService: services.User, authService: services.Auth, mediaService: services.Media, loginLogRepo: repos.LoginLog, logger: logger},
		Admin: &AdminHandler{
			userService:       services.User,
			authService:       services.Auth,
			transcodeService:  services.Transcode,
			permissionService: services.Permission,
			libraryService:    services.Library,
			metadataService:   services.Metadata,
			seriesService:     services.Series,
			settingRepo:       repos.SystemSetting,
			libraryRepo:       repos.Library,
			loginLogRepo:      repos.LoginLog,
			auditLogRepo:      repos.AuditLog,
			inviteRepo:        repos.InviteCode,
			mediaRepo:         repos.Media,
			cfg:               cfg,
			logger:            logger,
			db:                repos.DB(),
		},
		Subtitle:      &SubtitleHandler{scanner: services.Scanner, streamService: services.Stream, asrService: nil, logger: logger},
		Metadata:      &MetadataHandler{metadataService: services.Metadata, logger: logger},
		Playlist:      &PlaylistHandler{playlistService: services.Playlist, logger: logger},
		Recommend:     &RecommendHandler{recommendService: services.Recommend, logger: logger},
		Cast:          &CastHandler{castService: services.Cast, logger: logger},
		WS:            &WSHandler{hub: services.WSHub, logger: logger},
		Bookmark:      &BookmarkHandler{bookmarkService: services.Bookmark, logger: logger},
		Stats:         &StatsHandler{statsService: services.Stats, logger: logger},
		ScrapeManager: &ScrapeManagerHandler{scrapeService: services.ScrapeManager, logger: logger},
		FileManager:   &FileManagerHandler{fileService: services.FileManager, logger: logger},
		SubtitleSearch: &SubtitleSearchHandler{
			subtitleSearch: services.SubtitleSearch,
			streamService:  services.Stream,
			logger:         logger,
		},
		BatchMetadata: &BatchMetadataHandler{batchService: services.BatchMetadata, importExportSvc: services.ImportExport, logger: logger},
		AI: &AIHandler{
			aiService:   services.AI,
			router:      services.AIRouter,
			usageRepo:   repos.AIUsage,
			failoverLog: repos.AIFailover,
			cfg:         cfg,
			logger:      logger,
		},
		AICost:    NewAICostHandler(services.AICost, logger),
		Storage:   NewStorageHandler(services.WebDAV, services.RemoteStorage, cfg, logger),
		SystemLog: &SystemLogHandler{logRepo: repos.SystemLog, logger: logger},
		Collection: &CollectionHandler{
			collectionService: services.Collection,
			streamService:     services.Stream,
			logger:            logger,
		},
	}
}
