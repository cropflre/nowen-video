package main

import (
	"os"
	"strings"
	"testing"
)

func TestPersistentRuntimeExecutionSurfaceRemoved(t *testing.T) {
	requireSource(t, "../../internal/service/transcode.go", "type ArtifactMaintenanceService = TranscodeService")
	requireSource(t, "../../internal/service/transcode.go", "NewArtifactMaintenanceService")
	requireSource(t, "../../internal/service/playback_session.go", "execution   *MediaExecutionService")
	requireSource(t, "../../internal/service/stream.go", "execution   *MediaExecutionService")
	requireSource(t, "../../internal/handler/admin.go", "RetiredRuntimeTranscode")
	requireSource(t, "../../internal/handler/stream.go", "RetiredRuntimeHLS")

	for _, check := range []struct{ path, marker string }{
		{"../../internal/service/transcode.go", "go service.worker"},
		{"../../internal/service/media_execution.go", "playbackCompatibilityAdapter"},
		{"../../internal/service/playback_session.go", "*TranscodeService"},
		{"../../internal/service/stream.go", "s.transcoder"},
		{"../../internal/service/stream.go", "GetMasterPlaylistFiltered"},
		{"../../internal/handler/admin.go", "ListTranscodeTasks"},
		{"../../internal/handler/stream.go", "func (h *StreamHandler) Master"},
	} {
		content, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("read %s: %v", check.path, err)
		}
		if strings.Contains(string(content), check.marker) {
			t.Fatalf("%s still contains retired marker %q", check.path, check.marker)
		}
	}
}

func requireSource(t *testing.T, path, marker string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(content), marker) {
		t.Fatalf("%s is missing architecture marker %q", path, marker)
	}
}
