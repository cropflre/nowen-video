package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// 敏感参数列表
var sensitiveKeys = []string{
	"api_key",
	"ApiKey",
	"X-Emby-Token",
	"X-MediaBrowser-Token",
	"Pw",
	"Password",
	"password",
	"Authorization",
	"X-Emby-ApiKey",
}

// LogSanitizer 日志脱敏中间件
// 脱敏敏感参数，避免 token、密码等出现在日志中
func LogSanitizer() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 脱敏 query 参数
		query := c.Request.URL.Query()
		modified := false
		for _, key := range sensitiveKeys {
			if query.Has(key) {
				query.Set(key, "***REDACTED***")
				modified = true
			}
		}
		if modified {
			c.Request.URL.RawQuery = query.Encode()
		}

		// 脱敏 header
		for _, key := range sensitiveKeys {
			if c.Request.Header.Get(key) != "" {
				c.Request.Header.Set(key, "***REDACTED***")
			}
		}

		// 脱敏 X-Emby-Authorization 中的 Token
		if auth := c.Request.Header.Get("X-Emby-Authorization"); auth != "" {
			sanitized := sanitizeEmbyAuth(auth)
			if sanitized != auth {
				c.Request.Header.Set("X-Emby-Authorization", sanitized)
			}
		}

		c.Next()
	}
}

// sanitizeEmbyAuth 脱敏 Emby Authorization 头中的 Token
func sanitizeEmbyAuth(auth string) string {
	// 格式: MediaBrowser Client="xxx", Token="xxx", ...
	// 需要脱敏 Token="xxx" 部分

	// 简单实现：如果包含 Token=，将其替换
	if strings.Contains(auth, "Token=") {
		// 找到 Token= 的位置
		idx := strings.Index(auth, "Token=")
		if idx >= 0 {
			// 找到 Token 值的结束位置（下一个逗号或结尾）
			endIdx := len(auth)
			commaIdx := strings.Index(auth[idx:], ",")
			if commaIdx >= 0 {
				endIdx = idx + commaIdx
			}

			// 替换 Token 值
			return auth[:idx+6] + "***REDACTED***" + auth[endIdx:]
		}
	}
	return auth
}

// SanitizeLogMessage 脱敏日志消息中的敏感信息
func SanitizeLogMessage(message string) string {
	sensitivePatterns := []struct {
		key   string
		value string
	}{
		{"api_key=", "***REDACTED***"},
		{"ApiKey=", "***REDACTED***"},
		{"X-Emby-Token=", "***REDACTED***"},
		{"Pw=", "***REDACTED***"},
		{"Password=", "***REDACTED***"},
		{"password=", "***REDACTED***"},
		{"Authorization=", "***REDACTED***"},
	}

	result := message
	for _, p := range sensitivePatterns {
		if strings.Contains(result, p.key) {
			// 找到 key= 的位置
			idx := strings.Index(result, p.key)
			if idx >= 0 {
				// 找到值的结束位置（下一个空格或结尾）
				endIdx := len(result)
				spaceIdx := strings.Index(result[idx+len(p.key):], " ")
				if spaceIdx >= 0 {
					endIdx = idx + len(p.key) + spaceIdx
				}

				// 替换值
				result = result[:idx+len(p.key)] + "***REDACTED***" + result[endIdx:]
			}
		}
	}

	return result
}
