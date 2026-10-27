package service

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

func TestViewingPreferencesPersistWithoutChangingIdentity(t *testing.T) {
	db := setupTestDB(t)
	repos := repository.NewRepositories(db)
	first := &model.User{Username: "viewer", Password: "hash", Role: "user", TokenVersion: 7}
	second := &model.User{Username: "other", Password: "hash", Role: "user"}
	if err := repos.User.Create(first); err != nil {
		t.Fatal(err)
	}
	if err := repos.User.Create(second); err != nil {
		t.Fatal(err)
	}
	svc := NewUserService(repos.User, nil, &config.Config{}, zap.NewNop().Sugar())
	autoplay := false
	prefs := &model.ViewingPreferences{LargeText: true, SubtitleLanguage: "zh", AutoPlayNext: &autoplay}
	updated, renamed, err := svc.UpdateSelfProfile(first.ID, &UpdateSelfProfileRequest{ViewingPreferences: prefs})
	if err != nil {
		t.Fatal(err)
	}
	if renamed || updated.TokenVersion != 7 || updated.Role != "user" {
		t.Fatal("viewing preferences changed identity")
	}
	loaded, err := repos.User.FindByID(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.ViewingPreferences.LargeText || loaded.ViewingPreferences.SubtitleLanguage != "zh" || loaded.ViewingPreferences.AutoPlayNext == nil || *loaded.ViewingPreferences.AutoPlayNext {
		t.Fatalf("preferences not persisted: %+v", loaded.ViewingPreferences)
	}
	other, _ := repos.User.FindByID(second.ID)
	if other.ViewingPreferences.LargeText {
		t.Fatal("preferences leaked to another account")
	}
	_, _, err = svc.UpdateSelfProfile(first.ID, &UpdateSelfProfileRequest{ViewingPreferences: &model.ViewingPreferences{SubtitleLanguage: "unsupported"}})
	if !errors.Is(err, ErrInvalidViewingPreferences) {
		t.Fatalf("invalid language accepted: %v", err)
	}
	loaded, _ = repos.User.FindByID(first.ID)
	if loaded.ViewingPreferences.SubtitleLanguage != "zh" {
		t.Fatal("invalid update overwrote saved preferences")
	}
}

func TestFamilyInvitationWorksWithPublicRegistrationClosed(t *testing.T) {
	svc := setupAuthService(t)
	if _, err := svc.Register(&RegisterRequest{Username: "parent", Password: "password123"}, "", ""); err != nil {
		t.Fatal(err)
	}
	svc.cfg.Registration.Enabled = false
	expiry := time.Now().Add(time.Hour)
	invite := &model.InviteCode{Code: "family-once", MaxUses: 1, ExpiresAt: &expiry}
	if err := svc.inviteRepo.Create(invite); err != nil {
		t.Fatal(err)
	}
	joined, err := svc.Register(&RegisterRequest{Username: "family", Password: "password123", InviteCode: invite.Code}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if joined.User.Role != "user" {
		t.Fatal("invitation granted administrator role")
	}
	if _, err := svc.Register(&RegisterRequest{Username: "again", Password: "password123", InviteCode: invite.Code}, "", ""); !errors.Is(err, ErrInvalidInviteCode) {
		t.Fatalf("used invitation accepted: %v", err)
	}
	if _, err := svc.Register(&RegisterRequest{Username: "public", Password: "password123"}, "", ""); err == nil {
		t.Fatal("public registration was opened")
	}
}

func TestFamilyInvitationRejectsExpiredAndRevokedCodes(t *testing.T) {
	svc := setupAuthService(t)
	if _, err := svc.Register(&RegisterRequest{Username: "parent", Password: "password123"}, "", ""); err != nil {
		t.Fatal(err)
	}
	expired := time.Now().Add(-time.Hour)
	invite := &model.InviteCode{Code: "expired", MaxUses: 1, ExpiresAt: &expired}
	if err := svc.inviteRepo.Create(invite); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(&RegisterRequest{Username: "expired-user", Password: "password123", InviteCode: invite.Code}, "", ""); !errors.Is(err, ErrInvalidInviteCode) {
		t.Fatalf("expired invitation accepted: %v", err)
	}
	if err := svc.inviteRepo.Delete(invite.ID); err != nil {
		t.Fatal(err)
	}
	// Even when public registration is enabled, an explicitly submitted revoked
	// invitation must not silently fall back to public registration.
	if _, err := svc.Register(&RegisterRequest{Username: "revoked-user", Password: "password123", InviteCode: invite.Code}, "", ""); !errors.Is(err, ErrInvalidInviteCode) {
		t.Fatalf("revoked invitation accepted: %v", err)
	}
}

func TestInvitationSlotIsAtomicAndRollsBackFailedAccountCreation(t *testing.T) {
	db := setupTestDB(t)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	repos := repository.NewRepositories(db)
	if err := repos.User.Create(&model.User{Username: "existing", Password: "hash"}); err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(time.Hour)
	invite := &model.InviteCode{Code: "atomic", MaxUses: 1, ExpiresAt: &expiry}
	if err := repos.InviteCode.Create(invite); err != nil {
		t.Fatal(err)
	}
	if err := repos.User.CreateWithInvite(&model.User{Username: "existing", Password: "hash"}, invite.ID); err == nil {
		t.Fatal("duplicate account insert succeeded")
	}
	saved, _ := repos.InviteCode.FindByCode(invite.Code)
	if saved.UsedCount != 0 {
		t.Fatal("failed account creation consumed invitation")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, name := range []string{"one", "two"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			results <- repos.User.CreateWithInvite(&model.User{Username: name, Password: "hash", Role: "user"}, invite.ID)
		}(name)
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, repository.ErrInviteUnavailable) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("single-use invitation created %d accounts", successes)
	}
	saved, _ = repos.InviteCode.FindByCode(invite.Code)
	if saved.UsedCount != 1 {
		t.Fatalf("used count: %d", saved.UsedCount)
	}
}
