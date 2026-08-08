# 转码 Attempt 工作区与 Artifact Store 协议

本文定义 `Probe -> Planner -> Orchestrator -> Resource Governor -> Executor -> Artifact Store -> Client Playback` 链路中的 Artifact Store 正式边界。本阶段目标不是把输出目录换一个名字，而是消除旧 Worker、新 Worker、硬件回退 Attempt 和播放器之间的共享写目录竞态。

## 问题定义

旧实现按以下路径直接写入并读取运行中 HLS：

```text
cache/transcode/<media_id>/<profile>/stream.m3u8
cache/transcode/<media_id>/<profile>/seg0001.ts
```

同一个 Job 的硬件 Attempt、软件回退 Attempt、Lease 恢复后的新 Worker 都可能访问同一目录。即使数据库 Lease 能阻止旧 Worker提交终态，也不能阻止旧 FFmpeg 继续覆盖新 Worker 的 manifest 或分片。

因此数据库状态 fencing 和文件系统写入隔离必须同时成立。

## 领域模型

### Attempt Workspace

每个 Attempt 获得唯一、不可复用的工作区：

```text
cache/transcode/workspaces/<job_id>/<attempt_id>/hls/
```

规则：

- 一个 Attempt 只写自己的工作区。
- 硬件失败后的软件回退创建新的 Attempt 和新的工作区。
- Lease 回收后的新 Worker 创建新的 Attempt 和新的工作区。
- 失败、取消、超时和失去 Lease 的 Attempt 永远不能改写其他 Attempt 的目录。
- 工作区路径写入 `transcode_attempts.workspace_path`，作为诊断和清理依据。

### Artifact

`transcode_artifacts` 是播放器可读产物的唯一状态源。HLS Variant Artifact 包含：

- Media ID
- Job ID
- Attempt ID
- Profile ID
- Source Fingerprint
- Planner Version
- Manifest Path
- Workspace / Temp Path
- Published Path
- 状态
- 发布时间、大小、时长、分片时长和错误信息

Artifact 状态机：

```text
staging
  -> publishing
  -> published

staging / publishing
  -> failed / cancelled / abandoned

published
  -> superseded / expired
```

状态含义：

- `staging`：当前有效 Lease 所属 Attempt 正在写入工作区，可供实时播放读取已经原子完成的 manifest 和分片。
- `publishing`：FFmpeg 已成功退出，Artifact Store 正在把工作区发布为不可变版本。
- `published`：不可变版本已提交，可作为后续播放的稳定缓存。
- `failed`：Executor 失败，工作区仅供诊断和延迟清理。
- `cancelled`：用户取消或优雅关闭取消。
- `abandoned`：Attempt 已失去 Lease，不能再对外可读。
- `superseded`：同一媒体、源指纹和 Profile 已有更新版本。

## 文件布局

```text
cache/transcode/
  workspaces/
    <job_id>/
      <attempt_id>/
        hls/
          stream.m3u8
          seg0000.ts
          ...
  artifacts/
    <media_id>/
      <profile_id>/
        <artifact_id>/
          stream.m3u8
          seg0000.ts
          ...
```

工作区和发布目录必须位于同一文件系统，以便目录重命名保持原子性。Artifact ID 是发布版本，不复用 `media_id/profile` 目录作为可变目标。

## 播放读取协议

客户端继续请求统一 URL：

```text
/api/stream/:media_id/:profile/stream.m3u8
/api/stream/:media_id/:profile/:segment
```

客户端不感知 Job、Attempt 或物理目录。服务端 Artifact Resolver 按以下顺序选择：

1. 当前有效 Job、当前 Attempt、有效 Lease 对应的 `staging` / `publishing` Artifact。
2. 相同 Media、Profile、Source Fingerprint、Planner Version 的最新 `published` Artifact。
3. 迁移期 Legacy Artifact Adapter 导入的历史产物。

Resolver 不读取任意目录，不根据文件是否存在猜测权威状态。`staging` 只有在 Job 当前 Attempt 和 Lease 条件仍成立时可读；失去 Lease 后立即从解析结果中消失。

Manifest 和单个分片仍由 FFmpeg 临时写入并原子重命名，播放器不得读取半成品。

## 发布协议

完成发布采用两阶段协议：

### 1. Prepare Publish

数据库条件必须同时满足：

- Job ID 匹配。
- Lease Token 匹配。
- Lease 未过期。
- Desired State 为 running。
- Current Attempt ID 匹配。
- Artifact 状态为 staging。

满足后将 Artifact 标记为 `publishing`，写入目标不可变路径。

### 2. Filesystem Publish

将 Attempt 工作区目录原子重命名到：

```text
cache/transcode/artifacts/<media_id>/<profile_id>/<artifact_id>
```

若目标已存在，视为协议错误，不覆盖。

### 3. Commit Publish

单个数据库事务完成：

- 再次校验 Job Lease、Current Attempt 和 Artifact 状态。
- Artifact -> published。
- 写入 Manifest Path、PublishedAt、SizeBytes。
- 旧同源版本 -> superseded。
- Job -> completed，释放 Active Key 和 Lease。

若 Commit 失败，物理目录是不可见孤儿版本，不会被 Resolver 返回，由恢复清理器处理。旧 Worker不能把孤儿目录变成 published。

## 失败、取消与恢复

- Executor 失败：Attempt -> failed，Artifact -> failed，Job 可进入软件回退或 failed。
- 用户取消：Attempt -> cancelled，Artifact -> cancelled，Job -> cancelled。
- Lease 过期或优雅释放：当前 Artifact -> abandoned；新 Worker 创建新 Attempt，不复用旧工作区。
- 服务启动：扫描 `staging/publishing` Artifact。只有仍与有效 Job Lease、Current Attempt 一致的记录可继续对外读取，其余标记 abandoned。
- `publishing` 且目标目录存在但数据库未提交：若 Lease 和 Attempt 仍有效可重试 Commit；否则标记 abandoned 并延迟清理。

## 数据库迁移

新增字段采用向前兼容迁移：

### transcode_attempts

- `workspace_path`

### transcode_artifacts

- `media_id`
- `source_fingerprint`
- `planner_version`
- `manifest_path`
- `published_at`
- `error_code`
- `error_message`

迁移步骤：

1. AutoMigrate 只增加字段和索引，不删除旧列。
2. 根据 `transcode_artifacts.job_id -> transcode_jobs` 回填 Media、Source Fingerprint 和 Planner Version。
3. 历史 `published` 行将 `manifest_path` 回填为 `path/stream.m3u8`。
4. 旧 `transcode_tasks` 完成产物通过正式 Legacy Artifact Adapter 导入，标记迁移来源。
5. 新写入只走 Attempt Workspace 和 Artifact Store。

回滚策略：旧 `transcode_tasks.output_dir` 和旧目录在迁移观察期不删除，旧版本服务仍可读取；新增字段被旧版本忽略。观察期结束后通过独立迁移版本删除 Legacy Adapter，而不是长期保留双轨写入。

## 客户端边界

Web、PC、Android、Emby/Infuse 不增加 Artifact 判断。它们继续消费 Planner 返回的统一播放 URL和 fallback。Artifact 版本选择完全位于服务端。

客户端必须：

- 对 EVENT playlist 正常轮询。
- 不缓存运行中 manifest。
- 对不可变已发布分片允许长缓存。
- 不拼接物理目录、Job ID 或 Attempt ID。

## 可观测性

统计至少包含：

- staging / publishing / published / abandoned Artifact 数量
- Workspace 总占用
- 发布成功、发布回滚、孤儿清理次数
- Resolver 命中 staging / published / legacy 次数
- Attempt 工作区创建与清理失败次数

管理诊断必须能从 Job 展开 Attempt 和 Artifact，展示 Worker、Lease、Workspace、发布版本和恢复原因。

## 测试与验收

### 单元测试

- 路径布局不可逃逸 Cache Root。
- 每个 Attempt 生成不同工作区。
- Resolver 只返回当前有效 Lease 的 staging Artifact。
- Published 版本按 Source Fingerprint 和 Planner Version 隔离。
- Prepare / Commit 必须匹配 Lease 和 Current Attempt。

### 集成测试

- 硬件 Attempt 失败后软件 Attempt 不共享任何文件。
- Lease 回收后旧 Worker继续写入，不影响新 Worker目录和播放结果。
- Publish 后客户端 URL 无变化，Resolver 从 staging 切换到 published。
- 数据库 Commit 失败时物理孤儿不会被播放器读取。
- 服务重启可恢复 queued Job，并隔离旧 Workspace。

### 端到端与故障测试

- 播放中杀死服务并重启，新 Worker产出的首片可正常播放。
- FFmpeg 忽略取消并延迟退出时，旧进程不能污染新 Timeline。
- SQLite 和 PostgreSQL 均验证条件更新与事务语义。
- 本地磁盘、NAS 挂载和 Docker Volume 验证原子重命名前提。

### 性能基线

记录：

- Workspace 创建耗时
- 首片 P50/P95/P99
- Resolver 查询 P50/P95/P99
- Publish rename 和 Commit 耗时
- 多播放器并发读取运行中 Artifact 的延迟

## 阶段完成标准

本阶段只有在以下条件全部成立时完成：

- 所有 Runtime HLS Attempt 写入隔离工作区。
- 播放器不再直接读取共享可变 `media/profile` 目录。
- Lease 失效的 Attempt 立即失去可读资格。
- 成功产物以不可变 Artifact 版本发布。
- 硬件回退、重启恢复和旧 Worker 延迟退出故障测试通过。
- Legacy Adapter 有明确观察期和删除阶段，不新增双写。
