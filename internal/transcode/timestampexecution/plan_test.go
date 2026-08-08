package timestampexecution

import (
	"strings"
	"testing"
)

func TestPlanIdentityIsDeterministicAndFailClosed(t *testing.T) {
	plan, err := New(33_333, 64_000)
	if err != nil {
		t.Fatal(err)
	}
	version1, hash1, canonical1, err := Identity(plan)
	if err != nil {
		t.Fatal(err)
	}
	version2, hash2, canonical2, err := Identity(plan)
	if err != nil {
		t.Fatal(err)
	}
	if version1 != SchemaVersion || version1 != version2 || hash1 != hash2 || canonical1 != canonical2 {
		t.Fatal("timestamp execution identity is not deterministic")
	}
	plan.SeamlessAllowed = true
	plan.DiscontinuityRequired = false
	if err := plan.Validate(); err == nil {
		t.Fatal("timestamp execution v2 authorized seamless playback")
	}
}

func TestPlanRejectsRuntimeAndUncertifiedBackendMutation(t *testing.T) {
	plan := Baseline()
	plan.CertificationOnly = false
	if err := plan.Validate(); err == nil {
		t.Fatal("runtime timestamp execution plan was accepted")
	}
	plan = Baseline()
	plan.CertifiedBackends = []string{"qsv"}
	if err := plan.Validate(); err == nil {
		t.Fatal("uncertified backend was accepted")
	}
}

func TestApplyContinuationMergesFiltersWithoutMutatingCaller(t *testing.T) {
	plan, err := New(33_333, 64_000)
	if err != nil {
		t.Fatal(err)
	}
	original := []string{"-y", "-vf", "scale=640:360", "-i", "/media/source.mp4", "/cache/stream.m3u8"}
	args, err := ApplyContinuation(original, plan)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-vf scale=640:360,setpts=PTS+0.033333/TB",
		"-af asetpts=PTS+0.064000/TB",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("timestamp execution command missing %q: %s", want, joined)
		}
	}
	if strings.Join(original, " ") != "-y -vf scale=640:360 -i /media/source.mp4 /cache/stream.m3u8" {
		t.Fatalf("caller arguments were mutated: %v", original)
	}
}

func TestBaselineLeavesCommandUnchanged(t *testing.T) {
	args := []string{"-y", "-i", "/media/source.mp4", "/cache/stream.m3u8"}
	result, err := ApplyContinuation(args, Baseline())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result, " ") != strings.Join(args, " ") {
		t.Fatalf("baseline changed command: %v", result)
	}
}
