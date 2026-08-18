package service

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// MediaComputeProtocolVersion 是 Desktop / Android 与 Server 之间统一媒体计算协议版本。
	MediaComputeProtocolVersion = 2

	MediaComputeJobHighlightV1         = "highlight_v1"
	MediaComputeCapabilityHighlightV1 = "highlight_v1"
)

var ErrMediaComputeUnsupportedJob = errors.New("unsupported media compute job")

// MediaComputeHighlightInput 是 highlight_v1 adapter 的私有输入。
// 新任务类型必须定义自己的 input，不能再向 Claim 顶层追加业务字段。
type MediaComputeHighlightInput struct {
	MediaID       string    `json:"media_id"`
	Fingerprint   string    `json:"fingerprint"`
	Duration      float64   `json:"duration"`
	StreamURL     string    `json:"stream_url"`
	SampleTimes   []float64 `json:"sample_times"`
	MaxHighlights int       `json:"max_highlights"`
	EngineVersion int       `json:"engine_version"`
}

// MediaComputeTaskClaim 是统一 Media Compute Node V2 任务信封。
// Input 是 job 私有 JSON；RequiredCapability 用于真正的 capability-aware 调度。
// 末尾扁平字段仅为 V1 highlight 客户端兼容，新 job 禁止继续扩展这些字段。
type MediaComputeTaskClaim struct {
	ProtocolVersion    int             `json:"protocol_version"`
	JobType            string          `json:"job_type"`
	RequiredCapability string          `json:"required_capability"`
	TaskID             string          `json:"task_id"`
	ClaimToken         string          `json:"claim_token"`
	Input              json.RawMessage `json:"input"`
	LeaseExpiresAt     time.Time       `json:"lease_expires_at"`

	MediaID        string    `json:"media_id,omitempty"`
	Fingerprint    string    `json:"fingerprint,omitempty"`
	Duration       float64   `json:"duration,omitempty"`
	StreamURL      string    `json:"stream_url,omitempty"`
	SampleTimes    []float64 `json:"sample_times,omitempty"`
	MaxHighlights int       `json:"max_highlights,omitempty"`
	EngineVersion int       `json:"engine_version,omitempty"`
}

// MediaComputeTaskComplete 同时支持 V2 generic result 和旧 highlight 扁平结果。
// 新 V2 客户端应发送 job_type + result；旧客户端仍可直接发送 fingerprint + highlights。
type MediaComputeTaskComplete struct {
	ClaimToken string          `json:"claim_token"`
	JobType    string          `json:"job_type,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`

	Fingerprint string                         `json:"fingerprint,omitempty"`
	Highlights  []MediaAnalysisWorkerHighlight `json:"highlights,omitempty"`
}

// MediaComputeTaskRegistration 是后续媒体任务复用统一节点协议的注册边界。
// 业务服务仍负责自己的持久化、fallback 和结果 adapter；协调层只负责节点、能力、Claim 与租约。
type MediaComputeTaskRegistration struct {
	TaskID             string
	MediaID            string
	Fingerprint        string
	JobType            string
	RequiredCapability string
	Input              json.RawMessage
	CreatedAt          time.Time
}

type mediaComputeTaskDescriptor struct {
	JobType            string
	RequiredCapability string
	Input               json.RawMessage
}

type mediaComputeDescriptorState struct {
	mu    sync.Mutex
	tasks map[string]mediaComputeTaskDescriptor
}

var mediaComputeDescriptorStates sync.Map

func mediaComputeDescriptors(s *MediaAnalysisService) *mediaComputeDescriptorState {
	if value, ok := mediaComputeDescriptorStates.Load(s); ok {
		return value.(*mediaComputeDescriptorState)
	}
	created := &mediaComputeDescriptorState{tasks: make(map[string]mediaComputeTaskDescriptor)}
	actual, _ := mediaComputeDescriptorStates.LoadOrStore(s, created)
	return actual.(*mediaComputeDescriptorState)
}

// RegisterComputeTask 允许后续章节、预览图、波形、字幕等业务直接进入同一节点队列。
// highlight_v1 目前仍由稳定的旧 dispatcher 创建 remote task；没有 descriptor 时会走明确的兼容 adapter。
func (s *MediaAnalysisService) RegisterComputeTask(input MediaComputeTaskRegistration) error {
	input.TaskID = strings.TrimSpace(input.TaskID)
	input.JobType = strings.TrimSpace(input.JobType)
	input.RequiredCapability = strings.TrimSpace(input.RequiredCapability)
	if input.TaskID == "" || input.JobType == "" || input.RequiredCapability == "" || len(input.Input) == 0 || !json.Valid(input.Input) {
		return ErrMediaAnalysisWorkerResult
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now()
	}

	state := mediaAnalysisState(s)
	descriptors := mediaComputeDescriptors(s)
	state.mu.Lock()
	defer state.mu.Unlock()
	if _, exists := state.remoteTasks[input.TaskID]; exists {
		return ErrMediaAnalysisInProgress
	}
	// Lock order is always worker state -> descriptor state when both are needed.
	descriptors.mu.Lock()
	defer descriptors.mu.Unlock()
	descriptors.tasks[input.TaskID] = mediaComputeTaskDescriptor{
		JobType: input.JobType, RequiredCapability: input.RequiredCapability,
		Input: append(json.RawMessage(nil), input.Input...),
	}
	state.remoteTasks[input.TaskID] = &mediaAnalysisRemoteTask{
		TaskID: input.TaskID, MediaID: strings.TrimSpace(input.MediaID),
		Fingerprint: strings.TrimSpace(input.Fingerprint), CreatedAt: input.CreatedAt,
	}
	return nil
}

func (s *MediaAnalysisService) UnregisterComputeTask(taskID string) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return
	}
	state := mediaAnalysisState(s)
	descriptors := mediaComputeDescriptors(s)
	state.mu.Lock()
	defer state.mu.Unlock()
	descriptors.mu.Lock()
	defer descriptors.mu.Unlock()
	if remote := state.remoteTasks[taskID]; remote != nil && remote.ClaimedBy != "" {
		if worker, ok := state.workers[remote.ClaimedBy]; ok && worker.TaskID == taskID {
			worker.State, worker.TaskID, worker.LastSeen = "idle", "", time.Now()
			state.workers[remote.ClaimedBy] = worker
		}
	}
	delete(state.remoteTasks, taskID)
	delete(descriptors.tasks, taskID)
}

func mediaComputeDescriptor(s *MediaAnalysisService, taskID string) mediaComputeTaskDescriptor {
	descriptors := mediaComputeDescriptors(s)
	descriptors.mu.Lock()
	descriptor, ok := descriptors.tasks[taskID]
	descriptors.mu.Unlock()
	if ok {
		return descriptor
	}
	// Compatibility adapter: legacy dispatcher can only create highlight tasks.
	// This is intentionally isolated here rather than spread through the selector.
	return mediaComputeTaskDescriptor{
		JobType: MediaComputeJobHighlightV1,
		RequiredCapability: MediaComputeCapabilityHighlightV1,
	}
}

// MediaComputeNodeView 是管理台使用的统一节点视图。
type MediaComputeNodeView struct {
	WorkerID              string    `json:"worker_id"`
	Kind                  string    `json:"kind"`
	Name                  string    `json:"name"`
	Version               string    `json:"version"`
	Capabilities          []string  `json:"capabilities"`
	Network               string    `json:"network"`
	Charging              bool      `json:"charging"`
	BatteryPercent        int       `json:"battery_percent"`
	ClientProtocolVersion int       `json:"client_protocol_version"`
	LastSeen              time.Time `json:"last_seen"`
	State                 string    `json:"state"`
	TaskID                string    `json:"task_id,omitempty"`
	CurrentJobType        string    `json:"current_job_type,omitempty"`
}

func mediaComputeClientProtocolVersion(version string) int {
	value := strings.ToLower(strings.TrimSpace(version))
	if strings.Contains(value, "-v2/") || strings.HasPrefix(value, "v2/") {
		return MediaComputeProtocolVersion
	}
	return 1
}

func mediaComputeNodeSupportsCapability(input MediaAnalysisWorkerHeartbeat, capability string) bool {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return false
	}
	for _, item := range input.Capabilities {
		if strings.EqualFold(strings.TrimSpace(item), capability) {
			return true
		}
	}
	return false
}

// mediaComputeNodeAvailable 只判断设备是否适合参与分布式计算，不绑定任何具体 job。
func mediaComputeNodeAvailable(input MediaAnalysisWorkerHeartbeat) bool {
	hasCapability := false
	for _, capability := range input.Capabilities {
		if strings.TrimSpace(capability) != "" {
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

func mediaComputeNodeCanRun(input MediaAnalysisWorkerHeartbeat, capability string) bool {
	return mediaComputeNodeAvailable(input) && mediaComputeNodeSupportsCapability(input, capability)
}

func mediaComputeNodeView(s *MediaAnalysisService, worker MediaAnalysisWorkerView) MediaComputeNodeView {
	jobType := ""
	if worker.TaskID != "" {
		jobType = mediaComputeDescriptor(s, worker.TaskID).JobType
	}
	return MediaComputeNodeView{
		WorkerID: worker.WorkerID, Kind: worker.Kind, Name: worker.Name, Version: worker.Version,
		Capabilities: worker.Capabilities, Network: worker.Network, Charging: worker.Charging,
		BatteryPercent: worker.BatteryPercent,
		ClientProtocolVersion: mediaComputeClientProtocolVersion(worker.Version),
		LastSeen: worker.LastSeen, State: worker.State, TaskID: worker.TaskID, CurrentJobType: jobType,
	}
}

func (s *MediaAnalysisService) ComputeNodes() []MediaComputeNodeView {
	workers := s.Workers()
	items := make([]MediaComputeNodeView, 0, len(workers))
	for _, worker := range workers {
		items = append(items, mediaComputeNodeView(s, worker))
	}
	return items
}

// HeartbeatComputeNode 是 V2 通用心跳，不再要求 highlight_v1。
func (s *MediaAnalysisService) HeartbeatComputeNode(input MediaAnalysisWorkerHeartbeat) MediaComputeNodeView {
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
	if !mediaComputeNodeAvailable(input) {
		view.State = "unavailable"
	}
	for _, task := range state.remoteTasks {
		if task.ClaimedBy == input.WorkerID && task.LeaseUntil.After(now) {
			view.State, view.TaskID = "busy", task.TaskID
			break
		}
	}
	if input.WorkerID != "" {
		state.workers[input.WorkerID] = view
	}
	return mediaComputeNodeView(s, view)
}

func hasPreferredDesktopForCapabilityLocked(state *mediaAnalysisWorkerState, now time.Time, requestingWorkerID, capability string) bool {
	for workerID, worker := range state.workers {
		if workerID == requestingWorkerID || normalizeWorkerKind(worker.Kind) != "desktop" {
			continue
		}
		if worker.State != "idle" || now.Sub(worker.LastSeen) > remoteDesktopPreferenceTTL {
			continue
		}
		if mediaComputeNodeCanRun(worker.MediaAnalysisWorkerHeartbeat, capability) {
			return true
		}
	}
	return false
}

func mediaComputeJobUsesAnalysisMode(jobType string) bool {
	switch jobType {
	case MediaComputeJobHighlightV1, MediaComputeJobPreviewThumbnailV1, MediaComputeJobChapterDetectV1:
		return true
	default:
		return false
	}
}

// ClaimComputeTask 是 Media Compute Node V2 的 capability-aware 领取器。
// highlight / preview / chapter 都遵循 execution_mode；Desktop 优先仍然严格按 capability 生效。
func (s *MediaAnalysisService) ClaimComputeTask(input MediaAnalysisWorkerClaimRequest) (*MediaComputeTaskClaim, error) {
	node := s.HeartbeatComputeNode(input.MediaAnalysisWorkerHeartbeat)
	if node.WorkerID == "" {
		return nil, ErrMediaAnalysisWorkerClaim
	}
	if node.State == "unavailable" {
		return nil, ErrMediaAnalysisWorkerNoTask
	}

	heartbeat := MediaAnalysisWorkerHeartbeat{
		WorkerID: node.WorkerID, Kind: node.Kind, Name: node.Name, Version: node.Version,
		Capabilities: node.Capabilities, Network: node.Network, Charging: node.Charging,
		BatteryPercent: node.BatteryPercent,
	}
	analysisMode := s.ExecutionMode()
	state := mediaAnalysisState(s)
	now := time.Now()
	state.mu.Lock()
	cleanupWorkersLocked(state, now)
	var selected *mediaAnalysisRemoteTask
	var selectedDescriptor mediaComputeTaskDescriptor
	for _, task := range state.remoteTasks {
		if task.ClaimedBy != "" && task.LeaseUntil.After(now) {
			continue
		}
		if task.ClaimedBy != "" && !task.LeaseUntil.After(now) {
			if worker, ok := state.workers[task.ClaimedBy]; ok {
				worker.State, worker.TaskID = "idle", ""
				state.workers[task.ClaimedBy] = worker
			}
			task.ClaimedBy, task.ClaimToken, task.WorkerKind = "", "", ""
			task.LeaseUntil = time.Time{}
		}
		descriptor := mediaComputeDescriptor(s, task.TaskID)
		if mediaComputeJobUsesAnalysisMode(descriptor.JobType) &&
			(analysisMode == MediaAnalysisModeServerOnly || analysisMode == MediaAnalysisModeOff) {
			continue
		}
		if !mediaComputeNodeCanRun(heartbeat, descriptor.RequiredCapability) {
			continue
		}
		if normalizeWorkerKind(heartbeat.Kind) == "android" &&
			hasPreferredDesktopForCapabilityLocked(state, now, heartbeat.WorkerID, descriptor.RequiredCapability) {
			continue
		}
		if selected == nil || task.CreatedAt.Before(selected.CreatedAt) {
			selected, selectedDescriptor = task, descriptor
		}
	}
	if selected == nil {
		state.mu.Unlock()
		return nil, ErrMediaAnalysisWorkerNoTask
	}
	selected.ClaimedBy = heartbeat.WorkerID
	selected.WorkerKind = normalizeWorkerKind(heartbeat.Kind)
	selected.ClaimToken = uuid.NewString()
	selected.LeaseUntil = now.Add(remoteWorkerLease)
	claimSnapshot := *selected
	if saved, ok := state.workers[heartbeat.WorkerID]; ok {
		saved.State, saved.TaskID, saved.LastSeen = "busy", selected.TaskID, now
		state.workers[heartbeat.WorkerID] = saved
	}
	state.mu.Unlock()

	claim, err := s.buildMediaComputeClaim(&claimSnapshot, selectedDescriptor)
	if err != nil {
		s.releaseRemoteClaim(claimSnapshot.TaskID, claimSnapshot.ClaimToken)
		s.markWorkerIdle(claimSnapshot.ClaimedBy)
		return nil, err
	}
	if s.taskRepo != nil {
		if task, taskErr := s.taskRepo.FindByID(claimSnapshot.TaskID); taskErr == nil {
			started := time.Now()
			if task.StartedAt == nil {
				task.StartedAt = &started
			}
			task.Status = "running"
			s.updateTask(task, "client_analysis", math.Max(5, task.Progress), "")
		}
	}
	return claim, nil
}

func (s *MediaAnalysisService) buildMediaComputeClaim(remote *mediaAnalysisRemoteTask, descriptor mediaComputeTaskDescriptor) (*MediaComputeTaskClaim, error) {
	input := append(json.RawMessage(nil), descriptor.Input...)
	claim := &MediaComputeTaskClaim{
		ProtocolVersion: MediaComputeProtocolVersion, JobType: descriptor.JobType,
		RequiredCapability: descriptor.RequiredCapability, TaskID: remote.TaskID,
		ClaimToken: remote.ClaimToken, LeaseExpiresAt: remote.LeaseUntil,
	}
	if descriptor.JobType != MediaComputeJobHighlightV1 {
		if len(input) == 0 || !json.Valid(input) {
			return nil, ErrMediaAnalysisWorkerResult
		}
		claim.Input = input
		return claim, nil
	}

	media, err := s.mediaRepo.FindByID(remote.MediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}
	starts := sparseSampleStarts(media.Duration, coarseWindowSeconds, adaptiveSampleCount(media.Duration))
	times := make([]float64, 0, len(starts))
	for _, start := range starts {
		times = append(times, math.Min(media.Duration, start+coarseWindowSeconds/2))
	}
	highlightInput := MediaComputeHighlightInput{
		MediaID: remote.MediaID, Fingerprint: remote.Fingerprint, Duration: media.Duration,
		StreamURL: "/api/stream/" + media.ID + "/direct", SampleTimes: times,
		MaxHighlights: maxHighlightCount, EngineVersion: 3,
	}
	input, err = json.Marshal(highlightInput)
	if err != nil {
		return nil, err
	}
	claim.Input = input
	// V1 compatibility mirror. Released clients ignore the V2 envelope and keep reading these fields.
	claim.MediaID, claim.Fingerprint, claim.Duration = highlightInput.MediaID, highlightInput.Fingerprint, highlightInput.Duration
	claim.StreamURL, claim.SampleTimes = highlightInput.StreamURL, highlightInput.SampleTimes
	claim.MaxHighlights, claim.EngineVersion = highlightInput.MaxHighlights, highlightInput.EngineVersion
	return claim, nil
}

// CompleteComputeTask 接受 V2 generic result，同时兼容旧 highlight complete payload。
func (s *MediaAnalysisService) CompleteComputeTask(taskID string, input MediaComputeTaskComplete) error {
	if len(input.Result) == 0 && input.JobType == "" {
		return s.CompleteWorkerTask(taskID, MediaAnalysisWorkerComplete{
			ClaimToken: input.ClaimToken, Fingerprint: input.Fingerprint, Highlights: input.Highlights,
		})
	}
	remote, err := s.validateRemoteClaim(taskID, input.ClaimToken)
	if err != nil {
		return err
	}
	descriptor := mediaComputeDescriptor(s, taskID)
	jobType := strings.TrimSpace(input.JobType)
	if jobType == "" {
		jobType = descriptor.JobType
	}
	if jobType != descriptor.JobType || len(input.Result) == 0 || !json.Valid(input.Result) {
		return ErrMediaAnalysisWorkerResult
	}

	switch descriptor.JobType {
	case MediaComputeJobHighlightV1:
		var result struct {
			Fingerprint string                         `json:"fingerprint"`
			Highlights  []MediaAnalysisWorkerHighlight `json:"highlights"`
		}
		if err := json.Unmarshal(input.Result, &result); err != nil {
			return ErrMediaAnalysisWorkerResult
		}
		err := s.CompleteWorkerTask(taskID, MediaAnalysisWorkerComplete{
			ClaimToken: input.ClaimToken, Fingerprint: result.Fingerprint, Highlights: result.Highlights,
		})
		if err == nil {
			descriptors := mediaComputeDescriptors(s)
			descriptors.mu.Lock()
			delete(descriptors.tasks, remote.TaskID)
			descriptors.mu.Unlock()
		}
		return err
	case MediaComputeJobPreviewThumbnailV1:
		return s.completePreviewThumbnailComputeTask(taskID, remote, descriptor, input.Result)
	case MediaComputeJobChapterDetectV1:
		return s.completeChapterDetectComputeTask(taskID, remote, descriptor, input.Result)
	default:
		return ErrMediaComputeUnsupportedJob
	}
}
