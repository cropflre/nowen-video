package artifactstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttemptWorkspacesAndPublishedArtifactsAreIsolated(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "transcode"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.PrepareWorkspace("job-1", "attempt-1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.PrepareWorkspace("job-1", "attempt-2")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("different attempts shared one workspace")
	}
	published, err := store.PublishedDir("media-1", "1080p", "artifact-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(published, first+string(filepath.Separator)) || published == first {
		t.Fatal("published path is nested inside mutable workspace")
	}
}

func TestValidateAndPublishHLSArtifact(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "transcode"))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := store.PrepareWorkspace("job-1", "attempt-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "seg0000.ts"), []byte("segment"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2.0,\nseg0000.ts\n"
	if err := os.WriteFile(filepath.Join(workspace, manifestName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	validation, err := store.ValidateHLS(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if validation.SegmentCount != 1 || validation.SizeBytes <= 0 {
		t.Fatalf("unexpected validation: %+v", validation)
	}
	target, err := store.PublishedDir("media-1", "720p", "artifact-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Publish(workspace, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, manifestName)); err != nil {
		t.Fatalf("published manifest missing: %v", err)
	}
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatalf("workspace still exists after atomic publish: %v", err)
	}
}

func TestStoreRejectsPathTraversalAndExternalManifestSegments(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "transcode"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.WorkspaceDir("../job", "attempt"); err == nil {
		t.Fatal("path traversal job id was accepted")
	}
	workspace, err := store.PrepareWorkspace("job", "attempt")
	if err != nil {
		t.Fatal(err)
	}
	manifest := "#EXTM3U\n#EXTINF:2.0,\n../outside.ts\n"
	if err := os.WriteFile(filepath.Join(workspace, manifestName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ValidateHLS(workspace); err == nil {
		t.Fatal("manifest traversal was accepted")
	}
}
