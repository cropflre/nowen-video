package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

func TestDevicePairingHTTPScopesApprovalAndRevokesRefreshAndMedia(t *testing.T) {
	db, repos, _ := childFixture(t)
	cfg := &config.Config{}
	cfg.Secrets.JWTSecret = childTestSecret
	auth := service.NewAuthService(repos.User, repos.InviteCode, repos.LoginLog, repos.AuditLog, cfg, zap.NewNop().Sugar())
	h := &AuthHandler{authService: auth, logger: zap.NewNop().Sugar()}
	r := gin.New()
	h.RegisterDevicePairing(r)
	api := r.Group("/api", middleware.JWTAuthWithValidator(childTestSecret, auth.ValidateTokenVersion, auth.ValidateDeviceSession), ChildAccessGate(db))
	h.RegisterDeviceManagement(api)
	api.GET("/stream/:id/direct", func(c *gin.Context) { c.Status(204) })
	r.POST("/api/auth/refresh", middleware.JWTAuthAllowExpired(childTestSecret, auth.ValidateTokenVersion, auth.ValidateDeviceSession), h.RefreshToken)
	parent, _ := repos.User.FindByID("parent")
	child, _ := repos.User.FindByID("child")
	request, err := auth.CreateDevicePairing("Child television", "")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"code": request.Code, "user_id": "parent", "role": "admin"})
	if got := performChildRequest(t, r, "POST", "/api/users/me/device-pairing/approve", "", string(body)); got.Code != 401 {
		t.Fatal("anonymous approval admitted", got.Code)
	}
	if got := performChildRequest(t, r, "POST", "/api/users/me/device-pairing/inspect", childToken(t, child), string(body)); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := performChildRequest(t, r, "POST", "/api/users/me/device-pairing/approve", childToken(t, child), string(body)); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	poll, _ := json.Marshal(map[string]string{"id": request.ID, "secret": request.Secret})
	response := performChildRequest(t, r, "POST", "/api/auth/device-pairing/status", "", string(poll))
	var authorized struct {
		Data service.DevicePairStatus `json:"data"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &authorized) != nil || authorized.Data.Token == nil {
		t.Fatal("pair claim failed", response.Code)
	}
	if !authorized.Data.Token.User.ChildMode || authorized.Data.Token.User.Role != "user" || authorized.Data.Token.User.ID != child.ID {
		t.Fatal("request body changed approval identity")
	}
	claims := &middleware.Claims{}
	if _, err := jwt.ParseWithClaims(authorized.Data.Token.Token, claims, func(*jwt.Token) (any, error) { return []byte(childTestSecret), nil }); err != nil {
		t.Fatal(err)
	}
	if got := performChildRequest(t, r, "GET", "/api/stream/safe/direct", authorized.Data.Token.Token, ""); got.Code != 204 {
		t.Fatal("paired child refused approved content", got.Code)
	}
	if got := performChildRequest(t, r, "GET", "/api/stream/DO_NOT_LEAK_ADULT/direct", authorized.Data.Token.Token, ""); got.Code != 404 {
		t.Fatal("paired child escaped content scope", got.Code)
	}
	if got := performChildRequest(t, r, "DELETE", "/api/users/me/devices/"+claims.SessionID, childToken(t, parent), ""); got.Code != 200 {
		t.Fatal(got.Code)
	}
	if err := auth.ValidateDeviceSession(child.ID, claims.SessionID); err != nil {
		t.Fatal("parent revoked another account's device through self API", err)
	}
	if got := performChildRequest(t, r, "DELETE", "/api/users/me/devices/"+claims.SessionID, childToken(t, child), ""); got.Code != 200 {
		t.Fatal(got.Code)
	}
	for _, path := range []string{"/api/stream/safe/direct", "/api/auth/refresh"} {
		method := "GET"
		if strings.Contains(path, "refresh") {
			method = "POST"
		}
		if got := performChildRequest(t, r, method, path, authorized.Data.Token.Token, `{}`); got.Code != 401 {
			t.Fatal("revoked token accepted", path, got.Code)
		}
	}
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("authorization response cacheable")
	}
}

func TestDeviceRefreshHTTPKeepsSessionAndExpiresAbsolutely(t *testing.T) {
	_, repos, _ := childFixture(t)
	cfg := &config.Config{}
	cfg.Secrets.JWTSecret = childTestSecret
	auth := service.NewAuthService(repos.User, repos.InviteCode, repos.LoginLog, repos.AuditLog, cfg, zap.NewNop().Sugar())
	h := &AuthHandler{authService: auth, logger: zap.NewNop().Sugar()}
	r := gin.New()
	r.POST("/refresh", middleware.JWTAuthAllowExpired(childTestSecret, auth.ValidateTokenVersion, auth.ValidateDeviceSession), h.RefreshToken)
	user, _ := repos.User.FindByID("parent")
	pair, err := auth.CreateDevicePairing("Screen", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.ApproveDevicePairing(user.ID, pair.Code, user.TokenVersion); err != nil {
		t.Fatal(err)
	}
	authorized, err := auth.PollDevicePairing(pair.ID, pair.Secret)
	if err != nil {
		t.Fatal(err)
	}
	claims := &middleware.Claims{}
	jwt.ParseWithClaims(authorized.Token.Token, claims, func(*jwt.Token) (any, error) { return []byte(childTestSecret), nil })
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(childTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	response := performChildRequest(t, r, "POST", "/refresh", expired, `{}`)
	var refreshed service.TokenResponse
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &refreshed) != nil {
		t.Fatal("expired access token not refreshed", response.Code)
	}
	newClaims := &middleware.Claims{}
	jwt.ParseWithClaims(refreshed.Token, newClaims, func(*jwt.Token) (any, error) { return []byte(childTestSecret), nil })
	if newClaims.SessionID != claims.SessionID {
		t.Fatal("refresh created a new device identity")
	}
	if err := auth.RevokeDevice(user.ID, claims.SessionID); err != nil {
		t.Fatal(err)
	}
	response = performChildRequest(t, r, "POST", "/refresh", expired, `{}`)
	if response.Code != 401 {
		t.Fatal("expired revoked token refreshed", response.Code)
	}
}

func TestDeviceRevocationStopsExistingWebSocketDelivery(t *testing.T) {
	_, repos, _ := childFixture(t)
	cfg := &config.Config{}
	cfg.Secrets.JWTSecret = childTestSecret
	auth := service.NewAuthService(repos.User, repos.InviteCode, repos.LoginLog, repos.AuditLog, cfg, zap.NewNop().Sugar())
	parent, _ := repos.User.FindByID("parent")
	request, err := auth.CreateDevicePairing("Screen", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.ApproveDevicePairing(parent.ID, request.Code, parent.TokenVersion); err != nil {
		t.Fatal(err)
	}
	issued, err := auth.PollDevicePairing(request.ID, request.Secret)
	if err != nil {
		t.Fatal(err)
	}
	claims := &middleware.Claims{}
	if _, err := jwt.ParseWithClaims(issued.Token.Token, claims, func(*jwt.Token) (any, error) { return []byte(childTestSecret), nil }); err != nil {
		t.Fatal(err)
	}
	hub := service.NewWSHub(zap.NewNop().Sugar())
	go hub.Run()
	h := &WSHandler{hub: hub, users: service.NewUserService(repos.User, nil, cfg, zap.NewNop().Sugar()), auth: auth, logger: zap.NewNop().Sugar()}
	r := gin.New()
	r.GET("/api/ws", middleware.JWTAuthWithValidator(childTestSecret, auth.ValidateTokenVersion, auth.ValidateDeviceSession), h.HandleWebSocket)
	server := httptest.NewServer(r)
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/ws", http.Header{"Authorization": []string{"Bearer " + issued.Token.Token}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	deadline := time.Now().Add(time.Second)
	for hub.ClientCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	hub.BroadcastEvent("fixture", "allowed")
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, body, err := conn.ReadMessage(); err != nil || !strings.Contains(string(body), "allowed") {
		t.Fatal("active socket could not receive event", err)
	}
	if err := auth.RevokeDevice(parent.ID, claims.SessionID); err != nil {
		t.Fatal(err)
	}
	hub.BroadcastEvent("fixture", "REVOKED_DEVICE_MUST_NOT_RECEIVE")
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("revoked socket received event")
	}
}
