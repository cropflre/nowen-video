package certification

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	transcodeavsync "github.com/nowen-video/nowen-video/internal/transcode/avsync"
)

const (
	AVSyncVarianceMatrixSchemaVersion = "ffmpeg-av-boundary-sync-variance-matrix-v1"
	AVSyncVarianceRepeatCount         = 3
	AVSyncVarianceToleranceMicros     = int64(1)
)

const (
	AVSyncComparison48K  = "48k_per_stream_vs_baseline"
	AVSyncComparison44K1 = "44k1_common_aac_two_vs_baseline"
)

var avSyncVarianceCaseIDs = []string{
	ShapingCase48KBaseline,
	ShapingCase48KPerStream,
	ShapingCase44K1Baseline,
	ShapingCase44K1CommonAACTwo,
}

type AVSyncRunReport struct {
	Ordinal       int                    `json:"ordinal"`
	Shaping       ShapingCaseReport      `json:"shaping"`
	AVSyncVersion string                 `json:"av_sync_version"`
	AVSyncHash    string                 `json:"av_sync_hash"`
	AVSync        transcodeavsync.Contract `json:"av_sync"`
}

type MetricRange struct {
	MinMicros  int64 `json:"min_micros"`
	MaxMicros  int64 `json:"max_micros"`
	SpanMicros int64 `json:"span_micros"`
}

type AVSyncVarianceSummary struct {
	RepeatCount                 int         `json:"repeat_count"`
	VideoBoundaryDeltaMicros    MetricRange `json:"video_boundary_delta_micros"`
	AudioBoundaryDeltaMicros    MetricRange `json:"audio_boundary_delta_micros"`
	StartupEndSkewMicros        MetricRange `json:"startup_end_skew_micros"`
	ContinuationStartSkewMicros MetricRange `json:"continuation_start_skew_micros"`
	BoundaryDeltaSkewMicros     MetricRange `json:"boundary_delta_skew_micros"`
	MaxObservedSpanMicros       int64       `json:"max_observed_span_micros"`
	Stable                      bool        `json:"stable"`
}

type AVSyncCaseVarianceReport struct {
	Case    ShapingCaseSpec       `json:"case"`
	Runs    []AVSyncRunReport     `json:"runs"`
	Summary AVSyncVarianceSummary `json:"summary"`
}

type AVSyncVarianceComparison struct {
	Name                               string `json:"name"`
	BaselineCaseID                     string `json:"baseline_case_id"`
	CandidateCaseID                    string `json:"candidate_case_id"`
	BaselineMaxAbsDeltaSkewMicros      int64  `json:"baseline_max_abs_delta_skew_micros"`
	CandidateMaxAbsDeltaSkewMicros     int64  `json:"candidate_max_abs_delta_skew_micros"`
	DeltaSkewImprovementMicros         int64  `json:"delta_skew_improvement_micros"`
}

type AVSyncVarianceMatrixReport struct {
	SchemaVersion           string                     `json:"schema_version"`
	RepeatCount             int                        `json:"repeat_count"`
	VarianceToleranceMicros int64                      `json:"variance_tolerance_micros"`
	Cases                   []AVSyncCaseVarianceReport `json:"cases"`
	Comparisons             []AVSyncVarianceComparison `json:"comparisons"`
}

func AvailableAVSyncVarianceCases() []ShapingCaseSpec {
	result := make([]ShapingCaseSpec, 0, len(avSyncVarianceCaseIDs))
	for _, caseID := range avSyncVarianceCaseIDs {
		spec, ok := LookupShapingCase(caseID)
		if !ok {
			panic(fmt.Sprintf("A/V sync variance case %q is not registered", caseID))
		}
		result = append(result, spec)
	}
	return result
}

func RunAVSyncVarianceMatrix(ctx context.Context, config Config) (AVSyncVarianceMatrixReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cases := make([]AVSyncCaseVarianceReport, 0, len(avSyncVarianceCaseIDs))
	for _, caseID := range avSyncVarianceCaseIDs {
		spec, ok := LookupShapingCase(caseID)
		if !ok {
			return AVSyncVarianceMatrixReport{}, fmt.Errorf("unknown A/V sync variance case %q", caseID)
		}
		runs := make([]AVSyncRunReport, 0, AVSyncVarianceRepeatCount)
		for ordinal := 1; ordinal <= AVSyncVarianceRepeatCount; ordinal++ {
			runConfig := config
			if config.WorkDir != "" {
				runConfig.WorkDir = filepath.Join(config.WorkDir, "av-sync-variance", caseID, fmt.Sprintf("run-%02d", ordinal))
			}
			shaping, err := RunShapingCase(ctx, runConfig, caseID)
			if err != nil {
				return AVSyncVarianceMatrixReport{}, fmt.Errorf("run A/V sync case %s repeat %d: %w", caseID, ordinal, err)
			}
			avSync, err := transcodeavsync.FromBoundary(shaping.Evidence)
			if err != nil {
				return AVSyncVarianceMatrixReport{}, fmt.Errorf("build A/V sync evidence for %s repeat %d: %w", caseID, ordinal, err)
			}
			version, hash, _, err := transcodeavsync.Identity(avSync)
			if err != nil {
				return AVSyncVarianceMatrixReport{}, err
			}
			runs = append(runs, AVSyncRunReport{
				Ordinal:       ordinal,
				Shaping:       shaping,
				AVSyncVersion: version,
				AVSyncHash:    hash,
				AVSync:        avSync,
			})
		}
		caseReport := AVSyncCaseVarianceReport{
			Case:    spec,
			Runs:    runs,
			Summary: buildAVSyncVarianceSummary(runs),
		}
		if err := caseReport.Validate(); err != nil {
			return AVSyncVarianceMatrixReport{}, err
		}
		cases = append(cases, caseReport)
	}
	matrix := AVSyncVarianceMatrixReport{
		SchemaVersion:           AVSyncVarianceMatrixSchemaVersion,
		RepeatCount:             AVSyncVarianceRepeatCount,
		VarianceToleranceMicros: AVSyncVarianceToleranceMicros,
		Cases:                   cases,
	}
	comparisons, err := buildAVSyncVarianceComparisons(cases)
	if err != nil {
		return AVSyncVarianceMatrixReport{}, err
	}
	matrix.Comparisons = comparisons
	if err := matrix.Validate(); err != nil {
		return AVSyncVarianceMatrixReport{}, err
	}
	return matrix, nil
}

func (r AVSyncRunReport) Validate(caseSpec ShapingCaseSpec, ordinal int) error {
	if r.Ordinal != ordinal {
		return fmt.Errorf("A/V sync repeat ordinal is invalid")
	}
	if err := r.Shaping.Validate(); err != nil {
		return err
	}
	if r.Shaping.Case != caseSpec {
		return fmt.Errorf("A/V sync shaping case does not match matrix case")
	}
	if err := r.AVSync.ValidateAgainst(r.Shaping.Evidence); err != nil {
		return err
	}
	version, hash, _, err := transcodeavsync.Identity(r.AVSync)
	if err != nil {
		return err
	}
	if r.AVSyncVersion != version || r.AVSyncHash != hash {
		return fmt.Errorf("A/V sync evidence identity is invalid")
	}
	if r.AVSync.SeamlessAllowed || !r.AVSync.DiscontinuityRequired {
		return fmt.Errorf("A/V sync evidence attempted to authorize seamless playback")
	}
	return nil
}

func (r AVSyncCaseVarianceReport) Validate() error {
	expected, ok := LookupShapingCase(r.Case.ID)
	if !ok || r.Case != expected || !containsString(avSyncVarianceCaseIDs, r.Case.ID) {
		return fmt.Errorf("A/V sync variance case %q does not match registry", r.Case.ID)
	}
	if len(r.Runs) != AVSyncVarianceRepeatCount {
		return fmt.Errorf("A/V sync variance case %s has %d repeats, want %d", r.Case.ID, len(r.Runs), AVSyncVarianceRepeatCount)
	}
	planHash := ""
	for index, run := range r.Runs {
		if err := run.Validate(r.Case, index+1); err != nil {
			return fmt.Errorf("validate A/V sync case %s repeat %d: %w", r.Case.ID, index+1, err)
		}
		if planHash == "" {
			planHash = run.Shaping.PlanHash
		} else if run.Shaping.PlanHash != planHash {
			return fmt.Errorf("A/V sync repeats changed timestamp execution plan identity")
		}
	}
	expectedSummary := buildAVSyncVarianceSummary(r.Runs)
	if r.Summary != expectedSummary {
		return fmt.Errorf("A/V sync variance summary is inconsistent")
	}
	if !r.Summary.Stable {
		return fmt.Errorf("A/V sync case %s exceeded repeated-run variance tolerance", r.Case.ID)
	}
	return nil
}

func (m AVSyncVarianceMatrixReport) Validate() error {
	if m.SchemaVersion != AVSyncVarianceMatrixSchemaVersion {
		return fmt.Errorf("unsupported A/V sync variance matrix schema %q", m.SchemaVersion)
	}
	if m.RepeatCount != AVSyncVarianceRepeatCount || m.VarianceToleranceMicros != AVSyncVarianceToleranceMicros {
		return fmt.Errorf("A/V sync variance matrix policy is invalid")
	}
	if len(m.Cases) != len(avSyncVarianceCaseIDs) {
		return fmt.Errorf("A/V sync variance matrix is incomplete")
	}
	ffmpegVersion := ""
	ffprobeVersion := ""
	for index, report := range m.Cases {
		if report.Case.ID != avSyncVarianceCaseIDs[index] {
			return fmt.Errorf("A/V sync variance case order is invalid")
		}
		if err := report.Validate(); err != nil {
			return err
		}
		for _, run := range report.Runs {
			if ffmpegVersion == "" {
				ffmpegVersion = run.Shaping.Evidence.FFmpegVersion
				ffprobeVersion = run.Shaping.Evidence.FFprobeVersion
			} else if run.Shaping.Evidence.FFmpegVersion != ffmpegVersion || run.Shaping.Evidence.FFprobeVersion != ffprobeVersion {
				return fmt.Errorf("A/V sync variance matrix toolchain identity is inconsistent")
			}
		}
	}
	expectedComparisons, err := buildAVSyncVarianceComparisons(m.Cases)
	if err != nil {
		return err
	}
	if len(m.Comparisons) != len(expectedComparisons) {
		return fmt.Errorf("A/V sync variance comparisons are incomplete")
	}
	for index := range expectedComparisons {
		if m.Comparisons[index] != expectedComparisons[index] {
			return fmt.Errorf("A/V sync variance comparison is inconsistent")
		}
		if m.Comparisons[index].DeltaSkewImprovementMicros <= 0 {
			return fmt.Errorf("A/V sync candidate %s did not improve relative boundary skew", m.Comparisons[index].CandidateCaseID)
		}
	}
	return nil
}

func MarshalAVSyncVarianceMatrixReport(report AVSyncVarianceMatrixReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal A/V sync variance matrix: %w", err)
	}
	return append(content, '\n'), nil
}

func buildAVSyncVarianceSummary(runs []AVSyncRunReport) AVSyncVarianceSummary {
	videoDelta := make([]int64, 0, len(runs))
	audioDelta := make([]int64, 0, len(runs))
	startupSkew := make([]int64, 0, len(runs))
	continuationSkew := make([]int64, 0, len(runs))
	boundarySkew := make([]int64, 0, len(runs))
	for _, run := range runs {
		videoDelta = append(videoDelta, run.AVSync.VideoBoundaryDeltaMicros)
		audioDelta = append(audioDelta, run.AVSync.AudioBoundaryDeltaMicros)
		startupSkew = append(startupSkew, run.AVSync.StartupEndSkewMicros)
		continuationSkew = append(continuationSkew, run.AVSync.ContinuationStartSkewMicros)
		boundarySkew = append(boundarySkew, run.AVSync.BoundaryDeltaSkewMicros)
	}
	summary := AVSyncVarianceSummary{
		RepeatCount:                 len(runs),
		VideoBoundaryDeltaMicros:    metricRange(videoDelta),
		AudioBoundaryDeltaMicros:    metricRange(audioDelta),
		StartupEndSkewMicros:        metricRange(startupSkew),
		ContinuationStartSkewMicros: metricRange(continuationSkew),
		BoundaryDeltaSkewMicros:     metricRange(boundarySkew),
	}
	summary.MaxObservedSpanMicros = max64Certification(
		summary.VideoBoundaryDeltaMicros.SpanMicros,
		summary.AudioBoundaryDeltaMicros.SpanMicros,
		summary.StartupEndSkewMicros.SpanMicros,
		summary.ContinuationStartSkewMicros.SpanMicros,
		summary.BoundaryDeltaSkewMicros.SpanMicros,
	)
	summary.Stable = len(runs) == AVSyncVarianceRepeatCount && summary.MaxObservedSpanMicros <= AVSyncVarianceToleranceMicros
	return summary
}

func buildAVSyncVarianceComparisons(cases []AVSyncCaseVarianceReport) ([]AVSyncVarianceComparison, error) {
	byID := make(map[string]AVSyncCaseVarianceReport, len(cases))
	for _, report := range cases {
		byID[report.Case.ID] = report
	}
	build := func(name, baselineID, candidateID string) (AVSyncVarianceComparison, error) {
		baseline, ok := byID[baselineID]
		if !ok {
			return AVSyncVarianceComparison{}, fmt.Errorf("missing A/V sync baseline %s", baselineID)
		}
		candidate, ok := byID[candidateID]
		if !ok {
			return AVSyncVarianceComparison{}, fmt.Errorf("missing A/V sync candidate %s", candidateID)
		}
		baselineAbs := maxAbsRange(baseline.Summary.BoundaryDeltaSkewMicros)
		candidateAbs := maxAbsRange(candidate.Summary.BoundaryDeltaSkewMicros)
		return AVSyncVarianceComparison{
			Name:                           name,
			BaselineCaseID:                 baselineID,
			CandidateCaseID:                candidateID,
			BaselineMaxAbsDeltaSkewMicros:  baselineAbs,
			CandidateMaxAbsDeltaSkewMicros: candidateAbs,
			DeltaSkewImprovementMicros:     baselineAbs - candidateAbs,
		}, nil
	}
	comparison48K, err := build(AVSyncComparison48K, ShapingCase48KBaseline, ShapingCase48KPerStream)
	if err != nil {
		return nil, err
	}
	comparison44K1, err := build(AVSyncComparison44K1, ShapingCase44K1Baseline, ShapingCase44K1CommonAACTwo)
	if err != nil {
		return nil, err
	}
	return []AVSyncVarianceComparison{comparison48K, comparison44K1}, nil
}

func metricRange(values []int64) MetricRange {
	if len(values) == 0 {
		return MetricRange{}
	}
	minimum := values[0]
	maximum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	return MetricRange{MinMicros: minimum, MaxMicros: maximum, SpanMicros: maximum - minimum}
}

func maxAbsRange(value MetricRange) int64 {
	left := abs64Certification(value.MinMicros)
	right := abs64Certification(value.MaxMicros)
	if left > right {
		return left
	}
	return right
}

func max64Certification(values ...int64) int64 {
	var maximum int64
	for _, value := range values {
		if value > maximum {
			maximum = value
		}
	}
	return maximum
}

func abs64Certification(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
