package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

const (
	TaskActionCancel = "cancel"
	TaskActionRetry  = "retry"

	EventTaskUpdated = "task_updated"
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

type transcodeTaskLookup interface {
	FindByID(id string) (*model.TranscodeTask, error)
}

type scrapeTaskLookup interface {
	FindByID(id string) (*model.ScrapeTask, error)
}

type artifactCleanupLookup interface {
	FindArtifactCleanupOperation(id string) (*model.TranscodeArtifactRecord, error)
}

type transcodeTaskActions interface {
	CancelTranscode(taskID string) error
	RetryTask(taskID string, mediaResolver func(mediaID string) (*model.Media, error)) error
}

type artifactCleanupActions interface {
	RetryArtifactCleanup(artifactID string) error
}

type scrapeTaskActions interface {
	StartScrape(taskID, userID string) error
}

type TaskActionDispatcher struct {
	transcode       transcodeTaskActions
	artifactCleanup artifactCleanupActions
	scrape          scrapeTaskActions
	transcodeLookup transcodeTaskLookup
	artifactLookup  artifactCleanupLookup
	scrapeLookup    scrapeTaskLookup
	mediaResolver   func(mediaID string) (*model.Media, error)
	wsHub           *WSHub
	logger          *zap.SugaredLogger
}

func NewTaskActionDispatcher(
	transcode *TranscodeService,
	scrape *ScrapeManagerService,
	transcodeRepo *repository.TranscodeRepo,
	scrapeRepo *repository.ScrapeTaskRepo,
	mediaRepo *repository.MediaRepo,
	wsHub *WSHub,
	logger *zap.SugaredLogger,
) *TaskActionDispatcher {
	var resolver func(string) (*model.Media, error)
	if mediaRepo != nil {
		resolver = mediaRepo.FindByID
	}
	dispatcher := &TaskActionDispatcher{
		transcode:       transcode,
		scrape:          scrape,
		transcodeLookup: transcodeRepo,
		scrapeLookup:    scrapeRepo,
		mediaResolver:   resolver,
		wsHub:           wsHub,
		logger:          logger,
	}
	if transcode != nil {
		dispatcher.artifactCleanup = transcode
	}
	if transcodeRepo != nil && transcodeRepo.DB() != nil {
		dispatcher.artifactLookup = repository.NewTranscodeExecutionRepo(transcodeRepo.DB())
	}
	return dispatcher
}

// AvailableTaskActions is shared by the API and dispatcher. Queued transcode
// jobs are cancellable because cancellation is now a persisted desired state,
// not a best-effort signal consumed only by a running worker.
func AvailableTaskActions(kind, status string) []string {
	normalizedKind := strings.ToLower(strings.TrimSpace(kind))
	normalizedStatus := normalizeTaskStatus(status)

	switch normalizedKind {
	case TaskKindTranscode:
		switch normalizedStatus {
		case TaskStatusQueued, TaskStatusRunning:
			return []string{TaskActionCancel}
		case TaskStatusFailed, TaskStatusCancelled:
			return []string{TaskActionRetry}
		}
	case TaskKindScrape:
		if normalizedStatus == TaskStatusFailed || normalizedStatus == TaskStatusCancelled {
			return []string{TaskActionRetry}
		}
	case TaskKindArtifactCleanup:
		if normalizedStatus == TaskStatusFailed {
			return []string{TaskActionRetry}
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
	case TaskKindTranscode:
		err = d.executeTranscode(sourceID, action)
	case TaskKindScrape:
		err = d.executeScrape(sourceID, action, userID)
	case TaskKindArtifactCleanup:
		err = d.executeArtifactCleanup(sourceID, action)
	case TaskKindScan:
		err = fmt.Errorf("%w: scan tasks do not expose lifecycle controls", ErrTaskActionUnsupported)
	default:
		err = fmt.Errorf("%w: unknown task kind %q", ErrTaskActionUnsupported, kind)
	}
	if err != nil {
		return nil, err
	}

	result := &TaskActionResult{
		ID:       kind + ":" + sourceID,
		Kind:     kind,
		SourceID: sourceID,
		Action:   action,
		Accepted: true,
		Message:  taskActionMessage(kind, action),
	}
	if d.wsHub != nil {
		d.wsHub.BroadcastEvent(EventTaskUpdated, result)
	}
	if d.logger != nil {
		d.logger.Infof("统一任务操作已受理 kind=%s source_id=%s action=%s actor=%s", kind, sourceID, action, userID)
	}
	return result, nil
}

func (d *TaskActionDispatcher) executeTranscode(sourceID, action string) error {
	if d.transcodeLookup == nil || d.transcode == nil {
		return fmt.Errorf("转码任务执行器不可用")
	}
	task, err := d.transcodeLookup.FindByID(sourceID)
	if err != nil || task == nil {
		return fmt.Errorf("%w: transcode %s", ErrTaskNotFound, sourceID)
	}
	if !containsAction(AvailableTaskActions(TaskKindTranscode, task.Status), action) {
		if action == TaskActionCancel || action == TaskActionRetry {
			return fmt.Errorf("%w: transcode status=%s action=%s", ErrTaskActionConflict, task.Status, action)
		}
		return fmt.Errorf("%w: transcode action=%s", ErrTaskActionUnsupported, action)
	}

	switch action {
	case TaskActionCancel:
		if err := d.transcode.CancelTranscode(sourceID); err != nil {
			return fmt.Errorf("取消转码失败: %w", err)
		}
	case TaskActionRetry:
		if d.mediaResolver == nil {
			return fmt.Errorf("转码媒体解析器不可用")
		}
		if err := d.transcode.RetryTask(sourceID, d.mediaResolver); err != nil {
			return fmt.Errorf("重试转码失败: %w", err)
		}
	}
	return nil
}

func (d *TaskActionDispatcher) executeArtifactCleanup(sourceID, action string) error {
	if d.artifactLookup == nil || d.artifactCleanup == nil {
		return fmt.Errorf("Artifact 清理执行器不可用")
	}
	artifact, err := d.artifactLookup.FindArtifactCleanupOperation(sourceID)
	if err != nil || artifact == nil {
		return fmt.Errorf("%w: artifact cleanup %s", ErrTaskNotFound, sourceID)
	}
	if action != TaskActionRetry {
		return fmt.Errorf("%w: artifact cleanup action=%s", ErrTaskActionUnsupported, action)
	}
	if artifact.CleanupState != repository.ArtifactCleanupBlocked && artifact.CleanupState != repository.ArtifactCleanupRetryWait {
		return fmt.Errorf("%w: artifact cleanup state=%s action=%s", ErrTaskActionConflict, artifact.CleanupState, action)
	}
	if err := d.artifactCleanup.RetryArtifactCleanup(sourceID); err != nil {
		if errors.Is(err, ErrArtifactCleanupNotRetryable) {
			return fmt.Errorf("%w: artifact cleanup state changed", ErrTaskActionConflict)
		}
		return fmt.Errorf("重试 Artifact 清理失败: %w", err)
	}
	return nil
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
		if action == TaskActionCancel || action == TaskActionRetry {
			return fmt.Errorf("%w: scrape status=%s action=%s", ErrTaskActionConflict, task.Status, action)
		}
		return fmt.Errorf("%w: scrape action=%s", ErrTaskActionUnsupported, action)
	}
	if action == TaskActionRetry {
		if err := d.scrape.StartScrape(sourceID, userID); err != nil {
			return fmt.Errorf("重试刮削失败: %w", err)
		}
		return nil
	}
	return fmt.Errorf("%w: scrape action=%s", ErrTaskActionUnsupported, action)
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
	case TaskActionCancel:
		return "取消请求已提交"
	case TaskActionRetry:
		switch kind {
		case TaskKindScrape:
			return "刮削任务已重新提交"
		case TaskKindArtifactCleanup:
			return "Artifact 清理已重新执行"
		default:
			return "转码任务已重新提交"
		}
	default:
		return "任务操作已提交"
	}
}
