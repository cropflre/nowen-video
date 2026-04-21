package emby

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nowen-video/nowen-video/internal/middleware"
)

// EmbyAuthHeader 解析 X-Emby-Authorization / Authorization 头中的 MediaBrowser 参数。
// 格式示例：
//
//	MediaBrowser Client="Infuse-Direct", Device="iPhone", DeviceId="xxx",
//	             Version="7.6.4", Token="abcdef"
type EmbyAuthHeader struct {
	Client             string
	Device             string
	DeviceId           string
	Version            string
	Token              string
	UserId             string
}

// parseEmbyAuthHeader 解析 MediaBrowser 风格的 KV header。
// 兼容无前缀（纯 K="V" 列表）和带 "MediaBrowser" 前缀的格式。
func parseEmbyAuthHeader(raw string) EmbyAuthHeader {
	h := EmbyAuthHeader{}
	if raw == "" {
		return h
	}
	// 去掉 MediaBrowser / Emby 前缀
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "MediaBrowser ") {
		raw = strings.TrimPrefix(raw, "MediaBrowser ")
	} else if strings.HasPrefix(raw, "Emby ") {
		raw = strings.TrimPrefix(raw, "Emby ")
	}

	// 解析逗号分隔的 Key="Value"（Value 内可含空格，但通常不含引号或逗号）
	for _, part := range splitCSVRespectingQuotes(raw) {
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(part[:eq])
		v := strings.TrimSpace(part[eq+1:])
		// 去掉引号
		v = strings.Trim(v, "\"")
		switch strings.ToLower(k) {
		case "client":
			h.Client = v
		case "device":
			h.Device = v
		case "deviceid":
			h.DeviceId = v
		case "version":
			h.Version = v
		case "token":
			h.Token = v
		case "userid":
			h.UserId = v
		}
	}
	return h
}

// splitCSVRespectingQuotes 按逗号切分，但跳过引号内部的逗号。
func splitCSVRespectingQuotes(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			inQuote = !inQuote
			cur.WriteByte(c)
		case ',':
			if inQuote {
				cur.WriteByte(c)
			} else {
				out = append(out, strings.TrimSpace(cur.String()))
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		out = append(out, strings.TrimSpace(cur.String()))
	}
	return out
}

// extractToken 从请求中按优先级提取 Emby 客户端 Token：
//  1. X-Emby-Token（最常见）
//  2. X-MediaBrowser-Token
//  3. X-Emby-Authorization / Authorization 头中的 Token="..."
//  4. api_key / ApiKey 查询参数
//  5. X-Emby-ApiKey header
//
// 同时返回解析出的 EmbyAuthHeader，用于日志与会话建档。
func extractToken(c *gin.Context) (string, EmbyAuthHeader) {
	if t := c.GetHeader("X-Emby-Token"); t != "" {
		return t, parseEmbyAuthHeader(c.GetHeader("X-Emby-Authorization"))
	}
	if t := c.GetHeader("X-MediaBrowser-Token"); t != "" {
		return t, parseEmbyAuthHeader(c.GetHeader("X-Emby-Authorization"))
	}

	// 从 X-Emby-Authorization / Authorization 解析
	for _, hdr := range []string{"X-Emby-Authorization", "Authorization"} {
		v := c.GetHeader(hdr)
		if v == "" {
			continue
		}
		ah := parseEmbyAuthHeader(v)
		if ah.Token != "" {
			return ah.Token, ah
		}
	}

	// Query 参数
	for _, key := range []string{"api_key", "ApiKey", "X-Emby-Token"} {
		if t := c.Query(key); t != "" {
			return t, EmbyAuthHeader{}
		}
	}
	if t := c.GetHeader("X-Emby-ApiKey"); t != "" {
		return t, EmbyAuthHeader{}
	}
	return "", parseEmbyAuthHeader(c.GetHeader("X-Emby-Authorization"))
}

// EmbyAuth 是对 Emby/Infuse 客户端的认证中间件。
// 复用 nowen 自身的 JWT 令牌体系——客户端通过 /Users/AuthenticateByName 拿到的就是 JWT，
// 之后所有请求都用这个 JWT 当 Emby Token 即可。
//
// 成功时写入 context：
//   - user_id:   nowen 用户 UUID
//   - username:  用户名
//   - role:      admin / user
//   - emby_client / emby_device / emby_device_id / emby_version: 客户端标识
func EmbyAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, ah := extractToken(c)
		if tokenStr == "" {
			abortEmbyUnauthorized(c, "No token supplied")
			return
		}

		claims := &middleware.Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			abortEmbyUnauthorized(c, "Invalid or expired token")
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("emby_client", ah.Client)
		c.Set("emby_device", ah.Device)
		c.Set("emby_device_id", ah.DeviceId)
		c.Set("emby_version", ah.Version)
		c.Next()
	}
}

// abortEmbyUnauthorized 返回符合 Emby 客户端期望的 401。
// Emby 客户端在遇到 401 时会自动跳到登录页重试，因此这里不能返回 HTML 错误页。
func abortEmbyUnauthorized(c *gin.Context, reason string) {
	c.Header("X-Application-Error-Code", "Unauthorized")
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"Error":   "Unauthorized",
		"Message": reason,
	})
}

// EmbyCORS 针对 Emby 客户端（Web UI / 某些桌面端）输出额外的 CORS 头。
// Infuse/iOS/Android 原生客户端不走浏览器同源，因此这里主要给 Web 客户端用。
// 与项目已有的全局 CORS 中间件叠加时只需要补齐 Emby 自定义 Header。
func EmbyCORS() gin.HandlerFunc {
	// Emby Web / Jellyfin Web 会发这些自定义 Header
	allowHeaders := strings.Join([]string{
		"Content-Type",
		"Authorization",
		"Range",
		"X-Emby-Authorization",
		"X-Emby-Token",
		"X-Emby-ApiKey",
		"X-Emby-Client",
		"X-Emby-Client-Version",
		"X-Emby-Device-Id",
		"X-Emby-Device-Name",
		"X-Emby-Language",
		"X-MediaBrowser-Token",
	}, ", ")
	exposeHeaders := strings.Join([]string{
		"Content-Length",
		"Content-Range",
		"Accept-Ranges",
		"ETag",
		"Last-Modified",
		"X-Application-Error-Code",
	}, ", ")

	return func(c *gin.Context) {
		// 仅当请求来自浏览器（带 Origin）时输出 CORS 头；否则不干扰原生客户端。
		if origin := c.GetHeader("Origin"); origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, HEAD, OPTIONS")
			c.Header("Access-Control-Allow-Headers", allowHeaders)
			c.Header("Access-Control-Expose-Headers", exposeHeaders)
			c.Header("Access-Control-Max-Age", "86400")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// getIntQuery 从 query 中读取整数参数，缺省返回 def。
func getIntQuery(c *gin.Context, key string, def int) int {
	s := c.Query(key)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
