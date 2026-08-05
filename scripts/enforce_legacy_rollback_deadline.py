#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]


def replace_once(path: str, old: str, new: str) -> None:
    target = root / path
    content = target.read_text(encoding="utf-8")
    if content.count(old) != 1:
        raise RuntimeError(f"{path}: expected one occurrence, got {content.count(old)}")
    target.write_text(content.replace(old, new, 1), encoding="utf-8")


replace_once(
    "internal/service/task_center.go",
    '\tCompletedAt *time.Time `json:"completed_at,omitempty"`\n',
    '\tCompletedAt   *time.Time `json:"completed_at,omitempty"`\n\tRollbackUntil *time.Time `json:"rollback_until,omitempty"`\n',
)
replace_once(
    "internal/service/task_center.go",
    '\t\tStartedAt: artifact.CleanupClaimedAt,\n',
    '\t\tStartedAt:     artifact.CleanupClaimedAt,\n\t\tRollbackUntil: artifact.CleanupRollbackUntil,\n',
)

replace_once(
    "internal/service/task_actions.go",
    '\t"strings"\n',
    '\t"strings"\n\t"time"\n',
)
replace_once(
    "internal/service/task_actions.go",
    '''func (d *TaskActionDispatcher) Execute(kind, sourceID, action, userID string) (*TaskActionResult, error) {
''',
    '''func AvailableTaskActionsForTask(task UnifiedTask, now time.Time) []string {
	actions := AvailableTaskActions(task.Kind, task.Status)
	if task.Kind != TaskKindLegacyArtifactMigration {
		return actions
	}
	if task.RollbackUntil != nil && !now.After(*task.RollbackUntil) {
		return actions
	}
	filtered := make([]string, 0, len(actions))
	for _, action := range actions {
		if action != TaskActionRollback {
			filtered = append(filtered, action)
		}
	}
	return filtered
}

func (d *TaskActionDispatcher) Execute(kind, sourceID, action, userID string) (*TaskActionResult, error) {
''',
)
replace_once(
    "internal/service/task_actions.go",
    '''	if !containsAction(AvailableTaskActions(mapArtifactTaskKind(artifact), mapArtifactTaskStatus(artifact)), action) {
		return fmt.Errorf("%w: artifact cleanup state=%s action=%s", ErrTaskActionConflict, artifact.CleanupState, action)
	}
''',
    '''	task := UnifiedTask{
		Kind:          mapArtifactTaskKind(artifact),
		Status:        mapArtifactTaskStatus(artifact),
		RollbackUntil: artifact.CleanupRollbackUntil,
	}
	if !containsAction(AvailableTaskActionsForTask(task, time.Now()), action) {
		return fmt.Errorf("%w: artifact cleanup state=%s action=%s", ErrTaskActionConflict, artifact.CleanupState, action)
	}
''',
)

replace_once(
    "internal/handler/task_center.go",
    '\t"strconv"\n',
    '\t"strconv"\n\t"time"\n',
)
replace_once(
    "internal/handler/task_center.go",
    '\t\t\tActions:     service.AvailableTaskActions(task.Kind, task.Status),\n',
    '\t\t\tActions:     service.AvailableTaskActionsForTask(task, time.Now()),\n',
)

replace_once(
    "internal/repository/repo_transcode_artifact_cleanup.go",
    '''		Where(
			"id = ? AND migration_source = ? AND cleanup_state IN ?",
			artifactID,
			LegacyTranscodeArtifactMigrationSource,
			[]string{ArtifactCleanupPending, ArtifactCleanupRetryWait, ArtifactCleanupBlocked},
		).
''',
    '''		Where(
			"id = ? AND migration_source = ? AND cleanup_state IN ? AND cleanup_rollback_until IS NOT NULL AND cleanup_rollback_until >= ?",
			artifactID,
			LegacyTranscodeArtifactMigrationSource,
			[]string{ArtifactCleanupPending, ArtifactCleanupRetryWait, ArtifactCleanupBlocked},
			now,
		).
''',
)

replace_once(
    "internal/service/task_actions_test.go",
    '\t"testing"\n',
    '\t"testing"\n\t"time"\n',
)
replace_once(
    "internal/service/task_actions_test.go",
    '''	if got := AvailableTaskActions(TaskKindScan, "running"); len(got) != 0 {
		t.Fatalf("scan actions=%v", got)
	}
}
''',
    '''	if got := AvailableTaskActions(TaskKindScan, "running"); len(got) != 0 {
		t.Fatalf("scan actions=%v", got)
	}
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	if got := AvailableTaskActionsForTask(UnifiedTask{Kind: TaskKindLegacyArtifactMigration, Status: TaskStatusQueued, RollbackUntil: &future}, time.Now()); len(got) != 1 || got[0] != TaskActionRollback {
		t.Fatalf("future migration actions=%v", got)
	}
	if got := AvailableTaskActionsForTask(UnifiedTask{Kind: TaskKindLegacyArtifactMigration, Status: TaskStatusQueued, RollbackUntil: &past}, time.Now()); len(got) != 0 {
		t.Fatalf("expired migration actions=%v", got)
	}
}
''',
)
replace_once(
    "internal/service/task_actions_test.go",
    '''	actions := &fakeMigrationActions{}
	d := &TaskActionDispatcher{
''',
    '''	actions := &fakeMigrationActions{}
	rollbackUntil := time.Now().Add(time.Hour)
	d := &TaskActionDispatcher{
''',
)
replace_once(
    "internal/service/task_actions_test.go",
    '''			MigrationSource: repository.LegacyTranscodeArtifactMigrationSource,
			CleanupState:    repository.ArtifactCleanupPending,
''',
    '''			MigrationSource:      repository.LegacyTranscodeArtifactMigrationSource,
			CleanupState:         repository.ArtifactCleanupPending,
			CleanupRollbackUntil: &rollbackUntil,
''',
)

append_test = '''
func TestTaskActionDispatcherRejectsExpiredLegacyRollback(t *testing.T) {
	actions := &fakeMigrationActions{}
	rollbackUntil := time.Now().Add(-time.Minute)
	d := &TaskActionDispatcher{
		artifactCleanup: actions,
		artifactLookup: &fakeArtifactLookup{artifact: &model.TranscodeArtifactRecord{
			MigrationSource:      repository.LegacyTranscodeArtifactMigrationSource,
			CleanupState:         repository.ArtifactCleanupPending,
			CleanupRollbackUntil: &rollbackUntil,
		}},
	}
	if _, err := d.Execute(TaskKindLegacyArtifactMigration, "a-expired", TaskActionRollback, "admin"); !errors.Is(err, ErrTaskActionConflict) {
		t.Fatalf("expired rollback error=%v", err)
	}
	if actions.rolledBack != "" {
		t.Fatalf("expired rollback was dispatched: %q", actions.rolledBack)
	}
}
'''
path = root / "internal/service/task_actions_test.go"
path.write_text(path.read_text(encoding="utf-8") + append_test, encoding="utf-8")

replace_once(
    "docs/LEGACY_TRANSCODE_PROJECTION_RETIREMENT.md",
    '''Before cleanup is claimed, an administrator can choose **保留目录** in Task
Center. This changes the Artifact to `rollback_completed` and removes it from
the cleanup work set without modifying the directory or resurrecting the old
Runtime executor.

A claimed or completed cleanup cannot be rolled back because filesystem work
may already have started.
''',
    '''During the seven-day observation window, and only before cleanup is claimed,
an administrator can choose **保留目录** in Task Center. This changes the
Artifact to `rollback_completed` and removes it from the cleanup work set
without modifying the directory or resurrecting the old Runtime executor.

The action disappears when the persisted deadline expires. The repository
rechecks the same deadline atomically, so a stale browser cannot extend the
window. A claimed or completed cleanup cannot be rolled back because filesystem
work may already have started.
''',
)

print("legacy rollback deadline enforced")
