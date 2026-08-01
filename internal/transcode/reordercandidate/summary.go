package reordercandidate

import transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"

func BuildCandidateSummary(runs []RunEvidence) CandidateSummary {
	baseRuns := make([]transcodetimebase.RunEvidence, 0, len(runs))
	startupReordered := make([]int64, 0, len(runs))
	continuationReordered := make([]int64, 0, len(runs))
	startupDepth := make([]int64, 0, len(runs))
	continuationDepth := make([]int64, 0, len(runs))
	startupOffset := make([]int64, 0, len(runs))
	continuationOffset := make([]int64, 0, len(runs))
	startupSignature := ""
	continuationSignature := ""
	summary := CandidateSummary{
		PacketOrderStable: len(runs) == RepeatCount,
		StrictDTS: len(runs) == RepeatCount,
		ReorderObserved: len(runs) == RepeatCount,
	}
	for _, run := range runs {
		baseRuns = append(baseRuns, run.Base)
		startupReordered = append(startupReordered, int64(run.StartupPacketOrder.ReorderedPacketCount))
		continuationReordered = append(continuationReordered, int64(run.ContinuationPacketOrder.ReorderedPacketCount))
		startupDepth = append(startupDepth, int64(run.StartupPacketOrder.MaxPresentationReorderDepth))
		continuationDepth = append(continuationDepth, int64(run.ContinuationPacketOrder.MaxPresentationReorderDepth))
		startupOffset = append(startupOffset, run.StartupPacketOrder.MaxCompositionOffsetMicros)
		continuationOffset = append(continuationOffset, run.ContinuationPacketOrder.MaxCompositionOffsetMicros)
		startupCurrent := PacketOrderSignature(run.StartupPacketOrder)
		continuationCurrent := PacketOrderSignature(run.ContinuationPacketOrder)
		if startupSignature == "" {
			startupSignature = startupCurrent
			continuationSignature = continuationCurrent
		} else {
			summary.PacketOrderStable = summary.PacketOrderStable && startupCurrent == startupSignature && continuationCurrent == continuationSignature
		}
		summary.StrictDTS = summary.StrictDTS &&
			run.StartupPacketOrder.DTSNonMonotonicCount == 0 && run.StartupPacketOrder.DTSDuplicateCount == 0 &&
			run.ContinuationPacketOrder.DTSNonMonotonicCount == 0 && run.ContinuationPacketOrder.DTSDuplicateCount == 0
		summary.ReorderObserved = summary.ReorderObserved &&
			run.StartupPacketOrder.ReorderedPacketCount > 0 && run.StartupPacketOrder.AdjacentPTSInversionCount > 0 && run.StartupPacketOrder.MaxPresentationReorderDepth > 0 &&
			run.ContinuationPacketOrder.ReorderedPacketCount > 0 && run.ContinuationPacketOrder.AdjacentPTSInversionCount > 0 && run.ContinuationPacketOrder.MaxPresentationReorderDepth > 0
	}
	summary.Base = transcodetimebase.BuildCandidateSummary(baseRuns)
	summary.StartupReorderedPacketCount = metricRange(startupReordered)
	summary.ContinuationReorderedPacketCount = metricRange(continuationReordered)
	summary.StartupMaxReorderDepth = metricRange(startupDepth)
	summary.ContinuationMaxReorderDepth = metricRange(continuationDepth)
	summary.StartupMaxCompositionOffsetMicros = metricRange(startupOffset)
	summary.ContinuationMaxCompositionOffsetMicros = metricRange(continuationOffset)
	summary.Stable = summary.Base.Stable && summary.PacketOrderStable && summary.StrictDTS && summary.ReorderObserved &&
		summary.StartupReorderedPacketCount.Span <= PacketVarianceTolerance &&
		summary.ContinuationReorderedPacketCount.Span <= PacketVarianceTolerance &&
		summary.StartupMaxReorderDepth.Span <= PacketVarianceTolerance &&
		summary.ContinuationMaxReorderDepth.Span <= PacketVarianceTolerance &&
		summary.StartupMaxCompositionOffsetMicros.Span <= transcodetimebase.VarianceToleranceMicros &&
		summary.ContinuationMaxCompositionOffsetMicros.Span <= transcodetimebase.VarianceToleranceMicros
	return summary
}

func BuildCandidateComparison(a, b CandidateEvidence) CandidateComparison {
	leftBase := baseCandidate(a)
	rightBase := baseCandidate(b)
	comparison := CandidateComparison{Base: transcodetimebase.BuildCandidateComparison(leftBase, rightBase)}
	if len(a.Runs) != RepeatCount || len(b.Runs) != RepeatCount {
		return comparison
	}
	comparison.StartupPacketOrderEquivalent = true
	comparison.ContinuationPacketOrderEquivalent = true
	for index := range a.Runs {
		comparison.StartupPacketOrderEquivalent = comparison.StartupPacketOrderEquivalent &&
			PacketOrderSignature(a.Runs[index].StartupPacketOrder) == PacketOrderSignature(b.Runs[index].StartupPacketOrder)
		comparison.ContinuationPacketOrderEquivalent = comparison.ContinuationPacketOrderEquivalent &&
			PacketOrderSignature(a.Runs[index].ContinuationPacketOrder) == PacketOrderSignature(b.Runs[index].ContinuationPacketOrder)
	}
	comparison.Equivalent = comparison.Base.Equivalent && comparison.StartupPacketOrderEquivalent && comparison.ContinuationPacketOrderEquivalent
	return comparison
}

func baseCandidate(candidate CandidateEvidence) transcodetimebase.CandidateEvidence {
	runs := make([]transcodetimebase.RunEvidence, 0, len(candidate.Runs))
	for _, run := range candidate.Runs {
		runs = append(runs, run.Base)
	}
	return transcodetimebase.CandidateEvidence{
		Spec: candidate.Spec,
		Runs: runs,
		Summary: transcodetimebase.BuildCandidateSummary(runs),
	}
}

func metricRange(values []int64) transcodetimebase.MetricRange {
	if len(values) == 0 {
		return transcodetimebase.MetricRange{}
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
	return transcodetimebase.MetricRange{Min: minimum, Max: maximum, Span: maximum - minimum}
}
