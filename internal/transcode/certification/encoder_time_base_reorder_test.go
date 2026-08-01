package certification

import (
	"testing"

	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
)

func TestEncoderTimeBaseReorderRegistry(t *testing.T) {
	cases := AvailableEncoderTimeBaseReorderCases()
	if len(cases) != 6 {
		t.Fatalf("unexpected reorder case count %d", len(cases))
	}
	seen := map[string]bool{}
	for _, spec := range cases {
		if err := spec.Validate(); err != nil {
			t.Fatalf("invalid reorder case %s: %v", spec.Base.ID, err)
		}
		if seen[spec.Base.ID] {
			t.Fatalf("duplicate reorder case %s", spec.Base.ID)
		}
		seen[spec.Base.ID] = true
		if spec.BFrames < 1 || spec.BAdapt != 0 || spec.OpenGOP {
			t.Fatalf("case does not enforce deterministic closed-GOP B-frames: %+v", spec)
		}
	}
	if transcodereorder.RepeatCount != 3 {
		t.Fatalf("unexpected repeat policy %d", transcodereorder.RepeatCount)
	}
}

func TestRemoveReorderOptionPair(t *testing.T) {
	input := []string{"-y", "-tune", VideoTuneZeroLatency, "-c:v", "libx264", "out.m3u8"}
	got := removeReorderOptionPair(input, "-tune", VideoTuneZeroLatency)
	want := []string{"-y", "-c:v", "libx264", "out.m3u8"}
	if len(got) != len(want) {
		t.Fatalf("unexpected argument count: %v", got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("argument %d = %q, want %q", index, got[index], want[index])
		}
	}
}
