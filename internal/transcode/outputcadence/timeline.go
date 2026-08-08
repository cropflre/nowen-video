package outputcadence

import (
	"fmt"
	"sort"

	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
)

// NewTimelineEvidence derives one deterministic cadence histogram from packet
// presentation timestamps. Equal and decreasing PTS values are counted
// separately and never folded into the positive-delta histogram.
func NewTimelineEvidence(
	kind,
	timeBase string,
	windowStartMicros,
	windowEndMicros int64,
	ptsTicks []int64,
) (TimelineEvidence, error) {
	if len(ptsTicks) < 2 {
		return TimelineEvidence{}, fmt.Errorf("timeline has %d PTS values, want at least 2", len(ptsTicks))
	}
	firstMicros, err := transcodeboundary.TicksToMicros(ptsTicks[0], timeBase)
	if err != nil {
		return TimelineEvidence{}, err
	}
	lastMicros, err := transcodeboundary.TicksToMicros(ptsTicks[len(ptsTicks)-1], timeBase)
	if err != nil {
		return TimelineEvidence{}, err
	}

	counts := make(map[int64]int, 8)
	duplicatePTS := 0
	nonMonotonicPTS := 0
	for index := 0; index < len(ptsTicks)-1; index++ {
		delta := ptsTicks[index+1] - ptsTicks[index]
		switch {
		case delta == 0:
			duplicatePTS++
		case delta < 0:
			nonMonotonicPTS++
		default:
			counts[delta]++
		}
	}
	if len(counts) == 0 {
		return TimelineEvidence{}, fmt.Errorf("timeline has no positive PTS deltas")
	}

	deltaTicks := make([]int64, 0, len(counts))
	for delta := range counts {
		deltaTicks = append(deltaTicks, delta)
	}
	sort.Slice(deltaTicks, func(left, right int) bool { return deltaTicks[left] < deltaTicks[right] })

	histogram := make([]DeltaBucket, 0, len(deltaTicks))
	dominantTicks := int64(0)
	dominantMicros := int64(0)
	dominantCount := 0
	positiveDeltaCount := 0
	nearZeroCount := 0
	for _, ticks := range deltaTicks {
		micros, err := transcodeboundary.TicksToMicros(ticks, timeBase)
		if err != nil {
			return TimelineEvidence{}, err
		}
		count := counts[ticks]
		histogram = append(histogram, DeltaBucket{DeltaTicks: ticks, DeltaMicros: micros, Count: count})
		positiveDeltaCount += count
		if micros < NearZeroDeltaThresholdMicros {
			nearZeroCount += count
		}
		if count > dominantCount || (count == dominantCount && (dominantTicks == 0 || ticks < dominantTicks)) {
			dominantTicks = ticks
			dominantMicros = micros
			dominantCount = count
		}
	}

	minimum := histogram[0]
	maximum := histogram[len(histogram)-1]
	spread := maximum.DeltaMicros - minimum.DeltaMicros
	threshold := significantBucketMinimumCount(positiveDeltaCount)
	significantCount := 0
	outlierCount := 0
	significantMinMicros := int64(0)
	significantMaxMicros := int64(0)
	for _, bucket := range histogram {
		if bucket.Count >= threshold {
			significantCount++
			if significantCount == 1 {
				significantMinMicros = bucket.DeltaMicros
			}
			significantMaxMicros = bucket.DeltaMicros
		} else {
			outlierCount += bucket.Count
		}
	}
	materialVariable := significantCount >= 2 && significantMaxMicros-significantMinMicros >= transcodesourceorigin.VFRSpreadThresholdMicros

	evidence := TimelineEvidence{
		Kind:                          kind,
		TimeBase:                      timeBase,
		WindowStartMicros:             windowStartMicros,
		WindowEndMicros:               windowEndMicros,
		FrameCount:                    len(ptsTicks),
		FirstPTS:                      ptsTicks[0],
		LastPTS:                       ptsTicks[len(ptsTicks)-1],
		FirstPTSMicros:                firstMicros,
		LastPTSMicros:                 lastMicros,
		MinDeltaTicks:                 minimum.DeltaTicks,
		MaxDeltaTicks:                 maximum.DeltaTicks,
		MinDeltaMicros:                minimum.DeltaMicros,
		MaxDeltaMicros:                maximum.DeltaMicros,
		DurationSpreadMicros:          spread,
		DistinctDeltas:                len(histogram),
		VariableDuration:              spread >= transcodesourceorigin.VFRSpreadThresholdMicros,
		SignificantBucketMinimumCount: threshold,
		SignificantDeltaCount:         significantCount,
		OutlierDeltaCount:             outlierCount,
		NearZeroDeltaCount:            nearZeroCount,
		DominantDeltaTicks:            dominantTicks,
		DominantDeltaMicros:           dominantMicros,
		DominantDeltaCount:            dominantCount,
		MaterialVariableDuration:      materialVariable,
		DeltaHistogram:                histogram,
		DuplicatePTSCount:             duplicatePTS,
		NonMonotonicPTSCount:          nonMonotonicPTS,
	}
	if err := evidence.validate(kind); err != nil {
		return TimelineEvidence{}, err
	}
	return evidence, nil
}
