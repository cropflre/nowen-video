#!/usr/bin/env python3
from pathlib import Path

path = Path(__file__).with_name("harden_legacy_projection_cursor.py")
content = path.read_text(encoding="utf-8")

old_signature_patch = '''content = content.replace(
'''\\tcompleted bool, now time.Time, retirementWindow time.Duration) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {''',
'''\\tcompleted bool, now time.Time, retirementWindow, sourceCheckInterval time.Duration) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {
\\tif sourceCheckInterval <= 0 {
\\t\\tsourceCheckInterval = 15 * time.Minute
\\t}''',
1,
)
'''
new_signature_patch = '''complete_signature_old = '''func (r *TranscodeExecutionRepo) CompleteLegacyProjectionMigrationBatch(source, token string, cursor LegacyProjectionCursor, delta LegacyProjectionBatchDelta, completed bool, now time.Time, retirementWindow time.Duration) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {'''
complete_signature_new = '''func (r *TranscodeExecutionRepo) CompleteLegacyProjectionMigrationBatch(source, token string, cursor LegacyProjectionCursor, delta LegacyProjectionBatchDelta, completed bool, now time.Time, retirementWindow, sourceCheckInterval time.Duration) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {
\\tif sourceCheckInterval <= 0 {
\\t\\tsourceCheckInterval = 15 * time.Minute
\\t}'''
if content.count(complete_signature_old) != 1:
    raise RuntimeError("complete migration signature anchor mismatch")
content = content.replace(complete_signature_old, complete_signature_new, 1)
'''
if content.count(old_signature_patch) != 1:
    raise RuntimeError("hardening signature patch block mismatch")
content = content.replace(old_signature_patch, new_signature_patch, 1)

old_test_patch = '''content = content.replace(', now, 30*24*time.Hour)', ', now, 30*24*time.Hour, 15*time.Minute)', 2)
content = content.replace(', true, now, 30*24*time.Hour)', ', true, now, 30*24*time.Hour, 15*time.Minute)', 1)
'''
new_test_patch = '''content = content.replace("30*24*time.Hour)", "30*24*time.Hour, 15*time.Minute)")
'''
if content.count(old_test_patch) != 1:
    raise RuntimeError("repository test parameter patch block mismatch")
content = content.replace(old_test_patch, new_test_patch, 1)

path.write_text(content, encoding="utf-8")
