package attestation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/nowen-video/nowen-video/internal/transcode/encodingplan"
)

const SchemaVersion = "hls-produced-media-attestation-v1"

const (
	ScopeFirstSegment = "first_segment"
	ScopeComplete     = "complete"
)

// Attestation is immutable evidence derived from produced HLS media. Unlike an
// Encoding Plan, it records what ffprobe actually observed in the first and
// last materialized segments. Execution identity, filesystem paths and Lease
// tokens are intentionally excluded from the canonical payload.
type Attestation struct {
	SchemaVersion       string          `json:"schema_version"`
	Scope               string          `json:"scope"`
	EncodingPlanVersion string          `json:"encoding_plan_version"`
	EncodingPlanHash    string          `json:"encoding_plan_hash"`
	SegmentCount        int             `json:"segment_count"`
	First               SegmentEvidence `json:"first"`
	Last                SegmentEvidence `json:"last"`
}

type SegmentEvidence struct {
	Name     string         `json:"name"`
	Video    StreamIdentity `json:"video"`
	Audio    StreamIdentity `json:"audio"`
	Timeline Timeline       `json:"timeline"`
}

type StreamIdentity struct {
	CodecName      string `json:"codec_name"`
	Profile        string `json:"profile,omitempty"`
	Level          int    `json:"level,omitempty"`
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	PixelFormat    string `json:"pixel_format,omitempty"`
	ColorPrimaries string `json:"color_primaries,omitempty"`
	ColorTransfer  string `json:"color_transfer,omitempty"`
	ColorMatrix    string `json:"color_matrix,omitempty"`
	Channels       int    `json:"channels,omitempty"`
	SampleRate     int    `json:"sample_rate,omitempty"`
	FrameRateMilli int    `json:"frame_rate_milli,omitempty"`
	TimeBase       string `json:"time_base"`
}

type Timeline struct {
	Video PacketRange `json:"video"`
	Audio PacketRange `json:"audio"`
}

type PacketRange struct {
	FirstPTS int64 `json:"first_pts"`
	FirstDTS int64 `json:"first_dts"`
	LastPTS  int64 `json:"last_pts"`
	LastDTS  int64 `json:"last_dts"`
	EndPTS   int64 `json:"end_pts"`
	StartMS  int64 `json:"start_ms"`
	EndMS    int64 `json:"end_ms"`
}

func (a Attestation) Validate() error {
	if a.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported attestation schema %q", a.SchemaVersion)
	}
	if a.Scope != ScopeFirstSegment && a.Scope != ScopeComplete {
		return fmt.Errorf("unsupported attestation scope %q", a.Scope)
	}
	if strings.TrimSpace(a.EncodingPlanVersion) == "" || strings.TrimSpace(a.EncodingPlanHash) == "" {
		return fmt.Errorf("encoding plan identity is required")
	}
	if a.SegmentCount <= 0 {
		return fmt.Errorf("attestation segment count must be positive")
	}
	if err := a.First.Validate(); err != nil {
		return fmt.Errorf("first segment evidence: %w", err)
	}
	if err := a.Last.Validate(); err != nil {
		return fmt.Errorf("last segment evidence: %w", err)
	}
	if a.Scope == ScopeFirstSegment && a.First.Name != a.Last.Name {
		return fmt.Errorf("first-segment attestation must use one segment")
	}
	if err := compatibleStreamIdentity(a.First.Video, a.Last.Video, "video"); err != nil {
		return err
	}
	if err := compatibleStreamIdentity(a.First.Audio, a.Last.Audio, "audio"); err != nil {
		return err
	}
	return nil
}

func (s SegmentEvidence) Validate() error {
	if s.Name == "" || s.Name != path.Base(s.Name) {
		return fmt.Errorf("segment name is unsafe")
	}
	if err := s.Video.validateVideo(); err != nil {
		return err
	}
	if err := s.Audio.validateAudio(); err != nil {
		return err
	}
	if err := s.Timeline.Video.Validate(); err != nil {
		return fmt.Errorf("video timeline: %w", err)
	}
	if err := s.Timeline.Audio.Validate(); err != nil {
		return fmt.Errorf("audio timeline: %w", err)
	}
	return nil
}

func (s StreamIdentity) validateVideo() error {
	if s.CodecName == "" || s.Width <= 0 || s.Height <= 0 || s.PixelFormat == "" || s.TimeBase == "" {
		return fmt.Errorf("video stream identity is incomplete")
	}
	return nil
}

func (s StreamIdentity) validateAudio() error {
	if s.CodecName == "" || s.Channels <= 0 || s.SampleRate <= 0 || s.TimeBase == "" {
		return fmt.Errorf("audio stream identity is incomplete")
	}
	return nil
}

func (r PacketRange) Validate() error {
	if r.EndMS <= r.StartMS {
		return fmt.Errorf("packet range has no positive duration")
	}
	if r.EndPTS < r.LastPTS {
		return fmt.Errorf("packet range end precedes last pts")
	}
	return nil
}

func (a Attestation) CanonicalJSON() (string, error) {
	if err := a.Validate(); err != nil {
		return "", err
	}
	content, err := json.Marshal(a)
	if err != nil {
		return "", fmt.Errorf("marshal produced media attestation: %w", err)
	}
	return string(content), nil
}

func (a Attestation) Hash() (string, error) {
	canonical, err := a.CanonicalJSON()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:]), nil
}

func Identity(a Attestation) (version, hash, canonical string, err error) {
	canonical, err = a.CanonicalJSON()
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256([]byte(canonical))
	return a.SchemaVersion, hex.EncodeToString(digest[:]), canonical, nil
}

// VerifyAgainstEncodingPlan proves that observed output matches the immutable
// output contract. Profile/level and exact source frame rate remain evidence in
// v1 but are not hard gates because hardware encoders may lawfully choose them.
func VerifyAgainstEncodingPlan(a Attestation, planVersion, planHash, planJSON string) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if a.EncodingPlanVersion != planVersion || a.EncodingPlanHash != planHash {
		return fmt.Errorf("attestation encoding plan identity mismatch")
	}
	var plan encodingplan.Plan
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		return fmt.Errorf("decode encoding plan: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return err
	}
	version, hash, _, err := encodingplan.Identity(plan)
	if err != nil {
		return err
	}
	if version != planVersion || hash != planHash {
		return fmt.Errorf("persisted encoding plan identity is invalid")
	}
	for label, segment := range map[string]SegmentEvidence{"first": a.First, "last": a.Last} {
		if err := verifySegmentAgainstPlan(segment, plan); err != nil {
			return fmt.Errorf("%s segment: %w", label, err)
		}
	}
	return nil
}

func verifySegmentAgainstPlan(segment SegmentEvidence, plan encodingplan.Plan) error {
	if segment.Video.CodecName != plan.Video.Codec {
		return fmt.Errorf("video codec %q does not match %q", segment.Video.CodecName, plan.Video.Codec)
	}
	if segment.Video.Width != plan.Video.Width || segment.Video.Height != plan.Video.Height {
		return fmt.Errorf("video dimensions %dx%d do not match %dx%d", segment.Video.Width, segment.Video.Height, plan.Video.Width, plan.Video.Height)
	}
	if !pixelFormatMatches(plan.Video.PixelFormatContract, segment.Video.PixelFormat) {
		return fmt.Errorf("pixel format %q does not match contract %q", segment.Video.PixelFormat, plan.Video.PixelFormatContract)
	}
	if plan.Video.ColorPrimaries != "source" && segment.Video.ColorPrimaries != plan.Video.ColorPrimaries {
		return fmt.Errorf("color primaries %q do not match %q", segment.Video.ColorPrimaries, plan.Video.ColorPrimaries)
	}
	if plan.Video.Transfer != "source" && segment.Video.ColorTransfer != plan.Video.Transfer {
		return fmt.Errorf("color transfer %q does not match %q", segment.Video.ColorTransfer, plan.Video.Transfer)
	}
	if plan.Video.Matrix != "source" && segment.Video.ColorMatrix != plan.Video.Matrix {
		return fmt.Errorf("color matrix %q does not match %q", segment.Video.ColorMatrix, plan.Video.Matrix)
	}
	if segment.Audio.CodecName != plan.Audio.Codec {
		return fmt.Errorf("audio codec %q does not match %q", segment.Audio.CodecName, plan.Audio.Codec)
	}
	if segment.Audio.Channels != plan.Audio.Channels {
		return fmt.Errorf("audio channels %d do not match %d", segment.Audio.Channels, plan.Audio.Channels)
	}
	return nil
}

func pixelFormatMatches(contract, actual string) bool {
	switch contract {
	case "yuv420p-8bit":
		return actual == "yuv420p"
	default:
		return contract == actual
	}
}

// BridgeCompatible verifies actual codec/sample contracts across two Artifacts.
// It intentionally does not claim timestamp continuity. The caller must keep an
// HLS discontinuity until an explicit timestamp relation is proven separately.
func BridgeCompatible(startup, continuation Attestation) error {
	if err := startup.Validate(); err != nil {
		return fmt.Errorf("startup attestation: %w", err)
	}
	if err := continuation.Validate(); err != nil {
		return fmt.Errorf("continuation attestation: %w", err)
	}
	if startup.EncodingPlanVersion != continuation.EncodingPlanVersion || startup.EncodingPlanHash != continuation.EncodingPlanHash {
		return fmt.Errorf("bridge encoding plan identity mismatch")
	}
	if err := compatibleStreamIdentity(startup.Last.Video, continuation.First.Video, "video"); err != nil {
		return err
	}
	if err := compatibleStreamIdentity(startup.Last.Audio, continuation.First.Audio, "audio"); err != nil {
		return err
	}
	return nil
}

func compatibleStreamIdentity(left, right StreamIdentity, kind string) error {
	if left.CodecName != right.CodecName || left.TimeBase != right.TimeBase {
		return fmt.Errorf("%s stream codec/time base mismatch", kind)
	}
	if kind == "video" {
		if left.Width != right.Width || left.Height != right.Height || left.PixelFormat != right.PixelFormat ||
			left.ColorPrimaries != right.ColorPrimaries || left.ColorTransfer != right.ColorTransfer || left.ColorMatrix != right.ColorMatrix {
			return fmt.Errorf("video stream identity mismatch")
		}
		return nil
	}
	if left.Channels != right.Channels || left.SampleRate != right.SampleRate {
		return fmt.Errorf("audio stream identity mismatch")
	}
	return nil
}
