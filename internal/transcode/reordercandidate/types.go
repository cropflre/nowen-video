package reordercandidate

import (
	"fmt"
	"strings"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
)

const SchemaVersion = "encoder-time-base-reorder-evidence-v1"

const (
	RepeatCount             = transcodetimebase.RepeatCount
	PacketVarianceTolerance = int64(0)
)

type CaseSpec struct {
	Base            transcodetimebase.CaseSpec `json:"base"`
	BFrames         int                        `json:"b_frames"`
	BAdapt          int                        `json:"b_adapt"`
	ReferenceFrames int                        `json:"reference_frames"`
	OpenGOP         bool                       `json:"open_gop"`
}

type PacketTimestamp struct {
	PTS int64
	DTS int64
}

type OffsetBucket struct {
	OffsetTicks  int64 `json:"offset_ticks"`
	OffsetMicros int64 `json:"offset_micros"`
	Count        int   `json:"count"`
}

type PacketOrderEvidence struct {
	Kind                         string                              `json:"kind"`
	TimeBase                     string                              `json:"time_base"`
	PacketCount                  int                                 `json:"packet_count"`
	FirstPTS                     int64                               `json:"first_pts"`
	FirstDTS                     int64                               `json:"first_dts"`
	LastPTS                      int64                               `json:"last_pts"`
	LastDTS                      int64                               `json:"last_dts"`
	FirstPTSMicros               int64                               `json:"first_pts_micros"`
	FirstDTSMicros               int64                               `json:"first_dts_micros"`
	LastPTSMicros                int64                               `json:"last_pts_micros"`
	LastDTSMicros                int64                               `json:"last_dts_micros"`
	MinCompositionOffsetTicks    int64                               `json:"min_composition_offset_ticks"`
	MaxCompositionOffsetTicks    int64                               `json:"max_composition_offset_ticks"`
	MinCompositionOffsetMicros   int64                               `json:"min_composition_offset_micros"`
	MaxCompositionOffsetMicros   int64                               `json:"max_composition_offset_micros"`
	ReorderedPacketCount         int                                 `json:"reordered_packet_count"`
	PTSBeforeDTSCount            int                                 `json:"pts_before_dts_count"`
	PTSAfterDTSCount             int                                 `json:"pts_after_dts_count"`
	PTSEqualDTSCount             int                                 `json:"pts_equal_dts_count"`
	AdjacentPTSInversionCount    int                                 `json:"adjacent_pts_inversion_count"`
	DTSNonMonotonicCount         int                                 `json:"dts_non_monotonic_count"`
	DTSDuplicateCount            int                                 `json:"dts_duplicate_count"`
	MaxPresentationReorderDepth  int                                 `json:"max_presentation_reorder_depth"`
	DTSDeltaHistogram            []transcodeoutputcadence.DeltaBucket `json:"dts_delta_histogram"`
	CompositionOffsetHistogram   []OffsetBucket                      `json:"composition_offset_histogram"`
}

type RunEvidence struct {
	Ordinal                 int                            `json:"ordinal"`
	Base                    transcodetimebase.RunEvidence  `json:"base"`
	StartupPacketOrder      PacketOrderEvidence            `json:"startup_packet_order"`
	ContinuationPacketOrder PacketOrderEvidence            `json:"continuation_packet_order"`
}

type CandidateSummary struct {
	Base                              transcodetimebase.CandidateSummary `json:"base"`
	StartupReorderedPacketCount       transcodetimebase.MetricRange      `json:"startup_reordered_packet_count"`
	ContinuationReorderedPacketCount  transcodetimebase.MetricRange      `json:"continuation_reordered_packet_count"`
	StartupMaxReorderDepth            transcodetimebase.MetricRange      `json:"startup_max_reorder_depth"`
	ContinuationMaxReorderDepth       transcodetimebase.MetricRange      `json:"continuation_max_reorder_depth"`
	StartupMaxCompositionOffsetMicros transcodetimebase.MetricRange      `json:"startup_max_composition_offset_micros"`
	ContinuationMaxCompositionOffsetMicros transcodetimebase.MetricRange `json:"continuation_max_composition_offset_micros"`
	PacketOrderStable                 bool                               `json:"packet_order_stable"`
	StrictDTS                         bool                               `json:"strict_dts"`
	ReorderObserved                   bool                               `json:"reorder_observed"`
	Stable                            bool                               `json:"stable"`
}

type CandidateEvidence struct {
	Spec    transcodetimebase.CandidateSpec `json:"spec"`
	Runs    []RunEvidence                   `json:"runs"`
	Summary CandidateSummary                `json:"summary"`
}

type CandidateComparison struct {
	Base                        transcodetimebase.CandidateComparison `json:"base"`
	StartupPacketOrderEquivalent bool                                `json:"startup_packet_order_equivalent"`
	ContinuationPacketOrderEquivalent bool                           `json:"continuation_packet_order_equivalent"`
	Equivalent                  bool                                  `json:"equivalent"`
}

type CaseEvidence struct {
	Case                       CaseSpec                                  `json:"case"`
	SourceStartupTimeline      transcodeoutputcadence.TimelineEvidence   `json:"source_startup_timeline"`
	SourceContinuationTimeline transcodeoutputcadence.TimelineEvidence   `json:"source_continuation_timeline"`
	Candidates                 []CandidateEvidence                       `json:"candidates"`
	Comparison                 CandidateComparison                       `json:"comparison"`
}

type Contract struct {
	SchemaVersion         string         `json:"schema_version"`
	FFmpegVersion         string         `json:"ffmpeg_version"`
	FFprobeVersion        string         `json:"ffprobe_version"`
	RepeatCount           int            `json:"repeat_count"`
	PacketVarianceTicks   int64          `json:"packet_variance_ticks"`
	Cases                 []CaseEvidence `json:"cases"`
	SeamlessAllowed       bool           `json:"seamless_allowed"`
	DiscontinuityRequired bool           `json:"discontinuity_required"`
}

func (s CaseSpec) Validate() error {
	if err := s.Base.Validate(); err != nil {
		return err
	}
	if s.BFrames < 1 || s.BFrames > 4 {
		return fmt.Errorf("B-frame policy is invalid")
	}
	if s.BAdapt != 0 {
		return fmt.Errorf("v1 requires deterministic b-adapt=0")
	}
	if s.ReferenceFrames < 1 || s.ReferenceFrames > 8 {
		return fmt.Errorf("reference-frame policy is invalid")
	}
	if s.OpenGOP {
		return fmt.Errorf("v1 requires closed GOP output")
	}
	return nil
}

func (c Contract) ValidateIdentity() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported reorder schema %q", c.SchemaVersion)
	}
	if strings.TrimSpace(c.FFmpegVersion) == "" || strings.TrimSpace(c.FFprobeVersion) == "" {
		return fmt.Errorf("toolchain identity is incomplete")
	}
	if c.RepeatCount != RepeatCount || c.PacketVarianceTicks != PacketVarianceTolerance {
		return fmt.Errorf("reorder variance policy is invalid")
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("reorder evidence cannot authorize seamless playback")
	}
	return nil
}
