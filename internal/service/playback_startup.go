package service

import "fmt"

const (
	// StartupContinuationModeEventBridge is retained only for decoding old API
	// payloads and inspecting historical startup artifacts. Runtime playback no
	// longer plans or serves startup artifacts.
	StartupContinuationModeEventBridge = "event_bridge_v1"
)

// PlaybackStartupStream remains in the response schema for backward-compatible
// JSON decoding. New playback plans always leave this field nil.
type PlaybackStartupStream struct {
	ProfileID              string `json:"profile_id"`
	DurationMS             int64  `json:"duration_ms"`
	PlaylistURL            string `json:"playlist_url"`
	ContinuationMode       string `json:"continuation_mode"`
	DiscontinuityAtHandoff bool   `json:"discontinuity_at_handoff"`
	EncodingPlanVersion    string `json:"encoding_plan_version"`
	EncodingPlanHash       string `json:"encoding_plan_hash"`
}

// chooseTranscodeOrStartup is kept as a source-compatible boundary while the
// old startup-stream implementation is removed in stages. It deliberately does
// not query Artifact storage: every runtime transcode now creates an ephemeral
// Playback Session and writes only below playback-temp.
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
	_ = s
	_ = mediaID
	_ = caps
	_ = hlsURL
	return chooseTranscode(plan, "", transcodeReasonCode, transcodeReason), nil
}
