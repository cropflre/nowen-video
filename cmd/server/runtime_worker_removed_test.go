package main

import (
	"os"
	"strings"
	"testing"
)

func TestPersistentRuntimeExecutionSurfaceRemoved(t *testing.T) {
	requireSource(t, "../../internal/service/artifact_maintenance.go", "type ArtifactMaintenanceService struct")
	requireSource(t, "../../internal/service/artifact_maintenance.go", "NewArtifactMaintenanceService")
	requireSource(t, "../../internal/service/playback_session.go", "execution   *MediaExecutionService")
	requireSource(t, "../../internal/service/stream.go", "execution   *MediaExecutionService")

	for _, removed := range []string{"../../internal/service/transcode.go", "../../internal/service/transcode_queue.go", "../../internal/service/transcode_lease.go", "../../internal/service/transcode_progress.go", "../../internal/service/transcode_throttle.go", "../../internal/service/transcode_persistence.go", "../../internal/service/transcode_process_shutdown.go"} {
		if _, err := os.Stat(removed); !os.IsNotExist(err) {
			t.Fatalf("retired runtime file still exists: %s", removed)
		}
	}

	for _, check := range []struct{ path, marker string }{
		{"../../internal/service/artifact_maintenance.go", "TranscodeService"},
		{"../../internal/service/artifact_maintenance.go", "TranscodeJob"},
		{"../../internal/service/service.go", "Transcode           *ArtifactMaintenanceService"},
		{"../../internal/service/media_execution.go", "playbackCompatibilityAdapter"},
		{"../../internal/service/playback_session.go", "*TranscodeService"},
		{"../../internal/service/stream.go", "s.transcoder"},
		{"../../internal/service/stream.go", "GetMasterPlaylistFiltered"},
		{"../../internal/handler/admin.go", "ListTranscodeTasks"},
		{"../../internal/handler/stream.go", "func (h *StreamHandler) Master"},
		{"../server-lite/routes_core.go", "handlers.Stream.Master"},
		{"../server-lite/routes_core.go", "handlers.Stream.Segment"},
		{"../server-lite/routes_core.go", "handlers.Stream.AudioPlaylist"},
		{"../server-lite/routes_core.go", "handlers.Stream.AudioSegment"},
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
