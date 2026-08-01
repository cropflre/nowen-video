package timebasecandidate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
	transcodevfrisolation "github.com/nowen-video/nowen-video/internal/transcode/vfrisolation"
)

func BuildCandidateSummary(runs []RunEvidence) CandidateSummary {
	summary := CandidateSummary{RepeatCount: len(runs), AllPreserved: len(runs) == RepeatCount}
	startupFrames := make([]int64, 0, len(runs))
	continuationFrames := make([]int64, 0, len(runs))
	startupDominant := make([]int64, 0, len(runs))
	continuationDominant := make([]int64, 0, len(runs))
	startupNearZero := make([]int64, 0, len(runs))
	continuationNearZero := make([]int64, 0, len(runs))
	startupDuplicatePTS := make([]int64, 0, len(runs))
	continuationDuplicatePTS := make([]int64, 0, len(runs))
	startupAdjacent := make([]int64, 0, len(runs))
	continuationAdjacent := make([]int64, 0, len(runs))
	videoDelta := make([]int64, 0, len(runs))
	audioDelta := make([]int64, 0, len(runs))
	startupSkew := make([]int64, 0, len(runs))
	continuationSkew := make([]int64, 0, len(runs))
	boundarySkew := make([]int64, 0, len(runs))
	skewTransition := make([]int64, 0, len(runs))
	projectionResidual := make([]int64, 0, len(runs))
	startupSequence := ""
	continuationSequence := ""
	startupTimelineSignature := ""
	continuationTimelineSignature := ""
	summary.SequenceStable = len(runs) == RepeatCount
	summary.CadenceStable = len(runs) == RepeatCount
	for _, run := range runs {
		startupFrames = append(startupFrames, int64(run.StartupTimeline.FrameCount))
		continuationFrames = append(continuationFrames, int64(run.ContinuationTimeline.FrameCount))
		startupDominant = append(startupDominant, run.StartupTimeline.DominantDeltaMicros)
		continuationDominant = append(continuationDominant, run.ContinuationTimeline.DominantDeltaMicros)
		startupNearZero = append(startupNearZero, int64(run.StartupTimeline.NearZeroDeltaCount))
		continuationNearZero = append(continuationNearZero, int64(run.ContinuationTimeline.NearZeroDeltaCount))
		startupDuplicatePTS = append(startupDuplicatePTS, int64(run.StartupTimeline.DuplicatePTSCount))
		continuationDuplicatePTS = append(continuationDuplicatePTS, int64(run.ContinuationTimeline.DuplicatePTSCount))
		startupAdjacent = append(startupAdjacent, int64(run.StartupFingerprint.AdjacentDuplicateCount))
		continuationAdjacent = append(continuationAdjacent, int64(run.ContinuationFingerprint.AdjacentDuplicateCount))
		videoDelta = append(videoDelta, run.AVSync.VideoBoundaryDeltaMicros)
		audioDelta = append(audioDelta, run.AVSync.AudioBoundaryDeltaMicros)
		startupSkew = append(startupSkew, run.AVSync.StartupEndSkewMicros)
		continuationSkew = append(continuationSkew, run.AVSync.ContinuationStartSkewMicros)
		boundarySkew = append(boundarySkew, run.AVSync.BoundaryDeltaSkewMicros)
		skewTransition = append(skewTransition, run.AVSync.SkewTransitionMicros)
		projectionResidual = append(projectionResidual, run.AVSync.ProjectionResidualMicros)
		startupSig := timelineSignature(run.StartupTimeline)
		continuationSig := timelineSignature(run.ContinuationTimeline)
		if startupSequence == "" {
			startupSequence = run.StartupFingerprint.SequenceSHA256
			continuationSequence = run.ContinuationFingerprint.SequenceSHA256
			startupTimelineSignature = startupSig
			continuationTimelineSignature = continuationSig
		} else {
			summary.SequenceStable = summary.SequenceStable && run.StartupFingerprint.SequenceSHA256 == startupSequence && run.ContinuationFingerprint.SequenceSHA256 == continuationSequence
			summary.CadenceStable = summary.CadenceStable && startupSig == startupTimelineSignature && continuationSig == continuationTimelineSignature
		}
		if !runPreserved(run, run.StartupTimeline, run.ContinuationTimeline) {
			summary.AllPreserved = false
		}
	}
	summary.StartupFrameCount = metricRange(startupFrames)
	summary.ContinuationFrameCount = metricRange(continuationFrames)
	summary.StartupDominantDeltaMicros = metricRange(startupDominant)
	summary.ContinuationDominantDeltaMicros = metricRange(continuationDominant)
	summary.StartupNearZeroDeltaCount = metricRange(startupNearZero)
	summary.ContinuationNearZeroDeltaCount = metricRange(continuationNearZero)
	summary.StartupDuplicatePTSCount = metricRange(startupDuplicatePTS)
	summary.ContinuationDuplicatePTSCount = metricRange(continuationDuplicatePTS)
	summary.StartupAdjacentDuplicateFrameCount = metricRange(startupAdjacent)
	summary.ContinuationAdjacentDuplicateFrames = metricRange(continuationAdjacent)
	summary.VideoBoundaryDeltaMicros = metricRange(videoDelta)
	summary.AudioBoundaryDeltaMicros = metricRange(audioDelta)
	summary.StartupEndSkewMicros = metricRange(startupSkew)
	summary.ContinuationStartSkewMicros = metricRange(continuationSkew)
	summary.BoundaryDeltaSkewMicros = metricRange(boundarySkew)
	summary.SkewTransitionMicros = metricRange(skewTransition)
	summary.ProjectionResidualMicros = metricRange(projectionResidual)
	summary.AVSyncStable = summary.VideoBoundaryDeltaMicros.Span <= VarianceToleranceMicros &&
		summary.AudioBoundaryDeltaMicros.Span <= VarianceToleranceMicros &&
		summary.StartupEndSkewMicros.Span <= VarianceToleranceMicros &&
		summary.ContinuationStartSkewMicros.Span <= VarianceToleranceMicros &&
		summary.BoundaryDeltaSkewMicros.Span <= VarianceToleranceMicros &&
		summary.SkewTransitionMicros.Span <= VarianceToleranceMicros &&
		summary.ProjectionResidualMicros.Span <= VarianceToleranceMicros
	summary.Stable = summary.RepeatCount == RepeatCount && summary.SequenceStable && summary.CadenceStable && summary.AVSyncStable && summary.AllPreserved
	return summary
}

func BuildCandidateComparison(a, b CandidateEvidence) CandidateComparison {
	result := CandidateComparison{CandidateAID: a.Spec.ID, CandidateBID: b.Spec.ID}
	if len(a.Runs) != len(b.Runs) || len(a.Runs) != RepeatCount {
		return result
	}
	result.StartupSequenceEquivalent = true
	result.ContinuationSequenceEquivalent = true
	result.FrameMappingEquivalent = true
	result.CadenceEquivalent = true
	maxDifference := int64(0)
	for index := range a.Runs {
		left := a.Runs[index]
		right := b.Runs[index]
		result.StartupSequenceEquivalent = result.StartupSequenceEquivalent && left.StartupFingerprint.SequenceSHA256 == right.StartupFingerprint.SequenceSHA256
		result.ContinuationSequenceEquivalent = result.ContinuationSequenceEquivalent && left.ContinuationFingerprint.SequenceSHA256 == right.ContinuationFingerprint.SequenceSHA256
		result.FrameMappingEquivalent = result.FrameMappingEquivalent && left.StartupMapping == right.StartupMapping && left.ContinuationMapping == right.ContinuationMapping
		result.CadenceEquivalent = result.CadenceEquivalent && timelineSignature(left.StartupTimeline) == timelineSignature(right.StartupTimeline) && timelineSignature(left.ContinuationTimeline) == timelineSignature(right.ContinuationTimeline)
		for _, pair := range [][2]int64{
			{left.AVSync.VideoBoundaryDeltaMicros, right.AVSync.VideoBoundaryDeltaMicros},
			{left.AVSync.AudioBoundaryDeltaMicros, right.AVSync.AudioBoundaryDeltaMicros},
			{left.AVSync.StartupEndSkewMicros, right.AVSync.StartupEndSkewMicros},
			{left.AVSync.ContinuationStartSkewMicros, right.AVSync.ContinuationStartSkewMicros},
			{left.AVSync.BoundaryDeltaSkewMicros, right.AVSync.BoundaryDeltaSkewMicros},
			{left.AVSync.SkewTransitionMicros, right.AVSync.SkewTransitionMicros},
			{left.AVSync.ProjectionResidualMicros, right.AVSync.ProjectionResidualMicros},
		} {
			difference := abs64(pair[0] - pair[1])
			if difference > maxDifference {
				maxDifference = difference
			}
		}
	}
	result.MaxAVSyncMetricDifferenceMicros = maxDifference
	result.AVSyncWithinTolerance = maxDifference <= CrossCandidateToleranceMicros
	result.Equivalent = result.StartupSequenceEquivalent && result.ContinuationSequenceEquivalent && result.FrameMappingEquivalent && result.CadenceEquivalent && result.AVSyncWithinTolerance
	return result
}

func runPreserved(r RunEvidence, sourceStartup, sourceContinuation transcodeoutputcadence.TimelineEvidence) bool {
	return transcodevfrisolation.CadenceClassification(sourceStartup, r.StartupTimeline, r.StartupMapping) == transcodevfrisolation.ClassificationPreserved &&
		transcodevfrisolation.CadenceClassification(sourceContinuation, r.ContinuationTimeline, r.ContinuationMapping) == transcodevfrisolation.ClassificationPreserved &&
		r.StartupMapping.Status == transcodeoutputcadence.MappingAligned && r.ContinuationMapping.Status == transcodeoutputcadence.MappingAligned &&
		r.StartupTimeline.NearZeroDeltaCount == 0 && r.ContinuationTimeline.NearZeroDeltaCount == 0 &&
		r.StartupTimeline.DuplicatePTSCount == 0 && r.ContinuationTimeline.DuplicatePTSCount == 0 &&
		r.StartupTimeline.NonMonotonicPTSCount == 0 && r.ContinuationTimeline.NonMonotonicPTSCount == 0 &&
		r.StartupFingerprint.AdjacentDuplicateCount == 0 && r.ContinuationFingerprint.AdjacentDuplicateCount == 0
}

func validateFingerprint(f transcodevfrisolation.FrameFingerprint, frameCount int) error {
	if f.FrameCount != frameCount || f.FrameCount <= 0 || f.UniqueFrameCount <= 0 || f.UniqueFrameCount > f.FrameCount {
		return fmt.Errorf("decoded frame counts are invalid")
	}
	if f.AdjacentDuplicateCount < 0 || f.AdjacentDuplicateCount >= f.FrameCount {
		return fmt.Errorf("decoded adjacent duplicate count is invalid")
	}
	for _, value := range []string{f.SequenceSHA256, f.FirstFrameSHA256, f.LastFrameSHA256} {
		if !isSHA256(value) {
			return fmt.Errorf("decoded frame hash is invalid")
		}
	}
	return nil
}

func metricRange(values []int64) MetricRange {
	if len(values) == 0 {
		return MetricRange{}
	}
	minimum := values[0]
	maximum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	return MetricRange{Min: minimum, Max: maximum, Span: maximum - minimum}
}

func timelineSignature(t transcodeoutputcadence.TimelineEvidence) string {
	t.Kind = ""
	content, _ := json.Marshal(t)
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func abs64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
