#!/usr/bin/env python3
from pathlib import Path

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


update("internal/repository/repo_stats.go", fix_repo_stats)
update("internal/repository/repo_transcode_artifact_cleanup_test.go", fix_cleanup_repo_test)
