package config

import (
	cryptoRand "crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

// ==================== 子配置结构体 ====================

// DatabaseConfig 数据库连接参数
type DatabaseConfig struct {
	// 数据库文件路径，默认 ./data/nowen.db
	DBPath string `mapstructure:"db_path"`
	// SQLite WAL 模式，默认 true
	WALMode bool `mapstructure:"wal_mode"`
	// 繁忙超时（毫秒），默认 5000
	BusyTimeout int `mapstructure:"busy_timeout"`
	// 缓存大小（负数为KB），默认 -20000
	CacheSize int `mapstructure:"cache_size"`
	// 最大打开连接数，默认 1（SQLite 建议）
	MaxOpenConns int `mapstructure:"max_open_conns"`
	// 最大空闲连接数，默认 1
	MaxIdleConns int `mapstructure:"max_idle_conns"`
}

// SecretsConfig 敏感信息/第三方服务 API 密钥
type SecretsConfig struct {
	// JWT 签名密钥（必须修改默认值）
	JWTSecret string `mapstructure:"jwt_secret"`
	// TMDb API Key，用于元数据刮削
	TMDbAPIKey string `mapstructure:"tmdb_api_key"`
	// TMDb API 代理地址（解决国内直连超时问题，如 https://api.tmdb.org 的镜像）
	// 留空则使用官方地址 https://api.themoviedb.org
	TMDbAPIProxy string `mapstructure:"tmdb_api_proxy"`
	// TMDb 图片代理地址（解决国内图片下载超时，如 https://image.tmdb.org 的镜像）
	// 留空则使用官方地址 https://image.tmdb.org
	TMDbImageProxy string `mapstructure:"tmdb_image_proxy"`
	// Bangumi Access Token（用于提高 API 请求速率限制，可选）
	// 获取地址: https://next.bgm.tv/demo/access-token
	// 留空也可使用（匿名请求，速率较低）
	BangumiAccessToken string `mapstructure:"bangumi_access_token"`
	// TheTVDB API Key（用于获取电视剧集的详细元数据）
	// 申请地址: https://thetvdb.com/api-information
	// 留空则跳过 TheTVDB 数据源
	TheTVDBAPIKey string `mapstructure:"thetvdb_api_key"`
	// Fanart.tv API Key（用于获取高质量图片资源：ClearLogo、背景图、光碟封面等）
	// 申请地址: https://fanart.tv/get-an-api-key/
	// 留空则跳过 Fanart.tv 图片增强
	FanartTVAPIKey string `mapstructure:"fanart_tv_api_key"`
	// 豆瓣登录 Cookie（可选，用于提升豆瓣刮削成功率和降低风控概率）
	// 从浏览器登录豆瓣后 F12 -> Network -> Request Headers 中复制完整 Cookie 字符串
	// 关键字段：bid / dbcl2 / ck。留空则以匿名方式访问豆瓣，仍可工作但成功率较低。
	// 注意：Cookie 有效期约 1 个月，失效时需重新获取。仅供个人使用，请勿分享。
	DoubanCookie string `mapstructure:"douban_cookie"`
	// 预留：其他第三方服务密钥可在此扩展
}

// AppConfig 应用运行环境配置
type AppConfig struct {
	// 服务器监听端口，默认 8080
	Port int `mapstructure:"port"`
	// 调试模式，默认 false
	Debug bool `mapstructure:"debug"`
	// 运行环境标识：development / production / testing
	Env string `mapstructure:"env"`
	// 数据目录，默认 ./data
	DataDir string `mapstructure:"data_dir"`
	// 前端静态文件目录，默认 ./web/dist
	WebDir string `mapstructure:"web_dir"`
	// FFmpeg 可执行文件路径
	FFmpegPath string `mapstructure:"ffmpeg_path"`
	// FFprobe 可执行文件路径
	FFprobePath string `mapstructure:"ffprobe_path"`
	// VAAPI 设备路径，如 /dev/dri/renderD128
	// 保留该字段作为 Linux Intel 核显的逃生门，不在 UI 暴露。
	VAAPIDevice string `mapstructure:"vaapi_device"`
	// 允许的跨域来源列表
	CORSOrigins []string `mapstructure:"cors_origins"`
}

// LoggingConfig 日志记录设置
type LoggingConfig struct {
	// 日志级别: debug / info / warn / error
	Level string `mapstructure:"level"`
	// 日志输出格式: json / console
	Format string `mapstructure:"format"`
	// 日志输出文件路径，留空则输出到 stdout
	OutputPath string `mapstructure:"output_path"`
	// 错误日志输出路径，留空则输出到 stderr
	ErrorOutputPath string `mapstructure:"error_output_path"`
	// 是否启用日志文件轮转
	EnableRotation bool `mapstructure:"enable_rotation"`
	// 单个日志文件最大大小（MB），默认 100
	MaxSizeMB int `mapstructure:"max_size_mb"`
	// 日志文件最大保留天数，默认 30
	MaxAgeDays int `mapstructure:"max_age_days"`
	// 日志文件最大保留个数，默认 10
	MaxBackups int `mapstructure:"max_backups"`
}

// CacheConfig 缓存配置参数
type CacheConfig struct {
	// 转码缓存目录，默认 ./cache
	CacheDir string `mapstructure:"cache_dir"`
	// 缓存最大占用磁盘空间（MB），0 为不限制
	MaxDiskUsageMB int `mapstructure:"max_disk_usage_mb"`
	// 缓存文件过期时间（小时），0 为不过期
	TTLHours int `mapstructure:"ttl_hours"`
	// 是否启用自动清理过期缓存
	AutoCleanup bool `mapstructure:"auto_cleanup"`
	// 自动清理间隔（分钟），默认 60
	CleanupIntervalMin int `mapstructure:"cleanup_interval_min"`
}

// ==================== 主配置结构体 ====================

// AIConfig AI 功能配置
type AIConfig struct {
	// 是否启用 AI 功能（总开关）
	Enabled bool `mapstructure:"enabled"`
	// LLM 提供商: openai / deepseek / qwen / ollama
	Provider string `mapstructure:"provider"`
	// API 基础地址
	APIBase string `mapstructure:"api_base"`
	// API 密钥
	APIKey string `mapstructure:"api_key"`
	// 模型名称
	Model string `mapstructure:"model"`
	// 请求超时（秒）
	Timeout int `mapstructure:"timeout"`
	// 功能开关
	EnableSmartSearch     bool `mapstructure:"enable_smart_search"`
	EnableRecommendReason bool `mapstructure:"enable_recommend_reason"`
	EnableMetadataEnhance bool `mapstructure:"enable_metadata_enhance"`
	// 高级设置
	MonthlyBudget     int `mapstructure:"monthly_budget"`
	CacheTTLHours     int `mapstructure:"cache_ttl_hours"`
	MaxConcurrent     int `mapstructure:"max_concurrent"`
	RequestIntervalMs int `mapstructure:"request_interval_ms"`

	// ==================== ASR / Whisper 云端 API 独立配置 ====================
	// Whisper API 独立地址（留空则复用 APIBase）
	WhisperAPIBase string `mapstructure:"whisper_api_base"`
	// Whisper API 独立密钥（留空则复用 APIKey）
	WhisperAPIKey string `mapstructure:"whisper_api_key"`
	// Whisper 模型名称（留空则使用默认 whisper-1）
	WhisperModel string `mapstructure:"whisper_model"`
	// Whisper API 请求超时（秒，0 则使用默认 300）
	WhisperTimeout int `mapstructure:"whisper_timeout"`

	// ==================== ASR / 本地 whisper.cpp 配置 ====================
	// 本地 whisper.cpp 可执行文件路径（留空则仅使用云端 API）
	WhisperCppPath string `mapstructure:"whisper_cpp_path"`
	// 本地 Whisper 模型文件路径（如 ggml-large-v3.bin）
	WhisperModelPath string `mapstructure:"whisper_model_path"`
	// 本地 Whisper 线程数（默认 4）
	WhisperThreads int `mapstructure:"whisper_threads"`
	// 是否优先使用本地引擎（默认 false，优先云端）
	PreferLocalWhisper bool `mapstructure:"prefer_local_whisper"`

	// ==================== 字幕预处理配置 ====================
	// 是否在媒体库扫描后自动触发字幕预处理
	AutoSubtitlePreprocess bool `mapstructure:"auto_subtitle_preprocess"`
	// 自动预处理的目标翻译语言列表（逗号分隔，如 "zh,en"，留空则不翻译）
	SubtitleTargetLangs string `mapstructure:"subtitle_target_langs"`
	// 字幕预处理最大并发数（默认 1）
	SubtitlePreprocessWorkers int `mapstructure:"subtitle_preprocess_workers"`
	// 是否优先使用已有字幕（内嵌/外挂），而非重新 AI 生成（默认 true）
	PreferExistingSubtitle bool `mapstructure:"prefer_existing_subtitle"`

	// ==================== 图形字幕 OCR 配置 ====================
	// 是否启用图形字幕 OCR 识别（PGS/VobSub 等）
	OCREnabled bool `mapstructure:"ocr_enabled"`
	// Tesseract 可执行文件路径（留空则使用系统 PATH 中的 tesseract）
	TesseractPath string `mapstructure:"tesseract_path"`
	// Tesseract OCR 语言包（如 "chi_sim+eng"，默认 "eng"）
	TesseractLang string `mapstructure:"tesseract_lang"`
	// 图形字幕导出图片 DPI（默认 150）
	OCRDPI int `mapstructure:"ocr_dpi"`

	// ==================== 字幕清洗配置 ====================
	// 是否启用字幕内容清洗（在字幕提取/转换后、翻译前执行）
	SubCleanEnabled bool `mapstructure:"sub_clean_enabled"`
	// 去除 HTML 标签（<i>, <b>, <font> 等）
	SubCleanRemoveHTML bool `mapstructure:"sub_clean_remove_html"`
	// 去除 ASS 样式标签（{\an8}, {\pos()} 等）
	SubCleanRemoveASSStyle bool `mapstructure:"sub_clean_remove_ass_style"`
	// 统一标点符号（全角→半角，仅对非 CJK 文本生效）
	SubCleanNormalizePunct bool `mapstructure:"sub_clean_normalize_punct"`
	// 去除 SDH 标注（[音乐], (笑声), [门铃响] 等听障辅助描述）
	SubCleanRemoveSDH bool `mapstructure:"sub_clean_remove_sdh"`
	// 去除广告水印字幕（字幕组署名、网站地址等）
	SubCleanRemoveAds bool `mapstructure:"sub_clean_remove_ads"`
	// 合并过短的字幕条目（显示时长低于阈值时与相邻条目合并）
	SubCleanMergeShort bool `mapstructure:"sub_clean_merge_short"`
	// 拆分过长的字幕条目（超过最大字符数时按时间均分拆分）
	SubCleanSplitLong bool `mapstructure:"sub_clean_split_long"`
	// 处理前备份原始字幕文件（生成 .bak 文件）
	SubCleanBackup bool `mapstructure:"sub_clean_backup"`
	// 编码检测失败时的回退编码（如 "gbk"、"big5"、"shift_jis"）
	SubCleanFallbackEnc string `mapstructure:"sub_clean_fallback_enc"`
	// 全局时间轴偏移（毫秒，正数延后、负数提前）
	SubCleanTimeOffsetMs int64 `mapstructure:"sub_clean_time_offset_ms"`
	// 最小字幕显示时长（毫秒，低于此值的条目将被合并，默认 500）
	SubCleanMinDurationMs int64 `mapstructure:"sub_clean_min_duration_ms"`
	// 最大字幕显示时长（毫秒，超过此值的条目将被截断，默认 10000）
	SubCleanMaxDurationMs int64 `mapstructure:"sub_clean_max_duration_ms"`
	// 合并间隔阈值（毫秒，两条字幕间隔小于此值时可合并，默认 200）
	SubCleanMinGapMs int64 `mapstructure:"sub_clean_min_gap_ms"`
	// 每行最大字符数（用于拆分过长字幕，默认 42）
	SubCleanMaxCharsPerLine int `mapstructure:"sub_clean_max_chars_per_line"`
	// 每条字幕最大行数（默认 2）
	SubCleanMaxLinesPerCue int `mapstructure:"sub_clean_max_lines_per_cue"`
}

// RegistrationConfig 注册控制配置
type RegistrationConfig struct {
	// 是否允许公开注册，默认 false（仅管理员可创建用户）
	Enabled bool `mapstructure:"enabled"`
	// 邀请码（设置后注册时需提供正确的邀请码）
	InviteCode string `mapstructure:"invite_code"`
}

// EmbyConfig Emby/Jellyfin 兼容层配置（供移动端/桌面 Emby/Infuse/Jellyfin 客户端登录与播放）
type EmbyConfig struct {
	// 服务器对外显示名称（留空则使用主机名或 "nowen-video"）
	ServerName string `mapstructure:"server_name"`
	// 是否启用 UDP 7359 局域网服务器自动发现（Emby / Jellyfin 标准协议）
	// 开启后同网段的客户端会在"添加服务器"时自动发现本机
	EnableAutoDiscovery bool `mapstructure:"enable_auto_discovery"`
	// UDP 自动发现监听端口，默认 7359（Emby/Jellyfin 标准）
	AutoDiscoveryPort int `mapstructure:"auto_discovery_port"`
	// 是否在 /Users/Public 暴露用户列表（登录页展示用户头像点击登录）
	// 默认 false 以保护用户名隐私；开启更适合家庭共享场景
	PublicUserListEnabled bool `mapstructure:"public_user_list_enabled"`
	// 是否启用 WebSocket（/embywebsocket），消除客户端连接失败告警
	EnableWebSocket bool `mapstructure:"enable_websocket"`
	// 登录品牌自定义文案（Jellyfin 客户端 /Branding/Configuration 使用）
	// 登录页顶部欢迎语
	LoginDisclaimer string `mapstructure:"login_disclaimer"`
	// 自定义 CSS（Jellyfin Web 客户端 /Branding/Css）
	CustomCss string `mapstructure:"custom_css"`
}

// AdultScraperConfig 番号刮削配置（混合架构：Go 原生爬虫 + Python 微服务）
type AdultScraperConfig struct {
	// 是否启用番号刮削功能（总开关）
	Enabled bool `mapstructure:"enabled"`
	// 是否启用 JavBus 数据源（Go 原生爬虫）
	EnableJavBus bool `mapstructure:"enable_javbus"`
	// JavBus 镜像地址（留空使用默认 https://www.javbus.com）
	JavBusURL string `mapstructure:"javbus_url"`
	// 是否启用 JavDB 数据源（Go 原生爬虫）
	EnableJavDB bool `mapstructure:"enable_javdb"`
	// JavDB 镜像地址（留空使用默认 https://javdb.com）
	JavDBURL string `mapstructure:"javdb_url"`

	// ==================== P1 扩展：更多数据源 ====================
	// 是否启用 Freejavbt 数据源（Go 原生爬虫，中文元数据优秀）
	EnableFreejavbt bool `mapstructure:"enable_freejavbt"`
	// Freejavbt 镜像地址（留空使用默认 https://freejavbt.com）
	FreejavbtURL string `mapstructure:"freejavbt_url"`
	// 是否启用 JAV321 数据源（Go 原生爬虫，中文简介丰富）
	EnableJav321 bool `mapstructure:"enable_jav321"`
	// JAV321 镜像地址（留空使用默认 https://www.jav321.com）
	Jav321URL string `mapstructure:"jav321_url"`

	// ==================== P2 扩展：更多数据源 ====================
	// 是否启用 Fanza (DMM) 官方数据源（数据最权威，封面最清晰）
	EnableFanza bool `mapstructure:"enable_fanza"`
	// Fanza 站点地址（留空使用默认 https://www.dmm.co.jp）
	FanzaURL string `mapstructure:"fanza_url"`
	// 是否启用 MGStage 数据源（MGS 系列和 200GANA 等素人番号专用）
	EnableMGStage bool `mapstructure:"enable_mgstage"`
	// MGStage 站点地址（留空使用默认 https://www.mgstage.com）
	MGStageURL string `mapstructure:"mgstage_url"`
	// 是否启用 FC2Hub 数据源（FC2-PPV 无码作品专用）
	EnableFC2Hub bool `mapstructure:"enable_fc2hub"`
	// FC2Hub 站点地址（留空使用默认 https://fc2hub.com）
	FC2HubURL string `mapstructure:"fc2hub_url"`

	// ==================== P2 扩展：聚合模式 + 封面处理 ====================
	// 聚合刮削模式：并发调用所有启用的数据源，按字段优先级合并最完整结果
	// 开启后耗时变长但数据最完整，适合精刮场景
	EnableAggregatedMode bool `mapstructure:"enable_aggregated_mode"`
	// 封面裁剪：把横版大图裁剪成 2:3 竖版 poster（Emby 媒体墙更美观）
	EnablePosterCrop bool `mapstructure:"enable_poster_crop"`

	// ==================== P1 扩展：多媒体资源下载 ====================
	// 是否下载剧照（ExtraFanart，供 Emby/Jellyfin 显示多张剧照）
	DownloadExtraFanart bool `mapstructure:"download_extra_fanart"`
	// 最多下载多少张剧照（0 = 全部，默认 10）
	MaxExtraFanart int `mapstructure:"max_extra_fanart"`
	// 是否下载演员头像（写入 .actors/ 目录供 Emby 使用）
	DownloadActorPhoto bool `mapstructure:"download_actor_photo"`
	// 是否抓取 Trailer 预告片 URL（写入 NFO）
	FetchTrailer bool `mapstructure:"fetch_trailer"`

	// ==================== P1 扩展：翻译配置 ====================
	// 是否启用标题/简介翻译（日文 -> 中文）
	EnableTranslate bool `mapstructure:"enable_translate"`
	// 翻译服务提供商：google / baidu / youdao / deeplx / disabled
	TranslateProvider string `mapstructure:"translate_provider"`
	// 翻译服务接口地址（自建 deeplx 时必填，格式 http://host:port/translate）
	TranslateEndpoint string `mapstructure:"translate_endpoint"`
	// 翻译服务 API Key（百度/有道等商业服务需要）
	TranslateAPIKey string `mapstructure:"translate_api_key"`
	// 翻译服务 API Secret（百度/有道需要）
	TranslateAPISecret string `mapstructure:"translate_api_secret"`
	// 翻译目标语言（默认 zh-CN）
	TranslateTargetLang string `mapstructure:"translate_target_lang"`

	// Python 刮削微服务地址（用于 Cloudflare 等强反爬场景的 fallback）
	// 留空则不使用 Python 微服务
	// 示例: http://localhost:5000
	PythonServiceURL string `mapstructure:"python_service_url"`
	// Python 微服务 API Key（可选，用于认证）
	PythonServiceAPIKey string `mapstructure:"python_service_api_key"`
	// 是否在 Go 后端启动时自动拉起 Python 微服务子进程
	// 为 true 时，Go 进程会 fork 出一个 python app.py 子进程，并在主服务关闭时一并回收
	AutoStartPython bool `mapstructure:"auto_start_python"`
	// Python 可执行文件路径（留空自动探测 python3/python/py）
	PythonExecutable string `mapstructure:"python_executable"`
	// Python 微服务脚本目录（留空使用默认 scripts/adult-scraper）
	PythonServiceDir string `mapstructure:"python_service_dir"`
	// 请求间隔最小值（毫秒，默认 1500，防止被封 IP）
	MinRequestInterval int `mapstructure:"min_request_interval"`
	// 请求间隔最大值（毫秒，默认 3000）
	MaxRequestInterval int `mapstructure:"max_request_interval"`

	// ==================== Cookie 登录配置（参考 mdcx）====================
	// 某些站点（尤其 JavDB）非登录态内容受限，通过设置登录后的 Cookie 头
	// 可解锁：高清封面、完整演员信息、评分、预告片等
	// 格式：浏览器复制的完整 Cookie 字符串，如 "locale=zh; _jdb_session=abc..."
	//
	// 获取方法：登录网站 → F12 → Network → 任一请求 → 复制 Cookie 头
	CookieJavBus    string `mapstructure:"cookie_javbus"`
	CookieJavDB     string `mapstructure:"cookie_javdb"`
	CookieFreejavbt string `mapstructure:"cookie_freejavbt"`
	CookieJav321    string `mapstructure:"cookie_jav321"`
	CookieFanza     string `mapstructure:"cookie_fanza"`
	CookieMGStage   string `mapstructure:"cookie_mgstage"`
	CookieFC2Hub    string `mapstructure:"cookie_fc2hub"`
}

// StorageConfig 存储配置（支持本地、WebDAV、网盘等多种存储后端）
type StorageConfig struct {
	// ==================== WebDAV 存储配置 ====================
	WebDAV WebDAVConfig `mapstructure:"webdav"`

	// ==================== V2.3: Alist 聚合网盘配置 ====================
	// 通过 Alist HTTP API 对接阿里云盘 / 115 / 夸克 / 百度网盘 / OneDrive 等 20+ 网盘
	Alist AlistConfig `mapstructure:"alist"`

	// ==================== V2.3: S3 兼容对象存储配置 ====================
	// 对接 AWS S3 / MinIO / Cloudflare R2 / 阿里云 OSS / 腾讯云 COS 等
	S3 S3Config `mapstructure:"s3"`

	// ==================== 预留：未来扩展 ====================
	// OneDrive    OneDriveConfig    `mapstructure:"onedrive"`
}

// AlistConfig Alist 聚合网盘配置（V2.3）
//
// Alist 官网: https://alist.nn.ci/
// 认证模式：
//  1. Token 模式（推荐）：预先获取长期 Token，直接填入 Token 字段
//  2. 用户名密码模式：首次请求时调用 /api/auth/login 换取 Token
type AlistConfig struct {
	// 是否启用 Alist 存储
	Enabled bool `mapstructure:"enabled"`
	// Alist 服务器地址（如 https://alist.example.com）
	ServerURL string `mapstructure:"server_url"`
	// 用户名（Token 模式可不填）
	Username string `mapstructure:"username"`
	// 密码（Token 模式可不填）
	Password string `mapstructure:"password"`
	// 长期 Token（优先于用户名密码）
	Token string `mapstructure:"token"`
	// 基础路径（Alist 内的根目录，如 /aliyun/movies）
	BasePath string `mapstructure:"base_path"`
	// 连接超时（秒，默认 30）
	Timeout int `mapstructure:"timeout"`
	// 是否启用元数据缓存
	EnableCache bool `mapstructure:"enable_cache"`
	// 元数据缓存 TTL（小时，默认 12）
	CacheTTLHours int `mapstructure:"cache_ttl_hours"`
	// ReadAt 块缓存大小（MiB，默认 8，<=0 禁用）
	ReadBlockSizeMB int `mapstructure:"read_block_size_mb"`
	// ReadAt 块缓存最大块数（每文件，默认 4，<=0 禁用）
	ReadBlockCount int `mapstructure:"read_block_count"`
}

// S3Config S3 兼容对象存储配置（V2.3）
type S3Config struct {
	// 是否启用 S3 存储
	Enabled bool `mapstructure:"enabled"`
	// S3 Endpoint（如 https://s3.amazonaws.com、https://minio.example.com:9000）
	Endpoint string `mapstructure:"endpoint"`
	// 区域（AWS 必填，MinIO 可留空或 us-east-1）
	Region string `mapstructure:"region"`
	// Access Key
	AccessKey string `mapstructure:"access_key"`
	// Secret Key
	SecretKey string `mapstructure:"secret_key"`
	// Bucket 名称
	Bucket string `mapstructure:"bucket"`
	// 基础路径前缀（Object Key 前缀，如 media/）
	BasePath string `mapstructure:"base_path"`
	// 是否使用 Path-Style 寻址（MinIO 必开，AWS 默认 Virtual-Host-Style）
	PathStyle bool `mapstructure:"path_style"`
	// 连接超时（秒，默认 30）
	Timeout int `mapstructure:"timeout"`
	// 是否启用元数据缓存
	EnableCache bool `mapstructure:"enable_cache"`
	// 元数据缓存 TTL（小时，默认 24）
	CacheTTLHours int `mapstructure:"cache_ttl_hours"`
	// ReadAt 块缓存大小（MiB，默认 8，<=0 禁用）
	ReadBlockSizeMB int `mapstructure:"read_block_size_mb"`
	// ReadAt 块缓存最大块数（每文件，默认 4，<=0 禁用）
	ReadBlockCount int `mapstructure:"read_block_count"`
}

// WebDAVConfig WebDAV 远程存储配置
type WebDAVConfig struct {
	// 是否启用 WebDAV 存储
	Enabled bool `mapstructure:"enabled"`
	// WebDAV 服务器地址（如 https://dav.example.com）
	ServerURL string `mapstructure:"server_url"`
	// 用户名
	Username string `mapstructure:"username"`
	// 密码
	Password string `mapstructure:"password"`
	// 基础路径（服务器上的根目录，如 /media）
	BasePath string `mapstructure:"base_path"`
	// 连接超时（秒，默认 30）
	Timeout int `mapstructure:"timeout"`
	// 是否启用连接池
	EnablePool bool `mapstructure:"enable_pool"`
	// 连接池大小（默认 5）
	PoolSize int `mapstructure:"pool_size"`
	// 是否启用缓存（本地缓存远程文件元数据）
	EnableCache bool `mapstructure:"enable_cache"`
	// 缓存过期时间（小时，默认 24）
	CacheTTLHours int `mapstructure:"cache_ttl_hours"`
	// 最大重试次数（默认 3）
	MaxRetries int `mapstructure:"max_retries"`
	// 重试间隔（秒，默认 2）
	RetryInterval int `mapstructure:"retry_interval"`
	// V2.1: ReadAt 块缓存大小（MiB，默认 8，<=0 禁用）
	ReadBlockSizeMB int `mapstructure:"read_block_size_mb"`
	// V2.1: ReadAt 块缓存最大块数（每文件，默认 4，<=0 禁用）
	ReadBlockCount int `mapstructure:"read_block_count"`
}

// STRMConfig .strm 远程流全局配置
type STRMConfig struct {
	// 默认 User-Agent（Media 自身无 UA 时使用），留空则使用内置值
	DefaultUserAgent string `mapstructure:"default_user_agent"`
	// 默认 Referer（留空=不发送）
	DefaultReferer string `mapstructure:"default_referer"`
	// 代理远程流时的连接超时（秒，默认 30，仅影响首包握手；读取阶段不超时）
	ConnectTimeout int `mapstructure:"connect_timeout"`
	// 对 HLS (.m3u8) 子清单做 URL 重写，让分片继续走后端代理（解决跨域/鉴权透传）
	RewriteHLS bool `mapstructure:"rewrite_hls"`
	// 扫描时是否对直链 mp4/mkv 启动远程 FFprobe 拉元数据（慢但能得到真实时长/分辨率）
	RemoteProbe bool `mapstructure:"remote_probe"`
	// 远程 FFprobe 超时秒数（默认 8）
	RemoteProbeTimeout int `mapstructure:"remote_probe_timeout"`
	// 按域名白名单追加 UA，例如：{"115.com":"Mozilla/5.0 ..."}
	DomainUserAgents map[string]string `mapstructure:"domain_user_agents"`
	// 按域名白名单追加 Referer，例如：{"115.com":"https://115.com/"}
	DomainReferers map[string]string `mapstructure:"domain_referers"`
}

// Config 应用主配置（聚合所有子模块）
type Config struct {
	mu sync.RWMutex `mapstructure:"-"`

	// 子模块配置
	Database     DatabaseConfig     `mapstructure:"database"`
	Secrets      SecretsConfig      `mapstructure:"secrets"`
	App          AppConfig          `mapstructure:"app"`
	Logging      LoggingConfig      `mapstructure:"logging"`
	Cache        CacheConfig        `mapstructure:"cache"`
	AI           AIConfig           `mapstructure:"ai"`
	Registration RegistrationConfig `mapstructure:"registration"`
	Storage      StorageConfig      `mapstructure:"storage"`
	Emby         EmbyConfig         `mapstructure:"emby"`
	AdultScraper AdultScraperConfig `mapstructure:"adult_scraper"`
	STRM         STRMConfig         `mapstructure:"strm"`

	// ==================== 兼容性字段（向后兼容旧的扁平配置） ====================
	// 以下字段用于兼容旧版 config.yaml 中的扁平 key，
	// 加载后会自动合并到对应的子模块中。

	// 旧版兼容 - 数据库
	DBPath string `mapstructure:"db_path"`
	// 旧版兼容 - 密钥
	JWTSecret  string `mapstructure:"jwt_secret"`
	TMDbAPIKey string `mapstructure:"tmdb_api_key"`
	// 旧版兼容 - 应用
	Port        int      `mapstructure:"port"`
	Debug       bool     `mapstructure:"debug"`
	DataDir     string   `mapstructure:"data_dir"`
	WebDir      string   `mapstructure:"web_dir"`
	CacheDir    string   `mapstructure:"cache_dir"`
	FFmpegPath  string   `mapstructure:"ffmpeg_path"`
	FFprobePath string   `mapstructure:"ffprobe_path"`
	VAAPIDevice string   `mapstructure:"vaapi_device"`
	CORSOrigins []string `mapstructure:"cors_origins"`
}

// ==================== 加载逻辑 ====================

// Load 加载配置，支持以下方式（优先级从低到高）：
//  1. 内置默认值
//  2. 主配置文件 config.yaml（兼容旧版扁平格式）
//  3. config/ 目录下的分片配置文件（database.yaml, secrets.yaml 等）
//  4. 环境变量（NOWEN_ 前缀）
func Load() (*Config, error) {
	// 设置默认值
	setDefaults()

	// 配置文件搜索路径
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./data")
	viper.AddConfigPath("/etc/nowen-video")

	// 环境变量
	viper.SetEnvPrefix("NOWEN")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// 1. 读取主配置文件（不存在也不报错）
	_ = viper.ReadInConfig()

	// 2. 合并 config/ 目录下的分片配置文件
	if err := mergeConfigDir(); err != nil {
		return nil, fmt.Errorf("加载分片配置文件失败: %w", err)
	}

	// 3. 反序列化
	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 4. 向后兼容：将旧版扁平字段合并到子模块
	cfg.migrateFromFlatConfig()

	// 5. 确保目录存在
	for _, dir := range []string{cfg.App.DataDir, cfg.Cache.CacheDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
	}

	// 6. 处理 db_path 相对路径
	if !filepath.IsAbs(cfg.Database.DBPath) {
		cfg.Database.DBPath = filepath.Join(cfg.App.DataDir, filepath.Base(cfg.Database.DBPath))
	}

	// 7. 自动生成 JWT Secret（如果仍为默认值）
	if cfg.Secrets.JWTSecret == "nowen-video-secret-change-me" {
		cfg.Secrets.JWTSecret = generateRandomSecret(32)
	}

	return cfg, nil
}

// setDefaults 设置所有默认值
func setDefaults() {
	// ---- 数据库 ----
	viper.SetDefault("database.db_path", "./data/nowen.db")
	viper.SetDefault("database.wal_mode", true)
	viper.SetDefault("database.busy_timeout", 5000)
	viper.SetDefault("database.cache_size", -20000)
	viper.SetDefault("database.max_open_conns", 4)
	viper.SetDefault("database.max_idle_conns", 2)

	// ---- 密钥 ----
	viper.SetDefault("secrets.jwt_secret", "nowen-video-secret-change-me")
	viper.SetDefault("secrets.tmdb_api_key", "")
	viper.SetDefault("secrets.tmdb_api_proxy", "")
	viper.SetDefault("secrets.tmdb_image_proxy", "")
	viper.SetDefault("secrets.bangumi_access_token", "")
	viper.SetDefault("secrets.thetvdb_api_key", "")
	viper.SetDefault("secrets.fanart_tv_api_key", "")
	viper.SetDefault("secrets.douban_cookie", "")

	// ---- 应用 ----
	viper.SetDefault("app.port", 8080)
	viper.SetDefault("app.debug", false)
	viper.SetDefault("app.env", "production")
	viper.SetDefault("app.data_dir", "./data")
	viper.SetDefault("app.web_dir", "./web/dist")
	viper.SetDefault("app.ffmpeg_path", "ffmpeg")
	viper.SetDefault("app.ffprobe_path", "ffprobe")
	viper.SetDefault("app.vaapi_device", "/dev/dri/renderD128")
	viper.SetDefault("app.cors_origins", []string{})

	// ---- 日志 ----
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "console")
	viper.SetDefault("logging.output_path", "")
	viper.SetDefault("logging.error_output_path", "")
	viper.SetDefault("logging.enable_rotation", false)
	viper.SetDefault("logging.max_size_mb", 100)
	viper.SetDefault("logging.max_age_days", 30)
	viper.SetDefault("logging.max_backups", 10)

	// ---- AI ----
	viper.SetDefault("ai.enabled", false)
	viper.SetDefault("ai.provider", "openai")
	viper.SetDefault("ai.api_base", "https://api.openai.com/v1")
	viper.SetDefault("ai.api_key", "")
	viper.SetDefault("ai.model", "gpt-4o-mini")
	viper.SetDefault("ai.timeout", 30)
	viper.SetDefault("ai.enable_smart_search", true)
	viper.SetDefault("ai.enable_recommend_reason", true)
	viper.SetDefault("ai.enable_metadata_enhance", true)
	viper.SetDefault("ai.monthly_budget", 0)
	viper.SetDefault("ai.cache_ttl_hours", 168)
	viper.SetDefault("ai.max_concurrent", 3)
	viper.SetDefault("ai.request_interval_ms", 200)

	// ---- 字幕预处理（ASR/OCR/清洗）默认值 ----
	viper.SetDefault("ai.subtitle_preprocess_workers", 1)
	viper.SetDefault("ai.ocr_enabled", false)
	viper.SetDefault("ai.tesseract_path", "tesseract")
	viper.SetDefault("ai.tesseract_lang", "chi_sim+eng")

	// 字幕清洗：默认行为偏保守，开启后也不会误杀
	viper.SetDefault("ai.sub_clean_enabled", false)
	viper.SetDefault("ai.sub_clean_remove_html", true)
	viper.SetDefault("ai.sub_clean_remove_ass_style", true)
	viper.SetDefault("ai.sub_clean_normalize_punct", false)
	viper.SetDefault("ai.sub_clean_remove_sdh", true)
	viper.SetDefault("ai.sub_clean_remove_ads", true)
	viper.SetDefault("ai.sub_clean_merge_short", true)
	viper.SetDefault("ai.sub_clean_split_long", true)
	viper.SetDefault("ai.sub_clean_backup", true)
	viper.SetDefault("ai.sub_clean_fallback_enc", "gbk")
	viper.SetDefault("ai.sub_clean_time_offset_ms", 0)
	viper.SetDefault("ai.sub_clean_min_duration_ms", 500)
	viper.SetDefault("ai.sub_clean_max_duration_ms", 10000)
	viper.SetDefault("ai.sub_clean_min_gap_ms", 200)
	viper.SetDefault("ai.sub_clean_max_chars_per_line", 42)
	viper.SetDefault("ai.sub_clean_max_lines_per_cue", 2)

	// ---- 缓存 ----
	viper.SetDefault("cache.cache_dir", "./cache")
	viper.SetDefault("cache.max_disk_usage_mb", 0)
	viper.SetDefault("cache.ttl_hours", 0)
	viper.SetDefault("cache.auto_cleanup", false)
	viper.SetDefault("cache.cleanup_interval_min", 60)

	// ---- 注册控制 ----
	viper.SetDefault("registration.enabled", false)
	viper.SetDefault("registration.invite_code", "")

	// ---- Emby 兼容层 ----
	viper.SetDefault("emby.server_name", "")
	viper.SetDefault("emby.enable_auto_discovery", true)
	viper.SetDefault("emby.auto_discovery_port", 7359)
	viper.SetDefault("emby.public_user_list_enabled", false)
	viper.SetDefault("emby.enable_websocket", true)
	viper.SetDefault("emby.login_disclaimer", "")
	viper.SetDefault("emby.custom_css", "")

	// ---- 存储配置 ----
	// WebDAV 存储配置
	viper.SetDefault("storage.webdav.enabled", false)
	viper.SetDefault("storage.webdav.server_url", "")
	viper.SetDefault("storage.webdav.username", "")
	viper.SetDefault("storage.webdav.password", "")
	viper.SetDefault("storage.webdav.base_path", "")
	viper.SetDefault("storage.webdav.timeout", 30)
	viper.SetDefault("storage.webdav.enable_pool", true)
	viper.SetDefault("storage.webdav.pool_size", 5)
	viper.SetDefault("storage.webdav.enable_cache", true)
	viper.SetDefault("storage.webdav.cache_ttl_hours", 24)
	viper.SetDefault("storage.webdav.max_retries", 3)
	viper.SetDefault("storage.webdav.retry_interval", 2)
	// V2.1: ReadAt 块缓存（播放器 seek 加速）
	viper.SetDefault("storage.webdav.read_block_size_mb", 8)
	viper.SetDefault("storage.webdav.read_block_count", 4)

	// V2.3: Alist 聚合网盘默认值
	viper.SetDefault("storage.alist.enabled", false)
	viper.SetDefault("storage.alist.server_url", "")
	viper.SetDefault("storage.alist.base_path", "/")
	viper.SetDefault("storage.alist.timeout", 30)
	viper.SetDefault("storage.alist.enable_cache", true)
	viper.SetDefault("storage.alist.cache_ttl_hours", 12)
	viper.SetDefault("storage.alist.read_block_size_mb", 8)
	viper.SetDefault("storage.alist.read_block_count", 4)

	// V2.3: S3 兼容对象存储默认值
	viper.SetDefault("storage.s3.enabled", false)
	viper.SetDefault("storage.s3.endpoint", "")
	viper.SetDefault("storage.s3.region", "us-east-1")
	viper.SetDefault("storage.s3.bucket", "")
	viper.SetDefault("storage.s3.base_path", "")
	viper.SetDefault("storage.s3.path_style", true)
	viper.SetDefault("storage.s3.timeout", 30)
	viper.SetDefault("storage.s3.enable_cache", true)
	viper.SetDefault("storage.s3.cache_ttl_hours", 24)
	viper.SetDefault("storage.s3.read_block_size_mb", 8)
	viper.SetDefault("storage.s3.read_block_count", 4)

	// ---- STRM 远程流 ----
	viper.SetDefault("strm.default_user_agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	viper.SetDefault("strm.default_referer", "")
	viper.SetDefault("strm.connect_timeout", 30)
	viper.SetDefault("strm.rewrite_hls", true)
	viper.SetDefault("strm.remote_probe", true)
	viper.SetDefault("strm.remote_probe_timeout", 8)

	// ---- 番号刮削 ----
	viper.SetDefault("adult_scraper.enabled", false)
	viper.SetDefault("adult_scraper.enable_javbus", true)
	viper.SetDefault("adult_scraper.javbus_url", "")
	viper.SetDefault("adult_scraper.enable_javdb", true)
	viper.SetDefault("adult_scraper.javdb_url", "")
	viper.SetDefault("adult_scraper.python_service_url", "")
	viper.SetDefault("adult_scraper.python_service_api_key", "")
	viper.SetDefault("adult_scraper.auto_start_python", true)
	viper.SetDefault("adult_scraper.python_executable", "")
	viper.SetDefault("adult_scraper.python_service_dir", "scripts/adult-scraper")
	viper.SetDefault("adult_scraper.min_request_interval", 1500)
	viper.SetDefault("adult_scraper.max_request_interval", 3000)

	// ---- 旧版兼容默认值（当使用扁平 key 时） ----
	viper.SetDefault("port", 8080)
	viper.SetDefault("debug", false)
	viper.SetDefault("data_dir", "./data")
	viper.SetDefault("cache_dir", "./cache")
	viper.SetDefault("web_dir", "./web/dist")
	viper.SetDefault("db_path", "./data/nowen.db")
	viper.SetDefault("jwt_secret", "nowen-video-secret-change-me")
	viper.SetDefault("ffmpeg_path", "ffmpeg")
	viper.SetDefault("ffprobe_path", "ffprobe")
	viper.SetDefault("vaapi_device", "/dev/dri/renderD128")
	viper.SetDefault("tmdb_api_key", "")
}

// mergeConfigDir 合并 config/ 目录下的分片配置文件
func mergeConfigDir() error {
	// 搜索配置目录
	configDirs := []string{"./config", "./data/config", "/etc/nowen-video/config"}

	for _, dir := range configDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		// 按照固定顺序加载分片文件，确保优先级可预测
		configFiles := []struct {
			name   string // 文件名（不含扩展名）
			prefix string // 在 viper 中的 key 前缀
		}{
			{name: "database", prefix: "database"},
			{name: "secrets", prefix: "secrets"},
			{name: "app", prefix: "app"},
			{name: "logging", prefix: "logging"},
			{name: "cache", prefix: "cache"},
			{name: "ai", prefix: "ai"},
			{name: "storage", prefix: "storage"},
		}

		for _, cf := range configFiles {
			filePath := filepath.Join(dir, cf.name+".yaml")
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				continue
			}

			subViper := viper.New()
			subViper.SetConfigFile(filePath)
			if err := subViper.ReadInConfig(); err != nil {
				return fmt.Errorf("读取 %s 失败: %w", filePath, err)
			}

			// 将分片配置写入主 viper 的对应前缀下
			// 注意：分片配置中的空值不应覆盖主配置文件中已存在的非空值，
			// 避免 config/secrets.yaml 中的空 tmdb_api_key 覆盖 config.yaml 中用户已保存的值
			for _, key := range subViper.AllKeys() {
				fullKey := cf.prefix + "." + key
				newVal := subViper.Get(key)
				existingVal := viper.Get(fullKey)
				// 仅当分片配置的值非空，或主配置中尚无该值时，才进行覆盖
				if !isEmptyValue(newVal) || existingVal == nil || isEmptyValue(existingVal) {
					viper.Set(fullKey, newVal)
				}
			}
		}
	}

	return nil
}

// isEmptyValue 判断配置值是否为"空"（空字符串、nil、空切片等）
// 用于 mergeConfigDir 中避免分片配置的空值覆盖主配置中已有的非空值
func isEmptyValue(v interface{}) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case []interface{}:
		return len(val) == 0
	default:
		return false
	}
}

// migrateFromFlatConfig 将旧版扁平字段值合并到子模块配置中
// 规则：如果旧版字段有值且子模块字段为默认值，则使用旧版字段的值
func (c *Config) migrateFromFlatConfig() {
	// 数据库
	if c.DBPath != "" && c.DBPath != "./data/nowen.db" {
		c.Database.DBPath = c.DBPath
	}
	if c.Database.DBPath == "" {
		c.Database.DBPath = "./data/nowen.db"
	}

	// 密钥
	if c.JWTSecret != "" && c.JWTSecret != "nowen-video-secret-change-me" {
		c.Secrets.JWTSecret = c.JWTSecret
	}
	if c.Secrets.JWTSecret == "" {
		c.Secrets.JWTSecret = "nowen-video-secret-change-me"
	}
	if c.TMDbAPIKey != "" {
		c.Secrets.TMDbAPIKey = c.TMDbAPIKey
	}

	// 应用
	// 注意：扁平字段仅在嵌套字段为零值/默认值时才生效（向后兼容）
	// 如果嵌套字段已有非默认值（说明用户通过新版格式或 API 设置过），则以嵌套字段为准
	if c.App.Port == 0 {
		if c.Port != 0 {
			c.App.Port = c.Port
		} else {
			c.App.Port = 8080
		}
	}
	if c.Debug && !c.App.Debug {
		c.App.Debug = true
	}
	if c.App.DataDir == "" {
		if c.DataDir != "" && c.DataDir != "./data" {
			c.App.DataDir = c.DataDir
		} else {
			c.App.DataDir = "./data"
		}
	}
	if c.App.WebDir == "" {
		if c.WebDir != "" && c.WebDir != "./web/dist" {
			c.App.WebDir = c.WebDir
		} else {
			c.App.WebDir = "./web/dist"
		}
	}
	if c.App.FFmpegPath == "" {
		if c.FFmpegPath != "" && c.FFmpegPath != "ffmpeg" {
			c.App.FFmpegPath = c.FFmpegPath
		} else {
			c.App.FFmpegPath = "ffmpeg"
		}
	}
	if c.App.FFprobePath == "" {
		if c.FFprobePath != "" && c.FFprobePath != "ffprobe" {
			c.App.FFprobePath = c.FFprobePath
		} else {
			c.App.FFprobePath = "ffprobe"
		}
	}
	if c.App.VAAPIDevice == "" {
		if c.VAAPIDevice != "" {
			c.App.VAAPIDevice = c.VAAPIDevice
		} else {
			c.App.VAAPIDevice = "/dev/dri/renderD128"
		}
	}

	// 缓存
	if c.CacheDir != "" && c.CacheDir != "./cache" {
		c.Cache.CacheDir = c.CacheDir
	}
	if c.Cache.CacheDir == "" {
		c.Cache.CacheDir = "./cache"
	}
}

// ==================== 便捷访问方法（保持已有 API 兼容） ====================

// IsDefaultJWTSecret 检查是否使用自动生成的 JWT Secret（未在配置文件中显式设置）
// 注意：由于 Load() 中会自动替换默认值，此方法现在检查是否为用户显式配置
func (c *Config) IsDefaultJWTSecret() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// 如果 viper 中原始值仍为默认值，说明用户未显式配置
	return viper.GetString("secrets.jwt_secret") == "nowen-video-secret-change-me"
}

// GetTMDbAPIKey 获取 TMDb API Key（线程安全）
func (c *Config) GetTMDbAPIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Secrets.TMDbAPIKey
}

// GetTMDbAPIKeyMasked 获取掩码后的 TMDb API Key（用于前端展示）
func (c *Config) GetTMDbAPIKeyMasked() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := c.Secrets.TMDbAPIKey
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

// SetTMDbAPIKey 设置 TMDb API Key 并持久化到配置文件
func (c *Config) SetTMDbAPIKey(key string) error {
	c.mu.Lock()
	c.Secrets.TMDbAPIKey = key
	c.mu.Unlock()

	viper.Set("secrets.tmdb_api_key", key)

	// 同时更新分片配置文件（如果存在），确保重启后不会被旧的空值覆盖
	c.updateSecretsFile("tmdb_api_key", key)

	return c.saveConfig()
}

// ClearTMDbAPIKey 清除 TMDb API Key 并持久化
func (c *Config) ClearTMDbAPIKey() error {
	return c.SetTMDbAPIKey("")
}

// ==================== 豆瓣 Cookie 管理 ====================

// GetDoubanCookie 获取豆瓣登录 Cookie（线程安全）
func (c *Config) GetDoubanCookie() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Secrets.DoubanCookie
}

// GetDoubanCookieMasked 获取掩码后的豆瓣 Cookie（用于前端展示）
// 仅展示总长度和首尾几位，中间以 * 号遮蔽
func (c *Config) GetDoubanCookieMasked() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cookie := c.Secrets.DoubanCookie
	if cookie == "" {
		return ""
	}
	if len(cookie) <= 16 {
		return strings.Repeat("*", len(cookie))
	}
	return cookie[:8] + strings.Repeat("*", 12) + cookie[len(cookie)-8:]
}

// SetDoubanCookie 设置豆瓣 Cookie 并持久化到配置文件
func (c *Config) SetDoubanCookie(cookie string) error {
	c.mu.Lock()
	c.Secrets.DoubanCookie = cookie
	c.mu.Unlock()

	viper.Set("secrets.douban_cookie", cookie)

	// 同时更新分片配置文件（如果存在），避免重启后被旧的空值覆盖
	c.updateSecretsFile("douban_cookie", cookie)

	return c.saveConfig()
}

// ClearDoubanCookie 清除豆瓣 Cookie 并持久化
func (c *Config) ClearDoubanCookie() error {
	return c.SetDoubanCookie("")
}

// SaveAdultScraperConfig 将当前 AdultScraper 配置持久化到配置文件
// 调用前应先在内存中更新 c.AdultScraper 字段，本方法只负责同步到 viper 并写盘
func (c *Config) SaveAdultScraperConfig() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ac := c.AdultScraper
	// 同步到 viper（使用 adult_scraper.* 命名空间，与 mapstructure 标签保持一致）
	viper.Set("adult_scraper.enabled", ac.Enabled)
	viper.Set("adult_scraper.enable_javbus", ac.EnableJavBus)
	viper.Set("adult_scraper.javbus_url", ac.JavBusURL)
	viper.Set("adult_scraper.enable_javdb", ac.EnableJavDB)
	viper.Set("adult_scraper.javdb_url", ac.JavDBURL)
	viper.Set("adult_scraper.python_service_url", ac.PythonServiceURL)
	viper.Set("adult_scraper.python_service_api_key", ac.PythonServiceAPIKey)
	viper.Set("adult_scraper.auto_start_python", ac.AutoStartPython)
	viper.Set("adult_scraper.python_executable", ac.PythonExecutable)
	viper.Set("adult_scraper.python_service_dir", ac.PythonServiceDir)
	viper.Set("adult_scraper.min_request_interval", ac.MinRequestInterval)
	viper.Set("adult_scraper.max_request_interval", ac.MaxRequestInterval)

	// Cookie 登录（参考 mdcx 的设计，每个站点一个完整 Cookie 字符串）
	viper.Set("adult_scraper.cookie_javbus", ac.CookieJavBus)
	viper.Set("adult_scraper.cookie_javdb", ac.CookieJavDB)
	viper.Set("adult_scraper.cookie_freejavbt", ac.CookieFreejavbt)
	viper.Set("adult_scraper.cookie_jav321", ac.CookieJav321)
	viper.Set("adult_scraper.cookie_fanza", ac.CookieFanza)
	viper.Set("adult_scraper.cookie_mgstage", ac.CookieMGStage)
	viper.Set("adult_scraper.cookie_fc2hub", ac.CookieFC2Hub)

	return c.saveConfig()
}

// SaveSTRMConfig 将当前 STRM 配置持久化到配置文件
func (c *Config) SaveSTRMConfig() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	sc := c.STRM
	viper.Set("strm.default_user_agent", sc.DefaultUserAgent)
	viper.Set("strm.default_referer", sc.DefaultReferer)
	viper.Set("strm.connect_timeout", sc.ConnectTimeout)
	viper.Set("strm.rewrite_hls", sc.RewriteHLS)
	viper.Set("strm.remote_probe", sc.RemoteProbe)
	viper.Set("strm.remote_probe_timeout", sc.RemoteProbeTimeout)
	viper.Set("strm.domain_user_agents", sc.DomainUserAgents)
	viper.Set("strm.domain_referers", sc.DomainReferers)
	return c.saveConfig()
}

// saveConfig 将当前配置写入配置文件
func (c *Config) saveConfig() error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = "config.yaml"
	}
	return viper.WriteConfigAs(configFile)
}

// updateSecretsFile 更新 config/secrets.yaml 分片文件中的指定字段
// 避免分片文件中的旧值在重启时覆盖用户通过 API 保存的新值
func (c *Config) updateSecretsFile(key, value string) {
	secretsDirs := []string{"./config", "./data/config", "/etc/nowen-video/config"}
	for _, dir := range secretsDirs {
		filePath := filepath.Join(dir, "secrets.yaml")
		if _, err := os.Stat(filePath); err != nil {
			continue
		}
		subViper := viper.New()
		subViper.SetConfigFile(filePath)
		if err := subViper.ReadInConfig(); err != nil {
			continue
		}
		subViper.Set(key, value)
		_ = subViper.WriteConfigAs(filePath)
		return // 只更新第一个找到的文件
	}
}

// ==================== 数据库 DSN 构造 ====================

// generateRandomSecret 生成随机密钥字符串
func generateRandomSecret(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	// 使用 crypto/rand 生成安全随机数
	if _, err := cryptoRand.Read(b); err != nil {
		// 降级使用时间戳（极端情况）
		for i := range b {
			b[i] = charset[i%len(charset)]
		}
		return string(b)
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

// GetDBDSN 返回 SQLite 连接字符串（含优化参数）
func (c *Config) GetDBDSN() string {
	dsn := c.Database.DBPath
	params := []string{}

	if c.Database.WALMode {
		params = append(params, "_journal_mode=WAL")
	}
	if c.Database.BusyTimeout > 0 {
		params = append(params, fmt.Sprintf("_busy_timeout=%d", c.Database.BusyTimeout))
	}
	if c.Database.CacheSize != 0 {
		params = append(params, fmt.Sprintf("_cache_size=%d", c.Database.CacheSize))
	}

	if len(params) > 0 {
		dsn += "?" + strings.Join(params, "&")
	}
	return dsn
}
