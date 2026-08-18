package service

import (
	"strings"
	"time"
)

const (
	// MediaComputeProtocolVersion 是 Desktop / Android 与 Server 之间统一媒体计算协议版本。
	// V2 先承载现有精彩片段任务，并为章节、预览图、波形、字幕等后续任务保留 job/input 边界。
	MediaComputeProtocolVersion = 2

	MediaComputeJobHighlightV1         = "highlight_v1"
	MediaComputeCapabilityHighlightV1 = "highlight_v1"
)

// MediaComputeHighlightInput 是精彩片段适配器的任务输入。
// MediaComputeTaskClaim 同时保留历史扁平字段，已发布的 V1 Android/Desktop 可以继续工作；
// V2 客户端应优先读取 input，并按 job_type 分派到对应执行器。
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
// RequiredCapability 负责节点能力匹配；Input 是具体 job 的 payload。
// 末尾的扁平字段仅用于 V1 兼容，后续新 job 不应继续扩展扁平协议。
type MediaComputeTaskClaim struct {
	ProtocolVersion    int                        `json:"protocol_version"`
	JobType            string                     `json:"job_type"`
	RequiredCapability string                     `json:"required_capability"`
	TaskID             string                     `json:"task_id"`
	ClaimToken         string                     `json:"claim_token"`
	Input              MediaComputeHighlightInput `json:"input"`
	LeaseExpiresAt     time.Time                  `json:"lease_expires_at"`

	// V1 compatibility fields.
	MediaID        string    `json:"media_id"`
	Fingerprint    string    `json:"fingerprint"`
	Duration       float64   `json:"duration"`
	StreamURL      string    `json:"stream_url"`
	SampleTimes    []float64 `json:"sample_times"`
	MaxHighlights int       `json:"max_highlights"`
	EngineVersion int       `json:"engine_version"`
}

// MediaComputeNodeView 是管理台使用的统一节点视图。
// ClientProtocolVersion 表示客户端本身是否已经升级为 V2，而不是服务端响应版本。
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

func mediaComputeNodeView(worker MediaAnalysisWorkerView) MediaComputeNodeView {
	jobType := ""
	if worker.TaskID != "" {
		// 当前只有精彩片段适配器会产生远程任务；后续调度器接入更多 job 后，
		// 这里应直接从任务描述符读取，而不是根据 busy 状态推断。
		jobType = MediaComputeJobHighlightV1
	}
	return MediaComputeNodeView{
		WorkerID:              worker.WorkerID,
		Kind:                  worker.Kind,
		Name:                  worker.Name,
		Version:               worker.Version,
		Capabilities:          worker.Capabilities,
		Network:               worker.Network,
		Charging:              worker.Charging,
		BatteryPercent:        worker.BatteryPercent,
		ClientProtocolVersion: mediaComputeClientProtocolVersion(worker.Version),
		LastSeen:              worker.LastSeen,
		State:                 worker.State,
		TaskID:                worker.TaskID,
		CurrentJobType:        jobType,
	}
}

func (s *MediaAnalysisService) ComputeNodes() []MediaComputeNodeView {
	workers := s.Workers()
	items := make([]MediaComputeNodeView, 0, len(workers))
	for _, worker := range workers {
		items = append(items, mediaComputeNodeView(worker))
	}
	return items
}

func (s *MediaAnalysisService) HeartbeatComputeNode(input MediaAnalysisWorkerHeartbeat) MediaComputeNodeView {
	return mediaComputeNodeView(s.HeartbeatWorker(input))
}

// ClaimComputeTask 是 Media Compute Node V2 的统一领取边界。
// 当前第一适配器仍复用已经稳定的精彩片段租约/优先级实现，因此不会改变
// Desktop -> Android -> Server Sparse V2 的现有调度行为；未来 job 从这里增加适配器。
func (s *MediaAnalysisService) ClaimComputeTask(input MediaAnalysisWorkerClaimRequest) (*MediaComputeTaskClaim, error) {
	claim, err := s.ClaimWorkerTask(input)
	if err != nil {
		return nil, err
	}
	return mediaComputeHighlightClaim(claim), nil
}

func mediaComputeHighlightClaim(claim *MediaAnalysisWorkerClaim) *MediaComputeTaskClaim {
	if claim == nil {
		return nil
	}
	input := MediaComputeHighlightInput{
		MediaID:       claim.MediaID,
		Fingerprint:   claim.Fingerprint,
		Duration:      claim.Duration,
		StreamURL:     claim.StreamURL,
		SampleTimes:   claim.SampleTimes,
		MaxHighlights: claim.MaxHighlights,
		EngineVersion: claim.EngineVersion,
	}
	return &MediaComputeTaskClaim{
		ProtocolVersion:    MediaComputeProtocolVersion,
		JobType:            MediaComputeJobHighlightV1,
		RequiredCapability: MediaComputeCapabilityHighlightV1,
		TaskID:             claim.TaskID,
		ClaimToken:         claim.ClaimToken,
		Input:              input,
		LeaseExpiresAt:     claim.LeaseExpiresAt,
		MediaID:            input.MediaID,
		Fingerprint:        input.Fingerprint,
		Duration:           input.Duration,
		StreamURL:          input.StreamURL,
		SampleTimes:        input.SampleTimes,
		MaxHighlights:     input.MaxHighlights,
		EngineVersion:     input.EngineVersion,
	}
}
