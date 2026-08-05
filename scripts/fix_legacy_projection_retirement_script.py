#!/usr/bin/env python3
from pathlib import Path

path = Path(__file__).with_name("implement_legacy_projection_retirement.py")
content = path.read_text(encoding="utf-8")
old = "'return 0, fmt.Errorf(\"artifact cleanup did not delete record: %s\", artifact.ID)',"
new = "'return removed, fmt.Errorf(\"artifact cleanup did not delete record: %s\", artifact.ID)',"
if content.count(old) != 1:
    raise RuntimeError(f"cleanup anchor mismatch: {content.count(old)}")
path.write_text(content.replace(old, new, 1), encoding="utf-8")
