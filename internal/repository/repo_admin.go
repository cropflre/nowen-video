package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== ScheduledTaskRepo ====================

type ScheduledTaskRepo struct {
	db *gorm.DB
}

func (r *ScheduledTaskRepo) Create(task *model.ScheduledTask) error {
	return r.db.Create(task).Error
}

func (r *ScheduledTaskRepo) FindByID(id string) (*model.ScheduledTask, error) {
	var task model.ScheduledTask
	err := r.db.First(&task, "id = ?", id).Error
	return &task, err
}

func (r *ScheduledTaskRepo) List() ([]model.ScheduledTask, error) {
	var tasks []model.ScheduledTask
	err := r.db.Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

func (r *ScheduledTaskRepo) Update(task *model.ScheduledTask) error {
	return r.db.Save(task).Error
}

func (r *ScheduledTaskRepo) Delete(id string) error {
	return r.db.Delete(&model.ScheduledTask{}, "id = ?", id).Error
}

func (r *ScheduledTaskRepo) ListEnabled() ([]model.ScheduledTask, error) {
	var tasks []model.ScheduledTask
	err := r.db.Where("enabled = ?", true).Find(&tasks).Error
	return tasks, err
}

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
	existing.AllowedLibraries = perm.AllowedLibraries
	existing.MaxRatingLevel = perm.MaxRatingLevel
	existing.DailyTimeLimit = perm.DailyTimeLimit
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
