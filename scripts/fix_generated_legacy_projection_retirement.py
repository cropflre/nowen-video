#!/usr/bin/env python3
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]


def update(path: str, transform) -> None:
    target = ROOT / path
    content = target.read_text(encoding="utf-8")
    target.write_text(transform(content), encoding="utf-8")


def replace_exact(content: str, old: str, new: str, label: str) -> str:
    count = content.count(old)
    if count != 1:
        raise RuntimeError(f"{label}: expected 1 occurrence, got {count}")
    return content.replace(old, new, 1)


def fix_repo_stats(content: str) -> str:
    duplicate = '''func (r *TranscodeRepo) DB() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.db
}

'''
    content = replace_exact(content, duplicate, "", "duplicate TranscodeRepo.DB")
    content = replace_exact(content, '\t"time"\n', "", "unused repo_stats time import")
    return content


def fix_cleanup_repo_test(content: str) -> str:
    content = replace_exact(
        content,
        'repo.CompleteArtifactCleanupByClaim(artifact.ID, "token-a")',
        'repo.CompleteArtifactCleanupByClaim(artifact.ID, "token-a", "test_cleanup", now.Add(6*time.Minute))',
        "stale cleanup token call",
    )
    content = replace_exact(
        content,
        'repo.CompleteArtifactCleanupByClaim(artifact.ID, "token-b")',
        'repo.CompleteArtifactCleanupByClaim(artifact.ID, "token-b", "test_cleanup", now.Add(6*time.Minute))',
        "current cleanup token call",
    )
    content = replace_exact(
        content,
        'if count != 0 {\n\t\tt.Fatalf("artifact row survived cleanup commit: %d", count)\n\t}',
        '''if count != 1 {
		t.Fatalf("artifact cleanup tombstone missing: %d", count)
	}
	var tombstone model.TranscodeArtifactRecord
	if err := db.First(&tombstone, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if tombstone.CleanupState != ArtifactCleanupCompleted || tombstone.CleanupDisposition != "test_cleanup" {
		t.Fatalf("unexpected cleanup tombstone: %+v", tombstone)
	}''',
        "cleanup row assertion",
    )
    return content


def fix_task_lifecycle(content: str) -> str:
    content, count = re.subn(
        r'\tcase EventTranscodeStarted, EventTranscodeProgress:.*?\t\tupdate.Status = TaskStatusFailed\n',
        '',
        content,
        count=1,
        flags=re.S,
    )
    if count != 1:
        raise RuntimeError("transcode lifecycle event block mismatch")
    content, count = re.subn(
        r'\tcase \*TranscodeProgressData:.*?\t\treturn value.TaskID\n',
        '',
        content,
        count=1,
        flags=re.S,
    )
    if count != 1:
        raise RuntimeError("transcode lifecycle source block mismatch")
    return content


def add_repository_import(content: str, label: str) -> str:
    return replace_exact(
        content,
        '"github.com/nowen-video/nowen-video/internal/model"\n',
        '"github.com/nowen-video/nowen-video/internal/model"\n\t"github.com/nowen-video/nowen-video/internal/repository"\n',
        label,
    )


def fix_transcode_stats(content: str) -> str:
    return replace_exact(
        content,
        '''	counts, _ := s.repo.CountByStatus()
	if counts == nil {
		counts = map[string]int64{}
	}
''',
        '\tcounts := map[string]int64{}\n',
        "legacy transcode status counters",
    )


def fix_artifact_action_fake(content: str) -> str:
    marker = '''func (f *fakeArtifactCleanupActions) RetryArtifactCleanup(id string) error {
	f.retried = id
	return f.err
}
'''
    replacement = marker + '''
func (f *fakeArtifactCleanupActions) RollbackLegacyArtifactMigration(string) error {
	return f.err
}
'''
    content = replace_exact(content, marker, replacement, "artifact cleanup fake rollback")
    return replace_exact(
        content,
        'dispatcher.Execute(TaskKindArtifactCleanup, "artifact-blocked", TaskActionCancel, "admin")',
        'dispatcher.Execute(TaskKindArtifactCleanup, "artifact-blocked", "cancel", "admin")',
        "unsafe cleanup action literal",
    )


def fix_task_center_test(content: str) -> str:
    return replace_exact(
        content,
        '\n\t"github.com/nowen-video/nowen-video/internal/model"',
        '',
        "unused task center model import",
    )


LIFECYCLE_TEST = '''package service

import (
	"encoding/json"
	"testing"

	"go.uber.org/zap"
)

func TestTaskLifecycleUpdateForEvent(t *testing.T) {
	tests := []struct {
		event    string
		data     interface{}
		kind     string
		status   string
		sourceID string
	}{
		{EventScanStarted, &ScanProgressData{LibraryID: "library-1"}, TaskKindScan, TaskStatusRunning, "library-1"},
		{EventScanCompleted, ScanProgressData{LibraryID: "library-2"}, TaskKindScan, TaskStatusCompleted, "library-2"},
		{EventScrapeProgress, &ScrapeProgressData{LibraryID: "library-3"}, TaskKindScrape, TaskStatusRunning, "library-3"},
		{EventScrapeCompleted, ScrapeProgressData{LibraryID: "library-4"}, TaskKindScrape, TaskStatusCompleted, "library-4"},
	}

	for _, tt := range tests {
		update, ok := taskLifecycleUpdateForEvent(tt.event, tt.data)
		if !ok {
			t.Fatalf("event %s was not mapped", tt.event)
		}
		if update.Kind != tt.kind || update.Status != tt.status || update.SourceID != tt.sourceID || update.SourceEvent != tt.event {
			t.Fatalf("event=%s update=%+v", tt.event, update)
		}
	}

	for _, retired := range []string{EventTranscodeStarted, EventTranscodeProgress, EventTranscodeCompleted, EventTranscodeFailed} {
		if update, ok := taskLifecycleUpdateForEvent(retired, &TranscodeProgressData{TaskID: "retired"}); ok || update != nil {
			t.Fatalf("retired Runtime event must not enter Task Center: event=%s update=%+v", retired, update)
		}
	}
	if update, ok := taskLifecycleUpdateForEvent(EventLibraryUpdated, nil); ok || update != nil {
		t.Fatalf("non-task event must not be mapped: %+v", update)
	}
	if update, ok := taskLifecycleUpdateForEvent(EventTaskUpdated, nil); ok || update != nil {
		t.Fatalf("task_updated must not recursively map: %+v", update)
	}
}

func TestBroadcastEventAlsoEmitsTaskUpdated(t *testing.T) {
	hub := NewWSHub(zap.NewNop().Sugar())
	hub.BroadcastEvent(EventScrapeProgress, &ScrapeProgressData{LibraryID: "library-9"})

	var original WSEvent
	if err := json.Unmarshal(<-hub.broadcast, &original); err != nil {
		t.Fatal(err)
	}
	if original.Type != EventScrapeProgress {
		t.Fatalf("unexpected original event: %s", original.Type)
	}

	var unified WSEvent
	if err := json.Unmarshal(<-hub.broadcast, &unified); err != nil {
		t.Fatal(err)
	}
	if unified.Type != EventTaskUpdated {
		t.Fatalf("expected %s, got %s", EventTaskUpdated, unified.Type)
	}

	payload, err := json.Marshal(unified.Data)
	if err != nil {
		t.Fatal(err)
	}
	var update TaskLifecycleUpdate
	if err := json.Unmarshal(payload, &update); err != nil {
		t.Fatal(err)
	}
	if update.Kind != TaskKindScrape || update.SourceID != "library-9" || update.SourceEvent != EventScrapeProgress {
		t.Fatalf("unexpected unified update: %+v", update)
	}
}

func TestBroadcastEventDoesNotDuplicateNonTaskEvents(t *testing.T) {
	hub := NewWSHub(zap.NewNop().Sugar())
	hub.BroadcastEvent(EventLibraryUpdated, &LibraryChangedData{LibraryID: "library-1"})

	select {
	case <-hub.broadcast:
	default:
		t.Fatal("expected original event")
	}
	select {
	case unexpected := <-hub.broadcast:
		t.Fatalf("unexpected duplicate event: %s", unexpected)
	default:
	}
}
'''

update("internal/repository/repo_stats.go", fix_repo_stats)
update("internal/repository/repo_transcode_artifact_cleanup_test.go", fix_cleanup_repo_test)
update("internal/service/task_lifecycle_events.go", fix_task_lifecycle)
update("internal/service/transcode_cleanup_state.go", lambda content: add_repository_import(content, "cleanup state repository import"))
update("internal/service/transcode_runtime_retirement.go", lambda content: add_repository_import(content, "runtime retirement repository import"))
update("internal/service/transcode_stats.go", fix_transcode_stats)
update("internal/service/task_actions_artifact_cleanup_test.go", fix_artifact_action_fake)
update("internal/service/task_center_test.go", fix_task_center_test)
(ROOT / "internal/service/task_lifecycle_events_test.go").write_text(LIFECYCLE_TEST, encoding="utf-8")
