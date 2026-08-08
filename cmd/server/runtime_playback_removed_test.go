package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestRetiredRuntimePlaybackImplementationRemainsDeleted(t *testing.T) {
	for _, path := range []string{
		"../../internal/service/stream_startup_bridge.go",
		"../../internal/service/stream_startup_bridge_contract.go",
		"../../internal/service/transcode_handoff_attestation.go",
		"../../internal/service/transcode_startup.go",
		"../../internal/service/transcode_startup_continuation.go",
		"../../internal/service/transcode_process_shutdown.go",
	} {
		_, statErr := os.Stat(path)
		if statErr == nil {
			t.Fatalf("retired runtime playback file still exists: %s", path)
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("stat retired runtime playback file %s: %v", path, statErr)
		}
	}

	assertSourceOmits(t, "../../internal/service/ondemand.go", []string{
		"ExecutionRuntime().Run",
		"KindOnDemand",
		"GetOnDemandOutputDir",
		"-hls_playlist_type",
	})
	assertSourceOmits(t, "../../internal/service/stream_artifacts.go", []string{
		"ResolveHLSOutputDir",
		"importLegacyHLSArtifact",
		"WaitForFirstSegmentForMedia",
		"TouchArtifactAccess",
		"ServeContent",
	})
	assertSourceOmits(t, "../../internal/service/media_probe_warmup.go", []string{
		"runOnProbed(",
		"startupSubmitted.Add",
		"startupSkipped.Add",
		"startupFailed.Add",
	})
}

func assertSourceOmits(t *testing.T, path string, forbidden []string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, marker := range forbidden {
		if strings.Contains(string(content), marker) {
			t.Fatalf("%s reintroduced retired runtime marker %q", path, marker)
		}
	}
}
