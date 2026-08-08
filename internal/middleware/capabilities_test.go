package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/serverprofile"
)

func TestSecurityServesRegisteredPublicCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetPublicCapabilitiesProvider(func() (serverprofile.Manifest, error) {
		return serverprofile.Manifest{
			SchemaVersion: serverprofile.SchemaVersion,
			Profile:       "full",
			Capabilities: map[string]serverprofile.Capability{
				"preprocess": {Available: true, Enabled: true, Configured: true, Mode: "full"},
			},
		}, nil
	})
	defer SetPublicCapabilitiesProvider(nil)

	router := gin.New()
	router.Use(Security())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, recorder.Code)
	}
	body := recorder.Body.String()
	if body == "" || !stringsContain(body, `"profile":"full"`) || !stringsContain(body, `"preprocess"`) {
		t.Fatalf("unexpected capability response: %s", body)
	}
}

func TestSecurityDoesNotClaimCapabilitiesWithoutProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetPublicCapabilitiesProvider(nil)

	router := gin.New()
	router.Use(Security())
	router.GET("/api/capabilities", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "native-lite-route"})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !stringsContain(recorder.Body.String(), "native-lite-route") {
		t.Fatalf("unregistered provider must leave native route untouched: %s", recorder.Body.String())
	}
}

func stringsContain(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
