package reordercandidate

func PacketOrderEquivalentWithinTicks(left, right PacketOrderEvidence, toleranceTicks int64) bool {
	if toleranceTicks < 0 {
		return false
	}
	if toleranceTicks == 0 {
		return PacketOrderSignature(left) == PacketOrderSignature(right)
	}
	if err := left.Validate(); err != nil {
		return false
	}
	if err := right.Validate(); err != nil {
		return false
	}
	if left.TimeBase != right.TimeBase || left.PacketCount != right.PacketCount ||
		left.ReorderedPacketCount != right.ReorderedPacketCount ||
		left.PTSBeforeDTSCount != right.PTSBeforeDTSCount ||
		left.PTSAfterDTSCount != right.PTSAfterDTSCount ||
		left.PTSEqualDTSCount != right.PTSEqualDTSCount ||
		left.AdjacentPTSInversionCount != right.AdjacentPTSInversionCount ||
		left.DTSNonMonotonicCount != right.DTSNonMonotonicCount ||
		left.DTSDuplicateCount != right.DTSDuplicateCount ||
		left.MaxPresentationReorderDepth != right.MaxPresentationReorderDepth {
		return false
	}
	for _, pair := range [][2]int64{
		{left.FirstPTS, right.FirstPTS},
		{left.FirstDTS, right.FirstDTS},
		{left.LastPTS, right.LastPTS},
		{left.LastDTS, right.LastDTS},
		{left.MinCompositionOffsetTicks, right.MinCompositionOffsetTicks},
		{left.MaxCompositionOffsetTicks, right.MaxCompositionOffsetTicks},
	} {
		if absoluteDifference(pair[0], pair[1]) > toleranceTicks {
			return false
		}
	}
	leftDTS := expandDTSDeltaTicks(left)
	rightDTS := expandDTSDeltaTicks(right)
	if !tickSequencesEquivalent(leftDTS, rightDTS, toleranceTicks) {
		return false
	}
	leftOffsets := expandCompositionOffsetTicks(left)
	rightOffsets := expandCompositionOffsetTicks(right)
	return tickSequencesEquivalent(leftOffsets, rightOffsets, toleranceTicks)
}

func expandDTSDeltaTicks(evidence PacketOrderEvidence) []int64 {
	values := make([]int64, 0, evidence.PacketCount-1)
	for _, bucket := range evidence.DTSDeltaHistogram {
		for count := 0; count < bucket.Count; count++ {
			values = append(values, bucket.DeltaTicks)
		}
	}
	return values
}

func expandCompositionOffsetTicks(evidence PacketOrderEvidence) []int64 {
	values := make([]int64, 0, evidence.PacketCount)
	for _, bucket := range evidence.CompositionOffsetHistogram {
		for count := 0; count < bucket.Count; count++ {
			values = append(values, bucket.OffsetTicks)
		}
	}
	return values
}

func tickSequencesEquivalent(left, right []int64, toleranceTicks int64) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if absoluteDifference(left[index], right[index]) > toleranceTicks {
			return false
		}
	}
	return true
}

func absoluteDifference(left, right int64) int64 {
	if left >= right {
		return left - right
	}
	return right - left
}
