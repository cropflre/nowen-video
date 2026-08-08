package corpusgenerator

import (
	"slices"
	"strings"
	"testing"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

func TestBuildCommandIsOutputPathIndependent(t *testing.T) {
	for _, caseSpec := range transcodecorpus.DefaultSpec().Cases {
		first, err := BuildCommand(caseSpec, "/tmp/first.media")
		if err != nil {
			t.Fatalf("build %s first command: %v", caseSpec.ID, err)
		}
		second, err := BuildCommand(caseSpec, "/tmp/second.media")
		if err != nil {
			t.Fatalf("build %s second command: %v", caseSpec.ID, err)
		}
		if first.CommandSHA256 != second.CommandSHA256 {
			t.Fatalf("%s command hash depends on output path", caseSpec.ID)
		}
		if first.RelativePath != second.RelativePath || !strings.HasPrefix(first.RelativePath, "assets/") {
			t.Fatalf("%s has invalid relative output %q", caseSpec.ID, first.RelativePath)
		}
		joined := strings.Join(first.Args, " ")
		for _, required := range []string{
			"-threads:v 1",
			"-b_strategy 0",
			"b-adapt=0",
			"b-pyramid=none",
			"open-gop=0",
			"lookahead-threads=1",
			"colorprim=bt709",
			"-fps_mode passthrough",
			"-bitexact",
		} {
			if !strings.Contains(joined, required) {
				t.Fatalf("%s command is missing %q", caseSpec.ID, required)
			}
		}
	}
}

func TestBuildCommandContainerPolicies(t *testing.T) {
	for _, caseSpec := range transcodecorpus.DefaultSpec().Cases {
		plan, err := BuildCommand(caseSpec, "/tmp/output.media")
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(plan.Args, " ")
		switch caseSpec.Source.Container {
		case transcodecorpus.ContainerMP4:
			wantEditList := "-use_editlist 0"
			if caseSpec.Source.Timeline.HasEditList {
				wantEditList = "-use_editlist 1"
			}
			if !strings.Contains(joined, wantEditList) || !strings.Contains(joined, "-movflags +faststart") {
				t.Fatalf("%s MP4 policy is incomplete", caseSpec.ID)
			}
		case transcodecorpus.ContainerMatroska:
			if !strings.Contains(joined, "-write_crc32 0 -f matroska") {
				t.Fatalf("%s Matroska policy is incomplete", caseSpec.ID)
			}
		case transcodecorpus.ContainerMPEGTS:
			if !strings.Contains(joined, "-muxdelay 0 -muxpreload 0") || !strings.Contains(joined, "-f mpegts") {
				t.Fatalf("%s MPEG-TS policy is incomplete", caseSpec.ID)
			}
		}
	}
}

func TestBuildCommandVFRUsesExplicitSegmentConcat(t *testing.T) {
	caseSpec, ok := findCase(transcodecorpus.CaseMKVVFR24To30)
	if !ok {
		t.Fatal("VFR case is missing")
	}
	plan, err := BuildCommand(caseSpec, "/tmp/vfr.mkv")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Args, " ")
	for _, required := range []string{
		"rate=24:duration=20.000000",
		"rate=30:duration=20.000000",
		"concat=n=2:v=1:a=0[v]",
		"-map [v] -map 2:a:0",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("VFR command is missing %q", required)
		}
	}
}

func TestBuildCommandPreservesDeclaredOrigins(t *testing.T) {
	for _, id := range []string{transcodecorpus.CaseMP4CFR30000EditList, transcodecorpus.CaseTSCFR30B3} {
		caseSpec, ok := findCase(id)
		if !ok {
			t.Fatalf("case %s is missing", id)
		}
		plan, err := BuildCommand(caseSpec, "/tmp/output.media")
		if err != nil {
			t.Fatal(err)
		}
		want := formatMicros(caseSpec.Source.Timeline.OriginMicros)
		if !slices.Contains(plan.Args, want) {
			t.Fatalf("case %s does not contain origin %s", id, want)
		}
	}
}

func findCase(id string) (transcodecorpus.CaseSpec, bool) {
	for _, caseSpec := range transcodecorpus.DefaultSpec().Cases {
		if caseSpec.ID == id {
			return caseSpec, true
		}
	}
	return transcodecorpus.CaseSpec{}, false
}
