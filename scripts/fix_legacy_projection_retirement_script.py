#!/usr/bin/env python3
from pathlib import Path

path = Path(__file__).with_name("implement_legacy_projection_retirement.py")
content = path.read_text(encoding="utf-8")

old = "'return 0, fmt.Errorf(\"artifact cleanup did not delete record: %s\", artifact.ID)',"
new = "'return removed, fmt.Errorf(\"artifact cleanup did not delete record: %s\", artifact.ID)',"
if content.count(old) != 1:
    raise RuntimeError(f"cleanup anchor mismatch: {content.count(old)}")
content = content.replace(old, new, 1)

redundant = '''replace_once(
    "internal/service/artifact_maintenance.go",
    'cancelled=%d artifacts=%d attempts=%d tasks=%d paths=%d", report.JobsCancelled, report.ArtifactsDeleted, report.AttemptsRetired, report.TasksRetired, report.PathsRemoved',
    'cancelled=%d artifacts=%d attempts=%d paths=%d", report.JobsCancelled, report.ArtifactsDeleted, report.AttemptsRetired, report.PathsRemoved',
)
'''
if content.count(redundant) != 1:
    raise RuntimeError(f"retirement log assertion mismatch: {content.count(redundant)}")
content = content.replace(redundant, "", 1)

path.write_text(content, encoding="utf-8")
