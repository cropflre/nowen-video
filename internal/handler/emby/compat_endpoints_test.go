package emby

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newCompatEndpointTestRouter(t *testing.T) (*gin.Engine, *Handler, *gorm.DB, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.User{}, &model.Library{}, &model.Media{}, &model.Series{}); err != nil {
		t.Fatal(err)
	}
	fixtures := []interface{}{
		&model.User{ID: "compat-user", Username: "compat-user", Password: "unused", Role: "user"},
		&model.Library{ID: "compat-library", Name: "Compatibility Library", Path: "/media", Type: "movie"},
		&model.Media{ID: "compat-movie", LibraryID: "compat-library", Title: "Compat Movie", FilePath: "/media/movie.mp4", MediaType: "movie", Year: 2025, Genres: "Drama", MPAA: "PG"},
		&model.Series{ID: "compat-series", LibraryID: "compat-library", Title: "Compat Series", FolderPath: "/media/series", Year: 2024},
	}
	for _, fixture := range fixtures {
		if err := db.Create(fixture).Error; err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{}
	cfg.Secrets.JWTSecret = "compat-endpoint-test-secret"
	cfg.Emby.EnableWebSocket = true
	repos := repository.NewRepositories(db)
	logger := zap.NewNop().Sugar()
	auth := service.NewAuthService(repos.User, repos.InviteCode, repos.LoginLog, repos.AuditLog, cfg, logger)
	h := NewHandler(cfg, logger, auth, nil, repos)
	r := gin.New()
	RegisterRoutes(r, h, cfg.Secrets.JWTSecret)
	r.NoRoute(func(c *gin.Context) { c.Data(200, "text/html", []byte("SPA fallback")) })
	claims := middleware.Claims{
		UserID: "compat-user", Username: "compat-user", Role: "user",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.Secrets.JWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	return r, h, db, token
}

func compatEndpointRequest(r *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("X-Emby-Token", token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emby-Authorization", `MediaBrowser Client="Hills", DeviceId="compat-device"`)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCompatClientStartupEndpoints(t *testing.T) {
	r, h, _, token := newCompatEndpointTestRouter(t)
	for _, prefix := range []string{"", "/emby"} {
		for _, path := range []string{"/Sessions/Capabilities", "/Sessions/Capabilities/Full", "/sessions/capabilities/full"} {
			w := compatEndpointRequest(r, http.MethodPost, prefix+path, token,
				`{"DeviceProfile":{"DirectPlayProfiles":[{"Container":"mkv","VideoCodec":"hevc"}]}}`)
			if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
				t.Fatalf("capability handshake %s: status=%d body=%s", prefix+path, w.Code, w.Body)
			}
		}
		for _, path := range []string{"/Search/Hints?SearchTerm=Compat", "/Items/Filters", "/Items/Filters2", "/Library/VirtualFolders"} {
			w := compatEndpointRequest(r, http.MethodGet, prefix+path, token, "")
			if w.Code != http.StatusOK || !json.Valid(w.Body.Bytes()) {
				t.Fatalf("startup endpoint %s returned %d: %s", prefix+path, w.Code, w.Body)
			}
			unauthorized := compatEndpointRequest(r, http.MethodGet, prefix+path, "", "")
			if unauthorized.Code != http.StatusUnauthorized || !json.Valid(unauthorized.Body.Bytes()) {
				t.Fatalf("startup endpoint %s must require authentication", prefix+path)
			}
		}
	}
	caps, ok := h.caps.get("compat-device")
	if !ok || caps.DeviceProfile == nil || len(caps.DeviceProfile.DirectPlayProfiles) != 1 {
		t.Fatal("client capabilities were not recorded")
	}
	w := compatEndpointRequest(r, http.MethodGet, "/Search/Hints?SearchTerm=Compat", token, "")
	var result SearchHintResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	types := map[string]bool{}
	for _, hint := range result.SearchHints {
		types[hint.Type] = true
		if hint.Id == "" || hint.ItemId != hint.Id {
			t.Fatalf("search hint has an invalid client ID: %+v", hint)
		}
	}
	if !types["Movie"] || !types["Series"] {
		t.Fatalf("search did not include both media kinds: %+v", result)
	}
}

func TestCompatEndpointsRejectRevokedAndDisabledUsers(t *testing.T) {
	for _, condition := range []string{"revoked", "disabled", "deleted"} {
		t.Run(condition, func(t *testing.T) {
			r, _, db, token := newCompatEndpointTestRouter(t)
			switch condition {
			case "revoked":
				if err := db.Model(&model.User{}).Where("id = ?", "compat-user").Update("token_version", 1).Error; err != nil {
					t.Fatal(err)
				}
			case "disabled":
				if err := db.Model(&model.User{}).Where("id = ?", "compat-user").Update("disabled", true).Error; err != nil {
					t.Fatal(err)
				}
			case "deleted":
				if err := db.Where("id = ?", "compat-user").Delete(&model.User{}).Error; err != nil {
					t.Fatal(err)
				}
			}
			for _, path := range []string{"/Items/Filters", "/emby/Search/Hints", "/socket", "/emby/embywebsocket"} {
				w := compatEndpointRequest(r, http.MethodGet, path, token, "")
				if w.Code != http.StatusUnauthorized {
					t.Fatalf("%s user reached %s: status=%d body=%s", condition, path, w.Code, w.Body)
				}
			}
		})
	}
}
