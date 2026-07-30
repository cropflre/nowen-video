package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPulseRemovedReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/pulse/dashboard", nil)

	(&PulseHandler{}).GetDashboard(ctx)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, recorder.Code)
	}
	if recorder.Header().Get("Deprecation") != "" {
		t.Fatal("retired endpoint must not advertise deprecated module metadata")
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("retired endpoint must behave like a missing route, got %q", recorder.Body.String())
	}
}
