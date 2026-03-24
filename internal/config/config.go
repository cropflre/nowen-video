package config

import (
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
	// 转码预设: veryfast / fast / medium
	TranscodePreset string `mapstructure:"transcode_preset"`
	// 最大并发转码任务数
	MaxTranscodeJobs int `mapstructure:"max_transcode_jobs"`
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

// Config 应用主配置（聚合所有子模块）
type Config struct {
	mu sync.RWMutex `mapstructure:"-"`

	// 子模块配置
	Database DatabaseConfig `mapstructure:"database"`
	Secrets  SecretsConfig  `mapstructure:"secrets"`
	App      AppConfig      `mapstructure:"app"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Cache    CacheConfig    `mapstructure:"cache"`

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

	return cfg, nil
}

// setDefaults 设置所有默认值
func setDefaults() {
	// ---- 数据库 ----
	viper.SetDefault("database.db_path", "./data/nowen.db")
	viper.SetDefault("database.wal_mode", true)
	viper.SetDefault("database.busy_timeout", 5000)
	viper.SetDefault("database.cache_size", -20000)
	viper.SetDefault("database.max_open_conns", 1)
	viper.SetDefault("database.max_idle_conns", 1)

	// ---- 密钥 ----
	viper.SetDefault("secrets.jwt_secret", "nowen-video-secret-change-me")
	viper.SetDefault("secrets.tmdb_api_key", "")

	// ---- 应用 ----
	viper.SetDefault("app.port", 8080)
	viper.SetDefault("app.debug", false)
	viper.SetDefault("app.env", "production")
	viper.SetDefault("app.data_dir", "./data")
	viper.SetDefault("app.web_dir", "./web/dist")
	viper.SetDefault("app.ffmpeg_path", "ffmpeg")
	viper.SetDefault("app.ffprobe_path", "ffprobe")
	viper.SetDefault("app.hw_accel", "auto")
	viper.SetDefault("app.vaapi_device", "/dev/dri/renderD128")
	viper.SetDefault("app.transcode_preset", "veryfast")
	viper.SetDefault("app.max_transcode_jobs", 2)
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

	// ---- 缓存 ----
	viper.SetDefault("cache.cache_dir", "./cache")
	viper.SetDefault("cache.max_disk_usage_mb", 0)
	viper.SetDefault("cache.ttl_hours", 0)
	viper.SetDefault("cache.auto_cleanup", false)
	viper.SetDefault("cache.cleanup_interval_min", 60)

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
	viper.SetDefault("hw_accel", "auto")
	viper.SetDefault("vaapi_device", "/dev/dri/renderD128")
	viper.SetDefault("transcode_preset", "veryfast")
	viper.SetDefault("max_transcode_jobs", 2)
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
			for _, key := range subViper.AllKeys() {
				fullKey := cf.prefix + "." + key
				viper.Set(fullKey, subViper.Get(key))
			}
		}
	}

	return nil
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
	if c.MaxTranscodeJobs != 0 && c.MaxTranscodeJobs != 2 {
		c.App.MaxTranscodeJobs = c.MaxTranscodeJobs
	}
	if c.App.MaxTranscodeJobs == 0 {
		c.App.MaxTranscodeJobs = 2
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

// IsDefaultJWTSecret 检查是否使用默认的 JWT Secret
func (c *Config) IsDefaultJWTSecret() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Secrets.JWTSecret == "nowen-video-secret-change-me"
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
	return c.saveConfig()
}

// ClearTMDbAPIKey 清除 TMDb API Key 并持久化
func (c *Config) ClearTMDbAPIKey() error {
	return c.SetTMDbAPIKey("")
}

// saveConfig 将当前配置写入配置文件
func (c *Config) saveConfig() error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = "config.yaml"
	}
	return viper.WriteConfigAs(configFile)
}

// ==================== 数据库 DSN 构造 ====================

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
