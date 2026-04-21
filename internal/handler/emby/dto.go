package emby

import "time"

// ==================== Emby 标准 DTO ====================
// 字段命名严格遵循 Emby OpenAPI 规范（PascalCase），不要改成下划线。
// 这些结构体是最小可用子集，只覆盖 Infuse 等客户端真正会读取的字段；
// 其他字段缺省时 Emby 客户端会自行容错。

// SystemInfoPublic 对应 /System/Info/Public（无需认证）。
type SystemInfoPublic struct {
	LocalAddress            string `json:"LocalAddress"`
	ServerName              string `json:"ServerName"`
	Version                 string `json:"Version"`
	ProductName             string `json:"ProductName"`
	OperatingSystem         string `json:"OperatingSystem"`
	Id                      string `json:"Id"`
	StartupWizardCompleted  bool   `json:"StartupWizardCompleted"`
}

// SystemInfo 对应 /System/Info（需要认证，字段更丰富）。
type SystemInfo struct {
	SystemInfoPublic
	OperatingSystemDisplayName string   `json:"OperatingSystemDisplayName"`
	HasPendingRestart          bool     `json:"HasPendingRestart"`
	IsShuttingDown             bool     `json:"IsShuttingDown"`
	SupportsLibraryMonitor     bool     `json:"SupportsLibraryMonitor"`
	WebSocketPortNumber        int      `json:"WebSocketPortNumber"`
	CanSelfRestart             bool     `json:"CanSelfRestart"`
	CanSelfUpdate              bool     `json:"CanSelfUpdate"`
	CanLaunchWebBrowser        bool     `json:"CanLaunchWebBrowser"`
	ProgramDataPath            string   `json:"ProgramDataPath"`
	ItemsByNamePath            string   `json:"ItemsByNamePath"`
	CachePath                  string   `json:"CachePath"`
	LogPath                    string   `json:"LogPath"`
	InternalMetadataPath       string   `json:"InternalMetadataPath"`
	TranscodingTempPath        string   `json:"TranscodingTempPath"`
	HttpServerPortNumber       int      `json:"HttpServerPortNumber"`
	SupportsHttps              bool     `json:"SupportsHttps"`
	HttpsPortNumber            int      `json:"HttpsPortNumber"`
	HasUpdateAvailable         bool     `json:"HasUpdateAvailable"`
	EncoderLocation            string   `json:"EncoderLocation"`
	SystemArchitecture         string   `json:"SystemArchitecture"`
	CompletedInstallations     []string `json:"CompletedInstallations"`
}

// AuthenticationResult 对应 /Users/AuthenticateByName 的响应。
type AuthenticationResult struct {
	User        EmbyUser `json:"User"`
	AccessToken string   `json:"AccessToken"`
	ServerId    string   `json:"ServerId"`
	SessionInfo SessionInfo `json:"SessionInfo"`
}

// EmbyUser 对应 Emby User 对象。
type EmbyUser struct {
	Name                      string          `json:"Name"`
	ServerId                  string          `json:"ServerId"`
	Id                        string          `json:"Id"`
	HasPassword               bool            `json:"HasPassword"`
	HasConfiguredPassword     bool            `json:"HasConfiguredPassword"`
	HasConfiguredEasyPassword bool            `json:"HasConfiguredEasyPassword"`
	EnableAutoLogin           bool            `json:"EnableAutoLogin"`
	LastLoginDate             string          `json:"LastLoginDate,omitempty"`
	LastActivityDate          string          `json:"LastActivityDate,omitempty"`
	Configuration             UserConfig      `json:"Configuration"`
	Policy                    UserPolicy      `json:"Policy"`
	PrimaryImageTag           string          `json:"PrimaryImageTag,omitempty"`
}

// UserConfig 用户个人偏好。
type UserConfig struct {
	AudioLanguagePreference     string   `json:"AudioLanguagePreference"`
	PlayDefaultAudioTrack       bool     `json:"PlayDefaultAudioTrack"`
	SubtitleLanguagePreference  string   `json:"SubtitleLanguagePreference"`
	DisplayMissingEpisodes      bool     `json:"DisplayMissingEpisodes"`
	GroupedFolders              []string `json:"GroupedFolders"`
	SubtitleMode                string   `json:"SubtitleMode"`
	DisplayCollectionsView      bool     `json:"DisplayCollectionsView"`
	EnableLocalPassword         bool     `json:"EnableLocalPassword"`
	OrderedViews                []string `json:"OrderedViews"`
	LatestItemsExcludes         []string `json:"LatestItemsExcludes"`
	MyMediaExcludes             []string `json:"MyMediaExcludes"`
	HidePlayedInLatest          bool     `json:"HidePlayedInLatest"`
	RememberAudioSelections     bool     `json:"RememberAudioSelections"`
	RememberSubtitleSelections  bool     `json:"RememberSubtitleSelections"`
	EnableNextEpisodeAutoPlay   bool     `json:"EnableNextEpisodeAutoPlay"`
}

// UserPolicy 用户权限策略。
type UserPolicy struct {
	IsAdministrator                  bool     `json:"IsAdministrator"`
	IsHidden                         bool     `json:"IsHidden"`
	IsDisabled                       bool     `json:"IsDisabled"`
	EnableUserPreferenceAccess       bool     `json:"EnableUserPreferenceAccess"`
	EnableRemoteControlOfOtherUsers  bool     `json:"EnableRemoteControlOfOtherUsers"`
	EnableSharedDeviceControl        bool     `json:"EnableSharedDeviceControl"`
	EnableRemoteAccess               bool     `json:"EnableRemoteAccess"`
	EnableLiveTvManagement           bool     `json:"EnableLiveTvManagement"`
	EnableLiveTvAccess               bool     `json:"EnableLiveTvAccess"`
	EnableMediaPlayback              bool     `json:"EnableMediaPlayback"`
	EnableAudioPlaybackTranscoding   bool     `json:"EnableAudioPlaybackTranscoding"`
	EnableVideoPlaybackTranscoding   bool     `json:"EnableVideoPlaybackTranscoding"`
	EnablePlaybackRemuxing           bool     `json:"EnablePlaybackRemuxing"`
	EnableContentDeletion            bool     `json:"EnableContentDeletion"`
	EnableContentDownloading         bool     `json:"EnableContentDownloading"`
	EnableSubtitleDownloading        bool     `json:"EnableSubtitleDownloading"`
	EnableSubtitleManagement         bool     `json:"EnableSubtitleManagement"`
	EnableSyncTranscoding            bool     `json:"EnableSyncTranscoding"`
	EnableMediaConversion            bool     `json:"EnableMediaConversion"`
	EnableAllDevices                 bool     `json:"EnableAllDevices"`
	EnableAllChannels                bool     `json:"EnableAllChannels"`
	EnableAllFolders                 bool     `json:"EnableAllFolders"`
	InvalidLoginAttemptCount         int      `json:"InvalidLoginAttemptCount"`
	EnablePublicSharing              bool     `json:"EnablePublicSharing"`
	BlockedTags                      []string `json:"BlockedTags"`
	EnabledChannels                  []string `json:"EnabledChannels"`
	EnabledFolders                   []string `json:"EnabledFolders"`
	EnabledDevices                   []string `json:"EnabledDevices"`
	AuthenticationProviderId         string   `json:"AuthenticationProviderId"`
}

// SessionInfo 对应 /Sessions 项目。
type SessionInfo struct {
	Id                    string   `json:"Id"`
	UserId                string   `json:"UserId"`
	UserName              string   `json:"UserName"`
	Client                string   `json:"Client"`
	DeviceName            string   `json:"DeviceName"`
	DeviceId              string   `json:"DeviceId"`
	ApplicationVersion    string   `json:"ApplicationVersion"`
	IsActive              bool     `json:"IsActive"`
	ServerId              string   `json:"ServerId"`
	SupportsRemoteControl bool     `json:"SupportsRemoteControl"`
	PlayableMediaTypes    []string `json:"PlayableMediaTypes"`
	SupportedCommands     []string `json:"SupportedCommands"`
	LastActivityDate      string   `json:"LastActivityDate"`
}

// QueryResult 分页包装。
type QueryResult[T any] struct {
	Items            []T `json:"Items"`
	TotalRecordCount int `json:"TotalRecordCount"`
	StartIndex       int `json:"StartIndex"`
}

// BaseItemDto 是 Emby 中几乎所有媒体条目共用的核心 DTO，
// 本实现只填充 Infuse/Emby 客户端真正会读的关键字段。
type BaseItemDto struct {
	Name              string                `json:"Name"`
	OriginalTitle     string                `json:"OriginalTitle,omitempty"`
	ServerId          string                `json:"ServerId"`
	Id                string                `json:"Id"`
	Etag              string                `json:"Etag,omitempty"`
	DateCreated       string                `json:"DateCreated,omitempty"`
	CanDelete         bool                  `json:"CanDelete"`
	CanDownload       bool                  `json:"CanDownload"`
	SortName          string                `json:"SortName,omitempty"`
	PremiereDate      string                `json:"PremiereDate,omitempty"`
	ExternalUrls      []ExternalURL         `json:"ExternalUrls,omitempty"`
	Path              string                `json:"Path,omitempty"`
	OfficialRating    string                `json:"OfficialRating,omitempty"`
	Overview          string                `json:"Overview,omitempty"`
	Taglines          []string              `json:"Taglines,omitempty"`
	Genres            []string              `json:"Genres,omitempty"`
	CommunityRating   float64               `json:"CommunityRating,omitempty"`
	RunTimeTicks      int64                 `json:"RunTimeTicks,omitempty"`
	ProductionYear    int                   `json:"ProductionYear,omitempty"`
	IndexNumber       int                   `json:"IndexNumber,omitempty"`
	ParentIndexNumber int                   `json:"ParentIndexNumber,omitempty"`
	ProviderIds       map[string]string     `json:"ProviderIds,omitempty"`
	IsFolder          bool                  `json:"IsFolder"`
	ParentId          string                `json:"ParentId,omitempty"`
	Type              string                `json:"Type"`
	Studios           []NameIdPair          `json:"Studios,omitempty"`
	People            []BaseItemPerson      `json:"People,omitempty"`
	GenreItems        []NameIdPair          `json:"GenreItems,omitempty"`
	LocalTrailerCount int                   `json:"LocalTrailerCount,omitempty"`
	UserData          *UserItemData         `json:"UserData,omitempty"`
	ChildCount        int                   `json:"ChildCount,omitempty"`
	SeriesName        string                `json:"SeriesName,omitempty"`
	SeriesId          string                `json:"SeriesId,omitempty"`
	SeasonId          string                `json:"SeasonId,omitempty"`
	SeasonName        string                `json:"SeasonName,omitempty"`
	MediaStreams      []MediaStream         `json:"MediaStreams,omitempty"`
	MediaSources      []MediaSourceInfo     `json:"MediaSources,omitempty"`
	ImageTags         map[string]string     `json:"ImageTags,omitempty"`
	BackdropImageTags []string              `json:"BackdropImageTags,omitempty"`
	Width             int                   `json:"Width,omitempty"`
	Height            int                   `json:"Height,omitempty"`
	Container         string                `json:"Container,omitempty"`
	CollectionType    string                `json:"CollectionType,omitempty"`
	LocationType      string                `json:"LocationType,omitempty"`
	MediaType         string                `json:"MediaType,omitempty"`
	Chapters          []ChapterInfo         `json:"Chapters,omitempty"`
}

// ExternalURL 外部链接（如 TMDb 页面）。
type ExternalURL struct {
	Name string `json:"Name"`
	Url  string `json:"Url"`
}

// NameIdPair 用于 Genres/Studios 的 id-name 对。
type NameIdPair struct {
	Name string `json:"Name"`
	Id   string `json:"Id"`
}

// BaseItemPerson 演员/导演等。
type BaseItemPerson struct {
	Name            string            `json:"Name"`
	Id              string            `json:"Id"`
	Role            string            `json:"Role,omitempty"`
	Type            string            `json:"Type"` // Actor / Director / Writer
	PrimaryImageTag string            `json:"PrimaryImageTag,omitempty"`
	ProviderIds     map[string]string `json:"ProviderIds,omitempty"`
}

// UserItemData 用户针对单个媒体条目的观看状态。
type UserItemData struct {
	PlaybackPositionTicks int64   `json:"PlaybackPositionTicks"`
	PlayCount             int     `json:"PlayCount"`
	IsFavorite            bool    `json:"IsFavorite"`
	Played                bool    `json:"Played"`
	Key                   string  `json:"Key,omitempty"`
	PlayedPercentage      float64 `json:"PlayedPercentage,omitempty"`
	LastPlayedDate        string  `json:"LastPlayedDate,omitempty"`
}

// MediaStream 单条媒体流信息（video/audio/subtitle）。
type MediaStream struct {
	Codec             string  `json:"Codec"`
	Language          string  `json:"Language,omitempty"`
	DisplayTitle      string  `json:"DisplayTitle,omitempty"`
	TimeBase          string  `json:"TimeBase,omitempty"`
	VideoRange        string  `json:"VideoRange,omitempty"`
	DisplayLanguage   string  `json:"DisplayLanguage,omitempty"`
	IsInterlaced      bool    `json:"IsInterlaced"`
	IsDefault         bool    `json:"IsDefault"`
	IsForced          bool    `json:"IsForced"`
	Type              string  `json:"Type"` // Video / Audio / Subtitle
	AspectRatio       string  `json:"AspectRatio,omitempty"`
	Index             int     `json:"Index"`
	IsExternal        bool    `json:"IsExternal"`
	IsTextSubtitleStream bool `json:"IsTextSubtitleStream"`
	SupportsExternalStream bool `json:"SupportsExternalStream"`
	Protocol          string  `json:"Protocol,omitempty"`
	BitRate           int     `json:"BitRate,omitempty"`
	Channels          int     `json:"Channels,omitempty"`
	SampleRate        int     `json:"SampleRate,omitempty"`
	Width             int     `json:"Width,omitempty"`
	Height            int     `json:"Height,omitempty"`
	AverageFrameRate  float64 `json:"AverageFrameRate,omitempty"`
	RealFrameRate     float64 `json:"RealFrameRate,omitempty"`
	Profile           string  `json:"Profile,omitempty"`
	Level             float64 `json:"Level,omitempty"`
	PixelFormat       string  `json:"PixelFormat,omitempty"`
	DeliveryUrl       string  `json:"DeliveryUrl,omitempty"`
}

// MediaSourceInfo 对应 MediaSource（一个 Item 可有多个源，如多版本）。
type MediaSourceInfo struct {
	Protocol              string        `json:"Protocol"` // File / Http
	Id                    string        `json:"Id"`
	Path                  string        `json:"Path,omitempty"`
	Type                  string        `json:"Type"` // Default / Grouping / Placeholder
	Container             string        `json:"Container,omitempty"`
	Size                  int64         `json:"Size,omitempty"`
	Name                  string        `json:"Name,omitempty"`
	IsRemote              bool          `json:"IsRemote"`
	RunTimeTicks          int64         `json:"RunTimeTicks,omitempty"`
	SupportsTranscoding   bool          `json:"SupportsTranscoding"`
	SupportsDirectStream  bool          `json:"SupportsDirectStream"`
	SupportsDirectPlay    bool          `json:"SupportsDirectPlay"`
	IsInfiniteStream      bool          `json:"IsInfiniteStream"`
	RequiresOpening       bool          `json:"RequiresOpening"`
	RequiresClosing       bool          `json:"RequiresClosing"`
	RequiresLooping       bool          `json:"RequiresLooping"`
	SupportsProbing       bool          `json:"SupportsProbing"`
	MediaStreams          []MediaStream `json:"MediaStreams"`
	Formats               []string      `json:"Formats,omitempty"`
	Bitrate               int           `json:"Bitrate,omitempty"`
	ReadAtNativeFramerate bool          `json:"ReadAtNativeFramerate"`
	DirectStreamUrl       string        `json:"DirectStreamUrl,omitempty"`
	TranscodingUrl        string        `json:"TranscodingUrl,omitempty"`
	TranscodingSubProtocol string       `json:"TranscodingSubProtocol,omitempty"`
	TranscodingContainer  string        `json:"TranscodingContainer,omitempty"`
	ETag                  string        `json:"ETag,omitempty"`
}

// ChapterInfo 章节。
type ChapterInfo struct {
	StartPositionTicks int64  `json:"StartPositionTicks"`
	Name               string `json:"Name"`
	ImageTag           string `json:"ImageTag,omitempty"`
}

// PlaybackInfoResponse 对应 /Items/{id}/PlaybackInfo 的响应。
type PlaybackInfoResponse struct {
	MediaSources  []MediaSourceInfo `json:"MediaSources"`
	PlaySessionId string            `json:"PlaySessionId"`
	ErrorCode     string            `json:"ErrorCode,omitempty"`
}

// ==================== 辅助常量与工具 ====================

// TicksPerSecond Emby 内部时间单位：1 tick = 100 纳秒。
const TicksPerSecond int64 = 10_000_000

// secondsToTicks 秒 → Emby ticks。
func secondsToTicks(s float64) int64 {
	if s <= 0 {
		return 0
	}
	return int64(s * float64(TicksPerSecond))
}

// minutesToTicks 分钟 → Emby ticks。
func minutesToTicks(m int) int64 {
	if m <= 0 {
		return 0
	}
	return int64(m) * 60 * TicksPerSecond
}

// ticksToSeconds Emby ticks → 秒。
func ticksToSeconds(t int64) float64 {
	if t <= 0 {
		return 0
	}
	return float64(t) / float64(TicksPerSecond)
}

// formatEmbyTime 将 time.Time 转为 Emby 客户端期望的 ISO8601 时间字符串。
func formatEmbyTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05.0000000Z")
}
