package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

// RequestLogger 请求日志中间件 — 将所有 API 请求记录到 system_logs 表
// 排除高频/低价值路径（如 WebSocket、静态资源、健康检查等）
func RequestLogger(logRepo *repository.SystemLogRepo) gin.HandlerFunc {
	// 排除的路径前缀（不记录日志）
	excludePrefixes := []string{
		"/assets/",
		"/api/ws",
		"/api/stream/",             // 流媒体传输（高频）
		"/api/preprocess/media/",   // 预处理流媒体
		"/manifest.json",
		"/sw.js",
		"/favicon.ico",
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 跳过排除路径
		for _, prefix := range excludePrefixes {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		start := time.Now()

		// 执行后续处理
		c.Next()

		// 计算耗时
		latency := time.Since(start)
		statusCode := c.Writer.Status()

		// 确定日志级别
		level := model.LogLevelInfo
		if statusCode >= 500 {
			level = model.LogLevelError
		} else if statusCode >= 400 {
			level = model.LogLevelWarn
		}

		// 获取用户信息
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		userIDStr, _ := userID.(string)
		usernameStr, _ := username.(string)

		// 构建日志消息
		method := c.Request.Method
		message := method + " " + path + " " + http.StatusText(statusCode)

		log := &model.SystemLog{
			Type:       model.LogTypeAPI,
			Level:      level,
			Message:    message,
			Method:     method,
			Path:       path,
			StatusCode: statusCode,
			LatencyMs:  latency.Milliseconds(),
			ClientIP:   c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
			UserID:     userIDStr,
			Username:   usernameStr,
			CreatedAt:  time.Now(),
		}

		// 异步写入，不阻塞请求
		go func() {
			_ = logRepo.Create(log)
		}()
	}
}
