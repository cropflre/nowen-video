#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]


def replace_once(path: Path, old: str, new: str, label: str) -> None:
    content = path.read_text(encoding="utf-8")
    if content.count(old) != 1:
        raise RuntimeError(f"{label}: expected 1 occurrence, got {content.count(old)}")
    path.write_text(content.replace(old, new, 1), encoding="utf-8")


actions = root / "internal/service/task_actions.go"
replace_once(
    actions,
    '''	if !containsAction(AvailableTaskActions(mapArtifactTaskKind(artifact), mapArtifactTaskStatus(artifact)), action) {
		return fmt.Errorf("%w: artifact cleanup state=%s action=%s", ErrTaskActionConflict, artifact.CleanupState, action)
	}
	switch action {
''',
    '''	if action != TaskActionRetry && action != TaskActionRollback {
		return fmt.Errorf("%w: artifact cleanup action=%s", ErrTaskActionUnsupported, action)
	}
	if !containsAction(AvailableTaskActions(mapArtifactTaskKind(artifact), mapArtifactTaskStatus(artifact)), action) {
		return fmt.Errorf("%w: artifact cleanup state=%s action=%s", ErrTaskActionConflict, artifact.CleanupState, action)
	}
	switch action {
''',
    "artifact action validation",
)

cleanup_test = root / "internal/service/transcode_cleanup_state_test.go"
replace_once(
    cleanup_test,
    '''	var count int64
	if err := db.Model(&model.TranscodeArtifactRecord{}).Where("id = ?", artifact.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("recovered cleanup did not delete Artifact row: %d", count)
	}
''',
    '''	var tombstone model.TranscodeArtifactRecord
	if err := db.First(&tombstone, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if tombstone.CleanupState != repository.ArtifactCleanupCompleted || tombstone.Path != "" || tombstone.CleanupOriginalPath != dir {
		t.Fatalf("recovered cleanup did not preserve Artifact tombstone: %+v", tombstone)
	}
''',
    "recovered cleanup tombstone assertion",
)

pressure_test = root / "internal/service/transcode_disk_pressure_test.go"
replace_once(
    pressure_test,
    '"github.com/nowen-video/nowen-video/internal/model"\n',
    '"github.com/nowen-video/nowen-video/internal/model"\n\t"github.com/nowen-video/nowen-video/internal/repository"\n',
    "disk pressure repository import",
)
replace_once(
    pressure_test,
    '''	var count int64
	if err := db.Model(&model.TranscodeArtifactRecord{}).Where("id = ?", artifact.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("artifact metadata survived pressure cleanup: %d", count)
	}
''',
    '''	var tombstone model.TranscodeArtifactRecord
	if err := db.First(&tombstone, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if tombstone.CleanupState != repository.ArtifactCleanupCompleted || tombstone.Path != "" || tombstone.CleanupOriginalPath != path {
		t.Fatalf("pressure cleanup did not preserve Artifact tombstone: %+v", tombstone)
	}
''',
    "pressure cleanup tombstone assertion",
)
