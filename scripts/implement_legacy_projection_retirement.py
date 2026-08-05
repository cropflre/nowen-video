#!/usr/bin/env python3
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def write(path: str, content: str) -> None:
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")


def replace_once(path: str, old: str, new: str) -> None:
    content = read(path)
    if content.count(old) != 1:
        raise RuntimeError(f"{path}: expected one occurrence of {old!r}, found {content.count(old)}")
    write(path, content.replace(old, new, 1))


def regex_once(path: str, pattern: str, replacement: str) -> None:
    content = read(path)
    updated, count = re.subn(pattern, replacement, content, count=1, flags=re.S)
    if count != 1:
        raise RuntimeError(f"{path}: regex did not match exactly once: {pattern}")
    write(path, updated)


# ---------------------------------------------------------------------------
# Freeze the legacy transcode_tasks projection: fresh databases stop creating
# it, while existing databases retain the table for read-only migration/history.
# ---------------------------------------------------------------------------
for path in ("internal/model/model.go", "internal/model/migrate_lite.go"):
    content = read(path)
    marker = "\t\t&TranscodeTask{},\n"
    if content.count(marker) != 1:
        raise RuntimeError(f"{path}: TranscodeTask migration marker mismatch")
    write(path, content.replace(marker, "", 1))

repo_stats = read("internal/repository/repo_stats.go")
start = repo_stats.index("// ==================== TranscodeRepo ====================")
end = repo_stats.index("// ==================== PlaybackStatsRepo ====================")
transcode_repo = '''// ==================== TranscodeRepo ====================

// TranscodeRepo is now only the read-only migration gateway for an existing
// transcode_tasks table. New databases never create that table and no runtime
// component may write, update or delete its rows.
type TranscodeRepo struct {
\tdb *gorm.DB
}

func (r *TranscodeRepo) DB() *gorm.DB {
\tif r == nil {
\t\treturn nil
\t}
\treturn r.db
}

func (r *TranscodeRepo) LegacyTableExists() bool {
\treturn r != nil && r.db != nil && r.db.Migrator().HasTable(&model.TranscodeTask{})
}

// ListLegacyTerminalWithOutput returns a bounded inventory source. It never
// mutates the legacy projection and is a no-op for fresh databases.
func (r *TranscodeRepo) ListLegacyTerminalWithOutput(limit int) ([]model.TranscodeTask, error) {
\tif !r.LegacyTableExists() {
\t\treturn []model.TranscodeTask{}, nil
\t}
\tif limit <= 0 {
\t\tlimit = 500
\t}
\tif limit > 2000 {
\t\tlimit = 2000
\t}
\tvar tasks []model.TranscodeTask
\terr := r.db.Where(
\t\t"status IN ? AND TRIM(COALESCE(output_dir, '')) <> ''",
\t\t[]string{"done", "completed", "failed", "cancelled"},
\t).
\t\tOrder("updated_at ASC, id ASC").
\t\tLimit(limit).
\t\tFind(&tasks).Error
\treturn tasks, err
}

'''
write("internal/repository/repo_stats.go", repo_stats[:start] + transcode_repo + repo_stats[end:])

# Remove the retired durable queue's dependency on TranscodeTask payloads.
regex_once(
    "internal/repository/repo_transcode_queue.go",
    r'\n// LoadJobPayload reconstructs.*?\n}\n\n// CompleteUnleasedJob',
    '\n// CompleteUnleasedJob',
)
replace_once(
    "internal/repository/repo_transcode_queue.go",
    'import (\n\t"fmt"\n\t"strings"\n\t"time"\n',
    'import (\n\t"time"\n',
)

# ---------------------------------------------------------------------------
# Artifact cleanup becomes auditable: files are reclaimed, but the Artifact row
# becomes a completed tombstone with original paths and disposition evidence.
# ---------------------------------------------------------------------------
replace_once(
    "internal/model/transcode_execution.go",
    '\tCleanupErrorMessage   string     `json:"cleanup_error_message" gorm:"type:text"`\n\tCreatedAt             time.Time  `json:"created_at" gorm:"index"`',
    '\tCleanupErrorMessage   string     `json:"cleanup_error_message" gorm:"type:text"`\n'
    '\tCleanupCompletedAt    *time.Time `json:"cleanup_completed_at" gorm:"index"`\n'
    '\tCleanupDisposition    string     `json:"cleanup_disposition" gorm:"index;type:text"`\n'
    '\tCleanupOriginalPath   string     `json:"cleanup_original_path" gorm:"type:text"`\n'
    '\tCleanupOriginalTempPath string   `json:"cleanup_original_temp_path" gorm:"type:text"`\n'
    '\tCleanupOriginalManifestPath string `json:"cleanup_original_manifest_path" gorm:"type:text"`\n'
    '\tCleanupRollbackUntil  *time.Time `json:"cleanup_rollback_until" gorm:"index"`\n'
    '\tCreatedAt             time.Time  `json:"created_at" gorm:"index"`',
)

replace_once(
    "internal/repository/repo_transcode_artifact_cleanup.go",
    'const (\n\tArtifactCleanupPending   = "pending"\n\tArtifactCleanupClaimed   = "claimed"\n\tArtifactCleanupRetryWait = "retry_wait"\n\tArtifactCleanupBlocked   = "blocked"\n)',
    'const (\n'
    '\tArtifactCleanupPending           = "pending"\n'
    '\tArtifactCleanupClaimed           = "claimed"\n'
    '\tArtifactCleanupRetryWait         = "retry_wait"\n'
    '\tArtifactCleanupBlocked           = "blocked"\n'
    '\tArtifactCleanupCompleted         = "completed"\n'
    '\tArtifactCleanupRollbackCompleted = "rollback_completed"\n'
    '\tLegacyTranscodeArtifactMigrationSource = "legacy_transcode_task_v1"\n'
    '\tLegacyTranscodeArtifactKind = "legacy_hls_directory"\n'
    ')',
)

replacement_cleanup_methods = r'''// CompleteArtifactCleanupByClaim preserves a durable tombstone after the
// filesystem has been reclaimed. Runtime History therefore keeps the original
// path, byte count, completion time and disposition without treating the file
// as live storage.
func (r *TranscodeExecutionRepo) CompleteArtifactCleanupByClaim(
	artifactID,
	token,
	disposition string,
	now time.Time,
) (bool, error) {
	completed := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var artifact model.TranscodeArtifactRecord
		result := tx.Where(
			"id = ? AND cleanup_state = ? AND cleanup_token = ?",
			artifactID,
			ArtifactCleanupClaimed,
			token,
		).Limit(1).Find(&artifact)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		if err := tx.Where(
			"startup_artifact_id = ? OR continuation_artifact_id = ?",
			artifactID,
			artifactID,
		).Delete(&model.TranscodeHandoffAttestationRecord{}).Error; err != nil {
			return err
		}
		result = tx.Model(&model.TranscodeArtifactRecord{}).
			Where(
				"id = ? AND cleanup_state = ? AND cleanup_token = ?",
				artifactID,
				ArtifactCleanupClaimed,
				token,
			).
			Updates(map[string]any{
				"status":                         "deleted",
				"cleanup_state":                  ArtifactCleanupCompleted,
				"cleanup_completed_at":           now,
				"cleanup_disposition":            disposition,
				"cleanup_original_path":          artifact.Path,
				"cleanup_original_temp_path":     artifact.TempPath,
				"cleanup_original_manifest_path": artifact.ManifestPath,
				"path":                           "",
				"temp_path":                      "",
				"manifest_path":                  "",
				"cleanup_token":                  "",
				"cleanup_claimed_at":             nil,
				"cleanup_lease_expires_at":       nil,
				"cleanup_next_attempt_at":        nil,
				"cleanup_error_code":             "",
				"cleanup_error_message":          "",
				"updated_at":                     now,
			})
		if result.Error != nil {
			return result.Error
		}
		completed = result.RowsAffected == 1
		return nil
	})
	return completed, err
}

// RollbackLegacyArtifactCleanup removes a migrated legacy directory from the
// cleanup work set without changing or deleting the directory itself. Claimed
// or completed cleanup cannot be rolled back.
func (r *TranscodeExecutionRepo) RollbackLegacyArtifactCleanup(artifactID string, now time.Time) (bool, error) {
	result := r.db.Model(&model.TranscodeArtifactRecord{}).
		Where(
			"id = ? AND migration_source = ? AND cleanup_state IN ?",
			artifactID,
			LegacyTranscodeArtifactMigrationSource,
			[]string{ArtifactCleanupPending, ArtifactCleanupRetryWait, ArtifactCleanupBlocked},
		).
		Updates(map[string]any{
			"status":                    "migration_rolled_back",
			"cleanup_state":             ArtifactCleanupRollbackCompleted,
			"cleanup_completed_at":      now,
			"cleanup_disposition":       "rollback_preserved",
			"cleanup_token":             "",
			"cleanup_claimed_at":        nil,
			"cleanup_lease_expires_at":  nil,
			"cleanup_next_attempt_at":   nil,
			"cleanup_error_code":        "",
			"cleanup_error_message":     "",
			"cleanup_rollback_until":    nil,
			"updated_at":                now,
		})
	return result.RowsAffected == 1, result.Error
}

'''
regex_once(
    "internal/repository/repo_transcode_artifact_cleanup.go",
    r'// DeleteArtifactByCleanupClaim.*?\nfunc \(r \*TranscodeExecutionRepo\) ArtifactCleanupStateCounts',
    replacement_cleanup_methods + 'func (r *TranscodeExecutionRepo) ArtifactCleanupStateCounts',
)

# Successful cleanup now completes metadata rather than deleting it.
replace_once(
    "internal/service/transcode_cleanup_state.go",
    'deleted, err := s.executionRepo.DeleteArtifactByCleanupClaim(artifact.ID, token)',
    'deleted, err := s.executionRepo.CompleteArtifactCleanupByClaim(\n'
    '\t\tartifact.ID,\n'
    '\t\ttoken,\n'
    '\t\tartifactCleanupDisposition(artifact),\n'
    '\t\tnow,\n'
    '\t)',
)
replace_once(
    "internal/service/transcode_cleanup_state.go",
    'fmt.Errorf("delete artifact cleanup metadata: %w", err)',
    'fmt.Errorf("complete artifact cleanup metadata: %w", err)',
)
replace_once(
    "internal/service/transcode_cleanup_state.go",
    'return removedDirs, false, fmt.Errorf("artifact cleanup ownership lost: %s", artifact.ID)',
    'return removedDirs, false, fmt.Errorf("artifact cleanup completion ownership lost: %s", artifact.ID)',
)
replace_once(
    "internal/service/transcode_cleanup_state.go",
    'func (s *ArtifactMaintenanceService) persistArtifactCleanupFailure(',
    '''func artifactCleanupDisposition(artifact *model.TranscodeArtifactRecord) string {
	if artifact != nil && artifact.MigrationSource == repository.LegacyTranscodeArtifactMigrationSource {
		return "legacy_projection_reclaimed"
	}
	return "retention_reclaimed"
}

func (s *ArtifactMaintenanceService) persistArtifactCleanupFailure(''',
)

# Old task retention is no longer a cleanup source; only Artifact rows are.
regex_once(
    "internal/service/transcode_cleanup.go",
    r'func \(s \*ArtifactMaintenanceService\) CleanupStaleCache.*?\nfunc \(s \*ArtifactMaintenanceService\) cleanupArtifactRecord',
    '''func (s *ArtifactMaintenanceService) CleanupStaleCache(doneRetainDays, failedRetainDays int) (int, int, error) {
	_ = doneRetainDays
	if failedRetainDays <= 0 {
		failedRetainDays = 7
	}
	now := time.Now()
	terminalCutoff := now.AddDate(0, 0, -failedRetainDays)
	dirsCleaned, recordsCompleted, err := s.cleanupTerminalArtifactBatch(terminalCutoff, now)
	if err != nil {
		return dirsCleaned, recordsCompleted, fmt.Errorf("清理过期终态 Artifact 失败: %w", err)
	}
	if dirsCleaned > 0 {
		s.InvalidateCacheDiskUsage()
	}
	s.logger.Infof("Artifact 缓存清理完成: 删除 %d 个目录, 完成 %d 条清理墓碑", dirsCleaned, recordsCompleted)
	return dirsCleaned, recordsCompleted, nil
}

func (s *ArtifactMaintenanceService) cleanupArtifactRecord''',
)
replace_once("internal/service/transcode_cleanup.go", '\t"os"\n', '')
replace_once(
    "internal/service/transcode_cleanup.go",
    'return 0, fmt.Errorf("artifact cleanup did not delete record: %s", artifact.ID)',
    'return 0, fmt.Errorf("artifact cleanup did not complete tombstone: %s", artifact.ID)',
)

# ---------------------------------------------------------------------------
# Legacy output directory inventory: deterministic Job/Artifact records, a
# seven-day rollback window, bounded path validation and idempotent imports.
# ---------------------------------------------------------------------------
write("internal/service/legacy_transcode_projection_migration.go", r'''package service

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
			ID:                    artifactID,
			JobID:                 job.ID,
			MediaID:               task.MediaID,
			Kind:                  repository.LegacyTranscodeArtifactKind,
			ProfileID:             task.Quality,
			Path:                  outputDir,
			Status:                "expired",
			MigrationSource:       repository.LegacyTranscodeArtifactMigrationSource,
			CleanupState:          repository.ArtifactCleanupPending,
			CleanupNextAttemptAt:  &rollbackUntil,
			CleanupRollbackUntil:  &rollbackUntil,
			CreatedAt:             task.CreatedAt,
			UpdatedAt:             now,
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
		ID:               deterministicLegacyProjectionID("job", task.ID),
		LegacyTaskID:     &legacyID,
		MediaID:          task.MediaID,
		Intent:           "legacy_history_import",
		ProfileID:        task.Quality,
		AudioTrack:       -1,
		Status:           status,
		DesiredState:     "cancelled",
		CompletedAt:      completedAt,
		CreatedAt:        task.CreatedAt,
		UpdatedAt:        task.UpdatedAt,
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
''')

# Start-up and periodic inventory precede cleanup so the rollback deadline is
# persisted before any legacy directory can be reclaimed.
replace_once(
    "internal/service/artifact_maintenance.go",
    '\tif report, retireErr := service.retirePersistentRuntimePlayback(time.Now()); retireErr != nil {',
    '\tif report, inventoryErr := service.inventoryLegacyTranscodeProjection(time.Now()); inventoryErr != nil {\n'
    '\t\tlogger.Warnf("启动登记 Legacy 转码目录失败: %v", inventoryErr)\n'
    '\t} else if report.Changed() {\n'
    '\t\tlogger.Infof("启动登记 Legacy 转码目录 tasks=%d jobs=%d queued=%d blocked=%d missing=%d", report.TasksFound, report.JobsImported, report.ArtifactsQueued, report.ArtifactsBlocked, report.MissingPaths)\n'
    '\t}\n'
    '\tif report, retireErr := service.retirePersistentRuntimePlayback(time.Now()); retireErr != nil {',
)
replace_once(
    "internal/service/artifact_maintenance.go",
    '\t\tcase now := <-ticker.C:\n\t\t\ts.runStorageHealthTick(now, true)',
    '\t\tcase now := <-ticker.C:\n'
    '\t\t\ts.runStorageHealthTick(now, true)\n'
    '\t\t\tif report, err := s.inventoryLegacyTranscodeProjection(now); err != nil {\n'
    '\t\t\t\ts.logger.Warnf("周期登记 Legacy 转码目录失败: %v", err)\n'
    '\t\t\t} else if report.Changed() {\n'
    '\t\t\t\ts.logger.Infof("周期登记 Legacy 转码目录 tasks=%d jobs=%d queued=%d blocked=%d missing=%d", report.TasksFound, report.JobsImported, report.ArtifactsQueued, report.ArtifactsBlocked, report.MissingPaths)\n'
    '\t\t\t}',
)

# Admin retry and rollback remain state transitions through the repository.
append_admin = r'''

var ErrLegacyArtifactRollbackUnavailable = errors.New("legacy artifact rollback is unavailable")

func (s *ArtifactMaintenanceService) RollbackLegacyArtifactMigration(artifactID string) error {
	if s == nil || s.executionRepo == nil {
		return fmt.Errorf("Legacy Artifact 迁移服务不可用")
	}
	rolledBack, err := s.executionRepo.RollbackLegacyArtifactCleanup(artifactID, time.Now())
	if err != nil {
		return fmt.Errorf("回滚 Legacy Artifact 清理失败: %w", err)
	}
	if !rolledBack {
		return fmt.Errorf("%w: artifact=%s", ErrLegacyArtifactRollbackUnavailable, artifactID)
	}
	return nil
}
'''
write("internal/service/transcode_cleanup_admin.go", read("internal/service/transcode_cleanup_admin.go") + append_admin)
replace_once(
    "internal/service/transcode_cleanup_admin.go",
    '// The row disappeared because another cleanup owner completed the same\n\t\t// operation after requeue. This is an idempotent success.',
    '// The operation is no longer active because another cleanup owner completed\n\t\t// the tombstone after requeue. This is an idempotent success.',
)

# ---------------------------------------------------------------------------
# Task Center type boundary no longer contains legacy transcode execution.
# Legacy directory migration is an explicit task kind with rollback.
# ---------------------------------------------------------------------------
task_center = read("internal/service/task_center.go")
task_center = task_center.replace('\tTaskKindTranscode       = "transcode"\n', '\tTaskKindLegacyArtifactMigration = "legacy_artifact_migration"\n', 1)
task_center = task_center.replace('\ttranscodeRepo *repository.TranscodeRepo\n', '', 1)
regex = r'func NewTaskCenterService\(.*?\n}\n\nfunc \(s \*TaskCenterService\) Snapshot'
new_ctor = '''func NewTaskCenterService(
	library *LibraryService,
	scrapeRepo *repository.ScrapeTaskRepo,
	executionRepo *repository.TranscodeExecutionRepo,
	logger *zap.SugaredLogger,
) *TaskCenterService {
	return &TaskCenterService{
		library:       library,
		scrapeRepo:    scrapeRepo,
		executionRepo: executionRepo,
		logger:        logger,
	}
}

func (s *TaskCenterService) Snapshot'''
task_center, count = re.subn(regex, new_ctor, task_center, count=1, flags=re.S)
if count != 1:
    raise RuntimeError("task_center constructor replacement failed")
task_center, count = re.subn(r'\n\tif s\.transcodeRepo != nil \{.*?\n\t}\n\n\tif s\.scrapeRepo', '\n\tif s.scrapeRepo', task_center, count=1, flags=re.S)
if count != 1:
    raise RuntimeError("task_center transcode listing removal failed")
task_center, count = re.subn(r'\nfunc transcodeToUnifiedTask\(.*?\n}\n\nfunc scrapeToUnifiedTask', '\nfunc scrapeToUnifiedTask', task_center, count=1, flags=re.S)
if count != 1:
    raise RuntimeError("task_center transcode mapping removal failed")
artifact_mapping = r'''func artifactCleanupToUnifiedTask(artifact *model.TranscodeArtifactRecord) UnifiedTask {
	if artifact == nil {
		return UnifiedTask{}
	}
	kind := TaskKindArtifactCleanup
	migrationTask := artifact.MigrationSource == repository.LegacyTranscodeArtifactMigrationSource
	if migrationTask {
		kind = TaskKindLegacyArtifactMigration
	}
	status := TaskStatusQueued
	switch artifact.CleanupState {
	case repository.ArtifactCleanupClaimed:
		status = TaskStatusRunning
	case repository.ArtifactCleanupRetryWait, repository.ArtifactCleanupBlocked:
		status = TaskStatusFailed
	}

	title := "转码缓存清理"
	if migrationTask {
		title = "旧转码目录迁移"
	}
	if artifact.MediaID != "" {
		title += " · " + artifact.MediaID
	}
	subtitleParts := make([]string, 0, 4)
	if artifact.ProfileID != "" {
		subtitleParts = append(subtitleParts, artifact.ProfileID)
	}
	subtitleParts = append(subtitleParts, cleanupStateLabel(artifact.CleanupState))
	if migrationTask && artifact.CleanupRollbackUntil != nil {
		subtitleParts = append(subtitleParts, "可保留回滚至 "+artifact.CleanupRollbackUntil.Format("01-02 15:04"))
	}
	if artifact.CleanupAttempts > 0 {
		subtitleParts = append(subtitleParts, fmt.Sprintf("第 %d 次尝试", artifact.CleanupAttempts))
	}

	messageParts := make([]string, 0, 4)
	if artifact.CleanupErrorCode != "" {
		messageParts = append(messageParts, artifact.CleanupErrorCode)
	}
	if artifact.CleanupErrorMessage != "" {
		messageParts = append(messageParts, artifact.CleanupErrorMessage)
	}
	if artifact.CleanupState == repository.ArtifactCleanupRetryWait && artifact.CleanupNextAttemptAt != nil {
		messageParts = append(messageParts, "下次重试 "+artifact.CleanupNextAttemptAt.Format("01-02 15:04"))
	}
	if artifact.Path != "" {
		messageParts = append(messageParts, artifact.Path)
	} else if artifact.TempPath != "" {
		messageParts = append(messageParts, artifact.TempPath)
	}
	if len(messageParts) == 0 {
		if migrationTask {
			messageParts = append(messageParts, "观察期结束后进入 Cleanup Lease；回滚仅保留目录，不恢复旧执行器")
		} else {
			messageParts = append(messageParts, "等待 Artifact 清理 Worker")
		}
	}

	return UnifiedTask{
		ID:        kind + ":" + artifact.ID,
		Kind:      kind,
		Status:    status,
		Title:     title,
		Subtitle:  strings.Join(subtitleParts, " · "),
		Message:   strings.Join(messageParts, " · "),
		Progress:  0,
		SourceID:  artifact.ID,
		CreatedAt: timePtr(artifact.CreatedAt),
		UpdatedAt: timePtr(artifact.UpdatedAt),
		StartedAt: artifact.CleanupClaimedAt,
	}
}

'''
task_center, count = re.subn(r'func artifactCleanupToUnifiedTask\(.*?\nfunc cleanupStateLabel', artifact_mapping + 'func cleanupStateLabel', task_center, count=1, flags=re.S)
if count != 1:
    raise RuntimeError("task_center artifact mapping replacement failed")
write("internal/service/task_center.go", task_center)

write("internal/service/task_actions.go", r'''package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

const (
	TaskActionRetry    = "retry"
	TaskActionRollback = "rollback"
	EventTaskUpdated   = "task_updated"
)

var (
	ErrTaskActionConflict    = errors.New("task action conflicts with current status")
	ErrTaskActionUnsupported = errors.New("task action unsupported")
)

type TaskActionResult struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	SourceID string `json:"source_id"`
	Action   string `json:"action"`
	Accepted bool   `json:"accepted"`
	Message  string `json:"message"`
}

type scrapeTaskLookup interface {
	FindByID(id string) (*model.ScrapeTask, error)
}

type artifactCleanupLookup interface {
	FindArtifactCleanupOperation(id string) (*model.TranscodeArtifactRecord, error)
}

type artifactCleanupActions interface {
	RetryArtifactCleanup(artifactID string) error
	RollbackLegacyArtifactMigration(artifactID string) error
}

type scrapeTaskActions interface {
	StartScrape(taskID, userID string) error
}

type TaskActionDispatcher struct {
	artifactCleanup artifactCleanupActions
	scrape          scrapeTaskActions
	artifactLookup  artifactCleanupLookup
	scrapeLookup    scrapeTaskLookup
	wsHub           *WSHub
	logger          *zap.SugaredLogger
}

func NewTaskActionDispatcher(
	maintenance *ArtifactMaintenanceService,
	scrape *ScrapeManagerService,
	scrapeRepo *repository.ScrapeTaskRepo,
	wsHub *WSHub,
	logger *zap.SugaredLogger,
) *TaskActionDispatcher {
	dispatcher := &TaskActionDispatcher{
		scrape:       scrape,
		scrapeLookup: scrapeRepo,
		wsHub:        wsHub,
		logger:       logger,
	}
	if maintenance != nil {
		dispatcher.artifactCleanup = maintenance
		dispatcher.artifactLookup = maintenance.executionRepo
	}
	return dispatcher
}

func AvailableTaskActions(kind, status string) []string {
	normalizedKind := strings.ToLower(strings.TrimSpace(kind))
	normalizedStatus := normalizeTaskStatus(status)
	switch normalizedKind {
	case TaskKindScrape:
		if normalizedStatus == TaskStatusFailed || normalizedStatus == TaskStatusCancelled {
			return []string{TaskActionRetry}
		}
	case TaskKindArtifactCleanup:
		if normalizedStatus == TaskStatusFailed {
			return []string{TaskActionRetry}
		}
	case TaskKindLegacyArtifactMigration:
		switch normalizedStatus {
		case TaskStatusQueued:
			return []string{TaskActionRollback}
		case TaskStatusFailed:
			return []string{TaskActionRetry, TaskActionRollback}
		}
	}
	return []string{}
}

func (d *TaskActionDispatcher) Execute(kind, sourceID, action, userID string) (*TaskActionResult, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	sourceID = strings.TrimSpace(sourceID)
	action = strings.ToLower(strings.TrimSpace(action))
	if sourceID == "" {
		return nil, fmt.Errorf("%w: empty source id", ErrTaskNotFound)
	}
	var err error
	switch kind {
	case TaskKindScrape:
		err = d.executeScrape(sourceID, action, userID)
	case TaskKindArtifactCleanup:
		err = d.executeArtifactCleanup(sourceID, action, false)
	case TaskKindLegacyArtifactMigration:
		err = d.executeArtifactCleanup(sourceID, action, true)
	case TaskKindScan, TaskKindStorageIncident:
		err = fmt.Errorf("%w: task kind %s exposes no lifecycle controls", ErrTaskActionUnsupported, kind)
	default:
		err = fmt.Errorf("%w: unknown task kind %q", ErrTaskActionUnsupported, kind)
	}
	if err != nil {
		return nil, err
	}
	result := &TaskActionResult{
		ID: kind + ":" + sourceID, Kind: kind, SourceID: sourceID,
		Action: action, Accepted: true, Message: taskActionMessage(kind, action),
	}
	if d.wsHub != nil {
		d.wsHub.BroadcastEvent(EventTaskUpdated, result)
	}
	if d.logger != nil {
		d.logger.Infof("统一任务操作已受理 kind=%s source_id=%s action=%s actor=%s", kind, sourceID, action, userID)
	}
	return result, nil
}

func (d *TaskActionDispatcher) executeArtifactCleanup(sourceID, action string, legacyMigration bool) error {
	if d.artifactLookup == nil || d.artifactCleanup == nil {
		return fmt.Errorf("Artifact 清理执行器不可用")
	}
	artifact, err := d.artifactLookup.FindArtifactCleanupOperation(sourceID)
	if err != nil || artifact == nil {
		return fmt.Errorf("%w: artifact cleanup %s", ErrTaskNotFound, sourceID)
	}
	isLegacy := artifact.MigrationSource == repository.LegacyTranscodeArtifactMigrationSource
	if legacyMigration != isLegacy {
		return fmt.Errorf("%w: artifact migration kind mismatch", ErrTaskActionConflict)
	}
	if !containsAction(AvailableTaskActions(mapArtifactTaskKind(artifact), mapArtifactTaskStatus(artifact)), action) {
		return fmt.Errorf("%w: artifact cleanup state=%s action=%s", ErrTaskActionConflict, artifact.CleanupState, action)
	}
	switch action {
	case TaskActionRetry:
		if err := d.artifactCleanup.RetryArtifactCleanup(sourceID); err != nil {
			if errors.Is(err, ErrArtifactCleanupNotRetryable) {
				return fmt.Errorf("%w: artifact cleanup state changed", ErrTaskActionConflict)
			}
			return fmt.Errorf("重试 Artifact 清理失败: %w", err)
		}
	case TaskActionRollback:
		if err := d.artifactCleanup.RollbackLegacyArtifactMigration(sourceID); err != nil {
			if errors.Is(err, ErrLegacyArtifactRollbackUnavailable) {
				return fmt.Errorf("%w: legacy artifact cleanup already claimed or completed", ErrTaskActionConflict)
			}
			return fmt.Errorf("保留回滚 Legacy Artifact 失败: %w", err)
		}
	default:
		return fmt.Errorf("%w: artifact cleanup action=%s", ErrTaskActionUnsupported, action)
	}
	return nil
}

func mapArtifactTaskKind(artifact *model.TranscodeArtifactRecord) string {
	if artifact != nil && artifact.MigrationSource == repository.LegacyTranscodeArtifactMigrationSource {
		return TaskKindLegacyArtifactMigration
	}
	return TaskKindArtifactCleanup
}

func mapArtifactTaskStatus(artifact *model.TranscodeArtifactRecord) string {
	if artifact == nil {
		return TaskStatusFailed
	}
	switch artifact.CleanupState {
	case repository.ArtifactCleanupPending:
		return TaskStatusQueued
	case repository.ArtifactCleanupClaimed:
		return TaskStatusRunning
	default:
		return TaskStatusFailed
	}
}

func (d *TaskActionDispatcher) executeScrape(sourceID, action, userID string) error {
	if d.scrapeLookup == nil || d.scrape == nil {
		return fmt.Errorf("刮削任务执行器不可用")
	}
	task, err := d.scrapeLookup.FindByID(sourceID)
	if err != nil || task == nil {
		return fmt.Errorf("%w: scrape %s", ErrTaskNotFound, sourceID)
	}
	if !containsAction(AvailableTaskActions(TaskKindScrape, task.Status), action) {
		if action == TaskActionRetry {
			return fmt.Errorf("%w: scrape status=%s action=%s", ErrTaskActionConflict, task.Status, action)
		}
		return fmt.Errorf("%w: scrape action=%s", ErrTaskActionUnsupported, action)
	}
	if err := d.scrape.StartScrape(sourceID, userID); err != nil {
		return fmt.Errorf("重试刮削失败: %w", err)
	}
	return nil
}

func containsAction(actions []string, action string) bool {
	for _, candidate := range actions {
		if candidate == action {
			return true
		}
	}
	return false
}

func taskActionMessage(kind, action string) string {
	switch action {
	case TaskActionRetry:
		if kind == TaskKindScrape {
			return "刮削任务已重新提交"
		}
		return "Artifact 清理已重新执行"
	case TaskActionRollback:
		return "Legacy 目录已退出清理队列并保留"
	default:
		return "任务操作已提交"
	}
}
''')

# Remove compatibility constructors and their source-contract test.
for path in (
    "internal/service/task_center_runtime_retirement.go",
    "internal/service/task_center_runtime_retirement_test.go",
):
    target = ROOT / path
    if target.exists():
        target.unlink()

# Lite assembly constructs only the modern task types.
replace_once(
    "cmd/server-lite/router.go",
    '''\ttaskCenterService := service.NewTaskCenterServiceWithoutRuntimeTranscode(
\t\tservices.Library,
\t\trepos.Transcode,
\t\trepos.ScrapeTask,
\t\tlogger,
\t)
\ttaskActionDispatcher := service.NewTaskActionDispatcherWithoutRuntimeTranscode(
\t\tservices.ArtifactMaintenance,
\t\tservices.ScrapeManager,
\t\trepos.Transcode,
\t\trepos.ScrapeTask,
\t\trepos.Media,
\t\tservices.WSHub,
\t\tlogger,
\t)''',
    '''\texecutionRepo := repository.NewTranscodeExecutionRepo(repos.DB())
\ttaskCenterService := service.NewTaskCenterService(
\t\tservices.Library,
\t\trepos.ScrapeTask,
\t\texecutionRepo,
\t\tlogger,
\t)
\ttaskActionDispatcher := service.NewTaskActionDispatcher(
\t\tservices.ArtifactMaintenance,
\t\tservices.ScrapeManager,
\t\trepos.ScrapeTask,
\t\tservices.WSHub,
\t\tlogger,
\t)''',
)

# Web understands the migration kind and rollback action.
replace_once(
    "web/src/api/tasks.ts",
    "export type UnifiedTaskKind = 'scan' | 'scrape' | 'transcode' | 'artifact_cleanup' | 'storage_incident'",
    "export type UnifiedTaskKind = 'scan' | 'scrape' | 'artifact_cleanup' | 'legacy_artifact_migration' | 'storage_incident'",
)
replace_once(
    "web/src/api/tasks.ts",
    "export type UnifiedTaskAction = 'cancel' | 'retry'",
    "export type UnifiedTaskAction = 'retry' | 'rollback'",
)
replace_once(
    "web/src/hooks/useWebSocket.ts",
    "  action: 'cancel' | 'retry'",
    "  action: 'retry' | 'rollback'",
)
replace_once(
    "web/src/components/TaskCenter.tsx",
    "import { Activity, Ban, CheckCircle2, CircleAlert, Clock3, Database, Film, HardDrive, Loader2, RefreshCw, RotateCcw, X, XCircle } from 'lucide-react'",
    "import { Activity, CheckCircle2, CircleAlert, Clock3, Database, HardDrive, Loader2, RefreshCw, RotateCcw, ShieldCheck, X, XCircle } from 'lucide-react'",
)
replace_once(
    "web/src/components/TaskCenter.tsx",
    "  transcode: '视频转码',\n  artifact_cleanup: '转码缓存清理',",
    "  artifact_cleanup: '转码缓存清理',\n  legacy_artifact_migration: '旧转码目录迁移',",
)
replace_once(
    "web/src/components/TaskCenter.tsx",
    "  if (kind === 'artifact_cleanup' && status === 'failed') return <CircleAlert size={17} />",
    "  if ((kind === 'artifact_cleanup' || kind === 'legacy_artifact_migration') && status === 'failed') return <CircleAlert size={17} />",
)
replace_once(
    "web/src/components/TaskCenter.tsx",
    "  if (kind === 'artifact_cleanup') return <HardDrive size={17} />\n  if (kind === 'scan') return <Database size={17} />\n  if (kind === 'transcode') return <Film size={17} />",
    "  if (kind === 'artifact_cleanup' || kind === 'legacy_artifact_migration') return <HardDrive size={17} />\n  if (kind === 'scan') return <Database size={17} />",
)
replace_once(
    "web/src/components/TaskCenter.tsx",
    "  const cleanupTask = task.kind === 'artifact_cleanup'",
    "  const cleanupTask = task.kind === 'artifact_cleanup' || task.kind === 'legacy_artifact_migration'\n  const migrationTask = task.kind === 'legacy_artifact_migration'",
)
replace_once(
    "web/src/components/TaskCenter.tsx",
    "          {cleanupTask && task.status === 'failed' && (",
    "          {cleanupTask && task.status === 'failed' && !migrationTask && (",
)
rollback_button = '''              {actions.includes('rollback') && (
                <button
                  type="button"
                  onClick={() => onAction(task, 'rollback')}
                  disabled={actionLoading !== null}
                  className="inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs font-medium disabled:opacity-50"
                  style={{ borderColor: 'rgba(202,138,4,.3)', color: '#CA8A04' }}
                >
                  {actionLoading === `${task.id}:rollback` ? <Loader2 size={13} className="animate-spin" /> : <ShieldCheck size={13} />}
                  保留目录
                </button>
              )}
'''
replace_once(
    "web/src/components/TaskCenter.tsx",
    "              {actions.includes('cancel') && (",
    rollback_button + "              {actions.includes('cancel') && (",
)
# cancel is no longer part of the Task Center contract; remove its render block.
content = read("web/src/components/TaskCenter.tsx")
content, count = re.subn(r"\n\s*\{actions\.includes\('cancel'\) && \(.*?\n\s*\)\}", "", content, count=1, flags=re.S)
if count != 1:
    raise RuntimeError("TaskCenter cancel button removal failed")
write("web/src/components/TaskCenter.tsx", content)

# ---------------------------------------------------------------------------
# Runtime History exposes cleanup tombstones and counts only live bytes.
# ---------------------------------------------------------------------------
replace_once(
    "internal/repository/runtime_history.go",
    'if err := r.db.Model(&model.TranscodeArtifactRecord{}).Select("COALESCE(SUM(size_bytes), 0)").Scan(&counts.ArtifactBytes).Error; err != nil {',
    'if err := r.db.Model(&model.TranscodeArtifactRecord{}).\n'
    '\t\tWhere("cleanup_state <> ? OR cleanup_state IS NULL", ArtifactCleanupCompleted).\n'
    '\t\tSelect("COALESCE(SUM(size_bytes), 0)").Scan(&counts.ArtifactBytes).Error; err != nil {',
)
replace_once(
    "internal/service/runtime_history.go",
    '\tCleanupErrorMessage string     `json:"cleanup_error_message,omitempty"`\n\tPublishedAt',
    '\tCleanupErrorMessage string     `json:"cleanup_error_message,omitempty"`\n'
    '\tCleanupCompletedAt  *time.Time `json:"cleanup_completed_at,omitempty"`\n'
    '\tCleanupDisposition  string     `json:"cleanup_disposition,omitempty"`\n'
    '\tCleanupOriginalPath string     `json:"cleanup_original_path,omitempty"`\n'
    '\tCleanupRollbackUntil *time.Time `json:"cleanup_rollback_until,omitempty"`\n'
    '\tPublishedAt',
)
replace_once(
    "internal/service/runtime_history.go",
    '\t\t\tCleanupErrorMessage: truncateRuntimeHistoryText(row.CleanupErrorMessage),\n\t\t\tPublishedAt:',
    '\t\t\tCleanupErrorMessage: truncateRuntimeHistoryText(row.CleanupErrorMessage),\n'
    '\t\t\tCleanupCompletedAt: row.CleanupCompletedAt, CleanupDisposition: row.CleanupDisposition,\n'
    '\t\t\tCleanupOriginalPath: row.CleanupOriginalPath, CleanupRollbackUntil: row.CleanupRollbackUntil,\n'
    '\t\t\tPublishedAt:',
)
replace_once(
    "web/src/api/runtimeHistory.ts",
    '  cleanup_error_message?: string\n  published_at?: string',
    '  cleanup_error_message?: string\n'
    '  cleanup_completed_at?: string\n'
    '  cleanup_disposition?: string\n'
    '  cleanup_original_path?: string\n'
    '  cleanup_rollback_until?: string\n'
    '  published_at?: string',
)

# Existing legacy rows are history and must not be deleted by the general clear
# operation. Fresh databases may not contain the table at all.
replace_once(
    "internal/handler/admin_system.go",
    '\t\t{name: "转码任务", model: &model.TranscodeTask{}},\n',
    '',
)

# ---------------------------------------------------------------------------
# Retirement cleanup uses the same Artifact Cleanup Lease/tombstone state
# machine and never mutates transcode_tasks.
# ---------------------------------------------------------------------------
retirement = read("internal/service/transcode_runtime_retirement.go")
retirement = retirement.replace('\tTasksRetired     int\n', '', 1)
retirement = retirement.replace(' || r.TasksRetired > 0', '', 1)
new_retire = r'''func (s *ArtifactMaintenanceService) retirePersistentRuntimePlayback(now time.Time) (runtimePlaybackRetirementReport, error) {
	report := runtimePlaybackRetirementReport{}
	if s == nil || s.repo == nil || s.repo.DB() == nil || s.cfg == nil {
		return report, nil
	}
	db := s.repo.DB()
	var jobs []model.TranscodeJobRecord
	if err := db.Where("intent IN ?", retiredRuntimePlaybackIntents).Find(&jobs).Error; err != nil {
		return report, fmt.Errorf("list retired runtime playback jobs: %w", err)
	}
	report.JobsFound = len(jobs)
	allJobIDs := make([]string, 0, len(jobs))
	cleanupJobIDs := make([]string, 0, len(jobs))
	cancelJobIDs := make([]string, 0, len(jobs))
	liveJobIDs := make(map[string]struct{})
	for index := range jobs {
		job := &jobs[index]
		allJobIDs = append(allJobIDs, job.ID)
		if runtimePlaybackJobHasLiveLease(job, now) {
			liveJobIDs[job.ID] = struct{}{}
			report.JobsDeferred++
			continue
		}
		cleanupJobIDs = append(cleanupJobIDs, job.ID)
		if !runtimePlaybackJobTerminal(job.Status) {
			cancelJobIDs = append(cancelJobIDs, job.ID)
		}
	}
	if len(allJobIDs) > 0 {
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&model.TranscodeJobRecord{}).Where("id IN ?", allJobIDs).Updates(map[string]any{
				"desired_state": "cancelled", "active_key": nil,
				"cancel_requested_at": now, "updated_at": now,
			}).Error; err != nil {
				return err
			}
			if len(liveJobIDs) > 0 {
				ids := make([]string, 0, len(liveJobIDs))
				for id := range liveJobIDs { ids = append(ids, id) }
				if err := tx.Model(&model.TranscodeJobRecord{}).
					Where("id IN ? AND status NOT IN ?", ids, []string{"completed", "failed", "cancelled"}).
					Updates(map[string]any{"status": "cancel_requested", "updated_at": now}).Error; err != nil { return err }
			}
			if len(cancelJobIDs) > 0 {
				if err := tx.Model(&model.TranscodeJobRecord{}).Where("id IN ?", cancelJobIDs).Updates(map[string]any{
					"status": "cancelled", "worker_id": "", "lease_token": "",
					"claimed_at": nil, "last_heartbeat_at": nil, "lease_expires_at": nil,
					"completed_at": now, "updated_at": now,
				}).Error; err != nil { return err }
			}
			return nil
		})
		if err != nil { return report, fmt.Errorf("fence retired runtime playback jobs: %w", err) }
		report.JobsCancelled = len(cancelJobIDs)
	}

	var attempts []model.TranscodeAttemptRecord
	if len(cleanupJobIDs) > 0 {
		if err := db.Where("job_id IN ?", cleanupJobIDs).Find(&attempts).Error; err != nil {
			return report, fmt.Errorf("list retired runtime playback attempts: %w", err)
		}
	}
	var artifacts []model.TranscodeArtifactRecord
	if err := db.Where("kind IN ?", retiredRuntimeArtifactKinds).Find(&artifacts).Error; err != nil {
		return report, fmt.Errorf("list retired runtime playback artifacts: %w", err)
	}
	cleanupArtifacts := make([]model.TranscodeArtifactRecord, 0, len(artifacts))
	for index := range artifacts {
		if _, live := liveJobIDs[artifacts[index].JobID]; !live && artifacts[index].CleanupState != repository.ArtifactCleanupCompleted {
			cleanupArtifacts = append(cleanupArtifacts, artifacts[index])
		}
	}
	if len(cleanupArtifacts) > 0 {
		ids := make([]string, 0, len(cleanupArtifacts))
		for index := range cleanupArtifacts { ids = append(ids, cleanupArtifacts[index].ID) }
		if err := db.Model(&model.TranscodeArtifactRecord{}).Where("id IN ?", ids).Updates(map[string]any{
			"status": gorm.Expr("CASE WHEN status = ? THEN ? ELSE ? END", "published", "expired", "cancelled"),
			"updated_at": now,
		}).Error; err != nil { return report, fmt.Errorf("terminalize retired artifacts: %w", err) }
	}

	root := filepath.Join(s.cfg.Cache.CacheDir, "transcode")
	paths := make(map[string]struct{})
	for index := range attempts {
		path := strings.TrimSpace(attempts[index].WorkspacePath)
		if path != "" && runtimeRetirementPathAllowed(root, path) { paths[filepath.Clean(path)] = struct{}{} }
	}
	orderedPaths := make([]string, 0, len(paths))
	for path := range paths { orderedPaths = append(orderedPaths, path) }
	sort.Slice(orderedPaths, func(i, j int) bool { return len(orderedPaths[i]) > len(orderedPaths[j]) })
	cleanupErrors := make([]string, 0)
	for _, path := range orderedPaths {
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) { continue } else if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("inspect %s: %v", path, err)); continue
		}
		if err := os.RemoveAll(path); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("remove %s: %v", path, err)); continue
		}
		report.PathsRemoved++
	}
	for index := range cleanupArtifacts {
		removed, err := s.cleanupArtifactRecord(&cleanupArtifacts[index])
		report.PathsRemoved += removed
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("artifact %s: %v", cleanupArtifacts[index].ID, err))
			continue
		}
		report.ArtifactsDeleted++
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if len(attempts) > 0 {
			attemptIDs := make([]string, 0, len(attempts))
			for index := range attempts { attemptIDs = append(attemptIDs, attempts[index].ID) }
			if err := tx.Model(&model.TranscodeAttemptRecord{}).Where("id IN ?", attemptIDs).
				Updates(map[string]any{"workspace_path": "", "updated_at": now}).Error; err != nil { return err }
			if err := tx.Model(&model.TranscodeAttemptRecord{}).
				Where("id IN ? AND status NOT IN ?", attemptIDs, []string{"completed", "failed", "cancelled"}).
				Updates(map[string]any{"status": "cancelled", "completed_at": now,
					"error_code": "runtime_playback_retired", "error_message": ErrPersistentRuntimeTranscodeRetired.Error(), "updated_at": now}).Error; err != nil { return err }
			report.AttemptsRetired = len(attemptIDs)
		}
		if len(cleanupJobIDs) > 0 {
			if err := tx.Model(&model.TranscodeJobRecord{}).Where("id IN ?", cleanupJobIDs).
				Updates(map[string]any{"intent": retiredRuntimePlaybackIntent, "current_attempt_id": "", "updated_at": now}).Error; err != nil { return err }
		}
		return nil
	}); err != nil { return report, fmt.Errorf("finalize runtime playback retirement: %w", err) }
	s.InvalidateCacheDiskUsage()
	if len(cleanupErrors) > 0 {
		return report, fmt.Errorf("retire runtime playback storage: %s", strings.Join(cleanupErrors, "; "))
	}
	return report, nil
}

'''
retirement, count = re.subn(r'func \(s \*ArtifactMaintenanceService\) retirePersistentRuntimePlayback.*?\nfunc runtimeRetirementPathAllowed', new_retire + 'func runtimeRetirementPathAllowed', retirement, count=1, flags=re.S)
if count != 1:
    raise RuntimeError("runtime retirement function replacement failed")
retirement, _ = re.subn(r'\nfunc collectLegacyRuntimeDirectories\(.*?\n}\n?$', '\n', retirement, count=1, flags=re.S)
retirement = retirement.replace('\ttranscodeprofile "github.com/nowen-video/nowen-video/internal/transcode/profile"\n', '')
write("internal/service/transcode_runtime_retirement.go", retirement)
replace_once(
    "internal/service/artifact_maintenance.go",
    'cancelled=%d artifacts=%d attempts=%d tasks=%d paths=%d", report.JobsCancelled, report.ArtifactsDeleted, report.AttemptsRetired, report.TasksRetired, report.PathsRemoved',
    'cancelled=%d artifacts=%d attempts=%d paths=%d", report.JobsCancelled, report.ArtifactsDeleted, report.AttemptsRetired, report.PathsRemoved',
)
# Same log text appears twice.
content = read("internal/service/artifact_maintenance.go")
content = content.replace('cancelled=%d artifacts=%d attempts=%d tasks=%d paths=%d", report.JobsCancelled, report.ArtifactsDeleted, report.AttemptsRetired, report.TasksRetired, report.PathsRemoved', 'cancelled=%d artifacts=%d attempts=%d paths=%d", report.JobsCancelled, report.ArtifactsDeleted, report.AttemptsRetired, report.PathsRemoved')
write("internal/service/artifact_maintenance.go", content)

# ---------------------------------------------------------------------------
# Tests for the new boundaries. Retired compatibility tests are replaced with
# direct migration/tombstone contracts.
# ---------------------------------------------------------------------------
write("internal/service/task_actions_test.go", r'''package service

import (
	"errors"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

type fakeScrapeLookup struct{ task *model.ScrapeTask }
func (f *fakeScrapeLookup) FindByID(string) (*model.ScrapeTask, error) {
	if f.task == nil { return nil, errors.New("not found") }
	return f.task, nil
}
type fakeScrapeActions struct{ retried string }
func (f *fakeScrapeActions) StartScrape(id, _ string) error { f.retried = id; return nil }

type fakeMigrationActions struct{ retried, rolledBack string }
func (f *fakeMigrationActions) RetryArtifactCleanup(id string) error { f.retried = id; return nil }
func (f *fakeMigrationActions) RollbackLegacyArtifactMigration(id string) error { f.rolledBack = id; return nil }
type fakeArtifactLookup struct{ artifact *model.TranscodeArtifactRecord }
func (f *fakeArtifactLookup) FindArtifactCleanupOperation(string) (*model.TranscodeArtifactRecord, error) {
	if f.artifact == nil { return nil, errors.New("not found") }
	return f.artifact, nil
}

func TestAvailableTaskActions(t *testing.T) {
	if got := AvailableTaskActions(TaskKindScrape, "failed"); len(got) != 1 || got[0] != TaskActionRetry { t.Fatalf("scrape actions=%v", got) }
	if got := AvailableTaskActions(TaskKindLegacyArtifactMigration, "queued"); len(got) != 1 || got[0] != TaskActionRollback { t.Fatalf("migration actions=%v", got) }
	if got := AvailableTaskActions(TaskKindScan, "running"); len(got) != 0 { t.Fatalf("scan actions=%v", got) }
}

func TestTaskActionDispatcherRetryScrape(t *testing.T) {
	actions := &fakeScrapeActions{}
	d := &TaskActionDispatcher{scrape: actions, scrapeLookup: &fakeScrapeLookup{task: &model.ScrapeTask{Status: "failed"}}}
	if _, err := d.Execute(TaskKindScrape, "s-1", TaskActionRetry, "admin"); err != nil { t.Fatal(err) }
	if actions.retried != "s-1" { t.Fatalf("retried=%q", actions.retried) }
}

func TestTaskActionDispatcherRollbackLegacyMigration(t *testing.T) {
	actions := &fakeMigrationActions{}
	d := &TaskActionDispatcher{
		artifactCleanup: actions,
		artifactLookup: &fakeArtifactLookup{artifact: &model.TranscodeArtifactRecord{
			MigrationSource: repository.LegacyTranscodeArtifactMigrationSource,
			CleanupState: repository.ArtifactCleanupPending,
		}},
	}
	if _, err := d.Execute(TaskKindLegacyArtifactMigration, "a-1", TaskActionRollback, "admin"); err != nil { t.Fatal(err) }
	if actions.rolledBack != "a-1" { t.Fatalf("rolledBack=%q", actions.rolledBack) }
}
''')

# Remove only the retired transcode mapping test from task_center_test.go.
content = read("internal/service/task_center_test.go")
content, count = re.subn(r'\nfunc TestTranscodeTaskMapping\(.*?\n}\n', '\n', content, count=1, flags=re.S)
if count != 1:
    raise RuntimeError("remove TestTranscodeTaskMapping failed")
write("internal/service/task_center_test.go", content)

write("internal/service/transcode_runtime_retirement_test.go", r'''package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

func TestRetiredRuntimeArtifactUsesCleanupTombstone(t *testing.T) {
	service, db := newArtifactMaintenanceTestService(t)
	now := time.Now()
	path := filepath.Join(service.artifactStore.Root(), "artifacts", "media", "720p", "runtime-old")
	if err := os.MkdirAll(path, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(path, "seg.ts"), []byte("segment"), 0o644); err != nil { t.Fatal(err) }
	job := model.TranscodeJobRecord{ID: "runtime-old", MediaID: "media", Intent: retiredRuntimePlaybackIntents[0], Status: "completed", DesiredState: "cancelled", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&job).Error; err != nil { t.Fatal(err) }
	artifact := model.TranscodeArtifactRecord{ID: "artifact-old", JobID: job.ID, MediaID: job.MediaID, Kind: "hls_variant", ProfileID: "720p", Path: path, Status: "published", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&artifact).Error; err != nil { t.Fatal(err) }
	report, err := service.retirePersistentRuntimePlayback(now)
	if err != nil { t.Fatal(err) }
	if report.ArtifactsDeleted != 1 { t.Fatalf("report=%+v", report) }
	if _, err := os.Stat(path); !os.IsNotExist(err) { t.Fatalf("artifact path still exists: %v", err) }
	var stored model.TranscodeArtifactRecord
	if err := db.First(&stored, "id = ?", artifact.ID).Error; err != nil { t.Fatal(err) }
	if stored.CleanupState != repository.ArtifactCleanupCompleted || stored.Path != "" || stored.CleanupOriginalPath != path {
		t.Fatalf("unexpected tombstone: %+v", stored)
	}
}

func TestRuntimeRetirementPathFenceProtectsRoots(t *testing.T) {
	root := t.TempDir()
	if runtimeRetirementPathAllowed(root, root) { t.Fatal("root must be protected") }
	if runtimeRetirementPathAllowed(root, filepath.Join(root, "artifacts")) { t.Fatal("artifact namespace root must be protected") }
	if !runtimeRetirementPathAllowed(root, filepath.Join(root, "workspaces", "job", "attempt")) { t.Fatal("attempt workspace must be removable") }
}
''')

write("internal/model/legacy_transcode_projection_migration_test.go", r'''package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestFreshProfilesDoNotCreateLegacyTranscodeTasks(t *testing.T) {
	for _, migrate := range []struct {
		name string
		run func(*gorm.DB) error
	}{
		{name: "lite", run: func(db *gorm.DB) error { return AutoMigrateLite(db, false) }},
		{name: "full", run: AutoMigrate},
	} {
		t.Run(migrate.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:"+migrate.name+"-no-legacy?mode=memory&cache=shared"), &gorm.Config{})
			if err != nil { t.Fatal(err) }
			if err := migrate.run(db); err != nil { t.Fatal(err) }
			if db.Migrator().HasTable(&TranscodeTask{}) { t.Fatal("fresh profile created transcode_tasks") }
		})
	}
}

func TestExistingLegacyTranscodeTasksSurviveProfileMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:existing-legacy?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&TranscodeTask{}); err != nil { t.Fatal(err) }
	if err := AutoMigrateLite(db, false); err != nil { t.Fatal(err) }
	if !db.Migrator().HasTable(&TranscodeTask{}) { t.Fatal("existing legacy table was removed") }
}
''')

write("internal/service/legacy_transcode_projection_migration_test.go", r'''package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

func TestLegacyProjectionInventorySupportsRollback(t *testing.T) {
	service, db := newArtifactMaintenanceTestService(t)
	now := time.Now()
	path := filepath.Join(service.artifactStore.Root(), "media-legacy", "720p")
	if err := os.MkdirAll(path, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(path, "stream.m3u8"), []byte("#EXTM3U"), 0o644); err != nil { t.Fatal(err) }
	task := model.TranscodeTask{ID: "legacy-task", MediaID: "media-legacy", Status: "done", Quality: "720p", OutputDir: path, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	if err := db.Create(&task).Error; err != nil { t.Fatal(err) }
	report, err := service.inventoryLegacyTranscodeProjection(now)
	if err != nil { t.Fatal(err) }
	if report.ArtifactsQueued != 1 { t.Fatalf("report=%+v", report) }
	artifactID := deterministicLegacyProjectionID("artifact", task.ID)
	var artifact model.TranscodeArtifactRecord
	if err := db.First(&artifact, "id = ?", artifactID).Error; err != nil { t.Fatal(err) }
	if artifact.CleanupState != repository.ArtifactCleanupPending || artifact.CleanupRollbackUntil == nil { t.Fatalf("artifact=%+v", artifact) }
	if err := service.RollbackLegacyArtifactMigration(artifact.ID); err != nil { t.Fatal(err) }
	if err := db.First(&artifact, "id = ?", artifact.ID).Error; err != nil { t.Fatal(err) }
	if artifact.CleanupState != repository.ArtifactCleanupRollbackCompleted || artifact.Path != path { t.Fatalf("rollback=%+v", artifact) }
	if _, err := os.Stat(path); err != nil { t.Fatalf("rollback removed directory: %v", err) }
}

func TestLegacyProjectionCleanupPreservesTombstone(t *testing.T) {
	service, db := newArtifactMaintenanceTestService(t)
	now := time.Now()
	path := filepath.Join(service.artifactStore.Root(), "media-expired", "480p")
	if err := os.MkdirAll(path, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(path, "seg.ts"), []byte("segment"), 0o644); err != nil { t.Fatal(err) }
	task := model.TranscodeTask{ID: "legacy-expired", MediaID: "media-expired", Status: "failed", Quality: "480p", OutputDir: path, CreatedAt: now.Add(-30*24*time.Hour), UpdatedAt: now.Add(-30*24*time.Hour)}
	if err := db.Create(&task).Error; err != nil { t.Fatal(err) }
	inventoryAt := now.Add(-8 * 24 * time.Hour)
	if _, err := service.inventoryLegacyTranscodeProjection(inventoryAt); err != nil { t.Fatal(err) }
	if _, _, err := service.cleanupTerminalArtifactBatch(now.Add(-24*time.Hour), now); err != nil { t.Fatal(err) }
	artifactID := deterministicLegacyProjectionID("artifact", task.ID)
	var artifact model.TranscodeArtifactRecord
	if err := db.First(&artifact, "id = ?", artifactID).Error; err != nil { t.Fatal(err) }
	if artifact.CleanupState != repository.ArtifactCleanupCompleted || artifact.CleanupOriginalPath != path || artifact.Path != "" { t.Fatalf("tombstone=%+v", artifact) }
	if _, err := os.Stat(path); !os.IsNotExist(err) { t.Fatalf("legacy path remains: %v", err) }
	var legacy model.TranscodeTask
	if err := db.First(&legacy, "id = ?", task.ID).Error; err != nil { t.Fatalf("legacy row was mutated/deleted: %v", err) }
	if legacy.OutputDir != path || legacy.Status != "failed" { t.Fatalf("legacy row changed: %+v", legacy) }
}
''')

# Update old cleanup tests from row deletion to completed tombstones.
for path in (
    "internal/repository/repo_transcode_artifact_cleanup_test.go",
    "internal/service/transcode_cleanup_state_test.go",
    "internal/service/transcode_disk_pressure_test.go",
):
    content = read(path)
    content = content.replace("DeleteArtifactByCleanupClaim", "CompleteArtifactCleanupByClaim")
    write(path, content)

# Contract test keeps production code honest.
write("cmd/server/legacy_projection_retired_test.go", r'''package main

import (
	"os"
	"strings"
	"testing"
)

func TestLegacyTranscodeProjectionIsReadOnlyAndOptional(t *testing.T) {
	for _, path := range []string{"../../internal/model/model.go", "../../internal/model/migrate_lite.go"} {
		content, err := os.ReadFile(path)
		if err != nil { t.Fatal(err) }
		if strings.Contains(string(content), "&TranscodeTask{}") { t.Fatalf("%s still auto-migrates transcode_tasks", path) }
	}
	for _, path := range []string{
		"../../internal/service/task_center.go",
		"../../internal/service/task_actions.go",
		"../../internal/service/transcode_cleanup.go",
		"../../internal/service/transcode_runtime_retirement.go",
	} {
		content, err := os.ReadFile(path)
		if err != nil { t.Fatal(err) }
		if strings.Contains(string(content), "model.TranscodeTask") { t.Fatalf("%s still uses legacy task as runtime state", path) }
	}
	if _, err := os.Stat("../../internal/service/task_center_runtime_retirement.go"); !os.IsNotExist(err) {
		t.Fatalf("runtime-nil compatibility constructor still exists: %v", err)
	}
}

func TestArtifactCleanupKeepsAuditTombstones(t *testing.T) {
	content, err := os.ReadFile("../../internal/repository/repo_transcode_artifact_cleanup.go")
	if err != nil { t.Fatal(err) }
	text := string(content)
	for _, marker := range []string{"CompleteArtifactCleanupByClaim", "ArtifactCleanupCompleted", "cleanup_original_path", "RollbackLegacyArtifactCleanup"} {
		if !strings.Contains(text, marker) { t.Fatalf("cleanup repository missing %q", marker) }
	}
	if strings.Contains(text, ").Delete(&model.TranscodeArtifactRecord{})") {
		t.Fatal("cleanup still deletes Artifact audit rows")
	}
}
''')

# Documentation for operators and rollback semantics.
write("docs/LEGACY_TRANSCODE_PROJECTION_RETIREMENT.md", r'''# Legacy Transcode Projection Retirement

`transcode_jobs`, `transcode_attempts` and `transcode_artifacts` are the only
maintained Runtime history model. The old `transcode_tasks` table is no longer
created by Lite or Full migrations and has no write, retry, cancel, progress or
retention code path.

Existing deployments keep the table unchanged for database rollback and audit.
Runtime History may count it when present, but production code treats it as an
optional read-only import source.

## Legacy directory inventory

Terminal legacy rows with an `output_dir` are imported idempotently into a
deterministic historical Job and a `legacy_hls_directory` Artifact. The file is
not removed immediately. The Artifact receives a seven-day rollback deadline
and enters the same durable Cleanup Lease state machine used by all other
Artifact reclamation.

Invalid or out-of-root paths are blocked with evidence and are never removed.
Missing paths become completed audit tombstones.

## Rollback

Before cleanup is claimed, an administrator can choose **保留目录** in Task
Center. This changes the Artifact to `rollback_completed` and removes it from
the cleanup work set without modifying the directory or resurrecting the old
Runtime executor.

A claimed or completed cleanup cannot be rolled back because filesystem work
may already have started.

## Cleanup evidence

Successful cleanup no longer deletes `transcode_artifacts`. It clears the live
path fields and records:

- `cleanup_state=completed`
- completion time and disposition
- original path, temporary path and manifest path
- retained byte count and migration source

Runtime History therefore remains auditable while storage summaries count only
files that may still exist.
''')

print("legacy projection retirement implementation applied")
