package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	transcodediskpressure "github.com/nowen-video/nowen-video/internal/transcode/diskpressure"
)

func TestStorageReservationDirectorySizeIgnoresSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "segment-0.ts"), make([]byte, 128), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "segment-1.ts"), make([]byte, 256), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "segment-0.ts"), filepath.Join(root, "duplicate.ts")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	size, err := transcodeDirectorySize(root)
	if err != nil {
		t.Fatal(err)
	}
	if size != 384 {
		t.Fatalf("workspace size counted symlink or missed file: got=%d want=384", size)
	}
}

func TestStorageReservationDiskSampleFailsClosed(t *testing.T) {
	if !diskPressureSampleUnavailable(TranscodeDiskPressureStatus{}) {
		t.Fatal("zero disk sample must fail closed")
	}
	unavailable := TranscodeDiskPressureStatus{Snapshot: transcodediskpressure.Snapshot{
		Level:     transcodediskpressure.LevelCritical,
		Reasons:   []string{"disk_sample_unavailable"},
		SampledAt: time.Now(),
	}}
	if !diskPressureSampleUnavailable(unavailable) {
		t.Fatal("explicit unavailable sample must fail closed")
	}
	available := TranscodeDiskPressureStatus{Snapshot: transcodediskpressure.Snapshot{
		Level:     transcodediskpressure.LevelNormal,
		SampledAt: time.Now(),
	}}
	if diskPressureSampleUnavailable(available) {
		t.Fatal("valid disk sample was rejected")
	}
}
