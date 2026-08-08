package certification

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
)

const MatrixSchemaVersion = "ffmpeg-handoff-fixture-matrix-v1"

const (
	ComparisonZeroLatency48K = "x264_zerolatency_effect_48k"
	ComparisonSampleRate     = "aac_sample_rate_effect_zerolatency"
)

type MatrixReport struct {
	SchemaVersion string             `json:"schema_version"`
	Reports       []Report           `json:"reports"`
	Comparisons   []MatrixComparison `json:"comparisons"`
}

type MatrixComparison struct {
	Name                               string `json:"name"`
	BaselineFixtureID                  string `json:"baseline_fixture_id"`
	CandidateFixtureID                 string `json:"candidate_fixture_id"`
	VideoPresentationDeltaChangeMicros int64  `json:"video_presentation_delta_change_micros"`
	VideoDecodeDeltaChangeMicros       int64  `json:"video_decode_delta_change_micros"`
	AudioPresentationDeltaChangeMicros int64  `json:"audio_presentation_delta_change_micros"`
	AudioDecodeDeltaChangeMicros       int64  `json:"audio_decode_delta_change_micros"`
}

func RequiredMatrixFixtureIDs() []string {
	return []string{
		FixtureCFR48K,
		FixtureCFR48KZeroLatency,
		FixtureCFR44K1ZeroLatency,
	}
}

func RunMatrix(ctx context.Context, config Config) (MatrixReport, error) {
	reports := make([]Report, 0, len(RequiredMatrixFixtureIDs()))
	for _, fixtureID := range RequiredMatrixFixtureIDs() {
		fixtureConfig := config
		fixtureConfig.FixtureID = fixtureID
		if config.WorkDir != "" {
			fixtureConfig.WorkDir = filepath.Join(config.WorkDir, fixtureID)
		}
		report, err := Run(ctx, fixtureConfig)
		if err != nil {
			return MatrixReport{}, fmt.Errorf("run fixture %s: %w", fixtureID, err)
		}
		reports = append(reports, report)
	}
	return BuildMatrixReport(reports)
}

func BuildMatrixReport(reports []Report) (MatrixReport, error) {
	byID := make(map[string]Report, len(reports))
	ordered := make([]Report, 0, len(RequiredMatrixFixtureIDs()))
	for _, report := range reports {
		if err := ValidateCertifiedReport(report); err != nil {
			return MatrixReport{}, fmt.Errorf("validate fixture %s: %w", report.FixtureID, err)
		}
		if _, exists := byID[report.FixtureID]; exists {
			return MatrixReport{}, fmt.Errorf("duplicate fixture report %s", report.FixtureID)
		}
		byID[report.FixtureID] = report
	}
	for _, fixtureID := range RequiredMatrixFixtureIDs() {
		report, ok := byID[fixtureID]
		if !ok {
			return MatrixReport{}, fmt.Errorf("matrix is missing fixture %s", fixtureID)
		}
		ordered = append(ordered, report)
	}
	matrix := MatrixReport{
		SchemaVersion: MatrixSchemaVersion,
		Reports:       ordered,
		Comparisons: []MatrixComparison{
			compareReports(ComparisonZeroLatency48K, byID[FixtureCFR48K], byID[FixtureCFR48KZeroLatency]),
			compareReports(ComparisonSampleRate, byID[FixtureCFR48KZeroLatency], byID[FixtureCFR44K1ZeroLatency]),
		},
	}
	if err := matrix.Validate(); err != nil {
		return MatrixReport{}, err
	}
	return matrix, nil
}

func (m MatrixReport) Validate() error {
	if m.SchemaVersion != MatrixSchemaVersion {
		return fmt.Errorf("unsupported fixture matrix schema %q", m.SchemaVersion)
	}
	if len(m.Reports) != len(RequiredMatrixFixtureIDs()) || len(m.Comparisons) != 2 {
		return fmt.Errorf("fixture matrix is incomplete")
	}
	byID := make(map[string]Report, len(m.Reports))
	for _, report := range m.Reports {
		if err := ValidateCertifiedReport(report); err != nil {
			return err
		}
		if _, exists := byID[report.FixtureID]; exists {
			return fmt.Errorf("duplicate fixture report %s", report.FixtureID)
		}
		byID[report.FixtureID] = report
	}
	for _, fixtureID := range RequiredMatrixFixtureIDs() {
		if _, ok := byID[fixtureID]; !ok {
			return fmt.Errorf("fixture matrix is missing %s", fixtureID)
		}
	}
	expected := map[string]MatrixComparison{
		ComparisonZeroLatency48K: compareReports(ComparisonZeroLatency48K, byID[FixtureCFR48K], byID[FixtureCFR48KZeroLatency]),
		ComparisonSampleRate:     compareReports(ComparisonSampleRate, byID[FixtureCFR48KZeroLatency], byID[FixtureCFR44K1ZeroLatency]),
	}
	for _, comparison := range m.Comparisons {
		want, ok := expected[comparison.Name]
		if !ok || comparison != want {
			return fmt.Errorf("fixture matrix comparison %q is invalid", comparison.Name)
		}
		delete(expected, comparison.Name)
	}
	if len(expected) != 0 {
		return fmt.Errorf("fixture matrix comparisons are incomplete")
	}
	return nil
}

func compareReports(name string, baseline, candidate Report) MatrixComparison {
	return MatrixComparison{
		Name:                               name,
		BaselineFixtureID:                  baseline.FixtureID,
		CandidateFixtureID:                 candidate.FixtureID,
		VideoPresentationDeltaChangeMicros: candidate.Handoff.VideoPresentationDeltaMicros - baseline.Handoff.VideoPresentationDeltaMicros,
		VideoDecodeDeltaChangeMicros:       candidate.Handoff.VideoDecodeDeltaMicros - baseline.Handoff.VideoDecodeDeltaMicros,
		AudioPresentationDeltaChangeMicros: candidate.Handoff.AudioPresentationDeltaMicros - baseline.Handoff.AudioPresentationDeltaMicros,
		AudioDecodeDeltaChangeMicros:       candidate.Handoff.AudioDecodeDeltaMicros - baseline.Handoff.AudioDecodeDeltaMicros,
	}
}

func MarshalMatrixReport(report MatrixReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal fixture matrix: %w", err)
	}
	return append(content, '\n'), nil
}
