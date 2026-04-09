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
	// 硬件加速模式: auto / qsv / vaapi / nvenc / none
	HWAccel string `mapstructure:"hw_accel"`
	// VAAPI 设备路径，如 /dev/dri/renderD128
	VAAPIDevice string `mapstructure:"vaapi_device"`
	// 转码预设: ultrafast / veryfast / fast / medium
	TranscodePreset string `mapstructure:"transcode_preset"`
	// 最大并发转码任务数
	MaxTranscodeJobs int `mapstructure:"max_transcode_jobs"`
	// CPU 资源使用率上限（百分比，1~80），系统会自动保留 20% 缓冲
	// 例如设为 80 表示 CPU 使用率超过 80% 时暂停转码任务
	// 默认: 5（最低配置）
	ResourceLimit int `mapstructure:"resource_limit"`
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

	// ==================== ASR / Whisper 配置 ====================
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

	// ==================== 兼容性字段（向后兼容旧的扁平配置） ====================
	// 以下字段用于兼容旧版 config.yaml 中的扁平 key，
	// 加载后会自动合并到对应的子模块中。

	// 旧版兼容 - 数据库
	DBPath string `mapstructure:"db_path"`
	// 旧版兼容 - 密钥
	JWTSecret  string `mapstructure:"jwt_secret"`
	TMDbAPIKey string `mapstructure:"tmdb_api_key"`
	// 旧版兼容 - 应用
	Port             int      `mapstructure:"port"`
	Debug            bool     `mapstructure:"debug"`
	DataDir          string   `mapstructure:"data_dir"`
	WebDir           string   `mapstructure:"web_dir"`
	CacheDir         string   `mapstructure:"cache_dir"`
	FFmpegPath       string   `mapstructure:"ffmpeg_path"`
	FFprobePath      string   `mapstructure:"ffprobe_path"`
	HWAccel          string   `mapstructure:"hw_accel"`
	VAAPIDevice      string   `mapstructure:"vaapi_device"`
	TranscodePreset  string   `mapstructure:"transcode_preset"`
	MaxTranscodeJobs int      `mapstructure:"max_transcode_jobs"`
	ResourceLimit    int      `mapstructure:"resource_limit"`
	CORSOrigins      []string `mapstructure:"cors_origins"`
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

	// ---- 应用 ----
	viper.SetDefault("app.port", 8080)
	viper.SetDefault("app.debug", false)
	viper.SetDefault("app.env", "production")
	viper.SetDefault("app.data_dir", "./data")
	viper.SetDefault("app.web_dir", "./web/dist")
	viper.SetDefault("app.ffmpeg_path", "ffmpeg")
	viper.SetDefault("app.ffprobe_path", "ffprobe")
	viper.SetDefault("app.hw_accel", "none")
	viper.SetDefault("app.vaapi_device", "/dev/dri/renderD128")
	viper.SetDefault("app.transcode_preset", "ultrafast")
	viper.SetDefault("app.max_transcode_jobs", 1)
	viper.SetDefault("app.resource_limit", 5)
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

	// ---- 缓存 ----
	viper.SetDefault("cache.cache_dir", "./cache")
	viper.SetDefault("cache.max_disk_usage_mb", 0)
	viper.SetDefault("cache.ttl_hours", 0)
	viper.SetDefault("cache.auto_cleanup", false)
	viper.SetDefault("cache.cleanup_interval_min", 60)

	// ---- 注册控制 ----
	viper.SetDefault("registration.enabled", false)
	viper.SetDefault("registration.invite_code", "")

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
	viper.SetDefault("hw_accel", "none")
	viper.SetDefault("vaapi_device", "/dev/dri/renderD128")
	viper.SetDefault("transcode_preset", "ultrafast")
	viper.SetDefault("max_transcode_jobs", 1)
	viper.SetDefault("resource_limit", 5)
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
	if c.Port != 0 && c.Port != 8080 {
		c.App.Port = c.Port
	}
	if c.App.Port == 0 {
		c.App.Port = 8080
	}
	if c.Debug {
		c.App.Debug = true
	}
	if c.DataDir != "" && c.DataDir != "./data" {
		c.App.DataDir = c.DataDir
	}
	if c.App.DataDir == "" {
		c.App.DataDir = "./data"
	}
	if c.WebDir != "" && c.WebDir != "./web/dist" {
		c.App.WebDir = c.WebDir
	}
	if c.App.WebDir == "" {
		c.App.WebDir = "./web/dist"
	}
	if c.FFmpegPath != "" && c.FFmpegPath != "ffmpeg" {
		c.App.FFmpegPath = c.FFmpegPath
	}
	if c.App.FFmpegPath == "" {
		c.App.FFmpegPath = "ffmpeg"
	}
	if c.FFprobePath != "" && c.FFprobePath != "ffprobe" {
		c.App.FFprobePath = c.FFprobePath
	}
	if c.App.FFprobePath == "" {
		c.App.FFprobePath = "ffprobe"
	}
	if c.HWAccel != "" && c.HWAccel != "auto" {
		c.App.HWAccel = c.HWAccel
	}
	if c.App.HWAccel == "" {
		c.App.HWAccel = "auto"
	}
	if c.VAAPIDevice != "" && c.VAAPIDevice != "/dev/dri/renderD128" {
		c.App.VAAPIDevice = c.VAAPIDevice
	}
	if c.App.VAAPIDevice == "" {
		c.App.VAAPIDevice = "/dev/dri/renderD128"
	}
	if c.TranscodePreset != "" && c.TranscodePreset != "veryfast" {
		c.App.TranscodePreset = c.TranscodePreset
	}
	if c.App.TranscodePreset == "" {
		c.App.TranscodePreset = "veryfast"
	}
	if c.MaxTranscodeJobs != 0 && c.MaxTranscodeJobs != 1 {
		c.App.MaxTranscodeJobs = c.MaxTranscodeJobs
	}
	if c.App.MaxTranscodeJobs == 0 {
		c.App.MaxTranscodeJobs = 1
	}
	// 资源限制：允许用户配置 1~80，系统自动保留 20% 缓冲
	if c.ResourceLimit != 0 && c.ResourceLimit != 80 {
		c.App.ResourceLimit = c.ResourceLimit
	}
	if c.App.ResourceLimit <= 0 {
		c.App.ResourceLimit = 80
	}
	if c.App.ResourceLimit > 80 {
		c.App.ResourceLimit = 80 // 上限 80%，保留 20% 缓冲
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

// UpdatePerformanceConfig 更新性能配置并持久化
// 支持动态修改 resource_limit、max_transcode_jobs、transcode_preset、hw_accel
func (c *Config) UpdatePerformanceConfig(updates map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, val := range updates {
		switch key {
		case "resource_limit":
			if v, ok := val.(float64); ok {
				limit := int(v)
				if limit < 1 {
					limit = 1
				}
				if limit > 80 {
					limit = 80
				}
				c.App.ResourceLimit = limit
				viper.Set("app.resource_limit", limit)
			}
		case "max_transcode_jobs":
			if v, ok := val.(float64); ok {
				jobs := int(v)
				if jobs < 1 {
					jobs = 1
				}
				if jobs > 16 {
					jobs = 16
				}
				c.App.MaxTranscodeJobs = jobs
				viper.Set("app.max_transcode_jobs", jobs)
			}
		case "transcode_preset":
			if v, ok := val.(string); ok {
				validPresets := map[string]bool{
					"ultrafast": true, "superfast": true, "veryfast": true,
					"faster": true, "fast": true, "medium": true,
					"slow": true, "slower": true, "veryslow": true,
				}
				if validPresets[v] {
					c.App.TranscodePreset = v
					viper.Set("app.transcode_preset", v)
				}
			}
		case "hw_accel":
			if v, ok := val.(string); ok {
				validAccels := map[string]bool{
					"auto": true, "nvenc": true, "qsv": true,
					"vaapi": true, "none": true,
				}
				if validAccels[v] {
					c.App.HWAccel = v
					viper.Set("app.hw_accel", v)
				}
			}
		}
	}

	// 同时更新分片配置文件
	c.updateAppConfigFile(updates)

	return c.saveConfig()
}

// GetPerformanceConfig 获取当前性能配置
func (c *Config) GetPerformanceConfig() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]interface{}{
		"resource_limit":     c.App.ResourceLimit,
		"max_transcode_jobs": c.App.MaxTranscodeJobs,
		"transcode_preset":   c.App.TranscodePreset,
		"hw_accel":           c.App.HWAccel,
		"vaapi_device":       c.App.VAAPIDevice,
	}
}

// saveConfig 将当前配置写入配置文件
func (c *Config) saveConfig() error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = "config.yaml"
	}
	return viper.WriteConfigAs(configFile)
}

// updateAppConfigFile 更新 config/app.yaml 分片文件中的性能配置字段
func (c *Config) updateAppConfigFile(updates map[string]interface{}) {
	appDirs := []string{"./config", "./data/config", "/etc/nowen-video/config"}
	for _, dir := range appDirs {
		filePath := filepath.Join(dir, "app.yaml")
		if _, err := os.Stat(filePath); err != nil {
			continue
		}
		subViper := viper.New()
		subViper.SetConfigFile(filePath)
		if err := subViper.ReadInConfig(); err != nil {
			continue
		}
		for key, val := range updates {
			subViper.Set(key, val)
		}
		_ = subViper.WriteConfigAs(filePath)
		return
	}
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
