package service

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

func familyClock(t *testing.T) (*FamilyPlaybackService, *time.Time) {
	t.Helper()
	_, db := setupChildCatalog(t)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.Model(&model.UserPermission{}).Where("user_id = ?", "child").Update("daily_time_limit", 1).Error; err != nil {
		t.Fatal(err)
	}
	s := NewFamilyPlaybackService(db)
	now := time.Date(2026, 10, 27, 20, 0, 0, 0, time.FixedZone("test-family", 8*3600))
	s.now = func() time.Time { return now }
	return s, &now
}

func TestFamilyFinalHeartbeatReportsExhaustedBudget(t *testing.T) {
	s, now := familyClock(t)
	grant, err := s.Start("child", "safe-a")
	if err != nil {
		t.Fatal(err)
	}
	for elapsed := 5; elapsed <= 55; elapsed += 5 {
		*now = now.Add(5 * time.Second)
		if _, err = s.Renew("child", grant.LeaseID); err != nil {
			t.Fatal(err)
		}
	}
	*now = now.Add(5*time.Second + time.Millisecond)
	if _, err = s.Renew("child", grant.LeaseID); !errors.Is(err, ErrTimeLimitExceeded) {
		t.Fatalf("final heartbeat reported %v instead of budget exhaustion", err)
	}
	if err = s.CheckAccess("child", "safe-a"); !errors.Is(err, ErrFamilyLeaseExpired) {
		t.Fatalf("expired grant can access content: %v", err)
	}
}

func TestFamilyBudgetCountsServerTimeAcrossSeeksPausesAndRestarts(t *testing.T) {
	s, now := familyClock(t)
	grant, err := s.Start("child", "safe-a")
	if err != nil || grant.LeaseID == "" || grant.ExpiresInMS != 15000 {
		t.Fatalf("grant %+v %v", grant, err)
	}
	if err := s.CheckAccess("child", "safe-b"); err == nil {
		t.Fatal("lease used for another media")
	}
	*now = now.Add(7 * time.Second)
	status, err := s.Renew("child", grant.LeaseID)
	if err != nil || status.UsedSeconds != 7 {
		t.Fatalf("renew %+v %v", status, err)
	}
	*now = now.Add(3 * time.Second)
	status, err = s.Stop("child", grant.LeaseID)
	if err != nil || status.UsedSeconds != 10 || status.RemainingSeconds != 50 {
		t.Fatalf("pause %+v %v", status, err)
	}
	if s.CheckAccess("child", "safe-a") == nil {
		t.Fatal("paused lease can still stream")
	}
	*now = now.Add(time.Hour)
	// Recreate the service, as after a server restart. A seek/history reset has no effect.
	restarted := NewFamilyPlaybackService(s.db)
	restarted.now = s.now
	if err := s.db.Where("user_id = ?", "child").Delete(&model.WatchHistory{}).Error; err != nil {
		t.Fatal(err)
	}
	status, err = restarted.Status("child")
	if err != nil || status.UsedSeconds != 10 {
		t.Fatalf("budget reset %+v %v", status, err)
	}
	grant, err = restarted.Start("child", "safe-b")
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(20 * time.Second) // Lost telemetry charges only the reserved 15 seconds.
	status, err = restarted.Status("child")
	if err != nil || status.UsedSeconds != 25 {
		t.Fatalf("lost heartbeat %+v %v", status, err)
	}
	if restarted.CheckAccess("child", "safe-b") == nil {
		t.Fatal("expired grant still streams")
	}
	if _, err := restarted.Renew("child", grant.LeaseID); !errors.Is(err, ErrFamilyLeaseExpired) {
		t.Fatal(err)
	}
}

func TestFamilyBudgetCannotOverspendOrUseAnotherAccountsLease(t *testing.T) {
	s, now := familyClock(t)
	var wg sync.WaitGroup
	grants := make(chan FamilyPlaybackStatus, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if grant, err := s.Start("child", "safe-a"); err == nil {
				grants <- grant
			}
		}()
	}
	wg.Wait()
	close(grants)
	if len(grants) != 1 {
		t.Fatalf("concurrent starts = %d", len(grants))
	}
	grant := <-grants
	if _, err := s.Renew("parent", grant.LeaseID); err == nil {
		t.Fatal("another account renewed lease")
	}
	for i := 0; i < 3; i++ {
		*now = now.Add(15 * time.Second)
		var err error
		grant, err = s.Start("child", "safe-b")
		if err != nil {
			t.Fatal(err)
		}
	}
	*now = now.Add(15 * time.Second)
	if _, err := s.Start("child", "safe-a"); !errors.Is(err, ErrTimeLimitExceeded) {
		t.Fatalf("overspent: %v", err)
	}
	status, err := s.Status("child")
	if err != nil || status.UsedSeconds != 60 || status.RemainingSeconds != 0 {
		t.Fatalf("quota %+v %v", status, err)
	}
	// Parent may increase today's limit; already consumed time remains charged.
	s.db.Model(&model.UserPermission{}).Where("user_id = ?", "child").Update("daily_time_limit", 2)
	grant, err = s.Start("child", "safe-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Stop("child", "stale-lease-id"); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckAccess("child", "safe-a"); err != nil {
		t.Fatal("stale stop closed active lease", err)
	}
}

func TestFamilyBudgetUsesServerDayAndRejectsRevokedOrUnreviewedPlayback(t *testing.T) {
	s, now := familyClock(t)
	*now = time.Date(2026, 10, 27, 23, 59, 55, 0, now.Location())
	grant, err := s.Start("child", "safe-a")
	if err != nil || grant.ExpiresInMS != 5000 {
		t.Fatalf("midnight grant %+v %v", grant, err)
	}
	*now = now.Add(5 * time.Second)
	grant, err = s.Start("child", "safe-a")
	if err != nil || grant.Day != "2026-10-28" || grant.UsedSeconds != 0 {
		t.Fatalf("new day %+v %v", grant, err)
	}
	s.db.Model(&model.User{}).Where("id = ?", "child").Update("token_version", 1)
	if s.CheckAccess("child", "safe-a") == nil {
		t.Fatal("revoked version allowed")
	}
	if _, err := s.Renew("child", grant.LeaseID); !errors.Is(err, ErrFamilyLeaseExpired) {
		t.Fatal(err)
	}
	if _, err := s.Start("child", "DO_NOT_LEAK_UNRATED"); !errors.Is(err, ErrContentRestricted) {
		t.Fatal(err)
	}
	if err := s.db.Migrator().DropTable(&model.FamilyViewingDay{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start("child", "safe-a"); err == nil {
		t.Fatal("storage failure granted free time")
	}
}

func TestOnlyParentControlsChildAutoplay(t *testing.T) {
	s, _ := familyClock(t)
	repos := repository.NewRepositories(s.db)
	users := NewUserService(repos.User, nil, nil, zap.NewNop().Sugar())
	on := true
	prefs := &model.ViewingPreferences{SubtitleLanguage: "zh", AutoPlayNext: &on}
	user, _, err := users.UpdateSelfProfile("child", &UpdateSelfProfileRequest{ViewingPreferences: prefs})
	if err != nil || user.ViewingPreferences.AutoPlayNext == nil || *user.ViewingPreferences.AutoPlayNext {
		t.Fatalf("child changed autoplay %+v %v", user, err)
	}
	permissions := NewPermissionService(repos.UserPermission, repos.ContentRating, repos.WatchHistory, zap.NewNop().Sugar(), s.db)
	if err := permissions.UpdateUserPermission("child", "kids", "G", 1, true, true); err != nil {
		t.Fatal(err)
	}
	user, err = users.GetProfile("child")
	if err != nil || !*user.ViewingPreferences.AutoPlayNext {
		t.Fatal("parent setting not applied", err)
	}
	// Older administrators omitting the new field must preserve the parent choice.
	if err := permissions.UpdateUserPermission("child", "kids", "G", 1); err != nil {
		t.Fatal(err)
	}
	user, err = users.GetProfile("child")
	if err != nil || !*user.ViewingPreferences.AutoPlayNext {
		t.Fatal("legacy policy save reset autoplay", err)
	}
}

func TestFamilyHeartbeatRoundingDoesNotInflateContinuousUsage(t *testing.T) {
	s, now := familyClock(t)
	grant, err := s.Start("child", "safe-a")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		*now = now.Add(5100 * time.Millisecond)
		if _, err := s.Renew("child", grant.LeaseID); err != nil {
			t.Fatal(err)
		}
	}
	status, err := s.Stop("child", grant.LeaseID)
	if err != nil || status.UsedSeconds != 31 {
		t.Fatalf("30.6 seconds inflated by heartbeat rounding: %+v %v", status, err)
	}
}
