package encodingplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const SchemaVersion = "hls-encoding-plan-v1"

// Plan is the immutable output compatibility contract shared by independent
// transcode Jobs. It deliberately excludes execution range, priority, Worker,
// Lease, Attempt, filesystem and hardware-backend identity.
type Plan struct {
	SchemaVersion string        `json:"schema_version"`
	ProfileID     string        `json:"profile_id"`
	Transport     TransportPlan `json:"transport"`
	Video         VideoPlan     `json:"video"`
	Audio         AudioPlan     `json:"audio"`
}

type TransportPlan struct {
	Protocol          string `json:"protocol"`
	Container         string `json:"container"`
	SegmentFormat     string `json:"segment_format"`
	SegmentDurationMS int64  `json:"segment_duration_ms"`
}

type VideoPlan struct {
	Codec                string `json:"codec"`
	Width                int    `json:"width"`
	Height               int    `json:"height"`
	PixelFormatContract  string `json:"pixel_format_contract"`
	FrameRatePolicy      string `json:"frame_rate_policy"`
	SourceFrameRateMilli int    `json:"source_frame_rate_milli"`
	GOPSize              int    `json:"gop_size"`
	KeyframeIntervalMS   int64  `json:"keyframe_interval_ms"`
	ForceKeyframes       bool   `json:"force_keyframes"`
	SceneCut             bool   `json:"scene_cut"`
	ColorPolicy          string `json:"color_policy"`
	ColorPrimaries       string `json:"color_primaries"`
	Transfer             string `json:"transfer"`
	Matrix               string `json:"matrix"`
}

type AudioPlan struct {
	Codec            string `json:"codec"`
	Bitrate          string `json:"bitrate"`
	Channels         int    `json:"channels"`
	Track            int    `json:"track"`
	SampleRatePolicy string `json:"sample_rate_policy"`
}

func (p Plan) Validate() error {
	if p.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported encoding plan schema %q", p.SchemaVersion)
	}
	if strings.TrimSpace(p.ProfileID) == "" {
		return fmt.Errorf("encoding plan profile is required")
	}
	if p.Transport.Protocol != "hls" || p.Transport.Container != "mpegts" || p.Transport.SegmentFormat != "mpegts" {
		return fmt.Errorf("encoding plan transport is unsupported")
	}
	if p.Transport.SegmentDurationMS <= 0 {
		return fmt.Errorf("encoding plan segment duration must be positive")
	}
	if p.Video.Codec != "h264" || p.Video.Width <= 0 || p.Video.Height <= 0 {
		return fmt.Errorf("encoding plan video contract is incomplete")
	}
	if p.Video.PixelFormatContract == "" || p.Video.FrameRatePolicy == "" || p.Video.GOPSize <= 0 || p.Video.KeyframeIntervalMS <= 0 {
		return fmt.Errorf("encoding plan video timing contract is incomplete")
	}
	if p.Audio.Codec != "aac" || strings.TrimSpace(p.Audio.Bitrate) == "" || p.Audio.Channels <= 0 || p.Audio.SampleRatePolicy == "" {
		return fmt.Errorf("encoding plan audio contract is incomplete")
	}
	return nil
}

// CanonicalJSON is stable because Plan contains only ordered structs and scalar
// fields. Maps and runtime argument slices are intentionally excluded.
func (p Plan) CanonicalJSON() (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	content, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal encoding plan: %w", err)
	}
	return string(content), nil
}

func (p Plan) Hash() (string, error) {
	canonical, err := p.CanonicalJSON()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:]), nil
}

func Identity(p Plan) (version, hash, canonical string, err error) {
	canonical, err = p.CanonicalJSON()
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256([]byte(canonical))
	return p.SchemaVersion, hex.EncodeToString(digest[:]), canonical, nil
}
