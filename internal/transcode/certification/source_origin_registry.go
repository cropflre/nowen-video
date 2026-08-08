package certification

import (
	"fmt"
	"math"
	"strings"

	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
)

const (
	SourceOriginMatrixSchemaVersion = "ffmpeg-source-origin-matrix-v1"
	sourceOriginDurationSeconds     = 40
	sourceOriginAudioSampleRate     = 48_000
)

const (
	SourceOriginCaseCFR24Zero       = "source-cfr-24-origin-zero-v1"
	SourceOriginCaseCFR25Zero       = "source-cfr-25-origin-zero-v1"
	SourceOriginCaseCFR2997Zero     = "source-cfr-30000-1001-origin-zero-v1"
	SourceOriginCaseVFR2430Zero     = "source-vfr-24-30-origin-zero-v1"
	SourceOriginCaseCFR30Positive5S = "source-cfr-30-origin-positive-5s-v1"
	SourceOriginCaseCFR30Negative2S = "source-cfr-30-origin-negative-2s-v1"
)

type SourceOriginCaseSpec struct {
	ID                           string `json:"id"`
	Description                  string `json:"description"`
	FixtureID                    string `json:"fixture_id"`
	SourceMode                   string `json:"source_mode"`
	DeclaredFrameRateNumerator   int64  `json:"declared_frame_rate_numerator"`
	DeclaredFrameRateDenominator int64  `json:"declared_frame_rate_denominator"`
	SourceOffsetMicros           int64  `json:"source_offset_micros"`
	AudioSampleRate              int    `json:"audio_sample_rate"`
	GOPSize                      int    `json:"gop_size"`
	ExpectedBoundaryMicros       int64  `json:"expected_boundary_micros"`
}

var sourceOriginCaseSpecs = []SourceOriginCaseSpec{
	newSourceOriginCase(SourceOriginCaseCFR24Zero, "24 fps CFR source at zero origin", transcodesourceorigin.ModeCFR, 24, 1, 0, 48),
	newSourceOriginCase(SourceOriginCaseCFR25Zero, "25 fps CFR source at zero origin", transcodesourceorigin.ModeCFR, 25, 1, 0, 50),
	newSourceOriginCase(SourceOriginCaseCFR2997Zero, "30000/1001 fps CFR source at zero origin", transcodesourceorigin.ModeCFR, 30_000, 1_001, 0, 60),
	newSourceOriginCase(SourceOriginCaseVFR2430Zero, "VFR source with 24 fps then 30 fps cadence", transcodesourceorigin.ModeVFR, 27, 1, 0, 60),
	newSourceOriginCase(SourceOriginCaseCFR30Positive5S, "30 fps CFR source with positive five-second origin", transcodesourceorigin.ModeCFR, 30, 1, 5_000_000, 60),
	newSourceOriginCase(SourceOriginCaseCFR30Negative2S, "30 fps CFR source with negative two-second origin", transcodesourceorigin.ModeCFR, 30, 1, -2_000_000, 60),
}

func newSourceOriginCase(id, description, mode string, rateNumerator, rateDenominator, offsetMicros int64, gopSize int) SourceOriginCaseSpec {
	return SourceOriginCaseSpec{
		ID:                           id,
		Description:                  description,
		FixtureID:                    "fixture-" + id,
		SourceMode:                   mode,
		DeclaredFrameRateNumerator:   rateNumerator,
		DeclaredFrameRateDenominator: rateDenominator,
		SourceOffsetMicros:           offsetMicros,
		AudioSampleRate:              sourceOriginAudioSampleRate,
		GOPSize:                      gopSize,
		ExpectedBoundaryMicros:       boundaryReferenceMicros,
	}
}

func (s SourceOriginCaseSpec) Validate() error {
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.Description) == "" || strings.TrimSpace(s.FixtureID) == "" {
		return fmt.Errorf("source origin case identity is incomplete")
	}
	if s.SourceMode != transcodesourceorigin.ModeCFR && s.SourceMode != transcodesourceorigin.ModeVFR {
		return fmt.Errorf("unsupported source origin mode %q", s.SourceMode)
	}
	if s.DeclaredFrameRateNumerator <= 0 || s.DeclaredFrameRateDenominator <= 0 || s.GOPSize <= 0 {
		return fmt.Errorf("source origin frame-rate policy is invalid")
	}
	if s.AudioSampleRate != sourceOriginAudioSampleRate || s.ExpectedBoundaryMicros != boundaryReferenceMicros {
		return fmt.Errorf("source origin media policy is invalid")
	}
	if s.SourceMode == transcodesourceorigin.ModeVFR && (s.DeclaredFrameRateNumerator != 27 || s.DeclaredFrameRateDenominator != 1) {
		return fmt.Errorf("VFR fixture must declare the deterministic 27 fps mean")
	}
	return nil
}

func (s SourceOriginCaseSpec) DeclaredFrameRateMilli() int {
	return int(math.Round(float64(s.DeclaredFrameRateNumerator) * 1000 / float64(s.DeclaredFrameRateDenominator)))
}

func AvailableSourceOriginCases() []SourceOriginCaseSpec {
	result := make([]SourceOriginCaseSpec, len(sourceOriginCaseSpecs))
	copy(result, sourceOriginCaseSpecs)
	return result
}

func LookupSourceOriginCase(id string) (SourceOriginCaseSpec, bool) {
	candidate := strings.TrimSpace(id)
	for _, spec := range sourceOriginCaseSpecs {
		if spec.ID == candidate {
			return spec, true
		}
	}
	return SourceOriginCaseSpec{}, false
}
