package certification

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryCommandHashNormalizesEphemeralPaths(t *testing.T) {
	left := recoveryCommandHash(
		"/usr/bin/ffmpeg",
		[]string{"-i", "/tmp/a/source.mp4", "/tmp/a/output/stream.m3u8"},
		[]string{"NOWEN_ENOSPC_PATH=/tmp/a/output"},
		"/tmp/a",
		"/tmp/a/source.mp4",
	)
	right := recoveryCommandHash(
		"/usr/bin/ffmpeg",
		[]string{"-i", "/tmp/b/source.mp4", "/tmp/b/output/stream.m3u8"},
		[]string{"NOWEN_ENOSPC_PATH=/tmp/b/output"},
		"/tmp/b",
		"/tmp/b/source.mp4",
	)
	if left != right {
		t.Fatalf("normalized command hashes differ: %s != %s", left, right)
	}
}

func TestInspectPartialHLSCountsOnlyNonEmptySegments(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "stream.m3u8"), []byte("#EXTM3U\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "seg0000.ts"), []byte("segment"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "seg0001.ts"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	segments, manifest := inspectPartialHLS(root)
	if !manifest || segments != 1 {
		t.Fatalf("inspectPartialHLS = (%d, %t), want (1, true)", segments, manifest)
	}
}

func TestStderrMarkersRecognizeENOSPC(t *testing.T) {
	markers := stderrMarkers("av_interleaved_write_frame(): No space left on device")
	if !slicesContains(markers, "ENOSPC") {
		t.Fatalf("ENOSPC marker missing: %#v", markers)
	}
}
