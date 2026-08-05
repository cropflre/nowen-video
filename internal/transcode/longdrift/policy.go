package longdrift

import (
	"fmt"
	"strings"
)

const (
	HLSSegmentMicros      int64 = 2_000_000
	SegmentCountTolerance       = 10
)

// Policy makes duration and checkpoint geometry explicit while preserving the
// historical 30-minute v1 contract. Multi-hour certifications use the same
// evidence value objects with a different immutable policy.
type Policy struct {
	DurationMicros                int64 `json:"duration_micros"`
	CheckpointIntervalMicros      int64 `json:"checkpoint_interval_micros"`
	RepeatCount                   int   `json:"repeat_count"`
	StartToleranceMicros          int64 `json:"start_tolerance_micros"`
	EndToleranceMicros            int64 `json:"end_tolerance_micros"`
	CheckpointToleranceMicros     int64 `json:"checkpoint_tolerance_micros"`
	AVSkewToleranceMicros         int64 `json:"av_skew_tolerance_micros"`
	RepeatVarianceToleranceMicros int64 `json:"repeat_variance_tolerance_micros"`
	CrossCandidateToleranceMicros int64 `json:"cross_candidate_tolerance_micros"`
}

func DefaultPolicy() Policy {
	return Policy{
		DurationMicros:                DurationMicros,
		CheckpointIntervalMicros:      CheckpointMicros,
		RepeatCount:                   RepeatCount,
		StartToleranceMicros:          StartToleranceMicros,
		EndToleranceMicros:            EndToleranceMicros,
		CheckpointToleranceMicros:     CheckpointToleranceMicros,
		AVSkewToleranceMicros:         AVSkewToleranceMicros,
		RepeatVarianceToleranceMicros: RepeatVarianceToleranceMicros,
		CrossCandidateToleranceMicros: CrossCandidateToleranceMicros,
	}
}

func (p Policy) Validate() error {
	if p.DurationMicros <= 0 || p.CheckpointIntervalMicros <= 0 || p.DurationMicros%p.CheckpointIntervalMicros != 0 {
		return fmt.Errorf("long-duration policy geometry is invalid")
	}
	if p.RepeatCount <= 0 {
		return fmt.Errorf("long-duration policy repeat count is invalid")
	}
	for label, value := range map[string]int64{
		"start tolerance":           p.StartToleranceMicros,
		"end tolerance":             p.EndToleranceMicros,
		"checkpoint tolerance":      p.CheckpointToleranceMicros,
		"A/V skew tolerance":        p.AVSkewToleranceMicros,
		"repeat variance tolerance": p.RepeatVarianceToleranceMicros,
		"cross-candidate tolerance": p.CrossCandidateToleranceMicros,
	} {
		if value < 0 {
			return fmt.Errorf("long-duration %s is invalid", label)
		}
	}
	return nil
}

func BuildCandidateSummaryForPolicy(runs []RunEvidence, policy Policy) CandidateSummary {
	summary := CandidateSummary{RepeatCount: len(runs), Stable: policy.Validate() == nil && len(runs) == policy.RepeatCount}
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
		summary.MaximumAbsoluteVideoEndErrorMicros <= policy.EndToleranceMicros &&
		summary.MaximumAbsoluteAudioEndErrorMicros <= policy.EndToleranceMicros &&
		summary.MaximumAbsoluteAVSkewMicros <= policy.AVSkewToleranceMicros &&
		summary.MaximumAbsoluteCheckpointErrorMicros <= policy.CheckpointToleranceMicros &&
		summary.MaximumRepeatMetricVarianceMicros <= policy.RepeatVarianceToleranceMicros
	return summary
}

func BuildCandidateComparisonForPolicy(a, b CandidateEvidence, policy Policy) CandidateComparison {
	result := CandidateComparison{CandidateAID: a.ID, CandidateBID: b.ID}
	if policy.Validate() != nil || len(a.Runs) != policy.RepeatCount || len(b.Runs) != policy.RepeatCount {
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
	result.Equivalent = result.MaximumVideoEndDifferenceMicros <= policy.CrossCandidateToleranceMicros &&
		result.MaximumAudioEndDifferenceMicros <= policy.CrossCandidateToleranceMicros &&
		result.MaximumAVSkewDifferenceMicros <= policy.CrossCandidateToleranceMicros &&
		result.MaximumCheckpointDifferenceMicros <= policy.CrossCandidateToleranceMicros
	return result
}

func (c CandidateEvidence) ValidateForPolicy(policy Policy) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	if len(c.Runs) != policy.RepeatCount {
		return fmt.Errorf("candidate has %d repeats, want %d", len(c.Runs), policy.RepeatCount)
	}
	for index, run := range c.Runs {
		if err := run.ValidateForPolicy(index+1, policy); err != nil {
			return err
		}
	}
	if c.Summary != BuildCandidateSummaryForPolicy(c.Runs, policy) || !c.Summary.Stable {
		return fmt.Errorf("candidate summary is unstable")
	}
	return nil
}

func (r RunEvidence) ValidateForPolicy(ordinal int, policy Policy) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	if r.Ordinal != ordinal {
		return fmt.Errorf("run ordinal is invalid")
	}
	for label, value := range map[string]string{"command": r.CommandHash, "manifest": r.ManifestSHA256, "attestation": r.AttestationHash} {
		if !isSHA256(value) {
			return fmt.Errorf("%s identity is invalid", label)
		}
	}
	expectedSegments := int((policy.DurationMicros + HLSSegmentMicros - 1) / HLSSegmentMicros)
	if strings.TrimSpace(r.AttestationVersion) == "" || r.SegmentCount < expectedSegments-SegmentCountTolerance || r.SegmentCount > expectedSegments+SegmentCountTolerance {
		return fmt.Errorf("run attestation or segment count is invalid")
	}
	if err := r.Video.ValidateForPolicy("video", policy); err != nil {
		return err
	}
	if err := r.Audio.ValidateForPolicy("audio", policy); err != nil {
		return err
	}
	if r.FinalAVSkewMicros != r.Video.EndMicros-r.Audio.EndMicros || abs64(r.FinalAVSkewMicros) > policy.AVSkewToleranceMicros {
		return fmt.Errorf("final A/V skew is invalid")
	}
	return nil
}

func (s StreamEvidence) ValidateForPolicy(kind string, policy Policy) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	if s.Kind != kind || strings.TrimSpace(s.TimeBase) == "" || s.PacketCount <= 0 {
		return fmt.Errorf("%s stream identity is invalid", kind)
	}
	if abs64(s.StartMicros) > policy.StartToleranceMicros || s.EndMicros <= s.StartMicros || s.DurationMicros != s.EndMicros-s.StartMicros || s.EndErrorMicros != s.DurationMicros-policy.DurationMicros || abs64(s.EndErrorMicros) > policy.EndToleranceMicros {
		return fmt.Errorf("%s stream duration evidence is invalid", kind)
	}
	expectedCount := int(policy.DurationMicros/policy.CheckpointIntervalMicros) + 1
	if len(s.Checkpoints) != expectedCount {
		return fmt.Errorf("%s checkpoint count is invalid", kind)
	}
	lastPresentation := int64(-1)
	for index, checkpoint := range s.Checkpoints {
		wantTarget := int64(index) * policy.CheckpointIntervalMicros
		if checkpoint.TargetMicros != wantTarget || checkpoint.ErrorMicros != checkpoint.PresentationMicros-checkpoint.TargetMicros || abs64(checkpoint.ErrorMicros) > policy.CheckpointToleranceMicros || checkpoint.PresentationMicros < lastPresentation {
			return fmt.Errorf("%s checkpoint %d is invalid", kind, index)
		}
		lastPresentation = checkpoint.PresentationMicros
	}
	return nil
}

func CheckpointTargetsForPolicy(policy Policy) []int64 {
	if err := policy.Validate(); err != nil {
		return nil
	}
	result := make([]int64, 0, int(policy.DurationMicros/policy.CheckpointIntervalMicros)+1)
	for value := int64(0); value <= policy.DurationMicros; value += policy.CheckpointIntervalMicros {
		result = append(result, value)
	}
	return result
}
