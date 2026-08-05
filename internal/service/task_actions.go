package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

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

func AvailableTaskActionsForTask(task UnifiedTask, now time.Time) []string {
	actions := AvailableTaskActions(task.Kind, task.Status)
	if task.Kind != TaskKindLegacyArtifactMigration {
		return actions
	}
	if task.RollbackUntil != nil && !now.After(*task.RollbackUntil) {
		return actions
	}
	filtered := make([]string, 0, len(actions))
	for _, action := range actions {
		if action != TaskActionRollback {
			filtered = append(filtered, action)
		}
	}
	return filtered
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
	if action != TaskActionRetry && action != TaskActionRollback {
		return fmt.Errorf("%w: artifact cleanup action=%s", ErrTaskActionUnsupported, action)
	}
	task := UnifiedTask{
		Kind:          mapArtifactTaskKind(artifact),
		Status:        mapArtifactTaskStatus(artifact),
		RollbackUntil: artifact.CleanupRollbackUntil,
	}
	if !containsAction(AvailableTaskActionsForTask(task, time.Now()), action) {
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
