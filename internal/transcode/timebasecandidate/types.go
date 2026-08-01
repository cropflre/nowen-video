package timebasecandidate

import (
	"fmt"
	"math"
	"strings"

	transcodeavsync "github.com/nowen-video/nowen-video/internal/transcode/avsync"
	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodevfrisolation "github.com/nowen-video/nowen-video/internal/transcode/vfrisolation"
)

const SchemaVersion = "encoder-time-base-candidate-evidence-v1"

const (
	RepeatCount                   = 3
	VarianceToleranceMicros       = int64(1)
	CrossCandidateToleranceMicros = int64(1_000)
	BoundaryFrameTolerance        = 1
	CandidateAVTB                 = "encoder-time-base-avtb-v1"
	Candidate90K                  = "encoder-time-base-90k-v1"
)

type CaseSpec struct {
	ID                            string `json:"id"`
	Description                   string `json:"description"`
	SourceMode                    string `json:"source_mode"`
	PrimaryFrameRateNumerator     int64  `json:"primary_frame_rate_numerator"`
	PrimaryFrameRateDenominator   int64  `json:"primary_frame_rate_denominator"`
	SecondaryFrameRateNumerator   int64  `json:"secondary_frame_rate_numerator,omitempty"`
	SecondaryFrameRateDenominator int64  `json:"secondary_frame_rate_denominator,omitempty"`
	SourceOffsetMicros            int64  `json:"source_offset_micros"`
	AudioSampleRate               int    `json:"audio_sample_rate"`
	GOPSize                       int    `json:"gop_size"`
	ExpectedBoundaryMicros        int64  `json:"expected_boundary_micros"`
	DurationMicros                int64  `json:"duration_micros"`
}

type CandidateSpec struct {
	ID              string `json:"id"`
	Description     string `json:"description"`
	EncoderTimeBase string `json:"encoder_time_base"`
}

type MetricRange struct {
	Min  int64 `json:"min"`
	Max  int64 `json:"max"`
	Span int64 `json:"span"`
}

type RunEvidence struct {
	Ordinal                 int                                     `json:"ordinal"`
	StartupCommandHash      string                                  `json:"startup_command_hash"`
	ContinuationCommandHash string                                  `json:"continuation_command_hash"`
	StartupTimeline         transcodeoutputcadence.TimelineEvidence `json:"startup_timeline"`
	ContinuationTimeline    transcodeoutputcadence.TimelineEvidence `json:"continuation_timeline"`
	StartupMapping          transcodeoutputcadence.FrameMapping     `json:"startup_mapping"`
	ContinuationMapping     transcodeoutputcadence.FrameMapping     `json:"continuation_mapping"`
	StartupFingerprint      transcodevfrisolation.FrameFingerprint  `json:"startup_fingerprint"`
	ContinuationFingerprint transcodevfrisolation.FrameFingerprint  `json:"continuation_fingerprint"`
	BoundaryVersion         string                                  `json:"boundary_version"`
	BoundaryHash            string                                  `json:"boundary_hash"`
	Boundary                transcodeboundary.Contract              `json:"boundary"`
	AVSyncVersion           string                                  `json:"av_sync_version"`
	AVSyncHash              string                                  `json:"av_sync_hash"`
	AVSync                  transcodeavsync.Contract                `json:"av_sync"`
}

type CandidateSummary struct {
	RepeatCount                         int         `json:"repeat_count"`
	StartupFrameCount                   MetricRange `json:"startup_frame_count"`
	ContinuationFrameCount              MetricRange `json:"continuation_frame_count"`
	StartupDominantDeltaMicros          MetricRange `json:"startup_dominant_delta_micros"`
	ContinuationDominantDeltaMicros     MetricRange `json:"continuation_dominant_delta_micros"`
	StartupNearZeroDeltaCount           MetricRange `json:"startup_near_zero_delta_count"`
	ContinuationNearZeroDeltaCount      MetricRange `json:"continuation_near_zero_delta_count"`
	StartupDuplicatePTSCount            MetricRange `json:"startup_duplicate_pts_count"`
	ContinuationDuplicatePTSCount       MetricRange `json:"continuation_duplicate_pts_count"`
	StartupAdjacentDuplicateFrameCount  MetricRange `json:"startup_adjacent_duplicate_frame_count"`
	ContinuationAdjacentDuplicateFrames MetricRange `json:"continuation_adjacent_duplicate_frame_count"`
	VideoBoundaryDeltaMicros            MetricRange `json:"video_boundary_delta_micros"`
	AudioBoundaryDeltaMicros            MetricRange `json:"audio_boundary_delta_micros"`
	StartupEndSkewMicros                MetricRange `json:"startup_end_skew_micros"`
	ContinuationStartSkewMicros         MetricRange `json:"continuation_start_skew_micros"`
	BoundaryDeltaSkewMicros             MetricRange `json:"boundary_delta_skew_micros"`
	SkewTransitionMicros                MetricRange `json:"skew_transition_micros"`
	ProjectionResidualMicros            MetricRange `json:"projection_residual_micros"`
	MaximumAbsoluteFrameCountDelta      int         `json:"maximum_absolute_frame_count_delta"`
	BoundaryFrameToleranceUsed          bool        `json:"boundary_frame_tolerance_used"`
	SequenceStable                      bool        `json:"sequence_stable"`
	CadenceStable                       bool        `json:"cadence_stable"`
	AVSyncStable                        bool        `json:"av_sync_stable"`
	AllPreserved                        bool        `json:"all_preserved"`
	Stable                              bool        `json:"stable"`
}

type CandidateEvidence struct {
	Spec    CandidateSpec    `json:"spec"`
	Runs    []RunEvidence    `json:"runs"`
	Summary CandidateSummary `json:"summary"`
}

type CandidateComparison struct {
	CandidateAID                    string `json:"candidate_a_id"`
	CandidateBID                    string `json:"candidate_b_id"`
	StartupSequenceEquivalent       bool   `json:"startup_sequence_equivalent"`
	ContinuationSequenceEquivalent  bool   `json:"continuation_sequence_equivalent"`
	FrameMappingEquivalent          bool   `json:"frame_mapping_equivalent"`
	CadenceEquivalent               bool   `json:"cadence_equivalent"`
	MaxAVSyncMetricDifferenceMicros int64  `json:"max_av_sync_metric_difference_micros"`
	AVSyncWithinTolerance           bool   `json:"av_sync_within_tolerance"`
	Equivalent                      bool   `json:"equivalent"`
}

type CaseEvidence struct {
	Case                       CaseSpec                                `json:"case"`
	SourceStartupTimeline      transcodeoutputcadence.TimelineEvidence `json:"source_startup_timeline"`
	SourceContinuationTimeline transcodeoutputcadence.TimelineEvidence `json:"source_continuation_timeline"`
	Candidates                 []CandidateEvidence                     `json:"candidates"`
	Comparison                 CandidateComparison                     `json:"comparison"`
}

type Contract struct {
	SchemaVersion                 string         `json:"schema_version"`
	FFmpegVersion                 string         `json:"ffmpeg_version"`
	FFprobeVersion                string         `json:"ffprobe_version"`
	RepeatCount                   int            `json:"repeat_count"`
	VarianceToleranceMicros       int64          `json:"variance_tolerance_micros"`
	CrossCandidateToleranceMicros int64          `json:"cross_candidate_tolerance_micros"`
	Cases                         []CaseEvidence `json:"cases"`
	SeamlessAllowed               bool           `json:"seamless_allowed"`
	DiscontinuityRequired         bool           `json:"discontinuity_required"`
}

func (s CaseSpec) Validate() error {
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.Description) == "" {
		return fmt.Errorf("encoder time-base case identity is incomplete")
	}
	if s.SourceMode != transcodesourceorigin.ModeCFR && s.SourceMode != transcodesourceorigin.ModeVFR {
		return fmt.Errorf("unsupported source mode %q", s.SourceMode)
	}
	if s.PrimaryFrameRateNumerator <= 0 || s.PrimaryFrameRateDenominator <= 0 || s.GOPSize <= 0 {
		return fmt.Errorf("primary frame-rate policy is invalid")
	}
	if s.SourceMode == transcodesourceorigin.ModeCFR {
		if s.SecondaryFrameRateNumerator != 0 || s.SecondaryFrameRateDenominator != 0 {
			return fmt.Errorf("CFR case cannot declare a secondary frame rate")
		}
	} else if s.SecondaryFrameRateNumerator <= 0 || s.SecondaryFrameRateDenominator <= 0 {
		return fmt.Errorf("VFR case requires a secondary frame rate")
	}
	if s.AudioSampleRate != 48_000 || s.ExpectedBoundaryMicros <= 0 || s.DurationMicros <= s.ExpectedBoundaryMicros {
		return fmt.Errorf("case media policy is invalid")
	}
	return nil
}

func (s CaseSpec) DeclaredFrameRateMilli() int {
	primary := float64(s.PrimaryFrameRateNumerator) / float64(s.PrimaryFrameRateDenominator)
	if s.SourceMode == transcodesourceorigin.ModeCFR {
		return int(math.Round(primary * 1_000))
	}
	secondary := float64(s.SecondaryFrameRateNumerator) / float64(s.SecondaryFrameRateDenominator)
	return int(math.Round(((primary + secondary) / 2) * 1_000))
}

func (s CandidateSpec) Validate() error {
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.Description) == "" {
		return fmt.Errorf("encoder time-base candidate identity is incomplete")
	}
	switch s.ID {
	case CandidateAVTB:
		if s.EncoderTimeBase != "1/1000000" {
			return fmt.Errorf("AVTB candidate time base drifted")
		}
	case Candidate90K:
		if s.EncoderTimeBase != "1/90000" {
			return fmt.Errorf("90 kHz candidate time base drifted")
		}
	default:
		return fmt.Errorf("unsupported encoder time-base candidate %q", s.ID)
	}
	return nil
}
