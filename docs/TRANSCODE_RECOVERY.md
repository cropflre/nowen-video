# 转码持久队列、Artifact 恢复与关闭协议

本文档描述 `refactor/server-lite-v1` 当前转码编排的恢复边界。目标是在服务重启、Worker 失联、容器停止或旧 FFmpeg 延迟退出时保留任务意图，同时从数据库状态和文件系统产物两个层面阻止旧 Worker 污染新执行结果。

## 权威状态

- `transcode_jobs`：任务状态、排队顺序、Desired State、Worker 和 Lease 的唯一权威。
- `transcode_attempts`：每次实际进程执行及其诊断证据。
- `transcode_artifacts`：播放器可读产物及发布版本的唯一权威。
- `transcode_tasks`：迁移期管理 API 和页面兼容投影，不参与 Claim、Lease 或 Artifact 解析。

Job 基本状态流：

```text
queued
  -> claimed
  -> running
  -> completed / failed / cancelled
```

Artifact 基本状态流：

```text
staging
  -> publishing
  -> published

staging / publishing
  -> failed / cancelled / abandoned

published
  -> superseded / expired
```

可恢复路径：

```text
claimed / running
  -> Lease expired
  -> current Artifact abandoned
  -> Job queued

claimed / running
  -> graceful shutdown deadline
  -> current Artifact abandoned
  -> Job queued
```

回到 `queued` 时保留：

- Job ID
- Active Key
- Media ID
- Intent / Profile / StartMS / DurationMS
- Priority
- Source Fingerprint
- Planner Version
- 历史 Attempt 与 Artifact 记录

同时清理：

- Worker ID
- Lease Token
- ClaimedAt
- Heartbeat
- LeaseExpiresAt
- CurrentAttemptID
- CompletedAt

## 数据库直接领取

`transcode_jobs` 是持久队列，不再把数据库任务复制到进程内任务堆作为排队权威。

Worker 直接查找：

```text
status = queued
AND desired_state = running
ORDER BY priority DESC, created_at ASC, id ASC
```

随后使用带状态条件的原子更新 Claim。多个实例可以观察到同一候选，但只有一个实例能获得新的 Lease Token。

进程内信号仅用于唤醒 Worker，不保存任务状态。服务重启后不需要重建内存队列即可继续领取数据库中的 queued Job。

## Attempt 工作区隔离

每次 Attempt 使用唯一工作区：

```text
cache/transcode/workspaces/<job_id>/<attempt_id>/hls/
```

以下情况都会创建新 Attempt 和新工作区：

- 硬件转码失败后回退软件转码。
- Lease 过期后被其他 Worker 重新领取。
- 服务重启后恢复执行。
- 后续正式抢占恢复。

旧 Attempt 工作区不会被新 Worker 清空、覆盖或复用。即使旧 FFmpeg 忽略取消并继续写文件，也只能写自己的工作区。

Attempt 编号按数据库历史继续：

```text
MAX(transcode_attempts.number) + 1
```

例如：

```text
Attempt 1: QSV failed, workspace A
Attempt 2: software lost Lease, workspace B
服务重启
Attempt 3: software running, workspace C
```

## Artifact 可读资格

播放器不根据目录或文件是否存在判断哪个产物有效。每次 playlist 或 segment 请求都通过 Artifact Resolver 查询数据库。

运行中 Artifact 只有在以下条件全部成立时可读：

- Artifact 状态为 `staging` 或 `publishing`。
- Artifact Attempt 等于 Job `current_attempt_id`。
- Job 仍持有 Active Key。
- Desired State 为 `running`。
- Job 状态为 `claimed` 或 `running`。
- Lease Token 非空。
- Lease 未过期。

若运行中 Artifact 不满足条件，Resolver 只会查找相同 Media、Profile、Source Fingerprint、Planner Version 和 Kind 的最新 `published` 版本。

因此旧 Worker 即使仍在写自己的工作区，也会在 Lease 失效后立即失去播放可见性。

## 启动恢复

服务启动时按以下顺序恢复：

### 1. Artifact 所有权对账

扫描所有 `staging` / `publishing` Artifact。只有仍与有效 Job Lease 和 Current Attempt 一致的记录保留原状态，其余统一进入：

```text
status = abandoned
error_code = startup_reconciliation
```

其他实例仍持有未过期 Lease 的 Artifact 不会被抢占或 abandoned。

### 2. Job 状态恢复

#### queued

保持 queued，将兼容任务恢复为 pending，并唤醒数据库 Worker。

#### claimed / running 且没有 Lease

这是旧版本或异常升级留下的记录：

- Desired State 为 running：Job 回到 queued，当前 Artifact abandoned。
- 已请求取消：Job 进入 cancelled，当前 Artifact abandoned。

#### Lease 已过期

- Desired State 为 running：原子回到 queued，当前 Artifact abandoned。
- Desired State 为 cancelled：原子进入 cancelled，当前 Artifact abandoned。

#### Lease 仍有效

不抢占、不重置。等待当前所有者续租、发布、提交终态或自然过期。

#### 缺失依赖

以下情况不会无限重试：

- `legacy_task_id` 缺失且当前迁移阶段仍需要兼容载荷。
- 兼容任务已删除。
- Media 记录已删除。
- Intent 或 Profile 不受支持。
- Active Job 状态未知。

这些 Job 进入明确失败终态，释放 Active Key，并 abandoned 当前 Artifact。

## 两阶段发布恢复

完成产物发布分为：

1. 数据库 `staging -> publishing`，必须匹配 Lease Token 和 Current Attempt。
2. 工作区目录原子重命名到不可变版本：

```text
cache/transcode/artifacts/<media_id>/<profile_id>/<artifact_id>/
```

3. 单个数据库事务提交 Artifact `published` 与 Job `completed`。

故障边界：

- Prepare 前失去 Lease：不能进入 publishing。
- Rename 前失败：Artifact 进入 failed，工作区按保留策略清理。
- Rename 后、数据库 Commit 前崩溃：物理目录是不可见孤儿，不会被 Resolver 返回。
- Commit 时失去 Lease：事务回滚，Artifact abandoned，Job 不会被旧 Worker标记 completed。
- 发布切换瞬间：播放端会重新解析一次 Artifact，避免 workspace 原子改名窗口产生偶发 404。

## Lite / Full 优雅关闭

`cmd/server-lite` 与 `cmd/server` 收到 `SIGINT` 或 `SIGTERM` 后都使用同一套关闭协议：

1. 停止服务发现广播。
2. 停止 HTTP 接收新请求，避免关闭队列后仍提交新任务。
3. 停止 Worker 领取新 Job。
4. 最多等待 30 秒，让已 Claim 的任务正常完成并发布。
5. 超时后，当前 Lease 所有者将 Job 原子释放回 queued。
6. Lease 释放成功后，将当前 Attempt 的 staging / publishing Artifact 标记 abandoned。
7. 最后取消旧 Context。
8. 旧 Worker 延迟返回时，因 Lease、Current Attempt 和 Artifact 状态均不匹配，不能发布或提交终态。

关闭超时不会把任务伪装成 cancelled 或 failed，也不会让旧工作区继续对播放器可见。

## 历史产物迁移

旧版本使用：

```text
cache/transcode/<media_id>/<profile>/
```

迁移期由正式 Legacy Artifact Adapter 按需导入：

- 使用确定性 Artifact ID，重复导入幂等。
- 标记 `migration_source=legacy_runtime_hls_v1`。
- 只读导入，不再向旧目录双写。
- 新 Attempt 只写 workspaces，新成功版本只发布到 artifacts。
- 观察期结束后删除 Legacy Adapter 和旧目录读取路径。

数据库迁移只增加字段、索引并从 Job 回填 Artifact 身份，不删除旧 Job、Attempt、Artifact 或兼容任务。旧版本可忽略新增字段，满足回滚读取要求。

## 保留与清理

清理器按 Artifact 数据库记录删除文件，不再根据 `media/profile` 猜测目录。

- published：按完成缓存保留期处理。
- failed / cancelled / abandoned / superseded / expired：按失败缓存保留期处理。
- Attempt 诊断记录继续保留；Artifact 文件属于可回收缓存。
- 历史任务尚未导入 Artifact 时，只允许删除明确的旧目录，不调用 Resolver 删除新版本。
- 所有删除路径必须位于 Artifact Store Root 内，路径逃逸会被拒绝。

## 可观测性

转码统计至少返回：

```json
{
  "queue_depth": 2,
  "durable_queue_depth": 5,
  "scheduler": "database_priority_fifo",
  "lease_duration_seconds": 30,
  "artifact_status_counts": {
    "staging": 1,
    "published": 12,
    "abandoned": 2
  },
  "artifact_store_root": "/cache/transcode"
}
```

同时保留 Resource Governor、Probe、Probe Warmup 和磁盘使用量统计。

CI 固定验证：

- Lease / Artifact / Shutdown race tests。
- 旧 Worker 在 Lease 回收后不能 Prepare 或 Commit Publish。
- Resolver 在 Lease 失效后不再返回旧工作区。
- 启动对账保留其他实例有效 Lease，并 abandoned 孤儿 Artifact。
- 数据库迁移回填且不删除历史记录。
- Artifact Resolver benchmark。
- Lite / Full 编译和 Docker 镜像。

## 当前不包含

- 基于 HLS 可续跑点的运行中强制抢占。
- 启动流与持续转码之间经过媒体时间线验证的无缝衔接。
- PostgreSQL 专属并发领取优化，例如 `FOR UPDATE SKIP LOCKED`。
- 跨节点公平性与 Lease 配置管理页面。

运行中抢占必须建立在 Attempt 隔离、Artifact 发布、明确续跑点和 Timeline 校验之上，不能简单杀进程后宣称完成抢占。
