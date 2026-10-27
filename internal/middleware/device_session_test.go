package middleware

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestDeviceClaimsFailClosedAndRefreshNeverIgnoresBadSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "device-middleware-test-secret"
	sign := func(method jwt.SigningMethod, key string, expiry time.Time, sid string, notBefore *jwt.NumericDate) string {
		token, err := jwt.NewWithClaims(method, &Claims{UserID: "user", SessionID: sid, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expiry), NotBefore: notBefore}}).SignedString([]byte(key))
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	account := func(string) (int, bool, error) { return 0, false, nil }
	revoked := false
	unavailable := false
	device := func(userID, sid string) error {
		if unavailable {
			return errors.New("storage temporarily unavailable")
		}
		if userID != "user" || sid != "device" || revoked {
			return ErrDeviceSessionRejected
		}
		return nil
	}
	r := gin.New()
	r.GET("/api", JWTAuthWithValidator(secret, account, device), func(c *gin.Context) { c.Status(204) })
	r.GET("/refresh", JWTAuthAllowExpired(secret, account, device), func(c *gin.Context) { c.Status(204) })
	r.GET("/missing-provider", JWTAuthWithValidator(secret, account), func(c *gin.Context) { c.Status(204) })
	request := func(path, token string) int {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		return out.Code
	}
	now := time.Now()
	active := sign(jwt.SigningMethodHS256, secret, now.Add(time.Hour), "device", nil)
	if request("/api", active) != 204 {
		t.Fatal("active session refused")
	}
	if request("/missing-provider", active) != 401 {
		t.Fatal("session credential fell back to account-only auth")
	}
	expired := sign(jwt.SigningMethodHS256, secret, now.Add(-time.Minute), "device", nil)
	if request("/refresh", expired) != 204 {
		t.Fatal("valid expired access token not refreshed")
	}
	revoked = true
	if request("/api", active) != 401 || request("/refresh", expired) != 401 {
		t.Fatal("revoked device accepted")
	}
	revoked = false
	unavailable = true
	if request("/api", active) != 503 || request("/refresh", expired) != 503 {
		t.Fatal("storage error treated as an invalid credential")
	}
	unavailable = false
	forgedExpired := sign(jwt.SigningMethodHS256, "wrong-signing-key", now.Add(-time.Minute), "", nil)
	if request("/refresh", forgedExpired) != 401 {
		t.Fatal("expiry error hid invalid signature")
	}
	future := sign(jwt.SigningMethodHS256, secret, now.Add(-time.Minute), "", jwt.NewNumericDate(now.Add(time.Hour)))
	if request("/refresh", future) != 401 {
		t.Fatal("future not-before ignored")
	}
	wrongAlgorithm := sign(jwt.SigningMethodHS384, secret, now.Add(time.Hour), "", nil)
	if request("/api", wrongAlgorithm) != 401 || request("/refresh", wrongAlgorithm) != 401 {
		t.Fatal("unexpected signing method accepted")
	}
	legacy := sign(jwt.SigningMethodHS256, secret, now.Add(time.Hour), "", nil)
	if request("/api", legacy) != 204 {
		t.Fatal("legacy credentials rejected before migration")
	}
}
