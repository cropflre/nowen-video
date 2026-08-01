package main

import (
	"os"
	"strings"
	"testing"
)

func TestFullServerOwnsOrderedTranscodeShutdown(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read full server main.go: %v", err)
	}
	source := string(mainSource)

	required := []string{
		"signal.Stop(quit)",
		"srv.Shutdown(httpCtx)",
		"services.Transcode.Shutdown(transcodeCtx)",
		"context.WithTimeout(context.Background(), 30*time.Second)",
	}
	for _, token := range required {
		if !strings.Contains(source, token) {
			t.Fatalf("full server shutdown contract missing %q", token)
		}
	}

	httpShutdown := strings.Index(source, "srv.Shutdown(httpCtx)")
	transcodeShutdown := strings.Index(source, "services.Transcode.Shutdown(transcodeCtx)")
	if httpShutdown < 0 || transcodeShutdown < 0 || httpShutdown >= transcodeShutdown {
		t.Fatalf("full server must stop accepting HTTP requests before draining transcode jobs")
	}

	if strings.Contains(source, "services.Transcode.FenceForProcessExit") {
		t.Fatal("full server must use graceful Shutdown instead of immediate process-exit fencing")
	}
}

func TestLegacyFullSignalHookRemainsPassive(t *testing.T) {
	hookSource, err := os.ReadFile("../../internal/service/transcode_process_shutdown.go")
	if err != nil {
		t.Fatalf("read transcode process shutdown bridge: %v", err)
	}
	source := string(hookSource)

	for _, forbidden := range []string{
		"signal.Notify",
		"syscall.SIGINT",
		"syscall.SIGTERM",
		"FenceForProcessExit()",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("legacy full signal hook must remain passive: found %q", forbidden)
		}
	}
}
