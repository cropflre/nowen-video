package timebasecandidate

import (
	"testing"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
)

func TestMappingAcceptanceUsesDeclaredBoundaryTolerance(t *testing.T) {
	aligned := transcodeoutputcadence.NewFrameMapping(600, 600)
	within := transcodeoutputcadence.NewFrameMapping(599, 600)
	outside := transcodeoutputcadence.NewFrameMapping(598, 600)
	if !mappingAccepted(aligned) {
		t.Fatal("aligned mapping was rejected")
	}
	if !mappingAccepted(within) || within.Status != transcodeoutputcadence.MappingWithinTolerance {
		t.Fatalf("one-frame rational boundary tolerance was rejected: %+v", within)
	}
	if mappingAccepted(outside) {
		t.Fatalf("mapping beyond boundary tolerance was accepted: %+v", outside)
	}
}

func TestCandidateSummaryRecordsBoundaryToleranceUse(t *testing.T) {
	runs := syntheticRuns(t)
	for index := range runs {
		runs[index].ContinuationMapping = transcodeoutputcadence.NewFrameMapping(
			runs[index].ContinuationMapping.InputFrames-1,
			runs[index].ContinuationMapping.OutputFrames,
		)
	}
	summary := BuildCandidateSummary(runs)
	if !summary.Stable || !summary.AllPreserved {
		t.Fatalf("stable one-frame boundary tolerance was rejected: %+v", summary)
	}
	if !summary.BoundaryFrameToleranceUsed || summary.MaximumAbsoluteFrameCountDelta != 1 {
		t.Fatalf("boundary tolerance use was not recorded: %+v", summary)
	}
}
