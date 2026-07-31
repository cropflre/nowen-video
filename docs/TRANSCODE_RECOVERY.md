# 转码持久队列、恢复与关闭协议

本文档描述 `refactor/server-lite-v1` 当前转码编排的恢复边界。目标是在服务重启、Worker 失联或容器停止时保留任务意图，同时继续使用 Lease Token 阻止旧 Worker 写入错误终态。

## 权威状态

`transcode_jobs` 是执行状态源，旧 `transcode_tasks` 仅作为现有管理 API 和页面的兼容投影。

活跃 Job 的基本状态流：

```text
queued
  -> claimed
  -> running
  -> completed / failed / cancelled
```

可恢复路径：

```text
claimed / running
  -> Lease expired
  -> queued

claimed / running
  -> Lite graceful shutdown deadline
  -> queued
```

回到 `queued` 时保留：

- Job ID
- Active Key
- Media ID
- Intent / Profile / StartMS
- Priority
- Source Fingerprint
- Planner Version
- 历史 Attempt 记录

同时清理：

- Worker ID
- Lease Token
- ClaimedAt
- Heartbeat
- LeaseExpiresAt
- CurrentAttemptID
- CompletedAt

## 启动恢复

服务启动时会扫描所有仍持有 Active Key 的 Job。

### queued

保持 `queued`，将兼容任务恢复为 `pending`，随后重新装载执行载荷。

### claimed / running 且没有 Lease

这是旧版本或异常升级留下的记录。若 Desired State 仍为 `running`，会安全回到 `queued`；若已经请求取消，则确认 `cancelled`。

### Lease 已过期

- Desired State 为 `running`：原子回到 `queued`。
- Desired State 为 `cancelled`：原子进入 `cancelled`。

### Lease 仍有效

不抢占、不重置，等待当前所有者续租、提交终态或自然过期。

### 缺失依赖

以下情况不会无限重试：

- `legacy_task_id` 缺失
- 兼容任务已删除
- Media 记录已删除
- Profile 不受支持
- Active Job 状态未知

这些记录会进入明确失败终态并释放 Active Key。

## 持久队列装载

数据库中的 `queued` Job 按以下顺序扫描：

```text
priority DESC, created_at ASC
```

装载器重建：

- Media
- legacy TranscodeTask
- Context / CancelFunc
- Quality / Start Offset
- 本地 Priority Heap 项

扫描周期为 1 秒。多实例可以同时观察同一 Job，但真正执行前仍必须通过数据库原子 Claim；没有获得 Lease 的本地副本会退出，不会启动 FFmpeg。

当前实现是：

```text
Database durable queue
  -> process-local bounded priority heap
  -> atomic Claim
  -> Worker Lease
  -> FFmpeg Runtime
```

本地堆容量满时，数据库 Job 继续保持 `queued`，后续扫描会重新尝试装载，不会标记失败。

## Attempt 连续编号

一次 Job 在重启后不会删除旧 Attempt 证据。

新 Attempt 编号按数据库中的最大编号继续：

```text
MAX(transcode_attempts.number) + 1
```

例如：

```text
Attempt 1: QSV failed
Attempt 2: software cancelled by shutdown
服务重启
Attempt 3: software running
```

重新执行前会清理上一次未完成的输出目录，避免旧分片和新 Timeline 混合。

## Lite 优雅关闭

`cmd/server-lite` 收到 `SIGINT` 或 `SIGTERM` 后按以下顺序执行：

1. 停止 mDNS。
2. 停止 HTTP 接收新请求。
3. 关闭本机任务交付堆。
4. 未被 Worker Claim 的任务从本地堆移除，但数据库继续保持 `queued`。
5. 最多等待 30 秒，让已 Claim 的任务正常完成。
6. 超时后，使用当前 Lease Token 将本机任务原子释放回 `queued`。
7. Lease 释放成功后才取消旧 Context。
8. 旧 Worker 即使延迟返回，也因 Lease Token 不匹配而无法提交 Job 终态。

关闭超时不会把任务伪装成 `cancelled` 或 `failed`。

## 可观测性

转码统计接口返回：

```json
{
  "queue_depth": 2,
  "durable_queue_depth": 5,
  "scheduler": "durable_priority_fifo"
}
```

字段含义：

- `queue_depth`：当前实例本地交付堆中的任务数量。
- `durable_queue_depth`：数据库中所有仍可执行的 queued Job 数量。
- `scheduler`：当前调度实现标识。

两种深度可能不同。例如多实例、队列容量限制或刚完成重启装载时，数据库深度可能高于本地深度。

## 当前不包含

- 运行中 FFmpeg 强制抢占。
- 基于 HLS 可续跑点的断点续转。
- 完全取消本地交付堆、由 Worker 直接 ClaimNextJob。
- Full 入口的关闭时序接线。
- 跨节点公平性与租约配置管理页面。

运行中抢占必须先具备产物隔离、明确续跑点和 Timeline 校验，不能简单杀进程后宣称已完成抢占。
