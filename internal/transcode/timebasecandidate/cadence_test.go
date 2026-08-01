package timebasecandidate

import (
	"testing"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
)

func TestCandidateCadencePreservationIgnoresMaterialDominantTieBias(t *testing.T) {
	sourceTicks := timelineTicks([]deltaRun{{Delta: 33_333, Count: 51}, {Delta: 16_667, Count: 49}})
	outputTicks := timelineTicks([]deltaRun{{Delta: 33_333, Count: 49}, {Delta: 16_667, Count: 51}})
	source, err := transcodeoutputcadence.NewTimelineEvidence("source", "1/1000000", 0, 30_000_000, sourceTicks)
	if err != nil {
		t.Fatal(err)
	}
	output, err := transcodeoutputcadence.NewTimelineEvidence("output", "1/1000000", 0, 30_000_000, outputTicks)
	if err != nil {
		t.Fatal(err)
	}
	if source.DominantDeltaMicros == output.DominantDeltaMicros {
		t.Fatal("test fixture did not create dominant-bucket drift")
	}
	if !source.MaterialVariableDuration || !output.MaterialVariableDuration {
		t.Fatal("test fixture is not materially variable")
	}
	if !candidateCadencePreserved(source, output) {
		t.Fatalf("equivalent material cadence bounds were rejected: source=%+v output=%+v", source.DeltaHistogram, output.DeltaHistogram)
	}
}

type deltaRun struct {
	Delta int64
	Count int
}

func timelineTicks(runs []deltaRun) []int64 {
	ticks := []int64{0}
	current := int64(0)
	for _, run := range runs {
		for index := 0; index < run.Count; index++ {
			current += run.Delta
			ticks = append(ticks, current)
		}
	}
	return ticks
}
