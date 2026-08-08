# Server Lite 第三阶段 API 示例

## 查询活动任务

```http
GET /api/admin/tasks?active=true&limit=50
Authorization: Bearer <token>
```

任务项会返回服务端允许的操作，客户端不应根据状态自行猜测：

```json
{
  "id": "transcode:<task-id>",
  "kind": "transcode",
  "status": "running",
  "source_id": "<task-id>",
  "actions": ["cancel"]
}
```

## 执行统一任务操作

```http
POST /api/admin/tasks/transcode/<task-id>/cancel
Authorization: Bearer <token>
```

```http
POST /api/admin/tasks/transcode/<task-id>/retry
Authorization: Bearer <token>
```

```http
POST /api/admin/tasks/scrape/<task-id>/retry
Authorization: Bearer <token>
```

当前安全策略：

- 仅运行中的转码任务提供 `cancel`
- 失败或已取消的转码任务提供 `retry`
- 失败的刮削任务提供 `retry`
- 扫描任务和排队中的转码任务不暴露操作
- 操作继续委托原转码/刮削服务执行，不创建新的任务队列或数据库表

## 一次获取播放信息与规划

Lite 的常规播放信息接口已经是统一入口。客户端能力通过查询参数传入，响应保留原有播放字段，并额外包含 `playback_plan`。

```http
GET /api/stream/<media-id>/info?supports_direct=true&supports_remux=true&supports_hevc=false
Authorization: Bearer <token>
```

响应结构：

```json
{
  "data": {
    "media_id": "<media-id>",
    "direct_play_url": "/api/stream/<media-id>/direct",
    "hls_url": "/api/stream/<media-id>/master.m3u8",
    "can_direct_play": true,
    "playback_plan": {
      "method": "direct",
      "url": "/api/stream/<media-id>/direct",
      "reason_code": "native_direct_play",
      "requires_transcode": false,
      "fallback_method": "transcode",
      "fallback_url": "/api/stream/<media-id>/master.m3u8"
    }
  }
}
```

## 单独获取或重新计算播放规划

`/plan` 继续保留给诊断工具或需要显式重新规划的客户端。

```http
GET /api/stream/<media-id>/plan?supports_direct=true&supports_remux=true&supports_hevc=false
Authorization: Bearer <token>
```

## 强制兼容转码

```http
GET /api/stream/<media-id>/plan?force_transcode=true&max_bitrate=3000000
Authorization: Bearer <token>
```

## Pulse 旧接口

Pulse 已永久移除。Full 中暂时保留旧 URL 作为兼容墓碑，统一返回：

```http
HTTP/1.1 410 Gone
Deprecation: true
Sunset: Thu, 30 Jul 2026 00:00:00 GMT
```

```json
{
  "error": "Pulse 功能已永久移除",
  "code": "pulse_removed"
}
```
