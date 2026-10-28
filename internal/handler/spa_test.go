package handler

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSPAFallbackSeparatesMissingAPIsFromBrowserRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!DOCTYPE html><title>frontend</title>"), 0600); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.NoRoute(SPAFallback(dir))
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"GET", "/api", 404}, {"GET", "/api/unknown", 404}, {"POST", "/api/agent/unknown", 404}, {"GET", "/browse", 200}, {"GET", "/api-like-browser-route", 200}} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body)
			}
			if tc.status == 404 && (!strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") || !strings.Contains(w.Body.String(), "API_NOT_FOUND") || strings.Contains(w.Body.String(), "frontend")) {
				t.Fatalf("API returned frontend: %s", w.Body)
			}
			if tc.status == 200 && !strings.Contains(w.Body.String(), "frontend") {
				t.Fatalf("browser route lost SPA: %s", w.Body)
			}
			if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
				t.Fatal("fallback must not be cached")
			}
		})
	}
}
