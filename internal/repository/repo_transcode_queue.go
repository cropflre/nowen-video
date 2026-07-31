package repository

import "github.com/nowen-video/nowen-video/internal/model"

func (r *TranscodeExecutionRepo) CountQueuedJobs() (int64, error) {
	var count int64
	err := r.db.Model(&model.TranscodeJobRecord{}).
		Where("active_key IS NOT NULL AND status = ? AND desired_state = ?", "queued", "running").
		Count(&count).Error
	return count, err
}
