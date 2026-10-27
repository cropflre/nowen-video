package service

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/model"
)

func sessionClaims(t *testing.T, service *AuthService, token string) *middleware.Claims {
	t.Helper()
	claims := &middleware.Claims{}
	if _, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return []byte(service.cfg.Secrets.JWTSecret), nil }); err != nil {
		t.Fatal(err)
	}
	return claims
}

func TestDeviceSessionsRevokeOnePreserveOthersAndCannotRefreshRevoked(t *testing.T) {
	s := setupAuthService(t)
	sqlDB, _ := s.userRepo.DB().DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	first, err := s.Register(&RegisterRequest{Username: "parent", Password: "password123"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Login(&LoginRequest{Username: "parent", Password: "password123", DeviceName: "Living room"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	one, two := sessionClaims(t, s, first.Token), sessionClaims(t, s, second.Token)
	if one.SessionID == "" || two.SessionID == "" || one.SessionID == two.SessionID {
		t.Fatal("login did not create independent sessions")
	}
	refreshed, err := s.RefreshToken(first.User.ID, two.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if sessionClaims(t, s, refreshed.Token).SessionID != two.SessionID {
		t.Fatal("refresh escaped the original session")
	}
	if err := s.RevokeDevice(first.User.ID, one.SessionID); err != nil {
		t.Fatal(err)
	}
	if s.ValidateDeviceSession(first.User.ID, one.SessionID) == nil {
		t.Fatal("revoked session validated")
	}
	if _, err := s.RefreshToken(first.User.ID, one.SessionID); err == nil {
		t.Fatal("refresh revived revoked session")
	}
	if err := s.ValidateDeviceSession(first.User.ID, two.SessionID); err != nil {
		t.Fatal("other device revoked", err)
	}
	other, err := s.Register(&RegisterRequest{Username: "other", Password: "password123"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeDevice(other.User.ID, two.SessionID); err != nil {
		t.Fatal(err)
	}
	if s.ValidateDeviceSession(first.User.ID, two.SessionID) != nil {
		t.Fatal("another account revoked session")
	}
	if s.ValidateDeviceSession(other.User.ID, two.SessionID) == nil {
		t.Fatal("another account used session")
	}
	if err := s.RevokeAllDevices(first.User.ID); err != nil {
		t.Fatal(err)
	}
	if s.ValidateDeviceSession(first.User.ID, two.SessionID) == nil {
		t.Fatal("all-device revocation failed")
	}
	version, _, err := s.ValidateTokenVersion(first.User.ID)
	if err != nil || version <= one.TokenVersion {
		t.Fatal("legacy credentials not invalidated", err)
	}
}

func TestDevicePairingRequiresBothSecretsApprovalAndOneTimeClaim(t *testing.T) {
	s := setupAuthService(t)
	sqlDB, _ := s.userRepo.DB().DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	parent, err := s.Register(&RegisterRequest{Username: "parent", Password: "password123"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	request, err := s.CreateDevicePairing("Family screen", "")
	if err != nil {
		t.Fatal(err)
	}
	var stored model.DevicePairing
	if err := s.userRepo.DB().First(&stored, "id = ?", request.ID).Error; err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(stored)
	if strings.Contains(string(encoded), request.Secret) || strings.Contains(string(encoded), normalizePairCode(request.Code)) {
		t.Fatal("raw pairing credentials stored")
	}
	if _, err := s.PollDevicePairing(request.ID, request.Code); err == nil {
		t.Fatal("short code exchanged credentials")
	}
	status, err := s.PollDevicePairing(request.ID, request.Secret)
	if err != nil || status.Status != "pending" || status.Token != nil {
		t.Fatal("unapproved request authorized", err)
	}
	if err := s.ApproveDevicePairing(parent.User.ID, strings.ToLower(request.Code), parent.User.TokenVersion); err != nil {
		t.Fatal(err)
	}
	if err := s.ApproveDevicePairing(parent.User.ID, request.Code, parent.User.TokenVersion); !errors.Is(err, ErrPairBusy) {
		t.Fatal("approval reused", err)
	}
	var wg sync.WaitGroup
	results := make(chan *DevicePairStatus, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if response, err := s.PollDevicePairing(request.ID, request.Secret); err == nil {
				results <- response
			}
		}()
	}
	wg.Wait()
	close(results)
	if len(results) != 1 {
		t.Fatalf("concurrent claims succeeded %d times", len(results))
	}
	status = <-results
	if status.Token == nil || status.Token.User.ID != parent.User.ID || sessionClaims(t, s, status.Token.Token).SessionID == "" {
		t.Fatal("claim missing scoped credentials")
	}
	if _, err := s.PollDevicePairing(request.ID, request.Secret); !errors.Is(err, ErrPairFinished) {
		t.Fatal("claim replayed", err)
	}
}

func TestDevicePairingRejectsCancelledExpiredAndChangedAccount(t *testing.T) {
	s := setupAuthService(t)
	parent, err := s.Register(&RegisterRequest{Username: "parent", Password: "password123"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"cancelled", "expired", "revoked", "must-change", "disabled"} {
		t.Run(scenario, func(t *testing.T) {
			s.userRepo.DB().Model(&model.User{}).Where("id = ?", parent.User.ID).Updates(map[string]any{"disabled": false, "must_change_pwd": false})
			user, _ := s.userRepo.FindByID(parent.User.ID)
			request, err := s.CreateDevicePairing(scenario, "")
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApproveDevicePairing(user.ID, request.Code, user.TokenVersion); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "cancelled":
				err = s.CancelDevicePairing(request.ID, request.Secret)
			case "expired":
				err = s.userRepo.DB().Model(&model.DevicePairing{}).Where("id = ?", request.ID).Update("expires_at", time.Now().Add(-time.Second)).Error
			case "revoked":
				err = s.RevokeAllDevices(user.ID)
			case "must-change":
				err = s.userRepo.DB().Model(&model.User{}).Where("id = ?", user.ID).Update("must_change_pwd", true).Error
			case "disabled":
				err = s.userRepo.DB().Model(&model.User{}).Where("id = ?", user.ID).Update("disabled", true).Error
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.PollDevicePairing(request.ID, request.Secret); err == nil {
				t.Fatal("invalidated pairing issued credentials")
			}
		})
	}
}

func TestPairedChildKeepsItsOwnPolicyAndDeviceAbsoluteExpiry(t *testing.T) {
	s := setupAuthService(t)
	parent, err := s.Register(&RegisterRequest{Username: "parent", Password: "password123"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	child := &model.User{Username: "child", Password: "hash", Role: "user", ChildMode: true}
	if err := s.userRepo.Create(child); err != nil {
		t.Fatal(err)
	}
	if err := s.userRepo.DB().Create(&model.UserPermission{UserID: child.ID, DailyTimeLimit: 1, MaxRatingLevel: "G", AllowedLibraries: "kids"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.userRepo.DB().Create(&model.FamilyViewingDay{UserID: child.ID, Day: time.Now().Format("2006-01-02"), ChargedSeconds: 60}).Error; err != nil {
		t.Fatal(err)
	}
	request, err := s.CreateDevicePairing("Child screen", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApproveDevicePairing(child.ID, request.Code, child.TokenVersion); err != nil {
		t.Fatal(err)
	}
	response, err := s.PollDevicePairing(request.ID, request.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if !response.Token.User.ChildMode || response.Token.User.Role != "user" || response.Token.User.ID == parent.User.ID {
		t.Fatal("pairing elevated child")
	}
	usage, err := NewFamilyPlaybackService(s.userRepo.DB()).Status(response.Token.User.ID)
	if err != nil || usage.UsedSeconds != 60 || usage.RemainingSeconds != 0 {
		t.Fatal("pairing reset child's daily budget", err)
	}
	claims := sessionClaims(t, s, response.Token.Token)
	if err := s.userRepo.DB().Model(&model.DeviceSession{}).Where("id = ?", claims.SessionID).Update("expires_at", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if s.ValidateDeviceSession(child.ID, claims.SessionID) == nil {
		t.Fatal("absolute expiry ignored")
	}
	if _, err := s.RefreshToken(child.ID, claims.SessionID); err == nil {
		t.Fatal("expired device refreshed")
	}
}

func TestRevokingApprovingDeviceInvalidatesUnclaimedPairing(t *testing.T) {
	s := setupAuthService(t)
	parent, err := s.Register(&RegisterRequest{Username: "parent", Password: "password123"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	claims := sessionClaims(t, s, parent.Token)
	request, err := s.CreateDevicePairing("New screen", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApproveDevicePairing(parent.User.ID, request.Code, parent.User.TokenVersion, claims.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeDevice(parent.User.ID, claims.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PollDevicePairing(request.ID, request.Secret); err == nil {
		t.Fatal("revoked approving device minted a new login")
	}
}
