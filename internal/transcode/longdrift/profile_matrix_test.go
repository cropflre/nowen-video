package longdrift

import (
	"testing"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

func TestAvailableProfilesBindExpectedCorpusTraits(t *testing.T) {
	profiles := AvailableProfiles()
	if len(profiles) != 5 {
		t.Fatalf("profile count = %d, want 5", len(profiles))
	}
	seen := make(map[string]bool, len(profiles))
	for _, profile := range profiles {
		if seen[profile.ID] {
			t.Fatalf("duplicate profile ID %q", profile.ID)
		}
		seen[profile.ID] = true
		caseSpec, ok := transcodecorpus.LookupCase(profile.SourceCaseID)
		if !ok {
			t.Fatalf("profile %s source %s is missing", profile.ID, profile.SourceCaseID)
		}
		if caseSpec.Source.Container != profile.Container ||
			caseSpec.Source.Video.FrameRateMode != profile.FrameRateMode ||
			caseSpec.Source.Video.BFrames != profile.BFrames ||
			caseSpec.Source.Audio.Codec != profile.AudioCodec ||
			caseSpec.Source.Audio.SampleRate != profile.AudioSampleRate ||
			caseSpec.Source.Timeline.OriginMicros != profile.SourceOriginMicros ||
			caseSpec.Source.Timeline.HasEditList != profile.HasEditList {
			t.Fatalf("profile %s does not match corpus case %+v", profile.ID, caseSpec.Source)
		}
	}
}

func TestProfileMatrixCandidatesAreComparedPerProfile(t *testing.T) {
	leftRuns := []RunEvidence{validRun(1), validRun(2)}
	rightRuns := []RunEvidence{validRun(1), validRun(2)}
	left := CandidateEvidence{ID: "left", EncoderTimeBase: "1/1000000", Runs: leftRuns}
	right := CandidateEvidence{ID: "right", EncoderTimeBase: "1/90000", Runs: rightRuns}
	comparison := BuildCandidateComparison(left, right)
	if !comparison.Equivalent {
		t.Fatalf("equivalent profile candidates were rejected: %+v", comparison)
	}
	right.Runs[1].Audio.EndMicros += CrossCandidateToleranceMicros + 1
	right.Runs[1].Audio.DurationMicros = right.Runs[1].Audio.EndMicros - right.Runs[1].Audio.StartMicros
	right.Runs[1].Audio.EndErrorMicros = right.Runs[1].Audio.DurationMicros - DurationMicros
	right.Runs[1].FinalAVSkewMicros = right.Runs[1].Video.EndMicros - right.Runs[1].Audio.EndMicros
	comparison = BuildCandidateComparison(left, right)
	if comparison.Equivalent {
		t.Fatalf("profile candidate divergence was accepted: %+v", comparison)
	}
}
