package recoverystress

import (
	"encoding/json"

	transcodelongdrift "github.com/nowen-video/nowen-video/internal/transcode/longdrift"
)

const (
	ScenarioSchemaVersion  = "transcode-recovery-resource-scenario-evidence-v4"
	AggregateSchemaVersion = "transcode-recovery-resource-aggregate-evidence-v4"

	ScenarioCancelActiveWrite = "cancel-active-segment-write-v1"
	ScenarioSIGKILLRecovery   = "sigkill-lease-requeue-restart-v1"
	ScenarioENOSPCWrite       = "enospc-segment-write-v1"
	ScenarioBoundedResources  = "bounded-one-core-512m-v1"
	ScenarioStaleLeaseFence   = "stale-lease-finalize-fence-v1"
)

type ResourceLimits struct {
	CPUCount          int   `json:"cpu_count"`
	AddressSpaceBytes int64 `json:"address_space_bytes"`
	MemoryMaxBytes    int64 `json:"-"`
	ENOSPCAfterBytes  int64 `json:"enospc_after_bytes"`
}

func (r *ResourceLimits) UnmarshalJSON(content []byte) error {
	var decoded struct {
		CPUCount          int   `json:"cpu_count"`
		AddressSpaceBytes int64 `json:"address_space_bytes"`
		ENOSPCAfterBytes  int64 `json:"enospc_after_bytes"`
	}
	if err := json.Unmarshal(content, &decoded); err != nil {
		return err
	}
	r.CPUCount = decoded.CPUCount
	r.AddressSpaceBytes = decoded.AddressSpaceBytes
	r.MemoryMaxBytes = decoded.AddressSpaceBytes
	r.ENOSPCAfterBytes = decoded.ENOSPCAfterBytes
	return nil
}

type ScenarioSpec struct {
	ID                          string         `json:"id"`
	Purpose                     string         `json:"purpose"`
	FaultKind                   string         `json:"fault_kind"`
	LogicalDurationMicros       int64          `json:"logical_duration_micros"`
	TriggerMicros               int64          `json:"trigger_micros"`
	ExpectedProcessCount        int            `json:"expected_process_count"`
	ExpectedFinalJobStatus      string         `json:"expected_final_job_status"`
	ExpectedFinalArtifactStatus string         `json:"expected_final_artifact_status"`
	RequiresReplacementAttempt  bool           `json:"requires_replacement_attempt"`
	Limits                      ResourceLimits `json:"limits"`
}

type StateTransitionEvidence struct {
	Sequence        int    `json:"sequence"`
	JobStatus       string `json:"job_status"`
	DesiredState    string `json:"desired_state"`
	LeaseGeneration int    `json:"lease_generation"`
	AttemptOrdinal  int    `json:"attempt_ordinal"`
	ArtifactStatus  string `json:"artifact_status"`
	Reason          string `json:"reason"`
}

type ProcessEvidence struct {
	AttemptOrdinal        int      `json:"attempt_ordinal"`
	CommandHash           string   `json:"command_hash"`
	ExitCode              int      `json:"exit_code"`
	Cancelled             bool     `json:"cancelled"`
	TimedOut              bool     `json:"timed_out"`
	Signal                string   `json:"signal"`
	TriggerObservedMicros int64    `json:"trigger_observed_micros"`
	MaximumProgressMicros int64    `json:"maximum_progress_micros"`
	SegmentCount          int      `json:"segment_count"`
	ManifestExists        bool     `json:"manifest_exists"`
	WorkspaceExists       bool     `json:"workspace_exists"`
	PublishedExists       bool     `json:"published_exists"`
	StderrSHA256          string   `json:"stderr_sha256"`
	StderrMarkers         []string `json:"stderr_markers"`
	MaxRSSBytes           int64    `json:"max_rss_bytes"`
	ElapsedMillis         int64    `json:"elapsed_millis"`
	ResourceController    string   `json:"resource_controller"`
	CPUCountLimit         int      `json:"cpu_count_limit"`
	MemoryLimitBytes      int64    `json:"memory_limit_bytes"`
	FaultBackend          string   `json:"fault_backend"`
}

type LeaseFenceEvidence struct {
	FirstTokenHash              string `json:"first_token_hash"`
	SecondTokenHash             string `json:"second_token_hash"`
	LeaseExpiredRequeued        bool   `json:"lease_expired_requeued"`
	OldPrepareRejected          bool   `json:"old_prepare_rejected"`
	OldCommitRejected           bool   `json:"old_commit_rejected"`
	ReplacementLeaseDifferent   bool   `json:"replacement_lease_different"`
	ReplacementPublishCommitted bool   `json:"replacement_publish_committed"`
}

type ArtifactOutcomeEvidence struct {
	FinalJobStatus              string `json:"final_job_status"`
	FinalArtifactStatus         string `json:"final_artifact_status"`
	ReadableArtifactID          string `json:"readable_artifact_id"`
	PartialWorkspaceQuarantined bool   `json:"partial_workspace_quarantined"`
	CleanupEligible             bool   `json:"cleanup_eligible"`
}

type ScenarioContract struct {
	SchemaVersion               string                            `json:"schema_version"`
	SpecVersion                 string                            `json:"spec_version"`
	SpecHash                    string                            `json:"spec_hash"`
	ManifestVersion             string                            `json:"manifest_version"`
	ManifestHash                string                            `json:"manifest_hash"`
	SourceGeneratorVersion      string                            `json:"source_generator_version"`
	SourceFFmpegVersion         string                            `json:"source_ffmpeg_version"`
	SourceFFprobeVersion        string                            `json:"source_ffprobe_version"`
	CertificationFFmpegVersion  string                            `json:"certification_ffmpeg_version"`
	CertificationFFprobeVersion string                            `json:"certification_ffprobe_version"`
	TimestampPlanVersion        string                            `json:"timestamp_plan_version"`
	TimestampPlanHash           string                            `json:"timestamp_plan_hash"`
	Scenario                    ScenarioSpec                      `json:"scenario"`
	Source                      transcodelongdrift.SourceIdentity `json:"source"`
	Transitions                 []StateTransitionEvidence         `json:"transitions"`
	Processes                   []ProcessEvidence                 `json:"processes"`
	Fence                       LeaseFenceEvidence                `json:"fence"`
	Artifact                    ArtifactOutcomeEvidence           `json:"artifact"`
	ErrorCode                   string                            `json:"error_code"`
	Passed                      bool                              `json:"passed"`
	SeamlessAllowed             bool                              `json:"seamless_allowed"`
	DiscontinuityRequired       bool                              `json:"discontinuity_required"`
}

type ScenarioBinding struct {
	ScenarioID      string           `json:"scenario_id"`
	ContractVersion string           `json:"contract_version"`
	ContractHash    string           `json:"contract_hash"`
	Evidence        ScenarioContract `json:"evidence"`
}

type AggregateContract struct {
	SchemaVersion               string                            `json:"schema_version"`
	SpecVersion                 string                            `json:"spec_version"`
	SpecHash                    string                            `json:"spec_hash"`
	ManifestVersion             string                            `json:"manifest_version"`
	ManifestHash                string                            `json:"manifest_hash"`
	SourceGeneratorVersion      string                            `json:"source_generator_version"`
	SourceFFmpegVersion         string                            `json:"source_ffmpeg_version"`
	SourceFFprobeVersion        string                            `json:"source_ffprobe_version"`
	CertificationFFmpegVersion  string                            `json:"certification_ffmpeg_version"`
	CertificationFFprobeVersion string                            `json:"certification_ffprobe_version"`
	TimestampPlanVersion        string                            `json:"timestamp_plan_version"`
	TimestampPlanHash           string                            `json:"timestamp_plan_hash"`
	Source                      transcodelongdrift.SourceIdentity `json:"source"`
	Scenarios                   []ScenarioBinding                 `json:"scenarios"`
	TotalProcesses              int                               `json:"total_processes"`
	TotalSegmentsObserved       int                               `json:"total_segments_observed"`
	MaximumRSSBytes             int64                             `json:"maximum_rss_bytes"`
	AllPassed                   bool                              `json:"all_passed"`
	SeamlessAllowed             bool                              `json:"seamless_allowed"`
	DiscontinuityRequired       bool                              `json:"discontinuity_required"`
}
