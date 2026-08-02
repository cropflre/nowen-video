package realmediacandidate

import (
	"fmt"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
)

const (
	SchemaVersion                       = "real-media-corpus-candidate-evidence-v1"
	RepeatCount                         = transcodereorder.RepeatCount
	PacketOrderComparisonToleranceTicks = int64(1)
)

type EvidenceIdentity struct {
	Version string `json:"version"`
	Hash    string `json:"hash"`
}

type SourceIdentity struct {
	AssetIndex        int    `json:"asset_index"`
	CaseID            string `json:"case_id"`
	RelativePath      string `json:"relative_path"`
	SHA256            string `json:"sha256"`
	SizeBytes         int64  `json:"size_bytes"`
	AssetEvidenceHash string `json:"asset_evidence_hash"`
}

type CaseEvidence struct {
	Source            SourceIdentity                `json:"source"`
	RequiredEvidence  []string                      `json:"required_evidence"`
	TimestampPlan     EvidenceIdentity              `json:"timestamp_plan"`
	TimeBaseCandidate EvidenceIdentity              `json:"time_base_candidate"`
	ReorderCandidate  EvidenceIdentity              `json:"reorder_candidate"`
	Evidence          transcodereorder.CaseEvidence `json:"evidence"`
}

type Contract struct {
	SchemaVersion                       string         `json:"schema_version"`
	SpecVersion                         string         `json:"spec_version"`
	SpecHash                            string         `json:"spec_hash"`
	ManifestVersion                     string         `json:"manifest_version"`
	ManifestHash                        string         `json:"manifest_hash"`
	SourceGeneratorVersion              string         `json:"source_generator_version"`
	SourceFFmpegVersion                 string         `json:"source_ffmpeg_version"`
	SourceFFprobeVersion                string         `json:"source_ffprobe_version"`
	CertificationFFmpegVersion          string         `json:"certification_ffmpeg_version"`
	CertificationFFprobeVersion         string         `json:"certification_ffprobe_version"`
	RepeatCount                         int            `json:"repeat_count"`
	PacketOrderComparisonToleranceTicks int64          `json:"packet_order_comparison_tolerance_ticks"`
	Cases                               []CaseEvidence `json:"cases"`
	SeamlessAllowed                     bool           `json:"seamless_allowed"`
	DiscontinuityRequired               bool           `json:"discontinuity_required"`
}

func CaseSpecFor(caseSpec transcodecorpus.CaseSpec, asset transcodecorpus.AssetEvidence) (transcodereorder.CaseSpec, error) {
	if err := caseSpec.Validate(); err != nil {
		return transcodereorder.CaseSpec{}, err
	}
	if err := asset.ValidateFor(caseSpec); err != nil {
		return transcodereorder.CaseSpec{}, err
	}
	mode := transcodesourceorigin.ModeCFR
	if caseSpec.Source.Video.FrameRateMode == transcodecorpus.FrameRateVFR {
		mode = transcodesourceorigin.ModeVFR
	}
	rates := caseSpec.Source.Video.FrameRates
	base := transcodetimebase.CaseSpec{
		ID:                          caseSpec.ID,
		Description:                 caseSpec.Description,
		SourceMode:                  mode,
		PrimaryFrameRateNumerator:   rates[0].Numerator,
		PrimaryFrameRateDenominator: rates[0].Denominator,
		SourceOffsetMicros:          asset.Probe.StartMicros,
		AudioSampleRate:             caseSpec.Source.Audio.SampleRate,
		GOPSize:                     caseSpec.Source.Video.GOPSize,
		ExpectedBoundaryMicros:      caseSpec.BoundaryMicros,
		DurationMicros:              caseSpec.Source.Timeline.DurationMicros,
	}
	if mode == transcodesourceorigin.ModeVFR {
		base.SecondaryFrameRateNumerator = rates[1].Numerator
		base.SecondaryFrameRateDenominator = rates[1].Denominator
	}
	result := transcodereorder.CaseSpec{
		Base:            base,
		BFrames:         caseSpec.Source.Video.BFrames,
		BAdapt:          0,
		ReferenceFrames: caseSpec.Source.Video.ReferenceFrames,
		OpenGOP:         caseSpec.Source.Video.OpenGOP,
	}
	if err := result.Validate(); err != nil {
		return transcodereorder.CaseSpec{}, fmt.Errorf("real-media candidate case %s: %w", caseSpec.ID, err)
	}
	return result, nil
}

func BaseEvidence(evidence transcodereorder.CaseEvidence) transcodetimebase.CaseEvidence {
	candidates := make([]transcodetimebase.CandidateEvidence, 0, len(evidence.Candidates))
	for _, candidate := range evidence.Candidates {
		runs := make([]transcodetimebase.RunEvidence, 0, len(candidate.Runs))
		for _, run := range candidate.Runs {
			runs = append(runs, run.Base)
		}
		candidates = append(candidates, transcodetimebase.CandidateEvidence{
			Spec:    candidate.Spec,
			Runs:    runs,
			Summary: candidate.Summary.Base,
		})
	}
	return transcodetimebase.CaseEvidence{
		Case:                       evidence.Case.Base,
		SourceStartupTimeline:      evidence.SourceStartupTimeline,
		SourceContinuationTimeline: evidence.SourceContinuationTimeline,
		Candidates:                 candidates,
		Comparison:                 evidence.Comparison.Base,
	}
}
