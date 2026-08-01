package service

import "fmt"

const (
	// StartupContinuationModeEventBridge identifies the current server-owned
	// handoff protocol: one authenticated EVENT playlist exposes the immutable
	// startup Artifact first and appends the Lease-valid continuation Artifact.
	StartupContinuationModeEventBridge = "event_bridge_v1"
)

// PlaybackStartupStream is the client-safe startup-stream contract embedded in
// PlaybackPlan. It intentionally exposes no filesystem paths, canonical plan
// JSON, Job IDs, Attempt IDs, Lease tokens, or Artifact IDs.
type PlaybackStartupStream struct {
	ProfileID              string `json:"profile_id"`
	DurationMS             int64  `json:"duration_ms"`
	PlaylistURL            string `json:"playlist_url"`
	ContinuationMode       string `json:"continuation_mode"`
	DiscontinuityAtHandoff bool   `json:"discontinuity_at_handoff"`
	EncodingPlanVersion    string `json:"encoding_plan_version"`
	EncodingPlanHash       string `json:"encoding_plan_hash"`
}

// chooseTranscodeOrStartup is the only transition from a normal transcode
// decision to startup_stream. Direct, Remux, and Smart Remux decisions remain
// untouched. A client bitrate cap deliberately disables startup reuse because
// the immutable startup Artifact was encoded before that per-request cap was
// known; Runtime HLS remains the authoritative capped fallback.
func (s *StreamService) chooseTranscodeOrStartup(
	plan *PlaybackPlan,
	mediaID string,
	caps PlaybackClientCapabilities,
	hlsURL,
	transcodeReasonCode,
	transcodeReason string,
) (*PlaybackPlan, error) {
	if plan == nil {
		return nil, fmt.Errorf("playback plan is nil")
	}
	if caps.MaxBitrate > 0 {
		return chooseTranscode(plan, hlsURL, transcodeReasonCode, transcodeReason), nil
	}

	startup, err := s.GetStartupBridgeInfo(mediaID)
	if err != nil {
		return nil, fmt.Errorf("resolve startup stream: %w", err)
	}
	if !startupStreamCanBePlanned(startup) {
		return chooseTranscode(plan, hlsURL, transcodeReasonCode, transcodeReason), nil
	}
	return applyStartupStreamPlan(plan, hlsURL, startup), nil
}

func applyStartupStreamPlan(plan *PlaybackPlan, hlsURL string, startup *StartupBridgeInfo) *PlaybackPlan {
	plan.Method = PlaybackMethodStartupStream
	plan.URL = startup.PlaylistURL
	plan.ReasonCode = "startup_artifact_ready"
	plan.Reason = "已命中预生成启动流，并通过服务端统一时间线接续持续转码"
	plan.RequiresTranscode = true
	plan.FallbackMethod = PlaybackMethodTranscode
	plan.FallbackURL = hlsURL
	plan.StartupStream = &PlaybackStartupStream{
		ProfileID:              startup.ProfileID,
		DurationMS:             startup.DurationMS,
		PlaylistURL:            startup.PlaylistURL,
		ContinuationMode:       StartupContinuationModeEventBridge,
		DiscontinuityAtHandoff: true,
		EncodingPlanVersion:    startup.EncodingPlanVersion,
		EncodingPlanHash:       startup.EncodingPlanHash,
	}
	return plan
}

func startupStreamCanBePlanned(startup *StartupBridgeInfo) bool {
	return startup != nil &&
		startup.Available &&
		startup.ProfileID != "" &&
		startup.DurationMS > 0 &&
		startup.PlaylistURL != "" &&
		startup.EncodingPlanVersion != "" &&
		startup.EncodingPlanHash != ""
}
