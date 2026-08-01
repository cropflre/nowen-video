package timebasecandidate

import (
	"fmt"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
)

func candidateRunPreservationError(r RunEvidence, sourceStartup, sourceContinuation transcodeoutputcadence.TimelineEvidence) error {
	if !candidateCadencePreserved(sourceStartup, r.StartupTimeline) {
		return fmt.Errorf("startup cadence changed: source={%s} output={%s}", timelineDiagnostic(sourceStartup), timelineDiagnostic(r.StartupTimeline))
	}
	if !candidateCadencePreserved(sourceContinuation, r.ContinuationTimeline) {
		return fmt.Errorf("continuation cadence changed: source={%s} output={%s}", timelineDiagnostic(sourceContinuation), timelineDiagnostic(r.ContinuationTimeline))
	}
	if !mappingAccepted(r.StartupMapping) || !mappingAccepted(r.ContinuationMapping) {
		return fmt.Errorf("frame mapping exceeds rational-boundary tolerance: startup=%+v continuation=%+v", r.StartupMapping, r.ContinuationMapping)
	}
	if r.StartupTimeline.NearZeroDeltaCount != 0 || r.ContinuationTimeline.NearZeroDeltaCount != 0 {
		return fmt.Errorf("near-zero PTS detected: startup=%d continuation=%d", r.StartupTimeline.NearZeroDeltaCount, r.ContinuationTimeline.NearZeroDeltaCount)
	}
	if r.StartupTimeline.DuplicatePTSCount != 0 || r.ContinuationTimeline.DuplicatePTSCount != 0 {
		return fmt.Errorf("duplicate PTS detected: startup=%d continuation=%d", r.StartupTimeline.DuplicatePTSCount, r.ContinuationTimeline.DuplicatePTSCount)
	}
	if r.StartupTimeline.NonMonotonicPTSCount != 0 || r.ContinuationTimeline.NonMonotonicPTSCount != 0 {
		return fmt.Errorf("non-monotonic PTS detected: startup=%d continuation=%d", r.StartupTimeline.NonMonotonicPTSCount, r.ContinuationTimeline.NonMonotonicPTSCount)
	}
	if r.StartupFingerprint.AdjacentDuplicateCount != 0 || r.ContinuationFingerprint.AdjacentDuplicateCount != 0 {
		return fmt.Errorf("adjacent decoded duplicate frames detected: startup=%d continuation=%d", r.StartupFingerprint.AdjacentDuplicateCount, r.ContinuationFingerprint.AdjacentDuplicateCount)
	}
	return nil
}

func timelineDiagnostic(t transcodeoutputcadence.TimelineEvidence) string {
	minimum, maximum, significant := significantCadenceBounds(t)
	return fmt.Sprintf(
		"frames=%d material=%t dominant_us=%d significant=%t significant_min_us=%d significant_max_us=%d near_zero=%d duplicate_pts=%d non_monotonic_pts=%d histogram=%+v",
		t.FrameCount,
		t.MaterialVariableDuration,
		t.DominantDeltaMicros,
		significant,
		minimum,
		maximum,
		t.NearZeroDeltaCount,
		t.DuplicatePTSCount,
		t.NonMonotonicPTSCount,
		t.DeltaHistogram,
	)
}
