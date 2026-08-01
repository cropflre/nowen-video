package sourceorigin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	transcodeavsync "github.com/nowen-video/nowen-video/internal/transcode/avsync"
	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

const SchemaVersion = "hls-source-origin-evidence-v1"

const (
	ModeCFR = "cfr"
	ModeVFR = "vfr"

	OriginZero     = "zero"
	OriginPositive = "positive"
	OriginNegative = "negative"

	StreamVideo = "video"
	StreamAudio = "audio"

	VFRSpreadThresholdMicros int64 = 5_000
	MaxOriginErrorMicros     int64 = 100_000
)

// Contract proves the timestamp origin and packet cadence of one source before
// normalisation, then binds that source evidence to produced-media boundary and
// A/V sync evidence. It is diagnostic only and cannot authorize seamless HLS.
type Contract struct {
	SchemaVersion                   string         `json:"schema_version"`
	CaseID                         string         `json:"case_id"`
	FixtureID                      string         `json:"fixture_id"`
	SourceMode                     string         `json:"source_mode"`
	DeclaredFrameRateNumerator     int64          `json:"declared_frame_rate_numerator"`
	DeclaredFrameRateDenominator   int64          `json:"declared_frame_rate_denominator"`
	DeclaredFrameRateMilli         int            `json:"declared_frame_rate_milli"`
	SourceOffsetMicros             int64          `json:"source_offset_micros"`
	OriginClass                    string         `json:"origin_class"`
	OriginToleranceMicros          int64          `json:"origin_tolerance_micros"`
	ExpectedBoundaryMicros         int64          `json:"expected_boundary_micros"`
	FFmpegVersion                  string         `json:"ffmpeg_version"`
	FFprobeVersion                 string         `json:"ffprobe_version"`
	TimestampPlanVersion           string         `json:"timestamp_plan_version"`
	TimestampPlanHash              string         `json:"timestamp_plan_hash"`
	BoundaryEvidenceVersion        string         `json:"boundary_evidence_version"`
	BoundaryEvidenceHash           string         `json:"boundary_evidence_hash"`
	AVSyncEvidenceVersion          string         `json:"av_sync_evidence_version"`
	AVSyncEvidenceHash             string         `json:"av_sync_evidence_hash"`
	SourceVideo                    StreamEvidence `json:"source_video"`
	SourceAudio                    StreamEvidence `json:"source_audio"`
	NormalizedStartupVideoStartMS  int64          `json:"normalized_startup_video_start_ms"`
	NormalizedStartupAudioStartMS  int64          `json:"normalized_startup_audio_start_ms"`
	NormalizedContinuationVideoMS  int64          `json:"normalized_continuation_video_start_ms"`
	NormalizedContinuationAudioMS  int64          `json:"normalized_continuation_audio_start_ms"`
	SeamlessAllowed                bool           `json:"seamless_allowed"`
	DiscontinuityRequired          bool           `json:"discontinuity_required"`
}

type StreamEvidence struct {
	Kind                    string `json:"kind"`
	TimeBase                string `json:"time_base"`
	PacketCount             int    `json:"packet_count"`
	FirstPTS                int64  `json:"first_pts"`
	FirstDTS                int64  `json:"first_dts"`
	FirstPTSMicros          int64  `json:"first_pts_micros"`
	FirstDTSMicros          int64  `json:"first_dts_micros"`
	MinPacketDurationTicks  int64  `json:"min_packet_duration_ticks"`
	MaxPacketDurationTicks  int64  `json:"max_packet_duration_ticks"`
	MinPacketDurationMicros int64  `json:"min_packet_duration_micros"`
	MaxPacketDurationMicros int64  `json:"max_packet_duration_micros"`
	DurationSpreadMicros    int64  `json:"duration_spread_micros"`
	DistinctDurations       int    `json:"distinct_durations"`
	VariableDuration        bool   `json:"variable_duration"`
}

func (c Contract) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported source origin schema %q", c.SchemaVersion)
	}
	for label, value := range map[string]string{
		"case ID":                    c.CaseID,
		"fixture ID":                 c.FixtureID,
		"FFmpeg version":             c.FFmpegVersion,
		"FFprobe version":            c.FFprobeVersion,
		"timestamp plan version":     c.TimestampPlanVersion,
		"timestamp plan hash":        c.TimestampPlanHash,
		"boundary evidence version": c.BoundaryEvidenceVersion,
		"boundary evidence hash":    c.BoundaryEvidenceHash,
		"A/V sync evidence version": c.AVSyncEvidenceVersion,
		"A/V sync evidence hash":    c.AVSyncEvidenceHash,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", label)
		}
	}
	if c.SourceMode != ModeCFR && c.SourceMode != ModeVFR {
		return fmt.Errorf("unsupported source mode %q", c.SourceMode)
	}
	if c.DeclaredFrameRateNumerator <= 0 || c.DeclaredFrameRateDenominator <= 0 {
		return fmt.Errorf("declared frame rate is invalid")
	}
	wantRateMilli := int(math.Round(float64(c.DeclaredFrameRateNumerator) * 1000 / float64(c.DeclaredFrameRateDenominator)))
	if c.DeclaredFrameRateMilli != wantRateMilli {
		return fmt.Errorf("declared frame rate projection is inconsistent")
	}
	if c.OriginClass != classifyOrigin(c.SourceOffsetMicros) {
		return fmt.Errorf("source origin class is inconsistent")
	}
	if c.OriginToleranceMicros != MaxOriginErrorMicros || c.ExpectedBoundaryMicros <= 0 {
		return fmt.Errorf("source origin policy is invalid")
	}
	if c.TimestampPlanVersion != transcodetimestamp.SchemaVersion || !isSHA256(c.TimestampPlanHash) {
		return fmt.Errorf("timestamp plan identity is invalid")
	}
	if c.BoundaryEvidenceVersion != transcodeboundary.SchemaVersion || !isSHA256(c.BoundaryEvidenceHash) {
		return fmt.Errorf("boundary evidence identity is invalid")
	}
	if c.AVSyncEvidenceVersion != transcodeavsync.SchemaVersion || !isSHA256(c.AVSyncEvidenceHash) {
		return fmt.Errorf("A/V sync evidence identity is invalid")
	}
	if err := c.SourceVideo.validate(StreamVideo); err != nil {
		return fmt.Errorf("source video: %w", err)
	}
	if err := c.SourceAudio.validate(StreamAudio); err != nil {
		return fmt.Errorf("source audio: %w", err)
	}
	if abs64(c.SourceVideo.FirstPTSMicros-c.SourceOffsetMicros) > c.OriginToleranceMicros {
		return fmt.Errorf("source video origin does not match declared offset")
	}
	if abs64(c.SourceAudio.FirstPTSMicros-c.SourceOffsetMicros) > c.OriginToleranceMicros {
		return fmt.Errorf("source audio origin does not match declared offset")
	}
	wantVariable := c.SourceMode == ModeVFR
	if c.SourceVideo.VariableDuration != wantVariable {
		return fmt.Errorf("source video cadence classification is inconsistent")
	}
	if c.SourceAudio.VariableDuration {
		return fmt.Errorf("source audio cadence must remain packet-stable")
	}
	plan := transcodetimestamp.Default()
	boundaryMS := int64(math.Round(float64(c.ExpectedBoundaryMicros) / 1000))
	if err := plan.VerifyObservedStart(0, c.NormalizedStartupVideoStartMS, c.NormalizedStartupAudioStartMS); err != nil {
		return fmt.Errorf("normalized startup origin: %w", err)
	}
	if err := plan.VerifyObservedStart(boundaryMS, c.NormalizedContinuationVideoMS, c.NormalizedContinuationAudioMS); err != nil {
		return fmt.Errorf("normalized continuation origin: %w", err)
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("source origin evidence v1 cannot authorize seamless playback")
	}
	return nil
}

func (s StreamEvidence) validate(expectedKind string) error {
	if s.Kind != expectedKind {
		return fmt.Errorf("stream kind %q does not match %q", s.Kind, expectedKind)
	}
	if strings.TrimSpace(s.TimeBase) == "" || s.PacketCount < 2 || s.DistinctDurations <= 0 {
		return fmt.Errorf("stream identity is incomplete")
	}
	firstPTS, err := transcodeboundary.TicksToMicros(s.FirstPTS, s.TimeBase)
	if err != nil {
		return err
	}
	firstDTS, err := transcodeboundary.TicksToMicros(s.FirstDTS, s.TimeBase)
	if err != nil {
		return err
	}
	minDuration, err := transcodeboundary.TicksToMicros(s.MinPacketDurationTicks, s.TimeBase)
	if err != nil || minDuration <= 0 {
		return fmt.Errorf("minimum packet duration is invalid")
	}
	maxDuration, err := transcodeboundary.TicksToMicros(s.MaxPacketDurationTicks, s.TimeBase)
	if err != nil || maxDuration < minDuration {
		return fmt.Errorf("maximum packet duration is invalid")
	}
	if s.FirstPTSMicros != firstPTS || s.FirstDTSMicros != firstDTS ||
		s.MinPacketDurationMicros != minDuration || s.MaxPacketDurationMicros != maxDuration {
		return fmt.Errorf("stream microsecond projection is inconsistent")
	}
	spread := maxDuration - minDuration
	if s.DurationSpreadMicros != spread || s.VariableDuration != (spread >= VFRSpreadThresholdMicros) {
		return fmt.Errorf("stream duration spread is inconsistent")
	}
	return nil
}

func classifyOrigin(offsetMicros int64) string {
	switch {
	case offsetMicros > 0:
		return OriginPositive
	case offsetMicros < 0:
		return OriginNegative
	default:
		return OriginZero
	}
}

func (c Contract) CanonicalJSON() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	content, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal source origin evidence: %w", err)
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
