package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// LogSanitizer 日志脱敏中间件
//
// 业务请求中的 query/header 必须保持原样。此前这里直接把 Authorization、
// X-Emby-Token 等字段替换为 ***REDACTED***，导致后续 JWT/Emby 鉴权永远失败。
// 当前请求日志只记录 method/path/status，并不会记录这些敏感字段；如果将来需要
// 记录额外文本，应在写日志的最后一步调用 SanitizeLogMessage，而不能修改请求。
func LogSanitizer() gin.HandlerFunc {
	return func(c *gin.Context) {
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
