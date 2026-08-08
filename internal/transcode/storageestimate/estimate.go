package storageestimate

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	minimumReservationBytes = int64(64 * 1024 * 1024)
	unknownDurationFloor    = int64(2 * 1024 * 1024 * 1024)
	bitrateSafetyFactor     = 1.35
	sourceSafetyFactor      = 1.10
)

type Input struct {
	VideoBitrate string
	AudioBitrate string
	DurationMS   int64
	SourceBytes  int64
}

type Result struct {
	EstimatedBytes int64   `json:"estimated_bytes"`
	PayloadBytes   int64   `json:"payload_bytes"`
	DurationMS     int64   `json:"duration_ms"`
	BitrateBPS     int64   `json:"bitrate_bps"`
	SafetyFactor   float64 `json:"safety_factor"`
	Fallback       string  `json:"fallback,omitempty"`
}

// Estimate predicts the peak bytes required by one immutable HLS Artifact.
// Runtime HLS uses CRF and can exceed the nominal catalog bitrate, so known
// duration estimates include a 35% envelope for VBR excursions, transport
// overhead, manifests and segment boundary variance. Unknown duration fails
// conservatively to the source size or a 2 GiB floor.
func Estimate(input Input) (Result, error) {
	videoBPS, err := ParseBitrate(input.VideoBitrate)
	if err != nil {
		return Result{}, fmt.Errorf("video bitrate: %w", err)
	}
	audioBPS, err := ParseBitrate(input.AudioBitrate)
	if err != nil {
		return Result{}, fmt.Errorf("audio bitrate: %w", err)
	}
	bitrateBPS := videoBPS + audioBPS
	if input.DurationMS > 0 && bitrateBPS > 0 {
		payload := int64(math.Ceil(float64(bitrateBPS) * (float64(input.DurationMS) / 1000) / 8))
		estimated := int64(math.Ceil(float64(payload) * bitrateSafetyFactor))
		if estimated < minimumReservationBytes {
			estimated = minimumReservationBytes
		}
		return Result{
			EstimatedBytes: estimated,
			PayloadBytes:   payload,
			DurationMS:     input.DurationMS,
			BitrateBPS:     bitrateBPS,
			SafetyFactor:   bitrateSafetyFactor,
		}, nil
	}

	fallback := unknownDurationFloor
	fallbackReason := "unknown_duration_floor"
	if input.SourceBytes > 0 {
		fallback = int64(math.Ceil(float64(input.SourceBytes) * sourceSafetyFactor))
		fallbackReason = "source_size"
		if fallback < minimumReservationBytes {
			fallback = minimumReservationBytes
		}
	}
	return Result{
		EstimatedBytes: fallback,
		DurationMS:     input.DurationMS,
		BitrateBPS:     bitrateBPS,
		SafetyFactor:   sourceSafetyFactor,
		Fallback:       fallbackReason,
	}, nil
}

func ParseBitrate(value string) (int64, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return 0, nil
	}
	multiplier := float64(1)
	switch {
	case strings.HasSuffix(value, "kbps"):
		value = strings.TrimSpace(strings.TrimSuffix(value, "kbps"))
		multiplier = 1000
	case strings.HasSuffix(value, "mbps"):
		value = strings.TrimSpace(strings.TrimSuffix(value, "mbps"))
		multiplier = 1000 * 1000
	case strings.HasSuffix(value, "k"):
		value = strings.TrimSpace(strings.TrimSuffix(value, "k"))
		multiplier = 1000
	case strings.HasSuffix(value, "m"):
		value = strings.TrimSpace(strings.TrimSuffix(value, "m"))
		multiplier = 1000 * 1000
	}
	numeric, err := strconv.ParseFloat(value, 64)
	if err != nil || numeric < 0 {
		return 0, fmt.Errorf("invalid bitrate %q", value)
	}
	return int64(math.Ceil(numeric * multiplier)), nil
}
