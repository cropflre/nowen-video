package main

import (
	"os"
	"strings"
	"testing"
)

func TestLegacyTranscodeProjectionIsReadOnlyAndOptional(t *testing.T) {
	for _, path := range []string{"../../internal/model/model.go", "../../internal/model/migrate_lite.go"} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), "&TranscodeTask{}") {
			t.Fatalf("%s still auto-migrates transcode_tasks", path)
		}
	}
	for _, path := range []string{
		"../../internal/service/task_center.go",
		"../../internal/service/task_actions.go",
		"../../internal/service/transcode_cleanup.go",
		"../../internal/service/transcode_runtime_retirement.go",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), "model.TranscodeTask") {
			t.Fatalf("%s still uses legacy task as runtime state", path)
		}
	}
	if _, err := os.Stat("../../internal/service/task_center_runtime_retirement.go"); !os.IsNotExist(err) {
		t.Fatalf("runtime-nil compatibility constructor still exists: %v", err)
	}
}

func TestArtifactCleanupKeepsAuditTombstones(t *testing.T) {
	content, err := os.ReadFile("../../internal/repository/repo_transcode_artifact_cleanup.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, marker := range []string{"CompleteArtifactCleanupByClaim", "ArtifactCleanupCompleted", "cleanup_original_path", "RollbackLegacyArtifactCleanup"} {
		if !strings.Contains(text, marker) {
			t.Fatalf("cleanup repository missing %q", marker)
		}
	}
	if strings.Contains(text, ").Delete(&model.TranscodeArtifactRecord{})") {
		t.Fatal("cleanup still deletes Artifact audit rows")
	}
}
