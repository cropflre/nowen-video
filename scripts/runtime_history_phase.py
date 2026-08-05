#!/usr/bin/env python3
from __future__ import annotations

import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def write(path: str, content: str) -> None:
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")


def replace_once(path: str, old: str, new: str) -> None:
    content = read(path)
    count = content.count(old)
    if count != 1:
        raise RuntimeError(f"{path}: expected exactly one marker, found {count}: {old[:120]!r}")
    write(path, content.replace(old, new, 1))


write(
    "internal/repository/runtime_history.go",
    r'''package repository

import (
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// RuntimeHistoryFilter is a read-only query contract over retired persistent
// Runtime execution metadata. It never participates in scheduling or playback.
type RuntimeHistoryFilter struct {
	Page     int
	PageSize int
	Status   string
	Intent   string
	MediaID  string
	Search   string
	From     *time.Time
	To       *time.Time
}

type RuntimeHistoryCounts struct {
	Jobs              int64
	Attempts          int64
	Artifacts         int64
	LegacyTasks       int64
	OrphanLegacyTasks int64
	ArtifactBytes     int64
	ByStatus          map[string]int64
	OldestAt          *time.Time
	NewestAt          *time.Time
}

type RuntimeHistoryRepo struct {
	db *gorm.DB
}

func NewRuntimeHistoryRepo(db *gorm.DB) *RuntimeHistoryRepo {
	return &RuntimeHistoryRepo{db: db}
}

func (r *RuntimeHistoryRepo) ListJobs(filter RuntimeHistoryFilter) ([]model.TranscodeJobRecord, int64, error) {
	query := r.db.Model(&model.TranscodeJobRecord{})
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(filter.Intent); value != "" {
		query = query.Where("intent = ?", value)
	}
	if value := strings.TrimSpace(filter.MediaID); value != "" {
		query = query.Where("media_id = ?", value)
	}
	if filter.From != nil {
		query = query.Where("created_at >= ?", *filter.From)
	}
	if filter.To != nil {
		query = query.Where("created_at <= ?", *filter.To)
	}
	if value := strings.TrimSpace(filter.Search); value != "" {
		like := "%" + value + "%"
		query = query.Where(
			"id LIKE ? OR media_id LIKE ? OR intent LIKE ? OR profile_id LIKE ? OR worker_id LIKE ? OR session_id LIKE ?",
			like, like, like, like, like, like,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 25
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var jobs []model.TranscodeJobRecord
	err := query.
		Order("COALESCE(completed_at, updated_at) DESC, created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&jobs).Error
	return jobs, total, err
}

func (r *RuntimeHistoryRepo) FindJob(id string) (*model.TranscodeJobRecord, error) {
	var job model.TranscodeJobRecord
	if err := r.db.First(&job, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *RuntimeHistoryRepo) ListAttempts(jobIDs []string) ([]model.TranscodeAttemptRecord, error) {
	if len(jobIDs) == 0 {
		return []model.TranscodeAttemptRecord{}, nil
	}
	var rows []model.TranscodeAttemptRecord
	err := r.db.Where("job_id IN ?", jobIDs).Order("job_id ASC, number ASC, created_at ASC").Find(&rows).Error
	return rows, err
}

func (r *RuntimeHistoryRepo) ListArtifacts(jobIDs []string) ([]model.TranscodeArtifactRecord, error) {
	if len(jobIDs) == 0 {
		return []model.TranscodeArtifactRecord{}, nil
	}
	var rows []model.TranscodeArtifactRecord
	err := r.db.Where("job_id IN ?", jobIDs).Order("job_id ASC, created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *RuntimeHistoryRepo) MediaTitles(mediaIDs []string) (map[string]string, error) {
	result := make(map[string]string)
	if len(mediaIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		ID    string
		Title string
	}
	if err := r.db.Model(&model.Media{}).Select("id", "title").Where("id IN ?", mediaIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row.Title
	}
	return result, nil
}

func (r *RuntimeHistoryRepo) Counts() (*RuntimeHistoryCounts, error) {
	counts := &RuntimeHistoryCounts{ByStatus: make(map[string]int64)}
	if err := r.db.Model(&model.TranscodeJobRecord{}).Count(&counts.Jobs).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&model.TranscodeAttemptRecord{}).Count(&counts.Attempts).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&model.TranscodeArtifactRecord{}).Count(&counts.Artifacts).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&model.TranscodeArtifactRecord{}).Select("COALESCE(SUM(size_bytes), 0)").Scan(&counts.ArtifactBytes).Error; err != nil {
		return nil, err
	}
	if r.db.Migrator().HasTable(&model.TranscodeTask{}) {
		if err := r.db.Model(&model.TranscodeTask{}).Count(&counts.LegacyTasks).Error; err != nil {
			return nil, err
		}
		if err := r.db.Table("transcode_tasks AS legacy").
			Joins("LEFT JOIN transcode_jobs AS jobs ON jobs.legacy_task_id = legacy.id").
			Where("jobs.id IS NULL").
			Count(&counts.OrphanLegacyTasks).Error; err != nil {
			return nil, err
		}
	}

	var statusRows []struct {
		Status string
		Count  int64
	}
	if err := r.db.Model(&model.TranscodeJobRecord{}).
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&statusRows).Error; err != nil {
		return nil, err
	}
	for _, row := range statusRows {
		counts.ByStatus[row.Status] = row.Count
	}

	var bounds struct {
		OldestAt *time.Time
		NewestAt *time.Time
	}
	if err := r.db.Model(&model.TranscodeJobRecord{}).
		Select("MIN(created_at) AS oldest_at, MAX(COALESCE(completed_at, updated_at)) AS newest_at").
		Scan(&bounds).Error; err != nil {
		return nil, err
	}
	counts.OldestAt = bounds.OldestAt
	counts.NewestAt = bounds.NewestAt
	return counts, nil
}
''',
)

write(
    "internal/service/runtime_history.go",
    r'''package service

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
	ID                   string     `json:"id"`
	AttemptID            string     `json:"attempt_id,omitempty"`
	Kind                 string     `json:"kind"`
	ProfileID            string     `json:"profile_id,omitempty"`
	Status               string     `json:"status"`
	SizeBytes            int64      `json:"size_bytes"`
	DurationMS           int64      `json:"duration_ms"`
	AttestationStatus    string     `json:"attestation_status,omitempty"`
	AttestationHash      string     `json:"attestation_hash,omitempty"`
	ErrorCode            string     `json:"error_code,omitempty"`
	ErrorMessage         string     `json:"error_message,omitempty"`
	CleanupState         string     `json:"cleanup_state,omitempty"`
	CleanupAttempts      int        `json:"cleanup_attempts"`
	CleanupErrorCode     string     `json:"cleanup_error_code,omitempty"`
	CleanupErrorMessage  string     `json:"cleanup_error_message,omitempty"`
	PublishedAt          *time.Time `json:"published_at,omitempty"`
	ExpiresAt            *time.Time `json:"expires_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
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
	Job       RuntimeHistoryItem       `json:"job"`
	Attempts  []RuntimeHistoryAttempt  `json:"attempts"`
	Artifacts []RuntimeHistoryArtifact `json:"artifacts"`
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
		Job: items[0],
		Attempts: mapRuntimeHistoryAttempts(attempts),
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
			StderrTail: truncateRuntimeHistoryText(row.StderrTail),
			StartedAt: row.StartedAt, CompletedAt: row.CompletedAt, CreatedAt: row.CreatedAt,
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
			CleanupErrorCode: row.CleanupErrorCode,
			CleanupErrorMessage: truncateRuntimeHistoryText(row.CleanupErrorMessage),
			PublishedAt: row.PublishedAt, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
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
''',
)

write(
    "internal/handler/runtime_history.go",
    r'''package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RuntimeHistoryHandler struct {
	service *service.RuntimeHistoryService
	logger  *zap.SugaredLogger
}

func NewRuntimeHistoryHandler(history *service.RuntimeHistoryService, logger *zap.SugaredLogger) *RuntimeHistoryHandler {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &RuntimeHistoryHandler{service: history, logger: logger}
}

func (h *RuntimeHistoryHandler) List(c *gin.Context) {
	query, err := runtimeHistoryQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.service.List(query)
	if err != nil {
		h.logger.Errorf("读取 Runtime 历史失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取运行历史失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *RuntimeHistoryHandler) Summary(c *gin.Context) {
	result, err := h.service.Summary()
	if err != nil {
		h.logger.Errorf("汇总 Runtime 历史失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取运行历史汇总失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *RuntimeHistoryHandler) Detail(c *gin.Context) {
	result, err := h.service.Detail(c.Param("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "运行历史记录不存在"})
			return
		}
		h.logger.Errorf("读取 Runtime 历史详情失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取运行历史详情失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func runtimeHistoryQuery(c *gin.Context) (service.RuntimeHistoryQuery, error) {
	page, err := positiveQueryInt(c.Query("page"), 1)
	if err != nil {
		return service.RuntimeHistoryQuery{}, err
	}
	pageSize, err := positiveQueryInt(c.Query("page_size"), 25)
	if err != nil {
		return service.RuntimeHistoryQuery{}, err
	}
	if pageSize > 100 {
		pageSize = 100
	}
	from, err := optionalHistoryTime(c.Query("from"))
	if err != nil {
		return service.RuntimeHistoryQuery{}, err
	}
	to, err := optionalHistoryTime(c.Query("to"))
	if err != nil {
		return service.RuntimeHistoryQuery{}, err
	}
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 128 {
		return service.RuntimeHistoryQuery{}, errors.New("search 最多 128 个字符")
	}
	return service.RuntimeHistoryQuery{
		Page: page, PageSize: pageSize, Status: strings.TrimSpace(c.Query("status")),
		Intent: strings.TrimSpace(c.Query("intent")), MediaID: strings.TrimSpace(c.Query("media_id")),
		Search: search, From: from, To: to,
	}, nil
}

func positiveQueryInt(value string, fallback int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, errors.New("分页参数必须是正整数")
	}
	return parsed, nil
}

func optionalHistoryTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, errors.New("时间参数必须是 RFC3339 或 YYYY-MM-DD")
}
''',
)

write(
    "internal/service/runtime_history_test.go",
    r'''package service

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRuntimeHistoryIsReadOnlyPagedAndRedacted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:runtime-history?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Media{}, &model.TranscodeTask{}); err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	media := model.Media{ID: "media-history", Title: "历史影片", FilePath: "/private/media/history.mkv", MediaType: "movie"}
	if err := db.Create(&media).Error; err != nil {
		t.Fatal(err)
	}
	legacy := model.TranscodeTask{ID: "legacy-history", MediaID: media.ID, Status: "cancelled", Quality: "720p", CreatedAt: now.Add(-time.Hour)}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	legacyID := legacy.ID
	completed := now.Add(-30 * time.Minute)
	job := model.TranscodeJobRecord{
		ID: "job-history", LegacyTaskID: &legacyID, MediaID: media.ID,
		Intent: "retired_runtime_playback", ProfileID: "720p", Status: "cancelled",
		DesiredState: "cancelled", EncodingPlanHash: "encoding-hash",
		TimestampPlanHash: "timestamp-hash", CreatedAt: now.Add(-time.Hour),
		UpdatedAt: completed, CompletedAt: &completed,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	attempt := model.TranscodeAttemptRecord{
		ID: "attempt-history", JobID: job.ID, Number: 1, Backend: "nvenc",
		Status: "failed", CommandJSON: `["ffmpeg","-i","secret"]`,
		WorkspacePath: "/private/workspace", ErrorCode: "retired_runtime",
		ErrorMessage: "runtime retired", StderrTail: "diagnostic tail",
		ExitCode: 1, CreatedAt: now.Add(-50 * time.Minute), UpdatedAt: completed,
		CompletedAt: &completed,
	}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	artifact := model.TranscodeArtifactRecord{
		ID: "artifact-history", JobID: job.ID, AttemptID: attempt.ID, MediaID: media.ID,
		Kind: "hls_variant", ProfileID: "720p", Status: "retired",
		Path: "/private/artifact", TempPath: "/private/temp", ManifestPath: "/private/manifest.m3u8",
		SizeBytes: 4096, CreatedAt: now.Add(-45 * time.Minute), UpdatedAt: completed,
	}
	if err := db.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}

	history := NewRuntimeHistoryService(repository.NewRuntimeHistoryRepo(db), zap.NewNop().Sugar())
	list, err := history.List(RuntimeHistoryQuery{Page: 1, PageSize: 10, Search: "media-history"})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("unexpected history list: %+v", list)
	}
	item := list.Items[0]
	if item.MediaTitle != media.Title || item.AttemptCount != 1 || item.ArtifactCount != 1 || item.ArtifactBytes != 4096 {
		t.Fatalf("history projection incomplete: %+v", item)
	}
	if item.IntegrityState != "legacy_projection_linked" || item.LastBackend != "nvenc" || item.LastErrorCode != "retired_runtime" {
		t.Fatalf("history integrity evidence missing: %+v", item)
	}
	if list.Retention.AutomaticMetadataPrune || list.Retention.MetadataMode != "indefinite_audit_history" {
		t.Fatalf("unsafe retention policy: %+v", list.Retention)
	}

	detail, err := history.Detail(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Attempts) != 1 || len(detail.Artifacts) != 1 {
		t.Fatalf("history detail incomplete: %+v", detail)
	}
	if detail.Attempts[0].StderrTail != "diagnostic tail" {
		t.Fatalf("diagnostic evidence missing: %+v", detail.Attempts[0])
	}

	summary, err := history.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Jobs != 1 || summary.Attempts != 1 || summary.Artifacts != 1 || summary.LegacyTasks != 1 || summary.OrphanLegacyTasks != 0 {
		t.Fatalf("unexpected history summary: %+v", summary)
	}
	if summary.ArtifactBytes != 4096 || summary.ByStatus["cancelled"] != 1 {
		t.Fatalf("history summary evidence missing: %+v", summary)
	}
}
''',
)

write(
    "cmd/server/runtime_history_contract_test.go",
    r'''package main

import (
	"os"
	"strings"
	"testing"
)

func TestRuntimeHistoryRoutesAreSharedAndReadOnly(t *testing.T) {
	checks := []struct {
		path string
		markers []string
	}{
		{"main.go", []string{
			`admin.GET("/runtime-history", runtimeHistoryHandler.List)`,
			`admin.GET("/runtime-history/summary", runtimeHistoryHandler.Summary)`,
			`admin.GET("/runtime-history/jobs/:id", runtimeHistoryHandler.Detail)`,
		}},
		{"../server-lite/routes_admin.go", []string{
			`admin.GET("/runtime-history", runtimeHistory.List)`,
			`admin.GET("/runtime-history/summary", runtimeHistory.Summary)`,
			`admin.GET("/runtime-history/jobs/:id", runtimeHistory.Detail)`,
		}},
	}
	for _, check := range checks {
		content, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("read %s: %v", check.path, err)
		}
		source := string(content)
		for _, marker := range check.markers {
			if !strings.Contains(source, marker) {
				t.Fatalf("%s missing read-only history route %q", check.path, marker)
			}
		}
		for _, forbidden := range []string{
			`POST("/runtime-history`, `PUT("/runtime-history`, `PATCH("/runtime-history`, `DELETE("/runtime-history`,
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s exposes mutating history route %q", check.path, forbidden)
			}
		}
	}

	serviceSource, err := os.ReadFile("../../internal/service/runtime_history.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"Submit", "ClaimJob", "RenewJobLease", "exec.Command", "CancelJob", "RetryJob"} {
		if strings.Contains(string(serviceSource), forbidden) {
			t.Fatalf("runtime history service regained execution capability %q", forbidden)
		}
	}
}
''',
)

write(
    "web/src/api/runtimeHistory.ts",
    r'''import api from './client'

export interface RuntimeHistoryRetentionPolicy {
  metadata_mode: string
  automatic_metadata_prune: boolean
  artifact_content: string
  cleanup_evidence: string
  sensitive_fields_hidden: string[]
}

export interface RuntimeHistoryItem {
  id: string
  legacy_task_id?: string
  media_id: string
  media_title?: string
  intent: string
  profile_id?: string
  status: string
  desired_state?: string
  priority: number
  start_ms: number
  duration_ms: number
  session_id?: string
  planner_version?: string
  encoding_plan_hash?: string
  timestamp_plan_hash?: string
  timeline_origin_ms: number
  attempt_count: number
  artifact_count: number
  artifact_bytes: number
  last_backend?: string
  last_error_code?: string
  last_error_message?: string
  integrity_state: 'active_residual' | 'legacy_projection_linked' | 'execution_record_only' | string
  created_at: string
  updated_at: string
  claimed_at?: string
  completed_at?: string
}

export interface RuntimeHistoryAttempt {
  id: string
  number: number
  backend?: string
  status: string
  exit_code: number
  signal?: string
  error_code?: string
  error_message?: string
  stderr_tail?: string
  started_at?: string
  completed_at?: string
  created_at: string
}

export interface RuntimeHistoryArtifact {
  id: string
  attempt_id?: string
  kind: string
  profile_id?: string
  status: string
  size_bytes: number
  duration_ms: number
  attestation_status?: string
  attestation_hash?: string
  error_code?: string
  error_message?: string
  cleanup_state?: string
  cleanup_attempts: number
  cleanup_error_code?: string
  cleanup_error_message?: string
  published_at?: string
  expires_at?: string
  created_at: string
}

export interface RuntimeHistoryList {
  items: RuntimeHistoryItem[]
  total: number
  page: number
  page_size: number
  total_pages: number
  generated_at: string
  retention: RuntimeHistoryRetentionPolicy
}

export interface RuntimeHistoryDetail {
  job: RuntimeHistoryItem
  attempts: RuntimeHistoryAttempt[]
  artifacts: RuntimeHistoryArtifact[]
  retention: RuntimeHistoryRetentionPolicy
}

export interface RuntimeHistorySummary {
  jobs: number
  attempts: number
  artifacts: number
  legacy_tasks: number
  orphan_legacy_tasks: number
  artifact_bytes: number
  by_status: Record<string, number>
  oldest_at?: string
  newest_at?: string
  generated_at: string
  retention: RuntimeHistoryRetentionPolicy
}

export const runtimeHistoryApi = {
  list: (params?: {
    page?: number
    page_size?: number
    status?: string
    intent?: string
    media_id?: string
    search?: string
    from?: string
    to?: string
  }) => api.get<{ data: RuntimeHistoryList }>('/admin/runtime-history', { params }),
  summary: () => api.get<{ data: RuntimeHistorySummary }>('/admin/runtime-history/summary'),
  detail: (id: string) => api.get<{ data: RuntimeHistoryDetail }>(`/admin/runtime-history/jobs/${encodeURIComponent(id)}`),
}
''',
)

write(
    "web/src/components/RuntimeHistoryButton.tsx",
    r'''import { FormEvent, useCallback, useEffect, useState } from 'react'
import { Archive, ArrowLeft, ChevronLeft, ChevronRight, CircleAlert, Database, HardDrive, Loader2, RefreshCw, Search, X } from 'lucide-react'
import { runtimeHistoryApi } from '@/api'
import type { RuntimeHistoryDetail, RuntimeHistoryItem, RuntimeHistoryList, RuntimeHistorySummary } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useServerProfileStore } from '@/stores/serverProfile'

const PAGE_SIZE = 20

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let current = value
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  return `${current >= 10 || index === 0 ? current.toFixed(0) : current.toFixed(1)} ${units[index]}`
}

function formatDate(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(date)
}

function statusText(status: string) {
  const labels: Record<string, string> = {
    completed: '已完成', failed: '失败', cancelled: '已取消', retired: '已退役',
    queued: '历史排队残留', claimed: '历史 Claim 残留', running: '历史运行残留', cancel_requested: '取消中残留',
  }
  return labels[status] || status || '未知'
}

function statusColor(status: string) {
  if (status === 'completed') return '#16A34A'
  if (status === 'failed') return '#DC2626'
  if (status === 'cancelled' || status === 'retired') return 'var(--text-muted)'
  return '#CA8A04'
}

function requestError(error: unknown, fallback: string) {
  if (typeof error === 'object' && error !== null) {
    const response = (error as { response?: { data?: { error?: unknown } } }).response
    if (typeof response?.data?.error === 'string') return response.data.error
  }
  return error instanceof Error ? error.message : fallback
}

function HistoryCard({ item, onOpen }: { item: RuntimeHistoryItem; onOpen: (id: string) => void }) {
  const residual = item.integrity_state === 'active_residual'
  return (
    <button
      type="button"
      onClick={() => onOpen(item.id)}
      className="w-full rounded-xl border p-3 text-left transition-colors hover:bg-[var(--nav-hover-bg)]"
      style={{ borderColor: residual ? 'rgba(202,138,4,.35)' : 'var(--border-default)', background: 'var(--card-bg)' }}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{item.media_title || item.media_id || item.id}</p>
          <p className="mt-1 truncate text-xs" style={{ color: 'var(--text-tertiary)' }}>
            {item.intent || '历史执行'}{item.profile_id ? ` · ${item.profile_id}` : ''}{item.last_backend ? ` · ${item.last_backend}` : ''}
          </p>
        </div>
        <span className="shrink-0 text-xs font-medium" style={{ color: statusColor(item.status) }}>{statusText(item.status)}</span>
      </div>
      <div className="mt-3 grid grid-cols-3 gap-2 text-[11px]" style={{ color: 'var(--text-muted)' }}>
        <span>{item.attempt_count} 次尝试</span>
        <span>{item.artifact_count} 个 Artifact</span>
        <span className="text-right">{formatBytes(item.artifact_bytes)}</span>
      </div>
      {(item.last_error_code || item.last_error_message || residual) && (
        <div className="mt-2 flex items-start gap-1.5 text-[11px]" style={{ color: residual ? '#CA8A04' : '#DC2626' }}>
          <CircleAlert size={13} className="mt-0.5 shrink-0" />
          <span className="line-clamp-2 break-all">{residual ? '检测到旧 Runtime 活跃状态残留，维护服务会继续执行退役清扫。' : [item.last_error_code, item.last_error_message].filter(Boolean).join(' · ')}</span>
        </div>
      )}
      <p className="mt-2 text-[11px]" style={{ color: 'var(--text-muted)' }}>{formatDate(item.completed_at || item.updated_at)}</p>
    </button>
  )
}

export default function RuntimeHistoryButton() {
  const user = useAuthStore((state) => state.user)
  const profile = useServerProfileStore((state) => state.manifest?.profile)
  const [open, setOpen] = useState(false)
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState('')
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  const [list, setList] = useState<RuntimeHistoryList | null>(null)
  const [summary, setSummary] = useState<RuntimeHistorySummary | null>(null)
  const [detail, setDetail] = useState<RuntimeHistoryDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const enabled = user?.role === 'admin'

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [listResponse, summaryResponse] = await Promise.all([
        runtimeHistoryApi.list({ page, page_size: PAGE_SIZE, status: status || undefined, search: search || undefined }),
        runtimeHistoryApi.summary(),
      ])
      setList(listResponse.data.data)
      setSummary(summaryResponse.data.data)
    } catch (loadError) {
      setError(requestError(loadError, '无法读取运行历史'))
    } finally {
      setLoading(false)
    }
  }, [page, search, status])

  useEffect(() => {
    if (open && enabled) void load()
  }, [enabled, load, open])

  const openDetail = useCallback(async (id: string) => {
    setDetailLoading(true)
    setError(null)
    try {
      const response = await runtimeHistoryApi.detail(id)
      setDetail(response.data.data)
    } catch (detailError) {
      setError(requestError(detailError, '无法读取运行历史详情'))
    } finally {
      setDetailLoading(false)
    }
  }, [])

  const submitSearch = (event: FormEvent) => {
    event.preventDefault()
    setPage(1)
    setSearch(searchInput.trim())
  }

  if (!enabled) return null

  const rightClass = profile === 'lite' ? 'right-28 md:right-32' : 'right-4 md:right-6'

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className={`fixed top-14 z-40 flex h-10 items-center gap-2 rounded-xl border px-3 text-sm font-medium shadow-lg backdrop-blur ${rightClass}`}
        style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)', color: 'var(--text-secondary)' }}
        aria-label="打开运行历史"
      >
        <Archive size={18} />
        <span className="hidden sm:inline">历史</span>
      </button>

      {open && (
        <div className="fixed inset-0 z-[110]">
          <button type="button" className="absolute inset-0 bg-black/50 backdrop-blur-sm" aria-label="关闭运行历史" onClick={() => setOpen(false)} />
          <aside className="absolute inset-y-0 right-0 flex w-full max-w-2xl flex-col border-l shadow-2xl" style={{ borderColor: 'var(--border-default)', background: 'var(--bg-base)' }}>
            <header className="flex items-center justify-between border-b px-5 py-4" style={{ borderColor: 'var(--border-default)' }}>
              <div className="flex min-w-0 items-center gap-2">
                {detail && <button type="button" onClick={() => setDetail(null)} className="rounded-lg p-1.5 hover:bg-[var(--nav-hover-bg)]" aria-label="返回历史列表"><ArrowLeft size={18} /></button>}
                <Archive size={19} className="text-neon" />
                <div className="min-w-0">
                  <h2 className="font-semibold" style={{ color: 'var(--text-primary)' }}>{detail ? '运行历史详情' : '运行历史'}</h2>
                  <p className="mt-0.5 truncate text-xs" style={{ color: 'var(--text-tertiary)' }}>只读审计域，不提供重试、取消或恢复旧 Runtime 的操作</p>
                </div>
              </div>
              <div className="flex items-center gap-1">
                {!detail && <button type="button" onClick={() => void load()} disabled={loading} className="rounded-lg p-2 hover:bg-[var(--nav-hover-bg)] disabled:opacity-50" aria-label="刷新运行历史"><RefreshCw size={17} className={loading ? 'animate-spin' : ''} /></button>}
                <button type="button" onClick={() => setOpen(false)} className="rounded-lg p-2 hover:bg-[var(--nav-hover-bg)]" aria-label="关闭运行历史"><X size={19} /></button>
              </div>
            </header>

            <div className="flex-1 overflow-y-auto p-4 sm:p-5">
              {error && <div className="mb-4 flex items-center gap-2 rounded-xl border p-3 text-sm" style={{ borderColor: 'rgba(220,38,38,.25)', color: '#DC2626' }}><CircleAlert size={17} />{error}</div>}

              {detailLoading ? (
                <div className="flex min-h-64 items-center justify-center"><Loader2 size={26} className="animate-spin text-neon" /></div>
              ) : detail ? (
                <div className="space-y-5">
                  <section className="rounded-xl border p-4" style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}>
                    <div className="flex items-start justify-between gap-3">
                      <div><h3 className="font-medium" style={{ color: 'var(--text-primary)' }}>{detail.job.media_title || detail.job.media_id}</h3><p className="mt-1 break-all text-xs" style={{ color: 'var(--text-tertiary)' }}>{detail.job.id}</p></div>
                      <span className="text-sm font-medium" style={{ color: statusColor(detail.job.status) }}>{statusText(detail.job.status)}</span>
                    </div>
                    <dl className="mt-4 grid grid-cols-2 gap-x-4 gap-y-3 text-xs sm:grid-cols-3">
                      <div><dt style={{ color: 'var(--text-muted)' }}>Intent</dt><dd className="mt-1 break-all" style={{ color: 'var(--text-secondary)' }}>{detail.job.intent || '—'}</dd></div>
                      <div><dt style={{ color: 'var(--text-muted)' }}>Profile</dt><dd className="mt-1" style={{ color: 'var(--text-secondary)' }}>{detail.job.profile_id || '—'}</dd></div>
                      <div><dt style={{ color: 'var(--text-muted)' }}>完整性</dt><dd className="mt-1" style={{ color: 'var(--text-secondary)' }}>{detail.job.integrity_state}</dd></div>
                      <div><dt style={{ color: 'var(--text-muted)' }}>创建时间</dt><dd className="mt-1" style={{ color: 'var(--text-secondary)' }}>{formatDate(detail.job.created_at)}</dd></div>
                      <div><dt style={{ color: 'var(--text-muted)' }}>结束时间</dt><dd className="mt-1" style={{ color: 'var(--text-secondary)' }}>{formatDate(detail.job.completed_at)}</dd></div>
                      <div><dt style={{ color: 'var(--text-muted)' }}>Artifact 大小</dt><dd className="mt-1" style={{ color: 'var(--text-secondary)' }}>{formatBytes(detail.job.artifact_bytes)}</dd></div>
                    </dl>
                  </section>

                  <section><h3 className="mb-2 text-xs font-semibold uppercase tracking-[.14em]" style={{ color: 'var(--text-tertiary)' }}>Attempts · {detail.attempts.length}</h3><div className="space-y-2">{detail.attempts.length === 0 ? <p className="rounded-xl border p-4 text-sm" style={{ borderColor: 'var(--border-default)', color: 'var(--text-muted)' }}>没有 Attempt 记录</p> : detail.attempts.map((attempt) => <div key={attempt.id} className="rounded-xl border p-3" style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}><div className="flex justify-between text-sm"><span style={{ color: 'var(--text-primary)' }}>#{attempt.number} · {attempt.backend || 'unknown'}</span><span style={{ color: statusColor(attempt.status) }}>{statusText(attempt.status)}</span></div>{(attempt.error_code || attempt.error_message) && <p className="mt-2 break-all text-xs" style={{ color: '#DC2626' }}>{[attempt.error_code, attempt.error_message].filter(Boolean).join(' · ')}</p>}{attempt.stderr_tail && <pre className="mt-2 max-h-36 overflow-auto whitespace-pre-wrap break-all rounded-lg p-2 text-[11px]" style={{ background: 'var(--nav-hover-bg)', color: 'var(--text-tertiary)' }}>{attempt.stderr_tail}</pre>}</div>)}</div></section>

                  <section><h3 className="mb-2 text-xs font-semibold uppercase tracking-[.14em]" style={{ color: 'var(--text-tertiary)' }}>Artifacts · {detail.artifacts.length}</h3><div className="space-y-2">{detail.artifacts.length === 0 ? <p className="rounded-xl border p-4 text-sm" style={{ borderColor: 'var(--border-default)', color: 'var(--text-muted)' }}>没有 Artifact 记录</p> : detail.artifacts.map((artifact) => <div key={artifact.id} className="rounded-xl border p-3" style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}><div className="flex justify-between gap-3 text-sm"><span className="truncate" style={{ color: 'var(--text-primary)' }}>{artifact.kind}{artifact.profile_id ? ` · ${artifact.profile_id}` : ''}</span><span className="shrink-0" style={{ color: statusColor(artifact.status) }}>{statusText(artifact.status)}</span></div><div className="mt-2 flex justify-between text-xs" style={{ color: 'var(--text-muted)' }}><span>{artifact.attestation_status || '无 attestation'}</span><span>{formatBytes(artifact.size_bytes)}</span></div>{artifact.cleanup_error_code && <p className="mt-2 break-all text-xs" style={{ color: '#DC2626' }}>{artifact.cleanup_error_code} · {artifact.cleanup_error_message}</p>}</div>)}</div></section>
                </div>
              ) : (
                <div className="space-y-4">
                  {summary && (
                    <section className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                      <div className="rounded-xl border p-3" style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}><Database size={16} className="text-neon" /><p className="mt-2 text-xl font-semibold" style={{ color: 'var(--text-primary)' }}>{summary.jobs}</p><p className="text-[11px]" style={{ color: 'var(--text-muted)' }}>Jobs</p></div>
                      <div className="rounded-xl border p-3" style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}><Archive size={16} /><p className="mt-2 text-xl font-semibold" style={{ color: 'var(--text-primary)' }}>{summary.attempts}</p><p className="text-[11px]" style={{ color: 'var(--text-muted)' }}>Attempts</p></div>
                      <div className="rounded-xl border p-3" style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}><HardDrive size={16} /><p className="mt-2 text-xl font-semibold" style={{ color: 'var(--text-primary)' }}>{summary.artifacts}</p><p className="text-[11px]" style={{ color: 'var(--text-muted)' }}>{formatBytes(summary.artifact_bytes)}</p></div>
                      <div className="rounded-xl border p-3" style={{ borderColor: summary.orphan_legacy_tasks > 0 ? 'rgba(202,138,4,.35)' : 'var(--border-default)', background: 'var(--card-bg)' }}><CircleAlert size={16} /><p className="mt-2 text-xl font-semibold" style={{ color: 'var(--text-primary)' }}>{summary.orphan_legacy_tasks}</p><p className="text-[11px]" style={{ color: 'var(--text-muted)' }}>孤立 Legacy Tasks</p></div>
                    </section>
                  )}

                  <form onSubmit={submitSearch} className="flex gap-2">
                    <div className="relative min-w-0 flex-1"><Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2" style={{ color: 'var(--text-muted)' }} /><input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索媒体 ID、Job ID、Intent…" className="h-10 w-full rounded-xl border bg-transparent pl-9 pr-3 text-sm outline-none" style={{ borderColor: 'var(--border-default)', color: 'var(--text-primary)' }} /></div>
                    <select value={status} onChange={(event) => { setStatus(event.target.value); setPage(1) }} className="h-10 rounded-xl border bg-transparent px-3 text-sm outline-none" style={{ borderColor: 'var(--border-default)', color: 'var(--text-secondary)', background: 'var(--card-bg)' }}><option value="">全部状态</option><option value="completed">已完成</option><option value="failed">失败</option><option value="cancelled">已取消</option><option value="retired">已退役</option></select>
                    <button type="submit" className="h-10 rounded-xl border px-3 text-sm font-medium" style={{ borderColor: 'var(--border-default)', color: 'var(--text-secondary)' }}>查询</button>
                  </form>

                  {list?.retention && <div className="rounded-xl border p-3 text-xs leading-5" style={{ borderColor: 'var(--border-default)', color: 'var(--text-tertiary)' }}>元数据按审计历史长期保留，不自动删除；实际 Artifact 文件仍由 Artifact Maintenance 按磁盘压力和清理状态治理。命令行、工作目录和真实文件路径不会通过此接口返回。</div>}

                  {loading && !list ? <div className="flex min-h-64 items-center justify-center"><Loader2 size={26} className="animate-spin text-neon" /></div> : list?.items.length ? <div className="space-y-2">{list.items.map((item) => <HistoryCard key={item.id} item={item} onOpen={(id) => void openDetail(id)} />)}</div> : <div className="flex min-h-52 flex-col items-center justify-center text-center"><Archive size={34} className="mb-3" style={{ color: 'var(--text-muted)' }} /><p style={{ color: 'var(--text-primary)' }}>没有匹配的运行历史</p></div>}

                  {list && list.total_pages > 1 && <div className="flex items-center justify-between border-t pt-4" style={{ borderColor: 'var(--border-default)' }}><span className="text-xs" style={{ color: 'var(--text-muted)' }}>第 {list.page} / {list.total_pages} 页 · 共 {list.total} 条</span><div className="flex gap-2"><button type="button" onClick={() => setPage((value) => Math.max(1, value - 1))} disabled={page <= 1 || loading} className="rounded-lg border p-2 disabled:opacity-40" style={{ borderColor: 'var(--border-default)' }}><ChevronLeft size={16} /></button><button type="button" onClick={() => setPage((value) => Math.min(list.total_pages, value + 1))} disabled={page >= list.total_pages || loading} className="rounded-lg border p-2 disabled:opacity-40" style={{ borderColor: 'var(--border-default)' }}><ChevronRight size={16} /></button></div></div>}
                </div>
              )}
            </div>
          </aside>
        </div>
      )}
    </>
  )
}
''',
)

write(
    "docs/RUNTIME_HISTORY.md",
    r'''# Runtime History

The persistent Runtime worker has been physically removed. The historical
SQLite tables remain as audit evidence and are exposed through a dedicated
read-only domain.

## Authority

- `transcode_jobs` is the authoritative historical execution record.
- `transcode_attempts` stores backend, exit and diagnostic evidence.
- `transcode_artifacts` stores Artifact lifecycle and cleanup evidence.
- `transcode_tasks` is a legacy compatibility projection only. It is linked by
  `legacy_task_id` and never becomes an execution source again.

## API

All endpoints require an authenticated administrator:

- `GET /api/admin/runtime-history`
- `GET /api/admin/runtime-history/summary`
- `GET /api/admin/runtime-history/jobs/:id`

The API intentionally has no POST, PUT, PATCH or DELETE operation. It cannot
submit, retry, cancel, recover, claim or lease work.

Sensitive fields are omitted from responses: FFmpeg command JSON, workspace
paths, Artifact paths, temporary paths and manifest paths.

## Retention

Execution metadata is retained indefinitely for audit and rollback evidence.
There is no automatic metadata deletion. Artifact file content remains bounded
by `ArtifactMaintenanceService`, disk-pressure reclamation and cleanup retry
state. Cleanup failures stay visible until cleanup succeeds or an operator fixes
the underlying storage problem.
''',
)

# Lite composition and routes.
replace_once(
    "cmd/server-lite/router.go",
    '''\ttaskCenterHandler := handler.NewTaskCenterHandler(taskCenterService, taskActionDispatcher, logger)\n\ttaskCenterHandler.SetAuditService(services.User)\n\tplaybackPlanHandler := handler.NewPlaybackPlanHandler(services.Stream, logger)\n''',
    '''\ttaskCenterHandler := handler.NewTaskCenterHandler(taskCenterService, taskActionDispatcher, logger)\n\ttaskCenterHandler.SetAuditService(services.User)\n\truntimeHistoryHandler := handler.NewRuntimeHistoryHandler(\n\t\tservice.NewRuntimeHistoryService(repository.NewRuntimeHistoryRepo(repos.DB()), logger),\n\t\tlogger,\n\t)\n\tplaybackPlanHandler := handler.NewPlaybackPlanHandler(services.Stream, logger)\n''',
)
replace_once(
    "cmd/server-lite/router.go",
    '''\tregisterAdminAPI(r, cfg, handlers, taskCenterHandler, jwtMiddleware)\n''',
    '''\tregisterAdminAPI(r, cfg, handlers, taskCenterHandler, runtimeHistoryHandler, jwtMiddleware)\n''',
)
replace_once(
    "cmd/server-lite/routes_admin.go",
    '''func registerAdminAPI(r *gin.Engine, cfg *config.Config, handlers *handler.Handlers, taskCenter *handler.TaskCenterHandler, jwtMiddleware gin.HandlerFunc) {\n''',
    '''func registerAdminAPI(r *gin.Engine, cfg *config.Config, handlers *handler.Handlers, taskCenter *handler.TaskCenterHandler, runtimeHistory *handler.RuntimeHistoryHandler, jwtMiddleware gin.HandlerFunc) {\n''',
)
replace_once(
    "cmd/server-lite/routes_admin.go",
    '''\tadmin.GET("/tasks", taskCenter.List)\n''',
    '''\tadmin.GET("/tasks", taskCenter.List)\n\tadmin.GET("/runtime-history", runtimeHistory.List)\n\tadmin.GET("/runtime-history/summary", runtimeHistory.Summary)\n\tadmin.GET("/runtime-history/jobs/:id", runtimeHistory.Detail)\n''',
)

# Full composition and routes.
replace_once(
    "cmd/server/main.go",
    '''\thandlers := handler.NewHandlers(services, repos, cfg, sugar)\n\tplaybackRuntime, err := newFullPlaybackRuntime(cfg, services, repos, sugar)\n''',
    '''\thandlers := handler.NewHandlers(services, repos, cfg, sugar)\n\truntimeHistoryHandler := handler.NewRuntimeHistoryHandler(\n\t\tservice.NewRuntimeHistoryService(repository.NewRuntimeHistoryRepo(repos.DB()), sugar),\n\t\tsugar,\n\t)\n\tplaybackRuntime, err := newFullPlaybackRuntime(cfg, services, repos, sugar)\n''',
)
replace_once(
    "cmd/server/main.go",
    '''\t\tadmin.GET("/system", handlers.Admin.SystemInfo)\n\t\tadmin.GET("/transcode/status", handlers.Admin.RetiredRuntimeTranscode)\n''',
    '''\t\tadmin.GET("/system", handlers.Admin.SystemInfo)\n\t\tadmin.GET("/runtime-history", runtimeHistoryHandler.List)\n\t\tadmin.GET("/runtime-history/summary", runtimeHistoryHandler.Summary)\n\t\tadmin.GET("/runtime-history/jobs/:id", runtimeHistoryHandler.Detail)\n\t\tadmin.GET("/transcode/status", handlers.Admin.RetiredRuntimeTranscode)\n''',
)

# Web exports and global admin access.
replace_once(
    "web/src/api/index.ts",
    '''export { taskCenterApi } from './tasks'\n''',
    '''export { taskCenterApi } from './tasks'\nexport { runtimeHistoryApi } from './runtimeHistory'\nexport type {\n  RuntimeHistoryRetentionPolicy,\n  RuntimeHistoryItem,\n  RuntimeHistoryAttempt,\n  RuntimeHistoryArtifact,\n  RuntimeHistoryList,\n  RuntimeHistoryDetail,\n  RuntimeHistorySummary,\n} from './runtimeHistory'\n''',
)
replace_once(
    "web/src/components/Layout.tsx",
    '''import TaskCenter from './TaskCenter'\n''',
    '''import TaskCenter from './TaskCenter'\nimport RuntimeHistoryButton from './RuntimeHistoryButton'\n''',
)
replace_once(
    "web/src/components/Layout.tsx",
    '''      <TaskCenter />\n''',
    '''      <TaskCenter />\n      <RuntimeHistoryButton />\n''',
)
