package handler

import (
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// Handlers 聚合所有HTTP处理器
type Handlers struct {
	Auth           *AuthHandler
	Library        *LibraryHandler
	Media          *MediaHandler
	Series         *SeriesHandler
	Stream         *StreamHandler
	User           *UserHandler
	Admin          *AdminHandler
	Subtitle       *SubtitleHandler
	Metadata       *MetadataHandler
	Playlist       *PlaylistHandler
	Recommend      *RecommendHandler
	Cast           *CastHandler
	WS             *WSHandler
	Bookmark       *BookmarkHandler
	Comment        *CommentHandler
	Stats          *StatsHandler
	Backup         *BackupHandler
	AI             *AIHandler
	ScrapeManager  *ScrapeManagerHandler
	FileManager    *FileManagerHandler
	AIAssistant    *AIAssistantHandler
	Notification   *NotificationHandler
	SubtitleSearch *SubtitleSearchHandler
	BatchMetadata  *BatchMetadataHandler
	EmbyCompat     *EmbyCompatHandler
	// V2: 中期发展规划处理器
	UserProfile     *UserProfileHandler
	OfflineDownload *OfflineDownloadHandler
	ABR             *ABRHandler
	Plugin          *PluginHandler
	Music           *MusicHandler
	Photo           *PhotoHandler
	Federation      *FederationHandler
	// V3: 新增处理器
	AIScene      *AISceneHandler
	FamilySocial *FamilySocialHandler
	Live         *LiveHandler
	CloudSync    *CloudSyncHandler
	// V5: Pulse 数据中心
	Pulse *PulseHandler
	// V6: P1~P3 新增处理器
	Tag       *TagHandler
	ShareLink *ShareLinkHandler
	MatchRule *MatchRuleHandler
	// 视频预处理
	Preprocess *PreprocessHandler
}

func NewHandlers(services *service.Services, repos *repository.Repositories, cfg *config.Config, logger *zap.SugaredLogger) *Handlers {
	return &Handlers{
		Auth:    &AuthHandler{authService: services.Auth, logger: logger},
		Library: &LibraryHandler{libService: services.Library, logger: logger},
		Media:   &MediaHandler{mediaService: services.Media, mediaPersonRepo: repos.MediaPerson, logger: logger},
		Series:  &SeriesHandler{seriesService: services.Series, mediaPersonRepo: repos.MediaPerson, logger: logger},
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
			seriesService:     services.Series,
			settingRepo:       repos.SystemSetting,
			libraryRepo:       repos.Library,
			cfg:               cfg,
			logger:            logger,
			db:                repos.DB(),
		},
		Subtitle:       &SubtitleHandler{scanner: services.Scanner, streamService: services.Stream, asrService: services.ASR, logger: logger},
		Metadata:       &MetadataHandler{metadataService: services.Metadata, logger: logger},
		Playlist:       &PlaylistHandler{playlistService: services.Playlist, logger: logger},
		Recommend:      &RecommendHandler{recommendService: services.Recommend, logger: logger},
		Cast:           &CastHandler{castService: services.Cast, logger: logger},
		WS:             &WSHandler{hub: services.WSHub, logger: logger},
		Bookmark:       &BookmarkHandler{bookmarkService: services.Bookmark, logger: logger},
		Comment:        &CommentHandler{commentService: services.Comment, logger: logger},
		Stats:          &StatsHandler{statsService: services.Stats, logger: logger},
		Backup:         &BackupHandler{backupService: services.Backup, logger: logger},
		AI:             &AIHandler{aiService: services.AI, logger: logger},
		ScrapeManager:  &ScrapeManagerHandler{scrapeService: services.ScrapeManager, logger: logger},
		FileManager:    &FileManagerHandler{fileService: services.FileManager, logger: logger},
		AIAssistant:    &AIAssistantHandler{assistantService: services.AIAssistant, logger: logger},
		Notification:   &NotificationHandler{notifyService: services.Notification, logger: logger},
		SubtitleSearch: &SubtitleSearchHandler{subtitleSearch: services.SubtitleSearch, streamService: services.Stream, logger: logger},
		BatchMetadata:  &BatchMetadataHandler{batchService: services.BatchMetadata, importExportSvc: services.ImportExport, logger: logger},
		EmbyCompat:     &EmbyCompatHandler{embyService: services.EmbyCompat, logger: logger},
		// V2
		UserProfile:     &UserProfileHandler{profileService: services.UserProfile, logger: logger},
		OfflineDownload: &OfflineDownloadHandler{downloadService: services.OfflineDownload, logger: logger},
		ABR:             &ABRHandler{abrService: services.ABR, logger: logger},
		Plugin:          &PluginHandler{pluginService: services.Plugin, logger: logger},
		Music:           &MusicHandler{musicService: services.Music, logger: logger},
		Photo:           &PhotoHandler{photoService: services.Photo, logger: logger},
		Federation:      &FederationHandler{federationService: services.Federation, logger: logger},
		// V3
		AIScene:      &AISceneHandler{sceneService: services.AIScene, logger: logger},
		FamilySocial: &FamilySocialHandler{socialService: services.FamilySocial, logger: logger},
		Live:         &LiveHandler{liveService: services.Live, logger: logger},
		CloudSync:    &CloudSyncHandler{syncService: services.CloudSync, logger: logger},
		// V5: Pulse 数据中心
		Pulse: &PulseHandler{pulseService: services.Pulse, logger: logger},
		// V6: P1~P3 新增处理器
		Tag:       &TagHandler{tagService: services.TagSvc, logger: logger},
		ShareLink: &ShareLinkHandler{shareService: services.ShareLinkSvc, mediaService: services.Media, seriesService: services.Series, logger: logger},
		MatchRule: &MatchRuleHandler{ruleService: services.MatchRuleSvc, logger: logger},
		// 视频预处理
		Preprocess: NewPreprocessHandler(services.Preprocess),
	}
}
