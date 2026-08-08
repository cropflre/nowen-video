package artifactstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestArtifactStoreHealthProbeVerifiesAtomicWrite(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "transcode"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.ProbeWritable(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Writable || result.Root != store.Root() {
		t.Fatalf("unexpected health result: %+v", result)
	}
	entries, err := os.ReadDir(filepath.Join(store.Root(), ".health"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("health probe left files behind: %+v", entries)
	}
}

func TestArtifactStoreHealthProbeDetectsReadOnlyRootAndRecovers(t *testing.T) {
	root := filepath.Join(t.TempDir(), "transcode")
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	_, probeErr := store.ProbeWritable(time.Now())
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if probeErr == nil {
		t.Skip("current user bypasses directory write permissions")
	}
	result, err := store.ProbeWritable(time.Now())
	if err != nil || !result.Writable {
		t.Fatalf("writable permissions did not recover: result=%+v err=%v", result, err)
	}
}

func TestArtifactStoreHealthProbeDetectsDisconnectedRootAndRecovers(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "transcode")
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	moved := root + ".offline"
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root, []byte("mount unavailable"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ProbeWritable(time.Now()); err == nil {
		t.Fatal("disconnected artifact store was reported writable")
	}
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(moved, root); err != nil {
		t.Fatal(err)
	}
	result, err := store.ProbeWritable(time.Now())
	if err != nil || !result.Writable {
		t.Fatalf("restored artifact store did not recover: result=%+v err=%v", result, err)
	}
}
