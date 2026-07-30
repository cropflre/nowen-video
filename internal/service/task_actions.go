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

// TaskActionResult is returned after a task operation has been accepted by the
// existing module-specific executor. It does not introduce another queue or
// persistence layer.
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

type transcodeTaskActions interface {
	CancelTranscode(taskID string) error
	RetryTask(taskID string, mediaResolver func(mediaID string) (*model.Media, error)) error
}

type scrapeTaskActions interface {
	StartScrape(taskID, userID string) error
}

// TaskActionDispatcher validates unified task operations, then delegates to the
// existing transcode and scrape services. The original services remain the
// authoritative executors and task stores.
type TaskActionDispatcher struct {
	transcode       transcodeTaskActions
	scrape          scrapeTaskActions
	transcodeLookup transcodeTaskLookup
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
	return &TaskActionDispatcher{
		transcode:       transcode,
		scrape:          scrape,
		transcodeLookup: transcodeRepo,
		scrapeLookup:    scrapeRepo,
		mediaResolver:   resolver,
		wsHub:           wsHub,
		logger:          logger,
	}
}

// AvailableTaskActions is the shared policy used by both the API response and
// the action dispatcher. Only operations that are safe with the current
// executor implementation are exposed.
func AvailableTaskActions(kind, status string) []string {
	normalizedKind := strings.ToLower(strings.TrimSpace(kind))
	normalizedStatus := normalizeTaskStatus(status)

	switch normalizedKind {
	case TaskKindTranscode:
		switch normalizedStatus {
		case TaskStatusRunning:
			return []string{TaskActionCancel}
		case TaskStatusFailed, TaskStatusCancelled:
			return []string{TaskActionRetry}
		}
	case TaskKindScrape:
		if normalizedStatus == TaskStatusFailed || normalizedStatus == TaskStatusCancelled {
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
		if kind == TaskKindScrape {
			return "刮削任务已重新提交"
		}
		return "转码任务已重新提交"
	default:
		return "任务操作已提交"
	}
}
