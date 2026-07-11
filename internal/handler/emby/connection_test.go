package emby

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/config"
)

func newConnectionTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg:      &config.Config{},
		serverID: "0123456789ABCDEF0123456789ABCDEF01234567",
	}
	r := gin.New()
	RegisterRoutes(r, h, "test-jwt-secret")

	// 模拟生产环境 SPA fallback：缺失的 Emby 路由会返回 HTML 和 200，
	// 仅检查状态码无法发现客户端连接回归。
	r.NoRoute(func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<!doctype html><html></html>"))
	})
	return r
}

func performConnectionRequest(r http.Handler, method, path, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestOfficialClientSystemInfoRoutes(t *testing.T) {
	r := newConnectionTestRouter()

	paths := []string{
		"/System/Info/Public",
		"/system/info/public",
		"/emby/System/Info/Public",
		"/emby/system/info/public",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			w := performConnectionRequest(r, http.MethodGet, path, "", "")
			if w.Code != http.StatusOK {
				t.Fatalf("%s 状态码 = %d，期望 200", path, w.Code)
			}
			if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("%s 返回了非 JSON Content-Type: %s", path, w.Header().Get("Content-Type"))
			}

			var payload map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
				t.Fatalf("%s 返回内容不是合法 JSON: %v", path, err)
			}
			if payload["Id"] == "" || payload["ServerName"] == "" {
				t.Fatalf("%s 缺少 Emby 服务器标识字段: %s", path, w.Body.String())
			}
		})
	}
}

func TestOfficialClientAuxiliaryProbeRoutes(t *testing.T) {
	r := newConnectionTestRouter()

	w := performConnectionRequest(r, http.MethodGet, "/emby/system/wakeonlaninfo", "", "")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("WakeOnLanInfo 响应不兼容: code=%d body=%q", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("WakeOnLanInfo 应返回 JSON，实际为 %s", w.Header().Get("Content-Type"))
	}

	w = performConnectionRequest(r, http.MethodGet, "/emby/playback/bitratetest?Size=128", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("BitrateTest 状态码 = %d，期望 200", w.Code)
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/octet-stream") {
		t.Fatalf("BitrateTest Content-Type = %s", got)
	}
	if w.Body.Len() != 128 {
		t.Fatalf("BitrateTest 长度 = %d，期望 128", w.Body.Len())
	}

	w = performConnectionRequest(r, http.MethodGet, "/emby/web/manifest.json", "", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("manifest 未返回 JSON: code=%d content-type=%s", w.Code, w.Header().Get("Content-Type"))
	}
}

func TestOfficialClientLowercaseLoginRouteDoesNotFallBackToHTML(t *testing.T) {
	r := newConnectionTestRouter()
	w := performConnectionRequest(r, http.MethodPost, "/emby/users/authenticatebyname", "application/json", `{}`)

	if w.Code == http.StatusOK && strings.Contains(w.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("小写登录路由错误落入 SPA fallback: %s", w.Body.String())
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("空登录请求状态码 = %d，期望 401", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("登录错误应返回 JSON，实际为 %s", w.Header().Get("Content-Type"))
	}
}
