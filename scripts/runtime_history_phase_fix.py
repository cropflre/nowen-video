#!/usr/bin/env python3
from pathlib import Path


def replace_once(path: Path, old: str, new: str, message: str) -> None:
    content = path.read_text(encoding="utf-8")
    if content.count(old) != 1:
        raise RuntimeError(message)
    path.write_text(content.replace(old, new, 1), encoding="utf-8")


replace_once(
    Path("internal/service/runtime_history_test.go"),
    '"gorm.io/driver/sqlite"',
    '"github.com/glebarez/sqlite"',
    "runtime history test sqlite import marker missing",
)

replace_once(
    Path("internal/repository/runtime_history.go"),
    '''\tvar bounds struct {
\t\tOldestAt *time.Time
\t\tNewestAt *time.Time
\t}
\tif err := r.db.Model(&model.TranscodeJobRecord{}).
\t\tSelect("MIN(created_at) AS oldest_at, MAX(COALESCE(completed_at, updated_at)) AS newest_at").
\t\tScan(&bounds).Error; err != nil {
\t\treturn nil, err
\t}
\tcounts.OldestAt = bounds.OldestAt
\tcounts.NewestAt = bounds.NewestAt
''',
    '''\tvar oldest model.TranscodeJobRecord
\tif err := r.db.Model(&model.TranscodeJobRecord{}).
\t\tSelect("created_at").
\t\tOrder("created_at ASC").
\t\tTake(&oldest).Error; err == nil {
\t\tvalue := oldest.CreatedAt
\t\tcounts.OldestAt = &value
\t} else if err != gorm.ErrRecordNotFound {
\t\treturn nil, err
\t}

\tvar newest model.TranscodeJobRecord
\tif err := r.db.Model(&model.TranscodeJobRecord{}).
\t\tSelect("updated_at", "completed_at").
\t\tOrder("COALESCE(completed_at, updated_at) DESC").
\t\tTake(&newest).Error; err == nil {
\t\tif newest.CompletedAt != nil {
\t\t\tvalue := *newest.CompletedAt
\t\t\tcounts.NewestAt = &value
\t\t} else {
\t\t\tvalue := newest.UpdatedAt
\t\t\tcounts.NewestAt = &value
\t\t}
\t} else if err != gorm.ErrRecordNotFound {
\t\treturn nil, err
\t}
''',
    "runtime history aggregate bounds marker missing",
)
