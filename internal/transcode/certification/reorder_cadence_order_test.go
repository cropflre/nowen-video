package certification

import "testing"

func TestOrderCadencePointsForReorderEvidence(t *testing.T) {
	assertPresentationOrder(t, "candidate_reorder-cfr-24-b2-origin-zero-v1_encoder-time-base-avtb-v1_run_01_startup")
}

func TestOrderCadencePointsForRealMediaEvidence(t *testing.T) {
	assertPresentationOrder(t, "candidate_real-mp4-h264-aac-cfr-24000-1001-v1_encoder-time-base-avtb-v1_run_01_startup")
}

func TestOrderCadencePointsForRealMediaSourceEvidence(t *testing.T) {
	assertPresentationOrder(t, "real_media_source_full_real-mp4-h264-aac-cfr-24000-1001-v1")
}

func TestOrderCadencePointsKeepsExistingEvidenceOrder(t *testing.T) {
	points := []outputCadencePoint{{Ticks: 9_000}, {Ticks: 3_000}}
	pts := []int64{9_000, 3_000}
	orderedTicks, orderedPoints := orderCadencePointsForEvidence(
		"candidate-cfr-24-origin-zero_encoder-time-base-avtb-v1_run_01_startup",
		pts,
		points,
	)
	if orderedTicks[0] != 9_000 || orderedPoints[0].Ticks != 9_000 {
		t.Fatal("existing cadence contract order changed")
	}
}

func assertPresentationOrder(t *testing.T, kind string) {
	t.Helper()
	points := []outputCadencePoint{
		{Ticks: 0, Micros: 0},
		{Ticks: 9_000, Micros: 100_000},
		{Ticks: 3_000, Micros: 33_333},
		{Ticks: 6_000, Micros: 66_667},
	}
	pts := []int64{0, 9_000, 3_000, 6_000}
	orderedTicks, orderedPoints := orderCadencePointsForEvidence(kind, pts, points)
	want := []int64{0, 3_000, 6_000, 9_000}
	for index := range want {
		if orderedTicks[index] != want[index] || orderedPoints[index].Ticks != want[index] {
			t.Fatalf("presentation order at %d = ticks %d point %d, want %d", index, orderedTicks[index], orderedPoints[index].Ticks, want[index])
		}
	}
	if pts[1] != 9_000 || points[1].Ticks != 9_000 {
		t.Fatal("presentation ordering mutated packet-order inputs")
	}
}
