package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

// LogSystemEvent 便捷方法：记录系统事件日志
func (r *SystemLogRepo) LogSystemEvent(level, message, source, detail string) {
	_ = r.Create(&model.SystemLog{
		Type:      model.LogTypeSystem,
		Level:     level,
		Message:   message,
		Source:    source,
		Detail:    detail,
		CreatedAt: time.Now(),
	})
}

// LogInfo 便捷方法：记录 info 级别系统事件
func (r *SystemLogRepo) LogInfo(source, message string) {
	r.LogSystemEvent(model.LogLevelInfo, message, source, "")
}

// LogWarn 便捷方法：记录 warn 级别系统事件
func (r *SystemLogRepo) LogWarn(source, message, detail string) {
	r.LogSystemEvent(model.LogLevelWarn, message, source, detail)
}

// LogError 便捷方法：记录 error 级别系统事件
func (r *SystemLogRepo) LogError(source, message, detail string) {
	r.LogSystemEvent(model.LogLevelError, message, source, detail)
}
