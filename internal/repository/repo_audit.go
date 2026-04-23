package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== LoginLogRepo ====================

type LoginLogRepo struct {
	db *gorm.DB
}

func (r *LoginLogRepo) Create(log *model.LoginLog) error {
	return r.db.Create(log).Error
}

// ListByUser 查询某用户的登录历史（最近 limit 条）
func (r *LoginLogRepo) ListByUser(userID string, limit int) ([]model.LoginLog, error) {
	if limit <= 0 {
		limit = 20
	}
	var logs []model.LoginLog
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// ListRecent 管理员查看最近登录日志
func (r *LoginLogRepo) ListRecent(page, size int, onlyFailed bool) ([]model.LoginLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 50
	}
	q := r.db.Model(&model.LoginLog{})
	if onlyFailed {
		q = q.Where("success = ?", false)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []model.LoginLog
	err := q.Order("created_at DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&logs).Error
	return logs, total, err
}

// CleanOlderThan 清理超过指定天数的登录日志（避免无限增长）
func (r *LoginLogRepo) CleanOlderThan(days int) error {
	if days <= 0 {
		return nil
	}
	threshold := time.Now().AddDate(0, 0, -days)
	return r.db.Where("created_at < ?", threshold).Delete(&model.LoginLog{}).Error
}

// ==================== AuditLogRepo ====================

type AuditLogRepo struct {
	db *gorm.DB
}

func (r *AuditLogRepo) Create(log *model.AuditLog) error {
	return r.db.Create(log).Error
}

// List 按时间倒序分页
func (r *AuditLogRepo) List(page, size int, action string) ([]model.AuditLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 50
	}
	q := r.db.Model(&model.AuditLog{})
	if action != "" {
		q = q.Where("action = ?", action)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []model.AuditLog
	err := q.Order("created_at DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&logs).Error
	return logs, total, err
}

// ==================== InviteCodeRepo ====================

type InviteCodeRepo struct {
	db *gorm.DB
}

func (r *InviteCodeRepo) Create(code *model.InviteCode) error {
	return r.db.Create(code).Error
}

func (r *InviteCodeRepo) FindByCode(code string) (*model.InviteCode, error) {
	var ic model.InviteCode
	err := r.db.Where("code = ?", code).First(&ic).Error
	return &ic, err
}

func (r *InviteCodeRepo) List() ([]model.InviteCode, error) {
	var codes []model.InviteCode
	err := r.db.Order("created_at DESC").Find(&codes).Error
	return codes, err
}

func (r *InviteCodeRepo) Delete(id string) error {
	return r.db.Delete(&model.InviteCode{}, "id = ?", id).Error
}

// IncrUsed 原子自增使用次数
func (r *InviteCodeRepo) IncrUsed(id string) error {
	return r.db.Model(&model.InviteCode{}).Where("id = ?", id).
		UpdateColumn("used_count", gorm.Expr("used_count + 1")).Error
}
