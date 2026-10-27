package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrDeviceUnavailable = middleware.ErrDeviceSessionRejected
	ErrPairUnavailable   = errors.New("配对码无效或已过期，请在新设备重新生成")
	ErrPairFinished      = errors.New("配对已完成，请在新设备重新开始")
	ErrPairBusy          = errors.New("这个配对请求已经处理，请重新生成配对码")
	ErrPairCapacity      = errors.New("配对请求较多，请稍后再试")
)

const pairingAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func deviceDisplayName(name, ua string) string {
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name))
	if name != "" {
		return string([]rune(name)[:min(80, len([]rune(name)))])
	}
	browser := "浏览器"
	for _, item := range []struct{ match, name string }{{"Edg/", "Edge"}, {"Firefox/", "Firefox"}, {"FxiOS/", "Firefox"}, {"Chrome/", "Chrome"}, {"CriOS/", "Chrome"}, {"Safari/", "Safari"}} {
		if strings.Contains(ua, item.match) {
			browser = item.name
			break
		}
	}
	for _, os := range []string{"iPad", "iPhone", "Android", "Windows", "Macintosh", "Linux"} {
		if strings.Contains(ua, os) {
			if os == "Macintosh" {
				os = "Mac"
			}
			return browser + " · " + os
		}
	}
	return browser
}

func (s *AuthService) newDeviceToken(user *model.User, name string) (*TokenResponse, error) {
	var result *TokenResponse
	err := s.userRepo.DB().Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Where("expires_at < ?", now.Add(-90*24*time.Hour)).Delete(&model.DeviceSession{}).Error; err != nil {
			return err
		}
		device := model.DeviceSession{ID: uuid.NewString(), UserID: user.ID, TokenVersion: user.TokenVersion, Name: name, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(30 * 24 * time.Hour)}
		if err := tx.Create(&device).Error; err != nil {
			return err
		}
		var err error
		result, err = s.generateDeviceToken(user, device.ID, device.ExpiresAt)
		return err
	})
	return result, err
}

func (s *AuthService) activeDeviceSession(userID, sessionID string) (*model.DeviceSession, error) {
	return findActiveDeviceSession(s.userRepo.DB(), userID, sessionID)
}

func findActiveDeviceSession(db *gorm.DB, userID, sessionID string) (*model.DeviceSession, error) {
	var session model.DeviceSession
	err := db.Where("id = ? AND user_id = ? AND revoked_at IS NULL AND expires_at > ?", sessionID, userID, time.Now()).
		Where("token_version = (SELECT token_version FROM users WHERE id = ? AND deleted_at IS NULL AND disabled = ?)", userID, false).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceUnavailable
		}
		return nil, err
	}
	return &session, nil
}

func (s *AuthService) ValidateDeviceSession(userID, sessionID string) error {
	if sessionID == "" {
		return nil
	} // Existing signed credentials retain their account-wide revocation boundary.
	session, err := s.activeDeviceSession(userID, sessionID)
	if err != nil {
		return err
	}
	if session.LastSeenAt.Before(time.Now().Add(-5 * time.Minute)) {
		// Avoid a write per video segment. Failure to update metadata never revives a session.
		_ = s.userRepo.DB().Model(session).Where("last_seen_at < ?", time.Now().Add(-5*time.Minute)).UpdateColumn("last_seen_at", time.Now()).Error
	}
	return nil
}

func (s *AuthService) Devices(userID string) ([]model.DeviceSession, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	devices := []model.DeviceSession{}
	err = s.userRepo.DB().Where("user_id = ? AND token_version = ? AND revoked_at IS NULL AND expires_at > ?", userID, user.TokenVersion, time.Now()).Order("last_seen_at DESC").Find(&devices).Error
	return devices, err
}

func (s *AuthService) RevokeDevice(userID, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	return s.userRepo.DB().Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.DeviceSession{}).Where("id = ? AND user_id = ? AND revoked_at IS NULL", sessionID, userID).Update("revoked_at", time.Now())
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		} // No existence oracle across accounts.
		return s.deviceAudit(tx, userID, "device.revoke", sessionID)
	})
}

func (s *AuthService) RevokeAllDevices(userID string) error {
	return s.userRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", userID).UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.DeviceSession{}).Where("user_id = ? AND revoked_at IS NULL", userID).Update("revoked_at", time.Now()).Error; err != nil {
			return err
		}
		return s.deviceAudit(tx, userID, "device.revoke_all", userID)
	})
}

func (s *AuthService) deviceAudit(tx *gorm.DB, userID, action, target string) error {
	if s.auditRepo == nil {
		return nil
	}
	return tx.Create(&model.AuditLog{OperatorID: userID, Action: action, TargetType: "device", TargetID: target}).Error
}

func pairingHash(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])
}
func normalizePairCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.Join(strings.Fields(code), ""), "-", ""))
}

type DevicePairRequest struct {
	ID          string `json:"id"`
	Secret      string `json:"secret"`
	Code        string `json:"code"`
	DeviceName  string `json:"device_name"`
	ExpiresInMS int64  `json:"expires_in_ms"`
}
type DevicePairReview struct {
	DeviceName string    `json:"device_name"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type DevicePairStatus struct {
	Status string         `json:"status"`
	Token  *TokenResponse `json:"authorization,omitempty"`
}

func (s *AuthService) CreateDevicePairing(name, ua string) (*DevicePairRequest, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	for i := range bytes {
		bytes[i] = pairingAlphabet[int(bytes[i])%len(pairingAlphabet)]
	}
	code := string(bytes)
	request := &DevicePairRequest{ID: uuid.NewString(), Secret: base64.RawURLEncoding.EncodeToString(secret), Code: code[:4] + "-" + code[4:], DeviceName: deviceDisplayName(name, ua), ExpiresInMS: 300000}
	now := time.Now()
	err := s.userRepo.DB().Transaction(func(tx *gorm.DB) error {
		// This first write serializes admission on SQLite, and bounds retained requests.
		if err := tx.Where("expires_at < ?", now.Add(-24*time.Hour)).Delete(&model.DevicePairing{}).Error; err != nil {
			return err
		}
		var active int64
		if err := tx.Model(&model.DevicePairing{}).Where("expires_at > ? AND consumed_at IS NULL AND cancelled_at IS NULL", now).Count(&active).Error; err != nil {
			return err
		}
		if active >= 500 {
			return ErrPairCapacity
		}
		return tx.Create(&model.DevicePairing{ID: request.ID, CodeHash: pairingHash(code), SecretHash: pairingHash(request.Secret), DeviceName: request.DeviceName, CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute)}).Error
	})
	return request, err
}

func (s *AuthService) inspectPair(tx *gorm.DB, code string) (*model.DevicePairing, error) {
	code = normalizePairCode(code)
	if len(code) != 8 || strings.IndexFunc(code, func(r rune) bool { return !strings.ContainsRune(pairingAlphabet, r) }) >= 0 {
		return nil, ErrPairUnavailable
	}
	var pair model.DevicePairing
	if err := tx.Where("code_hash = ? AND expires_at > ? AND consumed_at IS NULL AND cancelled_at IS NULL", pairingHash(code), time.Now()).First(&pair).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPairUnavailable
		}
		return nil, err
	}
	return &pair, nil
}

func (s *AuthService) InspectDevicePairing(userID, code string) (*DevicePairReview, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil || user.Disabled || (user.ChildMode && user.Role == "admin") || user.MustChangePwd {
		return nil, ErrForbidden
	}
	pair, err := s.inspectPair(s.userRepo.DB(), code)
	if err != nil {
		return nil, err
	}
	if pair.ApprovedUserID != "" {
		return nil, ErrPairBusy
	}
	return &DevicePairReview{DeviceName: pair.DeviceName, ExpiresAt: pair.ExpiresAt}, nil
}

func (s *AuthService) ApproveDevicePairing(userID, code string, tokenVersion int, sessionIDs ...string) error {
	return s.userRepo.DB().Transaction(func(tx *gorm.DB) error {
		user, err := repository.NewRepositories(tx).User.FindByID(userID)
		if err != nil || user.Disabled || (user.ChildMode && user.Role == "admin") || user.MustChangePwd || user.TokenVersion != tokenVersion {
			return ErrForbidden
		}
		sessionID := ""
		if len(sessionIDs) > 0 {
			sessionID = sessionIDs[0]
		}
		if sessionID != "" {
			if _, err := findActiveDeviceSession(tx, userID, sessionID); err != nil {
				return ErrForbidden
			}
		}
		pair, err := s.inspectPair(tx, code)
		if err != nil {
			return err
		}
		result := tx.Model(pair).Where("approved_user_id = '' AND expires_at > ? AND consumed_at IS NULL AND cancelled_at IS NULL", time.Now()).Updates(map[string]any{"approved_user_id": user.ID, "approved_token_version": user.TokenVersion, "approved_session_id": sessionID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrPairBusy
		}
		return s.deviceAudit(tx, userID, "device.pair_approve", pair.ID)
	})
}

func (s *AuthService) PollDevicePairing(id, secret string, origin ...string) (*DevicePairStatus, error) {
	var response *DevicePairStatus
	err := s.userRepo.DB().Transaction(func(tx *gorm.DB) error {
		var pair model.DevicePairing
		if err := tx.First(&pair, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPairUnavailable
			}
			return err
		}
		if secret == "" || subtle.ConstantTimeCompare([]byte(pair.SecretHash), []byte(pairingHash(secret))) != 1 {
			return ErrPairUnavailable
		}
		if pair.ConsumedAt != nil {
			return ErrPairFinished
		}
		if pair.CancelledAt != nil || !time.Now().Before(pair.ExpiresAt) {
			return ErrPairUnavailable
		}
		if pair.ApprovedUserID == "" {
			response = &DevicePairStatus{Status: "pending"}
			return nil
		}
		user, err := repository.NewRepositories(tx).User.FindByID(pair.ApprovedUserID)
		if err != nil || user.Disabled || (user.ChildMode && user.Role == "admin") || user.MustChangePwd || user.TokenVersion != pair.ApprovedTokenVersion {
			return ErrPairUnavailable
		}
		if pair.ApprovedSessionID != "" {
			if _, err := findActiveDeviceSession(tx, user.ID, pair.ApprovedSessionID); err != nil {
				return ErrPairUnavailable
			}
		}
		now := time.Now()
		claimed := tx.Model(&model.DevicePairing{}).Where("id = ? AND consumed_at IS NULL AND cancelled_at IS NULL AND expires_at > ?", id, now).Update("consumed_at", now)
		if claimed.Error != nil {
			return claimed.Error
		}
		if claimed.RowsAffected != 1 {
			return ErrPairFinished
		}
		device := model.DeviceSession{ID: uuid.NewString(), UserID: user.ID, TokenVersion: user.TokenVersion, Name: pair.DeviceName, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(30 * 24 * time.Hour)}
		if err := tx.Create(&device).Error; err != nil {
			return err
		}
		ip, ua := "", ""
		if len(origin) > 0 {
			ip = origin[0]
		}
		if len(origin) > 1 {
			ua = origin[1]
		}
		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{"last_login_at": now, "last_login_ip": ip}).Error; err != nil {
			return err
		}
		if s.logRepo != nil {
			if err := tx.Create(&model.LoginLog{UserID: user.ID, Username: user.Username, IP: ip, UserAgent: ua, Success: true}).Error; err != nil {
				return err
			}
		}
		user.LastLoginAt = &now
		user.LastLoginIP = ip
		token, err := s.generateDeviceToken(user, device.ID, device.ExpiresAt)
		if err != nil {
			return err
		}
		response = &DevicePairStatus{Status: "authorized", Token: token}
		return s.deviceAudit(tx, user.ID, "device.pair_claim", device.ID)
	})
	return response, err
}

func (s *AuthService) CancelDevicePairing(id, secret string) error {
	if secret == "" {
		return ErrPairUnavailable
	}
	return s.userRepo.DB().Model(&model.DevicePairing{}).Where("id = ? AND secret_hash = ? AND consumed_at IS NULL", id, pairingHash(secret)).Update("cancelled_at", time.Now()).Error
}
