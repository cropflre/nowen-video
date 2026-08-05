package main

import (
	"os"
	"strings"
	"testing"
)

func TestPersistentRuntimeWorkerRemainsRetired(t *testing.T) {
	assertRuntimeSourceContains(t, "../../internal/service/transcode_queue.go", []string{
		"runtimeRetired bool",
		"runtimeRetired: true",
		"q.runtimeRetired || q.IsClosed()",
	})
	assertRuntimeSourceContains(t, "../../internal/service/media_execution.go", []string{
		"type MediaExecutionService struct",
		"playbackCompatibilityAdapter",
		"NewPlaybackSessionServiceWithExecution",
		"executionRuntime: s.executionRuntime",
	})
	assertRuntimeSourceContains(t, "../server-lite/main.go", []string{
		"NewMediaExecutionService",
		"NewPlaybackSessionServiceWithExecution",
	})
	assertRuntimeSourceContains(t, "playback_sessions.go", []string{
		"NewMediaExecutionService",
		"NewPlaybackSessionServiceWithExecution",
	})
	assertRuntimeSourceContains(t, "../../internal/service/task_center_runtime_retirement.go", []string{
		"service.transcodeRepo = nil",
		"dispatcher.transcode = nil",
	})
}

func assertRuntimeSourceContains(t *testing.T, path string, required []string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, marker := range required {
		if !strings.Contains(string(content), marker) {
			t.Fatalf("%s is missing architecture marker %q", path, marker)
		}
	}
}
