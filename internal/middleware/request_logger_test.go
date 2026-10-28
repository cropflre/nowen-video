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
