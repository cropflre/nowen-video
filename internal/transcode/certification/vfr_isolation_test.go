package certification

import "testing"

func TestReplaceOutputOptionUsesLastOutputPolicy(t *testing.T) {
	args := []string{"-y", "-fps_mode", "passthrough", "-f", "hls", "out.m3u8"}
	got := replaceOutputOption(args, "-fps_mode", "vfr")
	if got[2] != "vfr" || args[2] != "passthrough" {
		t.Fatalf("unexpected replacement: got=%v original=%v", got, args)
	}
}

func TestInsertIsolationBeforeOutput(t *testing.T) {
	args := []string{"-y", "-f", "hls", "out.m3u8"}
	got := insertIsolationBeforeOutput(args, "-enc_time_base:v:0", "1/90000")
	want := []string{"-y", "-f", "hls", "-enc_time_base:v:0", "1/90000", "out.m3u8"}
	if len(got) != len(want) {
		t.Fatalf("unexpected length: %v", got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected args at %d: got=%v want=%v", index, got, want)
		}
	}
}

func TestVFRIsolationRegistryHasStableParents(t *testing.T) {
	seen := make(map[string]struct{}, len(vfrIsolationVariantSpecs))
	for index, spec := range vfrIsolationVariantSpecs {
		if spec.ID == "" {
			t.Fatalf("variant %d has no ID", index)
		}
		if _, exists := seen[spec.ID]; exists {
			t.Fatalf("duplicate variant %q", spec.ID)
		}
		if spec.CopyOnly {
			if _, exists := seen[spec.ParentVariantID]; !exists {
				t.Fatalf("copy variant %q precedes parent %q", spec.ID, spec.ParentVariantID)
			}
		}
		seen[spec.ID] = struct{}{}
	}
}
