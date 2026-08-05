package certification

import (
	"fmt"
	"strings"

	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	timestampexecution "github.com/nowen-video/nowen-video/internal/transcode/timestampexecution"
)

const ShapingMatrixSchemaVersion = "ffmpeg-boundary-shaping-matrix-v1"

const (
	ShapingCase48KBaseline       = "shape-48k-baseline-v1"
	ShapingCase48KCommonVideo    = "shape-48k-common-video-frame-v1"
	ShapingCase48KCommonAACTwo   = "shape-48k-common-aac-two-v1"
	ShapingCase48KCommonAACThree = "shape-48k-common-aac-three-v1"
	ShapingCase48KPerStream      = "shape-48k-per-stream-v1"
	ShapingCase44K1Baseline      = "shape-44k1-baseline-v1"
	ShapingCase44K1CommonAACTwo  = "shape-44k1-common-aac-two-v1"
	ShapingCase44K1PerStream     = "shape-44k1-per-stream-v1"
)

var shapingCaseSpecs = []ShapingCaseSpec{
	newShapingCase(ShapingCase48KBaseline, "48 kHz production baseline without PTS shaping", FixtureCFR48KZeroLatency, 0, 0),
	newShapingCase(ShapingCase48KCommonVideo, "48 kHz common shift by one 30 fps frame", FixtureCFR48KZeroLatency, 33_333, 33_333),
	newShapingCase(ShapingCase48KCommonAACTwo, "48 kHz common shift by two AAC access units", FixtureCFR48KZeroLatency, 42_667, 42_667),
	newShapingCase(ShapingCase48KCommonAACThree, "48 kHz common shift by three AAC access units", FixtureCFR48KZeroLatency, 64_000, 64_000),
	newShapingCase(ShapingCase48KPerStream, "48 kHz one video frame plus three AAC access units", FixtureCFR48KZeroLatency, 33_333, 64_000),
	newShapingCase(ShapingCase44K1Baseline, "44.1 kHz production baseline without PTS shaping", FixtureCFR44K1ZeroLatency, 0, 0),
	newShapingCase(ShapingCase44K1CommonAACTwo, "44.1 kHz common shift by two AAC access units", FixtureCFR44K1ZeroLatency, 46_440, 46_440),
	newShapingCase(ShapingCase44K1PerStream, "44.1 kHz one video frame plus two AAC access units", FixtureCFR44K1ZeroLatency, 33_333, 46_440),
}

type ShapingCaseSpec struct {
	ID                     string `json:"id"`
	Description            string `json:"description"`
	FixtureID              string `json:"fixture_id"`
	ExpectedBoundaryMicros int64  `json:"expected_boundary_micros"`
	VideoPTSShiftMicros    int64  `json:"video_pts_shift_micros"`
	AudioPTSShiftMicros    int64  `json:"audio_pts_shift_micros"`
}

type ShapingCaseReport struct {
	Case            ShapingCaseSpec            `json:"case"`
	PlanVersion     string                     `json:"plan_version"`
	PlanHash        string                     `json:"plan_hash"`
	PlanJSON        string                     `json:"plan_json"`
	EvidenceVersion string                     `json:"evidence_version"`
	EvidenceHash    string                     `json:"evidence_hash"`
	Evidence        transcodeboundary.Contract `json:"evidence"`
}

type ShapingMatrixReport struct {
	SchemaVersion string              `json:"schema_version"`
	Cases         []ShapingCaseReport `json:"cases"`
}

func newShapingCase(id, description, fixtureID string, videoShiftMicros, audioShiftMicros int64) ShapingCaseSpec {
	return ShapingCaseSpec{
		ID:                     id,
		Description:            description,
		FixtureID:              fixtureID,
		ExpectedBoundaryMicros: boundaryReferenceMicros,
		VideoPTSShiftMicros:    videoShiftMicros,
		AudioPTSShiftMicros:    audioShiftMicros,
	}
}

func (s ShapingCaseSpec) Validate() error {
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.Description) == "" || strings.TrimSpace(s.FixtureID) == "" {
		return fmt.Errorf("shaping case identity is incomplete")
	}
	fixture, ok := LookupFixture(s.FixtureID)
	if !ok || fixture.Control || fixture.VideoTune != VideoTuneZeroLatency {
		return fmt.Errorf("shaping case must use a production software fixture")
	}
	if s.ExpectedBoundaryMicros != boundaryReferenceMicros {
		return fmt.Errorf("shaping case must use the canonical 30 second boundary")
	}
	if _, err := timestampexecution.New(s.VideoPTSShiftMicros, s.AudioPTSShiftMicros); err != nil {
		return fmt.Errorf("shaping case execution plan: %w", err)
	}
	return nil
}

func AvailableShapingCases() []ShapingCaseSpec {
	result := make([]ShapingCaseSpec, len(shapingCaseSpecs))
	copy(result, shapingCaseSpecs)
	return result
}

func LookupShapingCase(id string) (ShapingCaseSpec, bool) {
	candidate := strings.TrimSpace(id)
	for _, spec := range shapingCaseSpecs {
		if spec.ID == candidate {
			return spec, true
		}
	}
	return ShapingCaseSpec{}, false
}
