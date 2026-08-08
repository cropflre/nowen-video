package reordercandidate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
)

func NewPacketOrderEvidence(kind, timeBase string, packets []PacketTimestamp) (PacketOrderEvidence, error) {
	if kind == "" || timeBase == "" || len(packets) < 3 {
		return PacketOrderEvidence{}, fmt.Errorf("packet-order input is incomplete")
	}
	evidence := PacketOrderEvidence{
		Kind:        kind,
		TimeBase:    timeBase,
		PacketCount: len(packets),
		FirstPTS:    packets[0].PTS,
		FirstDTS:    packets[0].DTS,
		LastPTS:     packets[len(packets)-1].PTS,
		LastDTS:     packets[len(packets)-1].DTS,
	}
	var err error
	if evidence.FirstPTSMicros, err = transcodeboundary.TicksToMicros(evidence.FirstPTS, timeBase); err != nil {
		return PacketOrderEvidence{}, err
	}
	if evidence.FirstDTSMicros, err = transcodeboundary.TicksToMicros(evidence.FirstDTS, timeBase); err != nil {
		return PacketOrderEvidence{}, err
	}
	if evidence.LastPTSMicros, err = transcodeboundary.TicksToMicros(evidence.LastPTS, timeBase); err != nil {
		return PacketOrderEvidence{}, err
	}
	if evidence.LastDTSMicros, err = transcodeboundary.TicksToMicros(evidence.LastDTS, timeBase); err != nil {
		return PacketOrderEvidence{}, err
	}

	dtsDeltas := make(map[int64]int)
	offsets := make(map[int64]int)
	for index, packet := range packets {
		offset := packet.PTS - packet.DTS
		offsets[offset]++
		if index == 0 || offset < evidence.MinCompositionOffsetTicks {
			evidence.MinCompositionOffsetTicks = offset
		}
		if index == 0 || offset > evidence.MaxCompositionOffsetTicks {
			evidence.MaxCompositionOffsetTicks = offset
		}
		switch {
		case offset < 0:
			evidence.PTSBeforeDTSCount++
			evidence.ReorderedPacketCount++
		case offset > 0:
			evidence.PTSAfterDTSCount++
			evidence.ReorderedPacketCount++
		default:
			evidence.PTSEqualDTSCount++
		}
		if index > 0 {
			if packet.PTS < packets[index-1].PTS {
				evidence.AdjacentPTSInversionCount++
			}
			delta := packet.DTS - packets[index-1].DTS
			switch {
			case delta < 0:
				evidence.DTSNonMonotonicCount++
			case delta == 0:
				evidence.DTSDuplicateCount++
			default:
				dtsDeltas[delta]++
			}
		}
	}
	if evidence.MinCompositionOffsetMicros, err = transcodeboundary.TicksToMicros(evidence.MinCompositionOffsetTicks, timeBase); err != nil {
		return PacketOrderEvidence{}, err
	}
	if evidence.MaxCompositionOffsetMicros, err = transcodeboundary.TicksToMicros(evidence.MaxCompositionOffsetTicks, timeBase); err != nil {
		return PacketOrderEvidence{}, err
	}

	deltaKeys := sortedInt64Keys(dtsDeltas)
	for _, delta := range deltaKeys {
		micros, err := transcodeboundary.TicksToMicros(delta, timeBase)
		if err != nil {
			return PacketOrderEvidence{}, err
		}
		evidence.DTSDeltaHistogram = append(evidence.DTSDeltaHistogram, transcodeoutputcadence.DeltaBucket{
			DeltaTicks: delta, DeltaMicros: micros, Count: dtsDeltas[delta],
		})
	}
	offsetKeys := sortedInt64Keys(offsets)
	for _, offset := range offsetKeys {
		micros, err := transcodeboundary.TicksToMicros(offset, timeBase)
		if err != nil {
			return PacketOrderEvidence{}, err
		}
		evidence.CompositionOffsetHistogram = append(evidence.CompositionOffsetHistogram, OffsetBucket{
			OffsetTicks: offset, OffsetMicros: micros, Count: offsets[offset],
		})
	}

	presentationOrder := make([]int, len(packets))
	for index := range presentationOrder {
		presentationOrder[index] = index
	}
	sort.SliceStable(presentationOrder, func(left, right int) bool {
		if packets[presentationOrder[left]].PTS == packets[presentationOrder[right]].PTS {
			return packets[presentationOrder[left]].DTS < packets[presentationOrder[right]].DTS
		}
		return packets[presentationOrder[left]].PTS < packets[presentationOrder[right]].PTS
	})
	presentationRank := make([]int, len(packets))
	for rank, decodeIndex := range presentationOrder {
		presentationRank[decodeIndex] = rank
	}
	for decodeIndex, rank := range presentationRank {
		distance := rank - decodeIndex
		if distance < 0 {
			distance = -distance
		}
		if distance > evidence.MaxPresentationReorderDepth {
			evidence.MaxPresentationReorderDepth = distance
		}
	}
	if err := evidence.Validate(); err != nil {
		return PacketOrderEvidence{}, err
	}
	return evidence, nil
}

func (e PacketOrderEvidence) Validate() error {
	if e.Kind == "" || e.TimeBase == "" || e.PacketCount < 3 {
		return fmt.Errorf("packet-order identity is incomplete")
	}
	for ticks, micros := range map[int64]int64{
		e.FirstPTS:                  e.FirstPTSMicros,
		e.FirstDTS:                  e.FirstDTSMicros,
		e.LastPTS:                   e.LastPTSMicros,
		e.LastDTS:                   e.LastDTSMicros,
		e.MinCompositionOffsetTicks: e.MinCompositionOffsetMicros,
		e.MaxCompositionOffsetTicks: e.MaxCompositionOffsetMicros,
	} {
		projected, err := transcodeboundary.TicksToMicros(ticks, e.TimeBase)
		if err != nil || projected != micros {
			return fmt.Errorf("packet-order time projection is inconsistent")
		}
	}
	if e.MinCompositionOffsetTicks > e.MaxCompositionOffsetTicks || e.MinCompositionOffsetMicros > e.MaxCompositionOffsetMicros {
		return fmt.Errorf("composition-offset bounds are invalid")
	}
	if e.ReorderedPacketCount != e.PTSBeforeDTSCount+e.PTSAfterDTSCount ||
		e.ReorderedPacketCount+e.PTSEqualDTSCount != e.PacketCount {
		return fmt.Errorf("composition-offset counters are inconsistent")
	}
	if e.AdjacentPTSInversionCount < 0 || e.AdjacentPTSInversionCount >= e.PacketCount ||
		e.DTSNonMonotonicCount < 0 || e.DTSDuplicateCount < 0 || e.MaxPresentationReorderDepth < 0 {
		return fmt.Errorf("packet-order counters are invalid")
	}
	positiveDTS := e.PacketCount - 1 - e.DTSNonMonotonicCount - e.DTSDuplicateCount
	if positiveDTS <= 0 {
		return fmt.Errorf("packet-order evidence has no positive DTS cadence")
	}
	deltaTotal := 0
	previousDelta := int64(0)
	for index, bucket := range e.DTSDeltaHistogram {
		if bucket.DeltaTicks <= 0 || bucket.Count <= 0 || (index > 0 && bucket.DeltaTicks <= previousDelta) {
			return fmt.Errorf("DTS delta histogram is invalid")
		}
		micros, err := transcodeboundary.TicksToMicros(bucket.DeltaTicks, e.TimeBase)
		if err != nil || micros != bucket.DeltaMicros {
			return fmt.Errorf("DTS delta projection is inconsistent")
		}
		deltaTotal += bucket.Count
		previousDelta = bucket.DeltaTicks
	}
	if deltaTotal != positiveDTS {
		return fmt.Errorf("DTS delta count is inconsistent")
	}
	offsetTotal := 0
	previousOffset := int64(0)
	for index, bucket := range e.CompositionOffsetHistogram {
		if bucket.Count <= 0 || (index > 0 && bucket.OffsetTicks <= previousOffset) {
			return fmt.Errorf("composition-offset histogram is invalid")
		}
		micros, err := transcodeboundary.TicksToMicros(bucket.OffsetTicks, e.TimeBase)
		if err != nil || micros != bucket.OffsetMicros {
			return fmt.Errorf("composition-offset projection is inconsistent")
		}
		offsetTotal += bucket.Count
		previousOffset = bucket.OffsetTicks
	}
	if offsetTotal != e.PacketCount || len(e.CompositionOffsetHistogram) == 0 ||
		e.CompositionOffsetHistogram[0].OffsetTicks != e.MinCompositionOffsetTicks ||
		e.CompositionOffsetHistogram[len(e.CompositionOffsetHistogram)-1].OffsetTicks != e.MaxCompositionOffsetTicks {
		return fmt.Errorf("composition-offset histogram summary is inconsistent")
	}
	return nil
}

func PacketOrderSignature(e PacketOrderEvidence) string {
	e.Kind = ""
	content, _ := json.Marshal(e)
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func sortedInt64Keys(values map[int64]int) []int64 {
	keys := make([]int64, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
