package avsync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
)

const (
	SchemaVersion               = "hls-av-boundary-sync-evidence-v1"
	MaxProjectionResidualMicros = int64(2)
)

const (
	StatusAligned          = "aligned"
	StatusWithinOnePacket  = "within_one_packet"
	StatusOutsideOnePacket = "outside_one_packet"
)

// Contract explains how the relative audio/video presentation position changes
// across one Startup-to-Continuation boundary. It is diagnostic only and can
// never authorize seamless playback.
type Contract struct {
	SchemaVersion                string `json:"schema_version"`
	CaseID                       string `json:"case_id"`
	FixtureID                    string `json:"fixture_id"`
	BoundaryEvidenceVersion      string `json:"boundary_evidence_version"`
	BoundaryEvidenceHash         string `json:"boundary_evidence_hash"`
	StartupVideoEndMicros        int64  `json:"startup_video_end_micros"`
	StartupAudioEndMicros        int64  `json:"startup_audio_end_micros"`
	StartupEndSkewMicros         int64  `json:"startup_end_skew_micros"`
	ContinuationVideoStartMicros int64  `json:"continuation_video_start_micros"`
	ContinuationAudioStartMicros int64  `json:"continuation_audio_start_micros"`
	ContinuationStartSkewMicros  int64  `json:"continuation_start_skew_micros"`
	VideoBoundaryDeltaMicros     int64  `json:"video_boundary_delta_micros"`
	AudioBoundaryDeltaMicros     int64  `json:"audio_boundary_delta_micros"`
	BoundaryDeltaSkewMicros      int64  `json:"boundary_delta_skew_micros"`
	SkewTransitionMicros         int64  `json:"skew_transition_micros"`
	ProjectionResidualMicros     int64  `json:"projection_residual_micros"`
	ToleranceMicros              int64  `json:"tolerance_micros"`
	OnePacketBudgetMicros        int64  `json:"one_packet_budget_micros"`
	StartupStatus                string `json:"startup_status"`
	ContinuationStatus           string `json:"continuation_status"`
	MaxAbsoluteSkewMicros        int64  `json:"max_absolute_skew_micros"`
	SeamlessAllowed              bool   `json:"seamless_allowed"`
	DiscontinuityRequired        bool   `json:"discontinuity_required"`
}

func FromBoundary(source transcodeboundary.Contract) (Contract, error) {
	if err := source.Validate(); err != nil {
		return Contract{}, fmt.Errorf("validate boundary evidence: %w", err)
	}
	boundaryVersion, boundaryHash, _, err := transcodeboundary.Identity(source)
	if err != nil {
		return Contract{}, err
	}
	startupVideoEnd, err := transcodeboundary.TicksToMicros(source.Video.Startup.LatestEndPTS, source.Video.TimeBase)
	if err != nil {
		return Contract{}, err
	}
	startupAudioEnd, err := transcodeboundary.TicksToMicros(source.Audio.Startup.LatestEndPTS, source.Audio.TimeBase)
	if err != nil {
		return Contract{}, err
	}
	continuationVideoStart, err := transcodeboundary.TicksToMicros(source.Video.Continuation.EarliestPTS, source.Video.TimeBase)
	if err != nil {
		return Contract{}, err
	}
	continuationAudioStart, err := transcodeboundary.TicksToMicros(source.Audio.Continuation.EarliestPTS, source.Audio.TimeBase)
	if err != nil {
		return Contract{}, err
	}
	tolerance := max64(source.Video.ToleranceMicros, source.Audio.ToleranceMicros)
	onePacketBudget := max64(source.Video.NominalPacketDurationMicros, source.Audio.NominalPacketDurationMicros)
	startupSkew := startupAudioEnd - startupVideoEnd
	continuationSkew := continuationAudioStart - continuationVideoStart
	boundarySkew := source.Audio.PresentationDeltaMicros - source.Video.PresentationDeltaMicros
	skewTransition := continuationSkew - startupSkew
	result := Contract{
		SchemaVersion:                SchemaVersion,
		CaseID:                       source.CaseID,
		FixtureID:                    source.FixtureID,
		BoundaryEvidenceVersion:      boundaryVersion,
		BoundaryEvidenceHash:         boundaryHash,
		StartupVideoEndMicros:        startupVideoEnd,
		StartupAudioEndMicros:        startupAudioEnd,
		StartupEndSkewMicros:         startupSkew,
		ContinuationVideoStartMicros: continuationVideoStart,
		ContinuationAudioStartMicros: continuationAudioStart,
		ContinuationStartSkewMicros:  continuationSkew,
		VideoBoundaryDeltaMicros:     source.Video.PresentationDeltaMicros,
		AudioBoundaryDeltaMicros:     source.Audio.PresentationDeltaMicros,
		BoundaryDeltaSkewMicros:      boundarySkew,
		SkewTransitionMicros:         skewTransition,
		ProjectionResidualMicros:     skewTransition - boundarySkew,
		ToleranceMicros:              tolerance,
		OnePacketBudgetMicros:        onePacketBudget,
		StartupStatus:                Classify(startupSkew, tolerance, onePacketBudget),
		ContinuationStatus:           Classify(continuationSkew, tolerance, onePacketBudget),
		MaxAbsoluteSkewMicros:        max64(abs64(startupSkew), abs64(continuationSkew)),
		DiscontinuityRequired:        true,
	}
	if err := result.Validate(); err != nil {
		return Contract{}, err
	}
	return result, nil
}

func (c Contract) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported A/V boundary sync schema %q", c.SchemaVersion)
	}
	if strings.TrimSpace(c.CaseID) == "" || strings.TrimSpace(c.FixtureID) == "" {
		return fmt.Errorf("A/V boundary sync identity is incomplete")
	}
	if c.BoundaryEvidenceVersion != transcodeboundary.SchemaVersion || !isSHA256(c.BoundaryEvidenceHash) {
		return fmt.Errorf("boundary evidence identity is invalid")
	}
	if c.ToleranceMicros <= 0 || c.OnePacketBudgetMicros <= 0 {
		return fmt.Errorf("A/V boundary sync thresholds are invalid")
	}
	if c.StartupEndSkewMicros != c.StartupAudioEndMicros-c.StartupVideoEndMicros {
		return fmt.Errorf("startup A/V skew is inconsistent")
	}
	if c.ContinuationStartSkewMicros != c.ContinuationAudioStartMicros-c.ContinuationVideoStartMicros {
		return fmt.Errorf("continuation A/V skew is inconsistent")
	}
	if c.BoundaryDeltaSkewMicros != c.AudioBoundaryDeltaMicros-c.VideoBoundaryDeltaMicros {
		return fmt.Errorf("boundary A/V delta skew is inconsistent")
	}
	if c.SkewTransitionMicros != c.ContinuationStartSkewMicros-c.StartupEndSkewMicros {
		return fmt.Errorf("A/V skew transition is inconsistent")
	}
	if c.ProjectionResidualMicros != c.SkewTransitionMicros-c.BoundaryDeltaSkewMicros {
		return fmt.Errorf("A/V projection residual is inconsistent")
	}
	if abs64(c.ProjectionResidualMicros) > MaxProjectionResidualMicros {
		return fmt.Errorf("A/V projection residual exceeds integer-microsecond rounding bound")
	}
	if c.StartupStatus != Classify(c.StartupEndSkewMicros, c.ToleranceMicros, c.OnePacketBudgetMicros) {
		return fmt.Errorf("startup A/V status is inconsistent")
	}
	if c.ContinuationStatus != Classify(c.ContinuationStartSkewMicros, c.ToleranceMicros, c.OnePacketBudgetMicros) {
		return fmt.Errorf("continuation A/V status is inconsistent")
	}
	if c.MaxAbsoluteSkewMicros != max64(abs64(c.StartupEndSkewMicros), abs64(c.ContinuationStartSkewMicros)) {
		return fmt.Errorf("maximum A/V skew is inconsistent")
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("A/V boundary sync evidence v1 cannot authorize seamless playback")
	}
	return nil
}

func (c Contract) ValidateAgainst(source transcodeboundary.Contract) error {
	rebuilt, err := FromBoundary(source)
	if err != nil {
		return err
	}
	if rebuilt != c {
		return fmt.Errorf("A/V boundary sync evidence does not match packet evidence")
	}
	return nil
}

func (c Contract) CanonicalJSON() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	content, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal A/V boundary sync evidence: %w", err)
	}
	return string(content), nil
}

func (c Contract) Hash() (string, error) {
	canonical, err := c.CanonicalJSON()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:]), nil
}

func Identity(c Contract) (version, hash, canonical string, err error) {
	canonical, err = c.CanonicalJSON()
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256([]byte(canonical))
	return c.SchemaVersion, hex.EncodeToString(digest[:]), canonical, nil
}

// Classify uses packet-derived tolerance and a one-packet budget. Positive skew
// means audio is later than video; negative skew means audio is earlier.
func Classify(skewMicros, toleranceMicros, onePacketBudgetMicros int64) string {
	if abs64(skewMicros) <= toleranceMicros {
		return StatusAligned
	}
	if abs64(skewMicros) <= onePacketBudgetMicros+toleranceMicros {
		return StatusWithinOnePacket
	}
	return StatusOutsideOnePacket
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func abs64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func max64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
