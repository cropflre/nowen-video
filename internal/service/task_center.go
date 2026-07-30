package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

const (
	TaskKindScan      = "scan"
	TaskKindScrape    = "scrape"
	TaskKindTranscode = "transcode"

	TaskStatusQueued    = "queued"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusCancelled = "cancelled"
)

// UnifiedTask is the stable task representation consumed by the Lite UI.
// It intentionally adapts existing task stores instead of introducing another
// persistence table or execution queue.
type UnifiedTask struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	Title       string     `json:"title"`
	Subtitle    string     `json:"subtitle,omitempty"`
	Message     string     `json:"message,omitempty"`
	Progress    float64    `json:"progress"`
	SourceID    string     `json:"source_id,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type TaskCenterSummary struct {
	Total     int            `json:"total"`
	Active    int            `json:"active"`
	ByStatus  map[string]int `json:"by_status"`
	ByKind    map[string]int `json:"by_kind"`
	Generated time.Time      `json:"generated_at"`
}

type TaskCenterSnapshot struct {
	Tasks   []UnifiedTask     `json:"tasks"`
	Summary TaskCenterSummary `json:"summary"`
}

// TaskCenterService adapts scan phases plus persisted scrape/transcode tasks
// into one read model. Existing execution services remain the source of truth.
type TaskCenterService struct {
	library       *LibraryService
	transcodeRepo *repository.TranscodeRepo
	scrapeRepo    *repository.ScrapeTaskRepo
	logger        *zap.SugaredLogger
}

func NewTaskCenterService(
	library *LibraryService,
	transcodeRepo *repository.TranscodeRepo,
	scrapeRepo *repository.ScrapeTaskRepo,
	logger *zap.SugaredLogger,
) *TaskCenterService {
	return &TaskCenterService{
		library:       library,
		transcodeRepo: transcodeRepo,
		scrapeRepo:    scrapeRepo,
		logger:        logger,
	}
}

func (s *TaskCenterService) Snapshot(activeOnly bool, limit int) (*TaskCenterSnapshot, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	tasks := make([]UnifiedTask, 0, limit+8)
	now := time.Now()

	if s.library != nil {
		for _, phase := range s.library.ActiveScanPhases() {
			updated := now
			tasks = append(tasks, scanPhaseToUnifiedTask(phase, &updated))
		}
	}

	if s.transcodeRepo != nil {
		rows, _, err := s.transcodeRepo.ListAll(1, limit, "")
		if err != nil {
			return nil, fmt.Errorf("list transcode tasks: %w", err)
		}
		for i := range rows {
			task := transcodeToUnifiedTask(&rows[i])
			if !activeOnly || isTaskActive(task.Status) {
				tasks = append(tasks, task)
			}
		}
	}

	if s.scrapeRepo != nil {
		rows, _, err := s.scrapeRepo.List(1, limit, "", "")
		if err != nil {
			return nil, fmt.Errorf("list scrape tasks: %w", err)
		}
		for i := range rows {
			task := scrapeToUnifiedTask(&rows[i])
			if !activeOnly || isTaskActive(task.Status) {
				tasks = append(tasks, task)
			}
		}
	}

	sort.SliceStable(tasks, func(i, j int) bool {
		leftActive := isTaskActive(tasks[i].Status)
		rightActive := isTaskActive(tasks[j].Status)
		if leftActive != rightActive {
			return leftActive
		}
		return taskSortTime(tasks[i]).After(taskSortTime(tasks[j]))
	})

	if len(tasks) > limit {
		tasks = tasks[:limit]
	}

	summary := TaskCenterSummary{
		Total:     len(tasks),
		ByStatus:  make(map[string]int),
		ByKind:    make(map[string]int),
		Generated: now,
	}
	for _, task := range tasks {
		summary.ByStatus[task.Status]++
		summary.ByKind[task.Kind]++
		if isTaskActive(task.Status) {
			summary.Active++
		}
	}

	return &TaskCenterSnapshot{Tasks: tasks, Summary: summary}, nil
}

func scanPhaseToUnifiedTask(phase ScanPhaseData, updatedAt *time.Time) UnifiedTask {
	progress := taskProgress(phase.Current, phase.Total)
	if phase.Total <= 0 {
		progress = taskProgress(phase.StepCurrent, phase.StepTotal)
	}
	return UnifiedTask{
		ID:        "scan:" + phase.LibraryID,
		Kind:      TaskKindScan,
		Status:    TaskStatusRunning,
		Title:     fallbackText(phase.LibraryName, "媒体库扫描"),
		Subtitle:  phaseLabel(phase.Phase),
		Message:   phase.Message,
		Progress:  progress,
		SourceID:  phase.LibraryID,
		UpdatedAt: updatedAt,
	}
}

func transcodeToUnifiedTask(task *model.TranscodeTask) UnifiedTask {
	title := task.MediaTitle
	if title == "" {
		title = task.Media.Title
	}
	return UnifiedTask{
		ID:          "transcode:" + task.ID,
		Kind:        TaskKindTranscode,
		Status:      normalizeTaskStatus(task.Status),
		Title:       fallbackText(title, "视频转码"),
		Subtitle:    task.Quality,
		Message:     task.Error,
		Progress:    clampProgress(task.Progress),
		SourceID:    task.ID,
		CreatedAt:   timePtr(task.CreatedAt),
		UpdatedAt:   timePtr(task.UpdatedAt),
		StartedAt:   task.StartedAt,
		CompletedAt: task.CompletedAt,
	}
}

func scrapeToUnifiedTask(task *model.ScrapeTask) UnifiedTask {
	message := task.ErrorMessage
	if message == "" && task.URL != "" {
		message = task.URL
	}
	return UnifiedTask{
		ID:        "scrape:" + task.ID,
		Kind:      TaskKindScrape,
		Status:    normalizeTaskStatus(task.Status),
		Title:     fallbackText(task.Title, task.ResultTitle, "元数据刮削"),
		Subtitle:  strings.ToUpper(task.Source),
		Message:   message,
		Progress:  clampProgress(float64(task.Progress)),
		SourceID:  task.ID,
		CreatedAt: timePtr(task.CreatedAt),
		UpdatedAt: timePtr(task.UpdatedAt),
	}
}

func normalizeTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending", "queued", "waiting":
		return TaskStatusQueued
	case "running", "scanning", "scraping", "translating", "processing":
		return TaskStatusRunning
	case "done", "scraped", "completed", "success":
		return TaskStatusCompleted
	case "cancelled", "canceled", "paused":
		return TaskStatusCancelled
	case "failed", "error":
		return TaskStatusFailed
	default:
		return TaskStatusQueued
	}
}

func isTaskActive(status string) bool {
	return status == TaskStatusQueued || status == TaskStatusRunning
}

func taskProgress(current, total int) float64 {
	if total <= 0 {
		return 0
	}
	return clampProgress(float64(current) / float64(total) * 100)
}

func clampProgress(progress float64) float64 {
	if progress < 0 {
		return 0
	}
	if progress > 100 {
		return 100
	}
	return progress
}

func taskSortTime(task UnifiedTask) time.Time {
	if task.UpdatedAt != nil {
		return *task.UpdatedAt
	}
	if task.StartedAt != nil {
		return *task.StartedAt
	}
	if task.CreatedAt != nil {
		return *task.CreatedAt
	}
	return time.Time{}
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value
	return &copy
}

func fallbackText(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return "任务"
}

func phaseLabel(phase string) string {
	switch phase {
	case "scanning":
		return "扫描文件"
	case "scraping":
		return "匹配元数据"
	case "cleaning":
		return "清理索引"
	case "merging":
		return "合并剧集"
	case "matching":
		return "匹配合集"
	case "ai_organizing":
		return "智能整理"
	default:
		return "扫描媒体库"
	}
}
