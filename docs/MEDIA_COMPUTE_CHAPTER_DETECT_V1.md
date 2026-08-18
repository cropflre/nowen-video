# Media Compute `chapter_detect_v1`

`chapter_detect_v1` 是 Media Compute Node V2 的第三个真实生产任务，用于把原有章节生成链路中的视频场景探测从 Server 全片解码升级为 Desktop / Android 优先的稀疏候选检测。

## 目标

现有 Web API 保持不变：

- `POST /api/media/:id/ai/chapters`
- `GET /api/media/:id/chapters`
- `GET /api/media/:id/ai/tasks`
- `GET /api/ai/tasks/:taskId`

正式后端内部改为：

1. Server 创建 `chapter_gen` durable task。
2. Server 根据媒体时长生成有限数量的候选中心时间点。
3. `chapter_detect_v1` 进入统一 Media Compute Node V2 队列。
4. Desktop 优先；同 capability 的 Desktop 不可用时 Android 可领取。
5. 客户端围绕每个中心点分别抓取前后约 3 秒两帧，计算低分辨率亮度签名差异。
6. 客户端只回传 `{time, score}` 候选，不上传图片，也不决定最终章节。
7. Server 二次校验 fingerprint、采样时间点、分数范围和候选完整性。
8. Server 根据阈值、章节最小间距和最大章节数归并候选，并原子替换 `VideoChapter`。
9. `auto` 模式没有可用客户端或客户端租约过期时，Server 使用短窗口 sparse scene probe fallback，不再回到旧的全片 scene filter 热路径。

## 调度模式

- `auto`: Desktop → Android → Server sparse fallback
- `client_preferred`: Desktop → Android；没有客户端时保持等待，不主动启动 Server 探测
- `server_only`: 直接 Server sparse scene probe
- `off`: 禁止启动新的章节检测任务

章节任务与 `highlight_v1` 一样是 durable task，会通过 progress 延长 Claim lease，并在服务重启后把遗留的 pending/running `chapter_gen` 标记为 `interrupted`。

## Claim input

```json
{
  "protocol_version": 2,
  "job_type": "chapter_detect_v1",
  "required_capability": "chapter_detect_v1",
  "task_id": "...",
  "claim_token": "...",
  "input": {
    "media_id": "...",
    "fingerprint": "...",
    "duration": 7200,
    "stream_url": "/api/stream/.../direct",
    "sample_times": [150, 300, 450],
    "probe_gap_seconds": 3,
    "min_chapter_seconds": 180,
    "max_chapters": 12,
    "capture_width": 240,
    "engine_version": 1
  }
}
```

采样数量根据媒体时长自适应，当前上限 72；常见电影最多约 48~60 个中心点。客户端每个中心点只执行两次随机 Seek，不需要顺序解码整部电影。

## Result

```json
{
  "claim_token": "...",
  "job_type": "chapter_detect_v1",
  "result": {
    "fingerprint": "...",
    "candidates": [
      {"time": 300, "score": 0.42},
      {"time": 450, "score": 0.08}
    ]
  }
}
```

客户端必须对 Server 下发的每个 `sample_time` 返回一个候选。Server 会把时间点重新映射到自己的采样计划，允许有限浮点误差，但不接受客户端自行插入额外时间点。

## Server 最终决策

客户端不拥有章节写入权。Server 会：

- 拒绝 NaN / Inf / 越界时间；
- 拒绝小于 0 或大于 1 的 score；
- 校验当前媒体 fingerprint 未发生变化；
- 排除靠近开头/结尾的噪声点；
- 依据当前候选分布动态选择较高分候选；
- 保证章节边界最小间隔；
- 限制最大章节数量；
- 无可靠候选时使用均匀章节 fallback；
- 使用数据库事务原子替换该媒体全部 `VideoChapter`。

因此 Desktop / Android 只提供“计算建议”，最终数据一致性仍由 Server 掌控。

## 客户端实现

### Desktop

继续复用 Tauri 已有 `highlight_capture_frame` / libmpv 单帧随机 Seek。章节执行器独立放在 `DesktopChapterCompute.ts`，避免把通用 Agent 继续堆成巨型业务文件。

### Android

继续复用 `MediaMetadataRetriever`。章节执行器独立放在 `ChapterComputeData.kt`，并由现有 `HighlightComputeAgent` 的统一 Claim 循环分派，保持一个物理 Android 设备只对应一个 Media Compute Node。

Android 仍受现有 Wi-Fi、电量、充电、节能和温控资格约束。

## 与旧章节系统的关系

旧全量 Server 的 `AISceneService` 继续保留给 legacy compatibility server，不在本阶段物理删除。

正式 `cmd/server-lite` 已接管 Web 既有章节 URL，默认章节标题暂为确定性的 `第 N 章`，不引入 AI 依赖。后续若需要 AI 标题，应作为 Server 端独立 enrichment 阶段，而不是把 AI 配置重新塞回媒体计算节点协议。
