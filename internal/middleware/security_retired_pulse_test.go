package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityRedirectsRetiredPulseRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, path := range []string{"/pulse", "/pulse/analytics", "/pulse/dashboard/trends"} {
		router := gin.New()
		router.Use(Security())
		router.GET(path, func(c *gin.Context) {
			c.String(http.StatusOK, "legacy pulse page")
		})

		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusTemporaryRedirect {
			t.Fatalf("path=%s expected %d, got %d", path, http.StatusTemporaryRedirect, response.Code)
		}
		if location := response.Header().Get("Location"); location != "/admin" {
			t.Fatalf("path=%s expected redirect to /admin, got %q", path, location)
		}
		if cacheControl := response.Header().Get("Cache-Control"); cacheControl == "" {
			t.Fatalf("path=%s must disable cache", path)
		}
	}
}

func TestSecurityDisablesServiceWorkerCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Security())
	router.GET("/sw.js", func(c *gin.Context) {
		c.String(http.StatusOK, "service worker")
	})

	request := httptest.NewRequest(http.MethodGet, "/sw.js", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}
	if got := response.Header().Get("Service-Worker-Allowed"); got != "/" {
		t.Fatalf("expected Service-Worker-Allowed=/, got %q", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate" {
		t.Fatalf("unexpected Cache-Control: %q", got)
	}
}

func TestSecurityKeepsUnrelatedRoutesAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Security())
	router.GET("/admin", func(c *gin.Context) {
		c.String(http.StatusOK, "admin")
	})

	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}
}
