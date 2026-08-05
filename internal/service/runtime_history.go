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

const runtimeHistoryTextLimit = 2048

type RuntimeHistoryRetentionPolicy struct {
	MetadataMode           string   `json:"metadata_mode"`
	AutomaticMetadataPrune bool     `json:"automatic_metadata_prune"`
	ArtifactContent        string   `json:"artifact_content"`
	CleanupEvidence        string   `json:"cleanup_evidence"`
	SensitiveFieldsHidden  []string `json:"sensitive_fields_hidden"`
}

type RuntimeHistoryQuery struct {
	Page     int
	PageSize int
	Status   string
	Intent   string
	MediaID  string
	Search   string
	From     *time.Time
	To       *time.Time
}

type RuntimeHistoryItem struct {
	ID                string     `json:"id"`
	LegacyTaskID      *string    `json:"legacy_task_id,omitempty"`
	MediaID           string     `json:"media_id"`
	MediaTitle        string     `json:"media_title,omitempty"`
	Intent            string     `json:"intent"`
	ProfileID         string     `json:"profile_id,omitempty"`
	Status            string     `json:"status"`
	DesiredState      string     `json:"desired_state,omitempty"`
	Priority          int        `json:"priority"`
	StartMS           int64      `json:"start_ms"`
	DurationMS        int64      `json:"duration_ms"`
	SessionID         string     `json:"session_id,omitempty"`
	PlannerVersion    string     `json:"planner_version,omitempty"`
	EncodingPlanHash  string     `json:"encoding_plan_hash,omitempty"`
	TimestampPlanHash string     `json:"timestamp_plan_hash,omitempty"`
	TimelineOriginMS  int64      `json:"timeline_origin_ms"`
	AttemptCount      int        `json:"attempt_count"`
	ArtifactCount     int        `json:"artifact_count"`
	ArtifactBytes     int64      `json:"artifact_bytes"`
	LastBackend       string     `json:"last_backend,omitempty"`
	LastErrorCode     string     `json:"last_error_code,omitempty"`
	LastErrorMessage  string     `json:"last_error_message,omitempty"`
	IntegrityState    string     `json:"integrity_state"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	ClaimedAt         *time.Time `json:"claimed_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
}

type RuntimeHistoryAttempt struct {
	ID           string     `json:"id"`
	Number       int        `json:"number"`
	Backend      string     `json:"backend,omitempty"`
	Status       string     `json:"status"`
	ExitCode     int        `json:"exit_code"`
	Signal       string     `json:"signal,omitempty"`
	ErrorCode    string     `json:"error_code,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	StderrTail   string     `json:"stderr_tail,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type RuntimeHistoryArtifact struct {
	ID                  string     `json:"id"`
	AttemptID           string     `json:"attempt_id,omitempty"`
	Kind                string     `json:"kind"`
	ProfileID           string     `json:"profile_id,omitempty"`
	Status              string     `json:"status"`
	SizeBytes           int64      `json:"size_bytes"`
	DurationMS          int64      `json:"duration_ms"`
	AttestationStatus   string     `json:"attestation_status,omitempty"`
	AttestationHash     string     `json:"attestation_hash,omitempty"`
	ErrorCode           string     `json:"error_code,omitempty"`
	ErrorMessage        string     `json:"error_message,omitempty"`
	CleanupState        string     `json:"cleanup_state,omitempty"`
	CleanupAttempts     int        `json:"cleanup_attempts"`
	CleanupErrorCode    string     `json:"cleanup_error_code,omitempty"`
	CleanupErrorMessage string     `json:"cleanup_error_message,omitempty"`
	PublishedAt         *time.Time `json:"published_at,omitempty"`
	ExpiresAt           *time.Time `json:"expires_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

type RuntimeHistoryList struct {
	Items      []RuntimeHistoryItem          `json:"items"`
	Total      int64                         `json:"total"`
	Page       int                           `json:"page"`
	PageSize   int                           `json:"page_size"`
	TotalPages int                           `json:"total_pages"`
	Generated  time.Time                     `json:"generated_at"`
	Retention  RuntimeHistoryRetentionPolicy `json:"retention"`
}

type RuntimeHistoryDetail struct {
	Job       RuntimeHistoryItem            `json:"job"`
	Attempts  []RuntimeHistoryAttempt       `json:"attempts"`
	Artifacts []RuntimeHistoryArtifact      `json:"artifacts"`
	Retention RuntimeHistoryRetentionPolicy `json:"retention"`
}

type RuntimeHistorySummary struct {
	Jobs              int64                         `json:"jobs"`
	Attempts          int64                         `json:"attempts"`
	Artifacts         int64                         `json:"artifacts"`
	LegacyTasks       int64                         `json:"legacy_tasks"`
	OrphanLegacyTasks int64                         `json:"orphan_legacy_tasks"`
	ArtifactBytes     int64                         `json:"artifact_bytes"`
	ByStatus          map[string]int64              `json:"by_status"`
	OldestAt          *time.Time                    `json:"oldest_at,omitempty"`
	NewestAt          *time.Time                    `json:"newest_at,omitempty"`
	Generated         time.Time                     `json:"generated_at"`
	Retention         RuntimeHistoryRetentionPolicy `json:"retention"`
}

// RuntimeHistoryService is a read model only. It has no methods for submit,
// retry, cancel, claim, lease, process control, artifact publication or playback.
type RuntimeHistoryService struct {
	repo   *repository.RuntimeHistoryRepo
	logger *zap.SugaredLogger
}

func NewRuntimeHistoryService(repo *repository.RuntimeHistoryRepo, logger *zap.SugaredLogger) *RuntimeHistoryService {
	if repo == nil {
		panic("runtime history repository is required")
	}
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &RuntimeHistoryService{repo: repo, logger: logger}
}

func RuntimeHistoryRetention() RuntimeHistoryRetentionPolicy {
	return RuntimeHistoryRetentionPolicy{
		MetadataMode:           "indefinite_audit_history",
		AutomaticMetadataPrune: false,
		ArtifactContent:        "bounded_by_artifact_maintenance",
		CleanupEvidence:        "retained_until_cleanup_succeeds_or_operator_resolves",
		SensitiveFieldsHidden: []string{
			"command_json",
			"workspace_path",
			"artifact_path",
			"temporary_path",
			"manifest_path",
		},
	}
}

func (s *RuntimeHistoryService) List(query RuntimeHistoryQuery) (*RuntimeHistoryList, error) {
	page, pageSize := normalizeRuntimeHistoryPage(query.Page, query.PageSize)
	jobs, total, err := s.repo.ListJobs(repository.RuntimeHistoryFilter{
		Page: page, PageSize: pageSize, Status: query.Status, Intent: query.Intent,
		MediaID: query.MediaID, Search: query.Search, From: query.From, To: query.To,
	})
	if err != nil {
		return nil, fmt.Errorf("list runtime history jobs: %w", err)
	}
	jobIDs, mediaIDs := runtimeHistoryIDs(jobs)
	attempts, err := s.repo.ListAttempts(jobIDs)
	if err != nil {
		return nil, fmt.Errorf("list runtime history attempts: %w", err)
	}
	artifacts, err := s.repo.ListArtifacts(jobIDs)
	if err != nil {
		return nil, fmt.Errorf("list runtime history artifacts: %w", err)
	}
	titles, err := s.repo.MediaTitles(mediaIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve runtime history media titles: %w", err)
	}
	items := buildRuntimeHistoryItems(jobs, attempts, artifacts, titles)
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return &RuntimeHistoryList{
		Items: items, Total: total, Page: page, PageSize: pageSize,
		TotalPages: totalPages, Generated: time.Now(), Retention: RuntimeHistoryRetention(),
	}, nil
}

func (s *RuntimeHistoryService) Detail(jobID string) (*RuntimeHistoryDetail, error) {
	job, err := s.repo.FindJob(strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	attempts, err := s.repo.ListAttempts([]string{job.ID})
	if err != nil {
		return nil, fmt.Errorf("list runtime history attempts: %w", err)
	}
	artifacts, err := s.repo.ListArtifacts([]string{job.ID})
	if err != nil {
		return nil, fmt.Errorf("list runtime history artifacts: %w", err)
	}
	titles, err := s.repo.MediaTitles([]string{job.MediaID})
	if err != nil {
		return nil, fmt.Errorf("resolve runtime history media title: %w", err)
	}
	items := buildRuntimeHistoryItems([]model.TranscodeJobRecord{*job}, attempts, artifacts, titles)
	return &RuntimeHistoryDetail{
		Job:       items[0],
		Attempts:  mapRuntimeHistoryAttempts(attempts),
		Artifacts: mapRuntimeHistoryArtifacts(artifacts),
		Retention: RuntimeHistoryRetention(),
	}, nil
}

func (s *RuntimeHistoryService) Summary() (*RuntimeHistorySummary, error) {
	counts, err := s.repo.Counts()
	if err != nil {
		return nil, fmt.Errorf("summarize runtime history: %w", err)
	}
	return &RuntimeHistorySummary{
		Jobs: counts.Jobs, Attempts: counts.Attempts, Artifacts: counts.Artifacts,
		LegacyTasks: counts.LegacyTasks, OrphanLegacyTasks: counts.OrphanLegacyTasks,
		ArtifactBytes: counts.ArtifactBytes, ByStatus: counts.ByStatus,
		OldestAt: counts.OldestAt, NewestAt: counts.NewestAt,
		Generated: time.Now(), Retention: RuntimeHistoryRetention(),
	}, nil
}

func normalizeRuntimeHistoryPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func runtimeHistoryIDs(jobs []model.TranscodeJobRecord) ([]string, []string) {
	jobIDs := make([]string, 0, len(jobs))
	mediaIDs := make([]string, 0, len(jobs))
	seenMedia := make(map[string]struct{})
	for _, job := range jobs {
		jobIDs = append(jobIDs, job.ID)
		if job.MediaID != "" {
			if _, ok := seenMedia[job.MediaID]; !ok {
				seenMedia[job.MediaID] = struct{}{}
				mediaIDs = append(mediaIDs, job.MediaID)
			}
		}
	}
	return jobIDs, mediaIDs
}

func buildRuntimeHistoryItems(
	jobs []model.TranscodeJobRecord,
	attempts []model.TranscodeAttemptRecord,
	artifacts []model.TranscodeArtifactRecord,
	titles map[string]string,
) []RuntimeHistoryItem {
	attemptsByJob := make(map[string][]model.TranscodeAttemptRecord)
	for _, attempt := range attempts {
		attemptsByJob[attempt.JobID] = append(attemptsByJob[attempt.JobID], attempt)
	}
	artifactsByJob := make(map[string][]model.TranscodeArtifactRecord)
	for _, artifact := range artifacts {
		artifactsByJob[artifact.JobID] = append(artifactsByJob[artifact.JobID], artifact)
	}
	items := make([]RuntimeHistoryItem, 0, len(jobs))
	for _, job := range jobs {
		jobAttempts := attemptsByJob[job.ID]
		jobArtifacts := artifactsByJob[job.ID]
		sort.SliceStable(jobAttempts, func(i, j int) bool {
			if jobAttempts[i].Number == jobAttempts[j].Number {
				return jobAttempts[i].CreatedAt.Before(jobAttempts[j].CreatedAt)
			}
			return jobAttempts[i].Number < jobAttempts[j].Number
		})
		var latest *model.TranscodeAttemptRecord
		if len(jobAttempts) > 0 {
			latest = &jobAttempts[len(jobAttempts)-1]
		}
		var artifactBytes int64
		for _, artifact := range jobArtifacts {
			artifactBytes += artifact.SizeBytes
		}
		item := RuntimeHistoryItem{
			ID: job.ID, LegacyTaskID: job.LegacyTaskID, MediaID: job.MediaID,
			MediaTitle: titles[job.MediaID], Intent: job.Intent, ProfileID: job.ProfileID,
			Status: job.Status, DesiredState: job.DesiredState, Priority: job.Priority,
			StartMS: job.StartMS, DurationMS: job.DurationMS, SessionID: job.SessionID,
			PlannerVersion: job.PlannerVersion, EncodingPlanHash: job.EncodingPlanHash,
			TimestampPlanHash: job.TimestampPlanHash, TimelineOriginMS: job.TimelineOriginMS,
			AttemptCount: len(jobAttempts), ArtifactCount: len(jobArtifacts), ArtifactBytes: artifactBytes,
			IntegrityState: runtimeHistoryIntegrity(job), CreatedAt: job.CreatedAt,
			UpdatedAt: job.UpdatedAt, ClaimedAt: job.ClaimedAt, CompletedAt: job.CompletedAt,
		}
		if latest != nil {
			item.LastBackend = latest.Backend
			item.LastErrorCode = latest.ErrorCode
			item.LastErrorMessage = truncateRuntimeHistoryText(latest.ErrorMessage)
		}
		items = append(items, item)
	}
	return items
}

func runtimeHistoryIntegrity(job model.TranscodeJobRecord) string {
	if job.ActiveKey != nil || job.LeaseToken != "" || job.Status == "queued" || job.Status == "claimed" || job.Status == "running" || job.Status == "cancel_requested" {
		return "active_residual"
	}
	if job.LegacyTaskID != nil && *job.LegacyTaskID != "" {
		return "legacy_projection_linked"
	}
	return "execution_record_only"
}

func mapRuntimeHistoryAttempts(rows []model.TranscodeAttemptRecord) []RuntimeHistoryAttempt {
	result := make([]RuntimeHistoryAttempt, 0, len(rows))
	for _, row := range rows {
		result = append(result, RuntimeHistoryAttempt{
			ID: row.ID, Number: row.Number, Backend: row.Backend, Status: row.Status,
			ExitCode: row.ExitCode, Signal: row.Signal, ErrorCode: row.ErrorCode,
			ErrorMessage: truncateRuntimeHistoryText(row.ErrorMessage),
			StderrTail:   truncateRuntimeHistoryText(row.StderrTail),
			StartedAt:    row.StartedAt, CompletedAt: row.CompletedAt, CreatedAt: row.CreatedAt,
		})
	}
	return result
}

func mapRuntimeHistoryArtifacts(rows []model.TranscodeArtifactRecord) []RuntimeHistoryArtifact {
	result := make([]RuntimeHistoryArtifact, 0, len(rows))
	for _, row := range rows {
		result = append(result, RuntimeHistoryArtifact{
			ID: row.ID, AttemptID: row.AttemptID, Kind: row.Kind, ProfileID: row.ProfileID,
			Status: row.Status, SizeBytes: row.SizeBytes, DurationMS: row.DurationMS,
			AttestationStatus: row.AttestationStatus, AttestationHash: row.AttestationHash,
			ErrorCode: row.ErrorCode, ErrorMessage: truncateRuntimeHistoryText(row.ErrorMessage),
			CleanupState: row.CleanupState, CleanupAttempts: row.CleanupAttempts,
			CleanupErrorCode:    row.CleanupErrorCode,
			CleanupErrorMessage: truncateRuntimeHistoryText(row.CleanupErrorMessage),
			PublishedAt:         row.PublishedAt, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
		})
	}
	return result
}

func truncateRuntimeHistoryText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= runtimeHistoryTextLimit {
		return value
	}
	return value[:runtimeHistoryTextLimit] + "…"
}
