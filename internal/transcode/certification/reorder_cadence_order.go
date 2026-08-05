package certification

import (
	"sort"
	"strings"
)

const (
	reorderCadenceKindPrefix   = "candidate_reorder-"
	realMediaCadenceKindPrefix = "candidate_real-"
	realMediaSourceKindPrefix  = "real_media_source_"
)

// orderCadencePointsForEvidence keeps the legacy packet-order behavior for
// existing cadence contracts. Reorder certification uses presentation order:
// packet PTS is sorted while the independent PacketOrderEvidence retains the
// original demux/decode order for DTS and B-frame reorder validation.
//
// Real Media Corpus v1 source files and produced candidates all declare
// deterministic B-frame reordering, so both sides use the same presentation-
// order cadence semantics. Packet-order evidence remains untouched.
func orderCadencePointsForEvidence(
	kind string,
	ptsTicks []int64,
	points []outputCadencePoint,
) ([]int64, []outputCadencePoint) {
	if !strings.HasPrefix(kind, reorderCadenceKindPrefix) &&
		!strings.HasPrefix(kind, realMediaCadenceKindPrefix) &&
		!strings.HasPrefix(kind, realMediaSourceKindPrefix) {
		return ptsTicks, points
	}
	ordered := append([]outputCadencePoint(nil), points...)
	sort.SliceStable(ordered, func(left, right int) bool {
		if ordered[left].Ticks == ordered[right].Ticks {
			return ordered[left].Micros < ordered[right].Micros
		}
		return ordered[left].Ticks < ordered[right].Ticks
	})
	orderedTicks := make([]int64, len(ordered))
	for index, point := range ordered {
		orderedTicks[index] = point.Ticks
	}
	return orderedTicks, ordered
}
