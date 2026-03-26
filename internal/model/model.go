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
	// V2 扩展字段
	TMDbID     int    `json:"tmdb_id" gorm:"index"`         // TMDb 唯一 ID
	DoubanID   string `json:"douban_id" gorm:"type:text"`   // 豆瓣 ID
	BangumiID  int    `json:"bangumi_id" gorm:"index"`      // Bangumi 条目 ID
	Country    string `json:"country" gorm:"type:text"`     // 制片国家
	Language   string `json:"language" gorm:"type:text"`    // 语言
	Tagline    string `json:"tagline" gorm:"type:text"`     // 标语/宣传语
	Studio     string `json:"studio" gorm:"type:text"`      // 出品公司
	TrailerURL string `json:"trailer_url" gorm:"type:text"` // 预告片链接（YouTube）
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
	)
}
