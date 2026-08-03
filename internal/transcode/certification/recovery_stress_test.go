package certification

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	transcoderecovery "github.com/nowen-video/nowen-video/internal/transcode/recoverystress"
)

func TestRecoveryCommandHashNormalizesEphemeralPaths(t *testing.T) {
	left := recoveryCommandHash(
		"/usr/bin/ffmpeg",
		[]string{"-i", "/tmp/a/source.mp4", "/tmp/a/output/stream.m3u8"},
		nil,
		"/tmp/a",
		"/tmp/a/source.mp4",
	)
	right := recoveryCommandHash(
		"/usr/bin/ffmpeg",
		[]string{"-i", "/tmp/b/source.mp4", "/tmp/b/output/stream.m3u8"},
		nil,
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

func TestENOSPCPrefillLeavesBoundedHeadroom(t *testing.T) {
	const capacity = int64(1_000_000)
	prefill, err := enospcPrefillBytes(capacity)
	if err != nil {
		t.Fatal(err)
	}
	const wantHeadroom = int64(64 * 1024)
	if prefill != capacity-wantHeadroom {
		t.Fatalf("prefill = %d, want %d", prefill, capacity-wantHeadroom)
	}
	if _, err := enospcPrefillBytes(wantHeadroom); err == nil {
		t.Fatal("enospcPrefillBytes accepted a capacity without write headroom")
	}
}

func TestResourceLimitHelperSourceCompiles(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("C compiler is unavailable")
	}
	path, err := buildResourceHelper(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("resource helper is not executable: %s", info.Mode())
	}
}

func TestReadMemoryPeak(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.peak")
	if err := os.WriteFile(path, []byte("123456\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	peak, err := readMemoryPeak(path)
	if err != nil {
		t.Fatal(err)
	}
	if peak != 123456 {
		t.Fatalf("memory peak = %d, want 123456", peak)
	}
}

func TestBoundedCommandRejectsNonCanonicalLimits(t *testing.T) {
	_, _, _, err := boundedCommand(t.TempDir(), "/usr/bin/ffmpeg", nil, transcoderecovery.ResourceLimits{
		CPUCount:          2,
		AddressSpaceBytes: 512 * 1024 * 1024,
		MemoryMaxBytes:    512 * 1024 * 1024,
	})
	if err == nil {
		t.Fatal("boundedCommand accepted a non-canonical CPU limit")
	}
}
