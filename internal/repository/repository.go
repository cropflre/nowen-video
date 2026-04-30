package repository

import (
	"gorm.io/gorm"
)

// Repositories 聚合所有数据仓储
type Repositories struct {
	db             *gorm.DB
	User           *UserRepo
	Library        *LibraryRepo
	Media          *MediaRepo
	Series         *SeriesRepo
	Person         *PersonRepo
	MediaPerson    *MediaPersonRepo
	WatchHistory   *WatchHistoryRepo
	Favorite       *FavoriteRepo
	Transcode      *TranscodeRepo
	Playlist       *PlaylistRepo
	Bookmark       *BookmarkRepo
	Comment        *CommentRepo
	ScheduledTask  *ScheduledTaskRepo
	ContentRating  *ContentRatingRepo
	UserPermission *UserPermissionRepo
	SystemSetting  *SystemSettingRepo
	PlaybackStats  *PlaybackStatsRepo
	ScrapeTask     *ScrapeTaskRepo
	ScrapeHistory  *ScrapeHistoryRepo
	// V3: AI 场景识别与内容理解
	VideoChapter   *VideoChapterRepo
	VideoHighlight *VideoHighlightRepo
	AIAnalysisTask *AIAnalysisTaskRepo
	// V3: AI 封面优化
	CoverCandidate *CoverCandidateRepo
	// V4: 缓存与标签优化
	AICache        *AICacheRepo
	GenreMapping   *GenreMappingRepo
	RecommendCache *RecommendCacheRepo
	// V5: Pulse 数据中心
	Pulse *PulseRepo
	// V6: P1~P3 新增功能
	Preprocess         *PreprocessRepo
	SubtitlePreprocess *SubtitlePreprocessRepo
	// 电影系列合集
	MovieCollection *MovieCollectionRepo
	// 多用户安全（审计 / 登录日志 / 邀请码）
	LoginLog   *LoginLogRepo
	AuditLog   *AuditLogRepo
	InviteCode *InviteCodeRepo
	// 系统日志
	SystemLog *SystemLogRepo
	// 文件管理操作日志（持久化）
	FileOpLog *FileOpLogRepo
}

func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		db:             db,
		User:           &UserRepo{db: db},
		Library:        &LibraryRepo{db: db},
		Media:          &MediaRepo{db: db},
		Series:         &SeriesRepo{db: db},
		Person:         &PersonRepo{db: db},
		MediaPerson:    &MediaPersonRepo{db: db},
		WatchHistory:   &WatchHistoryRepo{db: db},
		Favorite:       &FavoriteRepo{db: db},
		Transcode:      &TranscodeRepo{db: db},
		Playlist:       &PlaylistRepo{db: db},
		Bookmark:       &BookmarkRepo{db: db},
		Comment:        &CommentRepo{db: db},
		ScheduledTask:  &ScheduledTaskRepo{db: db},
		ContentRating:  &ContentRatingRepo{db: db},
		UserPermission: &UserPermissionRepo{db: db},
		SystemSetting:  &SystemSettingRepo{db: db},
		PlaybackStats:  &PlaybackStatsRepo{db: db},
		ScrapeTask:     &ScrapeTaskRepo{db: db},
		ScrapeHistory:  &ScrapeHistoryRepo{db: db},
		// V3
		VideoChapter:   &VideoChapterRepo{db: db},
		VideoHighlight: &VideoHighlightRepo{db: db},
		AIAnalysisTask: &AIAnalysisTaskRepo{db: db},
		CoverCandidate: &CoverCandidateRepo{db: db},
		// V4
		AICache:        &AICacheRepo{db: db},
		GenreMapping:   &GenreMappingRepo{db: db},
		RecommendCache: &RecommendCacheRepo{db: db},
		// V5: Pulse 数据中心
		Pulse: &PulseRepo{db: db},
		// V6: P1~P3 新增功能
		Preprocess:         &PreprocessRepo{db: db},
		SubtitlePreprocess: &SubtitlePreprocessRepo{db: db},
		// 电影系列合集
		MovieCollection: &MovieCollectionRepo{db: db},
		// 多用户安全
		LoginLog:   &LoginLogRepo{db: db},
		AuditLog:   &AuditLogRepo{db: db},
		InviteCode: &InviteCodeRepo{db: db},
		// 系统日志
		SystemLog: &SystemLogRepo{db: db},
		// 文件管理操作日志
		FileOpLog: NewFileOpLogRepo(db),
	}
}

// DB 返回底层数据库连接（供需要直接操作数据库的服务使用）
func (r *Repositories) DB() *gorm.DB {
	return r.db
}
