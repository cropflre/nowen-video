package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPulseRemovedReturnsGone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/pulse/dashboard", nil)

	(&PulseHandler{}).GetDashboard(ctx)

	if recorder.Code != http.StatusGone {
		t.Fatalf("expected %d, got %d", http.StatusGone, recorder.Code)
	}
	if recorder.Header().Get("Deprecation") != "true" {
		t.Fatal("expected deprecation header")
	}
	if body := recorder.Body.String(); body == "" || !contains(body, "pulse_removed") {
		t.Fatalf("expected pulse_removed response, got %q", body)
	}
}

func contains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
