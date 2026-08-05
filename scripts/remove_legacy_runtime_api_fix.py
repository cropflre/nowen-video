#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]
path = root / "internal/middleware/middleware.go"
content = path.read_text(encoding="utf-8")

function_block = '''func retiredPersistentRuntimeAdminPath(path string) bool {
	return path == "/api/admin/transcode" ||
		strings.HasPrefix(path, "/api/admin/transcode/") ||
		path == "/api/admin/transcode-tasks" ||
		strings.HasPrefix(path, "/api/admin/transcode-tasks/") ||
		path == "/api/admin/tasks/transcode" ||
		strings.HasPrefix(path, "/api/admin/tasks/transcode/")
}

'''
if content.count(function_block) != 1:
    raise RuntimeError("retired Runtime admin path helper marker mismatch")
content = content.replace(function_block, "", 1)

tombstone_block = '''		if retiredPersistentRuntimeAdminPath(c.Request.URL.Path) {
			c.Header("Cache-Control", "no-store")
			c.JSON(http.StatusGone, gin.H{
				"error": "持久 Runtime 转码任务已退役，请使用播放会话或管理员预处理",
				"code":  "persistent_runtime_transcode_retired",
			})
			c.Abort()
			return
		}
'''
if content.count(tombstone_block) != 1:
    raise RuntimeError("retired Runtime AdminOnly tombstone marker mismatch")
content = content.replace(tombstone_block, "", 1)
path.write_text(content, encoding="utf-8")
