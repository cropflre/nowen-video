package timebasecandidate

import (
	"strings"
	"testing"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
	transcodevfrisolation "github.com/nowen-video/nowen-video/internal/transcode/vfrisolation"
)

func TestCandidateSpecsAreStrict(t *testing.T) {
	for _, spec := range []CandidateSpec{
		{ID: CandidateAVTB, Description: "AVTB", EncoderTimeBase: "1/1000000"},
		{ID: Candidate90K, Description: "90 kHz", EncoderTimeBase: "1/90000"},
	} {
		if err := spec.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	bad := CandidateSpec{ID: CandidateAVTB, Description: "drift", EncoderTimeBase: "1/90000"}
	if err := bad.Validate(); err == nil {
		t.Fatal("candidate time-base drift was accepted")
	}
}

func TestCandidateSummaryDetectsSequenceDrift(t *testing.T) {
	runs := syntheticRuns(t)
	summary := BuildCandidateSummary(runs)
	if !summary.Stable || !summary.AllPreserved || !summary.SequenceStable {
		t.Fatalf("unexpected stable summary: %+v", summary)
	}
	runs[2].ContinuationFingerprint.SequenceSHA256 = strings.Repeat("f", 64)
	summary = BuildCandidateSummary(runs)
	if summary.Stable || summary.SequenceStable {
		t.Fatalf("sequence drift was accepted: %+v", summary)
	}
}

func TestCandidateComparisonRequiresCadenceEquality(t *testing.T) {
	runs := syntheticRuns(t)
	left := CandidateEvidence{Spec: CandidateSpec{ID: CandidateAVTB, Description: "AVTB", EncoderTimeBase: "1/1000000"}, Runs: runs}
	left.Summary = BuildCandidateSummary(left.Runs)
	right := CandidateEvidence{Spec: CandidateSpec{ID: Candidate90K, Description: "90 kHz", EncoderTimeBase: "1/90000"}, Runs: append([]RunEvidence(nil), runs...)}
	right.Summary = BuildCandidateSummary(right.Runs)
	comparison := BuildCandidateComparison(left, right)
	if !comparison.Equivalent {
		t.Fatalf("equivalent candidates were rejected: %+v", comparison)
	}
	ticks := make([]int64, 300)
	for index := range ticks {
		ticks[index] = int64(index) * 3_001
	}
	changed, err := transcodeoutputcadence.NewTimelineEvidence(
		TimelineKind("case", Candidate90K, 1, "continuation"),
		"1/90000", 30_000_000, 40_000_000, ticks,
	)
	if err != nil {
		t.Fatal(err)
	}
	right.Runs[0].ContinuationTimeline = changed
	comparison = BuildCandidateComparison(left, right)
	if comparison.Equivalent || comparison.CadenceEquivalent {
		t.Fatalf("cadence drift was accepted: %+v", comparison)
	}
}

func syntheticRuns(t *testing.T) []RunEvidence {
	t.Helper()
	startupSourceTicks := make([]int64, 900)
	for index := range startupSourceTicks {
		startupSourceTicks[index] = int64(index) * 3_000
	}
	continuationSourceTicks := make([]int64, 300)
	for index := range continuationSourceTicks {
		continuationSourceTicks[index] = int64(index) * 3_000
	}
	sourceStartup, err := transcodeoutputcadence.NewTimelineEvidence(
		transcodeoutputcadence.TimelineSourceStartup, "1/90000", 0, 30_000_000, startupSourceTicks,
	)
	if err != nil {
		t.Fatal(err)
	}
	sourceContinuation, err := transcodeoutputcadence.NewTimelineEvidence(
		transcodeoutputcadence.TimelineSourceContinuation, "1/90000", 30_000_000, 40_000_000, continuationSourceTicks,
	)
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("0", 64)
	runs := make([]RunEvidence, 0, RepeatCount)
	for ordinal := 1; ordinal <= RepeatCount; ordinal++ {
		startup, err := transcodeoutputcadence.NewTimelineEvidence(
			TimelineKind("case", CandidateAVTB, ordinal, "startup"), "1/90000", 0, 30_000_000, startupSourceTicks,
		)
		if err != nil {
			t.Fatal(err)
		}
		continuation, err := transcodeoutputcadence.NewTimelineEvidence(
			TimelineKind("case", CandidateAVTB, ordinal, "continuation"), "1/90000", 30_000_000, 40_000_000, continuationSourceTicks,
		)
		if err != nil {
			t.Fatal(err)
		}
		runs = append(runs, RunEvidence{
			Ordinal:              ordinal,
			StartupTimeline:      startup,
			ContinuationTimeline: continuation,
			StartupMapping:       transcodeoutputcadence.NewFrameMapping(sourceStartup.FrameCount, startup.FrameCount),
			ContinuationMapping:  transcodeoutputcadence.NewFrameMapping(sourceContinuation.FrameCount, continuation.FrameCount),
			StartupFingerprint: transcodevfrisolation.FrameFingerprint{
				FrameCount: startup.FrameCount, UniqueFrameCount: startup.FrameCount,
				SequenceSHA256: hash, FirstFrameSHA256: hash, LastFrameSHA256: hash,
			},
			ContinuationFingerprint: transcodevfrisolation.FrameFingerprint{
				FrameCount: continuation.FrameCount, UniqueFrameCount: continuation.FrameCount,
				SequenceSHA256: hash, FirstFrameSHA256: hash, LastFrameSHA256: hash,
			},
		})
	}
	return runs
}
