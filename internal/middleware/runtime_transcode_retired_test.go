package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAdminOnlyRetiresPersistentRuntimeTranscodeAfterAuthorization(t *testing.T) {
	for _, path := range []string{
		"/api/admin/transcode/status",
		"/api/admin/transcode-tasks",
		"/api/admin/tasks/transcode/legacy-task/retry",
	} {
		t.Run(path, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("role", "admin")
				c.Next()
			})
			router.Any(path, AdminOnly(), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, path, nil)
			router.ServeHTTP(response, request)

			if response.Code != http.StatusGone {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusGone)
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["code"] != "persistent_runtime_transcode_retired" {
				t.Fatalf("unexpected tombstone response: %+v", body)
			}
			if got := response.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("cache control = %q", got)
			}
		})
	}
}

func TestAdminOnlyKeepsAuthorizationBeforeRetirement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	router.GET("/api/admin/transcode-tasks", AdminOnly(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/transcode-tasks", nil)
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want authorization failure %d", response.Code, http.StatusForbidden)
	}
}

func TestAdminOnlyDoesNotRetirePreprocessAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.GET("/api/admin/preprocess/tasks", AdminOnly(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/preprocess/tasks", nil)
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("preprocess status = %d, want %d", response.Code, http.StatusNoContent)
	}
}
