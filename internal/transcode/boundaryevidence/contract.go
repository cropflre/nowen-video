package boundaryevidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"path"
	"strconv"
	"strings"
)

const SchemaVersion = "hls-boundary-packet-evidence-v1"

const (
	StreamVideo = "video"
	StreamAudio = "audio"

	WindowTail = "tail"
	WindowHead = "head"

	StatusAligned             = "aligned"
	StatusSinglePacketGap     = "single_packet_gap"
	StatusMultiPacketGap      = "multi_packet_gap"
	StatusSinglePacketOverlap = "single_packet_overlap"
	StatusMultiPacketOverlap  = "multi_packet_overlap"

	MaxWindowPackets = 6
)

// Contract is immutable packet-level evidence around one Startup-to-
// Continuation boundary. It is deliberately diagnostic: v1 can explain packet
// quantization and encoder delay, but can never authorize seamless playback.
type Contract struct {
	SchemaVersion                  string         `json:"schema_version"`
	CaseID                         string         `json:"case_id"`
	FixtureID                      string         `json:"fixture_id"`
	ExpectedBoundaryMicros         int64          `json:"expected_boundary_micros"`
	FFmpegVersion                  string         `json:"ffmpeg_version"`
	FFprobeVersion                 string         `json:"ffprobe_version"`
	EncodingPlanVersion            string         `json:"encoding_plan_version"`
	EncodingPlanHash               string         `json:"encoding_plan_hash"`
	TimestampPlanVersion           string         `json:"timestamp_plan_version"`
	TimestampPlanHash              string         `json:"timestamp_plan_hash"`
	StartupAttestationVersion      string         `json:"startup_attestation_version"`
	StartupAttestationHash         string         `json:"startup_attestation_hash"`
	ContinuationAttestationVersion string         `json:"continuation_attestation_version"`
	ContinuationAttestationHash    string         `json:"continuation_attestation_hash"`
	Video                          StreamEvidence `json:"video"`
	Audio                          StreamEvidence `json:"audio"`
	SeamlessAllowed                bool           `json:"seamless_allowed"`
	DiscontinuityRequired          bool           `json:"discontinuity_required"`
}

type StreamEvidence struct {
	Kind                        string              `json:"kind"`
	TimeBase                    string              `json:"time_base"`
	SampleRate                  int                 `json:"sample_rate,omitempty"`
	FrameRateMilli              int                 `json:"frame_rate_milli,omitempty"`
	Startup                     SegmentWindow       `json:"startup"`
	Continuation                SegmentWindow       `json:"continuation"`
	PresentationDeltaTicks      int64               `json:"presentation_delta_ticks"`
	PresentationDeltaMicros     int64               `json:"presentation_delta_micros"`
	DecodeDeltaTicks            int64               `json:"decode_delta_ticks"`
	DecodeDeltaMicros           int64               `json:"decode_delta_micros"`
	NominalPacketDurationTicks  int64               `json:"nominal_packet_duration_ticks"`
	NominalPacketDurationMicros int64               `json:"nominal_packet_duration_micros"`
	ToleranceMicros             int64               `json:"tolerance_micros"`
	BoundaryUnitsMilli          int64               `json:"boundary_units_milli"`
	Status                      string              `json:"status"`
	AudioDelay                  *AudioDelayEvidence `json:"audio_delay,omitempty"`
}

type SegmentWindow struct {
	SegmentName  string           `json:"segment_name"`
	PacketCount  int              `json:"packet_count"`
	Position     string           `json:"position"`
	Packets      []PacketEvidence `json:"packets"`
	EarliestPTS  int64            `json:"earliest_pts"`
	LatestEndPTS int64            `json:"latest_end_pts"`
	FirstDTS     int64            `json:"first_dts"`
	LastEndDTS   int64            `json:"last_end_dts"`
}

type PacketEvidence struct {
	Ordinal        int              `json:"ordinal"`
	PTS            int64            `json:"pts"`
	DTS            int64            `json:"dts"`
	Duration       int64            `json:"duration"`
	PTSMicros      int64            `json:"pts_micros"`
	DTSMicros      int64            `json:"dts_micros"`
	DurationMicros int64            `json:"duration_micros"`
	KeyFrame       bool             `json:"key_frame"`
	SideData       []PacketSideData `json:"side_data,omitempty"`
}

type PacketSideData struct {
	Type           string `json:"type"`
	SkipSamples    int64  `json:"skip_samples,omitempty"`
	DiscardPadding int64  `json:"discard_padding,omitempty"`
}

type AudioDelayEvidence struct {
	NominalPacketSamples       int64 `json:"nominal_packet_samples"`
	BoundaryDeltaSamples       int64 `json:"boundary_delta_samples"`
	StartupSkipSamples         int64 `json:"startup_skip_samples"`
	ContinuationSkipSamples    int64 `json:"continuation_skip_samples"`
	StartupDiscardPadding      int64 `json:"startup_discard_padding"`
	ContinuationDiscardPadding int64 `json:"continuation_discard_padding"`
	SideDataObserved           bool  `json:"side_data_observed"`
}

func (c Contract) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported boundary evidence schema %q", c.SchemaVersion)
	}
	for label, value := range map[string]string{
		"case ID":                          c.CaseID,
		"fixture ID":                       c.FixtureID,
		"FFmpeg version":                   c.FFmpegVersion,
		"FFprobe version":                  c.FFprobeVersion,
		"encoding plan version":            c.EncodingPlanVersion,
		"encoding plan hash":               c.EncodingPlanHash,
		"timestamp plan version":           c.TimestampPlanVersion,
		"timestamp plan hash":              c.TimestampPlanHash,
		"startup attestation version":      c.StartupAttestationVersion,
		"startup attestation hash":         c.StartupAttestationHash,
		"continuation attestation version": c.ContinuationAttestationVersion,
		"continuation attestation hash":    c.ContinuationAttestationHash,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", label)
		}
	}
	if c.ExpectedBoundaryMicros <= 0 {
		return fmt.Errorf("expected boundary must be positive")
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("boundary evidence v1 cannot authorize seamless playback")
	}
	if err := c.Video.validate(StreamVideo); err != nil {
		return fmt.Errorf("video evidence: %w", err)
	}
	if err := c.Audio.validate(StreamAudio); err != nil {
		return fmt.Errorf("audio evidence: %w", err)
	}
	return nil
}

func (s StreamEvidence) validate(expectedKind string) error {
	if s.Kind != expectedKind {
		return fmt.Errorf("stream kind %q does not match %q", s.Kind, expectedKind)
	}
	if _, _, err := parseTimeBase(s.TimeBase); err != nil {
		return err
	}
	if expectedKind == StreamVideo {
		if s.FrameRateMilli <= 0 || s.SampleRate != 0 || s.AudioDelay != nil {
			return fmt.Errorf("video stream metadata is invalid")
		}
	} else {
		if s.SampleRate <= 0 || s.FrameRateMilli != 0 || s.AudioDelay == nil {
			return fmt.Errorf("audio stream metadata is invalid")
		}
	}
	if err := s.Startup.validate(WindowTail, s.TimeBase); err != nil {
		return fmt.Errorf("startup window: %w", err)
	}
	if err := s.Continuation.validate(WindowHead, s.TimeBase); err != nil {
		return fmt.Errorf("continuation window: %w", err)
	}
	wantPresentationTicks := s.Continuation.EarliestPTS - s.Startup.LatestEndPTS
	wantDecodeTicks := s.Continuation.FirstDTS - s.Startup.LastEndDTS
	if s.PresentationDeltaTicks != wantPresentationTicks || s.DecodeDeltaTicks != wantDecodeTicks {
		return fmt.Errorf("boundary tick deltas are inconsistent")
	}
	presentationMicros, err := TicksToMicros(s.PresentationDeltaTicks, s.TimeBase)
	if err != nil {
		return err
	}
	decodeMicros, err := TicksToMicros(s.DecodeDeltaTicks, s.TimeBase)
	if err != nil {
		return err
	}
	if s.PresentationDeltaMicros != presentationMicros || s.DecodeDeltaMicros != decodeMicros {
		return fmt.Errorf("boundary microsecond deltas are inconsistent")
	}
	if s.NominalPacketDurationTicks <= 0 {
		return fmt.Errorf("nominal packet duration must be positive")
	}
	nominalMicros, err := TicksToMicros(s.NominalPacketDurationTicks, s.TimeBase)
	if err != nil || nominalMicros <= 0 {
		return fmt.Errorf("nominal packet duration is invalid")
	}
	if s.NominalPacketDurationMicros != nominalMicros {
		return fmt.Errorf("nominal packet duration projection is inconsistent")
	}
	if expectedKind == StreamVideo {
		frameRateMilli, err := FrameRateMilliFromPacketDuration(s.NominalPacketDurationTicks, s.TimeBase)
		if err != nil || s.FrameRateMilli != frameRateMilli {
			return fmt.Errorf("video frame rate projection is inconsistent")
		}
	}
	if s.ToleranceMicros != toleranceMicros(nominalMicros) {
		return fmt.Errorf("boundary tolerance is inconsistent")
	}
	if s.BoundaryUnitsMilli != signedRatioMilli(s.PresentationDeltaTicks, s.NominalPacketDurationTicks) {
		return fmt.Errorf("boundary packet-unit projection is inconsistent")
	}
	if s.Status != Classify(s.PresentationDeltaMicros, s.NominalPacketDurationMicros, s.ToleranceMicros) {
		return fmt.Errorf("boundary status is inconsistent")
	}
	if expectedKind == StreamAudio {
		if err := s.validateAudioDelay(); err != nil {
			return err
		}
	}
	return nil
}

func (s StreamEvidence) validateAudioDelay() error {
	delay := s.AudioDelay
	if delay == nil {
		return fmt.Errorf("audio delay evidence is required")
	}
	nominalSamples, err := TicksToSamples(s.NominalPacketDurationTicks, s.TimeBase, s.SampleRate)
	if err != nil || nominalSamples <= 0 {
		return fmt.Errorf("nominal audio packet samples are invalid")
	}
	boundarySamples, err := TicksToSamples(s.PresentationDeltaTicks, s.TimeBase, s.SampleRate)
	if err != nil {
		return err
	}
	startupSkip, startupDiscard, startupObserved := aggregateSideData(s.Startup.Packets)
	continuationSkip, continuationDiscard, continuationObserved := aggregateSideData(s.Continuation.Packets)
	if delay.NominalPacketSamples != nominalSamples || delay.BoundaryDeltaSamples != boundarySamples ||
		delay.StartupSkipSamples != startupSkip || delay.ContinuationSkipSamples != continuationSkip ||
		delay.StartupDiscardPadding != startupDiscard || delay.ContinuationDiscardPadding != continuationDiscard ||
		delay.SideDataObserved != (startupObserved || continuationObserved) {
		return fmt.Errorf("audio delay projection is inconsistent")
	}
	return nil
}

func (w SegmentWindow) validate(expectedPosition, timeBase string) error {
	if strings.TrimSpace(w.SegmentName) == "" || path.Base(w.SegmentName) != w.SegmentName || strings.ContainsAny(w.SegmentName, `\\/`) {
		return fmt.Errorf("segment name is unsafe")
	}
	if w.Position != expectedPosition {
		return fmt.Errorf("window position %q does not match %q", w.Position, expectedPosition)
	}
	if w.PacketCount <= 0 || len(w.Packets) == 0 || len(w.Packets) > MaxWindowPackets || w.PacketCount < len(w.Packets) {
		return fmt.Errorf("packet window size is invalid")
	}
	if w.LatestEndPTS <= w.EarliestPTS || w.LastEndDTS <= w.FirstDTS {
		return fmt.Errorf("segment packet range is invalid")
	}
	firstOrdinal := 0
	if expectedPosition == WindowTail {
		firstOrdinal = w.PacketCount - len(w.Packets)
	}
	previousDTS := int64(math.MinInt64)
	for index, packet := range w.Packets {
		if packet.Ordinal != firstOrdinal+index {
			return fmt.Errorf("packet ordinal sequence is invalid")
		}
		if err := packet.validate(timeBase); err != nil {
			return fmt.Errorf("packet %d: %w", packet.Ordinal, err)
		}
		if packet.DTS < previousDTS {
			return fmt.Errorf("packet DTS order is not monotonic")
		}
		previousDTS = packet.DTS
		if packet.PTS < w.EarliestPTS || packet.PTS+packet.Duration > w.LatestEndPTS ||
			packet.DTS < w.FirstDTS || packet.DTS+packet.Duration > w.LastEndDTS {
			return fmt.Errorf("packet is outside segment summary range")
		}
	}
	return nil
}

func (p PacketEvidence) validate(timeBase string) error {
	if p.Ordinal < 0 || p.Duration <= 0 {
		return fmt.Errorf("packet identity or duration is invalid")
	}
	ptsMicros, err := TicksToMicros(p.PTS, timeBase)
	if err != nil {
		return err
	}
	dtsMicros, err := TicksToMicros(p.DTS, timeBase)
	if err != nil {
		return err
	}
	durationMicros, err := TicksToMicros(p.Duration, timeBase)
	if err != nil || durationMicros <= 0 {
		return fmt.Errorf("packet duration projection is invalid")
	}
	if p.PTSMicros != ptsMicros || p.DTSMicros != dtsMicros || p.DurationMicros != durationMicros {
		return fmt.Errorf("packet microsecond projection is inconsistent")
	}
	for _, sideData := range p.SideData {
		if strings.TrimSpace(sideData.Type) == "" || sideData.SkipSamples < 0 || sideData.DiscardPadding < 0 {
			return fmt.Errorf("packet side data is invalid")
		}
	}
	return nil
}

func (c Contract) CanonicalJSON() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	content, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal boundary packet evidence: %w", err)
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

func Classify(deltaMicros, nominalMicros, toleranceMicros int64) string {
	if abs64(deltaMicros) <= toleranceMicros {
		return StatusAligned
	}
	if deltaMicros < 0 {
		if abs64(deltaMicros) <= nominalMicros+toleranceMicros {
			return StatusSinglePacketOverlap
		}
		return StatusMultiPacketOverlap
	}
	if deltaMicros <= nominalMicros+toleranceMicros {
		return StatusSinglePacketGap
	}
	return StatusMultiPacketGap
}

func ToleranceMicros(nominalMicros int64) int64 {
	return toleranceMicros(nominalMicros)
}

func toleranceMicros(nominalMicros int64) int64 {
	tolerance := nominalMicros / 8
	if tolerance < 1000 {
		return 1000
	}
	if tolerance > 5000 {
		return 5000
	}
	return tolerance
}

func TicksToMicros(ticks int64, timeBase string) (int64, error) {
	numerator, denominator, err := parseTimeBase(timeBase)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(float64(ticks) * float64(numerator) * 1_000_000 / float64(denominator))), nil
}

func TicksToSamples(ticks int64, timeBase string, sampleRate int) (int64, error) {
	if sampleRate <= 0 {
		return 0, fmt.Errorf("sample rate must be positive")
	}
	numerator, denominator, err := parseTimeBase(timeBase)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(float64(ticks) * float64(numerator) * float64(sampleRate) / float64(denominator))), nil
}

func FrameRateMilliFromPacketDuration(ticks int64, timeBase string) (int, error) {
	if ticks <= 0 {
		return 0, fmt.Errorf("packet duration must be positive")
	}
	numerator, denominator, err := parseTimeBase(timeBase)
	if err != nil {
		return 0, err
	}
	value := int(math.Round(float64(denominator) * 1000 / (float64(numerator) * float64(ticks))))
	if value <= 0 {
		return 0, fmt.Errorf("packet-derived frame rate is invalid")
	}
	return value, nil
}

func IsAudioDelaySideData(sideData PacketSideData) bool {
	typeName := strings.ToLower(strings.TrimSpace(sideData.Type))
	return sideData.SkipSamples != 0 || sideData.DiscardPadding != 0 ||
		strings.Contains(typeName, "skip samples") || strings.Contains(typeName, "discard padding")
}

func parseTimeBase(value string) (int64, int64, error) {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("time base %q is invalid", value)
	}
	numerator, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || numerator <= 0 {
		return 0, 0, fmt.Errorf("time base numerator is invalid")
	}
	denominator, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || denominator <= 0 {
		return 0, 0, fmt.Errorf("time base denominator is invalid")
	}
	return numerator, denominator, nil
}

func signedRatioMilli(value, unit int64) int64 {
	if unit <= 0 {
		return 0
	}
	return int64(math.Round(float64(value) * 1000 / float64(unit)))
}

func aggregateSideData(packets []PacketEvidence) (skipSamples, discardPadding int64, observed bool) {
	for _, packet := range packets {
		for _, sideData := range packet.SideData {
			if !IsAudioDelaySideData(sideData) {
				continue
			}
			observed = true
			skipSamples += sideData.SkipSamples
			discardPadding += sideData.DiscardPadding
		}
	}
	return skipSamples, discardPadding, observed
}

func abs64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
