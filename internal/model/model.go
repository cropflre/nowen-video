package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID       string `json:"id" gorm:"primaryKey;type:text"`
	Username string `json:"username" gorm:"uniqueIndex;type:text;not null"`
	Password string `json:"-" gorm:"type:text;not null"`        // bcrypt哈希，JSON不输出
	Role     string `json:"role" gorm:"type:text;default:user"` // admin / user
	Avatar   string `json:"avatar" gorm:"type:text"`
	// 扩展资料
	Nickname string `json:"nickname" gorm:"type:text"`
	Email    string `json:"email" gorm:"type:text;index"`
	// 账号状态
	Disabled      bool `json:"disabled" gorm:"default:false"`        // 是否被封禁
	MustChangePwd bool `json:"must_change_pwd" gorm:"default:false"` // 首次登录强制改密
	// 会话版本号：用于吊销旧 Token（密码/角色/封禁变更时自增）
	TokenVersion int `json:"-" gorm:"default:0"`
	// 最近登录
	LastLoginAt *time.Time     `json:"last_login_at"`
	LastLoginIP string         `json:"last_login_ip" gorm:"type:text"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// LoginLog 登录日志
type LoginLog struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	UserID    string    `json:"user_id" gorm:"index;type:text"`
	Username  string    `json:"username" gorm:"type:text"` // 冗余便于展示（用户删除后仍可回溯）
	IP        string    `json:"ip" gorm:"type:text"`
	UserAgent string    `json:"user_agent" gorm:"type:text"`
	Success   bool      `json:"success" gorm:"index"`
	Reason    string    `json:"reason" gorm:"type:text"` // 失败原因（password_error / user_disabled / user_not_found）
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

func (l *LoginLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}

// AuditLog 管理员操作审计
type AuditLog struct {
	ID         string    `json:"id" gorm:"primaryKey;type:text"`
	OperatorID string    `json:"operator_id" gorm:"index;type:text"`
	Operator   string    `json:"operator" gorm:"type:text"`     // 操作者用户名（冗余）
	Action     string    `json:"action" gorm:"index;type:text"` // user.create / user.delete / user.reset_password / user.disable ...
	TargetType string    `json:"target_type" gorm:"type:text"`  // user / library / media / system
	TargetID   string    `json:"target_id" gorm:"type:text"`
	Detail     string    `json:"detail" gorm:"type:text"` // JSON 或文本描述
	IP         string    `json:"ip" gorm:"type:text"`
	CreatedAt  time.Time `json:"created_at" gorm:"index"`
}

func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// InviteCode 邀请码（支持一码一用、过期、多次使用）
type InviteCode struct {
	ID        string     `json:"id" gorm:"primaryKey;type:text"`
	Code      string     `json:"code" gorm:"uniqueIndex;type:text;not null"`
	MaxUses   int        `json:"max_uses" gorm:"default:1"` // 最大使用次数，0 表示无限
	UsedCount int        `json:"used_count" gorm:"default:0"`
	ExpiresAt *time.Time `json:"expires_at"` // 过期时间（nil 表示永不过期）
	CreatorID string     `json:"creator_id" gorm:"type:text"`
	Note      string     `json:"note" gorm:"type:text"`
	CreatedAt time.Time  `json:"created_at" gorm:"index"`
}

func (i *InviteCode) BeforeCreate(tx *gorm.DB) error {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	return nil
}

// Library 媒体库
type Library struct {
	ID   string `json:"id" gorm:"primaryKey;type:text"`
	Name string `json:"name" gorm:"type:text;not null"`
	// Path 主路径（第一个媒体文件夹，保留以兼容历史数据）
	Path string `json:"path" gorm:"type:text;not null"`
	// ExtraPaths 额外媒体文件夹列表（JSON 数组），用于支持多个目录
	// 对外字段名为 extra_paths；聚合后的完整路径列表请使用 AllPaths()
	ExtraPaths string     `json:"extra_paths" gorm:"type:text"`
	Type       string     `json:"type" gorm:"type:text;default:movie"` // movie / tvshow / mixed / other
	LastScan   *time.Time `json:"last_scan"`
	// 高级设置
	PreferLocalNFO    bool   `json:"prefer_local_nfo" gorm:"default:true"`         // 优先读取本地NFO和图片
	MinFileSize       int    `json:"min_file_size" gorm:"default:3"`               // 排除小于此大小(MB)的视频文件
	EnableFileFilter  bool   `json:"enable_file_filter" gorm:"default:true"`       // 启用文件过滤
	MetadataLang      string `json:"metadata_lang" gorm:"type:text;default:zh-CN"` // 媒体元数据下载语言
	AllowAdultContent bool   `json:"allow_adult_content" gorm:"default:false"`     // 允许成人内容
	AutoDownloadSub   bool   `json:"auto_download_sub" gorm:"default:false"`       // 自动下载字幕
	// 扫描行为设置
	AutoScrapeMetadata bool `json:"auto_scrape_metadata" gorm:"default:true"` // 扫描后自动刮削元数据
	// 实时文件监控（媒体库级别设置）
	EnableFileWatch bool `json:"enable_file_watch" gorm:"default:false"` // 启用实时文件监控
	// 时间戳
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// AllPaths 返回媒体库的全部媒体文件夹路径（主路径 + ExtraPaths）
// 结果已去重、剔除空值；始终把 Path 作为第一个元素以保持主路径顺序
func (l *Library) AllPaths() []string {
	result := make([]string, 0, 4)
	seen := make(map[string]bool)
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		result = append(result, p)
	}
	add(l.Path)
	if strings.TrimSpace(l.ExtraPaths) != "" {
		var extras []string
		if err := json.Unmarshal([]byte(l.ExtraPaths), &extras); err == nil {
			for _, p := range extras {
				add(p)
			}
		}
	}
	return result
}

// SetAllPaths 根据完整路径数组写入 Path + ExtraPaths
// 第一个路径作为主路径（Path），其余序列化为 JSON 写入 ExtraPaths
func (l *Library) SetAllPaths(paths []string) {
	cleaned := make([]string, 0, len(paths))
	seen := make(map[string]bool)
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		cleaned = append(cleaned, p)
	}
	if len(cleaned) == 0 {
		// 保留原主路径以免破坏 not null 约束，调用方应在此之前完成校验
		l.ExtraPaths = ""
		return
	}
	l.Path = cleaned[0]
	if len(cleaned) == 1 {
		l.ExtraPaths = ""
		return
	}
	if data, err := json.Marshal(cleaned[1:]); err == nil {
		l.ExtraPaths = string(data)
	}
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
	// 刮削状态：pending / scraped / partial / failed / manual（C 方案新增）
	ScrapeStatus string     `json:"scrape_status" gorm:"type:text;default:pending;index"`
	LastScrapeAt *time.Time `json:"last_scrape_at,omitempty"`
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

// MovieCollection 电影系列合集（如"逃学威龙"系列、"速度与激情"系列）
type MovieCollection struct {
	ID          string    `json:"id" gorm:"primaryKey;type:text"`
	Name        string    `json:"name" gorm:"index;type:text;not null"` // 合集名称（如"逃学威龙"）
	Overview    string    `json:"overview" gorm:"type:text"`            // 合集简介
	PosterPath  string    `json:"poster_path" gorm:"type:text"`         // 合集海报（取第一部的海报）
	TMDbCollID  int       `json:"tmdb_coll_id" gorm:"index"`            // TMDb Collection ID
	MediaCount  int       `json:"media_count"`                          // 去重后的电影数量（同一部电影的不同版本算一部）
	FileCount   int       `json:"file_count"`                           // 原始文件总数（每个版本各算一个）
	AutoMatched bool      `json:"auto_matched" gorm:"default:true"`     // 是否自动匹配生成
	YearRange   string    `json:"year_range" gorm:"type:text"`          // 年份范围（如"1991-1993"或"2020"）
	CreatedAt   time.Time `json:"created_at" gorm:"index"`
	UpdatedAt   time.Time `json:"updated_at"`

	Media []Media `json:"media,omitempty" gorm:"foreignKey:CollectionID"`
}

func (mc *MovieCollection) BeforeCreate(tx *gorm.DB) error {
	if mc.ID == "" {
		mc.ID = uuid.New().String()
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
	// STRM 增强字段：由 .strm 同目录 .json / KODI #KODIPROP / M3U 扩展头 携带的请求参数
	StreamUA         string `json:"stream_ua" gorm:"type:text"`          // 自定义 User-Agent（某些云盘/HLS 源要求）
	StreamReferer    string `json:"stream_referer" gorm:"type:text"`     // 自定义 Referer
	StreamCookie     string `json:"stream_cookie" gorm:"type:text"`      // 自定义 Cookie（多条用分号分隔）
	StreamHeaders    string `json:"stream_headers" gorm:"type:text"`     // 额外自定义 Header，JSON: {"X-K":"V"}
	StreamRefreshURL string `json:"stream_refresh_url" gorm:"type:text"` // 刷新直链的上游 API（预留，留空表示不刷新）
	// V2 扩展字段
	TMDbID     int    `json:"tmdb_id" gorm:"index"`           // TMDb 唯一 ID
	IMDbID     string `json:"imdb_id" gorm:"index;type:text"` // IMDB ID (tt开头)
	DoubanID   string `json:"douban_id" gorm:"type:text"`     // 豆瓣 ID
	BangumiID  int    `json:"bangumi_id" gorm:"index"`        // Bangumi 条目 ID
	Premiered  string `json:"premiered" gorm:"type:text"`     // 首映日期（NFO YYYY-MM-DD）
	Country    string `json:"country" gorm:"type:text"`       // 制片国家
	Language   string `json:"language" gorm:"type:text"`      // 语言
	Tagline    string `json:"tagline" gorm:"type:text"`       // 标语/宣传语
	Studio     string `json:"studio" gorm:"type:text"`        // 出品公司
	TrailerURL string `json:"trailer_url" gorm:"type:text"`   // 预告片链接（YouTube）
	// NFO 扩展字段（方案 B 完整建模，用于番号/成人内容等精细展示）
	Num          string `json:"num" gorm:"index;type:text"`       // 番号（如 MIDD-835），取自 NFO <num>
	SortTitle    string `json:"sort_title" gorm:"type:text"`      // 排序用标题，取自 NFO <sorttitle>
	Outline      string `json:"outline" gorm:"type:text"`         // 剧情简要（短摘要），取自 NFO <outline>
	OriginalPlot string `json:"original_plot" gorm:"type:text"`   // 原始剧情（日文/原文），取自 NFO <originalplot>
	MPAA         string `json:"mpaa" gorm:"type:text"`            // 分级（如 JP-18+），取自 NFO <mpaa>/<customrating>
	CountryCode  string `json:"country_code" gorm:"type:text"`    // 国家代码（JP/CN/US），取自 NFO <countrycode>
	Maker        string `json:"maker" gorm:"index;type:text"`     // 制作商，取自 NFO <maker>
	Publisher    string `json:"publisher" gorm:"index;type:text"` // 发行商，取自 NFO <publisher>
	Label        string `json:"label" gorm:"index;type:text"`     // 厂牌，取自 NFO <label>
	Tags         string `json:"tags" gorm:"type:text"`            // 用户标签（逗号分隔），取自 NFO <tag>，与 genres 并列
	Website      string `json:"website" gorm:"type:text"`         // 官方网站，取自 NFO <website>
	ReleaseDate  string `json:"release_date" gorm:"type:text"`    // 发行日期（可独立于首映日期），取自 NFO <releasedate>
	// 多CD堆叠 & 多版本聚合（P2）
	StackGroup   string `json:"stack_group" gorm:"index;type:text"`   // 堆叠组 ID（cd1/cd2 共享同一组 ID）
	StackOrder   int    `json:"stack_order"`                          // 堆叠顺序（1=cd1, 2=cd2...）
	VersionTag   string `json:"version_tag" gorm:"type:text"`         // 版本标识（"4K", "Director's Cut" 等）
	VersionGroup string `json:"version_group" gorm:"index;type:text"` // 同一内容的不同版本共享此 ID
	// 刮削状态追踪（P3）
	ScrapeStatus   string     `json:"scrape_status" gorm:"type:text;default:pending"` // pending / scraped / failed / manual
	ScrapeAttempts int        `json:"scrape_attempts"`                                // 刮削尝试次数
	LastScrapeAt   *time.Time `json:"last_scrape_at"`                                 // 最后一次刮削时间
	// 电影系列合集
	CollectionID string `json:"collection_id" gorm:"index;type:text"` // 所属电影合集 ID
	// 重复媒体检测
	DuplicateOf    string `json:"duplicate_of" gorm:"index;type:text"`    // 重复的原始媒体 ID（为空表示非重复）
	DuplicateGroup string `json:"duplicate_group" gorm:"index;type:text"` // 重复组标识（相同标题+年份的媒体共享此标识）
	// 剧集专属字段
	SeriesID     string `json:"series_id" gorm:"index;type:text"`
	SeasonNum    int    `json:"season_num"`
	EpisodeNum   int    `json:"episode_num"`
	EpisodeTitle string `json:"episode_title" gorm:"type:text"` // 单集标题（如有）
	// 时间戳
	CreatedAt time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Library    Library          `json:"-" gorm:"foreignKey:LibraryID"`
	Series     *Series          `json:"series,omitempty" gorm:"foreignKey:SeriesID"`
	Collection *MovieCollection `json:"collection,omitempty" gorm:"foreignKey:CollectionID"`
}

// DisplayTitle 返回带集数信息的显示标题
// 对于剧集类型，格式为 "标题 S01E02" 或 "标题 S01E02 - 单集标题"
func (m *Media) DisplayTitle() string {
	if m.MediaType == "episode" && m.EpisodeNum > 0 {
		title := fmt.Sprintf("%s S%02dE%02d", m.Title, m.SeasonNum, m.EpisodeNum)
		if m.EpisodeTitle != "" {
			title += " - " + m.EpisodeTitle
		}
		return title
	}
	return m.Title
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
	ProfileID string    `json:"profile_id" gorm:"index;type:text"` // Profile 隔离（空=账号级历史，兼容旧数据）
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
	ProfileID string    `json:"profile_id" gorm:"index;type:text"` // Profile 隔离（空=账号级收藏，兼容旧数据）
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

// ScheduledTask 定时任务
type ScheduledTask struct {
	ID        string     `json:"id" gorm:"primaryKey;type:text"`
	Name      string     `json:"name" gorm:"type:text;not null"`     // 任务名称
	Type      string     `json:"type" gorm:"type:text;not null"`     // scan, scrape, cleanup
	Schedule  string     `json:"schedule" gorm:"type:text;not null"` // 调度表达式，仅支持：@daily / @weekly / @every 6h（不支持标准cron）
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

// ==================== 视频预处理任务 ====================
// PreprocessTask 视频预处理任务
type PreprocessTask struct {
	ID         string  `json:"id" gorm:"primaryKey;type:text"`
	MediaID    string  `json:"media_id" gorm:"index;type:text;not null"`
	Status     string  `json:"status" gorm:"type:text;default:pending"` // pending / queued / running / paused / completed / failed / cancelled
	Phase      string  `json:"phase" gorm:"type:text"`                  // probe / thumbnail / keyframes / transcode_360p / transcode_480p / transcode_720p / transcode_1080p / abr_master
	Progress   float64 `json:"progress"`                                // 0-100 总体进度
	Priority   int     `json:"priority" gorm:"default:0"`               // 优先级（数字越大越优先）
	Message    string  `json:"message" gorm:"type:text"`                // 当前状态描述
	Error      string  `json:"error" gorm:"type:text"`                  // 错误信息
	Retries    int     `json:"retries" gorm:"default:0"`                // 已重试次数
	MaxRetry   int     `json:"max_retry" gorm:"default:3"`              // 最大重试次数
	InputPath  string  `json:"input_path" gorm:"type:text"`             // 输入文件路径
	OutputDir  string  `json:"output_dir" gorm:"type:text"`             // 输出目录
	MediaTitle string  `json:"media_title" gorm:"type:text"`            // 媒体标题（冗余，方便展示）
	// 预处理结果
	ThumbnailPath  string  `json:"thumbnail_path" gorm:"type:text"`  // 封面缩略图路径
	KeyframesDir   string  `json:"keyframes_dir" gorm:"type:text"`   // 关键帧预览目录
	SpritePath     string  `json:"sprite_path" gorm:"type:text"`     // 进度条雪碧图路径
	SpriteVTTPath  string  `json:"sprite_vtt_path" gorm:"type:text"` // 进度条雪碧图 WebVTT 索引路径
	HLSMasterPath  string  `json:"hls_master_path" gorm:"type:text"` // HLS 主播放列表路径
	Variants       string  `json:"variants" gorm:"type:text"`        // 已完成的变体列表（JSON: ["360p","720p","1080p"]）
	SourceHeight   int     `json:"source_height"`                    // 源视频高度
	SourceWidth    int     `json:"source_width"`                     // 源视频宽度
	SourceCodec    string  `json:"source_codec" gorm:"type:text"`    // 源视频编码
	SourceDuration float64 `json:"source_duration"`                  // 源视频时长（秒）
	SourceSize     int64   `json:"source_size"`                      // 源文件大小（字节）
	// 性能统计
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	ElapsedSec  float64    `json:"elapsed_sec"` // 总耗时（秒）
	SpeedRatio  float64    `json:"speed_ratio"` // 转码速度比（如 2.5x）
	// 时间戳
	CreatedAt time.Time `json:"created_at" gorm:"index"`
	UpdatedAt time.Time `json:"updated_at"`

	Media Media `json:"-" gorm:"foreignKey:MediaID"`
}

func (t *PreprocessTask) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// ==================== 字幕预处理任务 ====================

// SubtitlePreprocessTask 字幕预处理任务
type SubtitlePreprocessTask struct {
	ID         string  `json:"id" gorm:"primaryKey;type:text"`
	MediaID    string  `json:"media_id" gorm:"index;type:text;not null"`
	Status     string  `json:"status" gorm:"type:text;default:pending"` // pending / running / completed / failed / cancelled / skipped
	Phase      string  `json:"phase" gorm:"type:text"`                  // check / extract / generate / clean / translate / done
	Progress   float64 `json:"progress"`                                // 0-100 总体进度
	Message    string  `json:"message" gorm:"type:text"`                // 当前状态描述
	Error      string  `json:"error" gorm:"type:text"`                  // 错误信息
	MediaTitle string  `json:"media_title" gorm:"type:text"`            // 媒体标题（冗余，方便展示）
	// 配置
	SourceLang      string `json:"source_lang" gorm:"type:text;default:auto"` // 源语言（auto 自动检测）
	TargetLangs     string `json:"target_langs" gorm:"type:text"`             // 目标翻译语言列表（逗号分隔: zh,en,ja）
	ForceRegenerate bool   `json:"force_regenerate"`                          // 是否强制重新生成
	// 结果
	OriginalVTTPath  string `json:"original_vtt_path" gorm:"type:text"` // 原始字幕 VTT 路径
	TranslatedPaths  string `json:"translated_paths" gorm:"type:text"`  // 翻译字幕路径（格式: lang=path|lang=path）
	SubtitleSource   string `json:"subtitle_source" gorm:"type:text"`   // 字幕来源: ai_cached / external_vtt / extracted / ai_generated
	DetectedLanguage string `json:"detected_language" gorm:"type:text"` // 检测到的源语言
	CueCount         int    `json:"cue_count"`                          // 字幕条目数
	// 增强字段
	FailedLangs     string `json:"failed_langs" gorm:"type:text"`      // 翻译失败的语言列表（逗号分隔），用于前端展示
	CleanReportJSON string `json:"clean_report_json" gorm:"type:text"` // 清洗详细报告（JSON 字符串）
	// 性能统计
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	ElapsedSec  float64    `json:"elapsed_sec"` // 总耗时（秒）
	// 时间戳
	CreatedAt time.Time `json:"created_at" gorm:"index"`
	UpdatedAt time.Time `json:"updated_at"`

	Media Media `json:"-" gorm:"foreignKey:MediaID"`
}

func (t *SubtitlePreprocessTask) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// ==================== 文件管理操作日志（持久化） ====================

// FileOperationLog 文件管理操作日志（持久化到数据库）
// 记录导入/编辑/删除/刮削/重命名/重分类等操作，支持审计与跨会话查询
type FileOperationLog struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	Action    string    `json:"action" gorm:"index;type:text"`   // import / edit / delete / scrape / rename / batch_scrape / batch_rename / reclassify / create_folder / rename_folder / delete_folder
	MediaID   string    `json:"media_id" gorm:"index;type:text"` // 关联的媒体ID（如适用）
	Detail    string    `json:"detail" gorm:"type:text"`         // 操作详情
	OldValue  string    `json:"old_value" gorm:"type:text"`      // 旧值（用于回滚）
	NewValue  string    `json:"new_value" gorm:"type:text"`      // 新值
	UserID    string    `json:"user_id" gorm:"index;type:text"`  // 操作者
	CreatedAt time.Time `json:"created_at" gorm:"index"`         // 操作时间
}

func (l *FileOperationLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}

// AutoMigrate 自动迁移所有模型
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&User{},
		&LoginLog{},
		&AuditLog{},
		&InviteCode{},
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
		// V4: 性能优化与标签统一
		&AICacheEntry{},
		&GenreMapping{},
		&RecommendCache{},
		// 视频预处理
		&PreprocessTask{},
		// 字幕预处理
		&SubtitlePreprocessTask{},
		// 电影系列合集
		&MovieCollection{},
		// 系统日志
		&SystemLog{},
		// 文件管理操作日志
		&FileOperationLog{},
	); err != nil {
		return err
	}

	// SQLite 列补全安全网：GORM AutoMigrate 在 SQLite 上有时无法正确添加新列
	// （尤其是从旧版数据库升级时），这里手动检查并补全关键缺失列
	ensureSQLiteColumns(db)

	return nil
}

// ensureSQLiteColumns 检查并补全 SQLite 表中可能缺失的列
// GORM 的 AutoMigrate 对 SQLite 的 ALTER TABLE ADD COLUMN 支持不完善，
// 当旧数据库文件残留时，新增字段可能静默丢失，导致运行时 SQL 报错
func ensureSQLiteColumns(db *gorm.DB) {
	// 定义需要检查的表和列（表名 -> []{ 列名, 列定义 }）
	requiredColumns := map[string][]struct {
		Column string
		DDL    string
	}{
		"media": {
			{Column: "tmdb_id", DDL: "ALTER TABLE `media` ADD COLUMN `tmdb_id` integer DEFAULT 0"},
			{Column: "imdb_id", DDL: "ALTER TABLE `media` ADD COLUMN `imdb_id` text DEFAULT ''"},
			{Column: "douban_id", DDL: "ALTER TABLE `media` ADD COLUMN `douban_id` text DEFAULT ''"},
			{Column: "bangumi_id", DDL: "ALTER TABLE `media` ADD COLUMN `bangumi_id` integer DEFAULT 0"},
			{Column: "country", DDL: "ALTER TABLE `media` ADD COLUMN `country` text DEFAULT ''"},
			{Column: "language", DDL: "ALTER TABLE `media` ADD COLUMN `language` text DEFAULT ''"},
			{Column: "tagline", DDL: "ALTER TABLE `media` ADD COLUMN `tagline` text DEFAULT ''"},
			{Column: "studio", DDL: "ALTER TABLE `media` ADD COLUMN `studio` text DEFAULT ''"},
			{Column: "trailer_url", DDL: "ALTER TABLE `media` ADD COLUMN `trailer_url` text DEFAULT ''"},
			{Column: "stack_group", DDL: "ALTER TABLE `media` ADD COLUMN `stack_group` text DEFAULT ''"},
			{Column: "stack_order", DDL: "ALTER TABLE `media` ADD COLUMN `stack_order` integer DEFAULT 0"},
			{Column: "version_tag", DDL: "ALTER TABLE `media` ADD COLUMN `version_tag` text DEFAULT ''"},
			{Column: "version_group", DDL: "ALTER TABLE `media` ADD COLUMN `version_group` text DEFAULT ''"},
			{Column: "scrape_status", DDL: "ALTER TABLE `media` ADD COLUMN `scrape_status` text DEFAULT 'pending'"},
			{Column: "scrape_attempts", DDL: "ALTER TABLE `media` ADD COLUMN `scrape_attempts` integer DEFAULT 0"},
			{Column: "last_scrape_at", DDL: "ALTER TABLE `media` ADD COLUMN `last_scrape_at` datetime"},
			{Column: "collection_id", DDL: "ALTER TABLE `media` ADD COLUMN `collection_id` text DEFAULT ''"},
			{Column: "stream_url", DDL: "ALTER TABLE `media` ADD COLUMN `stream_url` text DEFAULT ''"},
			// NFO 完整建模新增字段（方案 B）
			{Column: "num", DDL: "ALTER TABLE `media` ADD COLUMN `num` text DEFAULT ''"},
			{Column: "sort_title", DDL: "ALTER TABLE `media` ADD COLUMN `sort_title` text DEFAULT ''"},
			{Column: "outline", DDL: "ALTER TABLE `media` ADD COLUMN `outline` text DEFAULT ''"},
			{Column: "original_plot", DDL: "ALTER TABLE `media` ADD COLUMN `original_plot` text DEFAULT ''"},
			{Column: "mpaa", DDL: "ALTER TABLE `media` ADD COLUMN `mpaa` text DEFAULT ''"},
			{Column: "country_code", DDL: "ALTER TABLE `media` ADD COLUMN `country_code` text DEFAULT ''"},
			{Column: "maker", DDL: "ALTER TABLE `media` ADD COLUMN `maker` text DEFAULT ''"},
			{Column: "publisher", DDL: "ALTER TABLE `media` ADD COLUMN `publisher` text DEFAULT ''"},
			{Column: "label", DDL: "ALTER TABLE `media` ADD COLUMN `label` text DEFAULT ''"},
			{Column: "tags", DDL: "ALTER TABLE `media` ADD COLUMN `tags` text DEFAULT ''"},
			{Column: "website", DDL: "ALTER TABLE `media` ADD COLUMN `website` text DEFAULT ''"},
			{Column: "release_date", DDL: "ALTER TABLE `media` ADD COLUMN `release_date` text DEFAULT ''"},
		},
		"users": {
			{Column: "nickname", DDL: "ALTER TABLE `users` ADD COLUMN `nickname` text DEFAULT ''"},
			{Column: "email", DDL: "ALTER TABLE `users` ADD COLUMN `email` text DEFAULT ''"},
			{Column: "disabled", DDL: "ALTER TABLE `users` ADD COLUMN `disabled` numeric DEFAULT 0"},
			{Column: "must_change_pwd", DDL: "ALTER TABLE `users` ADD COLUMN `must_change_pwd` numeric DEFAULT 0"},
			{Column: "token_version", DDL: "ALTER TABLE `users` ADD COLUMN `token_version` integer DEFAULT 0"},
			{Column: "last_login_at", DDL: "ALTER TABLE `users` ADD COLUMN `last_login_at` datetime"},
			{Column: "last_login_ip", DDL: "ALTER TABLE `users` ADD COLUMN `last_login_ip` text DEFAULT ''"},
		},
		"series": {
			{Column: "tmdb_id", DDL: "ALTER TABLE `series` ADD COLUMN `tmdb_id` integer DEFAULT 0"},
			{Column: "bangumi_id", DDL: "ALTER TABLE `series` ADD COLUMN `bangumi_id` integer DEFAULT 0"},
			{Column: "douban_id", DDL: "ALTER TABLE `series` ADD COLUMN `douban_id` text DEFAULT ''"},
			{Column: "imdb_id", DDL: "ALTER TABLE `series` ADD COLUMN `imdb_id` text DEFAULT ''"},
			{Column: "country", DDL: "ALTER TABLE `series` ADD COLUMN `country` text DEFAULT ''"},
			{Column: "language", DDL: "ALTER TABLE `series` ADD COLUMN `language` text DEFAULT ''"},
			{Column: "studio", DDL: "ALTER TABLE `series` ADD COLUMN `studio` text DEFAULT ''"},
			{Column: "tagline", DDL: "ALTER TABLE `series` ADD COLUMN `tagline` text DEFAULT ''"},
		},
		"watch_histories": {
			{Column: "profile_id", DDL: "ALTER TABLE `watch_histories` ADD COLUMN `profile_id` text DEFAULT ''"},
		},
		"favorites": {
			{Column: "profile_id", DDL: "ALTER TABLE `favorites` ADD COLUMN `profile_id` text DEFAULT ''"},
		},
		"movie_collections": {
			{Column: "file_count", DDL: "ALTER TABLE `movie_collections` ADD COLUMN `file_count` integer DEFAULT 0"},
		},
	}

	for table, columns := range requiredColumns {
		for _, col := range columns {
			if !columnExists(db, table, col.Column) {
				if err := db.Exec(col.DDL).Error; err != nil {
					// 列可能已存在（并发或其他原因），忽略 "duplicate column" 错误
					fmt.Printf("[数据库迁移] 补全列 %s.%s 失败（可忽略）: %v\n", table, col.Column, err)
				} else {
					fmt.Printf("[数据库迁移] 已补全缺失列: %s.%s\n", table, col.Column)
				}
			}
		}
	}
}

// columnExists 检查 SQLite 表中是否存在指定列
func columnExists(db *gorm.DB, table, column string) bool {
	type ColumnInfo struct {
		Name string `gorm:"column:name"`
	}
	var columns []ColumnInfo
	db.Raw("PRAGMA table_info(`" + table + "`)").Scan(&columns)
	for _, c := range columns {
		if c.Name == column {
			return true
		}
	}
	return false
}
