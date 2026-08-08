package certification

import (
	"strings"
	"testing"

	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
)

func TestEncoderTimeBaseRegistryIsStable(t *testing.T) {
	if len(encoderTimeBaseCaseSpecs) != 12 {
		t.Fatalf("unexpected case count: %d", len(encoderTimeBaseCaseSpecs))
	}
	seen := make(map[string]struct{}, len(encoderTimeBaseCaseSpecs))
	for _, spec := range encoderTimeBaseCaseSpecs {
		if err := spec.Validate(); err != nil {
			t.Fatalf("invalid case %s: %v", spec.ID, err)
		}
		if _, exists := seen[spec.ID]; exists {
			t.Fatalf("duplicate case %s", spec.ID)
		}
		seen[spec.ID] = struct{}{}
	}
	if len(encoderTimeBaseCandidateSpecs) != 2 ||
		encoderTimeBaseCandidateSpecs[0].ID != transcodetimebase.CandidateAVTB ||
		encoderTimeBaseCandidateSpecs[1].ID != transcodetimebase.Candidate90K {
		t.Fatalf("candidate order drifted: %+v", encoderTimeBaseCandidateSpecs)
	}
}

func TestEncoderTimeBaseInputGraphCoversCFRAndVFR(t *testing.T) {
	cfr := encoderTimeBaseInputGraph(encoderTimeBaseCaseSpecs[0])
	if !strings.Contains(cfr, "rate=24000/1001") || strings.Contains(cfr, "concat=n=2") {
		t.Fatalf("unexpected CFR graph: %s", cfr)
	}
	var vfrGraph string
	for _, spec := range encoderTimeBaseCaseSpecs {
		if spec.SourceMode == transcodesourceorigin.ModeVFR && spec.ID == EncoderTimeBaseCaseVFR29975994Zero {
			vfrGraph = encoderTimeBaseInputGraph(spec)
			break
		}
	}
	if !strings.Contains(vfrGraph, "rate=30000/1001") || !strings.Contains(vfrGraph, "rate=60000/1001") || !strings.Contains(vfrGraph, "concat=n=2") {
		t.Fatalf("unexpected VFR graph: %s", vfrGraph)
	}
}

func TestEncoderTimeBaseCandidateCommandInsertion(t *testing.T) {
	args := []string{"-y", "-fps_mode", "passthrough", "-f", "hls", "stream.m3u8"}
	got := insertIsolationBeforeOutput(args, "-enc_time_base:v:0", "1/90000")
	want := []string{"-y", "-fps_mode", "passthrough", "-f", "hls", "-enc_time_base:v:0", "1/90000", "stream.m3u8"}
	if len(got) != len(want) {
		t.Fatalf("unexpected command length: %v", got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected command at %d: got=%v want=%v", index, got, want)
		}
	}
}
