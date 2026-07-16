package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestLogSanitizer_PreservesQueryParamsForHandlers(t *testing.T) {
	// 设置 Gin 测试模式
	gin.SetMode(gin.TestMode)

	// 创建测试路由
	router := gin.New()
	router.Use(LogSanitizer())
	router.GET("/test", func(c *gin.Context) {
		// 返回原始 query 参数（解码后）
		c.JSON(http.StatusOK, gin.H{
			"api_key": c.Query("api_key"),
			"normal":  c.Query("normal"),
		})
	})

	// 测试 api_key 参数
	req := httptest.NewRequest("GET", "/test?api_key=my-secret-key&normal=value", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// 中间件不能破坏业务输入；真正写日志时再对日志副本脱敏。
	assert.Contains(t, w.Body.String(), "my-secret-key")
	assert.Contains(t, w.Body.String(), "value")
}

func TestLogSanitizer_PreservesAuthenticationHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(LogSanitizer())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"authorization":        c.Request.Header.Get("Authorization"),
			"x-emby-token":         c.Request.Header.Get("X-Emby-Token"),
			"x-mediabrowser-token": c.Request.Header.Get("X-MediaBrowser-Token"),
		})
	})

	// 测试 X-Emby-Token header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer signed-jwt")
	req.Header.Set("X-Emby-Token", "my-secret-token")
	req.Header.Set("X-MediaBrowser-Token", "my-secret-token-2")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Bearer signed-jwt")
	assert.Contains(t, w.Body.String(), "my-secret-token")
	assert.Contains(t, w.Body.String(), "my-secret-token-2")
}

func TestLogSanitizer_PreservesEmbyAuthorizationForHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(LogSanitizer())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"authorization": c.Request.Header.Get("X-Emby-Authorization"),
		})
	})

	// 测试 X-Emby-Authorization 中的 Token
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Emby-Authorization", `MediaBrowser Client="Infuse", Token="my-secret-token"`)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "my-secret-token")
}

func TestLogSanitizer_NormalParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(LogSanitizer())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"normal": c.Query("normal"),
		})
	})

	// 测试普通参数不被脱敏
	req := httptest.NewRequest("GET", "/test?normal=value", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "value")
	assert.NotContains(t, w.Body.String(), "***REDACTED***")
}

func TestSanitizeLogMessage(t *testing.T) {
	// 测试 api_key
	msg := "Request with api_key=my-secret-key&other=value"
	sanitized := SanitizeLogMessage(msg)
	assert.Contains(t, sanitized, "***REDACTED***")
	assert.NotContains(t, sanitized, "my-secret-key")

	// 测试 Password
	msg = "Login with Password=my-password"
	sanitized = SanitizeLogMessage(msg)
	assert.Contains(t, sanitized, "***REDACTED***")
	assert.NotContains(t, sanitized, "my-password")

	// 测试普通消息
	msg = "Normal log message"
	sanitized = SanitizeLogMessage(msg)
	assert.Equal(t, msg, sanitized)
}

func TestSanitizeEmbyAuth(t *testing.T) {
	// 测试包含 Token 的 Authorization
	auth := `MediaBrowser Client="Infuse", Token="my-secret-token"`
	sanitized := sanitizeEmbyAuth(auth)
	assert.Contains(t, sanitized, "***REDACTED***")
	assert.NotContains(t, sanitized, "my-secret-token")

	// 测试不包含 Token 的 Authorization
	auth = `MediaBrowser Client="Infuse", Device="iPhone"`
	sanitized = sanitizeEmbyAuth(auth)
	assert.Equal(t, auth, sanitized)

	// 测试空字符串
	assert.Equal(t, "", sanitizeEmbyAuth(""))
}
