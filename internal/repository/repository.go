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
	AccessLog      *AccessLogRepo
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
	// V3: 家庭社交互动
	FamilyGroup         *FamilyGroupRepo
	FamilyMember        *FamilyMemberRepo
	MediaShare          *MediaShareRepo
	MediaLike           *MediaLikeRepo
	MediaRecommendation *MediaRecommendationRepo
	// V3: 实时直播
	LiveSource    *LiveSourceRepo
	LivePlaylist  *LivePlaylistRepo
	LiveRecording *LiveRecordingRepo
	// V3: 云端同步
	SyncDevice     *SyncDeviceRepo
	SyncRecord     *SyncRecordRepo
	UserSyncConfig *UserSyncConfigRepo
	// V4: 缓存与标签优化
	AICache        *AICacheRepo
	GenreMapping   *GenreMappingRepo
	RecommendCache *RecommendCacheRepo
	// V5: Pulse 数据中心
	Pulse *PulseRepo
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
		AccessLog:      &AccessLogRepo{db: db},
		ScheduledTask:  &ScheduledTaskRepo{db: db},
		ContentRating:  &ContentRatingRepo{db: db},
		UserPermission: &UserPermissionRepo{db: db},
		SystemSetting:  &SystemSettingRepo{db: db},
		PlaybackStats:  &PlaybackStatsRepo{db: db},
		ScrapeTask:     &ScrapeTaskRepo{db: db},
		ScrapeHistory:  &ScrapeHistoryRepo{db: db},
		// V3
		VideoChapter:        &VideoChapterRepo{db: db},
		VideoHighlight:      &VideoHighlightRepo{db: db},
		AIAnalysisTask:      &AIAnalysisTaskRepo{db: db},
		CoverCandidate:      &CoverCandidateRepo{db: db},
		FamilyGroup:         &FamilyGroupRepo{db: db},
		FamilyMember:        &FamilyMemberRepo{db: db},
		MediaShare:          &MediaShareRepo{db: db},
		MediaLike:           &MediaLikeRepo{db: db},
		MediaRecommendation: &MediaRecommendationRepo{db: db},
		LiveSource:          &LiveSourceRepo{db: db},
		LivePlaylist:        &LivePlaylistRepo{db: db},
		LiveRecording:       &LiveRecordingRepo{db: db},
		SyncDevice:          &SyncDeviceRepo{db: db},
		SyncRecord:          &SyncRecordRepo{db: db},
		UserSyncConfig:      &UserSyncConfigRepo{db: db},
		// V4
		AICache:        &AICacheRepo{db: db},
		GenreMapping:   &GenreMappingRepo{db: db},
		RecommendCache: &RecommendCacheRepo{db: db},
		// V5: Pulse 数据中心
		Pulse: &PulseRepo{db: db},
	}
}

// DB 返回底层数据库连接（供需要直接操作数据库的服务使用）
func (r *Repositories) DB() *gorm.DB {
	return r.db
}
