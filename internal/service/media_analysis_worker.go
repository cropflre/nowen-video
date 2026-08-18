package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"gorm.io/gorm"
)

const (
	MediaAnalysisModeAuto            = "auto"
	MediaAnalysisModeClientPreferred = "client_preferred"
	MediaAnalysisModeServerOnly      = "server_only"
	MediaAnalysisModeOff             = "off"

	mediaAnalysisExecutionModeKey = "media_analysis.execution_mode"
	remoteWorkerGrace             = 12 * time.Second
	remoteWorkerLease             = 4 * time.Minute
	remoteWorkerTTL               = 90 * time.Second
	remoteDispatchPoll            = 750 * time.Millisecond
	maxRemoteThumbnailBytes       = 512 * 1024
	maxRemotePayloadBytes         = 3 * 1024 * 1024
)

var (
	ErrMediaAnalysisDisabled           = errors.New("media analysis is disabled")
	ErrMediaAnalysisInvalidMode        = errors.New("invalid media analysis execution mode")
	ErrMediaAnalysisWorkerNoTask       = errors.New("no media analysis task available")
	ErrMediaAnalysisWorkerClaim        = errors.New("invalid or expired media analysis worker claim")
	ErrMediaAnalysisFingerprintChanged = errors.New("media fingerprint changed during client analysis")
	ErrMediaAnalysisWorkerResult       = errors.New("invalid media analysis worker result")

	mediaAnalysisWorkerStates sync.Map
)

type MediaAnalysisWorkerHeartbeat struct {
	WorkerID       string   `json:"worker_id"`
	Kind           string   `json:"kind"`
	Name           string   `json:"name"`
	Version        string   `json:"version"`
	Capabilities   []string `json:"capabilities"`
	Network        string   `json:"network"`
	Charging       bool     `json:"charging"`
	BatteryPercent int      `json:"battery_percent"`
}

type MediaAnalysisWorkerView struct {
	MediaAnalysisWorkerHeartbeat
	LastSeen time.Time `json:"last_seen"`
	State    string    `json:"state"`
	TaskID   string    `json:"task_id,omitempty"`
}

type MediaAnalysisWorkerClaimRequest struct {
	MediaAnalysisWorkerHeartbeat
}

type MediaAnalysisWorkerClaim struct {
	TaskID         string    `json:"task_id"`
	ClaimToken     string    `json:"claim_token"`
	MediaID        string    `json:"media_id"`
	Fingerprint    string    `json:"fingerprint"`
	Duration       float64   `json:"duration"`
	StreamURL      string    `json:"stream_url"`
	SampleTimes    []float64 `json:"sample_times"`
	MaxHighlights  int       `json:"max_highlights"`
	EngineVersion  int       `json:"engine_version"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}

type MediaAnalysisWorkerProgress struct {
	ClaimToken string  `json:"claim_token"`
	Stage      string  `json:"stage"`
	Progress   float64 `json:"progress"`
}

type MediaAnalysisWorkerHighlight struct {
	Title           string  `json:"title"`
	StartTime       float64 `json:"start_time"`
	EndTime         float64 `json:"end_time"`
	Score           float64 `json:"score"`
	AnalysisMethod  string  `json:"analysis_method"`
	ThumbnailBase64 string  `json:"thumbnail_base64,omitempty"`
	ThumbnailMime   string  `json:"thumbnail_mime,omitempty"`
}

type MediaAnalysisWorkerComplete struct {
	ClaimToken  string                         `json:"claim_token"`
	Fingerprint string                         `json:"fingerprint"`
	Highlights  []MediaAnalysisWorkerHighlight `json:"highlights"`
}

type MediaAnalysisWorkerFailure struct {
	ClaimToken string `json:"claim_token"`
	Error      string `json:"error"`
}

type mediaAnalysisRemoteTask struct {
	TaskID       string
	MediaID      string
	Fingerprint  string
	CreatedAt    time.Time
	ClaimedBy    string
	ClaimToken   string
	WorkerKind   string
	LeaseUntil   time.Time
}

type mediaAnalysisWorkerState struct {
	mu          sync.Mutex
	settingRepo *repository.SystemSettingRepo
	workers     map[string]MediaAnalysisWorkerView
	remoteTasks map[string]*mediaAnalysisRemoteTask
}

func mediaAnalysisState(s *MediaAnalysisService) *mediaAnalysisWorkerState {
	if value, ok := mediaAnalysisWorkerStates.Load(s); ok {
		return value.(*mediaAnalysisWorkerState)
	}
	created := &mediaAnalysisWorkerState{
		workers:     make(map[string]MediaAnalysisWorkerView),
		remoteTasks: make(map[string]*mediaAnalysisRemoteTask),
	}
	actual, _ := mediaAnalysisWorkerStates.LoadOrStore(s, created)
	return actual.(*mediaAnalysisWorkerState)
}

// AttachMediaAnalysisWorkerSettings 把系统设置仓库挂到独立 Worker 协调层。
// 这样分布式计算可以独立演进，而不侵入稳定的本地 FFmpeg Sparse V2 实现。
func AttachMediaAnalysisWorkerSettings(s *MediaAnalysisService, repo *repository.SystemSettingRepo) {
	if s == nil {
		return
	}
	state := mediaAnalysisState(s)
	state.mu.Lock()
	state.settingRepo = repo
	state.mu.Unlock()
}

func (s *MediaAnalysisService) ExecutionMode() string {
	state := mediaAnalysisState(s)
	state.mu.Lock()
	repo := state.settingRepo
	state.mu.Unlock()
	if repo == nil {
		return MediaAnalysisModeAuto
	}
	value, err := repo.Get(mediaAnalysisExecutionModeKey)
	if err != nil || strings.TrimSpace(value) == "" {
		return MediaAnalysisModeAuto
	}
	value = strings.TrimSpace(strings.ToLower(value))
	if !isValidMediaAnalysisMode(value) {
		return MediaAnalysisModeAuto
	}
	return value
}

func (s *MediaAnalysisService) SetExecutionMode(mode string) error {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if !isValidMediaAnalysisMode(mode) {
		return ErrMediaAnalysisInvalidMode
	}
	state := mediaAnalysisState(s)
	state.mu.Lock()
	repo := state.settingRepo
	state.mu.Unlock()
	if repo == nil {
		return errors.New("system setting repository unavailable")
	}
	return repo.Set(mediaAnalysisExecutionModeKey, mode)
}

func isValidMediaAnalysisMode(mode string) bool {
	switch mode {
	case MediaAnalysisModeAuto, MediaAnalysisModeClientPreferred, MediaAnalysisModeServerOnly, MediaAnalysisModeOff:
		return true
	default:
		return false
	}
}

// AnalyzeHighlightsDistributed 创建统一任务。auto/client_preferred 先交给客户端，
// server_only 直接使用现有 Sparse V2；auto 在短暂等待不到客户端时自动回退服务端。
func (s *MediaAnalysisService) AnalyzeHighlightsDistributed(mediaID string) (*model.AIAnalysisTask, error) {
	if s.ExecutionMode() == MediaAnalysisModeOff {
		return nil, ErrMediaAnalysisDisabled
	}
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}
	if err := s.ensureSupported(media); err != nil {
		return nil, err
	}
	if existing, err := s.taskRepo.FindActiveByMediaAndType(mediaID, mediaHighlightTaskType); err == nil {
		return existing, ErrMediaAnalysisInProgress
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	task := &model.AIAnalysisTask{
		MediaID: mediaID, TaskType: mediaHighlightTaskType,
		Status: "pending", Stage: "queued", Progress: 0,
	}
	if err := s.taskRepo.Create(task); err != nil {
		return nil, err
	}
	go s.dispatchHighlightTask(task.ID, mediaID)
	return task, nil
}

func (s *MediaAnalysisService) HeartbeatWorker(input MediaAnalysisWorkerHeartbeat) MediaAnalysisWorkerView {
	now := time.Now()
	input.WorkerID = strings.TrimSpace(input.WorkerID)
	input.Kind = normalizeWorkerKind(input.Kind)
	input.Name = strings.TrimSpace(input.Name)
	input.Version = strings.TrimSpace(input.Version)
	input.Network = strings.TrimSpace(strings.ToLower(input.Network))
	input.BatteryPercent = clampInt(input.BatteryPercent, 0, 100)

	state := mediaAnalysisState(s)
	state.mu.Lock()
	defer state.mu.Unlock()
	cleanupWorkersLocked(state, now)
	view := MediaAnalysisWorkerView{MediaAnalysisWorkerHeartbeat: input, LastSeen: now, State: "idle"}
	if !workerEligible(input) {
		view.State = "unavailable"
	}
	for _, task := range state.remoteTasks {
		if task.ClaimedBy == input.WorkerID && task.LeaseUntil.After(now) {
			view.State = "busy"
			view.TaskID = task.TaskID
			break
		}
	}
	if input.WorkerID != "" {
		state.workers[input.WorkerID] = view
	}
	return view
}

func (s *MediaAnalysisService) Workers() []MediaAnalysisWorkerView {
	state := mediaAnalysisState(s)
	now := time.Now()
	state.mu.Lock()
	defer state.mu.Unlock()
	cleanupWorkersLocked(state, now)
	items := make([]MediaAnalysisWorkerView, 0, len(state.workers))
	for _, worker := range state.workers {
		items = append(items, worker)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].LastSeen.After(items[j].LastSeen) })
	return items
}

func cleanupWorkersLocked(state *mediaAnalysisWorkerState, now time.Time) {
	for id, worker := range state.workers {
		if now.Sub(worker.LastSeen) > remoteWorkerTTL {
			delete(state.workers, id)
		}
	}
}

func workerEligible(input MediaAnalysisWorkerHeartbeat) bool {
	hasCapability := false
	for _, capability := range input.Capabilities {
		if strings.EqualFold(strings.TrimSpace(capability), "highlight_v1") {
			hasCapability = true
			break
		}
	}
	if !hasCapability {
		return false
	}
	if normalizeWorkerKind(input.Kind) == "android" {
		return strings.EqualFold(input.Network, "wifi") && (input.Charging || input.BatteryPercent >= 40)
	}
	return true
}

func (s *MediaAnalysisService) dispatchHighlightTask(taskID, mediaID string) {
	mode := s.ExecutionMode()
	if mode == MediaAnalysisModeOff {
		if task, err := s.taskRepo.FindByID(taskID); err == nil {
			s.failTask(task, "disabled", ErrMediaAnalysisDisabled)
		}
		return
	}
	if mode == MediaAnalysisModeServerOnly {
		s.runHighlightTask(taskID, mediaID)
		return
	}

	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		if task, taskErr := s.taskRepo.FindByID(taskID); taskErr == nil {
			s.failTask(task, "dispatch", ErrMediaNotFound)
		}
		return
	}
	fingerprint, err := s.mediaFingerprint(media)
	if err != nil {
		if task, taskErr := s.taskRepo.FindByID(taskID); taskErr == nil {
			s.failTask(task, "dispatch", err)
		}
		return
	}

	createdAt := time.Now()
	state := mediaAnalysisState(s)
	state.mu.Lock()
	state.remoteTasks[taskID] = &mediaAnalysisRemoteTask{TaskID: taskID, MediaID: mediaID, Fingerprint: fingerprint, CreatedAt: createdAt}
	state.mu.Unlock()
	if task, taskErr := s.taskRepo.FindByID(taskID); taskErr == nil {
		s.updateTask(task, "waiting_client", 2, "")
	}

	ticker := time.NewTicker(remoteDispatchPoll)
	defer ticker.Stop()
	for range ticker.C {
		task, taskErr := s.taskRepo.FindByID(taskID)
		if taskErr != nil {
			s.dropRemoteTask(taskID)
			return
		}
		if task.Status == "completed" || task.Status == "failed" || task.Status == "interrupted" {
			s.dropRemoteTask(taskID)
			return
		}

		currentMode := s.ExecutionMode()
		if currentMode == MediaAnalysisModeOff {
			s.dropRemoteTask(taskID)
			s.failTask(task, "disabled", ErrMediaAnalysisDisabled)
			return
		}
		if currentMode == MediaAnalysisModeServerOnly {
			s.dropRemoteTask(taskID)
			s.runHighlightTask(taskID, mediaID)
			return
		}

		now := time.Now()
		state.mu.Lock()
		remote := state.remoteTasks[taskID]
		if remote == nil {
			state.mu.Unlock()
			return
		}
		claimed := remote.ClaimedBy != "" && remote.LeaseUntil.After(now)
		expiredClaim := remote.ClaimedBy != "" && !remote.LeaseUntil.After(now)
		if expiredClaim {
			if worker, ok := state.workers[remote.ClaimedBy]; ok {
				worker.State, worker.TaskID = "idle", ""
				state.workers[remote.ClaimedBy] = worker
			}
			remote.ClaimedBy, remote.ClaimToken, remote.WorkerKind = "", "", ""
			remote.LeaseUntil = time.Time{}
		}
		state.mu.Unlock()
		if claimed {
			continue
		}
		if currentMode == MediaAnalysisModeClientPreferred {
			if expiredClaim {
				s.updateTask(task, "waiting_client", maxFloat(2, task.Progress), "客户端计算租约已过期，等待其他客户端")
			}
			continue
		}
		if now.Sub(createdAt) >= remoteWorkerGrace || expiredClaim {
			s.dropRemoteTask(taskID)
			s.logger.Infof("media analysis client unavailable, fallback to server media=%s task=%s", mediaID, taskID)
			s.runHighlightTask(taskID, mediaID)
			return
		}
	}
}

func (s *MediaAnalysisService) ClaimWorkerTask(input MediaAnalysisWorkerClaimRequest) (*MediaAnalysisWorkerClaim, error) {
	worker := s.HeartbeatWorker(input.MediaAnalysisWorkerHeartbeat)
	if worker.WorkerID == "" {
		return nil, ErrMediaAnalysisWorkerClaim
	}
	if !workerEligible(worker.MediaAnalysisWorkerHeartbeat) {
		return nil, ErrMediaAnalysisWorkerNoTask
	}
	mode := s.ExecutionMode()
	if mode == MediaAnalysisModeServerOnly || mode == MediaAnalysisModeOff {
		return nil, ErrMediaAnalysisWorkerNoTask
	}

	state := mediaAnalysisState(s)
	now := time.Now()
	state.mu.Lock()
	var selected *mediaAnalysisRemoteTask
	for _, task := range state.remoteTasks {
		if task.ClaimedBy != "" && task.LeaseUntil.After(now) {
			continue
		}
		if selected == nil || task.CreatedAt.Before(selected.CreatedAt) {
			selected = task
		}
	}
	if selected == nil {
		state.mu.Unlock()
		return nil, ErrMediaAnalysisWorkerNoTask
	}
	selected.ClaimedBy = worker.WorkerID
	selected.WorkerKind = normalizeWorkerKind(worker.Kind)
	selected.ClaimToken = uuid.NewString()
	selected.LeaseUntil = now.Add(remoteWorkerLease)
	claimSnapshot := *selected
	if saved, ok := state.workers[worker.WorkerID]; ok {
		saved.State, saved.TaskID = "busy", selected.TaskID
		state.workers[worker.WorkerID] = saved
	}
	state.mu.Unlock()

	media, err := s.mediaRepo.FindByID(claimSnapshot.MediaID)
	if err != nil {
		s.releaseRemoteClaim(claimSnapshot.TaskID, claimSnapshot.ClaimToken)
		return nil, ErrMediaNotFound
	}
	starts := sparseSampleStarts(media.Duration, coarseWindowSeconds, adaptiveSampleCount(media.Duration))
	times := make([]float64, 0, len(starts))
	for _, start := range starts {
		times = append(times, math.Min(media.Duration, start+coarseWindowSeconds/2))
	}
	if task, taskErr := s.taskRepo.FindByID(claimSnapshot.TaskID); taskErr == nil {
		started := time.Now()
		if task.StartedAt == nil {
			task.StartedAt = &started
		}
		task.Status = "running"
		s.updateTask(task, "client_analysis", 5, "")
	}
	return &MediaAnalysisWorkerClaim{
		TaskID: claimSnapshot.TaskID, ClaimToken: claimSnapshot.ClaimToken,
		MediaID: claimSnapshot.MediaID, Fingerprint: claimSnapshot.Fingerprint,
		Duration: media.Duration, StreamURL: fmt.Sprintf("/api/stream/%s/direct", media.ID),
		SampleTimes: times, MaxHighlights: maxHighlightCount, EngineVersion: 3,
		LeaseExpiresAt: claimSnapshot.LeaseUntil,
	}, nil
}

func (s *MediaAnalysisService) UpdateWorkerProgress(taskID string, input MediaAnalysisWorkerProgress) error {
	if _, err := s.validateRemoteClaim(taskID, input.ClaimToken); err != nil {
		return err
	}
	s.extendRemoteLease(taskID, input.ClaimToken)
	task, err := s.taskRepo.FindByID(taskID)
	if err != nil {
		return err
	}
	stage := strings.TrimSpace(input.Stage)
	if stage == "" {
		stage = "client_analysis"
	}
	s.updateTask(task, stage, math.Max(5, math.Min(94, input.Progress)), "")
	return nil
}

func (s *MediaAnalysisService) CompleteWorkerTask(taskID string, input MediaAnalysisWorkerComplete) error {
	remote, err := s.validateRemoteClaim(taskID, input.ClaimToken)
	if err != nil {
		return err
	}
	media, err := s.mediaRepo.FindByID(remote.MediaID)
	if err != nil {
		return ErrMediaNotFound
	}
	fingerprint, err := s.mediaFingerprint(media)
	if err != nil {
		return err
	}
	if input.Fingerprint == "" || input.Fingerprint != remote.Fingerprint || fingerprint != remote.Fingerprint {
		return ErrMediaAnalysisFingerprintChanged
	}
	if len(input.Highlights) == 0 || len(input.Highlights) > maxHighlightCount {
		return ErrMediaAnalysisWorkerResult
	}

	payloadBytes := 0
	oldHighlights, _ := s.highlightRepo.ListByMediaID(media.ID)
	runDir := filepath.Join(s.cfg.Cache.CacheDir, "media-analysis", media.ID, taskID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return err
	}
	prepared := make([]model.VideoHighlight, 0, len(input.Highlights))
	for index, item := range input.Highlights {
		if item.StartTime < 0 || item.EndTime <= item.StartTime || item.EndTime > media.Duration+1 || item.Score < 0 || item.Score > 10 {
			_ = os.RemoveAll(runDir)
			return ErrMediaAnalysisWorkerResult
		}
		id := uuid.NewString()
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = highlightTitle((item.StartTime+item.EndTime)/2, media.Duration, item.Score, index)
		}
		method := strings.TrimSpace(item.AnalysisMethod)
		if method == "" {
			method = "client_sparse_v1"
		}
		thumbnailPath := ""
		if encoded := strings.TrimSpace(item.ThumbnailBase64); encoded != "" {
			data, decodeErr := base64.StdEncoding.DecodeString(encoded)
			if decodeErr != nil || len(data) == 0 || len(data) > maxRemoteThumbnailBytes {
				_ = os.RemoveAll(runDir)
				return ErrMediaAnalysisWorkerResult
			}
			payloadBytes += len(data)
			if payloadBytes > maxRemotePayloadBytes {
				_ = os.RemoveAll(runDir)
				return ErrMediaAnalysisWorkerResult
			}
			thumbnailPath = filepath.Join(runDir, id+thumbnailExtension(item.ThumbnailMime))
			if err := os.WriteFile(thumbnailPath, data, 0o644); err != nil {
				_ = os.RemoveAll(runDir)
				return err
			}
		}
		prepared = append(prepared, model.VideoHighlight{
			ID: id, MediaID: media.ID, Title: title,
			StartTime: item.StartTime, EndTime: item.EndTime,
			Score: math.Round(item.Score*10) / 10, Tags: media.Genres,
			Thumbnail: thumbnailPath, Source: "client_" + normalizeWorkerKind(remote.WorkerKind),
			AnalysisMethod: method, Fingerprint: fingerprint, Version: 3,
		})
	}
	if err := s.highlightRepo.ReplaceByMediaID(media.ID, prepared); err != nil {
		_ = os.RemoveAll(runDir)
		return err
	}
	for _, old := range oldHighlights {
		s.removeHighlightAssets(old)
	}

	task, err := s.taskRepo.FindByID(taskID)
	if err != nil {
		return err
	}
	completed := time.Now()
	task.Status, task.Stage, task.Progress, task.Error = "completed", "completed", 100, ""
	task.CompletedAt = &completed
	resultJSON, _ := json.Marshal(map[string]any{
		"highlight_count": len(prepared), "analysis_method": "distributed_client_v1",
		"worker_kind": normalizeWorkerKind(remote.WorkerKind), "worker_id": remote.ClaimedBy,
		"fingerprint": fingerprint, "engine_version": 3,
	})
	task.Result = string(resultJSON)
	if err := s.taskRepo.Update(task); err != nil {
		return err
	}
	s.broadcastTask(task)
	s.dropRemoteTask(taskID)
	s.markWorkerIdle(remote.ClaimedBy)
	s.logger.Infof("distributed media highlights completed media=%s task=%s worker=%s", media.ID, taskID, remote.ClaimedBy)
	return nil
}

func (s *MediaAnalysisService) FailWorkerTask(taskID string, input MediaAnalysisWorkerFailure) error {
	remote, err := s.validateRemoteClaim(taskID, input.ClaimToken)
	if err != nil {
		return err
	}
	s.releaseRemoteClaim(taskID, input.ClaimToken)
	s.markWorkerIdle(remote.ClaimedBy)
	if task, taskErr := s.taskRepo.FindByID(taskID); taskErr == nil {
		message := strings.TrimSpace(input.Error)
		if message == "" {
			message = "客户端计算失败"
		}
		task.Status = "pending"
		s.updateTask(task, "waiting_client", maxFloat(2, task.Progress), message)
	}
	return nil
}

func (s *MediaAnalysisService) validateRemoteClaim(taskID, claimToken string) (*mediaAnalysisRemoteTask, error) {
	state := mediaAnalysisState(s)
	now := time.Now()
	state.mu.Lock()
	defer state.mu.Unlock()
	remote := state.remoteTasks[taskID]
	if remote == nil || claimToken == "" || remote.ClaimToken != claimToken || remote.ClaimedBy == "" || !remote.LeaseUntil.After(now) {
		return nil, ErrMediaAnalysisWorkerClaim
	}
	copy := *remote
	return &copy, nil
}

func (s *MediaAnalysisService) extendRemoteLease(taskID, claimToken string) {
	state := mediaAnalysisState(s)
	state.mu.Lock()
	defer state.mu.Unlock()
	if remote := state.remoteTasks[taskID]; remote != nil && remote.ClaimToken == claimToken {
		remote.LeaseUntil = time.Now().Add(remoteWorkerLease)
	}
}

func (s *MediaAnalysisService) releaseRemoteClaim(taskID, claimToken string) {
	state := mediaAnalysisState(s)
	state.mu.Lock()
	defer state.mu.Unlock()
	if remote := state.remoteTasks[taskID]; remote != nil && remote.ClaimToken == claimToken {
		remote.ClaimedBy, remote.ClaimToken, remote.WorkerKind = "", "", ""
		remote.LeaseUntil = time.Time{}
	}
}

func (s *MediaAnalysisService) dropRemoteTask(taskID string) {
	state := mediaAnalysisState(s)
	state.mu.Lock()
	defer state.mu.Unlock()
	delete(state.remoteTasks, taskID)
}

func (s *MediaAnalysisService) markWorkerIdle(workerID string) {
	if workerID == "" {
		return
	}
	state := mediaAnalysisState(s)
	state.mu.Lock()
	defer state.mu.Unlock()
	if worker, ok := state.workers[workerID]; ok {
		worker.State, worker.TaskID, worker.LastSeen = "idle", "", time.Now()
		state.workers[workerID] = worker
	}
}

func normalizeWorkerKind(kind string) string {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case "android":
		return "android"
	case "desktop", "windows", "macos", "linux":
		return "desktop"
	default:
		return "client"
	}
}

func thumbnailExtension(mime string) string {
	switch strings.TrimSpace(strings.ToLower(mime)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	default:
		return ".webp"
	}
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
