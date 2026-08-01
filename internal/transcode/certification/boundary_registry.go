package certification

import (
	"fmt"
	"math"
	"strings"

	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
)

const BoundaryMatrixSchemaVersion = "ffmpeg-boundary-placement-matrix-v1"

const (
	BoundaryCase48KKeyframe     = "boundary-48k-keyframe-v1"
	BoundaryCase48KVideoBefore  = "boundary-48k-video-before-v1"
	BoundaryCase48KVideoAfter   = "boundary-48k-video-after-v1"
	BoundaryCase48KAudioBefore  = "boundary-48k-audio-before-v1"
	BoundaryCase48KAudioAfter   = "boundary-48k-audio-after-v1"
	BoundaryCase44K1Keyframe    = "boundary-44k1-keyframe-v1"
	BoundaryCase44K1AudioBefore = "boundary-44k1-audio-before-v1"
	BoundaryCase44K1AudioAfter  = "boundary-44k1-audio-after-v1"

	BoundaryOffsetKeyframe    = "keyframe"
	BoundaryOffsetVideoBefore = "video_frame_before"
	BoundaryOffsetVideoAfter  = "video_frame_after"
	BoundaryOffsetAudioBefore = "audio_packet_before"
	BoundaryOffsetAudioAfter  = "audio_packet_after"

	boundaryReferenceMicros int64 = 30_000_000
)

var boundaryCaseSpecs = []BoundaryCaseSpec{
	newBoundaryCase(BoundaryCase48KKeyframe, "48 kHz production policy at the exact two-second keyframe", FixtureCFR48KZeroLatency, BoundaryOffsetKeyframe, 0),
	newBoundaryCase(BoundaryCase48KVideoBefore, "48 kHz production policy one 30 fps frame before the keyframe", FixtureCFR48KZeroLatency, BoundaryOffsetVideoBefore, -unitMicros(1, fixtureFrameRate)),
	newBoundaryCase(BoundaryCase48KVideoAfter, "48 kHz production policy one 30 fps frame after the keyframe", FixtureCFR48KZeroLatency, BoundaryOffsetVideoAfter, unitMicros(1, fixtureFrameRate)),
	newBoundaryCase(BoundaryCase48KAudioBefore, "48 kHz production policy one AAC packet before the keyframe", FixtureCFR48KZeroLatency, BoundaryOffsetAudioBefore, -unitMicros(1024, 48_000)),
	newBoundaryCase(BoundaryCase48KAudioAfter, "48 kHz production policy one AAC packet after the keyframe", FixtureCFR48KZeroLatency, BoundaryOffsetAudioAfter, unitMicros(1024, 48_000)),
	newBoundaryCase(BoundaryCase44K1Keyframe, "44.1 kHz production policy at the exact two-second keyframe", FixtureCFR44K1ZeroLatency, BoundaryOffsetKeyframe, 0),
	newBoundaryCase(BoundaryCase44K1AudioBefore, "44.1 kHz production policy one AAC packet before the keyframe", FixtureCFR44K1ZeroLatency, BoundaryOffsetAudioBefore, -unitMicros(1024, 44_100)),
	newBoundaryCase(BoundaryCase44K1AudioAfter, "44.1 kHz production policy one AAC packet after the keyframe", FixtureCFR44K1ZeroLatency, BoundaryOffsetAudioAfter, unitMicros(1024, 44_100)),
}

type BoundaryCaseSpec struct {
	ID                      string `json:"id"`
	Description             string `json:"description"`
	FixtureID               string `json:"fixture_id"`
	OffsetKind              string `json:"offset_kind"`
	ReferenceBoundaryMicros int64  `json:"reference_boundary_micros"`
	OffsetMicros            int64  `json:"offset_micros"`
	ExpectedBoundaryMicros  int64  `json:"expected_boundary_micros"`
}

type BoundaryCaseReport struct {
	Case            BoundaryCaseSpec           `json:"case"`
	ContractVersion string                     `json:"contract_version"`
	ContractHash    string                     `json:"contract_hash"`
	Evidence        transcodeboundary.Contract `json:"evidence"`
}

type BoundaryMatrixReport struct {
	SchemaVersion string               `json:"schema_version"`
	Cases         []BoundaryCaseReport `json:"cases"`
}

func newBoundaryCase(id, description, fixtureID, offsetKind string, offsetMicros int64) BoundaryCaseSpec {
	return BoundaryCaseSpec{
		ID:                      id,
		Description:             description,
		FixtureID:               fixtureID,
		OffsetKind:              offsetKind,
		ReferenceBoundaryMicros: boundaryReferenceMicros,
		OffsetMicros:            offsetMicros,
		ExpectedBoundaryMicros:  boundaryReferenceMicros + offsetMicros,
	}
}

func unitMicros(numerator, denominator int64) int64 {
	return int64(math.Round(float64(numerator) * 1_000_000 / float64(denominator)))
}

func (s BoundaryCaseSpec) Validate() error {
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.Description) == "" || strings.TrimSpace(s.FixtureID) == "" {
		return fmt.Errorf("boundary case identity is incomplete")
	}
	fixture, ok := LookupFixture(s.FixtureID)
	if !ok || fixture.Control || fixture.VideoTune != VideoTuneZeroLatency {
		return fmt.Errorf("boundary case must use a production software fixture")
	}
	if s.ReferenceBoundaryMicros != boundaryReferenceMicros || s.ExpectedBoundaryMicros != s.ReferenceBoundaryMicros+s.OffsetMicros || s.ExpectedBoundaryMicros <= 0 {
		return fmt.Errorf("boundary case timing is invalid")
	}
	switch s.OffsetKind {
	case BoundaryOffsetKeyframe, BoundaryOffsetVideoBefore, BoundaryOffsetVideoAfter, BoundaryOffsetAudioBefore, BoundaryOffsetAudioAfter:
	default:
		return fmt.Errorf("unsupported boundary offset kind %q", s.OffsetKind)
	}
	return nil
}

func AvailableBoundaryCases() []BoundaryCaseSpec {
	result := make([]BoundaryCaseSpec, len(boundaryCaseSpecs))
	copy(result, boundaryCaseSpecs)
	return result
}

func LookupBoundaryCase(id string) (BoundaryCaseSpec, bool) {
	candidate := strings.TrimSpace(id)
	for _, spec := range boundaryCaseSpecs {
		if spec.ID == candidate {
			return spec, true
		}
	}
	return BoundaryCaseSpec{}, false
}
