package timeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
)

const SchemaVersion = "startup-handoff-timeline-v2"

const (
	StatusAligned = "aligned"
	StatusGap     = "gap"
	StatusOverlap = "overlap"
	StatusMixed   = "mixed"
)

const (
	DecisionClientCertificationPending = "client_certification_pending"
	DecisionTimelineGap                 = "timeline_gap"
	DecisionTimelineOverlap             = "timeline_overlap"
	DecisionTimelineMixed               = "timeline_mixed"
)

// Contract is immutable evidence for the boundary between a published Startup
// Artifact and one current Continuation Artifact. It binds observed packet
// relations to the shared Timestamp Plan and each Job-owned timeline origin.
// Version v2 still does not grant permission to remove the HLS discontinuity.
type Contract struct {
	SchemaVersion                  string         `json:"schema_version"`
	EncodingPlanVersion            string         `json:"encoding_plan_version"`
	EncodingPlanHash               string         `json:"encoding_plan_hash"`
	TimestampPlanVersion           string         `json:"timestamp_plan_version"`
	TimestampPlanHash              string         `json:"timestamp_plan_hash"`
	StartupTimelineOriginMS        int64          `json:"startup_timeline_origin_ms"`
	ContinuationTimelineOriginMS   int64          `json:"continuation_timeline_origin_ms"`
	ExpectedBoundaryMS             int64          `json:"expected_boundary_ms"`
	StartupAttestationVersion      string         `json:"startup_attestation_version"`
	StartupAttestationHash         string         `json:"startup_attestation_hash"`
	ContinuationAttestationVersion string         `json:"continuation_attestation_version"`
	ContinuationAttestationHash    string         `json:"continuation_attestation_hash"`
	Video                          StreamRelation `json:"video"`
	Audio                          StreamRelation `json:"audio"`
	Status                         string         `json:"status"`
	SeamlessAllowed                bool           `json:"seamless_allowed"`
	DiscontinuityRequired          bool           `json:"discontinuity_required"`
	DecisionReason                 string         `json:"decision_reason"`
}

// StreamRelation compares the end of the Startup stream with the first packet
// of the Continuation stream in the shared stream time base. End DTS is derived
// from the final packet duration captured by produced-media attestation.
type StreamRelation struct {
	TimeBase                string `json:"time_base"`
	StartupEndPTS           int64  `json:"startup_end_pts"`
	ContinuationFirstPTS    int64  `json:"continuation_first_pts"`
	PresentationDeltaTicks  int64  `json:"presentation_delta_ticks"`
	PresentationDeltaMicros int64  `json:"presentation_delta_micros"`
	StartupEndDTS           int64  `json:"startup_end_dts"`
	ContinuationFirstDTS    int64  `json:"continuation_first_dts"`
	DecodeDeltaTicks        int64  `json:"decode_delta_ticks"`
	DecodeDeltaMicros       int64  `json:"decode_delta_micros"`
	ToleranceMicros         int64  `json:"tolerance_micros"`
	Status                  string `json:"status"`
}

func Evaluate(
	startup transcodeattestation.Attestation,
	startupVersion,
	startupHash string,
	continuation transcodeattestation.Attestation,
	continuationVersion,
	continuationHash,
	timestampPlanVersion,
	timestampPlanHash string,
	startupTimelineOriginMS,
	continuationTimelineOriginMS,
	expectedBoundaryMS int64,
) (Contract, error) {
	if startupVersion == "" || startupHash == "" || continuationVersion == "" || continuationHash == "" {
		return Contract{}, fmt.Errorf("produced-media attestation identities are required")
	}
	if timestampPlanVersion == "" || timestampPlanHash == "" {
		return Contract{}, fmt.Errorf("timestamp plan identity is required")
	}
	if err := transcodeattestation.BridgeCompatible(startup, continuation); err != nil {
		return Contract{}, err
	}

	video, err := evaluateStream(
		startup.Last.Timeline.Video,
		startup.Last.Video,
		continuation.First.Timeline.Video,
	)
	if err != nil {
		return Contract{}, fmt.Errorf("video timeline: %w", err)
	}
	audio, err := evaluateStream(
		startup.Last.Timeline.Audio,
		startup.Last.Audio,
		continuation.First.Timeline.Audio,
	)
	if err != nil {
		return Contract{}, fmt.Errorf("audio timeline: %w", err)
	}

	status := aggregateStatus(video.Status, audio.Status)
	contract := Contract{
		SchemaVersion:                  SchemaVersion,
		EncodingPlanVersion:            startup.EncodingPlanVersion,
		EncodingPlanHash:               startup.EncodingPlanHash,
		TimestampPlanVersion:           timestampPlanVersion,
		TimestampPlanHash:              timestampPlanHash,
		StartupTimelineOriginMS:        startupTimelineOriginMS,
		ContinuationTimelineOriginMS:   continuationTimelineOriginMS,
		ExpectedBoundaryMS:             expectedBoundaryMS,
		StartupAttestationVersion:      startupVersion,
		StartupAttestationHash:         startupHash,
		ContinuationAttestationVersion: continuationVersion,
		ContinuationAttestationHash:    continuationHash,
		Video:                          video,
		Audio:                          audio,
		Status:                         status,
		SeamlessAllowed:                false,
		DiscontinuityRequired:          true,
		DecisionReason:                 decisionReason(status),
	}
	if err := contract.Validate(); err != nil {
		return Contract{}, err
	}
	return contract, nil
}

func (c Contract) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported timeline contract schema %q", c.SchemaVersion)
	}
	if c.EncodingPlanVersion == "" || c.EncodingPlanHash == "" ||
		c.TimestampPlanVersion == "" || c.TimestampPlanHash == "" ||
		c.StartupAttestationVersion == "" || c.StartupAttestationHash == "" ||
		c.ContinuationAttestationVersion == "" || c.ContinuationAttestationHash == "" {
		return fmt.Errorf("timeline contract identities are incomplete")
	}
	if c.StartupTimelineOriginMS < 0 || c.ContinuationTimelineOriginMS <= c.StartupTimelineOriginMS {
		return fmt.Errorf("timeline origins are invalid")
	}
	if c.ExpectedBoundaryMS != c.ContinuationTimelineOriginMS {
		return fmt.Errorf("expected boundary must equal continuation origin")
	}
	if err := c.Video.Validate(); err != nil {
		return fmt.Errorf("video relation: %w", err)
	}
	if err := c.Audio.Validate(); err != nil {
		return fmt.Errorf("audio relation: %w", err)
	}
	if c.Status != aggregateStatus(c.Video.Status, c.Audio.Status) {
		return fmt.Errorf("timeline aggregate status is invalid")
	}
	if c.DecisionReason != decisionReason(c.Status) {
		return fmt.Errorf("timeline decision reason is invalid")
	}
	// This is the safety property of schema v2. A future certified protocol must
	// use a new schema rather than silently changing the meaning of this record.
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("timeline contract v2 cannot authorize seamless handoff")
	}
	return nil
}

func (r StreamRelation) Validate() error {
	if _, _, ok := parseTimeBase(r.TimeBase); !ok {
		return fmt.Errorf("invalid time base %q", r.TimeBase)
	}
	if r.ToleranceMicros <= 0 {
		return fmt.Errorf("timeline tolerance must be positive")
	}
	if r.PresentationDeltaTicks != r.ContinuationFirstPTS-r.StartupEndPTS {
		return fmt.Errorf("presentation delta ticks are inconsistent")
	}
	if r.DecodeDeltaTicks != r.ContinuationFirstDTS-r.StartupEndDTS {
		return fmt.Errorf("decode delta ticks are inconsistent")
	}
	presentationMicros, ok := ticksToMicros(r.PresentationDeltaTicks, r.TimeBase)
	if !ok || presentationMicros != r.PresentationDeltaMicros {
		return fmt.Errorf("presentation delta time is inconsistent")
	}
	decodeMicros, ok := ticksToMicros(r.DecodeDeltaTicks, r.TimeBase)
	if !ok || decodeMicros != r.DecodeDeltaMicros {
		return fmt.Errorf("decode delta time is inconsistent")
	}
	if r.Status != relationStatus(r.PresentationDeltaMicros, r.DecodeDeltaMicros, r.ToleranceMicros) {
		return fmt.Errorf("stream relation status is invalid")
	}
	return nil
}

func (c Contract) CanonicalJSON() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	content, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal timeline contract: %w", err)
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

func evaluateStream(
	startup transcodeattestation.PacketRange,
	identity transcodeattestation.StreamIdentity,
	continuation transcodeattestation.PacketRange,
) (StreamRelation, error) {
	packetDurationTicks := startup.EndPTS - startup.LastPTS
	if packetDurationTicks <= 0 {
		return StreamRelation{}, fmt.Errorf("startup final packet duration is unavailable")
	}
	startupEndDTS := startup.LastDTS + packetDurationTicks
	presentationDeltaTicks := continuation.FirstPTS - startup.EndPTS
	decodeDeltaTicks := continuation.FirstDTS - startupEndDTS
	presentationDeltaMicros, ok := ticksToMicros(presentationDeltaTicks, identity.TimeBase)
	if !ok {
		return StreamRelation{}, fmt.Errorf("presentation delta cannot be converted")
	}
	decodeDeltaMicros, ok := ticksToMicros(decodeDeltaTicks, identity.TimeBase)
	if !ok {
		return StreamRelation{}, fmt.Errorf("decode delta cannot be converted")
	}
	packetDurationMicros, ok := ticksToMicros(packetDurationTicks, identity.TimeBase)
	if !ok || packetDurationMicros <= 0 {
		return StreamRelation{}, fmt.Errorf("startup packet duration cannot be converted")
	}
	tolerance := packetDurationMicros / 8
	if tolerance < 1000 {
		tolerance = 1000
	}
	if tolerance > 5000 {
		tolerance = 5000
	}
	return StreamRelation{
		TimeBase:                identity.TimeBase,
		StartupEndPTS:           startup.EndPTS,
		ContinuationFirstPTS:    continuation.FirstPTS,
		PresentationDeltaTicks:  presentationDeltaTicks,
		PresentationDeltaMicros: presentationDeltaMicros,
		StartupEndDTS:           startupEndDTS,
		ContinuationFirstDTS:    continuation.FirstDTS,
		DecodeDeltaTicks:        decodeDeltaTicks,
		DecodeDeltaMicros:       decodeDeltaMicros,
		ToleranceMicros:         tolerance,
		Status:                  relationStatus(presentationDeltaMicros, decodeDeltaMicros, tolerance),
	}, nil
}

func relationStatus(presentationDelta, decodeDelta, tolerance int64) string {
	presentationAligned := abs64(presentationDelta) <= tolerance
	decodeAligned := abs64(decodeDelta) <= tolerance
	if presentationAligned && decodeAligned {
		return StatusAligned
	}
	if presentationDelta > tolerance && decodeDelta > tolerance {
		return StatusGap
	}
	if presentationDelta < -tolerance && decodeDelta < -tolerance {
		return StatusOverlap
	}
	return StatusMixed
}

func aggregateStatus(video, audio string) string {
	if video == StatusAligned && audio == StatusAligned {
		return StatusAligned
	}
	if video == StatusMixed || audio == StatusMixed {
		return StatusMixed
	}
	if (video == StatusGap || video == StatusAligned) && (audio == StatusGap || audio == StatusAligned) {
		return StatusGap
	}
	if (video == StatusOverlap || video == StatusAligned) && (audio == StatusOverlap || audio == StatusAligned) {
		return StatusOverlap
	}
	return StatusMixed
}

func decisionReason(status string) string {
	switch status {
	case StatusAligned:
		return DecisionClientCertificationPending
	case StatusGap:
		return DecisionTimelineGap
	case StatusOverlap:
		return DecisionTimelineOverlap
	default:
		return DecisionTimelineMixed
	}
}

func ticksToMicros(ticks int64, timeBase string) (int64, bool) {
	numerator, denominator, ok := parseTimeBase(timeBase)
	if !ok {
		return 0, false
	}
	value := float64(ticks) * float64(numerator) / float64(denominator) * 1_000_000
	if math.IsNaN(value) || math.IsInf(value, 0) || value > math.MaxInt64 || value < math.MinInt64 {
		return 0, false
	}
	return int64(math.Round(value)), true
}

func parseTimeBase(value string) (int64, int64, bool) {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) != 2 {
		return 0, 0, false
	}
	numerator, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || numerator <= 0 {
		return 0, 0, false
	}
	denominator, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || denominator <= 0 {
		return 0, 0, false
	}
	return numerator, denominator, true
}

func abs64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
