package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const legacyProjectionRollbackWindow = 7 * 24 * time.Hour

type legacyProjectionInventoryReport struct {
	TasksFound       int
	JobsImported     int
	ArtifactsQueued  int
	ArtifactsBlocked int
	MissingPaths     int
}

func (r legacyProjectionInventoryReport) Changed() bool {
	return r.JobsImported > 0 || r.ArtifactsQueued > 0 || r.ArtifactsBlocked > 0 || r.MissingPaths > 0
}

func (s *ArtifactMaintenanceService) inventoryLegacyTranscodeProjection(now time.Time) (legacyProjectionInventoryReport, error) {
	report := legacyProjectionInventoryReport{}
	if s == nil || s.repo == nil || s.repo.DB() == nil || s.cfg == nil {
		return report, nil
	}
	tasks, err := s.repo.ListLegacyTerminalWithOutput(500)
	if err != nil {
		return report, fmt.Errorf("list legacy transcode projection: %w", err)
	}
	report.TasksFound = len(tasks)
	root := filepath.Join(s.cfg.Cache.CacheDir, "transcode")
	db := s.repo.DB()

	for index := range tasks {
		task := &tasks[index]
		job, imported, jobErr := ensureLegacyProjectionJob(db, task)
		if jobErr != nil {
			return report, jobErr
		}
		if imported {
			report.JobsImported++
		}

		artifactID := deterministicLegacyProjectionID("artifact", task.ID)
		var existing int64
		if err := db.Model(&model.TranscodeArtifactRecord{}).Where("id = ?", artifactID).Count(&existing).Error; err != nil {
			return report, fmt.Errorf("check legacy artifact %s: %w", artifactID, err)
		}
		if existing > 0 {
			continue
		}

		rollbackUntil := now.Add(legacyProjectionRollbackWindow)
		outputDir := filepath.Clean(strings.TrimSpace(task.OutputDir))
		artifact := &model.TranscodeArtifactRecord{
			ID:                   artifactID,
			JobID:                job.ID,
			MediaID:              task.MediaID,
			Kind:                 repository.LegacyTranscodeArtifactKind,
			ProfileID:            task.Quality,
			Path:                 outputDir,
			Status:               "expired",
			MigrationSource:      repository.LegacyTranscodeArtifactMigrationSource,
			CleanupState:         repository.ArtifactCleanupPending,
			CleanupNextAttemptAt: &rollbackUntil,
			CleanupRollbackUntil: &rollbackUntil,
			CreatedAt:            task.CreatedAt,
			UpdatedAt:            now,
		}
		if artifact.CreatedAt.IsZero() {
			artifact.CreatedAt = now
		}

		if !legacyProjectionPathAllowed(root, outputDir) {
			artifact.CleanupState = repository.ArtifactCleanupBlocked
			artifact.CleanupNextAttemptAt = nil
			artifact.CleanupErrorCode = "legacy_path_outside_store"
			artifact.CleanupErrorMessage = "legacy output directory is outside the managed transcode store"
			report.ArtifactsBlocked++
		} else if info, statErr := os.Stat(outputDir); errors.Is(statErr, os.ErrNotExist) {
			artifact.Status = "deleted"
			artifact.Path = ""
			artifact.CleanupState = repository.ArtifactCleanupCompleted
			artifact.CleanupCompletedAt = &now
			artifact.CleanupDisposition = "missing_at_inventory"
			artifact.CleanupOriginalPath = outputDir
			artifact.CleanupNextAttemptAt = nil
			artifact.CleanupRollbackUntil = nil
			report.MissingPaths++
		} else if statErr != nil {
			artifact.CleanupState = repository.ArtifactCleanupRetryWait
			artifact.CleanupErrorCode = "legacy_path_unavailable"
			artifact.CleanupErrorMessage = statErr.Error()
			report.ArtifactsBlocked++
		} else if !info.IsDir() {
			artifact.CleanupState = repository.ArtifactCleanupBlocked
			artifact.CleanupNextAttemptAt = nil
			artifact.CleanupErrorCode = "legacy_path_not_directory"
			artifact.CleanupErrorMessage = "legacy output path is not a directory"
			report.ArtifactsBlocked++
		} else {
			artifact.SizeBytes, _ = directorySize(outputDir)
			manifest := filepath.Join(outputDir, "stream.m3u8")
			if _, manifestErr := os.Stat(manifest); manifestErr == nil {
				artifact.ManifestPath = manifest
			}
			report.ArtifactsQueued++
		}

		if err := s.executionRepo.ImportLegacyHLSArtifact(artifact); err != nil {
			return report, fmt.Errorf("import legacy transcode artifact %s: %w", artifact.ID, err)
		}
	}
	return report, nil
}

func ensureLegacyProjectionJob(db *gorm.DB, task *model.TranscodeTask) (*model.TranscodeJobRecord, bool, error) {
	var existing model.TranscodeJobRecord
	result := db.Where("legacy_task_id = ?", task.ID).Limit(1).Find(&existing)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		return &existing, false, nil
	}
	legacyID := task.ID
	completedAt := task.CompletedAt
	if completedAt == nil {
		value := task.UpdatedAt
		if value.IsZero() {
			value = time.Now()
		}
		completedAt = &value
	}
	status := "cancelled"
	switch strings.ToLower(strings.TrimSpace(task.Status)) {
	case "done", "completed":
		status = "completed"
	case "failed":
		status = "failed"
	}
	job := &model.TranscodeJobRecord{
		ID:           deterministicLegacyProjectionID("job", task.ID),
		LegacyTaskID: &legacyID,
		MediaID:      task.MediaID,
		Intent:       "legacy_history_import",
		ProfileID:    task.Quality,
		AudioTrack:   -1,
		Status:       status,
		DesiredState: "cancelled",
		CompletedAt:  completedAt,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = *completedAt
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = *completedAt
	}
	create := db.Clauses(clause.OnConflict{DoNothing: true}).Create(job)
	if create.Error != nil {
		return nil, false, fmt.Errorf("create legacy history job: %w", create.Error)
	}
	if create.RowsAffected == 0 {
		if err := db.Where("legacy_task_id = ?", task.ID).First(&existing).Error; err != nil {
			return nil, false, err
		}
		return &existing, false, nil
	}
	return job, true, nil
}

func deterministicLegacyProjectionID(kind, legacyTaskID string) string {
	return "legacy-" + kind + "-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(legacyTaskID)).String()
}

func legacyProjectionPathAllowed(root, candidate string) bool {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false
	}
	candidateAbs, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	first := strings.Split(relative, string(filepath.Separator))[0]
	switch first {
	case "artifacts", "workspaces", "ondemand":
		return false
	default:
		return true
	}
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.Walk(root, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
