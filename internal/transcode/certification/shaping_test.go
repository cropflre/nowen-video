package certification

import (
	"strings"
	"testing"

	timestampexecution "github.com/nowen-video/nowen-video/internal/transcode/timestampexecution"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func TestShapingCasesAreValidUniqueAndRegistered(t *testing.T) {
	cases := AvailableShapingCases()
	if len(cases) != 8 {
		t.Fatalf("shaping case count = %d, want 8", len(cases))
	}
	seen := make(map[string]struct{}, len(cases))
	for _, spec := range cases {
		if err := spec.Validate(); err != nil {
			t.Fatalf("shaping case %s is invalid: %v", spec.ID, err)
		}
		registered, ok := LookupShapingCase(spec.ID)
		if !ok || registered != spec {
			t.Fatalf("shaping case %s is not registry stable", spec.ID)
		}
		if _, exists := seen[spec.ID]; exists {
			t.Fatalf("duplicate shaping case %s", spec.ID)
		}
		seen[spec.ID] = struct{}{}
	}
}

func TestShapingContinuationCommandUsesMicrosecondPTSOffsets(t *testing.T) {
	fixture, _ := LookupFixture(FixtureCFR48KZeroLatency)
	args, err := boundaryHLSArgs(
		"/media/source.mp4",
		"/cache/continuation",
		transcodetimestamp.Default(),
		fixture,
		30_000_000,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := timestampexecution.New(33_333, 64_000)
	if err != nil {
		t.Fatal(err)
	}
	args, err = timestampexecution.ApplyContinuation(args, plan)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-copyts -start_at_zero -ss 30.00",
		"-vf scale=640:360,setpts=PTS+0.033333/TB",
		"-af asetpts=PTS+0.064000/TB",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("shaping command missing %q: %s", want, joined)
		}
	}
}

func TestShapingMatrixRejectsMissingCases(t *testing.T) {
	if _, err := BuildShapingMatrixReport(nil); err == nil {
		t.Fatal("incomplete shaping matrix was accepted")
	}
}
