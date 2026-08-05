#!/usr/bin/env python3
import re
from pathlib import Path

root = Path(__file__).resolve().parents[1]

repo_path = root / "internal/repository/repo_legacy_transcode_projection_migration.go"
content = repo_path.read_text(encoding="utf-8")
old_signature = "func (r *TranscodeExecutionRepo) CompleteLegacyProjectionMigrationBatch(source, token string, cursor LegacyProjectionCursor, delta LegacyProjectionBatchDelta, completed bool, now time.Time, retirementWindow time.Duration) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {"
new_signature = "func (r *TranscodeExecutionRepo) CompleteLegacyProjectionMigrationBatch(source, token string, cursor LegacyProjectionCursor, delta LegacyProjectionBatchDelta, completed bool, now time.Time, retirementWindow, sourceCheckInterval time.Duration) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {\n\tif sourceCheckInterval <= 0 {\n\t\tsourceCheckInterval = 15 * time.Minute\n\t}"
if content.count(old_signature) != 1:
    raise RuntimeError(f"complete migration signature anchor count={content.count(old_signature)}")
repo_path.write_text(content.replace(old_signature, new_signature, 1), encoding="utf-8")

test_path = root / "internal/repository/repo_legacy_transcode_projection_migration_test.go"
tests = test_path.read_text(encoding="utf-8")
for function_name in (
    "PrepareLegacyProjectionMigration",
    "CompleteLegacyProjectionMigrationBatch",
):
    pattern = re.compile(
        rf"({function_name}\([^\n]*?30\*24\*time\.Hour)(?:,\s*15\*time\.Minute)?\)"
    )
    tests, count = pattern.subn(r"\1, 15*time.Minute)", tests)
    if count == 0:
        raise RuntimeError(f"no {function_name} test calls were normalized")

old_expectation = "if err != nil || changed || state.Generation != 1 {\n\t\tt.Fatalf(\"same high-water reopened state=%+v changed=%v err=%v\", state, changed, err)\n\t}"
new_expectation = "if err != nil || !changed || state.Generation != 1 {\n\t\tt.Fatalf(\"same high-water source-check scheduling state=%+v changed=%v err=%v\", state, changed, err)\n\t}"
if tests.count(old_expectation) != 1:
    raise RuntimeError(f"same high-water expectation anchor count={tests.count(old_expectation)}")
tests = tests.replace(old_expectation, new_expectation, 1)
test_path.write_text(tests, encoding="utf-8")
