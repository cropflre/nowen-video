#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]
path = root / "internal/service/task_actions.go"
content = path.read_text(encoding="utf-8")
old = '''	if !containsAction(AvailableTaskActions(mapArtifactTaskKind(artifact), mapArtifactTaskStatus(artifact)), action) {
		return fmt.Errorf("%w: artifact cleanup state=%s action=%s", ErrTaskActionConflict, artifact.CleanupState, action)
	}
	switch action {
'''
new = '''	if action != TaskActionRetry && action != TaskActionRollback {
		return fmt.Errorf("%w: artifact cleanup action=%s", ErrTaskActionUnsupported, action)
	}
	if !containsAction(AvailableTaskActions(mapArtifactTaskKind(artifact), mapArtifactTaskStatus(artifact)), action) {
		return fmt.Errorf("%w: artifact cleanup state=%s action=%s", ErrTaskActionConflict, artifact.CleanupState, action)
	}
	switch action {
'''
if content.count(old) != 1:
    raise RuntimeError(f"artifact action validation anchor mismatch: {content.count(old)}")
path.write_text(content.replace(old, new, 1), encoding="utf-8")
