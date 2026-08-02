package longdrift

import (
	"strings"
	"testing"
)

func TestPolicySupportsSixHourEvidence(t *testing.T) {
	policy := Policy{
		DurationMicros:                6 * 60 * 60 * 1_000_000,
		CheckpointIntervalMicros:      60 * 60 * 1_000_000,
		RepeatCount:                   1,
		StartToleranceMicros:          StartToleranceMicros,
		EndToleranceMicros:            EndToleranceMicros,
		CheckpointToleranceMicros:     CheckpointToleranceMicros,
		AVSkewToleranceMicros:         AVSkewToleranceMicros,
		RepeatVarianceToleranceMicros: 0,
		CrossCandidateToleranceMicros: CrossCandidateToleranceMicros,
	}
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
	run := validPolicyRun(policy)
	candidate := CandidateEvidence{
		ID:              "candidate",
		EncoderTimeBase: "1/90000",
		Runs:            []RunEvidence{run},
	}
	candidate.Summary = BuildCandidateSummaryForPolicy(candidate.Runs, policy)
	if err := candidate.ValidateForPolicy(policy); err != nil {
		t.Fatalf("six-hour policy rejected: %v", err)
	}
	if len(run.Video.Checkpoints) != 7 || run.SegmentCount != 10_800 {
		t.Fatalf("unexpected six-hour geometry: checkpoints=%d segments=%d", len(run.Video.Checkpoints), run.SegmentCount)
	}
}

func TestPolicyRejectsNonDivisibleCheckpointInterval(t *testing.T) {
	policy := DefaultPolicy()
	policy.CheckpointIntervalMicros = 7 * 60 * 1_000_000
	if err := policy.Validate(); err == nil {
		t.Fatal("non-divisible checkpoint interval was accepted")
	}
}

func TestPolicyComparisonSupportsSingleExecution(t *testing.T) {
	policy := DefaultPolicy()
	policy.RepeatCount = 1
	leftRun := validPolicyRun(policy)
	rightRun := validPolicyRun(policy)
	left := CandidateEvidence{ID: "left", Runs: []RunEvidence{leftRun}}
	right := CandidateEvidence{ID: "right", Runs: []RunEvidence{rightRun}}
	comparison := BuildCandidateComparisonForPolicy(left, right, policy)
	if !comparison.Equivalent {
		t.Fatalf("equal single executions diverged: %+v", comparison)
	}
	right.Runs[0].Video.Checkpoints[2].ErrorMicros = policy.CrossCandidateToleranceMicros + 1
	comparison = BuildCandidateComparisonForPolicy(left, right, policy)
	if comparison.Equivalent {
		t.Fatalf("checkpoint divergence was accepted: %+v", comparison)
	}
}

func validPolicyRun(policy Policy) RunEvidence {
	hash := strings.Repeat("a", 64)
	video := validPolicyStream("video", policy)
	audio := validPolicyStream("audio", policy)
	return RunEvidence{
		Ordinal:            1,
		CommandHash:        hash,
		ManifestSHA256:     hash,
		AttestationVersion: "hls-produced-media-attestation-v1",
		AttestationHash:    hash,
		SegmentCount:       int(policy.DurationMicros / HLSSegmentMicros),
		Video:              video,
		Audio:              audio,
		FinalAVSkewMicros:  video.EndMicros - audio.EndMicros,
	}
}

func validPolicyStream(kind string, policy Policy) StreamEvidence {
	checkpoints := make([]CheckpointEvidence, 0, len(CheckpointTargetsForPolicy(policy)))
	for _, target := range CheckpointTargetsForPolicy(policy) {
		checkpoints = append(checkpoints, CheckpointEvidence{TargetMicros: target, PresentationMicros: target})
	}
	return StreamEvidence{
		Kind:           kind,
		TimeBase:       "1/90000",
		PacketCount:    100,
		StartMicros:    0,
		EndMicros:      policy.DurationMicros,
		DurationMicros: policy.DurationMicros,
		Checkpoints:    checkpoints,
	}
}
