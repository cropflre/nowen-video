# Server Lite 统一任务事件

Lite 任务中心使用 `task_updated` 作为统一的 WebSocket 失效通知。服务端任务快照仍以 `GET /api/admin/tasks` 为唯一权威来源；客户端收到事件后重新读取快照，不从事件负载自行拼装任务状态。

## 生命周期事件

扫描、刮削和转码模块继续发送原有事件，同时额外发送统一信封：

```json
{
  "type": "task_updated",
  "data": {
    "kind": "transcode",
    "source_id": "task-id",
    "status": "running",
    "source_event": "transcode_progress"
  },
  "timestamp": 1785440000000
}
```

字段：

- `kind`：`scan`、`scrape` 或 `transcode`
- `source_id`：原模块中的媒体库 ID 或转码任务 ID；原事件无法提供时省略
- `status`：统一状态 `queued`、`running`、`completed`、`failed` 或 `cancelled`
- `source_event`：触发统一通知的原始模块事件

原有 `scan_*`、`scrape_*`、`transcode_*` 事件不会删除，Full 管理页面和旧客户端可以继续订阅。

## 操作受理事件

管理员通过统一任务操作接口提交取消或重试后，同一个 `task_updated` 类型会承载操作结果：

```json
{
  "type": "task_updated",
  "data": {
    "id": "transcode:task-id",
    "kind": "transcode",
    "source_id": "task-id",
    "action": "cancel",
    "accepted": true,
    "message": "取消请求已提交"
  },
  "timestamp": 1785440000000
}
```

Web 客户端将生命周期负载和操作负载建模为联合类型。消费者应根据 `source_event` 或 `action` 字段区分两种负载，不应假设所有 `task_updated` 都包含操作字段。

## Task Center 刷新策略

任务进度事件可能高频产生。Task Center 使用以下策略避免请求风暴：

1. 250ms 窗口内的连续 `task_updated` 合并为一次刷新。
2. `GET /api/admin/tasks` 执行期间收到的新事件只记录一次追加刷新。
3. 同一时刻只允许一个任务快照请求，防止较旧响应覆盖较新响应。
4. 任务操作成功后立即刷新，不等待事件合并窗口。
5. WebSocket 断线时继续使用 30 秒轮询兜底。

该策略只减少重复请求，不改变任务执行、状态持久化或原模块事件。