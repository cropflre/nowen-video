package service

import (
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrFamilyLeaseBusy    = errors.New("这个账号正在另一处观看，请先暂停那里，再试一次")
	ErrFamilyLeaseExpired = errors.New("观看授权已到期，请重新开始播放")
)

type FamilyPlaybackStatus struct {
	ChildMode         bool   `json:"child_mode"`
	AllowAutoPlayNext bool   `json:"allow_auto_play_next"`
	Day               string `json:"day"`
	Timezone          string `json:"timezone"`
	DailyLimitSeconds int64  `json:"daily_limit_seconds"`
	UsedSeconds       int64  `json:"used_seconds"`
	RemainingSeconds  int64  `json:"remaining_seconds"`
	LeaseID           string `json:"lease_id,omitempty"`
	ExpiresInMS       int64  `json:"expires_in_ms,omitempty"`
}

type FamilyPlaybackService struct {
	db  *gorm.DB
	now func() time.Time
}

func NewFamilyPlaybackService(db *gorm.DB) *FamilyPlaybackService {
	return &FamilyPlaybackService{db: db, now: time.Now}
}

func familyPolicy(db *gorm.DB, userID string) (model.User, model.UserPermission, error) {
	var user model.User
	var perm model.UserPermission
	if err := db.First(&user, "id = ?", userID).Error; err != nil {
		return user, perm, err
	}
	if user.Disabled || (user.ChildMode && user.Role == "admin") {
		return user, perm, ErrForbidden
	}
	err := db.Where("user_id = ?", userID).First(&perm).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	if user.ChildMode && (perm.DailyTimeLimit < 0 || perm.DailyTimeLimit > 1440) {
		return user, perm, ErrInvalidPermission
	}
	return user, perm, err
}

// Status never exposes another page's lease ID. Usage is independent of history
// edits, seeking, media duration, playback speed and client timestamps.
func (s *FamilyPlaybackService) Status(userID string) (FamilyPlaybackStatus, error) {
	var result FamilyPlaybackStatus
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user, perm, err := familyPolicy(tx, userID)
		if err != nil {
			return err
		}
		result, err = s.status(tx, user, perm, s.now())
		return err
	})
	return result, err
}

func (s *FamilyPlaybackService) status(tx *gorm.DB, user model.User, perm model.UserPermission, now time.Time) (FamilyPlaybackStatus, error) {
	r := FamilyPlaybackStatus{ChildMode: user.ChildMode, AllowAutoPlayNext: perm.AllowAutoPlayNext, Day: now.Format("2006-01-02"), Timezone: now.Format("MST Z07:00")}
	if !user.ChildMode {
		return r, nil
	}
	r.DailyLimitSeconds = int64(perm.DailyTimeLimit) * 60
	var day model.FamilyViewingDay
	err := tx.Where("user_id = ? AND day = ?", user.ID, r.Day).First(&day).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return r, err
	}
	r.UsedSeconds = day.ChargedSeconds
	var lease model.FamilyViewingLease
	err = tx.Where("user_id = ?", user.ID).First(&lease).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return r, err
	}
	if lease.Day == r.Day {
		r.UsedSeconds -= unspentFamilySeconds(lease, now)
	}
	if r.UsedSeconds < 0 {
		r.UsedSeconds = 0
	}
	if r.DailyLimitSeconds > 0 {
		r.RemainingSeconds = max(int64(0), r.DailyLimitSeconds-r.UsedSeconds)
	}
	return r, nil
}

func unspentFamilySeconds(lease model.FamilyViewingLease, now time.Time) int64 {
	if lease.ID == "" || !now.Before(lease.ExpiresAt) {
		return 0
	}
	// Round actual elapsed time upwards. Repeated start/stop cannot produce free
	// sub-second playback; a backwards server clock never refunds a reservation.
	if now.Before(lease.StartedAt) {
		return 0
	}
	elapsed := int64(math.Ceil(now.Sub(lease.StartedAt).Seconds()))
	return max(int64(0), lease.ReservedSeconds-elapsed)
}

// Start, Renew and Stop serialize on the account lease row before reading the
// budget. Reservations are charged atomically and persisted before returning.
func (s *FamilyPlaybackService) Start(userID, mediaID string) (FamilyPlaybackStatus, error) {
	return s.mutate(userID, mediaID, "", "start")
}
func (s *FamilyPlaybackService) Renew(userID, leaseID string) (FamilyPlaybackStatus, error) {
	return s.mutate(userID, "", leaseID, "renew")
}
func (s *FamilyPlaybackService) Stop(userID, leaseID string) (FamilyPlaybackStatus, error) {
	return s.mutate(userID, "", leaseID, "stop")
}

func (s *FamilyPlaybackService) mutate(userID, mediaID, leaseID, action string) (FamilyPlaybackStatus, error) {
	var result FamilyPlaybackStatus
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.FamilyViewingLease{UserID: userID}).Error; err != nil {
			return err
		}
		// SQLite acquires its write lock at the INSERT above. FOR UPDATE covers
		// databases that support row locks; no process-local mutex is required.
		var lease model.FamilyViewingLease
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lease, "user_id = ?", userID).Error; err != nil {
			return err
		}
		user, perm, err := familyPolicy(tx, userID)
		if err != nil {
			return err
		}
		now := s.now()
		if !user.ChildMode {
			return ErrForbidden
		}
		if action == "stop" {
			if lease.ID != leaseID || leaseID == "" {
				result, err = s.status(tx, user, perm, now)
				return err
			} // Idempotent; stale stop cannot close a new lease.
		} else if action == "renew" {
			if lease.ID != leaseID || leaseID == "" || !now.Before(lease.ExpiresAt) || now.Before(lease.StartedAt) || lease.TokenVersion != user.TokenVersion {
				return ErrFamilyLeaseExpired
			}
			mediaID = lease.MediaID
		} else if lease.ID != "" && now.Before(lease.ExpiresAt) && lease.TokenVersion == user.TokenVersion {
			return ErrFamilyLeaseBusy
		}
		if action != "stop" {
			catalog := NewChildCatalog(tx)
			policy, err := catalog.Policy(userID)
			if err != nil {
				return err
			}
			if err := catalog.CheckMedia(policy, mediaID); err != nil {
				return ErrContentRestricted
			}
		}
		if action == "renew" && perm.DailyTimeLimit > 0 {
			var day model.FamilyViewingDay
			if err := tx.First(&day, "user_id = ? AND day = ?", userID, lease.Day).Error; err != nil {
				return err
			}
			if day.ChargedSeconds < 0 {
				return ErrInvalidPermission
			}
			until := now.Add(15 * time.Second)
			nextDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			if until.After(nextDay) {
				until = nextDay
			}
			extra := int64(math.Floor(until.Sub(lease.ExpiresAt).Seconds()))
			remaining := int64(perm.DailyTimeLimit)*60 - day.ChargedSeconds
			if extra > remaining {
				extra = remaining
			}
			if extra > 0 {
				if err := tx.Model(&day).UpdateColumn("charged_seconds", gorm.Expr("charged_seconds + ?", extra)).Error; err != nil {
					return err
				}
				lease.ReservedSeconds += extra
				lease.ExpiresAt = lease.ExpiresAt.Add(time.Duration(extra) * time.Second)
			}
			// Preserve the original start: rounding each heartbeat separately would
			// inflate usage on slow networks. Round only the continuous interval.
			if err := tx.Save(&lease).Error; err != nil {
				return err
			}
			result, err = s.grantStatus(tx, user, perm, lease, now)
			return err
		}
		if refund := unspentFamilySeconds(lease, now); refund > 0 {
			if err := tx.Model(&model.FamilyViewingDay{}).Where("user_id = ? AND day = ?", userID, lease.Day).UpdateColumn("charged_seconds", gorm.Expr("MAX(0, charged_seconds - ?)", refund)).Error; err != nil {
				return err
			}
		}
		lease.ID = ""
		lease.ReservedSeconds = 0
		if action != "stop" && perm.DailyTimeLimit > 0 {
			day := model.FamilyViewingDay{UserID: userID, Day: now.Format("2006-01-02")}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&day).Error; err != nil {
				return err
			}
			if err := tx.First(&day, "user_id = ? AND day = ?", userID, day.Day).Error; err != nil {
				return err
			}
			if day.ChargedSeconds < 0 {
				return ErrInvalidPermission
			}
			remaining := int64(perm.DailyTimeLimit)*60 - day.ChargedSeconds
			if remaining <= 0 {
				return ErrTimeLimitExceeded
			}
			nextDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			seconds := int64(15)
			if remaining < seconds {
				seconds = remaining
			}
			if untilMidnight := int64(math.Ceil(nextDay.Sub(now).Seconds())); untilMidnight < seconds {
				seconds = untilMidnight
			}
			lease = model.FamilyViewingLease{UserID: userID, ID: leaseID, MediaID: mediaID, Day: day.Day, TokenVersion: user.TokenVersion, StartedAt: now, ExpiresAt: now.Add(time.Duration(seconds) * time.Second), ReservedSeconds: seconds}
			if lease.ExpiresAt.After(nextDay) {
				lease.ExpiresAt = nextDay
			}
			if action == "start" {
				lease.ID = uuid.NewString()
			}
			charged := tx.Model(&model.FamilyViewingDay{}).Where("user_id = ? AND day = ? AND charged_seconds + ? <= ?", userID, day.Day, seconds, int64(perm.DailyTimeLimit)*60).UpdateColumn("charged_seconds", gorm.Expr("charged_seconds + ?", seconds))
			if charged.Error != nil {
				return charged.Error
			}
			if charged.RowsAffected != 1 {
				return ErrTimeLimitExceeded
			}
		}
		if err := tx.Save(&lease).Error; err != nil {
			return err
		}
		result, err = s.grantStatus(tx, user, perm, lease, now)
		return err
	})
	return result, err
}

func (s *FamilyPlaybackService) grantStatus(tx *gorm.DB, user model.User, perm model.UserPermission, lease model.FamilyViewingLease, now time.Time) (FamilyPlaybackStatus, error) {
	result, err := s.status(tx, user, perm, now)
	result.LeaseID = lease.ID
	if lease.ID != "" {
		result.ExpiresInMS = lease.ExpiresAt.Sub(now).Milliseconds()
	}
	return result, err
}

// CheckAccess protects new stream/session/segment requests. Already downloaded
// data cannot be recalled; the official player also enforces the grant deadline.
func (s *FamilyPlaybackService) CheckAccess(userID, mediaID string) error {
	user, perm, err := familyPolicy(s.db, userID)
	if err != nil {
		return err
	}
	if !user.ChildMode || perm.DailyTimeLimit <= 0 {
		return nil
	}
	var lease model.FamilyViewingLease
	if err := s.db.First(&lease, "user_id = ?", userID).Error; err != nil {
		return ErrFamilyLeaseExpired
	}
	now := s.now()
	if lease.ID == "" || lease.MediaID != mediaID || lease.TokenVersion != user.TokenVersion || !now.Before(lease.ExpiresAt) || now.Before(lease.StartedAt) {
		return ErrFamilyLeaseExpired
	}
	return nil
}
