# Media Compute Node V2

Media Compute Node V2 把最初只服务于“精彩片段”的远程 Worker 协议提升为可复用的媒体计算节点平台。

## 已统一的节点生命周期

V2 统一：

1. Node heartbeat / online state
2. capability declaration
3. capability-aware task claim
4. Desktop same-capability preference
5. claim token / lease
6. progress + lease renewal
7. failure release / retry
8. generic job result envelope
9. job 私有 input / result adapter
10. durable 与 ephemeral 两类任务生命周期
11. Server fallback 或显式 client-only 策略

## 已接入的三个真实 Job

### `highlight_v1`

- capability: `highlight_v1`
- durable task
- Desktop: Tauri libmpv 单帧随机 Seek
- Android: `MediaMetadataRetriever`
- Server fallback: Sparse audio + scene analysis
- 调度：Desktop → Android → Server

### `preview_thumbnail_v1`

- capability: `preview_thumbnail_v1`
- request-scoped ephemeral task
- 接管精彩片段首次 hover Animated WebP 生产链路
- Server 下发少量采样时间点，Desktop/Android 稀疏抽帧
- 客户端上传受限静态帧，Server 校验后只做轻量 Animated WebP 封装
- `auto` 最多等待客户端 10 秒，然后取消远端任务并 Server fallback

### `chapter_detect_v1`

- capability: `chapter_detect_v1`
- durable task，复用既有 `chapter_gen` 任务状态和 `VideoChapter` 数据模型
- 正式后端继续兼容既有 Web URL：
  - `POST /api/media/:id/ai/chapters`
  - `GET /api/media/:id/chapters`
- Server 根据时长下发有限数量的候选中心点
- Desktop/Android 对每个中心点抽取前后约 3 秒两帧，计算低分辨率画面签名差异
- 客户端只返回 `{time, score}`，不决定最终章节，也不直接写数据库
- Server 校验 fingerprint、采样点、分数和完整性后，再执行阈值、最小章节间距、最大章节数和原子持久化
- `auto` 无客户端时走短窗口 sparse scene probe fallback，不回到旧的整片 FFmpeg scene filter 热路径

详见 `docs/MEDIA_COMPUTE_CHAPTER_DETECT_V1.md`。

## 调度模式

当前 `highlight_v1`、`preview_thumbnail_v1`、`chapter_detect_v1` 都遵守同一 execution mode：

- `auto`: Desktop → Android → Server fallback
- `client_preferred`: 只使用客户端；长任务可持续等待可用节点
- `server_only`: 不允许客户端领取这些 Job
- `off`: 不产生新的媒体分析计算；已缓存产物仍可读取

Desktop 优先严格按 `required_capability` 生效。一个只支持其他能力的 Desktop 不会阻塞 Android 领取当前 Job。

## V2 Claim envelope

```json
{
  "protocol_version": 2,
  "job_type": "chapter_detect_v1",
  "required_capability": "chapter_detect_v1",
  "task_id": "...",
  "claim_token": "...",
  "lease_expires_at": "...",
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

客户端必须先按 `job_type` 分派执行器，再解析对应 `input`。新 Job 禁止继续向 Claim 顶层添加业务字段。

## V2 result envelope

```json
{
  "claim_token": "...",
  "job_type": "chapter_detect_v1",
  "result": {
    "fingerprint": "...",
    "candidates": [
      {"time": 300, "score": 0.42}
    ]
  }
}
```

所有客户端结果必须经过对应 Server adapter 验证后才能持久化。客户端永远不拥有最终数据库写入权。

## V1 兼容

- `/api/media-analysis/workers/**` 暂时继续作为兼容传输 URL。
- `highlight_v1` Claim 顶层继续镜像旧扁平字段，保证已发布 V1 客户端可继续工作。
- complete 路由同时接受 V1 highlight body 和 V2 `job_type + result`。
- 旧客户端不会声明 `preview_thumbnail_v1` / `chapter_detect_v1`，因此不会误领新任务。
- 新 Server 可以与旧 Desktop/Android 分批升级。

在更多客户端版本实际发布并稳定前，不为了命名把历史 URL 强行迁移成 `/media-compute/**`。

## 通用任务注册

业务通过：

- `RegisterComputeTask(MediaComputeTaskRegistration)`
- `UnregisterComputeTask(taskID)`

进入统一协调层，只提供 task id、job type、required capability、私有 input，以及可选 media id / fingerprint。

协调层负责节点状态、能力匹配、Claim、租约和 Desktop preference；业务 adapter 负责自己的持久化、结果安全校验和 fallback。

三个 Job 已经分别验证：

- `highlight_v1`: 长任务 + 图片结果
- `preview_thumbnail_v1`: 交互短任务 + 请求级超时取消
- `chapter_detect_v1`: 长任务 + 客户端候选 + Server 汇总决策

因此 V2 已经不再是“精彩片段专用协议的泛化命名”，而是实际承载不同任务生命周期与结果形态的媒体计算平台。

## 新 Job 接入规则

1. 每个任务定义独立 `job_type`。
2. 每个执行能力定义独立 `required_capability`。
3. 私有参数只进入 `input`。
4. Desktop/Android 只声明真实实现的 capabilities。
5. Server 按 capability 调度，不能按“节点在线”盲发任务。
6. durable 长任务通过 progress 续租；ephemeral 任务必须有请求级超时/取消。
7. 客户端结果先校验，再持久化。
8. dispatcher 必须定义 Server fallback 或明确 client-only。
9. fallback 前必须正确撤销/释放远端 Claim，避免迟到结果覆盖 Server 结果。
10. 新计算器优先拆成 job-specific executor，Agent 保持为节点生命周期与分派层。

## 下一阶段

第三个真实 Job 已经验证“客户端候选 → Server 聚合 → durable persistence”模式。

下一步优先建议 `waveform_v1`，用于验证音频 workload、不同输入输出形态和跨 Job 资源调度；同时应开始对已发布 Desktop/Android 版本做真实端到端观察，重点看 Claim 延迟、租约续期、移动端功耗和 Server fallback 命中率。
