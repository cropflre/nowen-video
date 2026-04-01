package middleware

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Security 安全头中间件
func Security() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		// CSP: 允许自身、blob、data URI（视频播放需要）
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' 'unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob: https:; "+
				"media-src 'self' blob:; "+
				"connect-src 'self' ws: wss:; "+
				"font-src 'self' data:;")

		c.Next()
	}
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	MaxRequests  int           // 窗口期内最大请求数
	Window       time.Duration // 滑动窗口时长
	ExcludePaths []string      // 排除的路径前缀（不受速率限制）
}

// RateLimit 速率限制中间件（滑动窗口算法，带路径排除和自动清理）
// maxRequestsPerMinute: 每分钟最大请求数
func RateLimit(maxRequestsPerMinute int) gin.HandlerFunc {
	return RateLimitWithConfig(RateLimitConfig{
		MaxRequests: maxRequestsPerMinute,
		Window:      time.Minute,
	})
}

// RateLimitWithConfig 可配置的速率限制中间件（滑动窗口算法）
func RateLimitWithConfig(cfg RateLimitConfig) gin.HandlerFunc {
	type visitor struct {
		timestamps []time.Time // 请求时间戳列表（滑动窗口）
	}

	var mu sync.Mutex
	visitors := make(map[string]*visitor)

	// 启动后台清理协程，每2分钟清理过期条目，防止内存泄漏
	go func() {
		for {
			time.Sleep(2 * time.Minute)
			now := time.Now()
			mu.Lock()
			for ip, v := range visitors {
				// 清理过期的时间戳
				cutoff := now.Add(-cfg.Window)
				valid := v.timestamps[:0]
				for _, ts := range v.timestamps {
					if ts.After(cutoff) {
						valid = append(valid, ts)
					}
				}
				if len(valid) == 0 {
					delete(visitors, ip)
				} else {
					v.timestamps = valid
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		// 检查路径是否在排除列表中
		path := c.Request.URL.Path
		for _, prefix := range cfg.ExcludePaths {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		ip := c.ClientIP()
		now := time.Now()
		cutoff := now.Add(-cfg.Window)

		mu.Lock()
		v, exists := visitors[ip]
		if !exists {
			v = &visitor{}
			visitors[ip] = v
		}

		// 滑动窗口：移除过期的时间戳
		valid := v.timestamps[:0]
		for _, ts := range v.timestamps {
			if ts.After(cutoff) {
				valid = append(valid, ts)
			}
		}
		v.timestamps = valid

		// 检查是否超过限制
		if len(v.timestamps) >= cfg.MaxRequests {
			// 计算最早请求过期的时间，作为 Retry-After
			oldest := v.timestamps[0]
			retryAfter := oldest.Add(cfg.Window).Sub(now).Seconds()
			if retryAfter < 1 {
				retryAfter = 1
			}
			mu.Unlock()
			c.Header("Retry-After", fmt.Sprintf("%.0f", retryAfter))
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", cfg.MaxRequests))
			c.Header("X-RateLimit-Remaining", "0")
			c.AbortWithStatus(429)
			return
		}

		// 记录本次请求
		v.timestamps = append(v.timestamps, now)
		remaining := cfg.MaxRequests - len(v.timestamps)
		mu.Unlock()

		// 添加速率限制信息头，方便前端感知
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", cfg.MaxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Next()
	}
}

// RequestID 请求 ID 中间件
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// generateRequestID 生成简单的请求 ID
func generateRequestID() string {
	return strings.Replace(
		time.Now().Format("20060102150405.000000"),
		".", "", 1,
	)
}
