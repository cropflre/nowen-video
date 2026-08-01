package certification

import (
	"testing"

	transcodeavsync "github.com/nowen-video/nowen-video/internal/transcode/avsync"
)

func TestAVSyncVarianceRegistryIsStable(t *testing.T) {
	cases := AvailableAVSyncVarianceCases()
	if len(cases) != 4 {
		t.Fatalf("A/V sync variance case count = %d, want 4", len(cases))
	}
	for index, spec := range cases {
		if spec.ID != avSyncVarianceCaseIDs[index] {
			t.Fatalf("A/V sync variance case %d = %s, want %s", index, spec.ID, avSyncVarianceCaseIDs[index])
		}
		if err := spec.Validate(); err != nil {
			t.Fatalf("A/V sync variance case %s is invalid: %v", spec.ID, err)
		}
	}
}

func TestAVSyncVarianceSummaryDetectsDrift(t *testing.T) {
	runs := []AVSyncRunReport{
		{AVSync: transcodeavsync.Contract{VideoBoundaryDeltaMicros: 12_000, AudioBoundaryDeltaMicros: 5_333, StartupEndSkewMicros: 20_000, ContinuationStartSkewMicros: 13_333, BoundaryDeltaSkewMicros: -6_667}},
		{AVSync: transcodeavsync.Contract{VideoBoundaryDeltaMicros: 12_000, AudioBoundaryDeltaMicros: 5_333, StartupEndSkewMicros: 20_000, ContinuationStartSkewMicros: 13_333, BoundaryDeltaSkewMicros: -6_667}},
		{AVSync: transcodeavsync.Contract{VideoBoundaryDeltaMicros: 12_002, AudioBoundaryDeltaMicros: 5_333, StartupEndSkewMicros: 20_000, ContinuationStartSkewMicros: 13_333, BoundaryDeltaSkewMicros: -6_667}},
	}
	summary := buildAVSyncVarianceSummary(runs)
	if summary.Stable {
		t.Fatal("A/V sync variance greater than one microsecond was accepted")
	}
	if summary.MaxObservedSpanMicros != 2 || summary.VideoBoundaryDeltaMicros.SpanMicros != 2 {
		t.Fatalf("unexpected variance summary: %+v", summary)
	}
}

func TestAVSyncVarianceComparisonRequiresImprovement(t *testing.T) {
	cases := []AVSyncCaseVarianceReport{
		{Case: mustShapingCase(t, ShapingCase48KBaseline), Summary: summaryWithBoundarySkew(-37_334)},
		{Case: mustShapingCase(t, ShapingCase48KPerStream), Summary: summaryWithBoundarySkew(-6_667)},
		{Case: mustShapingCase(t, ShapingCase44K1Baseline), Summary: summaryWithBoundarySkew(-23_400)},
		{Case: mustShapingCase(t, ShapingCase44K1CommonAACTwo), Summary: summaryWithBoundarySkew(-10_289)},
	}
	comparisons, err := buildAVSyncVarianceComparisons(cases)
	if err != nil {
		t.Fatal(err)
	}
	if len(comparisons) != 2 || comparisons[0].DeltaSkewImprovementMicros <= 0 || comparisons[1].DeltaSkewImprovementMicros <= 0 {
		t.Fatalf("unexpected A/V sync comparisons: %+v", comparisons)
	}
}

func TestAVSyncVarianceMatrixRejectsIncompleteCases(t *testing.T) {
	matrix := AVSyncVarianceMatrixReport{
		SchemaVersion:           AVSyncVarianceMatrixSchemaVersion,
		RepeatCount:             AVSyncVarianceRepeatCount,
		VarianceToleranceMicros: AVSyncVarianceToleranceMicros,
	}
	if err := matrix.Validate(); err == nil {
		t.Fatal("incomplete A/V sync variance matrix was accepted")
	}
}

func mustShapingCase(t *testing.T, id string) ShapingCaseSpec {
	t.Helper()
	spec, ok := LookupShapingCase(id)
	if !ok {
		t.Fatalf("missing shaping case %s", id)
	}
	return spec
}

func summaryWithBoundarySkew(value int64) AVSyncVarianceSummary {
	rangeValue := MetricRange{MinMicros: value, MaxMicros: value}
	return AVSyncVarianceSummary{
		RepeatCount:             AVSyncVarianceRepeatCount,
		BoundaryDeltaSkewMicros: rangeValue,
		Stable:                  true,
	}
}
