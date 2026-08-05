package model

import (
	"encoding/json"
	"math"
	"time"
)

const MediaProbeVersion = "ffprobe-v1"

// MediaProbeAudioStream is the normalized audio information consumed by the
// playback planner. It deliberately excludes request headers and source URLs.
type MediaProbeAudioStream struct {
	Index         int    `json:"index"`
	Codec         string `json:"codec"`
	Channels      int    `json:"channels"`
	ChannelLayout string `json:"channel_layout"`
	SampleRate    int    `json:"sample_rate"`
	Language      string `json:"language,omitempty"`
	Title         string `json:"title,omitempty"`
	Default       bool   `json:"default"`
}

// MediaProbeRecord is the authoritative technical description of a media
// source. Media keeps user-facing summary fields for compatibility, while this
// table owns frame rate, colour metadata and cache invalidation information.
type MediaProbeRecord struct {
	MediaID           string `json:"media_id" gorm:"primaryKey;type:text"`
	SourceFingerprint string `json:"source_fingerprint" gorm:"index;type:text;not null"`
	SourcePath        string `json:"source_path" gorm:"type:text;not null"`
	SourceSize        int64  `json:"source_size"`
	SourceModTimeNS   int64  `json:"source_mod_time_ns"`
	ProbeVersion      string `json:"probe_version" gorm:"index;type:text;not null"`

	FormatName string `json:"format_name" gorm:"type:text"`
	DurationMS int64  `json:"duration_ms"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`

	FrameRateNum int `json:"frame_rate_num"`
	FrameRateDen int `json:"frame_rate_den"`

	VideoCodec     string `json:"video_codec" gorm:"type:text"`
	PixelFormat    string `json:"pixel_format" gorm:"type:text"`
	BitDepth       int    `json:"bit_depth"`
	ColorTransfer  string `json:"color_transfer" gorm:"type:text"`
	ColorPrimaries string `json:"color_primaries" gorm:"type:text"`
	ColorSpace     string `json:"color_space" gorm:"type:text"`
	ColorRange     string `json:"color_range" gorm:"type:text"`
	HDR            bool   `json:"hdr" gorm:"index"`

	AudioStreamsJSON string    `json:"-" gorm:"type:text"`
	ProbedAt         time.Time `json:"probed_at" gorm:"index"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (MediaProbeRecord) TableName() string { return "media_probe_cache" }

func (p *MediaProbeRecord) FrameRate() float64 {
	if p == nil || p.FrameRateNum <= 0 || p.FrameRateDen <= 0 {
		return 0
	}
	return float64(p.FrameRateNum) / float64(p.FrameRateDen)
}

// GOPSize returns a key-frame interval aligned to the actual source frame rate.
// It is clamped to protect FFmpeg from corrupt or absurd probe metadata.
func (p *MediaProbeRecord) GOPSize(segmentSeconds int) int {
	if segmentSeconds <= 0 {
		segmentSeconds = 2
	}
	fps := p.FrameRate()
	if fps <= 0 || math.IsNaN(fps) || math.IsInf(fps, 0) {
		fps = 25
	}
	gop := int(math.Round(fps * float64(segmentSeconds)))
	if gop < 12 {
		return 12
	}
	if gop > 240 {
		return 240
	}
	return gop
}

func (p *MediaProbeRecord) SetAudioStreams(streams []MediaProbeAudioStream) error {
	data, err := json.Marshal(streams)
	if err != nil {
		return err
	}
	p.AudioStreamsJSON = string(data)
	return nil
}

func (p *MediaProbeRecord) AudioStreams() []MediaProbeAudioStream {
	if p == nil || p.AudioStreamsJSON == "" {
		return nil
	}
	var streams []MediaProbeAudioStream
	if err := json.Unmarshal([]byte(p.AudioStreamsJSON), &streams); err != nil {
		return nil
	}
	return streams
}
