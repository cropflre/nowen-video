package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 日志级别常量
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// 日志类型常量
const (
	LogTypeAPI      = "api"      // API 请求日志
	LogTypePlayback = "playback" // 播放错误日志
	LogTypeSystem   = "system"   // 系统事件日志
)

// SystemLog 统一系统日志
type SystemLog struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	Type      string    `json:"type" gorm:"index;type:text;not null"`  // api / playback / system
	Level     string    `json:"level" gorm:"index;type:text;not null"` // debug / info / warn / error
	Message   string    `json:"message" gorm:"type:text"`
	Detail    string    `json:"detail" gorm:"type:text"` // JSON 格式的详细信息

	// API 请求相关字段
	Method     string `json:"method,omitempty" gorm:"type:text"`      // GET / POST / PUT / DELETE
	Path       string `json:"path,omitempty" gorm:"index;type:text"`  // 请求路径
	StatusCode int    `json:"status_code,omitempty" gorm:"index"`     // HTTP 状态码
	LatencyMs  int64  `json:"latency_ms,omitempty"`                   // 响应时间（毫秒）
	ClientIP   string `json:"client_ip,omitempty" gorm:"type:text"`   // 客户端 IP
	UserAgent  string `json:"user_agent,omitempty" gorm:"type:text"`  // User-Agent
	UserID     string `json:"user_id,omitempty" gorm:"index;type:text"` // 操作用户 ID
	Username   string `json:"username,omitempty" gorm:"type:text"`    // 操作用户名

	// 播放错误相关字段
	MediaID    string `json:"media_id,omitempty" gorm:"index;type:text"` // 关联媒体 ID
	MediaTitle string `json:"media_title,omitempty" gorm:"type:text"`    // 媒体标题

	// 系统事件相关字段
	Source string `json:"source,omitempty" gorm:"type:text"` // 事件来源（service / scheduler / startup 等）

	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

func (l *SystemLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}
