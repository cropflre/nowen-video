package emby

import (
	"encoding/json"
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
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestEmbyCannotBypassChildPolicyOrFetchAnonymousArtwork(t *testing.T) {
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
	defer sqlDB.Close()
	repos := repository.NewRepositories(db)
	password, _ := bcrypt.GenerateFromPassword([]byte("child-password"), bcrypt.MinCost)
	kid := &model.User{ID: "child-id", Username: "child-account", Password: string(password), ChildMode: true, Role: "user"}
	parent := &model.User{ID: "parent-id", Username: "parent-account", Password: string(password), Role: "admin"}
	for _, user := range []*model.User{kid, parent} {
		if err := repos.User.Create(user); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{}
	cfg.Secrets.JWTSecret = "emby-child-test-secret"
	cfg.Emby.PublicUserListEnabled = true
	auth := service.NewAuthService(repos.User, repos.InviteCode, repos.LoginLog, repos.AuditLog, cfg, zap.NewNop().Sugar())
	h := NewHandler(cfg, zap.NewNop().Sugar(), auth, nil, repos)
	r := gin.New()
	RegisterRoutes(r, h, cfg.Secrets.JWTSecret)
	tokenFor := func(user *model.User) string {
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &middleware.Claims{UserID: user.ID, Username: user.Username, Role: user.Role, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).SignedString([]byte(cfg.Secrets.JWTSecret))
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	for _, prefix := range []string{"", "/emby"} {
		request := httptest.NewRequest("GET", prefix+"/Items/123/Images/Primary", nil)
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		if response.Code != 401 {
			t.Fatalf("anonymous image %s -> %d", prefix, response.Code)
		}
		for _, path := range []string{"/Items", "/Users/Me", "/Items/123/Images/Primary"} {
			request = httptest.NewRequest("GET", prefix+path, nil)
			request.Header.Set("X-Emby-Token", tokenFor(kid))
			response = httptest.NewRecorder()
			r.ServeHTTP(response, request)
			if response.Code != 401 {
				t.Fatalf("child compatibility route %s -> %d", path, response.Code)
			}
		}
		request = httptest.NewRequest("GET", prefix+"/Users/Me", nil)
		request.Header.Set("X-Emby-Token", tokenFor(parent))
		response = httptest.NewRecorder()
		r.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatalf("parent compatibility denied: %d %s", response.Code, response.Body.String())
		}
	}
	parentLogin, err := auth.Login(&service.LoginRequest{Username: parent.Username, Password: "child-password"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	claims := &middleware.Claims{}
	if _, err := jwt.ParseWithClaims(parentLogin.Token, claims, func(*jwt.Token) (any, error) { return []byte(cfg.Secrets.JWTSecret), nil }); err != nil {
		t.Fatal(err)
	}
	requestWithSession := httptest.NewRequest("GET", "/Users/Me", nil)
	requestWithSession.Header.Set("X-Emby-Token", parentLogin.Token)
	withSession := httptest.NewRecorder()
	r.ServeHTTP(withSession, requestWithSession)
	if withSession.Code != 200 {
		t.Fatal("active session rejected by compatibility auth", withSession.Code)
	}
	if err := auth.RevokeDevice(parent.ID, claims.SessionID); err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"", "/emby"} {
		for _, path := range []string{"/Users/Me", "/Items/123/Images/Primary", "/Videos/123/stream"} {
			request := httptest.NewRequest("GET", prefix+path+"?api_key="+parentLogin.Token, nil)
			response := httptest.NewRecorder()
			r.ServeHTTP(response, request)
			if response.Code != 401 {
				t.Fatal("revoked device bypassed compatibility auth", path, response.Code)
			}
		}
	}
	body, _ := json.Marshal(map[string]string{"Username": kid.Username, "Pw": "child-password"})
	request := httptest.NewRequest("POST", "/Users/AuthenticateByName", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != 403 || strings.Contains(response.Body.String(), "AccessToken") {
		t.Fatal("child login exposed a compatibility token")
	}
	response = httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest("GET", "/Users/Public", nil))
	if strings.Contains(response.Body.String(), kid.Username) {
		t.Fatal("unsupported child account was advertised in compatibility user picker")
	}
}
