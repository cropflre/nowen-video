package repository

import (
	"errors"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== ContentRatingRepo ====================

type ContentRatingRepo struct {
	db *gorm.DB
}

func (r *ContentRatingRepo) Upsert(rating *model.ContentRating) error {
	var existing model.ContentRating
	err := r.db.Where("media_id = ?", rating.MediaID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(rating).Error
	}
	existing.Level = rating.Level
	return r.db.Save(&existing).Error
}

func (r *ContentRatingRepo) FindByMediaID(mediaID string) (*model.ContentRating, error) {
	var rating model.ContentRating
	err := r.db.Where("media_id = ?", mediaID).First(&rating).Error
	return &rating, err
}

func (r *ContentRatingRepo) Delete(mediaID string) error {
	return r.db.Where("media_id = ?", mediaID).Delete(&model.ContentRating{}).Error
}

// ==================== UserPermissionRepo ====================

type UserPermissionRepo struct {
	db *gorm.DB
}

func (r *UserPermissionRepo) Upsert(perm *model.UserPermission) error {
	var existing model.UserPermission
	err := r.db.Where("user_id = ?", perm.UserID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(perm).Error
	}
	if err != nil {
		return err
	}
	existing.AllowedLibraries = perm.AllowedLibraries
	existing.MaxRatingLevel = perm.MaxRatingLevel
	existing.DailyTimeLimit = perm.DailyTimeLimit
	existing.AllowAutoPlayNext = perm.AllowAutoPlayNext
	return r.db.Save(&existing).Error
}

func (r *UserPermissionRepo) FindByUserID(userID string) (*model.UserPermission, error) {
	var perm model.UserPermission
	err := r.db.Where("user_id = ?", userID).First(&perm).Error
	return &perm, err
}

func (r *UserPermissionRepo) Delete(userID string) error {
	return r.db.Where("user_id = ?", userID).Delete(&model.UserPermission{}).Error
}

// ==================== SystemSettingRepo ====================

type SystemSettingRepo struct {
	db *gorm.DB
}

func (r *SystemSettingRepo) Get(key string) (string, error) {
	var setting model.SystemSetting
	err := r.db.Where("`key` = ?", key).First(&setting).Error
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (r *SystemSettingRepo) Set(key, value string) error {
	var existing model.SystemSetting
	err := r.db.Where("`key` = ?", key).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(&model.SystemSetting{Key: key, Value: value}).Error
	}
	if err != nil {
		return err
	}
	existing.Value = value
	return r.db.Save(&existing).Error
}

func (r *SystemSettingRepo) GetAll() (map[string]string, error) {
	var settings []model.SystemSetting
	err := r.db.Find(&settings).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

func (r *SystemSettingRepo) SetMulti(kvs map[string]string) error {
	for key, value := range kvs {
		if err := r.Set(key, value); err != nil {
			return err
		}
	}
	return nil
}

// UpdateFamilyPolicy saves permissions and the account mode atomically. Any
// policy change revokes previously issued tokens, including cached image URLs.
func (r *UserPermissionRepo) UpdateFamilyPolicy(perm *model.UserPermission) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.First(&user, "id = ?", perm.UserID).Error; err != nil {
			return err
		}
		if user.Role == "admin" && perm.ChildMode {
			return ErrInvalidFamilyPolicy
		}
		scoped := &UserPermissionRepo{db: tx}
		if err := scoped.Upsert(perm); err != nil {
			return err
		}
		return tx.Model(&model.User{}).Where("id = ?", perm.UserID).Updates(map[string]interface{}{
			"child_mode": perm.ChildMode, "token_version": gorm.Expr("token_version + 1"),
		}).Error
	})
}

var ErrInvalidFamilyPolicy = errors.New("administrators cannot be child accounts")
