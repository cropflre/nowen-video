package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/model"
	playbacksession "github.com/nowen-video/nowen-video/internal/playback/session"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const childTestSecret = "child-access-test-key-not-a-secret"

func childFixture(t *testing.T) (*gorm.DB, *repository.Repositories, *service.PermissionService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateLite(db, false); err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	must := func(value interface{}) {
		t.Helper()
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	must(&model.User{ID: "child", Username: "child", Password: "hash", ChildMode: true, Role: "user"})
	must(&model.User{ID: "parent", Username: "parent", Password: "hash", Role: "admin"})
	must(&model.Library{ID: "kids", Name: "Kids", Path: "/kids"})
	must(&model.Library{ID: "private", Name: "DO_NOT_LEAK_LIBRARY", Path: "/private"})
	must(&model.UserPermission{UserID: "child", AllowedLibraries: "kids", MaxRatingLevel: "G"})
	for _, item := range []struct{ id, library, level string }{{"safe", "kids", "G"}, {"DO_NOT_LEAK_ADULT", "kids", "R"}, {"DO_NOT_LEAK_UNRATED", "kids", ""}, {"DO_NOT_LEAK_PRIVATE", "private", "G"}} {
		must(&model.Media{ID: item.id, Title: item.id, LibraryID: item.library, FilePath: "/" + item.id + ".mp4", MediaType: "movie", SubtitlePaths: `["/subs/` + item.id + `.vtt"]`})
		if item.level != "" {
			must(&model.ContentRating{MediaID: item.id, Level: item.level})
		}
		must(&model.Favorite{UserID: "child", MediaID: item.id})
		must(&model.WatchHistory{UserID: "child", MediaID: item.id, Position: 10, Duration: 100})
	}
	repos := repository.NewRepositories(db)
	return db, repos, service.NewPermissionService(repos.UserPermission, repos.ContentRating, repos.WatchHistory, zap.NewNop().Sugar(), db)
}
func childToken(t *testing.T, user *model.User) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &middleware.Claims{UserID: user.ID, Role: user.Role, Username: user.Username, TokenVersion: user.TokenVersion, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).SignedString([]byte(childTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	return token
}
func childRouter(t *testing.T, db *gorm.DB, repos *repository.Repositories) *gin.Engine {
	t.Helper()
	r := gin.New()
	auth := service.NewAuthService(repos.User, repos.InviteCode, repos.LoginLog, repos.AuditLog, &config.Config{}, zap.NewNop().Sugar())
	g := r.Group("/api", middleware.JWTAuthWithValidator(childTestSecret, auth.ValidateTokenVersion), ChildAccessGate(db))
	raw := func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=86400")
		c.JSON(200, gin.H{"data": "RAW_ADULT_HANDLER", "id": c.Param("id")})
	}
	for _, path := range []string{"/libraries", "/media", "/media/mixed", "/media/recent", "/media/recent/aggregated", "/media/recent/mixed", "/media/aggregated", "/media/continue", "/search", "/search/advanced", "/search/mixed", "/recommend", "/recommend/similar/:mediaId", "/series", "/series/:id", "/series/:id/seasons", "/series/:id/seasons/:season", "/series/:id/next", "/series/:id/poster", "/users/me/favorites", "/users/me/history", "/media/:id", "/media/:id/enhanced", "/media/:id/versions", "/media/:id/poster", "/media/:id/backdrop", "/media/:id/logo", "/media/:id/persons", "/media/:id/collection", "/media/:id/comments", "/media/:id/highlights", "/media/:id/highlights/status", "/media/:id/highlights/:highlightId/thumbnail", "/media/:id/highlights/:highlightId/preview", "/stream/:id/info", "/stream/:id/plan", "/stream/:id/direct", "/stream/:id/remux", "/subtitle/:id/tracks", "/subtitle/:id/extract/:index", "/subtitle/external", "/subtitle/download", "/collections", "/collections/:id", "/collections/:id/poster", "/persons/:id", "/future-unguarded-feature", "/playback/sessions/unreviewed-feature", "/ws"} {
		g.GET(path, raw)
	}
	userService := service.NewUserService(repos.User, nil, &config.Config{}, zap.NewNop().Sugar())
	userHandler := &UserHandler{userService: userService, logger: zap.NewNop().Sugar()}
	g.PUT("/users/me", userHandler.UpdateProfile)
	g.DELETE("/users/me/history", raw)
	g.POST("/playlists", raw)
	return r
}
func performChildRequest(t *testing.T, r http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	return response
}
func TestChildHTTPReadsUseScopedQueriesAndDoNotLeakCounts(t *testing.T) {
	db, repos, _ := childFixture(t)
	r := childRouter(t, db, repos)
	user, _ := repos.User.FindByID("child")
	token := childToken(t, user)
	paths := []string{"/api/libraries", "/api/media?size=1", "/api/media/mixed?size=1", "/api/media/recent", "/api/media/recent/mixed", "/api/media/recent/aggregated", "/api/media/aggregated", "/api/media/continue", "/api/search?q=DO_NOT_LEAK", "/api/search/advanced?q=DO_NOT_LEAK", "/api/search/mixed?q=DO_NOT_LEAK", "/api/recommend", "/api/recommend/similar/safe", "/api/users/me/favorites", "/api/users/me/history", "/api/series", "/api/collections"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			response := performChildRequest(t, r, "GET", path, token, "")
			if response.Code != 200 {
				t.Fatalf("status %d %s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "DO_NOT_LEAK") || strings.Contains(response.Body.String(), "RAW_ADULT_HANDLER") {
				t.Fatal(response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("child response cacheable")
			}
		})
	}
	response := performChildRequest(t, r, "GET", "/api/media/mixed?size=1", token, "")
	var result struct {
		Total       int `json:"total"`
		MovieCount  int `json:"movie_count"`
		SeriesCount int `json:"series_count"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || result.MovieCount != 1 || result.SeriesCount != 0 {
		t.Fatalf("counts %+v", result)
	}
	parent, _ := repos.User.FindByID("parent")
	response = performChildRequest(t, r, "GET", "/api/media/mixed", childToken(t, parent), "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), "RAW_ADULT_HANDLER") {
		t.Fatal("adult route was intercepted")
	}
}
func TestChildHTTPDirectIdentifiersImagesAndSubtitlesCannotBypassPolicy(t *testing.T) {
	db, repos, _ := childFixture(t)
	r := childRouter(t, db, repos)
	user, _ := repos.User.FindByID("child")
	token := childToken(t, user)
	templates := []string{"/api/media/ID", "/api/media/ID/enhanced", "/api/media/ID/versions", "/api/media/ID/poster", "/api/media/ID/backdrop", "/api/media/ID/logo", "/api/media/ID/collection", "/api/media/ID/persons", "/api/media/ID/comments", "/api/media/ID/highlights", "/api/media/ID/highlights/status", "/api/media/ID/highlights/highlight/thumbnail", "/api/media/ID/highlights/highlight/preview", "/api/stream/ID/info", "/api/stream/ID/plan", "/api/stream/ID/direct", "/api/stream/ID/remux", "/api/subtitle/ID/tracks", "/api/subtitle/ID/extract/0", "/api/recommend/similar/ID"}
	for _, id := range []string{"DO_NOT_LEAK_UNRATED", "DO_NOT_LEAK_ADULT", "DO_NOT_LEAK_PRIVATE"} {
		for _, template := range templates {
			path := strings.ReplaceAll(template, "ID", id)
			response := performChildRequest(t, r, "GET", path, token, "")
			if response.Code != 404 {
				t.Errorf("%s -> %d %s", path, response.Code, response.Body.String())
			}
		}
	}
	for _, path := range []string{"/api/subtitle/external?path=/subs/DO_NOT_LEAK_ADULT.vtt", "/api/subtitle/download?path=/private/file.vtt", "/api/future-unguarded-feature", "/api/playback/sessions/unreviewed-feature", "/api/ws"} {
		response := performChildRequest(t, r, "GET", path, token, "")
		if response.Code < 400 {
			t.Errorf("allowed %s", path)
		}
	}
	for _, path := range []string{"/api/media/safe/poster", "/api/stream/safe/direct", "/api/subtitle/external?path=/subs/safe.vtt"} {
		response := performChildRequest(t, r, "GET", path, token, "")
		if response.Code != 200 {
			t.Fatalf("%s -> %s", path, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("downstream handler overrode private cache policy")
		}
	}
}
func TestChildCannotChangeOwnModeOrPromoteSelfAndPolicyChangesRevokeJWT(t *testing.T) {
	db, repos, permission := childFixture(t)
	r := childRouter(t, db, repos)
	child, _ := repos.User.FindByID("child")
	token := childToken(t, child)
	response := performChildRequest(t, r, "PUT", "/api/users/me", token, `{"child_mode":false,"role":"admin","nickname":"hello"}`)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	child, _ = repos.User.FindByID("child")
	if !child.ChildMode || child.Role != "user" {
		t.Fatal("child escaped through self profile")
	}
	for _, request := range []struct{ method, path string }{{"DELETE", "/api/users/me/history"}, {"POST", "/api/playlists"}} {
		if performChildRequest(t, r, request.method, request.path, token, `{}`).Code != 403 {
			t.Fatal("child mutation escaped allowlist")
		}
	}
	if err := permission.UpdateUserPermission("child", "", "G", 0, true); err != nil {
		t.Fatal(err)
	}
	if response := performChildRequest(t, r, "GET", "/api/media", token, ""); response.Code != 401 {
		t.Fatalf("old token still valid: %d", response.Code)
	}
	child, _ = repos.User.FindByID("child")
	response = performChildRequest(t, r, "GET", "/api/media", childToken(t, child), "")
	if response.Code != 200 || strings.Contains(response.Body.String(), `"safe"`) {
		t.Fatal("empty library policy exposed content")
	}
}

func TestAdultSocketStopsDeliveringAfterAccountBecomesChild(t *testing.T) {
	db, repos, permissions := childFixture(t)
	adult := &model.User{ID: "viewer", Username: "viewer", Password: "hash", Role: "user"}
	if err := repos.User.Create(adult); err != nil {
		t.Fatal(err)
	}
	hub := service.NewWSHub(zap.NewNop().Sugar())
	go hub.Run()
	users := service.NewUserService(repos.User, nil, nil, zap.NewNop().Sugar())
	ws := &WSHandler{hub: hub, users: users, logger: zap.NewNop().Sugar()}
	auth := service.NewAuthService(repos.User, nil, nil, nil, &config.Config{}, zap.NewNop().Sugar())
	r := gin.New()
	r.GET("/api/ws", middleware.JWTAuthWithValidator(childTestSecret, auth.ValidateTokenVersion), ChildAccessGate(db), ws.HandleWebSocket)
	server := httptest.NewServer(r)
	defer server.Close()
	headers := http.Header{"Authorization": []string{"Bearer " + childToken(t, adult)}}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/ws", headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	deadline := time.Now().Add(time.Second)
	for hub.ClientCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if hub.ClientCount() != 1 {
		t.Fatal("adult socket did not register")
	}
	hub.BroadcastEvent("fixture", "allowed")
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, message, err := conn.ReadMessage(); err != nil || !strings.Contains(string(message), "allowed") {
		t.Fatalf("adult delivery failed: %s %v", message, err)
	}
	if err := permissions.UpdateUserPermission(adult.ID, "kids", "G", 0, true); err != nil {
		t.Fatal(err)
	}
	hub.BroadcastEvent("fixture", "DO_NOT_LEAK_AFTER_CONVERSION")
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, message, err := conn.ReadMessage(); err == nil {
		t.Fatalf("revoked socket received data: %s", message)
	}
}
func TestFamilyRatingSeriesApprovalAndWithdrawal(t *testing.T) {
	db, repos, _ := childFixture(t)
	if err := db.Create(&model.Series{ID: "show", Title: "show", LibraryID: "kids", FolderPath: "/show"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ep1", "ep2"} {
		if err := db.Create(&model.Media{ID: id, Title: id, LibraryID: "kids", FilePath: "/" + id, MediaType: "episode", SeriesID: "show"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	handler := &AdminHandler{db: db, userService: service.NewUserService(repos.User, nil, &config.Config{}, zap.NewNop().Sugar())}
	r := gin.New()
	r.PUT("/admin/series/:seriesId/rating", handler.SetFamilyRating)
	response := performChildRequest(t, r, "PUT", "/admin/series/show/rating", "", `{"level":"G"}`)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	catalog := service.NewChildCatalog(db)
	p, _ := catalog.Policy("child")
	if catalog.CheckSeries(p, "show") != nil {
		t.Fatal("bulk approval not applied")
	}
	response = performChildRequest(t, r, "PUT", "/admin/series/show/rating", "", `{"level":""}`)
	if response.Code != 200 || catalog.CheckSeries(p, "show") == nil {
		t.Fatal("withdrawal failed")
	}
}

type childSessionStub struct {
	id    string
	opens int
}

func (s *childSessionStub) Status(string, string) (service.PlaybackSessionResult, error) {
	return service.PlaybackSessionResult{Session: playbacksession.SessionSnapshot{MediaID: s.id}}, nil
}
func (s *childSessionStub) Create(context.Context, string, service.PlaybackSessionCreateRequest) (service.PlaybackSessionResult, error) {
	panic("unexpected create")
}
func (s *childSessionStub) Heartbeat(string, string, service.PlaybackSessionHeartbeatRequest) (service.PlaybackSessionResult, error) {
	panic("unexpected heartbeat")
}
func (s *childSessionStub) Restart(context.Context, string, string, service.PlaybackSessionRestartRequest) (service.PlaybackSessionResult, error) {
	panic("unexpected restart")
}
func (s *childSessionStub) Close(context.Context, string, string, string) error { return nil }
func (s *childSessionStub) OpenPlaylist(string, string, uint64) (*service.PlaybackSessionFile, error) {
	s.opens++
	panic("unexpected playlist")
}
func (s *childSessionStub) OpenSegment(string, string, uint64, string) (*service.PlaybackSessionFile, error) {
	s.opens++
	panic("unexpected segment")
}
func (s *childSessionStub) ReconcilePlaybackThrottle(string) error { return nil }
func TestChildExistingPlaybackSessionRechecksRevokedMediaBeforeServing(t *testing.T) {
	db, repos, permission := childFixture(t)
	runtime := &childSessionStub{id: "safe"}
	handler := NewPlaybackSessionHandler(runtime, permission, repos.Media, zap.NewNop().Sugar())
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "child"); c.Set("role", "user"); c.Next() })
	r.GET("/status/:sessionID", handler.Status)
	r.GET("/playlist/:sessionID/:generationID", handler.Playlist)
	r.GET("/segment/:sessionID/:generationID/:file", handler.Segment)
	r.POST("/heartbeat/:sessionID", handler.Heartbeat)
	r.POST("/restart/:sessionID", handler.Restart)
	if response := performChildRequest(t, r, "GET", "/status/session", "", ""); response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	if err := db.Where("media_id = ?", "safe").Delete(&model.ContentRating{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, request := range []struct{ method, path string }{{"GET", "/status/session"}, {"GET", "/playlist/session/1"}, {"GET", "/segment/session/1/segment.ts"}, {"POST", "/heartbeat/session"}, {"POST", "/restart/session"}} {
		response := performChildRequest(t, r, request.method, request.path, "", `{}`)
		if response.Code != 403 {
			t.Fatalf("%s -> %d %s", request.path, response.Code, response.Body.String())
		}
	}
	if runtime.opens != 0 {
		t.Fatal("opened a revoked session file")
	}
}
