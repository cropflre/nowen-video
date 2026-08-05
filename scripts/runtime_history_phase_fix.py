#!/usr/bin/env python3
from pathlib import Path

path = Path("internal/service/runtime_history_test.go")
content = path.read_text(encoding="utf-8")
old = '"gorm.io/driver/sqlite"'
new = '"github.com/glebarez/sqlite"'
if content.count(old) != 1:
    raise RuntimeError("runtime history test sqlite import marker missing")
path.write_text(content.replace(old, new, 1), encoding="utf-8")
