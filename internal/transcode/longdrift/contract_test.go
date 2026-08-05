package longdrift

import (
	"strings"
	"testing"
)

func TestCandidateSummaryRejectsAccumulatedDrift(t *testing.T) {
	runs := []RunEvidence{validRun(1), validRun(2)}
	summary := BuildCandidateSummary(runs)
	if !summary.Stable {
		t.Fatalf("valid runs were rejected: %+v", summary)
	}
	runs[1].Audio.EndMicros += EndToleranceMicros + 1
	runs[1].Audio.DurationMicros = runs[1].Audio.EndMicros - runs[1].Audio.StartMicros
	runs[1].Audio.EndErrorMicros = runs[1].Audio.EndMicros - DurationMicros
	runs[1].FinalAVSkewMicros = runs[1].Video.EndMicros - runs[1].Audio.EndMicros
	summary = BuildCandidateSummary(runs)
	if summary.Stable {
		t.Fatalf("accumulated audio drift was accepted: %+v", summary)
	}
}

func TestCandidateComparisonUsesCheckpointErrors(t *testing.T) {
	leftRuns := []RunEvidence{validRun(1), validRun(2)}
	rightRuns := []RunEvidence{validRun(1), validRun(2)}
	left := CandidateEvidence{ID: "left", EncoderTimeBase: "1/1000000", Runs: leftRuns}
	right := CandidateEvidence{ID: "right", EncoderTimeBase: "1/90000", Runs: rightRuns}
	comparison := BuildCandidateComparison(left, right)
	if !comparison.Equivalent {
		t.Fatalf("equivalent candidates were rejected: %+v", comparison)
	}
	right.Runs[0].Video.Checkpoints[3].PresentationMicros += CrossCandidateToleranceMicros + 1
	right.Runs[0].Video.Checkpoints[3].ErrorMicros += CrossCandidateToleranceMicros + 1
	comparison = BuildCandidateComparison(left, right)
	if comparison.Equivalent {
		t.Fatalf("checkpoint divergence was accepted: %+v", comparison)
	}
}

func validRun(ordinal int) RunEvidence {
	hash := strings.Repeat("a", 64)
	video := validStream("video")
	audio := validStream("audio")
	return RunEvidence{
		Ordinal:            ordinal,
		CommandHash:        hash,
		ManifestSHA256:     hash,
		AttestationVersion: "hls-produced-media-attestation-v1",
		AttestationHash:    hash,
		SegmentCount:       900,
		Video:              video,
		Audio:              audio,
		FinalAVSkewMicros:  video.EndMicros - audio.EndMicros,
	}
}

func validStream(kind string) StreamEvidence {
	checkpoints := make([]CheckpointEvidence, 0, len(CheckpointTargets()))
	for _, target := range CheckpointTargets() {
		checkpoints = append(checkpoints, CheckpointEvidence{TargetMicros: target, PresentationMicros: target})
	}
	return StreamEvidence{
		Kind:           kind,
		TimeBase:       "1/90000",
		PacketCount:    100,
		StartMicros:    0,
		EndMicros:      DurationMicros,
		DurationMicros: DurationMicros,
		Checkpoints:    checkpoints,
	}
}
