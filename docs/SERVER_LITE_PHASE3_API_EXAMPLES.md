# Server Lite 第三阶段 API 示例

## 查询活动任务

```http
GET /api/admin/tasks?active=true&limit=50
Authorization: Bearer <token>
```

## 获取播放规划

```http
GET /api/stream/<media-id>/plan?supports_direct=true&supports_remux=true&supports_hevc=false
Authorization: Bearer <token>
```

## 强制兼容转码

```http
GET /api/stream/<media-id>/plan?force_transcode=true&max_bitrate=3000000
Authorization: Bearer <token>
```
