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
	TaskKindScan                      = "scan"
	TaskKindScrape                    = "scrape"
	TaskKindLegacyArtifactMigration   = "legacy_artifact_migration"
	TaskKindLegacyProjectionMigration = "legacy_projection_migration"
	TaskKindArtifactCleanup           = "artifact_cleanup"
	TaskKindStorageIncident           = "storage_incident"

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
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	Status        string     `json:"status"`
	Title         string     `json:"title"`
	Subtitle      string     `json:"subtitle,omitempty"`
	Message       string     `json:"message,omitempty"`
	Progress      float64    `json:"progress"`
	SourceID      string     `json:"source_id,omitempty"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	RollbackUntil *time.Time `json:"rollback_until,omitempty"`
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

// TaskCenterService adapts scan phases plus persisted scrape/transcode/cleanup
// work and storage incidents into one read model. Existing execution services
// remain the source of truth.
type TaskCenterService struct {
	library       *LibraryService
	scrapeRepo    *repository.ScrapeTaskRepo
	executionRepo *repository.TranscodeExecutionRepo
	logger        *zap.SugaredLogger
}

func NewTaskCenterService(
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

func (s *TaskCenterService) Snapshot(activeOnly bool, limit int) (*TaskCenterSnapshot, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	tasks := make([]UnifiedTask, 0, limit+16)
	now := time.Now()

	if s.library != nil {
		for _, phase := range s.library.ActiveScanPhases() {
			updated := now
			tasks = append(tasks, scanPhaseToUnifiedTask(phase, &updated))
		}
	}

	if s.executionRepo != nil {
		migration, err := s.executionRepo.LegacyProjectionMigrationState(repository.LegacyTranscodeArtifactMigrationSource)
		if err != nil {
			return nil, fmt.Errorf("read legacy projection migration: %w", err)
		}
		if migration != nil && (migration.TargetRows > 0 || migration.Status == repository.LegacyProjectionMigrationFailed || migration.Status == repository.LegacyProjectionMigrationRunning) {
			task := legacyProjectionMigrationToUnifiedTask(migration, now)
			if !activeOnly || isTaskActive(task.Status) {
				tasks = append(tasks, task)
			}
		}

		incidents, err := s.executionRepo.ListActiveStorageIncidents(limit)
		if err != nil {
			return nil, fmt.Errorf("list storage incidents: %w", err)
		}
		for i := range incidents {
			task := storageIncidentToUnifiedTask(&incidents[i])
			if !activeOnly || isTaskActive(task.Status) {
				tasks = append(tasks, task)
			}
		}

		rows, err := s.executionRepo.ListArtifactCleanupOperations(limit)
		if err != nil {
			return nil, fmt.Errorf("list artifact cleanup operations: %w", err)
		}
		for i := range rows {
			task := artifactCleanupToUnifiedTask(&rows[i])
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
		leftStorageFailure := tasks[i].Kind == TaskKindStorageIncident && tasks[i].Status == TaskStatusFailed
		rightStorageFailure := tasks[j].Kind == TaskKindStorageIncident && tasks[j].Status == TaskStatusFailed
		if leftStorageFailure != rightStorageFailure {
			return leftStorageFailure
		}
		leftCleanupFailure := tasks[i].Kind == TaskKindArtifactCleanup && tasks[i].Status == TaskStatusFailed
		rightCleanupFailure := tasks[j].Kind == TaskKindArtifactCleanup && tasks[j].Status == TaskStatusFailed
		if leftCleanupFailure != rightCleanupFailure {
			return leftCleanupFailure
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

func storageIncidentToUnifiedTask(incident *model.TranscodeStorageIncidentRecord) UnifiedTask {
	if incident == nil {
		return UnifiedTask{}
	}
	subtitle := storageIncidentCodeLabel(incident.Code)
	if incident.Occurrences > 1 {
		subtitle += fmt.Sprintf(" · 已出现 %d 次", incident.Occurrences)
	}
	messageParts := make([]string, 0, 3)
	if incident.Message != "" {
		messageParts = append(messageParts, incident.Message)
	}
	if incident.Path != "" {
		messageParts = append(messageParts, incident.Path)
	}
	if incident.Retryable {
		messageParts = append(messageParts, "系统将持续探测并在存储恢复后自动解除队列暂停")
	} else {
		messageParts = append(messageParts, "需要管理员修复挂载模式或目录权限")
	}
	return UnifiedTask{
		ID:        TaskKindStorageIncident + ":" + incident.ID,
		Kind:      TaskKindStorageIncident,
		Status:    TaskStatusFailed,
		Title:     "转码存储不可写",
		Subtitle:  subtitle,
		Message:   strings.Join(messageParts, " · "),
		Progress:  0,
		SourceID:  incident.ID,
		CreatedAt: timePtr(incident.FirstSeenAt),
		UpdatedAt: timePtr(incident.LastSeenAt),
		StartedAt: timePtr(incident.FirstSeenAt),
	}
}

func storageIncidentCodeLabel(code string) string {
	switch code {
	case "no_space":
		return "空间耗尽"
	case "read_only":
		return "只读文件系统"
	case "permission_denied":
		return "目录权限不足"
	case "unavailable":
		return "挂载不可用"
	case "io_error":
		return "存储 I/O 错误"
	default:
		return "存储状态异常"
	}
}

func legacyProjectionMigrationToUnifiedTask(state *model.LegacyTranscodeProjectionMigrationState, now time.Time) UnifiedTask {
	if state == nil {
		return UnifiedTask{}
	}
	status := TaskStatusQueued
	switch state.Status {
	case repository.LegacyProjectionMigrationRunning:
		status = TaskStatusRunning
	case repository.LegacyProjectionMigrationFailed:
		status = TaskStatusFailed
	case repository.LegacyProjectionMigrationCompleted:
		status = TaskStatusCompleted
	}
	progress := float64(0)
	if state.TargetRows > 0 {
		progress = float64(state.ScannedRows) / float64(state.TargetRows) * 100
		if progress > 100 {
			progress = 100
		}
	} else if status == TaskStatusCompleted {
		progress = 100
	}
	subtitle := fmt.Sprintf("第 %d 代 · %d/%d", state.Generation, state.ScannedRows, state.TargetRows)
	message := "按持久游标登记旧转码目录"
	if state.Status == repository.LegacyProjectionMigrationFailed {
		message = strings.TrimSpace(strings.Join([]string{state.LastErrorCode, state.LastErrorMessage}, " · "))
	} else if state.Status == repository.LegacyProjectionMigrationCompleted && state.SourceRetireAfter != nil {
		if now.Before(*state.SourceRetireAfter) {
			message = "迁移完成；旧表只读观察至 " + state.SourceRetireAfter.Format("2006-01-02 15:04")
		} else {
			message = "迁移完成；旧表已达到人工废弃评审时间"
		}
	} else if state.CursorUpdatedAt != nil {
		message = "当前游标 " + state.CursorUpdatedAt.Format("2006-01-02 15:04:05") + " / " + state.CursorID
	}
	return UnifiedTask{
		ID:          TaskKindLegacyProjectionMigration + ":" + state.Source,
		Kind:        TaskKindLegacyProjectionMigration,
		Status:      status,
		Title:       "旧转码历史登记",
		Subtitle:    subtitle,
		Message:     message,
		Progress:    progress,
		SourceID:    state.Source,
		CreatedAt:   timePtr(state.CreatedAt),
		UpdatedAt:   timePtr(state.UpdatedAt),
		StartedAt:   state.LastBatchStartedAt,
		CompletedAt: state.CompletedAt,
	}
}

func artifactCleanupToUnifiedTask(artifact *model.TranscodeArtifactRecord) UnifiedTask {
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
		ID:            kind + ":" + artifact.ID,
		Kind:          kind,
		Status:        status,
		Title:         title,
		Subtitle:      strings.Join(subtitleParts, " · "),
		Message:       strings.Join(messageParts, " · "),
		Progress:      0,
		SourceID:      artifact.ID,
		CreatedAt:     timePtr(artifact.CreatedAt),
		UpdatedAt:     timePtr(artifact.UpdatedAt),
		StartedAt:     artifact.CleanupClaimedAt,
		RollbackUntil: artifact.CleanupRollbackUntil,
	}
}

func cleanupStateLabel(state string) string {
	switch state {
	case repository.ArtifactCleanupPending:
		return "等待清理"
	case repository.ArtifactCleanupClaimed:
		return "正在清理"
	case repository.ArtifactCleanupRetryWait:
		return "等待重试"
	case repository.ArtifactCleanupBlocked:
		return "已阻断"
	default:
		return "清理状态未知"
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
