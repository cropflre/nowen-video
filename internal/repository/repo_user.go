package repository

import (
	"errors"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== UserRepo ====================

type UserRepo struct {
	db *gorm.DB
}

func (r *UserRepo) DB() *gorm.DB { return r.db }

func (r *UserRepo) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepo) FindByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err == nil {
		err = r.applyChildViewingPreferences(&user)
	}
	return &user, err
}

func (r *UserRepo) FindByID(id string) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, "id = ?", id).Error
	if err == nil {
		err = r.applyChildViewingPreferences(&user)
	}
	return &user, err
}

func (r *UserRepo) List() ([]model.User, error) {
	var users []model.User
	err := r.db.Order("created_at DESC").Find(&users).Error
	return users, err
}

func (r *UserRepo) Count() (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).Count(&count).Error
	return count, err
}

func (r *UserRepo) Delete(id string) error {
	return r.db.Delete(&model.User{}, "id = ?", id).Error
}

// UpdatePassword 更新用户密码
func (r *UserRepo) UpdatePassword(id string, hashedPassword string) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Update("password", hashedPassword).Error
}

// UpdateFields 通用字段更新
func (r *UserRepo) UpdateFields(id string, fields map[string]interface{}) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Updates(fields).Error
}

// IncrTokenVersion 令牌版本号自增（用于吊销所有旧Token）
func (r *UserRepo) IncrTokenVersion(id string) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
}

// UpdateLastLogin 更新最后登录信息
func (r *UserRepo) UpdateLastLogin(id, ip string) error {
	now := time.Now()
	return r.db.Model(&model.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_login_at": &now,
		"last_login_ip": ip,
	}).Error
}

// SetDisabled 设置用户启用/禁用状态
func (r *UserRepo) SetDisabled(id string, disabled bool) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Update("disabled", disabled).Error
}

// CountAdmins 统计管理员人数（用于防止删除最后一个管理员）
func (r *UserRepo) CountAdmins() (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).Where("role = ?", "admin").Count(&count).Error
	return count, err
}

// ErrInviteUnavailable covers expired, revoked and exhausted invitations.
var ErrInviteUnavailable = errors.New("invitation unavailable")

// CreateWithInvite consumes a slot and creates the account in one transaction.
// A failed account insert rolls back the slot; concurrent joins cannot exceed max_uses.
func (r *UserRepo) CreateWithInvite(user *model.User, inviteID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.InviteCode{}).
			Where("id = ? AND (expires_at IS NULL OR expires_at > ?) AND (max_uses = 0 OR used_count < max_uses)", inviteID, time.Now()).
			UpdateColumn("used_count", gorm.Expr("used_count + 1"))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInviteUnavailable
		}
		return tx.Create(user).Error
	})
}

// The child autoplay value is parent policy, never a self-service preference.
func (r *UserRepo) applyChildViewingPreferences(user *model.User) error {
	if !user.ChildMode {
		return nil
	}
	var permission model.UserPermission
	err := r.db.Where("user_id = ?", user.ID).First(&permission).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	allowed := permission.AllowAutoPlayNext
	user.ViewingPreferences.AutoPlayNext = &allowed
	return nil
}
