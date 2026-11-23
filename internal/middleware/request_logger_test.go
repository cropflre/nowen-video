package middleware

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLoggerDoesNotRecordQueryCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	r := gin.New()
	r.Use(RequestLogger(&output))
	r.GET("/api/media/:id/poster", func(c *gin.Context) { c.Status(204) })
	req := httptest.NewRequest("GET", "/api/media/sample/poster?token=fixture-private-token&api_key=fixture-private-key&sig=fixture-private-signature", nil)
	req.Header.Set("Authorization", "Bearer fixture-header-token")
	r.ServeHTTP(httptest.NewRecorder(), req)
	logged := output.String()
	if !strings.Contains(logged, "GET") || !strings.Contains(logged, "/api/media/sample/poster") || !strings.Contains(logged, "204") {
		t.Fatalf("missing useful request context: %q", logged)
	}
	for _, secret := range []string{"fixture-private-token", "fixture-private-key", "fixture-private-signature", "fixture-header-token", "?token="} {
		if strings.Contains(logged, secret) {
			t.Fatal("access log contains credential or query")
		}
	}
}

func TestRequestLoggerWithLevelErrorOnlyLogsFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	r := gin.New()
	r.Use(RequestLoggerWithLevel(&output, "error"))
	r.GET("/ok", func(c *gin.Context) { c.Status(200) })
	r.GET("/partial", func(c *gin.Context) { c.Status(206) })
	r.GET("/missing", func(c *gin.Context) { c.Status(404) })
	r.GET("/boom", func(c *gin.Context) { c.Status(500) })

	for _, path := range []string{"/ok", "/partial", "/missing?token=secret", "/boom"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}

	logged := output.String()
	if strings.Contains(logged, "/ok") || strings.Contains(logged, "/partial") {
		t.Fatalf("expected 2xx requests to be suppressed in error level, got: %q", logged)
	}
	if !strings.Contains(logged, "/missing") || !strings.Contains(logged, "404") {
		t.Fatalf("expected 404 request to be logged, got: %q", logged)
	}
	if !strings.Contains(logged, "/boom") || !strings.Contains(logged, "500") {
		t.Fatalf("expected 500 request to be logged, got: %q", logged)
	}
	if strings.Contains(logged, "secret") {
		t.Fatalf("query secret leaked into error access log: %q", logged)
	}
}
