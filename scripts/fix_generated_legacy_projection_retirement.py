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
    return replace_exact(content, marker, replacement, "artifact cleanup fake rollback")


update("internal/repository/repo_stats.go", fix_repo_stats)
update("internal/repository/repo_transcode_artifact_cleanup_test.go", fix_cleanup_repo_test)
update("internal/service/task_lifecycle_events.go", fix_task_lifecycle)
update("internal/service/transcode_cleanup_state.go", lambda content: add_repository_import(content, "cleanup state repository import"))
update("internal/service/transcode_runtime_retirement.go", lambda content: add_repository_import(content, "runtime retirement repository import"))
update("internal/service/transcode_stats.go", fix_transcode_stats)
update("internal/service/task_actions_artifact_cleanup_test.go", fix_artifact_action_fake)
