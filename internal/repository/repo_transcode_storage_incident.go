package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TranscodeStorageIncidentInput struct {
	Code             string
	Severity         string
	Operation        string
	Path             string
	Message          string
	Retryable        bool
	AdmissionBlocked bool
	QueuePaused      bool
}

type TranscodeStorageIncidentSummary struct {
	ActiveCount      int64 `json:"active_count"`
	CriticalCount    int64 `json:"critical_count"`
	RecoveredCount   int64 `json:"recovered_count"`
	TotalOccurrences int64 `json:"total_occurrences"`
}

func storageIncidentActiveKey(input TranscodeStorageIncidentInput) string {
	return strings.Join([]string{
		strings.TrimSpace(input.Operation),
		strings.TrimSpace(input.Path),
		strings.TrimSpace(input.Code),
	}, "|")
}

func (r *TranscodeExecutionRepo) ReportStorageIncident(input TranscodeStorageIncidentInput, now time.Time) (*model.TranscodeStorageIncidentRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("transcode storage incident repository is unavailable")
	}
	if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.Operation) == "" {
		return nil, fmt.Errorf("storage incident code and operation are required")
	}
	key := storageIncidentActiveKey(input)
	record := model.TranscodeStorageIncidentRecord{
		ActiveKey:        &key,
		Code:             input.Code,
		Severity:         input.Severity,
		Operation:        input.Operation,
		Path:             input.Path,
		Message:          input.Message,
		Retryable:        input.Retryable,
		AdmissionBlocked: input.AdmissionBlocked,
		QueuePaused:      input.QueuePaused,
		Occurrences:      1,
		FirstSeenAt:      now,
		LastSeenAt:       now,
		Status:           model.TranscodeStorageIncidentActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "active_key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"severity":          input.Severity,
			"message":           input.Message,
			"retryable":         input.Retryable,
			"admission_blocked": input.AdmissionBlocked,
			"queue_paused":      input.QueuePaused,
			"occurrences":       gorm.Expr("occurrences + 1"),
			"last_seen_at":      now,
			"updated_at":        now,
		}),
	}).Create(&record).Error; err != nil {
		return nil, err
	}
	var active model.TranscodeStorageIncidentRecord
	if err := r.db.Where("active_key = ? AND status = ?", key, model.TranscodeStorageIncidentActive).
		First(&active).Error; err != nil {
		return nil, err
	}
	return &active, nil
}

func (r *TranscodeExecutionRepo) RecoverStorageIncidents(operation string, recoveredAt time.Time) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("transcode storage incident repository is unavailable")
	}
	query := r.db.Model(&model.TranscodeStorageIncidentRecord{}).
		Where("status = ?", model.TranscodeStorageIncidentActive)
	if strings.TrimSpace(operation) != "" {
		query = query.Where("operation = ?", operation)
	}
	result := query.Updates(map[string]any{
		"active_key":   nil,
		"status":       model.TranscodeStorageIncidentRecovered,
		"recovered_at": recoveredAt,
		"updated_at":   recoveredAt,
	})
	return result.RowsAffected, result.Error
}

func (r *TranscodeExecutionRepo) ListActiveStorageIncidents(limit int) ([]model.TranscodeStorageIncidentRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	var rows []model.TranscodeStorageIncidentRecord
	err := r.db.Where("status = ?", model.TranscodeStorageIncidentActive).
		Order("CASE severity WHEN 'critical' THEN 0 ELSE 1 END, last_seen_at DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (r *TranscodeExecutionRepo) LatestActiveStorageIncident() (*model.TranscodeStorageIncidentRecord, error) {
	var row model.TranscodeStorageIncidentRecord
	result := r.db.Where("status = ?", model.TranscodeStorageIncidentActive).
		Order("CASE severity WHEN 'critical' THEN 0 ELSE 1 END, last_seen_at DESC").
		Limit(1).
		Find(&row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

func (r *TranscodeExecutionRepo) StorageIncidentSummary() (TranscodeStorageIncidentSummary, error) {
	var summary TranscodeStorageIncidentSummary
	if err := r.db.Model(&model.TranscodeStorageIncidentRecord{}).
		Where("status = ?", model.TranscodeStorageIncidentActive).
		Count(&summary.ActiveCount).Error; err != nil {
		return summary, err
	}
	if err := r.db.Model(&model.TranscodeStorageIncidentRecord{}).
		Where("status = ? AND severity = ?", model.TranscodeStorageIncidentActive, "critical").
		Count(&summary.CriticalCount).Error; err != nil {
		return summary, err
	}
	if err := r.db.Model(&model.TranscodeStorageIncidentRecord{}).
		Where("status = ?", model.TranscodeStorageIncidentRecovered).
		Count(&summary.RecoveredCount).Error; err != nil {
		return summary, err
	}
	if err := r.db.Model(&model.TranscodeStorageIncidentRecord{}).
		Select("COALESCE(SUM(occurrences), 0)").
		Scan(&summary.TotalOccurrences).Error; err != nil {
		return summary, err
	}
	return summary, nil
}
