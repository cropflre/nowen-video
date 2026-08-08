package certification

import (
	"testing"

	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
)

func TestSourceOriginRegistryIsStable(t *testing.T) {
	cases := AvailableSourceOriginCases()
	expected := []string{
		SourceOriginCaseCFR24Zero,
		SourceOriginCaseCFR25Zero,
		SourceOriginCaseCFR2997Zero,
		SourceOriginCaseVFR2430Zero,
		SourceOriginCaseCFR30Positive5S,
		SourceOriginCaseCFR30Negative2S,
	}
	if len(cases) != len(expected) {
		t.Fatalf("source origin case count = %d, want %d", len(cases), len(expected))
	}
	for index, spec := range cases {
		if spec.ID != expected[index] {
			t.Fatalf("source origin case %d = %s, want %s", index, spec.ID, expected[index])
		}
		if err := spec.Validate(); err != nil {
			t.Fatalf("source origin case %s is invalid: %v", spec.ID, err)
		}
	}
}

func TestSourceOriginRateProjection(t *testing.T) {
	spec, ok := LookupSourceOriginCase(SourceOriginCaseCFR2997Zero)
	if !ok {
		t.Fatal("missing 30000/1001 source origin case")
	}
	if got := spec.DeclaredFrameRateMilli(); got != 29_970 {
		t.Fatalf("frame rate milli = %d, want 29970", got)
	}
}

func TestSourceOriginOffsetExpression(t *testing.T) {
	for _, test := range []struct {
		offset int64
		want   string
	}{
		{0, "PTS"},
		{5_000_000, "PTS+5.00/TB"},
		{-2_000_000, "PTS-2.00/TB"},
	} {
		if got := sourceOriginOffsetExpression(test.offset); got != test.want {
			t.Fatalf("offset %d expression = %q, want %q", test.offset, got, test.want)
		}
	}
}

func TestSourceOriginMatrixRejectsIncompleteCases(t *testing.T) {
	matrix := SourceOriginMatrixReport{SchemaVersion: SourceOriginMatrixSchemaVersion}
	if err := matrix.Validate(); err == nil {
		t.Fatal("incomplete source origin matrix was accepted")
	}
}

func TestSourceOriginClass(t *testing.T) {
	if sourceOriginClass(0) != transcodesourceorigin.OriginZero ||
		sourceOriginClass(1) != transcodesourceorigin.OriginPositive ||
		sourceOriginClass(-1) != transcodesourceorigin.OriginNegative {
		t.Fatal("source origin class mapping is invalid")
	}
}
