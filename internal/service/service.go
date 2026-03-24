package service

import (
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// Services 聚合所有服务
type Services struct {
	User       *UserService
	Auth       *AuthService
	Library    *LibraryService
	Media      *MediaService
	Series     *SeriesService
	Stream     *StreamService
	Transcode  *TranscodeService
	Metadata   *MetadataService
	Scanner    *ScannerService
	Playlist   *PlaylistService
	Recommend  *RecommendService
	Cast       *CastService
	Bookmark   *BookmarkService
	Comment    *CommentService
	Permission *PermissionService
	Scheduler  *SchedulerService
	Monitor    *MonitorService
	WSHub      *WSHub
}

func NewServices(repos *repository.Repositories, cfg *config.Config, logger *zap.SugaredLogger) *Services {
	transcoder := NewTranscodeService(repos.Transcode, cfg, logger)
	scanner := NewScannerService(repos.Media, repos.Series, cfg, logger)
	metadata := NewMetadataService(repos.Media, repos.Series, cfg, logger)

	// 创建WebSocket Hub
	wsHub := NewWSHub(logger)
	go wsHub.Run()

	// 注入WSHub到各服务
	scanner.SetWSHub(wsHub)
	transcoder.SetWSHub(wsHub)
	metadata.SetWSHub(wsHub)

	// 创建Library服务
	libService := NewLibraryService(repos.Library, repos.Media, repos.Series, scanner, metadata, logger)
	libService.SetWSHub(wsHub)

	// 创建调度器服务
	scheduler := NewSchedulerService(repos.ScheduledTask, repos.Library, logger)
	scheduler.SetLibraryService(libService)
	scheduler.SetWSHub(wsHub)
	scheduler.Start()

	// 创建监控服务
	monitor := NewMonitorService(cfg, transcoder, logger)
	monitor.SetWSHub(wsHub)
	monitor.Start()

	return &Services{
		User:       NewUserService(repos.User, cfg, logger),
		Auth:       NewAuthService(repos.User, cfg, logger),
		Library:    libService,
		Media:      NewMediaService(repos.Media, repos.Series, repos.WatchHistory, repos.Favorite, logger),
		Series:     NewSeriesService(repos.Series, repos.Media, logger),
		Stream:     NewStreamService(repos.Media, transcoder, cfg, logger),
		Transcode:  transcoder,
		Metadata:   metadata,
		Scanner:    scanner,
		Playlist:   NewPlaylistService(repos.Playlist, logger),
		Recommend:  NewRecommendService(repos.Media, repos.WatchHistory, repos.Favorite, logger),
		Cast:       NewCastService(repos.Media, cfg, logger),
		Bookmark:   NewBookmarkService(repos.Bookmark, repos.Media, logger),
		Comment:    NewCommentService(repos.Comment, repos.Media, logger),
		Permission: NewPermissionService(repos.UserPermission, repos.ContentRating, repos.WatchHistory, repos.AccessLog, logger),
		Scheduler:  scheduler,
		Monitor:    monitor,
		WSHub:      wsHub,
	}
}
