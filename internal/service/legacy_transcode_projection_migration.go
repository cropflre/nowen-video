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

const (
	legacyProjectionRollbackWindow         = 7 * 24 * time.Hour
	legacyProjectionSourceRetirementWindow = 30 * 24 * time.Hour
	legacyProjectionMigrationLease         = 2 * time.Minute
	legacyProjectionDefaultBatchSize       = 250
)

type legacyProjectionInventoryReport struct {
	TasksFound       int
	JobsImported     int
	ArtifactsQueued  int
	ArtifactsBlocked int
	MissingPaths     int
	Generation       int64
	ScannedRows      int64
	TargetRows       int64
	Status           string
	HasMore          bool
}

func (r legacyProjectionInventoryReport) Changed() bool {
	return r.TasksFound > 0 || r.JobsImported > 0 || r.ArtifactsQueued > 0 || r.ArtifactsBlocked > 0 || r.MissingPaths > 0
}

func (s *ArtifactMaintenanceService) inventoryLegacyTranscodeProjection(now time.Time) (legacyProjectionInventoryReport, error) {
	report := legacyProjectionInventoryReport{}
	if s == nil || s.repo == nil || s.repo.DB() == nil || s.cfg == nil || s.executionRepo == nil {
		return report, nil
	}
	batchSize := s.legacyMigrationBatchSize
	if batchSize <= 0 {
		batchSize = legacyProjectionDefaultBatchSize
	}

	highWater, err := s.repo.LegacyProjectionHighWater()
	if err != nil {
		return report, fmt.Errorf("read legacy projection high-water: %w", err)
	}
	var targetRows int64
	if highWater != nil {
		targetRows, err = s.repo.CountLegacyTerminalWithOutputThrough(*highWater)
		if err != nil {
			return report, fmt.Errorf("count legacy projection target rows: %w", err)
		}
	}
	state, _, err := s.executionRepo.PrepareLegacyProjectionMigration(
		repository.LegacyTranscodeArtifactMigrationSource,
		highWater,
		targetRows,
		batchSize,
		now,
		legacyProjectionSourceRetirementWindow,
	)
	if err != nil {
		return report, fmt.Errorf("prepare legacy projection migration: %w", err)
	}
	report = legacyProjectionReportFromState(state)
	if state == nil || state.Status == repository.LegacyProjectionMigrationCompleted || state.HighWaterUpdatedAt == nil {
		return report, nil
	}

	token := uuid.NewString()
	claimed, ok, err := s.executionRepo.ClaimLegacyProjectionMigration(
		state.Source,
		s.legacyMigrationOwner,
		token,
		now,
		legacyProjectionMigrationLease,
	)
	if err != nil {
		return report, fmt.Errorf("claim legacy projection migration: %w", err)
	}
	if !ok || claimed == nil {
		return report, nil
	}
	state = claimed

	after := legacyProjectionCursor(state.CursorUpdatedAt, state.CursorID)
	through := repository.LegacyProjectionCursor{UpdatedAt: *state.HighWaterUpdatedAt, ID: state.HighWaterID}
	tasks, err := s.repo.ListLegacyTerminalWithOutputAfter(after, through, state.BatchSize)
	if err != nil {
		return s.failLegacyProjectionBatch(state, token, now, fmt.Errorf("list legacy migration batch: %w", err))
	}

	delta := repository.LegacyProjectionBatchDelta{ScannedRows: int64(len(tasks))}
	for index := range tasks {
		item, importErr := s.importLegacyProjectionTask(&tasks[index], now)
		if importErr != nil {
			return s.failLegacyProjectionBatch(state, token, now, importErr)
		}
		delta.ImportedJobs += int64(item.JobsImported)
		delta.ArtifactsQueued += int64(item.ArtifactsQueued)
		delta.ArtifactsBlocked += int64(item.ArtifactsBlocked)
		delta.MissingPaths += int64(item.MissingPaths)
	}

	cursor := through
	if len(tasks) > 0 {
		last := tasks[len(tasks)-1]
		cursor = repository.LegacyProjectionCursor{UpdatedAt: last.UpdatedAt, ID: last.ID}
	}
	completed := len(tasks) == 0 || !repository.LegacyProjectionCursorAfter(through, cursor)
	stored, updated, err := s.executionRepo.CompleteLegacyProjectionMigrationBatch(
		state.Source,
		token,
		cursor,
		delta,
		completed,
		now,
		legacyProjectionSourceRetirementWindow,
	)
	if err != nil {
		return report, fmt.Errorf("complete legacy migration batch: %w", err)
	}
	if !updated || stored == nil {
		return report, fmt.Errorf("legacy migration Lease was lost before batch completion")
	}
	report = legacyProjectionReportFromState(stored)
	report.TasksFound = len(tasks)
	report.JobsImported = int(delta.ImportedJobs)
	report.ArtifactsQueued = int(delta.ArtifactsQueued)
	report.ArtifactsBlocked = int(delta.ArtifactsBlocked)
	report.MissingPaths = int(delta.MissingPaths)
	s.broadcastLegacyProjectionMigration(stored)
	return report, nil
}

func (s *ArtifactMaintenanceService) failLegacyProjectionBatch(state *model.LegacyTranscodeProjectionMigrationState, token string, now time.Time, cause error) (legacyProjectionInventoryReport, error) {
	if state == nil {
		return legacyProjectionInventoryReport{}, cause
	}
	backoff := legacyProjectionRetryBackoff(state.FailureCount + 1)
	stored, _, persistErr := s.executionRepo.FailLegacyProjectionMigrationBatch(
		state.Source,
		token,
		"legacy_projection_batch_failed",
		cause.Error(),
		now.Add(backoff),
		now,
	)
	if persistErr != nil {
		return legacyProjectionReportFromState(state), fmt.Errorf("%v; persist migration failure: %w", cause, persistErr)
	}
	if stored != nil {
		s.broadcastLegacyProjectionMigration(stored)
	}
	return legacyProjectionReportFromState(stored), cause
}

func legacyProjectionRetryBackoff(failureCount int) time.Duration {
	if failureCount < 1 {
		failureCount = 1
	}
	backoff := 30 * time.Second
	for i := 1; i < failureCount && backoff < 30*time.Minute; i++ {
		backoff *= 2
	}
	if backoff > 30*time.Minute {
		return 30 * time.Minute
	}
	return backoff
}

func legacyProjectionCursor(updatedAt *time.Time, id string) *repository.LegacyProjectionCursor {
	if updatedAt == nil {
		return nil
	}
	return &repository.LegacyProjectionCursor{UpdatedAt: *updatedAt, ID: id}
}

func legacyProjectionReportFromState(state *model.LegacyTranscodeProjectionMigrationState) legacyProjectionInventoryReport {
	if state == nil {
		return legacyProjectionInventoryReport{}
	}
	return legacyProjectionInventoryReport{
		Generation:  state.Generation,
		ScannedRows: state.ScannedRows,
		TargetRows:  state.TargetRows,
		Status:      state.Status,
		HasMore:     state.Status == repository.LegacyProjectionMigrationPending || state.Status == repository.LegacyProjectionMigrationRunning,
	}
}

func (s *ArtifactMaintenanceService) broadcastLegacyProjectionMigration(state *model.LegacyTranscodeProjectionMigrationState) {
	if s == nil || s.wsHub == nil || state == nil {
		return
	}
	s.wsHub.BroadcastEvent(EventTaskUpdated, map[string]any{
		"kind":         TaskKindLegacyProjectionMigration,
		"status":       state.Status,
		"source_id":    state.Source,
		"generation":   state.Generation,
		"scanned_rows": state.ScannedRows,
		"target_rows":  state.TargetRows,
	})
}

func (s *ArtifactMaintenanceService) importLegacyProjectionTask(task *model.TranscodeTask, now time.Time) (legacyProjectionInventoryReport, error) {
	report := legacyProjectionInventoryReport{TasksFound: 1}
	if task == nil {
		return report, nil
	}
	root := filepath.Join(s.cfg.Cache.CacheDir, "transcode")
	db := s.repo.DB()
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
		return report, nil
	}

	rollbackUntil := now.Add(legacyProjectionRollbackWindow)
	outputDir := filepath.Clean(strings.TrimSpace(task.OutputDir))
	artifact := &model.TranscodeArtifactRecord{
		ID: artifactID, JobID: job.ID, MediaID: task.MediaID,
		Kind: repository.LegacyTranscodeArtifactKind, ProfileID: task.Quality,
		Path: outputDir, Status: "expired",
		MigrationSource:      repository.LegacyTranscodeArtifactMigrationSource,
		CleanupState:         repository.ArtifactCleanupPending,
		CleanupNextAttemptAt: &rollbackUntil,
		CleanupRollbackUntil: &rollbackUntil,
		CreatedAt:            task.CreatedAt, UpdatedAt: now,
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
		ID: deterministicLegacyProjectionID("job", task.ID), LegacyTaskID: &legacyID,
		MediaID: task.MediaID, Intent: "legacy_history_import", ProfileID: task.Quality,
		AudioTrack: -1, Status: status, DesiredState: "cancelled",
		CompletedAt: completedAt, CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt,
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
