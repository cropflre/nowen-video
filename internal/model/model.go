package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID        string         `json:"id" gorm:"primaryKey;type:text"`
	Username  string         `json:"username" gorm:"uniqueIndex;type:text;not null"`
	Password  string         `json:"-" gorm:"type:text;not null"`        // bcrypt哈希，JSON不输出
	Role      string         `json:"role" gorm:"type:text;default:user"` // admin / user
	Avatar    string         `json:"avatar" gorm:"type:text"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// Library 媒体库
type Library struct {
	ID       string     `json:"id" gorm:"primaryKey;type:text"`
	Name     string     `json:"name" gorm:"type:text;not null"`
	Path     string     `json:"path" gorm:"type:text;not null"`      // 媒体文件目录路径
	Type     string     `json:"type" gorm:"type:text;default:movie"` // movie / tvshow / mixed / other
	LastScan *time.Time `json:"last_scan"`
	// 高级设置
	PreferLocalNFO    bool   `json:"prefer_local_nfo" gorm:"default:true"`         // 优先读取本地NFO和图片
	MinFileSize       int    `json:"min_file_size" gorm:"default:3"`               // 排除小于此大小(MB)的视频文件
	EnableFileFilter  bool   `json:"enable_file_filter" gorm:"default:true"`       // 启用文件过滤
	MetadataLang      string `json:"metadata_lang" gorm:"type:text;default:zh-CN"` // 媒体元数据下载语言
	AllowAdultContent bool   `json:"allow_adult_content" gorm:"default:false"`     // 允许成人内容
	AutoDownloadSub   bool   `json:"auto_download_sub" gorm:"default:false"`       // 自动下载字幕
	// 实时文件监控（媒体库级别设置）
	EnableFileWatch bool `json:"enable_file_watch" gorm:"default:false"` // 启用实时文件监控
	// 时间戳
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// Series 剧集合集（电视剧系列）
type Series struct {
	ID           string  `json:"id" gorm:"primaryKey;type:text"`
	LibraryID    string  `json:"library_id" gorm:"index;type:text;not null"`
	Title        string  `json:"title" gorm:"index;type:text;not null"` // 剧集名称
	OrigTitle    string  `json:"orig_title" gorm:"type:text"`           // 原始标题
	Year         int     `json:"year" gorm:"index"`
	Overview     string  `json:"overview" gorm:"type:text"`
	PosterPath   string  `json:"poster_path" gorm:"type:text"`
	BackdropPath string  `json:"backdrop_path" gorm:"type:text"`
	Rating       float64 `json:"rating"`
	Genres       string  `json:"genres" gorm:"type:text"`
	FolderPath   string  `json:"folder_path" gorm:"uniqueIndex;type:text;not null"` // 剧集根目录路径
	SeasonCount  int     `json:"season_count"`                                      // 季数
	EpisodeCount int     `json:"episode_count"`                                     // 总集数
	// V2 扩展字段
	TMDbID    int    `json:"tmdb_id" gorm:"index"`
	IMDbID    string `json:"imdb_id" gorm:"index;type:text"` // IMDB ID (tt开头)
	DoubanID  string `json:"douban_id" gorm:"type:text"`
	BangumiID int    `json:"bangumi_id" gorm:"index"` // Bangumi 条目 ID
	Country   string `json:"country" gorm:"type:text"`
	Language  string `json:"language" gorm:"type:text"`
	Studio    string `json:"studio" gorm:"type:text"`
	// 时间戳
	CreatedAt time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Library  Library `json:"-" gorm:"foreignKey:LibraryID"`
	Episodes []Media `json:"episodes,omitempty" gorm:"foreignKey:SeriesID"`
}

func (s *Series) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// Media 媒体项（电影/剧集）
type Media struct {
	ID           string  `json:"id" gorm:"primaryKey;type:text"`
	LibraryID    string  `json:"library_id" gorm:"index;type:text;not null"`
	Title        string  `json:"title" gorm:"index;type:text;not null"`
	OrigTitle    string  `json:"orig_title" gorm:"type:text"` // 原始标题
	Year         int     `json:"year" gorm:"index"`
	Overview     string  `json:"overview" gorm:"type:text"`
	PosterPath   string  `json:"poster_path" gorm:"type:text"`   // 海报图片路径
	BackdropPath string  `json:"backdrop_path" gorm:"type:text"` // 背景图路径
	Rating       float64 `json:"rating"`
	Runtime      int     `json:"runtime"`                             // 时长（分钟）
	Genres       string  `json:"genres" gorm:"type:text"`             // 逗号分隔的类型
	FilePath     string  `json:"file_path" gorm:"type:text;not null"` // 视频文件绝对路径
	FileSize     int64   `json:"file_size"`
	MediaType    string  `json:"media_type" gorm:"type:text;default:movie"` // movie / episode
	// 视频信息
	VideoCodec string  `json:"video_codec" gorm:"type:text"`
	AudioCodec string  `json:"audio_codec" gorm:"type:text"`
	Resolution string  `json:"resolution" gorm:"type:text"` // 1080p, 4K 等
	Duration   float64 `json:"duration"`                    // 时长（秒）
	// 字幕
	SubtitlePaths string `json:"subtitle_paths" gorm:"type:text"` // 外挂字幕路径，| 分隔
	// STRM 远程流支持
	StreamURL string `json:"stream_url" gorm:"type:text"` // .strm 文件中的远程流地址（为空表示本地文件）
	// V2 扩展字段
	TMDbID     int    `json:"tmdb_id" gorm:"index"`           // TMDb 唯一 ID
	IMDbID     string `json:"imdb_id" gorm:"index;type:text"` // IMDB ID (tt开头)
	DoubanID   string `json:"douban_id" gorm:"type:text"`     // 豆瓣 ID
	BangumiID  int    `json:"bangumi_id" gorm:"index"`        // Bangumi 条目 ID
	Country    string `json:"country" gorm:"type:text"`       // 制片国家
	Language   string `json:"language" gorm:"type:text"`      // 语言
	Tagline    string `json:"tagline" gorm:"type:text"`       // 标语/宣传语
	Studio     string `json:"studio" gorm:"type:text"`        // 出品公司
	TrailerURL string `json:"trailer_url" gorm:"type:text"`   // 预告片链接（YouTube）
	// 多CD堆叠 & 多版本聚合（P2）
	StackGroup   string `json:"stack_group" gorm:"index;type:text"`   // 堆叠组 ID（cd1/cd2 共享同一组 ID）
	StackOrder   int    `json:"stack_order"`                          // 堆叠顺序（1=cd1, 2=cd2...）
	VersionTag   string `json:"version_tag" gorm:"type:text"`         // 版本标识（"4K", "Director's Cut" 等）
	VersionGroup string `json:"version_group" gorm:"index;type:text"` // 同一内容的不同版本共享此 ID
	// 刮削状态追踪（P3）
	ScrapeStatus   string     `json:"scrape_status" gorm:"type:text;default:pending"` // pending / scraped / failed / manual
	ScrapeAttempts int        `json:"scrape_attempts"`                                // 刮削尝试次数
	LastScrapeAt   *time.Time `json:"last_scrape_at"`                                 // 最后一次刮削时间
	// 剧集专属字段
	SeriesID     string `json:"series_id" gorm:"index;type:text"`
	SeasonNum    int    `json:"season_num"`
	EpisodeNum   int    `json:"episode_num"`
	EpisodeTitle string `json:"episode_title" gorm:"type:text"` // 单集标题（如有）
	// 时间戳
	CreatedAt time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Library Library `json:"-" gorm:"foreignKey:LibraryID"`
	Series  *Series `json:"series,omitempty" gorm:"foreignKey:SeriesID"`
}

// Person 演职人员
type Person struct {
	ID         string `json:"id" gorm:"primaryKey;type:text"`
	Name       string `json:"name" gorm:"index;type:text;not null"`
	OrigName   string `json:"orig_name" gorm:"type:text"`
	ProfileURL string `json:"profile_url" gorm:"type:text"` // 头像路径
	TMDbID     int    `json:"tmdb_id" gorm:"index"`
	// 时间戳
	CreatedAt time.Time `json:"created_at"`
}

func (p *Person) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// MediaPerson 媒体-人物关联表
type MediaPerson struct {
	ID        string `json:"id" gorm:"primaryKey;type:text"`
	MediaID   string `json:"media_id" gorm:"index;type:text;not null"`
	SeriesID  string `json:"series_id" gorm:"index;type:text"` // 也可以关联到 Series
	PersonID  string `json:"person_id" gorm:"index;type:text;not null"`
	Role      string `json:"role" gorm:"type:text;not null"` // director / actor / writer
	Character string `json:"character" gorm:"type:text"`     // 饰演角色名
	SortOrder int    `json:"sort_order" gorm:"default:0"`
	// 时间戳
	CreatedAt time.Time `json:"created_at"`

	Person Person `json:"person" gorm:"foreignKey:PersonID"`
}

func (mp *MediaPerson) BeforeCreate(tx *gorm.DB) error {
	if mp.ID == "" {
		mp.ID = uuid.New().String()
	}
	return nil
}

// WatchHistory 观看记录
type WatchHistory struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	UserID    string    `json:"user_id" gorm:"index;type:text;not null"`
	MediaID   string    `json:"media_id" gorm:"index;type:text;not null"`
	Position  float64   `json:"position"`  // 观看进度（秒）
	Duration  float64   `json:"duration"`  // 总时长（秒）
	Completed bool      `json:"completed"` // 是否看完
	UpdatedAt time.Time `json:"updated_at" gorm:"index"`
	CreatedAt time.Time `json:"created_at"`

	User  User  `json:"-" gorm:"foreignKey:UserID"`
	Media Media `json:"media" gorm:"foreignKey:MediaID"`
}

// Favorite 收藏
type Favorite struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	UserID    string    `json:"user_id" gorm:"index;type:text;not null"`
	MediaID   string    `json:"media_id" gorm:"index;type:text;not null"`
	CreatedAt time.Time `json:"created_at"`

	User  User  `json:"-" gorm:"foreignKey:UserID"`
	Media Media `json:"media" gorm:"foreignKey:MediaID"`
}

// TranscodeTask 转码任务
type TranscodeTask struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	MediaID   string    `json:"media_id" gorm:"index;type:text;not null"`
	Status    string    `json:"status" gorm:"type:text;default:pending"` // pending / running / done / failed
	Quality   string    `json:"quality" gorm:"type:text"`                // 720p / 1080p / 4k
	Progress  float64   `json:"progress"`                                // 0-100
	OutputDir string    `json:"output_dir" gorm:"type:text"`
	Error     string    `json:"error" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate 自动生成UUID
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

func (l *Library) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}

func (m *Media) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

func (w *WatchHistory) BeforeCreate(tx *gorm.DB) error {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	return nil
}

func (f *Favorite) BeforeCreate(tx *gorm.DB) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	return nil
}

func (t *TranscodeTask) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// Playlist 自定义播放列表
type Playlist struct {
	ID        string         `json:"id" gorm:"primaryKey;type:text"`
	UserID    string         `json:"user_id" gorm:"index;type:text;not null"`
	Name      string         `json:"name" gorm:"type:text;not null"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	User  User           `json:"-" gorm:"foreignKey:UserID"`
	Items []PlaylistItem `json:"items" gorm:"foreignKey:PlaylistID"`
}

// PlaylistItem 播放列表项
type PlaylistItem struct {
	ID         string    `json:"id" gorm:"primaryKey;type:text"`
	PlaylistID string    `json:"playlist_id" gorm:"index;type:text;not null"`
	MediaID    string    `json:"media_id" gorm:"index;type:text;not null"`
	SortOrder  int       `json:"sort_order" gorm:"default:0"`
	CreatedAt  time.Time `json:"created_at"`

	Media Media `json:"media" gorm:"foreignKey:MediaID"`
}

func (p *Playlist) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

func (pi *PlaylistItem) BeforeCreate(tx *gorm.DB) error {
	if pi.ID == "" {
		pi.ID = uuid.New().String()
	}
	return nil
}

// Bookmark 视频书签
type Bookmark struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	UserID    string    `json:"user_id" gorm:"index;type:text;not null"`
	MediaID   string    `json:"media_id" gorm:"index;type:text;not null"`
	Position  float64   `json:"position"`                        // 书签时间点（秒）
	Title     string    `json:"title" gorm:"type:text;not null"` // 书签标题
	Note      string    `json:"note" gorm:"type:text"`           // 备注
	CreatedAt time.Time `json:"created_at"`

	User  User  `json:"-" gorm:"foreignKey:UserID"`
	Media Media `json:"media,omitempty" gorm:"foreignKey:MediaID"`
}

func (b *Bookmark) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}

// Comment 评论
type Comment struct {
	ID        string         `json:"id" gorm:"primaryKey;type:text"`
	UserID    string         `json:"user_id" gorm:"index;type:text;not null"`
	MediaID   string         `json:"media_id" gorm:"index;type:text;not null"`
	Content   string         `json:"content" gorm:"type:text;not null"`
	Rating    float64        `json:"rating"` // 用户评分（0-10）
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	User  User  `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Media Media `json:"-" gorm:"foreignKey:MediaID"`
}

func (c *Comment) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

// AccessLog 访问日志
type AccessLog struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	UserID    string    `json:"user_id" gorm:"index;type:text;not null"`
	Username  string    `json:"username" gorm:"type:text"`
	Action    string    `json:"action" gorm:"type:text;not null"` // login, play, scrape, admin_op等
	Resource  string    `json:"resource" gorm:"type:text"`        // 操作的资源
	Detail    string    `json:"detail" gorm:"type:text"`          // 操作详情
	IP        string    `json:"ip" gorm:"type:text"`
	UserAgent string    `json:"user_agent" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

func (a *AccessLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// ScheduledTask 定时任务
type ScheduledTask struct {
	ID        string     `json:"id" gorm:"primaryKey;type:text"`
	Name      string     `json:"name" gorm:"type:text;not null"`     // 任务名称
	Type      string     `json:"type" gorm:"type:text;not null"`     // scan, scrape, cleanup
	Schedule  string     `json:"schedule" gorm:"type:text;not null"` // cron表达式或间隔，如 "@every 6h", "0 2 * * *"
	TargetID  string     `json:"target_id" gorm:"type:text"`         // 目标ID（如媒体库ID）
	Enabled   bool       `json:"enabled" gorm:"default:true"`
	LastRun   *time.Time `json:"last_run"`
	NextRun   *time.Time `json:"next_run"`
	Status    string     `json:"status" gorm:"type:text;default:idle"` // idle, running, error
	LastError string     `json:"last_error" gorm:"type:text"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (s *ScheduledTask) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// ContentRating 内容分级
type ContentRating struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	MediaID   string    `json:"media_id" gorm:"uniqueIndex;type:text;not null"`
	Level     string    `json:"level" gorm:"type:text;not null"` // G, PG, PG-13, R, NC-17
	CreatedAt time.Time `json:"created_at"`
}

func (cr *ContentRating) BeforeCreate(tx *gorm.DB) error {
	if cr.ID == "" {
		cr.ID = uuid.New().String()
	}
	return nil
}

// UserPermission 用户权限设置
type UserPermission struct {
	ID               string    `json:"id" gorm:"primaryKey;type:text"`
	UserID           string    `json:"user_id" gorm:"uniqueIndex;type:text;not null"`
	AllowedLibraries string    `json:"allowed_libraries" gorm:"type:text"`              // 允许访问的媒体库ID，逗号分隔，空表示全部
	MaxRatingLevel   string    `json:"max_rating_level" gorm:"type:text;default:NC-17"` // 最高允许观看的分级
	DailyTimeLimit   int       `json:"daily_time_limit" gorm:"default:0"`               // 每日观看时长限制（分钟），0表示不限
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	User User `json:"-" gorm:"foreignKey:UserID"`
}

func (up *UserPermission) BeforeCreate(tx *gorm.DB) error {
	if up.ID == "" {
		up.ID = uuid.New().String()
	}
	return nil
}

// SystemSetting 系统全局设置（KV 键值对存储）
type SystemSetting struct {
	Key       string    `json:"key" gorm:"primaryKey;type:text"`
	Value     string    `json:"value" gorm:"type:text"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PlaybackStats 播放统计
type PlaybackStats struct {
	ID           string    `json:"id" gorm:"primaryKey;type:text"`
	UserID       string    `json:"user_id" gorm:"index;type:text;not null"`
	MediaID      string    `json:"media_id" gorm:"index;type:text;not null"`
	WatchMinutes float64   `json:"watch_minutes"`               // 本次观看分钟数
	Date         string    `json:"date" gorm:"index;type:text"` // YYYY-MM-DD 格式
	CreatedAt    time.Time `json:"created_at"`
}

func (ps *PlaybackStats) BeforeCreate(tx *gorm.DB) error {
	if ps.ID == "" {
		ps.ID = uuid.New().String()
	}
	return nil
}

// AICacheEntry AI 缓存持久化条目（替代内存缓存，重启不丢失）
type AICacheEntry struct {
	CacheKey  string    `json:"cache_key" gorm:"primaryKey;type:text"`
	Value     string    `json:"value" gorm:"type:text"`
	ExpiresAt time.Time `json:"expires_at" gorm:"index"`
	CreatedAt time.Time `json:"created_at"`
}

// GenreMapping 类型标签统一映射表（标准化不同数据源的标签）
type GenreMapping struct {
	ID           string `json:"id" gorm:"primaryKey;type:text"`
	SourceGenre  string `json:"source_genre" gorm:"uniqueIndex:idx_source_genre;type:text;not null"` // 原始标签（如 "Sci-Fi"）
	SourceType   string `json:"source_type" gorm:"uniqueIndex:idx_source_genre;type:text;not null"`  // 来源（tmdb/douban/bangumi/ai）
	StandardName string `json:"standard_name" gorm:"index;type:text;not null"`                       // 标准化名称（如 "科幻"）
}

func (g *GenreMapping) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return nil
}

// RecommendCache 推荐结果缓存（避免每次重建评分矩阵）
type RecommendCache struct {
	UserID    string    `json:"user_id" gorm:"primaryKey;type:text"`
	Results   string    `json:"results" gorm:"type:text"` // JSON 序列化的推荐结果
	ExpiresAt time.Time `json:"expires_at" gorm:"index"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ==================== P2: 标签管理系统 ====================

// Tag 标签
type Tag struct {
	ID         string    `json:"id" gorm:"primaryKey;type:text"`
	Name       string    `json:"name" gorm:"uniqueIndex;type:text;not null"` // 标签名称（唯一）
	Color      string    `json:"color" gorm:"type:text;default:#3b82f6"`     // 标签颜色（十六进制）
	Icon       string    `json:"icon" gorm:"type:text"`                      // 标签图标（可选）
	Category   string    `json:"category" gorm:"index;type:text"`            // 标签分类（如：类型、心情、场景）
	SortOrder  int       `json:"sort_order" gorm:"default:0"`                // 排序权重
	UsageCount int       `json:"usage_count" gorm:"default:0"`               // 使用次数（冗余计数，加速排序）
	CreatedBy  string    `json:"created_by" gorm:"type:text"`                // 创建者用户ID
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (t *Tag) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// MediaTag 媒体-标签关联表
type MediaTag struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	MediaID   string    `json:"media_id" gorm:"uniqueIndex:idx_media_tag;type:text;not null"`
	TagID     string    `json:"tag_id" gorm:"uniqueIndex:idx_media_tag;index;type:text;not null"`
	CreatedBy string    `json:"created_by" gorm:"type:text"` // 打标签的用户
	CreatedAt time.Time `json:"created_at"`

	Tag Tag `json:"tag" gorm:"foreignKey:TagID"`
}

func (mt *MediaTag) BeforeCreate(tx *gorm.DB) error {
	if mt.ID == "" {
		mt.ID = uuid.New().String()
	}
	return nil
}

// ==================== P2: 分享链接功能 ====================

// ShareLink 分享链接
type ShareLink struct {
	ID            string     `json:"id" gorm:"primaryKey;type:text"`
	Code          string     `json:"code" gorm:"uniqueIndex;type:text;not null"` // 短链接码（如 abc123）
	MediaID       string     `json:"media_id" gorm:"index;type:text"`            // 分享的媒体ID（与SeriesID二选一）
	SeriesID      string     `json:"series_id" gorm:"index;type:text"`           // 分享的剧集ID
	CreatedBy     string     `json:"created_by" gorm:"index;type:text;not null"` // 创建者用户ID
	Title         string     `json:"title" gorm:"type:text"`                     // 分享标题（可自定义）
	Description   string     `json:"description" gorm:"type:text"`               // 分享描述
	Password      string     `json:"password,omitempty" gorm:"type:text"`        // 访问密码（可选）
	MaxViews      int        `json:"max_views" gorm:"default:0"`                 // 最大访问次数（0=不限）
	ViewCount     int        `json:"view_count" gorm:"default:0"`                // 已访问次数
	AllowDownload bool       `json:"allow_download" gorm:"default:false"`        // 是否允许下载
	ExpiresAt     *time.Time `json:"expires_at"`                                 // 过期时间（nil=永不过期）
	IsActive      bool       `json:"is_active" gorm:"default:true"`              // 是否启用
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (sl *ShareLink) BeforeCreate(tx *gorm.DB) error {
	if sl.ID == "" {
		sl.ID = uuid.New().String()
	}
	return nil
}

// ==================== P3: 自定义匹配规则 ====================

// MatchRule 自定义匹配规则
type MatchRule struct {
	ID          string    `json:"id" gorm:"primaryKey;type:text"`
	Name        string    `json:"name" gorm:"type:text;not null"`            // 规则名称
	Description string    `json:"description" gorm:"type:text"`              // 规则描述
	RuleType    string    `json:"rule_type" gorm:"index;type:text;not null"` // 规则类型：filename / path / regex / keyword
	Pattern     string    `json:"pattern" gorm:"type:text;not null"`         // 匹配模式（正则/关键词/路径模式）
	Action      string    `json:"action" gorm:"type:text;not null"`          // 动作：set_type / set_genre / set_tag / skip / set_library
	ActionValue string    `json:"action_value" gorm:"type:text"`             // 动作参数值
	Priority    int       `json:"priority" gorm:"default:0"`                 // 优先级（数字越大越先执行）
	Enabled     bool      `json:"enabled" gorm:"default:true"`               // 是否启用
	LibraryID   string    `json:"library_id" gorm:"index;type:text"`         // 限定媒体库（空=全局）
	HitCount    int       `json:"hit_count" gorm:"default:0"`                // 命中次数
	CreatedBy   string    `json:"created_by" gorm:"type:text"`               // 创建者
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (mr *MatchRule) BeforeCreate(tx *gorm.DB) error {
	if mr.ID == "" {
		mr.ID = uuid.New().String()
	}
	return nil
}

// AutoMigrate 自动迁移所有模型
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&Library{},
		&SystemSetting{},
		&Series{},
		&Media{},
		&Person{},
		&MediaPerson{},
		&WatchHistory{},
		&Favorite{},
		&TranscodeTask{},
		&Playlist{},
		&PlaylistItem{},
		&Bookmark{},
		&Comment{},
		&AccessLog{},
		&ScheduledTask{},
		&ContentRating{},
		&UserPermission{},
		&PlaybackStats{},
		&ScrapeTask{},
		&ScrapeHistory{},
		// V3: AI 场景识别与内容理解
		&VideoChapter{},
		&VideoHighlight{},
		&AIAnalysisTask{},
		// V3: AI 驱动的封面优化
		&CoverCandidate{},
		// V3: 家庭社交互动
		&FamilyGroup{},
		&FamilyMember{},
		&MediaShare{},
		&MediaLike{},
		&MediaRecommendation{},
		// V3: 实时直播扩展
		&LiveSource{},
		&LivePlaylist{},
		&LiveRecording{},
		// V3: 云端同步与多设备
		&SyncDevice{},
		&SyncRecord{},
		&UserSyncConfig{},
		// V4: 性能优化与标签统一
		&AICacheEntry{},
		&GenreMapping{},
		&RecommendCache{},
		// P2: 标签管理系统
		&Tag{},
		&MediaTag{},
		// P2: 分享链接功能
		&ShareLink{},
		// P3: 自定义匹配规则
		&MatchRule{},
	)
}
