package corpusgenerator

import (
	"strings"
	"testing"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

func TestClassifyRatesAcceptsMatroskaVFRQuantization(t *testing.T) {
	deltas := []int64{
		42_000, 41_000, 42_000, 41_000,
		33_000, 34_000, 33_000, 34_000,
	}
	want := []transcodecorpus.Rational{
		{Numerator: 24, Denominator: 1},
		{Numerator: 30, Denominator: 1},
	}
	got, err := classifyRates(deltas, transcodecorpus.Rational{Numerator: 1, Denominator: 1_000}, want)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("observed rates = %+v, want %+v", got, want)
	}
}

func TestClassifyRatesRejectsUndeclaredCadence(t *testing.T) {
	_, err := classifyRates(
		[]int64{41_667, 41_667, 20_000},
		transcodecorpus.Rational{Numerator: 1, Denominator: 90_000},
		[]transcodecorpus.Rational{{Numerator: 24, Denominator: 1}},
	)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected undeclared cadence failure, got %v", err)
	}
}

func TestClassifyRatesRequiresEveryDeclaredSegment(t *testing.T) {
	_, err := classifyRates(
		[]int64{41_667, 41_667, 41_667},
		transcodecorpus.Rational{Numerator: 1, Denominator: 90_000},
		[]transcodecorpus.Rational{
			{Numerator: 24, Denominator: 1},
			{Numerator: 30, Denominator: 1},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "not materially observed") {
		t.Fatalf("expected missing segment failure, got %v", err)
	}
}

func TestTicksToMicrosRoundsSignedValues(t *testing.T) {
	timeBase := transcodecorpus.Rational{Numerator: 1, Denominator: 90_000}
	if got := ticksToMicros(3, timeBase); got != 33 {
		t.Fatalf("positive ticks = %d, want 33", got)
	}
	if got := ticksToMicros(-3, timeBase); got != -33 {
		t.Fatalf("negative ticks = %d, want -33", got)
	}
}

func TestNormalizeContainer(t *testing.T) {
	for input, want := range map[string]string{
		"mov,mp4,m4a,3gp,3g2,mj2": transcodecorpus.ContainerMP4,
		"matroska,webm":           transcodecorpus.ContainerMatroska,
		"mpegts":                  transcodecorpus.ContainerMPEGTS,
	} {
		got, err := normalizeContainer(input)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("container %q = %q, want %q", input, got, want)
		}
	}
}
