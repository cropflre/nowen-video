# Media Compute Node V2

Media Compute Node V2 把原本只服务于“精彩片段”的远程 Worker 协议提升为可复用的媒体计算节点协议。

## 已统一的节点生命周期

V2 统一以下能力：

1. Node heartbeat / online state
2. capability declaration
3. capability-aware task claim
4. Desktop preference window
5. claim token / lease
6. progress + lease renewal（长任务按需上报）
7. failure release
8. generic job result envelope
9. job 私有 input / result adapter
10. Server fallback 或 client-only 策略

## 已接入的两个真实 Job

### 1. `highlight_v1`

- `job_type = highlight_v1`
- `required_capability = highlight_v1`
- Desktop 使用现有 Tauri libmpv 单帧随机 Seek。
- Android 使用 `MediaMetadataRetriever` 硬件抽帧。
- `auto` 调度：Desktop → Android → Server Sparse V2。
- 长任务通过 progress 上报续租并刷新节点在线时间。

### 2. `preview_thumbnail_v1`

- `job_type = preview_thumbnail_v1`
- `required_capability = preview_thumbnail_v1`
- 直接接管现有“精彩片段首次 hover 懒生成 Animated WebP”生产链路。
- 不创建第二套 Worker/Claim/Lease 协议，也不新增客户端解码库。

预览任务由 Server 生成 5 个位于片段中间约 2.5 秒窗口内的采样时间点。Desktop/Android 只按这些时间点稀疏 Seek 并分别解码单帧，不下载或顺序解码完整片段。

客户端上传的是受限静态帧，而不是直接上传最终动画：

- 最多 5 张当前任务帧（协议上限 8 张）；
- 单帧有大小限制；
- 总 payload 有大小限制；
- Server 校验 Base64、MIME 与 JPEG/PNG/WebP 文件魔数；
- Server 再校验 media fingerprint、media ID、highlight ID、时间点和任务租约；
- 通过后 Server 只对这些静态图片做轻量 Animated WebP 封装，不再解码源视频。

这样保留原有 `/api/media/:id/highlights/:highlightId/preview` URL 与 Animated WebP 展示格式，同时把最重的源视频 Seek/解码转移给 Desktop/Android。

## `preview_thumbnail_v1` 调度

该 Job 是用户 hover 触发的短生命周期交互任务，因此有独立的有界等待策略：

- `auto`
  1. 有新鲜、空闲且具备 capability 的 Desktop 时优先领取；
  2. Desktop 不可用/忙碌/过期时 Android 可领取；
  3. 没有可立即接单的客户端时直接 Server fallback；
  4. 已注册客户端任务最多等待 10 秒，超时后先取消远端任务，再走原有 Server FFmpeg fallback。
- `client_preferred`
  - 只允许客户端计算；没有节点或超时则不启动 Server FFmpeg。
- `server_only`
  - 直接沿用原有 Server FFmpeg lazy preview。
- `off`
  - 不产生新的预览计算；已经缓存的 preview 仍可直接读取。

只有最近活跃且当前 `idle` 的节点才会让交互请求进入客户端等待窗口，避免已经退出、忙碌或过期的节点增加 hover 延迟。

超时进入 Server fallback 前会先注销 waiter 和远端 Claim；迟到的客户端即使已经开始回传，也会在持久化前再次检查任务有效性，因此不会与 Server fallback 并发覆盖同一预览文件。

## V2 Claim envelope

```json
{
  "protocol_version": 2,
  "job_type": "preview_thumbnail_v1",
  "required_capability": "preview_thumbnail_v1",
  "task_id": "...",
  "claim_token": "...",
  "lease_expires_at": "...",
  "input": {
    "media_id": "...",
    "highlight_id": "...",
    "fingerprint": "...",
    "stream_url": "/api/stream/.../direct",
    "frame_times": [100.25, 100.75, 101.25, 101.75, 102.25],
    "max_width": 420,
    "frame_rate": 2
  }
}
```

客户端必须先按 `job_type` 分派执行器，再解析对应 `input`。Android 使用通用 `JsonObject` 保存 Claim input，Desktop 使用 `unknown` + job adapter 校验，因此协议不依赖某一个业务结构。

## V2 result envelope

```json
{
  "claim_token": "...",
  "job_type": "preview_thumbnail_v1",
  "result": {
    "fingerprint": "...",
    "highlight_id": "...",
    "frames": [
      {
        "time": 100.25,
        "mime": "image/webp",
        "data_base64": "..."
      }
    ]
  }
}
```

服务端先校验 claim + job_type，再进入对应 result adapter。客户端结果从不被直接信任。

## V1 兼容

为了允许 Server、Desktop、Android 分批升级：

- 现有 `/api/media-analysis/workers/**` URL 继续作为兼容传输层。
- `highlight_v1` Claim 顶层继续镜像 `media_id / fingerprint / duration / stream_url / sample_times / max_highlights / engine_version`。
- 旧客户端忽略 V2 新字段，并继续读取旧扁平字段。
- complete 路由同时接受 V1 highlight 扁平结果和 V2 `job_type + result` envelope。
- 旧客户端不会声明 `preview_thumbnail_v1`，因此永远不会误领第二 Job。

## 通用任务注册

业务通过 `RegisterComputeTask(MediaComputeTaskRegistration)` / `UnregisterComputeTask(taskID)` 进入统一协调层，只需要提供：

- `task_id`
- `job_type`
- `required_capability`
- job 私有 `input` JSON
- 可选 `media_id / fingerprint`

节点协调层负责在线状态、capability 匹配、Claim、租约和优先级；业务服务负责自己的 durable/ephemeral 生命周期、Server fallback 与 result adapter。

`highlight_v1` 是持久化长任务，使用 `AIAnalysisTask` + progress；`preview_thumbnail_v1` 是请求级临时任务，不强制写历史任务表。这验证了同一 Media Compute Node V2 可以同时承载不同生命周期的工作负载。

## Node view

管理端节点响应包括：

- `client_protocol_version`
- `current_job_type`
- `capabilities`

新 Desktop/Android 通过 `desktop-v2/...`、`android-v2/...` 标识理解 V2 envelope，并明确声明当前真正支持的能力。

## 新 Job 接入规则

1. 每个任务定义独立 `job_type`。
2. 每个执行能力定义独立 `required_capability`。
3. job 私有参数只进入 `input`，禁止继续增加 Claim 顶层业务字段。
4. Desktop/Android 只声明自己真实可执行的 capabilities。
5. Server 必须按 required capability 选择节点。
6. 长任务可使用 progress + lease renewal；短任务可以依赖足够长的 Claim lease，但必须有请求级超时/取消机制。
7. job 结果必须通过 Server adapter 校验后才能持久化。
8. 每个 dispatcher 必须定义 Server fallback 或显式 client-only 策略，不能无限等待不存在的节点。
9. 客户端计算超时进入 fallback 前必须撤销远端任务，防止迟到结果覆盖 fallback 产物。

## 下一阶段

第二个真实 Job 已经验证了通用 Claim、capability 调度、临时任务生命周期、Desktop/Android 执行器和 Server fallback。

下一步优先建议接入：

1. `chapter_detect_v1`：把场景切换/章节候选扫描迁移到客户端节点；
2. 或 `waveform_v1`：验证音频类 workload，而不是继续只验证视频抽帧类任务。

在更多客户端版本实际发布并稳定后，再考虑把历史 `/media-analysis/workers/**` 路由迁移为 `/media-compute/**`，不要为了命名提前破坏 V1 兼容。
