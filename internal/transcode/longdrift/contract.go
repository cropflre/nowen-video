package longdrift

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
)

const (
	SchemaVersion                         = "long-duration-drift-evidence-v1"
	SourceCaseID                          = "real-mp4-h264-aac-cfr-30-aac-44100-v1"
	DurationMicros                  int64 = 30 * 60 * 1_000_000
	CheckpointMicros                int64 = 5 * 60 * 1_000_000
	RepeatCount                           = 2
	StartToleranceMicros            int64 = 3_000_000
	EndToleranceMicros              int64 = 50_000
	CheckpointToleranceMicros       int64 = 50_000
	AVSkewToleranceMicros           int64 = 50_000
	RepeatVarianceToleranceMicros   int64 = 2_000
	CrossCandidateToleranceMicros   int64 = 2_000
)

type SourceIdentity struct {
	CaseID            string `json:"case_id"`
	RelativePath      string `json:"relative_path"`
	SHA256            string `json:"sha256"`
	SizeBytes         int64  `json:"size_bytes"`
	AssetEvidenceHash string `json:"asset_evidence_hash"`
}

type CheckpointEvidence struct {
	TargetMicros       int64 `json:"target_micros"`
	PresentationMicros int64 `json:"presentation_micros"`
	ErrorMicros        int64 `json:"error_micros"`
}

type StreamEvidence struct {
	Kind           string               `json:"kind"`
	TimeBase       string               `json:"time_base"`
	PacketCount    int                  `json:"packet_count"`
	StartMicros    int64                `json:"start_micros"`
	EndMicros      int64                `json:"end_micros"`
	DurationMicros int64                `json:"duration_micros"`
	EndErrorMicros int64                `json:"end_error_micros"`
	Checkpoints    []CheckpointEvidence `json:"checkpoints"`
}

type RunEvidence struct {
	Ordinal            int            `json:"ordinal"`
	CommandHash        string         `json:"command_hash"`
	ManifestSHA256     string         `json:"manifest_sha256"`
	AttestationVersion string         `json:"attestation_version"`
	AttestationHash    string         `json:"attestation_hash"`
	SegmentCount       int            `json:"segment_count"`
	Video              StreamEvidence `json:"video"`
	Audio              StreamEvidence `json:"audio"`
	FinalAVSkewMicros  int64          `json:"final_av_skew_micros"`
}

type CandidateSummary struct {
	RepeatCount                           int   `json:"repeat_count"`
	MaximumAbsoluteVideoEndErrorMicros    int64 `json:"maximum_absolute_video_end_error_micros"`
	MaximumAbsoluteAudioEndErrorMicros    int64 `json:"maximum_absolute_audio_end_error_micros"`
	MaximumAbsoluteAVSkewMicros           int64 `json:"maximum_absolute_av_skew_micros"`
	MaximumAbsoluteCheckpointErrorMicros int64 `json:"maximum_absolute_checkpoint_error_micros"`
	MaximumRepeatMetricVarianceMicros     int64 `json:"maximum_repeat_metric_variance_micros"`
	Stable                                bool  `json:"stable"`
}

type CandidateEvidence struct {
	ID              string           `json:"id"`
	EncoderTimeBase string           `json:"encoder_time_base"`
	Runs            []RunEvidence    `json:"runs"`
	Summary         CandidateSummary `json:"summary"`
}

type CandidateComparison struct {
	CandidateAID                      string `json:"candidate_a_id"`
	CandidateBID                      string `json:"candidate_b_id"`
	MaximumVideoEndDifferenceMicros   int64  `json:"maximum_video_end_difference_micros"`
	MaximumAudioEndDifferenceMicros   int64  `json:"maximum_audio_end_difference_micros"`
	MaximumAVSkewDifferenceMicros     int64  `json:"maximum_av_skew_difference_micros"`
	MaximumCheckpointDifferenceMicros int64  `json:"maximum_checkpoint_difference_micros"`
	Equivalent                        bool   `json:"equivalent"`
}

type Contract struct {
	SchemaVersion                  string              `json:"schema_version"`
	SpecVersion                    string              `json:"spec_version"`
	SpecHash                       string              `json:"spec_hash"`
	ManifestVersion                string              `json:"manifest_version"`
	ManifestHash                   string              `json:"manifest_hash"`
	SourceGeneratorVersion         string              `json:"source_generator_version"`
	SourceFFmpegVersion            string              `json:"source_ffmpeg_version"`
	SourceFFprobeVersion           string              `json:"source_ffprobe_version"`
	CertificationFFmpegVersion     string              `json:"certification_ffmpeg_version"`
	CertificationFFprobeVersion    string              `json:"certification_ffprobe_version"`
	TimestampPlanVersion           string              `json:"timestamp_plan_version"`
	TimestampPlanHash              string              `json:"timestamp_plan_hash"`
	Source                         SourceIdentity      `json:"source"`
	DurationMicros                 int64               `json:"duration_micros"`
	CheckpointIntervalMicros       int64               `json:"checkpoint_interval_micros"`
	RepeatCount                    int                 `json:"repeat_count"`
	StartToleranceMicros           int64               `json:"start_tolerance_micros"`
	EndToleranceMicros             int64               `json:"end_tolerance_micros"`
	CheckpointToleranceMicros      int64               `json:"checkpoint_tolerance_micros"`
	AVSkewToleranceMicros          int64               `json:"av_skew_tolerance_micros"`
	RepeatVarianceToleranceMicros  int64               `json:"repeat_variance_tolerance_micros"`
	CrossCandidateToleranceMicros  int64               `json:"cross_candidate_tolerance_micros"`
	Candidates                     []CandidateEvidence `json:"candidates"`
	Comparison                     CandidateComparison `json:"comparison"`
	SeamlessAllowed                bool                `json:"seamless_allowed"`
	DiscontinuityRequired          bool                `json:"discontinuity_required"`
}

func BuildCandidateSummary(runs []RunEvidence) CandidateSummary {
	summary := CandidateSummary{RepeatCount: len(runs), Stable: len(runs) == RepeatCount}
	metrics := make([][]int64, 0, len(runs))
	for _, run := range runs {
		checkpointMax := int64(0)
		for _, stream := range []StreamEvidence{run.Video, run.Audio} {
			for _, checkpoint := range stream.Checkpoints {
				checkpointMax = max64(checkpointMax, abs64(checkpoint.ErrorMicros))
			}
		}
		summary.MaximumAbsoluteVideoEndErrorMicros = max64(summary.MaximumAbsoluteVideoEndErrorMicros, abs64(run.Video.EndErrorMicros))
		summary.MaximumAbsoluteAudioEndErrorMicros = max64(summary.MaximumAbsoluteAudioEndErrorMicros, abs64(run.Audio.EndErrorMicros))
		summary.MaximumAbsoluteAVSkewMicros = max64(summary.MaximumAbsoluteAVSkewMicros, abs64(run.FinalAVSkewMicros))
		summary.MaximumAbsoluteCheckpointErrorMicros = max64(summary.MaximumAbsoluteCheckpointErrorMicros, checkpointMax)
		metrics = append(metrics, []int64{run.Video.EndErrorMicros, run.Audio.EndErrorMicros, run.FinalAVSkewMicros, checkpointMax})
	}
	for metric := 0; metric < 4 && len(metrics) > 0; metric++ {
		minimum := metrics[0][metric]
		maximum := minimum
		for _, values := range metrics[1:] {
			if values[metric] < minimum {
				minimum = values[metric]
			}
			if values[metric] > maximum {
				maximum = values[metric]
			}
		}
		summary.MaximumRepeatMetricVarianceMicros = max64(summary.MaximumRepeatMetricVarianceMicros, maximum-minimum)
	}
	summary.Stable = summary.Stable &&
		summary.MaximumAbsoluteVideoEndErrorMicros <= EndToleranceMicros &&
		summary.MaximumAbsoluteAudioEndErrorMicros <= EndToleranceMicros &&
		summary.MaximumAbsoluteAVSkewMicros <= AVSkewToleranceMicros &&
		summary.MaximumAbsoluteCheckpointErrorMicros <= CheckpointToleranceMicros &&
		summary.MaximumRepeatMetricVarianceMicros <= RepeatVarianceToleranceMicros
	return summary
}

func BuildCandidateComparison(a, b CandidateEvidence) CandidateComparison {
	result := CandidateComparison{CandidateAID: a.ID, CandidateBID: b.ID}
	if len(a.Runs) != RepeatCount || len(b.Runs) != RepeatCount {
		return result
	}
	for index := range a.Runs {
		left := a.Runs[index]
		right := b.Runs[index]
		result.MaximumVideoEndDifferenceMicros = max64(result.MaximumVideoEndDifferenceMicros, abs64(left.Video.EndErrorMicros-right.Video.EndErrorMicros))
		result.MaximumAudioEndDifferenceMicros = max64(result.MaximumAudioEndDifferenceMicros, abs64(left.Audio.EndErrorMicros-right.Audio.EndErrorMicros))
		result.MaximumAVSkewDifferenceMicros = max64(result.MaximumAVSkewDifferenceMicros, abs64(left.FinalAVSkewMicros-right.FinalAVSkewMicros))
		pairs := [][2][]CheckpointEvidence{
			{left.Video.Checkpoints, right.Video.Checkpoints},
			{left.Audio.Checkpoints, right.Audio.Checkpoints},
		}
		for _, pair := range pairs {
			if len(pair[0]) != len(pair[1]) {
				return result
			}
			for checkpoint := range pair[0] {
				if pair[0][checkpoint].TargetMicros != pair[1][checkpoint].TargetMicros {
					return result
				}
				result.MaximumCheckpointDifferenceMicros = max64(result.MaximumCheckpointDifferenceMicros, abs64(pair[0][checkpoint].ErrorMicros-pair[1][checkpoint].ErrorMicros))
			}
		}
	}
	result.Equivalent = result.MaximumVideoEndDifferenceMicros <= CrossCandidateToleranceMicros &&
		result.MaximumAudioEndDifferenceMicros <= CrossCandidateToleranceMicros &&
		result.MaximumAVSkewDifferenceMicros <= CrossCandidateToleranceMicros &&
		result.MaximumCheckpointDifferenceMicros <= CrossCandidateToleranceMicros
	return result
}

func (c Contract) ValidateFor(spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported long-duration drift schema %q", c.SchemaVersion)
	}
	if err := manifest.ValidateFor(spec); err != nil {
		return err
	}
	specVersion, specHash, _, err := transcodecorpus.SpecIdentity(spec)
	if err != nil {
		return err
	}
	manifestVersion, manifestHash, _, err := transcodecorpus.ManifestIdentity(manifest, spec)
	if err != nil {
		return err
	}
	if c.SpecVersion != specVersion || c.SpecHash != specHash || c.ManifestVersion != manifestVersion || c.ManifestHash != manifestHash {
		return fmt.Errorf("long-duration source contract identity is invalid")
	}
	if c.SourceGeneratorVersion != manifest.GeneratorVersion || c.SourceFFmpegVersion != manifest.FFmpegVersion || c.SourceFFprobeVersion != manifest.FFprobeVersion {
		return fmt.Errorf("long-duration source toolchain differs from manifest")
	}
	if strings.TrimSpace(c.CertificationFFmpegVersion) == "" || strings.TrimSpace(c.CertificationFFprobeVersion) == "" {
		return fmt.Errorf("certification toolchain is incomplete")
	}
	if strings.TrimSpace(c.TimestampPlanVersion) == "" || !isSHA256(c.TimestampPlanHash) {
		return fmt.Errorf("timestamp plan identity is invalid")
	}
	if c.DurationMicros != DurationMicros || c.CheckpointIntervalMicros != CheckpointMicros || c.RepeatCount != RepeatCount ||
		c.StartToleranceMicros != StartToleranceMicros || c.EndToleranceMicros != EndToleranceMicros || c.CheckpointToleranceMicros != CheckpointToleranceMicros ||
		c.AVSkewToleranceMicros != AVSkewToleranceMicros || c.RepeatVarianceToleranceMicros != RepeatVarianceToleranceMicros || c.CrossCandidateToleranceMicros != CrossCandidateToleranceMicros {
		return fmt.Errorf("long-duration drift policy is invalid")
	}
	caseSpec, asset, ok := findSource(spec, manifest)
	if !ok {
		return fmt.Errorf("long-duration source case is missing")
	}
	if c.Source.CaseID != SourceCaseID || c.Source.RelativePath != asset.RelativePath || c.Source.SHA256 != asset.SHA256 || c.Source.SizeBytes != asset.SizeBytes {
		return fmt.Errorf("long-duration source identity differs from manifest")
	}
	assetHash, err := CanonicalHash(asset)
	if err != nil {
		return err
	}
	if c.Source.AssetEvidenceHash != assetHash || !isSHA256(c.Source.AssetEvidenceHash) {
		return fmt.Errorf("long-duration asset identity is invalid")
	}
	if caseSpec.Source.Audio.SampleRate != 44_100 || caseSpec.Source.Video.BFrames != 3 {
		return fmt.Errorf("long-duration source profile is invalid")
	}
	if len(c.Candidates) != 2 {
		return fmt.Errorf("long-duration matrix must contain two candidates")
	}
	wants := []struct {
		id string
		tb string
	}{
		{transcodetimebase.CandidateAVTB, "1/1000000"},
		{transcodetimebase.Candidate90K, "1/90000"},
	}
	for index, candidate := range c.Candidates {
		want := wants[index]
		if candidate.ID != want.id || candidate.EncoderTimeBase != want.tb {
			return fmt.Errorf("long-duration candidate order is invalid")
		}
		if err := candidate.Validate(); err != nil {
			return fmt.Errorf("validate candidate %s: %w", candidate.ID, err)
		}
	}
	if c.Comparison != BuildCandidateComparison(c.Candidates[0], c.Candidates[1]) || !c.Comparison.Equivalent {
		return fmt.Errorf("long-duration candidates diverged")
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("long-duration evidence cannot authorize seamless playback")
	}
	return nil
}

func (c CandidateEvidence) Validate() error {
	if len(c.Runs) != RepeatCount {
		return fmt.Errorf("candidate has %d repeats, want %d", len(c.Runs), RepeatCount)
	}
	for index, run := range c.Runs {
		if err := run.Validate(index + 1); err != nil {
			return err
		}
	}
	if c.Summary != BuildCandidateSummary(c.Runs) || !c.Summary.Stable {
		return fmt.Errorf("candidate summary is unstable")
	}
	return nil
}

func (r RunEvidence) Validate(ordinal int) error {
	if r.Ordinal != ordinal {
		return fmt.Errorf("run ordinal is invalid")
	}
	for label, value := range map[string]string{"command": r.CommandHash, "manifest": r.ManifestSHA256, "attestation": r.AttestationHash} {
		if !isSHA256(value) {
			return fmt.Errorf("%s identity is invalid", label)
		}
	}
	if strings.TrimSpace(r.AttestationVersion) == "" || r.SegmentCount < 890 || r.SegmentCount > 910 {
		return fmt.Errorf("run attestation or segment count is invalid")
	}
	if err := r.Video.Validate("video"); err != nil {
		return err
	}
	if err := r.Audio.Validate("audio"); err != nil {
		return err
	}
	if r.FinalAVSkewMicros != r.Video.EndMicros-r.Audio.EndMicros || abs64(r.FinalAVSkewMicros) > AVSkewToleranceMicros {
		return fmt.Errorf("final A/V skew is invalid")
	}
	return nil
}

func (s StreamEvidence) Validate(kind string) error {
	if s.Kind != kind || strings.TrimSpace(s.TimeBase) == "" || s.PacketCount <= 0 {
		return fmt.Errorf("%s stream identity is invalid", kind)
	}
	if abs64(s.StartMicros) > StartToleranceMicros || s.EndMicros <= s.StartMicros || s.DurationMicros != s.EndMicros-s.StartMicros || s.EndErrorMicros != s.DurationMicros-DurationMicros || abs64(s.EndErrorMicros) > EndToleranceMicros {
		return fmt.Errorf("%s stream duration evidence is invalid", kind)
	}
	expectedCount := int(DurationMicros/CheckpointMicros) + 1
	if len(s.Checkpoints) != expectedCount {
		return fmt.Errorf("%s checkpoint count is invalid", kind)
	}
	lastPresentation := int64(-1)
	for index, checkpoint := range s.Checkpoints {
		wantTarget := int64(index) * CheckpointMicros
		if checkpoint.TargetMicros != wantTarget || checkpoint.ErrorMicros != checkpoint.PresentationMicros-checkpoint.TargetMicros || abs64(checkpoint.ErrorMicros) > CheckpointToleranceMicros || checkpoint.PresentationMicros < lastPresentation {
			return fmt.Errorf("%s checkpoint %d is invalid", kind, index)
		}
		lastPresentation = checkpoint.PresentationMicros
	}
	return nil
}

func Identity(contract Contract, spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) (version, hash, canonical string, err error) {
	if err := contract.ValidateFor(spec, manifest); err != nil {
		return "", "", "", err
	}
	content, err := json.Marshal(contract)
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256(content)
	return contract.SchemaVersion, hex.EncodeToString(digest[:]), string(content), nil
}

func CanonicalHash(value any) (string, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}

func findSource(spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) (transcodecorpus.CaseSpec, transcodecorpus.AssetEvidence, bool) {
	assets := make(map[string]transcodecorpus.AssetEvidence, len(manifest.Assets))
	for _, asset := range manifest.Assets {
		assets[asset.CaseID] = asset
	}
	for _, caseSpec := range spec.Cases {
		if caseSpec.ID == SourceCaseID {
			asset, ok := assets[caseSpec.ID]
			return caseSpec, asset, ok
		}
	}
	return transcodecorpus.CaseSpec{}, transcodecorpus.AssetEvidence{}, false
}

func CheckpointTargets() []int64 {
	result := make([]int64, 0, int(DurationMicros/CheckpointMicros)+1)
	for value := int64(0); value <= DurationMicros; value += CheckpointMicros {
		result = append(result, value)
	}
	return result
}

func SortCheckpoints(values []CheckpointEvidence) {
	sort.Slice(values, func(i, j int) bool { return values[i].TargetMicros < values[j].TargetMicros })
}

func abs64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func max64(a, b int64) int64 {
	if b > a {
		return b
	}
	return a
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
