package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestFullServerOwnsOrderedArtifactMaintenanceShutdown(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read full server main.go: %v", err)
	}
	source := string(mainSource)

	required := []string{
		"signal.Stop(quit)",
		"srv.Shutdown(httpCtx)",
		"services.ArtifactMaintenance.Shutdown(transcodeCtx)",
		"context.WithTimeout(context.Background(), 30*time.Second)",
	}
	for _, token := range required {
		if !strings.Contains(source, token) {
			t.Fatalf("full server shutdown contract missing %q", token)
		}
	}

	httpShutdown := strings.Index(source, "srv.Shutdown(httpCtx)")
	maintenanceShutdown := strings.Index(source, "services.ArtifactMaintenance.Shutdown(transcodeCtx)")
	if httpShutdown < 0 || maintenanceShutdown < 0 || httpShutdown >= maintenanceShutdown {
		t.Fatalf("full server must stop accepting HTTP requests before stopping artifact maintenance")
	}
}

func TestLegacyFullSignalHookIsPhysicallyRemoved(t *testing.T) {
	path := "../../internal/service/transcode_process_shutdown.go"
	_, err := os.Stat(path)
	if err == nil {
		t.Fatalf("legacy transcode process shutdown bridge still exists: %s", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat legacy transcode process shutdown bridge: %v", err)
	}
}
