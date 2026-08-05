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
test_path.write_text(tests.replace(old_expectation, new_expectation, 1), encoding="utf-8")

service_path = root / "internal/service/legacy_transcode_projection_migration.go"
service = service_path.read_text(encoding="utf-8")
old_size_block = '''\t\tartifact.SizeBytes, err = directorySizeWithCheckpoint(outputDir, func() error {
\t\t\tif checkpoint == nil {
\t\t\t\treturn nil
\t\t\t}
\t\t\treturn checkpoint(false)
\t\t})
\t\tif err != nil {
\t\t\treturn report, fmt.Errorf("inventory legacy directory size: %w", err)
\t\t}
'''
new_size_block = '''\t\tsizeBytes, sizeErr := directorySizeWithCheckpoint(outputDir, func() error {
\t\t\tif checkpoint == nil {
\t\t\t\treturn nil
\t\t\t}
\t\t\treturn checkpoint(false)
\t\t})
\t\tif sizeErr != nil {
\t\t\treturn report, fmt.Errorf("inventory legacy directory size: %w", sizeErr)
\t\t}
\t\tartifact.SizeBytes = sizeBytes
'''
if service.count(old_size_block) != 1:
    raise RuntimeError(f"directory size block anchor count={service.count(old_size_block)}")
service_path.write_text(service.replace(old_size_block, new_size_block, 1), encoding="utf-8")

history_path = root / "internal/service/runtime_history.go"
history = history_path.read_text(encoding="utf-8")
if "ConsecutiveFailures int" not in history:
    old = '\tFailureCount       int        `json:"failure_count"`\n\tLastErrorCode      string     `json:"last_error_code,omitempty"`'
    new = '\tFailureCount        int        `json:"failure_count"`\n\tConsecutiveFailures int        `json:"consecutive_failures"`\n\tLastErrorCode       string     `json:"last_error_code,omitempty"`'
    if history.count(old) != 1:
        raise RuntimeError(f"Runtime History consecutive failures anchor count={history.count(old)}")
    history = history.replace(old, new, 1)
if "NextSourceCheckAt" not in history.split("type RuntimeHistorySummary", 1)[0]:
    old = '\tSourceRetireAfter  *time.Time `json:"source_retire_after,omitempty"`\n\tRetirementEligible bool       `json:"retirement_eligible"`'
    new = '\tSourceRetireAfter   *time.Time `json:"source_retire_after,omitempty"`\n\tNextSourceCheckAt  *time.Time `json:"next_source_check_at,omitempty"`\n\tRetirementEligible bool       `json:"retirement_eligible"`'
    if history.count(old) != 1:
        raise RuntimeError(f"Runtime History source check anchor count={history.count(old)}")
    history = history.replace(old, new, 1)
history_path.write_text(history, encoding="utf-8")

web_path = root / "web/src/api/runtimeHistory.ts"
web = web_path.read_text(encoding="utf-8")
if "consecutive_failures: number" not in web:
    web = web.replace("  failure_count: number\n", "  failure_count: number\n  consecutive_failures: number\n", 1)
if "next_source_check_at?: string" not in web:
    web = web.replace("  source_retire_after?: string\n", "  source_retire_after?: string\n  next_source_check_at?: string\n", 1)
web_path.write_text(web, encoding="utf-8")
