#!/usr/bin/env python3
from pathlib import Path

path = Path(__file__).with_name("implement_legacy_projection_cursor.py")
content = path.read_text(encoding="utf-8")

# Keep migration lookup separate from Artifact cleanup lookup.
content = content.replace(
'''type legacyProjectionActions interface {
\tRetryLegacyProjectionMigration(source string) error
}
''',
'''type legacyProjectionActions interface {
\tRetryLegacyProjectionMigration(source string) error
}

type legacyProjectionLookup interface {
\tLegacyProjectionMigrationState(source string) (*model.LegacyTranscodeProjectionMigrationState, error)
}
''',
1,
)
content = content.replace(
"content = content.replace('\\tartifactCleanup artifactCleanupActions\\n', '\\tartifactCleanup artifactCleanupActions\\n\\tlegacyProjection legacyProjectionActions\\n', 1)",
"content = content.replace('\\tartifactCleanup artifactCleanupActions\\n', '\\tartifactCleanup artifactCleanupActions\\n\\tlegacyProjection legacyProjectionActions\\n\\tlegacyProjectionLookup legacyProjectionLookup\\n', 1)",
1,
)
content = content.replace(
'''\t\tdispatcher.artifactCleanup = maintenance
\t\tdispatcher.legacyProjection = maintenance
\t\tdispatcher.artifactLookup = maintenance.executionRepo
''',
'''\t\tdispatcher.artifactCleanup = maintenance
\t\tdispatcher.legacyProjection = maintenance
\t\tdispatcher.legacyProjectionLookup = maintenance.executionRepo
\t\tdispatcher.artifactLookup = maintenance.executionRepo
''',
1,
)
content = content.replace(
'''\tif d.legacyProjection == nil || d.artifactLookup == nil {
\t\treturn fmt.Errorf("Legacy Projection 迁移执行器不可用")
\t}
\tstate, err := d.artifactLookup.(*repository.TranscodeExecutionRepo).LegacyProjectionMigrationState(sourceID)
''',
'''\tif d.legacyProjection == nil || d.legacyProjectionLookup == nil {
\t\treturn fmt.Errorf("Legacy Projection 迁移执行器不可用")
\t}
\tstate, err := d.legacyProjectionLookup.LegacyProjectionMigrationState(sourceID)
''',
1,
)
content = content.replace(
'''# Avoid unsafe type assertion by extending lookup interface instead.
content = content.replace('type artifactCleanupLookup interface {\\n\\tFindArtifactCleanupOperation(id string) (*model.TranscodeArtifactRecord, error)\\n}', 'type artifactCleanupLookup interface {\\n\\tFindArtifactCleanupOperation(id string) (*model.TranscodeArtifactRecord, error)\\n\\tLegacyProjectionMigrationState(source string) (*model.LegacyTranscodeProjectionMigrationState, error)\\n}', 1)
new_func = new_func.replace('state, err := d.artifactLookup.(*repository.TranscodeExecutionRepo).LegacyProjectionMigrationState(sourceID)', 'state, err := d.artifactLookup.LegacyProjectionMigrationState(sourceID)')
''',
'''# Migration lookup is deliberately separate from Artifact lookup.
''',
1,
)
content = content.replace(
'''# Update task action message switch if present.
content = content.replace('case TaskKindLegacyArtifactMigration:', 'case TaskKindLegacyProjectionMigration:\\n\\t\\tif action == TaskActionRetry {\\n\\t\\t\\treturn "旧转码历史登记已重新排队"\\n\\t\\t}\\n\\tcase TaskKindLegacyArtifactMigration:', 1) if 'func taskActionMessage' in content else content
''',
'''message_old = '''\tcase TaskActionRetry:
\t\tif kind == TaskKindScrape {
\t\t\treturn "刮削任务已重新提交"
\t\t}
\t\treturn "Artifact 清理已重新执行"
'''
message_new = '''\tcase TaskActionRetry:
\t\tif kind == TaskKindScrape {
\t\t\treturn "刮削任务已重新提交"
\t\t}
\t\tif kind == TaskKindLegacyProjectionMigration {
\t\t\treturn "旧转码历史登记已重新排队"
\t\t}
\t\treturn "Artifact 清理已重新执行"
'''
if content.count(message_old) != 1:
    raise RuntimeError("task action message anchor mismatch")
content = content.replace(message_old, message_new, 1)
''',
1,
)
# The fake Artifact lookup no longer needs the migration method.
start = content.find('# Existing fake lookup must satisfy the expanded lookup interface.')
end = content.find('# 9. Runtime History exposes migration progress', start)
if start != -1 and end != -1:
    content = content[:start] + '# Artifact lookup fakes remain unchanged because migration lookup is separate.\n\n' + content[end:]

# Missing migration-state tables are valid in narrow repository tests.
content = content.replace(
'''func (r *TranscodeExecutionRepo) LegacyProjectionMigrationState(source string) (*model.LegacyTranscodeProjectionMigrationState, error) {
\tvar state model.LegacyTranscodeProjectionMigrationState
''',
'''func (r *TranscodeExecutionRepo) LegacyProjectionMigrationState(source string) (*model.LegacyTranscodeProjectionMigrationState, error) {
\tif r == nil || r.db == nil || !r.db.Migrator().HasTable(&model.LegacyTranscodeProjectionMigrationState{}) {
\t\treturn nil, nil
\t}
\tvar state model.LegacyTranscodeProjectionMigrationState
''',
1,
)

# Add migration data only to RuntimeHistorySummary, not List or Detail.
content = content.replace(
"content = content.replace('\\tRetention         RuntimeHistoryRetentionPolicy `json:\"retention\"`\\n}', '\\tRetention         RuntimeHistoryRetentionPolicy `json:\"retention\"`\\n\\tLegacyMigration   *RuntimeHistoryLegacyMigration `json:\"legacy_migration,omitempty\"`\\n}', 1)",
'''summary_start = content.index("type RuntimeHistorySummary struct {")
summary_end = content.index("\\n}", summary_start)
summary_block = content[summary_start:summary_end]
summary_old = '\\tRetention         RuntimeHistoryRetentionPolicy `json:"retention"`'
summary_new = summary_old + '\\n\\tLegacyMigration   *RuntimeHistoryLegacyMigration `json:"legacy_migration,omitempty"`'
if summary_block.count(summary_old) != 1:
    raise RuntimeError("runtime history summary field anchor mismatch")
summary_block = summary_block.replace(summary_old, summary_new, 1)
content = content[:summary_start] + summary_block + content[summary_end:]
''',
1,
)
# Guard optional state table in Runtime History repository.
content = content.replace(
'''\tif migration, err := NewTranscodeExecutionRepo(r.db).LegacyProjectionMigrationState(LegacyTranscodeArtifactMigrationSource); err != nil {
\t\treturn nil, err
\t} else {
\t\tcounts.LegacyMigration = migration
\t}
''',
'''\tif r.db.Migrator().HasTable(&model.LegacyTranscodeProjectionMigrationState{}) {
\t\tif migration, err := NewTranscodeExecutionRepo(r.db).LegacyProjectionMigrationState(LegacyTranscodeArtifactMigrationSource); err != nil {
\t\t\treturn nil, err
\t\t} else {
\t\t\tcounts.LegacyMigration = migration
\t\t}
\t}
''',
1,
)

# Add the TypeScript field only to RuntimeHistorySummary.
content = content.replace(
"content = content.replace('  retention: RuntimeHistoryRetentionPolicy\\n}', '  retention: RuntimeHistoryRetentionPolicy\\n  legacy_migration?: RuntimeHistoryLegacyMigration\\n}', 1)",
'''summary_start = content.index("export interface RuntimeHistorySummary {")
summary_end = content.index("\\n}", summary_start)
summary_block = content[summary_start:summary_end]
summary_old = "  retention: RuntimeHistoryRetentionPolicy"
summary_new = summary_old + "\\n  legacy_migration?: RuntimeHistoryLegacyMigration"
if summary_block.count(summary_old) != 1:
    raise RuntimeError("web runtime history summary anchor mismatch")
summary_block = summary_block.replace(summary_old, summary_new, 1)
content = content[:summary_start] + summary_block + content[summary_end:]
''',
1,
)

path.write_text(content, encoding="utf-8")
